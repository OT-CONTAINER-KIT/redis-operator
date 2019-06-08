package redis

import (
	"context"
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"sync"

	k8sv1alpha1 "github.com/iamabhishek-dubey/redis-operator/pkg/apis/k8s/v1alpha1"
	"github.com/iamabhishek-dubey/redis-operator/pkg/redis"

	"github.com/cenkalti/backoff"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	policyv1beta1 "k8s.io/api/policy/v1beta1"
	"k8s.io/apimachinery/pkg/api/errors"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"

	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	logf "sigs.k8s.io/controller-runtime/pkg/runtime/log"
	"sigs.k8s.io/controller-runtime/pkg/source"
)

var (
	log = logf.Log.WithName("controller_redis")
	// used to check if the password is a simple alphanumeric string
	isAlphaNumeric = regexp.MustCompile(`^[[:alnum:]]+$`).MatchString
)

// Add creates a new Redis Controller and adds it to the Manager. The Manager will set fields on the Controller
// and Start it when the Manager is Started.
func Add(mgr manager.Manager) error {
	return add(mgr, newReconciler(mgr))
}

// newReconciler returns a new reconcile.Reconciler
func newReconciler(mgr manager.Manager) reconcile.Reconciler {
	return &ReconcileRedis{client: mgr.GetClient(), scheme: mgr.GetScheme()}
}

// add adds a new Controller to mgr with r as the reconcile.Reconciler
func add(mgr manager.Manager, r reconcile.Reconciler) error {
	// Create a new controller
	c, err := controller.New("redis-controller", mgr, controller.Options{Reconciler: r})
	if err != nil {
		return err
	}

	// Watch for changes to primary resource Redis
	if err := c.Watch(
		&source.Kind{Type: &k8sv1alpha1.Redis{}},
		&handler.EnqueueRequestForObject{},
	); err != nil {
		return err
	}

	for _, object := range []runtime.Object{
		&corev1.Secret{},
		&corev1.ConfigMap{},
		&corev1.Service{},
		&policyv1beta1.PodDisruptionBudget{},
		&appsv1.StatefulSet{},
	} {
		if err := c.Watch(
			&source.Kind{Type: object},
			&handler.EnqueueRequestForOwner{OwnerType: &k8sv1alpha1.Redis{}, IsController: true},
		); err != nil {
			return err
		}
	}
	return nil
}

// ReconcileRedis reconciles a Redis object
type ReconcileRedis struct {
	// This client, initialized using mgr.Client() above, is a split client
	// that reads objects from the cache and writes to the apiserver
	client client.Client
	scheme *runtime.Scheme
}

// strict implementation check
var _ reconcile.Reconciler = (*ReconcileRedis)(nil)

