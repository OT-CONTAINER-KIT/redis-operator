package controllers

import (
	"context"
	"strconv"
	"time"

	redisv1beta2 "github.com/OT-CONTAINER-KIT/redis-operator/api/v1beta2"
	"github.com/OT-CONTAINER-KIT/redis-operator/k8sutils"
	"github.com/go-logr/logr"
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

	if err = k8sutils.AddRedisReplicationFinalizer(instance, r.Client); err != nil {
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

	redisReplicationInfo, err := k8sutils.GetStatefulSet(instance.Namespace, instance.ObjectMeta.Name, r.K8sClient)
	if err != nil {
		return ctrl.Result{RequeueAfter: time.Second * 60}, err
	}

	// Check that the Leader and Follower are ready in redis replication
	if redisReplicationInfo.Status.ReadyReplicas != totalReplicas {
		reqLogger.Info("Redis replication nodes are not ready yet", "Ready.Replicas", strconv.Itoa(int(redisReplicationInfo.Status.ReadyReplicas)), "Expected.Replicas", totalReplicas)
		return ctrl.Result{RequeueAfter: time.Second * 60}, nil
	}

	if len(k8sutils.GetRedisNodesByRole(ctx, r.K8sClient, r.Log, instance, "master")) > int(leaderReplicas) {
		reqLogger.Info("Creating redis replication by executing replication creation commands", "Replication.Ready", strconv.Itoa(int(redisReplicationInfo.Status.ReadyReplicas)))
		masterNodes := k8sutils.GetRedisNodesByRole(ctx, r.K8sClient, r.Log, instance, "master")
		slaveNodes := k8sutils.GetRedisNodesByRole(ctx, r.K8sClient, r.Log, instance, "slave")
		err := k8sutils.CreateMasterSlaveReplication(ctx, r.K8sClient, r.Log, instance, masterNodes, slaveNodes)
		if err != nil {
			return ctrl.Result{RequeueAfter: time.Second * 60}, err
		}
	}
	reqLogger.Info("Will reconcile redis operator in again 10 seconds")
	return ctrl.Result{RequeueAfter: time.Second * 10}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *RedisReplicationReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&redisv1beta2.RedisReplication{}).
		Complete(r)
}
