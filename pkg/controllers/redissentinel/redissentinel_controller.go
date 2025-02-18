package redissentinel

import (
	"context"
	"time"

	redisv1beta2 "github.com/OT-CONTAINER-KIT/redis-operator/api/v1beta2"
	intctrlutil "github.com/OT-CONTAINER-KIT/redis-operator/pkg/controllerutil"
	"github.com/OT-CONTAINER-KIT/redis-operator/pkg/k8sutils"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

// RedisSentinelReconciler reconciles a RedisSentinel object
type RedisSentinelReconciler struct {
	client.Client
	K8sClient          kubernetes.Interface
	Dk8sClient         dynamic.Interface
	ReplicationWatcher *intctrlutil.ResourceWatcher
}

func (r *RedisSentinelReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	instance := &redisv1beta2.RedisSentinel{}

	err := r.Client.Get(ctx, req.NamespacedName, instance)
	if err != nil {
		return intctrlutil.RequeueWithErrorChecking(ctx, err, "failed to get RedisSentinel instance")
	}

	var reconcilers []reconciler
	if k8sutils.IsDeleted(instance) {
		reconcilers = []reconciler{
			{typ: "finalizer", rec: r.reconcileFinalizer},
		}
	} else {
		reconcilers = []reconciler{
			{typ: "annotation", rec: r.reconcileAnnotation},
			{typ: "finalizer", rec: r.reconcileFinalizer},
			{typ: "replication", rec: r.reconcileReplication},
			{typ: "sentinel", rec: r.reconcileSentinel},
			{typ: "pdb", rec: r.reconcilePDB},
			{typ: "service", rec: r.reconcileService},
		}
	}

	for _, reconciler := range reconcilers {
		result, err := reconciler.rec(ctx, instance)
		if err != nil {
			return intctrlutil.RequeueWithError(ctx, err, "")
		}
		if result.Requeue {
			return result, nil
		}
	}

	// DO NOT REQUEUE.
	// only reconcile on resource(sentinel && watched redis replication) changes
	return intctrlutil.Reconciled()
}

type reconciler struct {
	typ string
	rec func(ctx context.Context, instance *redisv1beta2.RedisSentinel) (ctrl.Result, error)
}

func (r *RedisSentinelReconciler) reconcileFinalizer(ctx context.Context, instance *redisv1beta2.RedisSentinel) (ctrl.Result, error) {
	if k8sutils.IsDeleted(instance) {
		if err := k8sutils.HandleRedisSentinelFinalizer(ctx, r.Client, instance); err != nil {
			return intctrlutil.RequeueWithError(ctx, err, "")
		}
		return intctrlutil.Reconciled()
	}
	if err := k8sutils.AddFinalizer(ctx, instance, k8sutils.RedisSentinelFinalizer, r.Client); err != nil {
		return intctrlutil.RequeueWithError(ctx, err, "")
	}
	return intctrlutil.Reconciled()
}

func (r *RedisSentinelReconciler) reconcileAnnotation(ctx context.Context, instance *redisv1beta2.RedisSentinel) (ctrl.Result, error) {
	if value, found := instance.ObjectMeta.GetAnnotations()["redissentinel.opstreelabs.in/skip-reconcile"]; found && value == "true" {
		log.FromContext(ctx).Info("found skip reconcile annotation", "namespace", instance.Namespace, "name", instance.Name)
		return intctrlutil.RequeueAfter(ctx, time.Second*10, "found skip reconcile annotation")
	}
	return intctrlutil.Reconciled()
}

func (r *RedisSentinelReconciler) reconcileReplication(ctx context.Context, instance *redisv1beta2.RedisSentinel) (ctrl.Result, error) {
	if instance.Spec.RedisSentinelConfig != nil && !k8sutils.IsRedisReplicationReady(ctx, r.K8sClient, r.Dk8sClient, instance) {
		return intctrlutil.RequeueAfter(ctx, time.Second*10, "Redis Replication is specified but not ready")
	}

	if instance.Spec.RedisSentinelConfig != nil {
		r.ReplicationWatcher.Watch(
			ctx,
			types.NamespacedName{
				Namespace: instance.Namespace,
				Name:      instance.Spec.RedisSentinelConfig.RedisReplicationName,
			},
			types.NamespacedName{
				Namespace: instance.Namespace,
				Name:      instance.Name,
			},
		)
	}
	return intctrlutil.Reconciled()
}

func (r *RedisSentinelReconciler) reconcileSentinel(ctx context.Context, instance *redisv1beta2.RedisSentinel) (ctrl.Result, error) {
	if err := k8sutils.CreateRedisSentinel(ctx, r.K8sClient, instance, r.K8sClient, r.Dk8sClient); err != nil {
		return intctrlutil.RequeueWithError(ctx, err, "")
	}
	return intctrlutil.Reconciled()
}

func (r *RedisSentinelReconciler) reconcilePDB(ctx context.Context, instance *redisv1beta2.RedisSentinel) (ctrl.Result, error) {
	if err := k8sutils.ReconcileSentinelPodDisruptionBudget(ctx, instance, instance.Spec.PodDisruptionBudget, r.K8sClient); err != nil {
		return intctrlutil.RequeueWithError(ctx, err, "")
	}
	return intctrlutil.Reconciled()
}

func (r *RedisSentinelReconciler) reconcileService(ctx context.Context, instance *redisv1beta2.RedisSentinel) (ctrl.Result, error) {
	if err := k8sutils.CreateRedisSentinelService(ctx, instance, r.K8sClient); err != nil {
		return intctrlutil.RequeueWithError(ctx, err, "")
	}
	return intctrlutil.Reconciled()
}

// SetupWithManager sets up the controller with the Manager.
func (r *RedisSentinelReconciler) SetupWithManager(mgr ctrl.Manager, opts controller.Options) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&redisv1beta2.RedisSentinel{}).
		WithOptions(opts).
		Watches(&redisv1beta2.RedisReplication{}, r.ReplicationWatcher).
		Complete(r)
}
