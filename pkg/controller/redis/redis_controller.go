package redis

import (
	"time"
	"strconv"
	"context"

	redisv1alpha1 "redis-operator/pkg/apis/redis/v1alpha1"
	"redis-operator/pkg/utils"

	appsv1 "k8s.io/api/apps/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	// corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
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

func (r *ReconcileRedis) Reconcile(request reconcile.Request) (reconcile.Result, error) {
	reqLogger := log.WithValues("Request.Namespace", request.Namespace, "Request.Name", request.Name)
	reqLogger.Info("Reconciling Opstree Redis")

	instance := &redisv1alpha1.Redis{}
	err := r.client.Get(context.TODO(), request.NamespacedName, instance)
	if err != nil {
		if errors.IsNotFound(err) {
			return reconcile.Result{}, nil
		}
		return reconcile.Result{}, err
	}

	if err := controllerutil.SetControllerReference(instance, instance, r.scheme); err != nil {
		return reconcile.Result{}, err
	}

	found := &appsv1.StatefulSet{}
	err = r.client.Get(context.TODO(), types.NamespacedName{Name: instance.Name, Namespace: instance.Namespace}, found)
	if err != nil && errors.IsNotFound(err) {
		otmachinery.CreateRedisSecret(instance)
		if instance.Spec.Mode == "cluster" {
			otmachinery.CreateRedisMaster(instance)
			otmachinery.CreateMasterService(instance)
			otmachinery.CreateMasterHeadlessService(instance)
			otmachinery.CreateRedisSlave(instance)
			otmachinery.CreateSlaveService(instance)
			otmachinery.CreateSlaveHeadlessService(instance)
			redisMasterInfo, err := otmachinery.GenerateK8sClient().AppsV1().StatefulSets(instance.Namespace).Get(instance.ObjectMeta.Name + "-master", metav1.GetOptions{})
			if err != nil {
				return reconcile.Result{}, err
			}
			redisSlaveInfo, err := otmachinery.GenerateK8sClient().AppsV1().StatefulSets(instance.Namespace).Get(instance.ObjectMeta.Name + "-slave", metav1.GetOptions{})
			if int(redisMasterInfo.Status.ReadyReplicas) != int(*instance.Spec.Size) && int(redisSlaveInfo.Status.ReadyReplicas) != int(*instance.Spec.Size) {
				reqLogger.Info("Redis master and slave nodes are not ready yet", "Ready.Replicas", strconv.Itoa(int(redisMasterInfo.Status.ReadyReplicas)))
				return reconcile.Result{RequeueAfter: time.Second*120}, nil
			} else {
				reqLogger.Info("Creating redis cluster by executing cluster creation command", "Ready.Replicas", strconv.Itoa(int(redisMasterInfo.Status.ReadyReplicas)))
				if otmachinery.CheckRedisCluster(instance) != int(*instance.Spec.Size) * 2 {
					otmachinery.ExecuteRedisClusterCommand(instance)
					otmachinery.ExecuteRedisReplicationCommand(instance)
				} else {
					reqLogger.Info("Redis master count is desired")
				}
			}
		} else if instance.Spec.Mode == "standalone" {
			otmachinery.CreateRedisStandalone(instance)
			otmachinery.CreateStandaloneService(instance)
		}
	} else if err != nil {
		return reconcile.Result{}, err
	}
	reqLogger.Info("Skipping reconcile: Cluster already exists", "Redis.Namespace", instance.Namespace, "Redis.Name", instance.Name)
	return reconcile.Result{}, nil
}
