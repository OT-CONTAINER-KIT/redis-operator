package controllers

import (
	"context"
	"strconv"
	"time"

	"github.com/go-logr/logr"
	redisv1beta2 "github.com/teocns/redis-operator/api/v1beta2"
	"github.com/teocns/redis-operator/k8sutils"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// RedisReplicationReconciler reconciles a RedisReplication object
type RedisReplicationReconciler struct {
	client.Client
	K8sClient  kubernetes.Interface
	Dk8sClient dynamic.Interface
	Log        logr.Logger
	Scheme     *runtime.Scheme
}

func (r *RedisReplicationReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	reqLogger := r.Log.WithValues("Request.Namespace", req.Namespace, "Request.Name", req.Name)
	reqLogger.Info("Reconciling opstree redis replication controller")
	instance := &redisv1beta2.RedisReplication{}

	err := r.Client.Get(context.TODO(), req.NamespacedName, instance)
	if err != nil {
		if errors.IsNotFound(err) {
			return ctrl.Result{}, nil
		}
		return ctrl.Result{}, err
	}

	if _, found := instance.ObjectMeta.GetAnnotations()["redisreplication.opstreelabs.in/skip-reconcile"]; found {
		reqLogger.Info("Found annotations redisreplication.opstreelabs.in/skip-reconcile, so skipping reconcile")
		return ctrl.Result{RequeueAfter: time.Second * 10}, nil
	}

	leaderReplicas := int32(1)
	followerReplicas := instance.Spec.GetReplicationCounts("replication") - leaderReplicas
	totalReplicas := leaderReplicas + followerReplicas

	if err = k8sutils.HandleRedisReplicationFinalizer(r.Client, r.K8sClient, r.Log, instance); err != nil {
		return ctrl.Result{}, err
	}

	if err = k8sutils.AddFinalizer(instance, k8sutils.RedisReplicationFinalizer, r.Client); err != nil {
		return ctrl.Result{}, err
	}

	err = k8sutils.CreateReplicationRedis(instance, r.K8sClient)
	if err != nil {
		return ctrl.Result{}, err
	}
	err = k8sutils.CreateReplicationService(instance, r.K8sClient)
	if err != nil {
		return ctrl.Result{}, err
	}

	// Set Pod distruptiuon Budget Later

	redisReplicationInfo, err := k8sutils.GetStatefulSet(r.K8sClient, r.Log, instance.GetNamespace(), instance.GetName())
	if err != nil {
		return ctrl.Result{RequeueAfter: time.Second * 60}, err
	}

	// Check that the Leader and Follower are ready in redis replication
	if redisReplicationInfo.Status.ReadyReplicas != totalReplicas {
		reqLogger.Info("Redis replication nodes are not ready yet", "Ready.Replicas", strconv.Itoa(int(redisReplicationInfo.Status.ReadyReplicas)), "Expected.Replicas", totalReplicas)
		return ctrl.Result{RequeueAfter: time.Second * 60}, nil
	}

	var realMaster string
	masterNodes := k8sutils.GetRedisNodesByRole(ctx, r.K8sClient, r.Log, instance, "master")
	if len(masterNodes) > int(leaderReplicas) {
		reqLogger.Info("Creating redis replication by executing replication creation commands", "Replication.Ready", strconv.Itoa(int(redisReplicationInfo.Status.ReadyReplicas)))
		slaveNodes := k8sutils.GetRedisNodesByRole(ctx, r.K8sClient, r.Log, instance, "slave")
		realMaster = k8sutils.GetRedisReplicationRealMaster(ctx, r.K8sClient, r.Log, instance, masterNodes)
		if len(slaveNodes) == 0 {
			realMaster = masterNodes[0]
		}
		err := k8sutils.CreateMasterSlaveReplication(ctx, r.K8sClient, r.Log, instance, masterNodes, realMaster)
		if err != nil {
			return ctrl.Result{RequeueAfter: time.Second * 60}, err
		}
	}
	realMaster = k8sutils.GetRedisReplicationRealMaster(ctx, r.K8sClient, r.Log, instance, masterNodes)
	if err := r.UpdateRedisReplicationMaster(ctx, instance, realMaster); err != nil {
		return ctrl.Result{}, err
	}
	reqLogger.Info("Will reconcile redis operator in again 10 seconds")
	return ctrl.Result{RequeueAfter: time.Second * 10}, nil
}

func (r *RedisReplicationReconciler) UpdateRedisReplicationMaster(ctx context.Context, instance *redisv1beta2.RedisReplication, masterNode string) error {
	if instance.Status.MasterNode == masterNode {
		return nil
	}
	instance.Status.MasterNode = masterNode
	if err := r.Client.Status().Update(ctx, instance); err != nil {
		return err
	}
	return nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *RedisReplicationReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&redisv1beta2.RedisReplication{}).
		Complete(r)
}
