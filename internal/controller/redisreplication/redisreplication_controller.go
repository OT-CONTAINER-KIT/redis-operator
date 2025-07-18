package redisreplication

import (
	"context"
	"time"

	rrvb2 "github.com/OT-CONTAINER-KIT/redis-operator/api/redisreplication/v1beta2"
	"github.com/OT-CONTAINER-KIT/redis-operator/internal/controller/common"
	redis "github.com/OT-CONTAINER-KIT/redis-operator/internal/controller/common/redis"
	intctrlutil "github.com/OT-CONTAINER-KIT/redis-operator/internal/controllerutil"
	"github.com/OT-CONTAINER-KIT/redis-operator/internal/k8sutils"
	"github.com/OT-CONTAINER-KIT/redis-operator/internal/monitoring"
	appsv1 "k8s.io/api/apps/v1"
	"k8s.io/client-go/kubernetes"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

const (
	RedisReplicationFinalizer = "redisReplicationFinalizer"
)

// Reconciler reconciles a RedisReplication object
type Reconciler struct {
	client.Client
	Healer    redis.Healer
	K8sClient kubernetes.Interface
}

func (r *Reconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	instance := &rrvb2.RedisReplication{}

	err := r.Get(ctx, req.NamespacedName, instance)
	if err != nil {
		return intctrlutil.RequeueECheck(ctx, err, "failed to get RedisReplication instance")
	}

	if k8sutils.IsDeleted(instance) {
		if err := k8sutils.HandleRedisReplicationFinalizer(ctx, r.Client, instance, RedisReplicationFinalizer); err != nil {
			return intctrlutil.RequeueE(ctx, err, "")
		}
		return intctrlutil.Reconciled()
	}

	monitoring.RedisReplicationSkipReconcile.WithLabelValues(instance.Namespace, instance.Name).Set(0)
	if common.IsSkipReconcile(ctx, instance) {
		monitoring.RedisReplicationSkipReconcile.WithLabelValues(instance.Namespace, instance.Name).Set(1)
		return intctrlutil.Reconciled()
	}

	reconcilers := []reconciler{
		{typ: "finalizer", rec: r.reconcileFinalizer},
		{typ: "statefulset", rec: r.reconcileStatefulSet},
		{typ: "service", rec: r.reconcileService},
		{typ: "poddisruptionbudget", rec: r.reconcilePDB},
		{typ: "redis", rec: r.reconcileRedis},
		{typ: "status", rec: r.reconcileStatus},
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

	return intctrlutil.RequeueAfter(ctx, time.Second*10, "")
}

func (r *Reconciler) UpdateRedisReplicationMaster(ctx context.Context, instance *rrvb2.RedisReplication, masterNode string) error {
	if masterNode == "" {
		monitoring.RedisReplicationHasMaster.WithLabelValues(instance.Namespace, instance.Name).Set(0)
	} else {
		monitoring.RedisReplicationHasMaster.WithLabelValues(instance.Namespace, instance.Name).Set(1)
	}

	if instance.Status.MasterNode == masterNode {
		return nil
	}

	if instance.Status.MasterNode != masterNode {
		monitoring.RedisReplicationMasterRoleChangesTotal.WithLabelValues(instance.Namespace, instance.Name).Inc()
		logger := log.FromContext(ctx)
		logger.Info("Updating master node",
			"previous", instance.Status.MasterNode,
			"new", masterNode)
	}
	instance.Status.MasterNode = masterNode
	if err := r.Client.Status().Update(ctx, instance); err != nil {
		return err
	}
	return nil
}

type reconciler struct {
	typ string
	rec func(ctx context.Context, instance *rrvb2.RedisReplication) (ctrl.Result, error)
}

func (r *Reconciler) reconcileFinalizer(ctx context.Context, instance *rrvb2.RedisReplication) (ctrl.Result, error) {
	if k8sutils.IsDeleted(instance) {
		if err := k8sutils.HandleRedisReplicationFinalizer(ctx, r.Client, instance, RedisReplicationFinalizer); err != nil {
			return intctrlutil.RequeueE(ctx, err, "")
		}
		return intctrlutil.Reconciled()
	}
	if err := k8sutils.AddFinalizer(ctx, instance, RedisReplicationFinalizer, r.Client); err != nil {
		return intctrlutil.RequeueE(ctx, err, "")
	}
	return intctrlutil.Reconciled()
}

func (r *Reconciler) reconcilePDB(ctx context.Context, instance *rrvb2.RedisReplication) (ctrl.Result, error) {
	if err := k8sutils.ReconcileReplicationPodDisruptionBudget(ctx, instance, instance.Spec.PodDisruptionBudget, r.K8sClient); err != nil {
		return intctrlutil.RequeueAfter(ctx, time.Second*60, "")
	}
	return intctrlutil.Reconciled()
}

func (r *Reconciler) reconcileStatefulSet(ctx context.Context, instance *rrvb2.RedisReplication) (ctrl.Result, error) {
	if err := k8sutils.CreateReplicationRedis(ctx, instance, r.K8sClient); err != nil {
		return intctrlutil.RequeueAfter(ctx, time.Second*60, "")
	}
	return intctrlutil.Reconciled()
}

func (r *Reconciler) reconcileService(ctx context.Context, instance *rrvb2.RedisReplication) (ctrl.Result, error) {
	if err := k8sutils.CreateReplicationService(ctx, instance, r.K8sClient); err != nil {
		return intctrlutil.RequeueAfter(ctx, time.Second*60, "")
	}
	return intctrlutil.Reconciled()
}

func (r *Reconciler) reconcileRedis(ctx context.Context, instance *rrvb2.RedisReplication) (ctrl.Result, error) {
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	var realMaster string
	masterNodes := k8sutils.GetRedisNodesByRole(ctx, r.K8sClient, instance, "master")
	slaveNodes := k8sutils.GetRedisNodesByRole(ctx, r.K8sClient, instance, "slave")
	if len(masterNodes) > 1 {
		log.FromContext(ctx).Info("Creating redis replication by executing replication creation commands")

		realMaster = k8sutils.GetRedisReplicationRealMaster(ctx, r.K8sClient, instance, masterNodes)
		if len(slaveNodes) == 0 {
			realMaster = masterNodes[0]
		}
		if err := k8sutils.CreateMasterSlaveReplication(ctx, r.K8sClient, instance, masterNodes, realMaster); err != nil {
			return intctrlutil.RequeueAfter(ctx, time.Second*60, "")
		}
	}

	monitoring.RedisReplicationReplicasSizeMismatch.WithLabelValues(instance.Namespace, instance.Name).Set(0)
	if instance.Spec.Size != nil && int(*instance.Spec.Size) != (len(masterNodes)+len(slaveNodes)) {
		monitoring.RedisReplicationReplicasSizeMismatch.WithLabelValues(instance.Namespace, instance.Name).Set(1)
	}

	monitoring.RedisReplicationReplicasSizeCurrent.WithLabelValues(instance.Namespace, instance.Name).Set(float64(len(masterNodes) + len(slaveNodes)))
	monitoring.RedisReplicationReplicasSizeDesired.WithLabelValues(instance.Namespace, instance.Name).Set(float64(*instance.Spec.Size))

	return intctrlutil.Reconciled()
}

// reconcileStatus update status and label.
func (r *Reconciler) reconcileStatus(ctx context.Context, instance *rrvb2.RedisReplication) (ctrl.Result, error) {
	var err error
	var realMaster string

	masterNodes := k8sutils.GetRedisNodesByRole(ctx, r.K8sClient, instance, "master")
	realMaster = k8sutils.GetRedisReplicationRealMaster(ctx, r.K8sClient, instance, masterNodes)
	if err = r.UpdateRedisReplicationMaster(ctx, instance, realMaster); err != nil {
		return intctrlutil.RequeueE(ctx, err, "")
	}
	labels := common.GetRedisLabels(instance.GetName(), common.SetupTypeReplication, "replication", instance.GetLabels())
	if err = r.Healer.UpdateRedisRoleLabel(ctx, instance.GetNamespace(), labels, instance.Spec.KubernetesConfig.ExistingPasswordSecret); err != nil {
		return intctrlutil.RequeueE(ctx, err, "")
	}

	slaveNodes := k8sutils.GetRedisNodesByRole(ctx, r.K8sClient, instance, "slave")
	if realMaster != "" {
		monitoring.RedisReplicationConnectedSlavesTotal.WithLabelValues(instance.Namespace, instance.Name).Set(float64(len(slaveNodes)))
	} else {
		monitoring.RedisReplicationConnectedSlavesTotal.WithLabelValues(instance.Namespace, instance.Name).Set(float64(0))
	}

	return intctrlutil.Reconciled()
}

// SetupWithManager sets up the controller with the Manager.
func (r *Reconciler) SetupWithManager(mgr ctrl.Manager, opts controller.Options) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&rrvb2.RedisReplication{}).
		WithOptions(opts).
		Owns(&appsv1.StatefulSet{}).
		Complete(r)
}
