package redisreplication

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	rrvb2 "github.com/OT-CONTAINER-KIT/redis-operator/api/redisreplication/v1beta2"
	"github.com/OT-CONTAINER-KIT/redis-operator/internal/controller/common"
	redis "github.com/OT-CONTAINER-KIT/redis-operator/internal/controller/common/redis"
	intctrlutil "github.com/OT-CONTAINER-KIT/redis-operator/internal/controllerutil"
	"github.com/OT-CONTAINER-KIT/redis-operator/internal/k8sutils"
	"github.com/OT-CONTAINER-KIT/redis-operator/internal/monitoring"
	goredis "github.com/redis/go-redis/v9"
	appsv1 "k8s.io/api/apps/v1"
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

func (r *Reconciler) reconcileRedis(
	ctx context.Context,
	instance *rrvb2.RedisReplication,
) (ctrl.Result, error) {

	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	//----------------------------------------------------------------------
	// 0. Resolve Redis password from Secret (pointer-safe)
	//----------------------------------------------------------------------
	secretRef := instance.Spec.KubernetesConfig.ExistingPasswordSecret
	if secretRef == nil ||
		secretRef.Name == nil || secretRef.Key == nil ||
		*secretRef.Name == "" || *secretRef.Key == "" {

		log.FromContext(ctx).Info("existingPasswordSecret name/key not set")
		return intctrlutil.RequeueAfter(ctx, 10*time.Second, "password ref missing")
	}

	secret, err := r.K8sClient.CoreV1().
		Secrets(instance.Namespace).
		Get(ctx, *secretRef.Name, metav1.GetOptions{})
	if err != nil {
		log.FromContext(ctx).Error(err, "unable to read redis password secret")
		return intctrlutil.RequeueAfter(ctx, 10*time.Second, "secret not ready")
	}

	rawPwd, ok := secret.Data[*secretRef.Key]
	if !ok {
		log.FromContext(ctx).Info("password key not present in secret", "key", *secretRef.Key)
		return intctrlutil.RequeueAfter(ctx, 10*time.Second, "key missing in secret")
	}
	password := string(rawPwd)

	//----------------------------------------------------------------------
	// 1. List all Redis pods for this instance
	//----------------------------------------------------------------------
	pods, err := r.K8sClient.CoreV1().
		Pods(instance.Namespace).
		List(ctx, metav1.ListOptions{
			LabelSelector: fmt.Sprintf("app.kubernetes.io/name=%s", instance.Name),
		})
	if err != nil {
		return intctrlutil.RequeueE(ctx, err, "list pods")
	}
	if len(pods.Items) == 0 {
		return intctrlutil.RequeueAfter(ctx, 10*time.Second, "no pods yet")
	}

	//----------------------------------------------------------------------
	// 2. Healthy topology shortcut
	//----------------------------------------------------------------------
	masters := k8sutils.GetRedisNodesByRole(ctx, r.K8sClient, instance, "master")
	slaves := k8sutils.GetRedisNodesByRole(ctx, r.K8sClient, instance, "slave")
	if len(masters) == 1 && len(slaves) > 0 {
		return intctrlutil.Reconciled()
	}

	//----------------------------------------------------------------------
	// 3. Pick best pod by master_repl_offset
	//----------------------------------------------------------------------
	var (
		bestPod    *corev1.Pod
		bestOffset int64 = -1
	)

	for i := range pods.Items {
		p := &pods.Items[i]
		if p.Status.PodIP == "" {
			continue
		}

		rdb := goredis.NewClient(&goredis.Options{
			Addr:     fmt.Sprintf("%s:6379", p.Status.PodIP),
			Password: password,
		})
		info, err := rdb.Info(ctx, "replication").Result()
		rdb.Close()
		if err != nil {
			continue
		}

		for _, line := range strings.Split(info, "\r\n") {
			if strings.HasPrefix(line, "master_repl_offset:") {
				val := strings.TrimSpace(strings.TrimPrefix(line, "master_repl_offset:"))
				if off, err := strconv.ParseInt(val, 10, 64); err == nil && off > bestOffset {
					bestOffset = off
					bestPod = p
				}
				break
			}
		}
	}

	if bestPod == nil || bestPod.Status.PodIP == "" {
		return intctrlutil.RequeueAfter(ctx, 10*time.Second, "no reachable redis pods")
	}
	masterIP := bestPod.Status.PodIP

	log.FromContext(ctx).Info("Reconfiguring replication",
		"newMaster", bestPod.Name, "offset", bestOffset)

	//----------------------------------------------------------------------
	// 4. Re-attach others as replicas
	//----------------------------------------------------------------------
	for _, p := range pods.Items {
		if p.Name == bestPod.Name || p.Status.PodIP == "" {
			continue
		}
		rdb := goredis.NewClient(&goredis.Options{
			Addr:     fmt.Sprintf("%s:6379", p.Status.PodIP),
			Password: password,
		})
		if err := rdb.SlaveOf(ctx, masterIP, "6379").Err(); err != nil {
			log.FromContext(ctx).Error(err, "SLAVEOF failed", "pod", p.Name)
		}
		rdb.Close()
	}

	// ------------------------------------------------------------------
	// 4b. Immediately correct pod labels so we don't have 2 masters
	// ------------------------------------------------------------------
	labels := common.GetRedisLabels(
		instance.GetName(),
		common.SetupTypeReplication,
		"replication",
		instance.GetLabels(),
	)

	if err := r.Healer.UpdateRedisRoleLabel(
		ctx,
		instance.Namespace,
		labels,
		instance.Spec.KubernetesConfig.ExistingPasswordSecret,
	); err != nil {

		log.FromContext(ctx).Error(err, "failed to update redis-role labels")
	}

	//----------------------------------------------------------------------
	// 5. Update CR status
	//----------------------------------------------------------------------
	if err := r.UpdateRedisReplicationMaster(ctx, instance, bestPod.Name); err != nil {
		return intctrlutil.RequeueE(ctx, err, "update master status")
	}

	return intctrlutil.RequeueAfter(ctx, 30*time.Second, "verify replication")
}