// Reconcile reads that state of the cluster for a Redis object and makes changes based on the state read
// and what is in the Redis.Spec
// Note:
// The Controller will requeue the Request to be processed again if the returned error is non-nil or
// Result.Requeue is true, otherwise upon completion it will remove the work from the queue.
func (reconciler *ReconcileRedis) Reconcile(request reconcile.Request) (reconcile.Result, error) {
	logger := log.WithValues("Namespace", request.Namespace, "Redis", request.Name)
	loggerDebug := logger.V(1).Info
	loggerDebug("Reconciling Redis")

	// Fetch the Redis instance
	fetchedRedis := &k8sv1alpha1.Redis{}
	if err := reconciler.client.Get(context.TODO(), request.NamespacedName, fetchedRedis); err != nil {
		if errors.IsNotFound(err) {
			// Request object not found, could have been deleted after reconcile request.
			// Owned objects are automatically garbage collected. For additional cleanup logic use finalizers.
			// Return and don't requeue
			return reconcile.Result{}, nil
		}
		// Error reading the object - requeue the request.
		return reconcile.Result{}, err
	}

	// work with the copy
	redisObject := fetchedRedis.DeepCopy()
	// initialize options
	options := objectGeneratorOptions{serviceType: serviceTypeAll}
	// adding some default labels on top of user-defined
	if redisObject.Labels == nil {
		redisObject.Labels = map[string]string{}
	}
	redisObject.Labels[redisName] = redisObject.Name

	// read password from Secret
	if redisObject.Spec.Password.SecretKeyRef != nil {
		passwordSecret := &corev1.Secret{}
		if err := reconciler.client.Get(
			context.TODO(),
			types.NamespacedName{Name: redisObject.Spec.Password.SecretKeyRef.Name, Namespace: request.Namespace},
			passwordSecret,
		); err != nil {
			return reconcile.Result{}, fmt.Errorf("Failed to fetch password: %s", err)
		}

		options.password = string(passwordSecret.Data[redisObject.Spec.Password.SecretKeyRef.Key])
		// Warning: since Redis is pretty fast an outside user can try up to
		// 150k passwords per second against a good box. This means that you should
		// use a very strong password otherwise it will be very easy to break.
		if len(options.password) < 8 && isAlphaNumeric(options.password) {
			logger.Info("WARNING: The password looks weak, please change it.")
		}
	}

	// create or update resources
	for i, object := range []runtime.Object{
		&corev1.Service{}, &corev1.Service{}, &corev1.Service{}, // 3 distinct services ;)
		&corev1.Secret{},
		&corev1.ConfigMap{},
		&policyv1beta1.PodDisruptionBudget{},
		&appsv1.StatefulSet{},
	} {
		switch object.(type) {
		case *corev1.ConfigMap, *policyv1beta1.PodDisruptionBudget, *appsv1.StatefulSet:
			// nothing special to do here
		case *corev1.Secret:
			if options.password == "" {
				continue
			}
		case *corev1.Service:
			// a bit hacky way to create three different instances of *v1.Service
			// without copy-pasting and introducing all the corresponding risks
			options.serviceType = serviceTypeAll + i
		default:
			// unknown type
			continue
		}

		if result, err := reconciler.createOrUpdate(object, redisObject, options); err != nil {
			return reconcile.Result{}, err
		} else if result.Requeue {
			logger.Info(fmt.Sprintf("Applied %T", object))
			return result, nil
		}
	}

	// all the kubernetes resources are OK.
	// Redis failover state should be checked and reconfigured if needed.
	podList := &corev1.PodList{}
	if err := reconciler.client.List(
		context.TODO(),
		&client.ListOptions{Namespace: request.Namespace, LabelSelector: labels.SelectorFromSet(redisObject.Labels)},
		podList,
	); err != nil {
		return reconcile.Result{}, fmt.Errorf("Failed to list Pods: %s", err)
	}

	addrs := []redis.Address{}

podIter:
	// filter out pods without assigned IP addresses and not having all containers ready
	for i := range podList.Items {
		if podList.Items[i].Status.Phase != corev1.PodRunning || podList.Items[i].Status.PodIP == "" {
			continue
		}
		for _, status := range podList.Items[i].Status.ContainerStatuses {
			if !status.Ready {
				continue podIter
			}
		}
		addrs = append(addrs, redis.Address{Host: podList.Items[i].Status.PodIP, Port: strconv.Itoa(redis.Port)})
	}

	// Run Redis Replication Reconfiguration
	instances, err := redis.NewInstances(options.password, addrs...)
	if err != nil {
		// This is considered part of normal operation - return and requeue
		logger.Info("Error creating Redis instances, requeueing", "error", err)
		return reconcile.Result{Requeue: true}, nil
	}
	defer instances.Disconnect()

	if len(instances) < redis.MinimumFailoverSize {
		// most likely not all Pods are ready - return and requeue
		logger.Info("No failover to perform")
		return reconcile.Result{Requeue: true}, nil
	}

	if err := instances.Reconfigure(); err != nil {
		return reconcile.Result{}, fmt.Errorf("Error reconfiguring instances: %s", err)
	}

	// Select master and assign the master and replica labels to the corresponding Pods.
	// Wrapping it with the exponential backoff timer in order to wait for the updated info replication.
	master := &redis.Redis{}
	exponentialBackOff := backoff.NewExponentialBackOff()
	exponentialBackOff.MaxElapsedTime = redis.DefaultFailoverTimeout

	if err := backoff.Retry(func() error {
		if err := instances.Refresh(); err != nil {
			return err
		}

		if master = instances.SelectMaster(); master == nil {
			return fmt.Errorf("No master discovered")
		}
		return nil
	}, exponentialBackOff); err != nil {
		logger.Info("No master discovered, requeueing", "error", err, "instances", instances)
		return reconcile.Result{Requeue: true}, nil
	}

	// update Pod labels asynchronously and fetch the master Pod's name
	var wg sync.WaitGroup
	errChan := make(chan error, len(podList.Items))
	masterChan := make(chan string, 1)

	wg.Add(len(podList.Items))
	for i := range podList.Items {
		go func(pod corev1.Pod, masterAddress string, wg *sync.WaitGroup) {
			defer wg.Done()
			if pod.Status.PodIP == masterAddress {
				select {
				case masterChan <- pod.Name:
					if pod.Labels[roleLabelKey] == masterLabel {
						return
					}
					pod.Labels[roleLabelKey] = masterLabel
				default:
					// very unlikely to happen but still...
					errChan <- fmt.Errorf("IP address conflict for pod %s: %s", pod.Name, pod.Status.PodIP)
					return
				}
			} else {
				if pod.Labels[roleLabelKey] == replicaLabel {
					return
				}
				pod.Labels[roleLabelKey] = replicaLabel
			}
			if err := reconciler.client.Update(context.TODO(), &pod); err != nil {
				errChan <- err
			}
		}(podList.Items[i], master.Host, &wg)
	}
	wg.Wait()

	close(errChan)
	if len(errChan) > 0 {
		var b strings.Builder
		defer b.Reset()
		for err := range errChan {
			if !errors.IsConflict(err) {
				fmt.Fprintf(&b, " %s;", err)
			}
		}
		if b.Len() > 0 {
			return reconcile.Result{}, fmt.Errorf("Failed to update Pods:%s", b.String())
		}
		loggerDebug("Conflict updating Pods, requeueing")
		return reconcile.Result{Requeue: true}, nil
	}

	// update configmap with the current master's IP address
	options.master = master.Address
	if result, err := reconciler.createOrUpdate(&corev1.ConfigMap{}, redisObject, options); err != nil {
		return result, err
	} else if result.Requeue {
		logger.Info("Updated ConfigMap")
		return result, nil
	}

	masterPodName := <-masterChan
	if fetchedRedis.Status.Replicas == len(instances) && fetchedRedis.Status.Master == masterPodName {
		// Everything is OK - don't requeue
		return reconcile.Result{}, nil
	}

	fetchedRedis.Status.Replicas = len(instances)
	fetchedRedis.Status.Master = masterPodName
	if err := reconciler.client.Status().Update(context.TODO(), fetchedRedis); err != nil {
		if errors.IsConflict(err) {
			loggerDebug("Conflict updating Redis status, requeueing")
			return reconcile.Result{Requeue: true}, nil
		}
		return reconcile.Result{}, fmt.Errorf("Failed to update Redis status: %s", err)
	}
	logger.Info("Updated Redis status")
	return reconcile.Result{}, nil
}

