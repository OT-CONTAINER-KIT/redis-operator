package redissentinel

import (
	"context"
	"time"

	redisv1beta2 "github.com/OT-CONTAINER-KIT/redis-operator/api/v1beta2"
	intctrlutil "github.com/OT-CONTAINER-KIT/redis-operator/pkg/controllerutil"
	"github.com/OT-CONTAINER-KIT/redis-operator/pkg/k8sutils"
	"github.com/go-logr/logr"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
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
		return intctrlutil.RequeueWithErrorChecking(err, reqLogger, "")
	}
	if instance.ObjectMeta.GetDeletionTimestamp() != nil {
		if err = k8sutils.HandleRedisSentinelFinalizer(r.Client, r.Log, instance); err != nil {
			return intctrlutil.RequeueWithError(err, reqLogger, "")
		}
		return intctrlutil.Reconciled()
	}

	if _, found := instance.ObjectMeta.GetAnnotations()["redissentinel.opstreelabs.in/skip-reconcile"]; found {
		return intctrlutil.RequeueAfter(reqLogger, time.Second*10, "found skip reconcile annotation")
	}

	// Get total Sentinel Replicas
	// sentinelReplicas := instance.Spec.GetSentinelCounts("sentinel")

	if err = k8sutils.AddFinalizer(instance, k8sutils.RedisSentinelFinalizer, r.Client); err != nil {
		return intctrlutil.RequeueWithError(err, reqLogger, "")
	}

	if instance.Spec.RedisSentinelConfig != nil && !k8sutils.IsRedisReplicationReady(ctx, reqLogger, r.K8sClient, r.Dk8sClient, instance) {
		return intctrlutil.RequeueAfter(reqLogger, time.Second*10, "Redis Replication is specified but not ready")
	}

	// Create Redis Sentinel
	err = k8sutils.CreateRedisSentinel(ctx, r.K8sClient, r.Log, instance, r.K8sClient, r.Dk8sClient)
	if err != nil {
		return intctrlutil.RequeueWithError(err, reqLogger, "")
	}

	err = k8sutils.ReconcileSentinelPodDisruptionBudget(instance, instance.Spec.PodDisruptionBudget, r.K8sClient)
	if err != nil {
		return intctrlutil.RequeueWithError(err, reqLogger, "")
	}

	// Create the Service for Redis Sentinel
	err = k8sutils.CreateRedisSentinelService(instance, r.K8sClient)
	if err != nil {
		return intctrlutil.RequeueWithError(err, reqLogger, "")
	}
	return intctrlutil.RequeueAfter(reqLogger, time.Second*10, "")
}

// SetupWithManager sets up the controller with the Manager.
func (r *RedisSentinelReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&redisv1beta2.RedisSentinel{}).
		Complete(r)
}
