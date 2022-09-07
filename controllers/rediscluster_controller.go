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
	"fmt"
	"k8s.io/apimachinery/pkg/types"
	"net/http"
	"os"
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
	// NOTE: retrieves redis deployment instance detail.
	// QUERY: But why not pass the ctx received in reconcile
	err := r.Client.Get(context.TODO(), req.NamespacedName, instance)
	if err != nil {
		if errors.IsNotFound(err) {
			return ctrl.Result{}, nil
		}
		return ctrl.Result{}, err
	}
	// NOTE: retrieve the expected number of leaders and followers from spec (not from cluster)
	leaderReplicas := instance.Spec.GetReplicaCounts("leader")
	followerReplicas := instance.Spec.GetReplicaCounts("follower")
	totalReplicas := leaderReplicas + followerReplicas

	// NOTE: if the redis cluster is marked to be deleted then execute deletion workflow.
	if err := k8sutils.HandleRedisClusterFinalizer(instance, r.Client); err != nil {
		return ctrl.Result{}, err
	}

	// QUERY: Add redis cluster finalizer but why ? Deletion is detected by deletion timestamp. so it can be done anyways.
	if err := k8sutils.AddRedisClusterFinalizer(instance, r.Client); err != nil {
		return ctrl.Result{}, err
	}

	// NOTE: Create a patch of stateful set definition and applies it.
	err = k8sutils.CreateRedisLeader(instance)
	if err != nil {
		return ctrl.Result{}, err
	}
	if leaderReplicas != 0 {
		// NOTE: Same. creates a patch for service and applies.
		err = k8sutils.CreateRedisLeaderService(instance)
		if err != nil {
			return ctrl.Result{}, err
		}
	}

	// NOTE: None of the clusters have PDB. So not applicable
	err = k8sutils.ReconcileRedisPodDisruptionBudget(instance, "leader", instance.Spec.RedisLeader.PodDisruptionBudget)
	if err != nil {
		return ctrl.Result{}, err
	}

	// START: Same for follower.
	err = k8sutils.CreateRedisFollower(instance)
	if err != nil {
		return ctrl.Result{}, err
	}
	// if we have followers create their service.
	if followerReplicas != 0 {
		err = k8sutils.CreateRedisFollowerService(instance)
		if err != nil {
			return ctrl.Result{}, err
		}
	}
	err = k8sutils.ReconcileRedisPodDisruptionBudget(instance, "follower", instance.Spec.RedisFollower.PodDisruptionBudget)
	if err != nil {
		return ctrl.Result{}, err
	}
	// END: Same for follower.

	redisLeaderInfo, err := k8sutils.GetStatefulSet(instance.Namespace, instance.ObjectMeta.Name+"-leader")
	if err != nil {
		return ctrl.Result{}, err
	}
	redisFollowerInfo, err := k8sutils.GetStatefulSet(instance.Namespace, instance.ObjectMeta.Name+"-follower")
	if err != nil {
		return ctrl.Result{}, err
	}

	if leaderReplicas == 0 {
		reqLogger.Info("Redis leaders Cannot be 0", "Ready.Replicas", strconv.Itoa(int(redisLeaderInfo.Status.ReadyReplicas)), "Expected.Replicas", leaderReplicas)
		return ctrl.Result{RequeueAfter: time.Second * 120}, nil
	}

	if int32(redisLeaderInfo.Status.ReadyReplicas) != leaderReplicas && int32(redisFollowerInfo.Status.ReadyReplicas) != followerReplicas {
		reqLogger.Info("Redis leader and follower nodes are not ready yet", "Ready.Replicas", strconv.Itoa(int(redisLeaderInfo.Status.ReadyReplicas)), "Expected.Replicas", leaderReplicas)
		return ctrl.Result{RequeueAfter: time.Second * 120}, nil
	}
	redisClusterNodes := k8sutils.CheckRedisNodeCount(instance, "")
	reqLogger.Info("Creating redis cluster by executing cluster creation commands",
		"Leaders.Ready", strconv.Itoa(int(redisLeaderInfo.Status.ReadyReplicas)),
		"Followers.Ready", strconv.Itoa(int(redisFollowerInfo.Status.ReadyReplicas)),
		"redisClusterNodes", redisClusterNodes)

	if redisClusterNodes != totalReplicas {
		leaderCount := k8sutils.CheckRedisNodeCount(instance, "leader")
		if leaderCount != leaderReplicas {
			reqLogger.Info("Not all leader are part of the cluster...",
				"Leaders.Count", leaderCount,
				"Instance.Size", leaderReplicas,
				"DangerouslyRecreateClusterOnError", instance.Spec.DangerouslyRecreateClusterOnError)
			err := k8sutils.ExecuteRedisClusterCommand(instance)
			if err != nil && instance.Spec.DangerouslyRecreateClusterOnError {
				reqLogger.Info("Adding Leaders failed. Executing fail-over")
				err = k8sutils.ExecuteFailoverOperation(instance)
				if err != nil {
					return ctrl.Result{RequeueAfter: time.Second * 10}, err
				}
				return ctrl.Result{RequeueAfter: time.Second * 120}, nil
			}
		} else {
			if followerReplicas > 0 {
				reqLogger.Info("All leader are part of the cluster, adding follower/replicas", "Leaders.Count", leaderCount, "Instance.Size", leaderReplicas, "Follower.Replicas", followerReplicas)
				err := k8sutils.ExecuteRedisReplicationCommand(instance)
				if err != nil && instance.Spec.DangerouslyRecreateClusterOnError {
					reqLogger.Info("Adding Leaders failed. Executing fail-over")
					err = k8sutils.ExecuteFailoverOperation(instance)
					if err != nil {
						return ctrl.Result{RequeueAfter: time.Second * 10}, err
					}
					return ctrl.Result{RequeueAfter: time.Second * 120}, nil
				}
			} else {
				reqLogger.Info("no follower/replicas configured, skipping replication configuration", "Leaders.Count", leaderCount, "Leader.Size", leaderReplicas, "Follower.Replicas", followerReplicas)
			}
		}
	} else {
		reqLogger.Info("Redis leader count is desired")
		failedNodesCount := k8sutils.CheckRedisClusterState(instance)
		executeForceClusterReset := instance.Spec.DangerouslyRecreateClusterOnError && (failedNodesCount > 0)
		reqLogger.Info("Dangerously Reset Cluster",
			"DangerouslyRecreateClusterOnError", instance.Spec.DangerouslyRecreateClusterOnError,
			"failedNodesCount", failedNodesCount)
		// PROBLEM: why failed count number has to be so large to execute failover.
		if failedNodesCount >= int(totalReplicas)-1 || executeForceClusterReset {
			reqLogger.Info("Redis leader is not desired, executing failover operation")
			err = k8sutils.ExecuteFailoverOperation(instance)
			if err != nil {
				return ctrl.Result{RequeueAfter: time.Second * 10}, err
			}
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

func (r *RedisClusterReconciler) forceRecreateCluster(w http.ResponseWriter, req *http.Request) {
	q := req.URL.Query()
	ns := q.Get("ns")
	name := q.Get("name")
	instance := &redisv1beta1.RedisCluster{}
	namespacedName := types.NamespacedName{
		Name:      name,
		Namespace: ns,
	}
	err := r.Client.Get(context.TODO(), namespacedName, instance)
	if err != nil {
		fmt.Fprintf(w, "ERROR")
	}
	k8sutils.ExecuteFailoverOperation(instance)
	fmt.Fprintf(w, "OK")
}

func (r *RedisClusterReconciler) SetupHttpCommandServer() {
	http.HandleFunc("/force-recreate", r.forceRecreateCluster)
	err := http.ListenAndServe(":8090", nil)
	if err != nil {
		os.Exit(1)
	}
}
