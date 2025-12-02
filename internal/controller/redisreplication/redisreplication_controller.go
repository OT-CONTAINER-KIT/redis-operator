package redisreplication

import (
	"context"
	"fmt"
	"strings"
	"time"

	rrvb2 "github.com/OT-CONTAINER-KIT/redis-operator/api/redisreplication/v1beta2"
	"github.com/OT-CONTAINER-KIT/redis-operator/internal/controller/common"
	redishealer "github.com/OT-CONTAINER-KIT/redis-operator/internal/controller/common/redis"
	"github.com/OT-CONTAINER-KIT/redis-operator/internal/controller/common/service"
	"github.com/OT-CONTAINER-KIT/redis-operator/internal/controller/common/statefulset"
	intctrlutil "github.com/OT-CONTAINER-KIT/redis-operator/internal/controllerutil"
	"github.com/OT-CONTAINER-KIT/redis-operator/internal/envs"
	"github.com/OT-CONTAINER-KIT/redis-operator/internal/k8sutils"
	"github.com/OT-CONTAINER-KIT/redis-operator/internal/monitoring"
	"github.com/OT-CONTAINER-KIT/redis-operator/internal/service/redis"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

const (
	RedisReplicationFinalizer = "redisReplicationFinalizer"
	masterGroupName           = "mymaster"
)

// Reconciler reconciles a RedisReplication object
type Reconciler struct {
	client.Client
	k8sutils.StatefulSet
	Healer    redishealer.Healer
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

	if common.ShouldSkipReconcile(ctx, instance) {
		return intctrlutil.Reconciled()
	}

	reconcilers := []reconciler{
		{typ: "finalizer", rec: r.reconcileFinalizer},
		{typ: "resources", rec: r.reconcileResources},
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

	return intctrlutil.RequeueAfter(ctx, time.Second*30, "")
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
	return r.updateStatus(ctx, instance, rrvb2.RedisReplicationStatus{
		MasterNode: masterNode,
	})
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

func (r *Reconciler) reconcileResources(ctx context.Context, instance *rrvb2.RedisReplication) (ctrl.Result, error) {
	if err := k8sutils.CreateReplicationRedis(ctx, instance, r.K8sClient); err != nil {
		return intctrlutil.RequeueAfter(ctx, time.Second*60, "")
	}
	if err := k8sutils.CreateReplicationService(ctx, instance, r.K8sClient); err != nil {
		return intctrlutil.RequeueAfter(ctx, time.Second*60, "")
	}
	if err := k8sutils.ReconcileReplicationPodDisruptionBudget(ctx, instance, instance.Spec.PodDisruptionBudget, r.K8sClient); err != nil {
		return intctrlutil.RequeueAfter(ctx, time.Second*60, "")
	}
	if instance.EnableSentinel() {
		svc := newSentinelService(instance)
		_, err := service.Reconcile(ctx, r.Client, svc, instance)
		if err != nil {
			return intctrlutil.RequeueE(ctx, err, "")
		}
		sts := newSentinelStatefulSet(instance, svc.Name)
		_, err = statefulset.Reconcile(ctx, r.Client, sts, instance)
		if err != nil {
			return intctrlutil.RequeueE(ctx, err, "")
		}
	}
	return intctrlutil.Reconciled()
}

func (r *Reconciler) configureSentinel(ctx context.Context, inst *rrvb2.RedisReplication, masterPodName string) error {
	if masterPodName == "" {
		return nil
	}
	masterPod, err := r.K8sClient.CoreV1().Pods(inst.Namespace).Get(ctx, masterPodName, metav1.GetOptions{})
	if err != nil {
		return fmt.Errorf("get master pod: %w", err)
	}
	var monitorAddr string
	if inst.Spec.Sentinel.ResolveHostnames == "yes" {
		monitorAddr = fmt.Sprintf(
			"%s.%s.%s.svc.%s",
			masterPodName,
			common.GetHeadlessServiceNameFromPodName(masterPodName),
			inst.Namespace,
			envs.GetServiceDNSDomain(),
		)
	} else {
		monitorAddr = masterPod.Status.PodIP
	}

	if monitorAddr == "" {
		return fmt.Errorf("master pod IP not ready")
	}

	var masterPassword string
	if inst.Spec.KubernetesConfig.ExistingPasswordSecret != nil {
		secret, err := r.K8sClient.CoreV1().Secrets(inst.Namespace).Get(
			ctx,
			*inst.Spec.KubernetesConfig.ExistingPasswordSecret.Name,
			metav1.GetOptions{},
		)
		if err != nil {
			return fmt.Errorf("get master password secret: %w", err)
		}
		masterPassword = string(secret.Data[*inst.Spec.KubernetesConfig.ExistingPasswordSecret.Key])
	}

	sentinelPods, err := r.getSentinelPods(ctx, inst)
	if err != nil {
		return fmt.Errorf("get sentinel pods: %w", err)
	}

	if len(sentinelPods.Items) == 0 {
		return nil
	}

	redisClient := redis.NewClient()
	for _, pod := range sentinelPods.Items {
		if err := r.configureSentinelPod(ctx, redisClient, inst, pod, monitorAddr, masterPassword); err != nil {
			log.FromContext(ctx).Error(err, "failed to configure sentinel pod", "pod", pod.Name)
			continue
		}
	}

	return nil
}

func (r *Reconciler) getSentinelPods(ctx context.Context, inst *rrvb2.RedisReplication) (*corev1.PodList, error) {
	labels := common.GetRedisLabels(
		inst.SentinelStatefulSet(),
		common.SetupTypeSentinel,
		"sentinel",
		inst.GetLabels(),
	)

	var selector []string
	for k, v := range labels {
		selector = append(selector, fmt.Sprintf("%s=%s", k, v))
	}

	return r.K8sClient.CoreV1().Pods(inst.Namespace).List(ctx, metav1.ListOptions{
		LabelSelector: strings.Join(selector, ","),
	})
}

func (r *Reconciler) configureSentinelPod(
	ctx context.Context,
	redisClient redis.Client,
	inst *rrvb2.RedisReplication,
	sentinelPod corev1.Pod,
	masterAddr string,
	masterPassword string,
) error {
	var sentinelPassword string
	if inst.Spec.Sentinel.ExistingPasswordSecret != nil {
		secret, err := r.K8sClient.CoreV1().Secrets(inst.Namespace).Get(
			ctx,
			*inst.Spec.Sentinel.ExistingPasswordSecret.Name,
			metav1.GetOptions{},
		)
		if err != nil {
			return err
		}
		sentinelPassword = string(secret.Data[*inst.Spec.Sentinel.ExistingPasswordSecret.Key])
	}

	sentinelConnInfo := &redis.ConnectionInfo{
		Host:     sentinelPod.Status.PodIP,
		Port:     "26379",
		Password: sentinelPassword,
	}

	sentinelService := redisClient.Connect(sentinelConnInfo)

	masterConnInfo := &redis.ConnectionInfo{
		Host:     masterAddr,
		Port:     "6379",
		Password: masterPassword,
	}

	quorum := int(inst.Spec.Sentinel.Size/2) + 1
	if err := sentinelService.SentinelMonitor(
		ctx,
		masterConnInfo,
		masterGroupName,
		fmt.Sprintf("%d", quorum),
	); err != nil {
		return err
	}

	for k, v := range map[string]string{
		"down-after-milliseconds": inst.Spec.Sentinel.DownAfterMilliseconds,
		"parallel-syncs":          inst.Spec.Sentinel.ParallelSyncs,
		"failover-timeout":        inst.Spec.Sentinel.FailoverTimeout,
	} {
		if v == "" {
			continue
		}
		if err := sentinelService.SentinelSet(ctx, masterGroupName, k, v); err != nil {
			return err
		}
	}

	if err := r.sentinelResetIfNeed(ctx, inst, sentinelService); err != nil {
		return err
	}

	return nil
}

func (r *Reconciler) sentinelResetIfNeed(ctx context.Context, inst *rrvb2.RedisReplication, redisService redis.Service) error {
	logger := log.FromContext(ctx)

	sentinelInfo, err := redisService.GetInfoSentinel(ctx)
	if err != nil {
		return fmt.Errorf("get sentinel info: %w", err)
	}

	var masterInfo *redis.SentinelMasterInfo
	for i := range sentinelInfo.Masters {
		if sentinelInfo.Masters[i].Name == masterGroupName {
			masterInfo = &sentinelInfo.Masters[i]
			break
		}
	}

	if masterInfo == nil {
		return fmt.Errorf("master group %s not found in sentinel info", masterGroupName)
	}

	expectedSlaves := int(*inst.Spec.Size - 1)        // Total size minus 1 master
	expectedSentinels := int(inst.Spec.Sentinel.Size) // Total sentinels minus current one

	needReset := false
	if masterInfo.Slaves != expectedSlaves {
		logger.Info("Sentinel has incorrect number of slaves, reset needed",
			"expected", expectedSlaves,
			"actual", masterInfo.Slaves)
		needReset = true
	}

	if masterInfo.Sentinels != expectedSentinels {
		logger.Info("Sentinel has incorrect number of other sentinels, reset needed",
			"expected", expectedSentinels,
			"actual", masterInfo.Sentinels)
		needReset = true
	}

	if needReset {
		if err := redisService.SentinelReset(ctx, masterGroupName); err != nil {
			return fmt.Errorf("reset sentinel: %w", err)
		}
	}

	return nil
}

func (r *Reconciler) reconcileRedis(ctx context.Context, instance *rrvb2.RedisReplication) (ctrl.Result, error) {
	if instance.EnableSentinel() {
		if !r.IsStatefulSetReady(ctx, instance.Namespace, instance.SentinelStatefulSet()) {
			return intctrlutil.RequeueAfter(ctx, time.Second*30, "waiting for sentinel statefulset to be ready")
		}
		if !r.IsStatefulSetReady(ctx, instance.Namespace, instance.RedisStatefulSet()) {
			return intctrlutil.RequeueAfter(ctx, time.Second*30, "waiting for redis statefulset to be ready")
		}
	}

	var realMaster string
	masterNodes, err := k8sutils.GetRedisNodesByRole(ctx, r.K8sClient, instance, "master")
	if err != nil {
		return intctrlutil.RequeueE(ctx, err, "")
	}
	slaveNodes, err := k8sutils.GetRedisNodesByRole(ctx, r.K8sClient, instance, "slave")
	if err != nil {
		return intctrlutil.RequeueE(ctx, err, "")
	}
	if len(masterNodes) > 1 {
		log.FromContext(ctx).Info("Creating redis replication by executing replication creation commands")

		realMaster = k8sutils.GetRedisReplicationRealMaster(ctx, r.K8sClient, instance, masterNodes)
		if len(slaveNodes) == 0 {
			realMaster = masterNodes[0]
		}
		if err := k8sutils.CreateMasterSlaveReplication(ctx, r.K8sClient, instance, masterNodes, realMaster); err != nil {
			return intctrlutil.RequeueAfter(ctx, time.Second*60, "")
		}
	} else if len(masterNodes) == 1 && len(slaveNodes) > 0 {
		realMaster = masterNodes[0]
		currentRealMaster := k8sutils.GetRedisReplicationRealMaster(ctx, r.K8sClient, instance, masterNodes)

		if currentRealMaster == "" && !instance.EnableSentinel() {
			log.FromContext(ctx).Info("Detected disconnected slaves, reconfiguring replication",
				"master", realMaster, "slaves", slaveNodes)

			allPods := append(masterNodes, slaveNodes...)
			if err := k8sutils.CreateMasterSlaveReplication(ctx, r.K8sClient, instance, allPods, realMaster); err != nil {
				log.FromContext(ctx).Error(err, "Failed to reconfigure master-slave replication",
					"master", realMaster, "slaves", slaveNodes)
				return intctrlutil.RequeueAfter(ctx, time.Second*60, "")
			}
			log.FromContext(ctx).Info("Successfully reconfigured slave replication")
		}
	}

	monitoring.RedisReplicationReplicasSizeMismatch.WithLabelValues(instance.Namespace, instance.Name).Set(0)
	if instance.Spec.Size != nil && int(*instance.Spec.Size) != (len(masterNodes)+len(slaveNodes)) {
		monitoring.RedisReplicationReplicasSizeMismatch.WithLabelValues(instance.Namespace, instance.Name).Set(1)
	}

	monitoring.RedisReplicationReplicasSizeCurrent.WithLabelValues(instance.Namespace, instance.Name).Set(float64(len(masterNodes) + len(slaveNodes)))
	monitoring.RedisReplicationReplicasSizeDesired.WithLabelValues(instance.Namespace, instance.Name).Set(float64(*instance.Spec.Size))

	if instance.EnableSentinel() {
		if err := r.configureSentinel(ctx, instance, realMaster); err != nil {
			log.FromContext(ctx).Error(err, "failed to configure sentinel")
		}
	}

	return intctrlutil.Reconciled()
}

// reconcileStatus update status and label.
func (r *Reconciler) reconcileStatus(ctx context.Context, instance *rrvb2.RedisReplication) (ctrl.Result, error) {
	var err error
	var realMaster string

	masterNodes, err := k8sutils.GetRedisNodesByRole(ctx, r.K8sClient, instance, "master")
	if err != nil {
		return intctrlutil.RequeueE(ctx, err, "")
	}
	realMaster = k8sutils.GetRedisReplicationRealMaster(ctx, r.K8sClient, instance, masterNodes)
	if err = r.UpdateRedisReplicationMaster(ctx, instance, realMaster); err != nil {
		return intctrlutil.RequeueE(ctx, err, "")
	}
	labels := common.GetRedisLabels(instance.GetName(), common.SetupTypeReplication, "replication", instance.GetLabels())
	if err = r.Healer.UpdateRedisRoleLabel(ctx, instance.GetNamespace(), labels, instance.Spec.KubernetesConfig.ExistingPasswordSecret, instance.Spec.TLS); err != nil {
		return intctrlutil.RequeueE(ctx, err, "")
	}

	slaveNodes, err := k8sutils.GetRedisNodesByRole(ctx, r.K8sClient, instance, "slave")
	if err != nil {
		return intctrlutil.RequeueE(ctx, err, "")
	}
	if realMaster != "" {
		monitoring.RedisReplicationConnectedSlavesTotal.WithLabelValues(instance.Namespace, instance.Name).Set(float64(len(slaveNodes)))
	} else {
		monitoring.RedisReplicationConnectedSlavesTotal.WithLabelValues(instance.Namespace, instance.Name).Set(float64(0))
	}

	return intctrlutil.Reconciled()
}

func (r *Reconciler) updateStatus(ctx context.Context, rr *rrvb2.RedisReplication, status rrvb2.RedisReplicationStatus) error {
	copy := rr.DeepCopy()
	copy.Spec = rrvb2.RedisReplicationSpec{}
	copy.Status = status
	return common.UpdateStatus(ctx, r.Client, copy)
}

// SetupWithManager sets up the controller with the Manager.
func (r *Reconciler) SetupWithManager(mgr ctrl.Manager, opts controller.Options) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&rrvb2.RedisReplication{}).
		WithOptions(opts).
		Complete(r)
}
