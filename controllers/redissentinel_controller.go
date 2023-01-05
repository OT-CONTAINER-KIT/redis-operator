package controllers

import (
	"context"
	"redis-operator/k8sutils"
	"strconv"
	"time"

	"github.com/go-logr/logr"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	redisv1beta1 "redis-operator/api/v1beta1"
)

// RedisSentinelReconciler reconciles a RedisSentinel object
type RedisSentinelReconciler struct {
	client.Client
	Log    logr.Logger
	Scheme *runtime.Scheme
}

// Reconcile is part of the main kubernetes reconciliation loop which aims
func (r *RedisSentinelReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	reqLogger := r.Log.WithValues("Request.Namespace", req.Namespace, "Request.Name", req.Name)
	reqLogger.Info("Reconciling opstree redis controller")
	instance := &redisv1beta1.RedisSentinel{}

	err := r.Client.Get(context.TODO(), req.NamespacedName, instance)
	if err != nil {
		if errors.IsNotFound(err) {
			return ctrl.Result{}, nil
		}
		return ctrl.Result{}, err
	}

	sentinelReplicas := instance.Spec.GetSentinelCounts("sentinel")

	if err := k8sutils.HandleRedisSentinelFinalizer(instance, r.Client); err != nil {
		return ctrl.Result{RequeueAfter: time.Second * 60}, err
	}

	if err := k8sutils.AddRedisSentinelFinalizer(instance, r.Client); err != nil {
		return ctrl.Result{RequeueAfter: time.Second * 60}, err
	}

	// Create Redis Sentinel
	err = k8sutils.CreateRedisSentinel(instance)
	if err != nil {
		return ctrl.Result{}, err
	}

	// Create the Service for Redis Sentinel
	err = k8sutils.CreateRedisSentinelService(instance)
	if err != nil {
		return ctrl.Result{}, err
	}

	redisSentinelInfo, err := k8sutils.GetStatefulSet(instance.Namespace, instance.ObjectMeta.Name+"-sentinel")
	if err != nil {
		return ctrl.Result{RequeueAfter: time.Second * 60}, err
	}

	if int32(redisSentinelInfo.Status.ReadyReplicas) != sentinelReplicas {
		reqLogger.Info("Redis sentinel nodes are not ready yet", "Ready.Replicas", strconv.Itoa(int(redisSentinelInfo.Status.ReadyReplicas)), "Expected.Replicas", sentinelReplicas)
		return ctrl.Result{RequeueAfter: time.Second * 120}, nil
	}

	reqLogger.Info("Creating sentinel cluster by executing sentinel cluster creation commands", "Sentinel.Ready", strconv.Itoa(int(redisSentinelInfo.Status.ReadyReplicas)))

	// Statefulset is the Leader that need to be watched
	var statefulsetName = instance.Spec.RedisSnt.RedisClusterName

	leaderSTS, err := k8sutils.GetStatefulSet("default", statefulsetName)
	if err != nil {
		reqLogger.Error(err, " The leader statefulset does not exist ")
		return ctrl.Result{RequeueAfter: time.Second * 30}, err
	}

	k8sutils.ExecuteSenitnelCommand(instance, leaderSTS)

	reqLogger.Info("Will reconcile redis operator in again 10 seconds")
	return ctrl.Result{RequeueAfter: time.Second * 10}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *RedisSentinelReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&redisv1beta1.RedisSentinel{}).
		Complete(r)
}
