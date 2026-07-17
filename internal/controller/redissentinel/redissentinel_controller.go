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
	"github.com/OT-CONTAINER-KIT/redis-operator/internal/envs"
	"github.com/OT-CONTAINER-KIT/redis-operator/internal/k8sutils"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
)

const (
	RedisSentinelFinalizer = "redisSentinelFinalizer"
)

// RedisSentinelReconciler reconciles a RedisSentinel object
type RedisSentinelReconciler struct {
	client.Client
	Checker            redis.Checker
	Healer             redis.Healer
	K8sClient          kubernetes.Interface
	ReplicationWatcher *intctrlutil.ResourceWatcher
	// ConfigMapWatcher and SecretWatcher enqueue this RedisSentinel when its
	// referenced external (additional) Sentinel config ConfigMap or mounted/
	// referenced Secret (TLS, password) changes, so an edit triggers a prompt
	// rollout of the Sentinel StatefulSet.
	ConfigMapWatcher *intctrlutil.ResourceWatcher
	SecretWatcher    *intctrlutil.ResourceWatcher
}

func (r *RedisSentinelReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	instance := &rsvb2.RedisSentinel{}

	err := r.Get(ctx, req.NamespacedName, instance)
	if err != nil {
		return intctrlutil.RequeueECheck(ctx, err, "failed to get RedisSentinel instance")
	}

	if k8sutils.IsDeleted(instance) {
		if err := k8sutils.HandleRedisSentinelFinalizer(ctx, r.Client, instance, RedisSentinelFinalizer); err != nil {
			return intctrlutil.RequeueE(ctx, err, "")
		}
		return intctrlutil.Reconciled()
	}

	if common.ShouldSkipReconcile(ctx, instance) {
		return intctrlutil.Reconciled()
	}

	// Register the source watches up front so they are wired regardless of whether
	// later reconcile steps short-circuit (e.g. replication not yet ready).
	r.registerSourceWatches(ctx, instance)

	reconcilers := []reconciler{
		{typ: "finalizer", rec: r.reconcileFinalizer},
		{typ: "replication", rec: r.reconcileReplication},
		{typ: "pdb", rec: r.reconcilePDB},
		{typ: "service", rec: r.reconcileService},
		{typ: "sentinel", rec: r.reconcileSentinel},
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

	return intctrlutil.Reconciled()
}

type reconciler struct {
	typ string
	rec func(ctx context.Context, instance *rsvb2.RedisSentinel) (ctrl.Result, error)
}

func (r *RedisSentinelReconciler) reconcileFinalizer(ctx context.Context, instance *rsvb2.RedisSentinel) (ctrl.Result, error) {
	if k8sutils.IsDeleted(instance) {
		if err := k8sutils.HandleRedisSentinelFinalizer(ctx, r.Client, instance, RedisSentinelFinalizer); err != nil {
			return intctrlutil.RequeueE(ctx, err, "")
		}
		return intctrlutil.Reconciled()
	}
	if err := k8sutils.AddFinalizer(ctx, instance, RedisSentinelFinalizer, r.Client); err != nil {
		return intctrlutil.RequeueE(ctx, err, "")
	}
	return intctrlutil.Reconciled()
}

func (r *RedisSentinelReconciler) reconcileReplication(ctx context.Context, instance *rsvb2.RedisSentinel) (ctrl.Result, error) {
	if instance.Spec.RedisSentinelConfig != nil && !k8sutils.IsRedisReplicationReady(ctx, r.K8sClient, r.Client, instance) {
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
	if err := k8sutils.CreateRedisSentinel(ctx, r.K8sClient, instance, r.K8sClient, r.Client); err != nil {
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
			monitorAddr = fmt.Sprintf("%s.%s.%s.svc.%s", master.Name, common.GetHeadlessServiceNameFromPodName(master.Name), rr.Namespace, envs.GetServiceDNSDomain())
		} else {
			monitorAddr = master.Status.PodIP
		}
	}
	if err := r.Healer.SentinelMonitor(ctx, instance, monitorAddr); err != nil {
		return intctrlutil.RequeueE(ctx, err, "")
	}
	if err := r.Healer.SentinelSet(ctx, instance, monitorAddr); err != nil {
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

// registerSourceWatches registers the external (additional) Sentinel config
// ConfigMap and the referenced Secrets (TLS, password) so a change to any of them
// enqueues this RedisSentinel and rolls the Sentinel StatefulSet via the checksum
// annotation. Sentinel has no ACL configuration.
func (r *RedisSentinelReconciler) registerSourceWatches(ctx context.Context, instance *rsvb2.RedisSentinel) {
	dependent := types.NamespacedName{Namespace: instance.Namespace, Name: instance.Name}
	if r.ConfigMapWatcher != nil && instance.Spec.RedisSentinelConfig != nil && instance.Spec.RedisSentinelConfig.AdditionalSentinelConfig != nil {
		r.ConfigMapWatcher.WatchMany(ctx, dependent, *instance.Spec.RedisSentinelConfig.AdditionalSentinelConfig)
	}
	if r.SecretWatcher != nil {
		r.SecretWatcher.WatchMany(ctx, dependent, common.ReferencedSecretNames(instance.Spec.TLS, nil, instance.Spec.KubernetesConfig.ExistingPasswordSecret)...)
	}
}

// SetupWithManager sets up the controller with the Manager.
func (r *RedisSentinelReconciler) SetupWithManager(mgr ctrl.Manager, opts controller.Options) error {
	if r.ConfigMapWatcher == nil {
		r.ConfigMapWatcher = intctrlutil.NewResourceWatcher()
	}
	if r.SecretWatcher == nil {
		r.SecretWatcher = intctrlutil.NewResourceWatcher()
	}
	return ctrl.NewControllerManagedBy(mgr).
		For(&rsvb2.RedisSentinel{}).
		Owns(&appsv1.StatefulSet{}).
		WithOptions(opts).
		Watches(&rrvb2.RedisReplication{}, r.ReplicationWatcher).
		Watches(&corev1.ConfigMap{}, r.ConfigMapWatcher).
		Watches(&corev1.Secret{}, r.SecretWatcher).
		Complete(r)
}