// reconcileStatus update status and label.
func (r *Reconciler) reconcileStatus(ctx context.Context, instance *rrvb2.RedisReplication) (ctrl.Result, error) {
	var err error
	var realMaster string

	masterNodes := k8sutils.GetRedisNodesByRole(ctx, r.K8sClient, instance, "master")
	realMaster = k8sutils.GetRedisReplicationRealMaster(ctx, r.K8sClient, instance, masterNodes)
	if realMaster != "" && len(masterNodes) == 1 {
		// ------------------------------------------------------------------
		// Inline: fetch master_repl_offset for the sole master pod.
		// If it is 0 (empty dataset), treat as “no valid master”.
		// ------------------------------------------------------------------
		var offset int64 = -1

		// 1.  Pull password from Secret (pointer-safe)
		if ref := instance.Spec.KubernetesConfig.ExistingPasswordSecret; ref != nil &&
			ref.Name != nil && ref.Key != nil &&
			*ref.Name != "" && *ref.Key != "" {

			if sec, err := r.K8sClient.CoreV1().
				Secrets(instance.Namespace).
				Get(ctx, *ref.Name, metav1.GetOptions{}); err == nil {

				if pwd, ok := sec.Data[*ref.Key]; ok {
					password := string(pwd)

					// 2. Get the master pod’s IP
					if pod, err := r.K8sClient.CoreV1().
						Pods(instance.Namespace).
						Get(ctx, realMaster, metav1.GetOptions{}); err == nil &&
						pod.Status.PodIP != "" {

						// 3. Query INFO replication and parse master_repl_offset
						rdb := goredis.NewClient(&goredis.Options{
							Addr:     fmt.Sprintf("%s:6379", pod.Status.PodIP),
							Password: password,
						})
						if info, err := rdb.Info(ctx, "replication").Result(); err == nil {
							for _, line := range strings.Split(info, "\r\n") {
								if strings.HasPrefix(line, "master_repl_offset:") {
									v := strings.TrimSpace(strings.TrimPrefix(line, "master_repl_offset:"))
									if o, err := strconv.ParseInt(v, 10, 64); err == nil {
										offset = o
									}
									break
								}
							}
						}
						rdb.Close()
					}
				}
			}
		}

		// 4. Empty dataset? -> act as if no master exists
		if offset == 0 {
			realMaster = ""
		}
	}
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
