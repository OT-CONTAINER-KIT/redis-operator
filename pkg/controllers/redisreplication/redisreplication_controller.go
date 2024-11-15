package redisreplication

import (
	"context"
	"time"

	redisv1beta2 "github.com/OT-CONTAINER-KIT/redis-operator/api/v1beta2"
	intctrlutil "github.com/OT-CONTAINER-KIT/redis-operator/pkg/controllerutil"
	"github.com/OT-CONTAINER-KIT/redis-operator/pkg/k8sutils"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

// Reconciler reconciles a RedisReplication object
type Reconciler struct {
	client.Client
	k8sutils.Pod
	k8sutils.StatefulSet
	K8sClient  kubernetes.Interface
	Dk8sClient dynamic.Interface
	Scheme     *runtime.Scheme
}

func (r *Reconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := log.FromContext(ctx, "Request.Namespace", req.Namespace, "Request.Name", req.Name)
	instance := &redisv1beta2.RedisReplication{}

	err := r.Client.Get(context.TODO(), req.NamespacedName, instance)
	if err != nil {
		return intctrlutil.RequeueWithErrorChecking(ctx, err, "")
	}
	if instance.ObjectMeta.GetDeletionTimestamp() != nil {
		if err = k8sutils.HandleRedisReplicationFinalizer(ctx, r.Client, r.K8sClient, instance); err != nil {
			return intctrlutil.RequeueWithError(ctx, err, "")
		}
		return intctrlutil.Reconciled()
	}
	if _, found := instance.ObjectMeta.GetAnnotations()["redisreplication.opstreelabs.in/skip-reconcile"]; found {
		return intctrlutil.RequeueAfter(ctx, time.Second*10, "found skip reconcile annotation")
	}
	if err = k8sutils.AddFinalizer(ctx, instance, k8sutils.RedisReplicationFinalizer, r.Client); err != nil {
		return intctrlutil.RequeueWithError(ctx, err, "")
	}
	err = k8sutils.CreateReplicationRedis(ctx, instance, r.K8sClient)
	if err != nil {
		return intctrlutil.RequeueWithError(ctx, err, "")
	}
	err = k8sutils.CreateReplicationService(ctx, instance, r.K8sClient)
	if err != nil {
		return intctrlutil.RequeueWithError(ctx, err, "")
	}
	if !r.IsStatefulSetReady(ctx, instance.Namespace, instance.Name) {
		return intctrlutil.Reconciled()
	}

	var realMaster string
	masterNodes := k8sutils.GetRedisNodesByRole(ctx, r.K8sClient, instance, "master")
	if len(masterNodes) > 1 {
		logger.Info("Creating redis replication by executing replication creation commands")
		slaveNodes := k8sutils.GetRedisNodesByRole(ctx, r.K8sClient, instance, "slave")
		realMaster = k8sutils.GetRedisReplicationRealMaster(ctx, r.K8sClient, instance, masterNodes)
		if len(slaveNodes) == 0 {
			realMaster = masterNodes[0]
		}
		if err = k8sutils.CreateMasterSlaveReplication(ctx, r.K8sClient, instance, masterNodes, realMaster); err != nil {
			return intctrlutil.RequeueAfter(ctx, time.Second*60, "")
		}
	}
	realMaster = k8sutils.GetRedisReplicationRealMaster(ctx, r.K8sClient, instance, masterNodes)
	if err = r.UpdateRedisReplicationMaster(ctx, instance, realMaster); err != nil {
		return intctrlutil.RequeueWithError(ctx, err, "")
	}
	if err = r.UpdateRedisPodRoleLabel(ctx, instance, realMaster); err != nil {
		return intctrlutil.RequeueWithError(ctx, err, "")
	}
	return intctrlutil.RequeueAfter(ctx, time.Second*10, "")
}

func (r *Reconciler) UpdateRedisReplicationMaster(ctx context.Context, instance *redisv1beta2.RedisReplication, masterNode string) error {
	if instance.Status.MasterNode == masterNode {
		return nil
	}
	instance.Status.MasterNode = masterNode
	if err := r.Client.Status().Update(ctx, instance); err != nil {
		return err
	}
	return nil
}

func (r *Reconciler) UpdateRedisPodRoleLabel(ctx context.Context, cr *redisv1beta2.RedisReplication, masterNode string) error {
	labels := k8sutils.GetRedisReplicationLabels(cr)
	pods, err := r.ListPods(ctx, cr.GetNamespace(), labels)
	if err != nil {
		return err
	}
	updateRoleLabelFunc := func(ctx context.Context, namespace string, pod corev1.Pod, role string) error {
		if pod.Labels[k8sutils.RedisRoleLabelKey] != role {
			return r.PatchPodLabels(ctx, namespace, pod.GetName(), map[string]string{k8sutils.RedisRoleLabelKey: role})
		}
		return nil
	}
	for _, pod := range pods.Items {
		if masterNode == pod.GetName() {
			err = updateRoleLabelFunc(ctx, cr.GetNamespace(), pod, k8sutils.RedisRoleLabelMaster)
		} else {
			err = updateRoleLabelFunc(ctx, cr.GetNamespace(), pod, k8sutils.RedisRoleLabelSlave)
		}
		if err != nil {
			return err
		}
	}
	return nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *Reconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&redisv1beta2.RedisReplication{}).
		Owns(&appsv1.StatefulSet{}).
		Complete(r)
}
