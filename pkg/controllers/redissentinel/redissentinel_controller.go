package redissentinel

import (
	"context"
	"time"

	redisv1beta2 "github.com/OT-CONTAINER-KIT/redis-operator/api/v1beta2"
	intctrlutil "github.com/OT-CONTAINER-KIT/redis-operator/pkg/controllerutil"
	"github.com/OT-CONTAINER-KIT/redis-operator/pkg/k8sutils"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
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
	Scheme     *runtime.Scheme

	ReplicationWatcher *intctrlutil.ResourceWatcher
}

func (r *RedisSentinelReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	instance := &redisv1beta2.RedisSentinel{}

	err := r.Client.Get(context.TODO(), req.NamespacedName, instance)
	if err != nil {
		return intctrlutil.RequeueWithErrorChecking(ctx, err, "")
	}
	if instance.ObjectMeta.GetDeletionTimestamp() != nil {
		if err = k8sutils.HandleRedisSentinelFinalizer(ctx, r.Client, instance); err != nil {
			return intctrlutil.RequeueWithError(ctx, err, "")
		}
		return intctrlutil.Reconciled()
	}

	if _, found := instance.ObjectMeta.GetAnnotations()["redissentinel.opstreelabs.in/skip-reconcile"]; found {
		return intctrlutil.RequeueAfter(ctx, time.Second*10, "found skip reconcile annotation")
	}

	// Get total Sentinel Replicas
	// sentinelReplicas := instance.Spec.GetSentinelCounts("sentinel")

	if err = k8sutils.AddFinalizer(ctx, instance, k8sutils.RedisSentinelFinalizer, r.Client); err != nil {
		return intctrlutil.RequeueWithError(ctx, err, "")
	}

	if instance.Spec.RedisSentinelConfig != nil && !k8sutils.IsRedisReplicationReady(ctx, r.K8sClient, r.Dk8sClient, instance) {
		return intctrlutil.RequeueAfter(ctx, time.Second*10, "Redis Replication is specified but not ready")
	}

	if instance.Spec.RedisSentinelConfig != nil {
		r.ReplicationWatcher.Watch(
			ctx,
			types.NamespacedName{
				Namespace: req.Namespace,
				Name:      instance.Spec.RedisSentinelConfig.RedisReplicationName,
			},
			req.NamespacedName,
		)
	}

	// Create Redis Sentinel
	err = k8sutils.CreateRedisSentinel(ctx, r.K8sClient, instance, r.K8sClient, r.Dk8sClient)
	if err != nil {
		return intctrlutil.RequeueWithError(ctx, err, "")
	}

	err = k8sutils.ReconcileSentinelPodDisruptionBudget(ctx, instance, instance.Spec.PodDisruptionBudget, r.K8sClient)
	if err != nil {
		return intctrlutil.RequeueWithError(ctx, err, "")
	}

	// Create the Service for Redis Sentinel
	err = k8sutils.CreateRedisSentinelService(ctx, instance, r.K8sClient)
	if err != nil {
		return intctrlutil.RequeueWithError(ctx, err, "")
	}
	return intctrlutil.Reconciled()
}

// SetupWithManager sets up the controller with the Manager.
func (r *RedisSentinelReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&redisv1beta2.RedisSentinel{}).
		Watches(&redisv1beta2.RedisReplication{}, r.ReplicationWatcher).
		Complete(r)
}
