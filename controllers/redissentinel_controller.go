package controllers

import (
	"context"
	"time"

	"github.com/go-logr/logr"
	redisv1beta2 "github.com/teocns/redis-operator/api/v1beta2"
	"github.com/teocns/redis-operator/k8sutils"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/util/workqueue"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/handler"
)

// RedisSentinelReconciler reconciles a RedisSentinel object
type RedisSentinelReconciler struct {
	client.Client
	K8sClient  kubernetes.Interface
	Dk8sClient dynamic.Interface
	Log        logr.Logger
	Scheme     *runtime.Scheme
}

func (r *RedisSentinelReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	reqLogger := r.Log.WithValues("Request.Namespace", req.Namespace, "Request.Name", req.Name)
	reqLogger.Info("Reconciling opstree redis controller")
	instance := &redisv1beta2.RedisSentinel{}

	err := r.Client.Get(context.TODO(), req.NamespacedName, instance)
	if err != nil {
		if errors.IsNotFound(err) {
			return ctrl.Result{}, nil
		}
		return ctrl.Result{}, err
	}

	if _, found := instance.ObjectMeta.GetAnnotations()["redissentinel.opstreelabs.in/skip-reconcile"]; found {
		reqLogger.Info("Found annotations redissentinel.opstreelabs.in/skip-reconcile, so skipping reconcile")
		return ctrl.Result{RequeueAfter: time.Second * 10}, nil
	}

	if instance.Spec.RedisSentinelConfig != nil && !k8sutils.IsRedisReplicationReady(ctx, reqLogger, r.K8sClient, r.Dk8sClient, instance) {
		reqLogger.Info("Redis Replication is specified but not ready, so will reconcile again in 10 seconds")
		return ctrl.Result{RequeueAfter: time.Second * 10}, nil
	}

	// Get total Sentinel Replicas
	// sentinelReplicas := instance.Spec.GetSentinelCounts("sentinel")

	if err = k8sutils.HandleRedisSentinelFinalizer(r.Client, r.Log, instance); err != nil {
		return ctrl.Result{RequeueAfter: time.Second * 60}, err
	}

	if err = k8sutils.AddFinalizer(instance, k8sutils.RedisSentinelFinalizer, r.Client); err != nil {
		return ctrl.Result{RequeueAfter: time.Second * 60}, err
	}

	// Create Redis Sentinel
	err = k8sutils.CreateRedisSentinel(ctx, r.K8sClient, r.Log, instance, r.K8sClient, r.Dk8sClient)
	if err != nil {
		return ctrl.Result{}, err
	}

	err = k8sutils.ReconcileSentinelPodDisruptionBudget(instance, instance.Spec.PodDisruptionBudget, r.K8sClient)
	if err != nil {
		return ctrl.Result{}, err
	}

	// Create the Service for Redis Sentinel
	err = k8sutils.CreateRedisSentinelService(instance, r.K8sClient)
	if err != nil {
		return ctrl.Result{}, err
	}

	reqLogger.Info("Will reconcile after 600 seconds")
	return ctrl.Result{RequeueAfter: time.Second * 600}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *RedisSentinelReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&redisv1beta2.RedisSentinel{}).
		Watches(&redisv1beta2.RedisReplication{}, &handler.Funcs{
			CreateFunc: nil,
			UpdateFunc: func(ctx context.Context, event event.UpdateEvent, limitingInterface workqueue.RateLimitingInterface) {
				_ = event.ObjectNew.GetName()
				_ = event.ObjectNew.GetNamespace()
			},
			DeleteFunc:  nil,
			GenericFunc: nil,
		}).
		Complete(r)
}
