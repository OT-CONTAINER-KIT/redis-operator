package redissentinel

import (
	"context"
	"fmt"
	"time"

	rrvb2 "github.com/OT-CONTAINER-KIT/redis-operator/api/redisreplication/v1beta2"
	rsvb2 "github.com/OT-CONTAINER-KIT/redis-operator/api/redissentinel/v1beta2"
	"github.com/OT-CONTAINER-KIT/redis-operator/internal/controller/common"
	"github.com/OT-CONTAINER-KIT/redis-operator/internal/controller/common/redis"
	intctrlutil "github.com/OT-CONTAINER-KIT/redis-operator/internal/controllerutil"
	"github.com/OT-CONTAINER-KIT/redis-operator/internal/k8sutils"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
)

// RedisSentinelReconciler reconciles a RedisSentinel object
type RedisSentinelReconciler struct {
	client.Client
	Checker            redis.Checker
	Healer             redis.Healer
	K8sClient          kubernetes.Interface
	Dk8sClient         dynamic.Interface
	ReplicationWatcher *intctrlutil.ResourceWatcher
}

func (r *RedisSentinelReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	instance := &rsvb2.RedisSentinel{}

	err := r.Client.Get(ctx, req.NamespacedName, instance)
	if err != nil {
		return intctrlutil.RequeueECheck(ctx, err, "failed to get RedisSentinel instance")
	}

	if k8sutils.IsDeleted(instance) {
		if err := k8sutils.HandleRedisSentinelFinalizer(ctx, r.Client, instance); err != nil {
			return intctrlutil.RequeueE(ctx, err, "")
		}
		return intctrlutil.Reconciled()
	}

	if common.IsSkipReconcile(ctx, instance) {
		return intctrlutil.Reconciled()
	}

	reconcilers := []reconciler{
		{typ: "finalizer", rec: r.reconcileFinalizer},
		{typ: "replication", rec: r.reconcileReplication},
		{typ: "sentinel", rec: r.reconcileSentinel},
		{typ: "pdb", rec: r.reconcilePDB},
		{typ: "service", rec: r.reconcileService},
	}

	for _, reconciler := range reconcilers {
		result, err := reconciler.rec(ctx, instance)
		if err != nil {
			return intctrlutil.RequeueE(ctx, err, "")
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
	rec func(ctx context.Context, instance *rsvb2.RedisSentinel) (ctrl.Result, error)
}

func (r *RedisSentinelReconciler) reconcileFinalizer(ctx context.Context, instance *rsvb2.RedisSentinel) (ctrl.Result, error) {
	if k8sutils.IsDeleted(instance) {
		if err := k8sutils.HandleRedisSentinelFinalizer(ctx, r.Client, instance); err != nil {
			return intctrlutil.RequeueE(ctx, err, "")
		}
		return intctrlutil.Reconciled()
	}
	if err := k8sutils.AddFinalizer(ctx, instance, k8sutils.RedisSentinelFinalizer, r.Client); err != nil {
		return intctrlutil.RequeueE(ctx, err, "")
	}
	return intctrlutil.Reconciled()
}

func (r *RedisSentinelReconciler) reconcileReplication(ctx context.Context, instance *rsvb2.RedisSentinel) (ctrl.Result, error) {
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

func (r *RedisSentinelReconciler) reconcileSentinel(ctx context.Context, instance *rsvb2.RedisSentinel) (ctrl.Result, error) {
	if err := k8sutils.CreateRedisSentinel(ctx, r.K8sClient, instance, r.K8sClient, r.Dk8sClient); err != nil {
		return intctrlutil.RequeueE(ctx, err, "")
	}

	if instance.Spec.RedisSentinelConfig == nil {
		return intctrlutil.Reconciled()
	}

	rr := &rrvb2.RedisReplication{}
	if err := r.Get(ctx, types.NamespacedName{
		Namespace: instance.Namespace,
		Name:      instance.Spec.RedisSentinelConfig.RedisReplicationName,
	}, rr); err != nil {
		return intctrlutil.RequeueE(ctx, err, "")
	}

	var monitorAddr string
	if master, err := r.Checker.GetMasterFromReplication(ctx, rr); err != nil {
		return intctrlutil.RequeueE(ctx, err, "")
	} else {
		if instance.Spec.RedisSentinelConfig.ResolveHostnames == "yes" {
			monitorAddr = fmt.Sprintf("%s.%s-headless.%s.svc", master.Name, rr.Name, rr.Namespace)
		} else {
			monitorAddr = master.Status.PodIP
		}
	}

	if err := r.Healer.SentinelMonitor(ctx, instance, monitorAddr); err != nil {
		return intctrlutil.RequeueE(ctx, err, "")
	}
	if err := r.Healer.SentinelReset(ctx, instance); err != nil {
		return intctrlutil.RequeueE(ctx, err, "")
	}

	return intctrlutil.Reconciled()
}

func (r *RedisSentinelReconciler) reconcilePDB(ctx context.Context, instance *rsvb2.RedisSentinel) (ctrl.Result, error) {
	if err := k8sutils.ReconcileSentinelPodDisruptionBudget(ctx, instance, instance.Spec.PodDisruptionBudget, r.K8sClient); err != nil {
		return intctrlutil.RequeueE(ctx, err, "")
	}
	return intctrlutil.Reconciled()
}

func (r *RedisSentinelReconciler) reconcileService(ctx context.Context, instance *rsvb2.RedisSentinel) (ctrl.Result, error) {
	if err := k8sutils.CreateRedisSentinelService(ctx, instance, r.K8sClient); err != nil {
		return intctrlutil.RequeueE(ctx, err, "")
	}
	return intctrlutil.Reconciled()
}

// SetupWithManager sets up the controller with the Manager.
func (r *RedisSentinelReconciler) SetupWithManager(mgr ctrl.Manager, opts controller.Options) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&rsvb2.RedisSentinel{}).
		WithOptions(opts).
		Watches(&rrvb2.RedisReplication{}, r.ReplicationWatcher).
		Complete(r)
}
