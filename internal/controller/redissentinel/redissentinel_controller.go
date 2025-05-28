package redissentinel

import (
	"context"
	"time"

	rrvb2 "github.com/OT-CONTAINER-KIT/redis-operator/api/redisreplication/v1beta2"
	rsvb2 "github.com/OT-CONTAINER-KIT/redis-operator/api/redissentinel/v1beta2"
	"github.com/OT-CONTAINER-KIT/redis-operator/internal/controller/common"
	intctrlutil "github.com/OT-CONTAINER-KIT/redis-operator/internal/controllerutil"
	"github.com/OT-CONTAINER-KIT/redis-operator/internal/k8sutils"
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
	instance := &rsvb2.RedisSentinel{}

	err := r.Client.Get(ctx, req.NamespacedName, instance)
	if err != nil {
		return intctrlutil.RequeueWithErrorChecking(ctx, err, "failed to get RedisSentinel instance")
	}

	var statusResult ctrl.Result

	if k8sutils.IsDeleted(instance) {
		if err := k8sutils.HandleRedisSentinelFinalizer(ctx, r.Client, instance); err != nil {
			return intctrlutil.RequeueWithError(ctx, err, "")
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
		{typ: "status", rec: r.reconcileStatus},
	}

	for _, reconciler := range reconcilers {
		result, err := reconciler.rec(ctx, instance)
		if err != nil {
			return intctrlutil.RequeueWithError(ctx, err, "")
		}

		if reconciler.typ == "status" {
			statusResult = result
		}

		if result.Requeue {
			return result, nil
		}
	}

	if statusResult.RequeueAfter > 0 {
		return statusResult, nil
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
			return intctrlutil.RequeueWithError(ctx, err, "")
		}
		return intctrlutil.Reconciled()
	}
	if err := k8sutils.AddFinalizer(ctx, instance, k8sutils.RedisSentinelFinalizer, r.Client); err != nil {
		return intctrlutil.RequeueWithError(ctx, err, "")
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
		return intctrlutil.RequeueWithError(ctx, err, "")
	}
	return intctrlutil.Reconciled()
}

func (r *RedisSentinelReconciler) reconcilePDB(ctx context.Context, instance *rsvb2.RedisSentinel) (ctrl.Result, error) {
	if err := k8sutils.ReconcileSentinelPodDisruptionBudget(ctx, instance, instance.Spec.PodDisruptionBudget, r.K8sClient); err != nil {
		return intctrlutil.RequeueWithError(ctx, err, "")
	}
	return intctrlutil.Reconciled()
}

func (r *RedisSentinelReconciler) reconcileService(ctx context.Context, instance *rsvb2.RedisSentinel) (ctrl.Result, error) {
	if err := k8sutils.CreateRedisSentinelService(ctx, instance, r.K8sClient); err != nil {
		return intctrlutil.RequeueWithError(ctx, err, "")
	}
	return intctrlutil.Reconciled()
}

func (r *RedisSentinelReconciler) reconcileStatus(ctx context.Context, instance *rsvb2.RedisSentinel) (ctrl.Result, error) {
	// check quorum
	quorumStatus := "Unhealthy"
	quorum, err := k8sutils.SentinelCheckQuorum(ctx, r.K8sClient, instance)
	if err != nil {
		log.FromContext(ctx).Error(err, "Non-critical error during quorum check")
	}

	if quorum {
		quorumStatus = "Healthy"
	}

	// master address
	masterAddress, err := k8sutils.SentinelGetMasterAddress(ctx, r.K8sClient, instance)
	if err != nil {
		instance.Status.MasterAddress = "Unknown"
		log.FromContext(ctx).Error(err, "Failed to get master address")
	}

	if instance.Status.Quorum != quorumStatus || instance.Status.MasterAddress != masterAddress {
		instance.Status.Quorum = quorumStatus
		instance.Status.MasterAddress = masterAddress
		if err := r.Status().Update(ctx, instance); err != nil {
			return intctrlutil.RequeueWithError(ctx, err, "failed to update status")
		}
	}

	return ctrl.Result{RequeueAfter: 10 * time.Second}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *RedisSentinelReconciler) SetupWithManager(mgr ctrl.Manager, opts controller.Options) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&rsvb2.RedisSentinel{}).
		WithOptions(opts).
		Watches(&rrvb2.RedisReplication{}, r.ReplicationWatcher).
		Complete(r)
}
