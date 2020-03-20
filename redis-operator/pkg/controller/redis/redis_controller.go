package redis

import (
	"context"

	redisv1alpha1 "redis-operator/redis-operator/pkg/apis/redis/v1alpha1"
	"redis-operator/redis-operator/pkg/utils"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	// metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
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
const (
	constAppImage         = "opstree/redis"
	constAppContainerName = "redis"
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
	err = c.Watch(&source.Kind{Type: &redisv1alpha1.Redis{}}, &handler.EnqueueRequestForObject{})
	if err != nil {
		return err
	}

	// TODO(user): Modify this to be the types you create that are owned by the primary resource
	// Watch for changes to secondary resource Pods and requeue the owner Redis
	err = c.Watch(&source.Kind{Type: &corev1.Pod{}}, &handler.EnqueueRequestForOwner{
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

	// Fetch the Redis instance
	instance := &redisv1alpha1.Redis{}
	err := r.client.Get(context.TODO(), request.NamespacedName, instance)
	if err != nil {
		if errors.IsNotFound(err) {
			// Request object not found, could have been deleted after reconcile request.
			// Owned objects are automatically garbage collected. For additional cleanup logic use finalizers.
			// Return and don't requeue
			return reconcile.Result{}, nil
		}
		// Error reading the object - requeue the request.
		return reconcile.Result{}, err
	}

	redisMaster := otmachinery.CreateRedisMaster(instance)
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
			if err != nil {
				return reconcile.Result{}, err
			}
		}
	} else if err != nil {
		return reconcile.Result{}, err
	}
	reqLogger.Info("Skip reconcile: Cluster already exists", "Redis.Namespace", found.Namespace, "Redis.Name", found.Name)
	return reconcile.Result{}, nil
}