// createOrUpdate abstracts away keeping in sync the desired and actual state of Kubernetes objects.
// passing an empty instance implementing runtime.Object will generate the appropriate ``expected'' object,
// create an object if it does not exist, compare the existing object with the generated one and update if needed.
// the Result.Requeue will be true if the object was successfully created or updated or in case there was a conflict updating the object.
func (reconciler *ReconcileRedis) createOrUpdate(object runtime.Object, redis *k8sv1alpha1.Redis, options objectGeneratorOptions) (result reconcile.Result, err error) {
	name, generatedObject := generateObject(redis, object, options)

	if err = reconciler.client.Get(context.TODO(), types.NamespacedName{Name: name, Namespace: redis.Namespace}, object); err != nil {
		if errors.IsNotFound(err) {
			// Set Redis instance as the owner and controller
			if err = controllerutil.SetControllerReference(redis, generatedObject.(metav1.Object), reconciler.scheme); err != nil {
				return reconcile.Result{}, fmt.Errorf("Failed to set owner for Object: %s", err)
			}
			if err = reconciler.client.Create(context.TODO(), generatedObject); err != nil && !errors.IsAlreadyExists(err) {
				return reconcile.Result{}, fmt.Errorf("Failed to create Object: %s", err)
			}
			return reconcile.Result{Requeue: true}, nil
		}
		return reconcile.Result{}, fmt.Errorf("Failed to fetch Object: %s", err)
	}

	if !objectUpdateNeeded(object, generatedObject) {
		return
	}

	if err = reconciler.client.Update(context.TODO(), object); err != nil {
		if errors.IsConflict(err) {
			// conflicts can be common, consider it part of normal operation
			return reconcile.Result{Requeue: true}, nil
		}
		return reconcile.Result{}, fmt.Errorf("Failed to update Object: %s", err)
	}
	return reconcile.Result{Requeue: true}, nil
}
