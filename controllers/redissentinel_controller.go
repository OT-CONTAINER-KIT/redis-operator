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

	"redis-operator/k8sutils"

	"github.com/go-logr/logr"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	redisv1beta1 "redis-operator/api/v1beta1"
)

// RedisSentinelReconciler reconciles a RedisSentinel object
type RedisSentinelReconciler struct {
	client.Client
	Log    logr.Logger
	Scheme *runtime.Scheme
}

// Reconcile is part of the main kubernetes reconciliation loop
func (r *RedisSentinelReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	reqLogger := r.Log.WithValues("Request.Namespace", req.Namespace, "Request.Name", req.Name)
	reqLogger.Info("Reconciling opstree redis Sentinel controller")
	instance := &redisv1beta1.RedisSentinel{}

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

	err = k8sutils.CreateRedisReplica(instance)
	if err != nil {
		return ctrl.Result{}, err
	}
	if instance.Spec.RedisReplica.Replicas != nil && *instance.Spec.RedisReplica.Replicas != 0 {
		err = k8sutils.CreateRedisSentinelReplicaService(instance)
		if err != nil {
			return ctrl.Result{}, err
		}
	}
	err = k8sutils.CreateRedisSentinel(instance)
	if err != nil {
		return ctrl.Result{}, err
	}

	err = k8sutils.CreateRedisSentinelReplicaService(instance)
	if err != nil {
		return ctrl.Result{}, err
	}

	err = k8sutils.CreateRedisSentinelService(instance)
	if err != nil {
		return ctrl.Result{}, err
	}

	redisReplicaInfo, err := k8sutils.GetStateFulSet(instance.Namespace, instance.ObjectMeta.Name+"-replica")
	if err != nil {
		return ctrl.Result{}, err
	}
	redisSentinelInfo, err := k8sutils.GetStateFulSet(instance.Namespace, instance.ObjectMeta.Name+"-sentinel")
	if err != nil {
		return ctrl.Result{}, err
	}

	redisReplicas := instance.Spec.RedisReplica.Replicas
	if redisReplicas == nil {
		redisReplicas = instance.Spec.Size
	}
	sentinelReplicas := instance.Spec.RedisSentinel.Replicas
	if sentinelReplicas == nil {
		sentinelReplicas = instance.Spec.Size
	}
	totalReplicas := int(*redisReplicas) + int(*sentinelReplicas)

	if *redisReplicas == 0 {
		reqLogger.Info("Redis Instances Cannot be 0", "Ready.Replicas", strconv.Itoa(int(redisReplicaInfo.Status.ReadyReplicas)), "Expected.Replicas", instance.Spec.Size)
		return ctrl.Result{RequeueAfter: time.Second * 120}, nil
	}

	if *sentinelReplicas == 0 {
		reqLogger.Info("Redis Sentinel Cannot be 0", "Ready.Replicas", strconv.Itoa(int(redisSentinelInfo.Status.ReadyReplicas)), "Expected.Replicas", instance.Spec.Size)
		return ctrl.Result{RequeueAfter: time.Second * 120}, nil
	}

	if int(redisReplicaInfo.Status.ReadyReplicas) != int(*redisReplicas) && int(redisSentinelInfo.Status.ReadyReplicas) != int(*sentinelReplicas) {
		reqLogger.Info("Redis and Sentinel nodes are not ready yet",
			"Replicas.Ready", strconv.Itoa(int(redisReplicaInfo.Status.ReadyReplicas)),
			"Sentinel.Ready", strconv.Itoa(int(redisSentinelInfo.Status.ReadyReplicas)),
			"Expected.Replicas", totalReplicas,
		)
		return ctrl.Result{RequeueAfter: time.Second * 120}, nil
	}

	//
	// reqLogger.Info("Creating redis cluster by executing cluster creation commands", "Leaders.Ready", strconv.Itoa(int(redisReplicaInfo.Status.ReadyReplicas)), "Followers.Ready", strconv.Itoa(int(redisSentinelInfo.Status.ReadyReplicas)))
	// if k8sutils.CheckRedisNodeCount(instance, "") != int(instanceReplicas) {
	// 	leaderCount := k8sutils.CheckRedisNodeCount(instance, "leader")
	// 	if leaderCount != int(*leaderReplicas) {
	// 		reqLogger.Info("Not all leader are part of the cluster...", "Leaders.Count", leaderCount, "Instance.Size", leaderReplicas)
	// 		k8sutils.ExecuteRedisSentinelCommand(instance)
	// 	} else {
	// 		if *followerReplicas > 0 {
	// 			reqLogger.Info("All leader are part of the cluster, adding follower/replicas", "Leaders.Count", leaderCount, "Instance.Size", leaderReplicas, "Follower.Replicas", followerReplicas)
	// 			k8sutils.ExecuteRedisReplicationCommand(instance)
	// 		} else {
	// 			reqLogger.Info("no follower/replicas configured, skipping replication configuration", "Leaders.Count", leaderCount, "Leader.Size", leaderReplicas, "Follower.Replicas", followerReplicas)
	// 		}
	// 	}
	// } else {
	// 	reqLogger.Info("Redis leader count is desired")
	// 	if k8sutils.CheckRedisSentinelState(instance) >= int(totalReplicas)-1 {
	// 		reqLogger.Info("Redis leader is not desired, executing failover operation")
	// 		k8sutils.ExecuteFailoverOperation(instance)
	// 	}
	// 	return ctrl.Result{RequeueAfter: time.Second * 120}, nil
	// }

	reqLogger.Info("Will reconcile redis cluster operator in again 10 seconds")
	return ctrl.Result{RequeueAfter: time.Second * 10}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *RedisSentinelReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&redisv1beta1.RedisSentinel{}).
		Complete(r)
}
