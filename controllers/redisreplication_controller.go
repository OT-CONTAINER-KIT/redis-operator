package controllers

import (
	"context"
	"strconv"
	"time"

	"redis-operator/k8sutils"

	"github.com/go-logr/logr"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	redisv1beta1 "redis-operator/api/v1beta1"
)

// RedisClusterReconciler reconciles a RedisCluster object
type RedisReplicationReconciler struct {
	client.Client
	Log    logr.Logger
	Scheme *runtime.Scheme
}

func (r *RedisReplicationReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {

	reqLogger := r.Log.WithValues("Request.Namespace", req.Namespace, "Request.Name", req.Name)
	reqLogger.Info("Reconciling opstree redis Cluster controller")
	instance := &redisv1beta1.RedisReplication{}

	err := r.Client.Get(context.TODO(), req.NamespacedName, instance)
	if err != nil {
		if errors.IsNotFound(err) {
			return ctrl.Result{}, nil
		}
		return ctrl.Result{}, err
	}

	if _, found := instance.ObjectMeta.GetAnnotations()["rediscluster.opstreelabs.in/skip-reconcile"]; found {
		reqLogger.Info("Found annotations rediscluster.opstreelabs.in/skip-reconcile, so skipping reconcile")
		return ctrl.Result{RequeueAfter: time.Second * 10}, nil
	}

	leaderReplicas := int32(1)
	followerReplicas := instance.Spec.GetReplicationCounts("replication") - leaderReplicas
	totalReplicas := leaderReplicas + followerReplicas

	if err := k8sutils.HandleRedisReplicationFinalizer(instance, r.Client); err != nil {
		return ctrl.Result{}, err
	}

	if err := k8sutils.AddRedisReplicationFinalizer(instance, r.Client); err != nil {
		return ctrl.Result{}, err
	}

	err = k8sutils.CreateReplicationRedis(instance)
	if err != nil {
		return ctrl.Result{}, err
	}
	err = k8sutils.CreateReplicationService(instance)
	if err != nil {
		return ctrl.Result{}, err
	}

	// Set Pod distruptiuon Budget Later

	redisReplicationInfo, err := k8sutils.GetStatefulSet(instance.Namespace, instance.ObjectMeta.Name+"-replication")
	if err != nil {
		return ctrl.Result{RequeueAfter: time.Second * 60}, err
	}

	// Check that the Leader and Follower are ready
	if int32(redisReplicationInfo.Status.ReadyReplicas) != totalReplicas {
		reqLogger.Info("Redis leader and follower nodes are not ready yet", "Ready.Replicas", strconv.Itoa(int(redisReplicationInfo.Status.ReadyReplicas)), "Expected.Replicas", totalReplicas)
		return ctrl.Result{RequeueAfter: time.Second * 120}, nil
	}

	reqLogger.Info("Creating redis cluster by executing cluster creation commands", "Leaders.Ready", strconv.Itoa(int(redisLeaderInfo.Status.ReadyReplicas)), "Followers.Ready", strconv.Itoa(int(redisFollowerInfo.Status.ReadyReplicas)))

	reqLogger.Info("Will reconcile redis operator in again 10 seconds")
	return ctrl.Result{RequeueAfter: time.Second * 10}, nil

}

// SetupWithManager sets up the controller with the Manager.
func (r *RedisReplicationReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&redisv1beta1.RedisReplication{}).
		Complete(r)
}
