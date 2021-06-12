/*
Copyright 2020 Opstree Solutions.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package controllers

import (
	"context"
	"strconv"
	"time"

	"github.com/go-logr/logr"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"redis-operator/k8sutils"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	redisv1beta1 "redis-operator/api/v1beta1"
)

// RedisClusterReconciler reconciles a RedisCluster object
type RedisClusterReconciler struct {
	client.Client
	Log    logr.Logger
	Scheme *runtime.Scheme
}

// Reconcile is part of the main kubernetes reconciliation loop
func (r *RedisClusterReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	reqLogger := r.Log.WithValues("Request.Namespace", req.Namespace, "Request.Name", req.Name)
	reqLogger.Info("Reconciling opstree redis Cluster controller")
	instance := &redisv1beta1.RedisCluster{}

	err := r.Client.Get(context.TODO(), req.NamespacedName, instance)
	if err != nil {
		if errors.IsNotFound(err) {
			return ctrl.Result{}, nil
		}
		return ctrl.Result{}, err
	}

	if err := controllerutil.SetControllerReference(instance, instance, r.Scheme); err != nil {
		return ctrl.Result{}, err
	}
	err = k8sutils.CreateRedisLeader(instance)
	if err != nil {
		return ctrl.Result{}, err
	}
	err = k8sutils.CreateRedisLeaderService(instance)
	if err != nil {
		return ctrl.Result{}, err
	}
	err = k8sutils.CreateRedisFollower(instance)
	if err != nil {
		return ctrl.Result{}, err
	}
	err = k8sutils.CreateRedisFollowerService(instance)
	if err != nil {
		return ctrl.Result{}, err
	}

	redisLeaderInfo, err := k8sutils.GetStateFulSet(instance.Namespace, instance.ObjectMeta.Name+"-leader")
	if err != nil {
		return ctrl.Result{}, err
	}
	redisFollowerInfo, err := k8sutils.GetStateFulSet(instance.Namespace, instance.ObjectMeta.Name+"-follower")
	if err != nil {
		return ctrl.Result{}, err
	}
	if int(redisLeaderInfo.Status.ReadyReplicas) != int(*instance.Spec.Size) && int(redisFollowerInfo.Status.ReadyReplicas) != int(*instance.Spec.Size) {
		reqLogger.Info("Redis leader and follower nodes are not ready yet", "Ready.Replicas", strconv.Itoa(int(redisLeaderInfo.Status.ReadyReplicas)), "Expected.Replicas", instance.Spec.Size)
		return ctrl.Result{RequeueAfter: time.Second * 120}, nil
	}
	reqLogger.Info("Creating redis cluster by executing cluster creation commands", "Ready.Replicas", strconv.Itoa(int(redisLeaderInfo.Status.ReadyReplicas)))
	if k8sutils.CheckRedisNodeCount(instance, "") != int(*instance.Spec.Size)*2 {
		leaderCount := k8sutils.CheckRedisNodeCount(instance, "leader")
		if leaderCount != int(*instance.Spec.Size) {
			reqLogger.Info("Not all leader are part of the cluster...", "Leaders.Count", leaderCount, "Instance.Size", instance.Spec.Size)
			k8sutils.ExecuteRedisClusterCommand(instance)
		} else {
			reqLogger.Info("All leader are part of the cluster, adding follower/replicas", "Leaders.Count", leaderCount, "Instance.Size", instance.Spec.Size)
			k8sutils.ExecuteRedisReplicationCommand(instance)
		}
	} else {
		reqLogger.Info("Redis leader count is desired")
		if k8sutils.CheckRedisClusterState(instance) >= int(*instance.Spec.Size)*2-1 {
			reqLogger.Info("Redis leader is not desired, executing failover operation")
			k8sutils.ExecuteFaioverOperation(instance)
		}
		return ctrl.Result{RequeueAfter: time.Second * 120}, nil
	}
	reqLogger.Info("Will reconcile redis cluster operator in again 10 seconds")
	return ctrl.Result{RequeueAfter: time.Second * 10}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *RedisClusterReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&redisv1beta1.RedisCluster{}).
		Complete(r)
}
