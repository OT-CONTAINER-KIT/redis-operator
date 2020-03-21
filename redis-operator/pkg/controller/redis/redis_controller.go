package redis

import (
	"time"
	"strconv"
	"context"

	redisv1alpha1 "redis-operator/redis-operator/pkg/apis/redis/v1alpha1"
	"redis-operator/redis-operator/pkg/utils"

	appsv1 "k8s.io/api/apps/v1"
	// corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/source"
)

var log = logf.Log.WithName("controller_redis")

/**
* USER ACTION REQUIRED: This is a scaffold file intended for the user to modify with their own Controller
* business logic.  Delete these comments after modifying this file.*
 */

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
	err = c.Watch(&source.Kind{Type: &redisv1alpha1.Redis{}}, &handler.EnqueueRequestForObject{})
	if err != nil {
		return err
	}

	// TODO(user): Modify this to be the types you create that are owned by the primary resource
	// Watch for changes to secondary resource Pods and requeue the owner Redis
	err = c.Watch(&source.Kind{Type: &appsv1.StatefulSet{}}, &handler.EnqueueRequestForOwner{
		IsController: true,
		OwnerType:    &redisv1alpha1.Redis{},
	})
	if err != nil {
		return err
	}

	return nil
}

// blank assignment to verify that ReconcileRedis implements reconcile.Reconciler
var _ reconcile.Reconciler = &ReconcileRedis{}

// ReconcileRedis reconciles a Redis object
type ReconcileRedis struct {
	// This client, initialized using mgr.Client() above, is a split client
	// that reads objects from the cache and writes to the apiserver
	client client.Client
	scheme *runtime.Scheme
}

// Reconcile reads that state of the cluster for a Redis object and makes changes based on the state read
// and what is in the Redis.Spec
// TODO(user): Modify this Reconcile function to implement your Controller logic.  This example creates
// a Pod as an example
// Note:
// The Controller will requeue the Request to be processed again if the returned error is non-nil or
// Result.Requeue is true, otherwise upon completion it will remove the work from the queue.
func (r *ReconcileRedis) Reconcile(request reconcile.Request) (reconcile.Result, error) {
	reqLogger := log.WithValues("Request.Namespace", request.Namespace, "Request.Name", request.Name)
	reqLogger.Info("Reconciling Opstree Redis")

	config, _ := rest.InClusterConfig()
	clientset, _ := kubernetes.NewForConfig(config)
	// Fetch the Redis instance
	instance := &redisv1alpha1.Redis{}
	err := r.client.Get(context.TODO(), request.NamespacedName, instance)
	if err != nil {
		if errors.IsNotFound(err) {
			return reconcile.Result{}, nil
		}
		// Error reading the object - requeue the request.
		return reconcile.Result{}, err
	}

	redisMaster := otmachinery.CreateRedisMaster(instance)
	redisSlave  := otmachinery.CreateRedisSlave(instance)
	// Set Redis instance as the owner and controller
	if err := controllerutil.SetControllerReference(instance, redisMaster, r.scheme); err != nil {
		return reconcile.Result{}, err
	}

	found := &appsv1.StatefulSet{}
	err = r.client.Get(context.TODO(), types.NamespacedName{Name: redisMaster.Name, Namespace: redisMaster.Namespace}, found)
	if err != nil && errors.IsNotFound(err) {
		if instance.Spec.Mode == "cluster" {
			reqLogger.Info("Creating a new Redis master setup", "Namespace", redisMaster.Namespace, "Master.Name", redisMaster.Name)
			err = r.client.Create(context.TODO(), redisMaster)
			for replicaCount, _ := range instance.Spec.Master {
				reqLogger.Info("Creating redis master services", "Namespace", redisMaster.Namespace, "Master.Name", redisMaster.Name, "Service.Name", instance.ObjectMeta.Name + "-master" + strconv.Itoa(int(replicaCount)))
				redisMasterService := otmachinery.CreateMasterService(instance, "master", strconv.Itoa(int(replicaCount)))
				err = r.client.Create(context.TODO(), redisMasterService)
			}
			reqLogger.Info("Creating redis master headless services", "Namespace", redisMaster.Namespace, "Master.Name", redisMaster.Name, "Headless.Service.Name", instance.ObjectMeta.Name + "-master")
			redisMasterHeadlessService := otmachinery.CreateMasterHeadlessService(instance, "master")
			err = r.client.Create(context.TODO(), redisMasterHeadlessService)
			if err != nil {
				return reconcile.Result{}, err
			}
		}
	} else if err != nil {
		return reconcile.Result{}, err
	} else {
		redisMasterInfo, err := clientset.AppsV1().StatefulSets(redisMaster.Namespace).Get(instance.ObjectMeta.Name + "-master", metav1.GetOptions{})
		if err != nil {
			return reconcile.Result{}, err
		}
		_, err = clientset.AppsV1().StatefulSets(redisSlave.Namespace).Get(instance.ObjectMeta.Name + "-slave", metav1.GetOptions{})
		if err != nil {
			if int(redisMasterInfo.Status.ReadyReplicas) != int(*redisMaster.Spec.Replicas) {
				reqLogger.Info("Redis master replicas are not ready yet", "Namespace", redisMaster.Namespace, "Master.Name", redisMaster.Name)
				return reconcile.Result{RequeueAfter: time.Second*60}, nil
			} else {
				reqLogger.Info("Creating a new Redis slave setup", "Namespace", redisSlave.Namespace, "Slave.Name", redisSlave.Name)
				err = r.client.Create(context.TODO(), redisSlave)
				for replicaCount, _ := range instance.Spec.Master {
					reqLogger.Info("Creating redis slave serveices", "Namespace", redisSlave.Namespace, "Slave.Name", redisSlave.Name, "Service.Name", instance.ObjectMeta.Name + "-slave" + strconv.Itoa(int(replicaCount)))
					redisSlaveService := otmachinery.CreateSlaveService(instance, "slave", strconv.Itoa(int(replicaCount)))
					err = r.client.Create(context.TODO(), redisSlaveService)
				}
				redisSlaveHeadlessService := otmachinery.CreateSlaveHeadlessService(instance, "slave")
				err = r.client.Create(context.TODO(), redisSlaveHeadlessService)
				if err != nil {
					return reconcile.Result{}, err
				}
			}
		}
	}
	reqLogger.Info("Skip reconcile: Cluster already exists", "Redis.Namespace", instance.Namespace, "Redis.Name", instance.Name)
	return reconcile.Result{}, nil
}
