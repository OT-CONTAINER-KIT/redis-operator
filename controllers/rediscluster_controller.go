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

	"github.com/OT-CONTAINER-KIT/redis-operator/api/status"
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

// RedisClusterReconciler reconciles a RedisCluster object
type RedisClusterReconciler struct {
	client.Client
	K8sClient  kubernetes.Interface
	Dk8sClient dynamic.Interface
	Log        logr.Logger
	Scheme     *runtime.Scheme
}

// Reconcile is part of the main kubernetes reconciliation loop
func (r *RedisClusterReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	reqLogger := r.Log.WithValues("Request.Namespace", req.Namespace, "Request.Name", req.Name)
	reqLogger.Info("Reconciling opstree redis Cluster controller")
	instance := &redisv1beta2.RedisCluster{}

	err := r.Client.Get(context.TODO(), req.NamespacedName, instance)
	if err != nil {
		if errors.IsNotFound(err) {
			return ctrl.Result{}, nil
		}
		return ctrl.Result{}, err
	}
	instance.SetDefault()

	if _, found := instance.ObjectMeta.GetAnnotations()["rediscluster.opstreelabs.in/skip-reconcile"]; found {
		reqLogger.Info("Found annotations rediscluster.opstreelabs.in/skip-reconcile, so skipping reconcile")
		return ctrl.Result{RequeueAfter: time.Second * 10}, nil
	}

	leaderReplicas := instance.Spec.GetReplicaCounts("leader")
	followerReplicas := instance.Spec.GetReplicaCounts("follower")
	totalReplicas := leaderReplicas + followerReplicas

	if err = k8sutils.HandleRedisClusterFinalizer(r.Client, r.K8sClient, r.Log, instance); err != nil {
		return ctrl.Result{}, err
	}

	if err = k8sutils.AddRedisClusterFinalizer(instance, r.Client); err != nil {
		return ctrl.Result{}, err
	}

	// Check if the cluster is downscaled
	if leaderReplicas < instance.Status.ReadyLeaderReplicas {
		reqLogger.Info("Redis cluster is downscaling...", "Ready.ReadyLeaderReplicas", instance.Status.ReadyLeaderReplicas, "Expected.ReadyLeaderReplicas", leaderReplicas)

		// loop count times to remove the latest leader/follower pod
		count := instance.Status.ReadyLeaderReplicas - leaderReplicas
		for i := int32(0); i < count; i++ {
			reqLogger.Info("Redis cluster is downscaling", "The times of loop", i)

			//  Imp if the last index of leader sts is not leader make it then
			// check whether the redis is leader or not ?
			// if not true then make it leader pod
			if !(k8sutils.VerifyLeaderPod(ctx, r.K8sClient, r.Log, instance)) {
				// lastLeaderPod is slaving right now Make it the master Pod
				// We have to bring a manual failover here to make it a leaderPod
				// clusterFailover should also include the clusterReplicate since we have to map the followers to new leader
				k8sutils.ClusterFailover(ctx, r.K8sClient, r.Log, instance)
			}
			// Step 1 Remove the Follower Node
			k8sutils.RemoveRedisFollowerNodesFromCluster(ctx, r.K8sClient, r.Log, instance)
			// Step 2 Reshard the Cluster
			k8sutils.ReshardRedisCluster(r.K8sClient, r.Log, instance, true)
		}
		reqLogger.Info("Redis cluster is downscaled... Rebalancing the cluster")
		// Step 3 Rebalance the cluster
		k8sutils.RebalanceRedisCluster(r.K8sClient, r.Log, instance)
		reqLogger.Info("Redis cluster is downscaled... Rebalancing the cluster is done")
		err = k8sutils.UpdateRedisClusterStatus(instance, status.RedisClusterReady, status.ReadyClusterReason, leaderReplicas, leaderReplicas, r.Dk8sClient)
		if err != nil {
			return ctrl.Result{}, err
		}
		return ctrl.Result{RequeueAfter: time.Second * 60}, nil
	}

	// Mark the cluster status as initializing if there are no leader or follower nodes
	if instance.Status.ReadyLeaderReplicas == 0 && instance.Status.ReadyFollowerReplicas == 0 {
		err = k8sutils.UpdateRedisClusterStatus(instance, status.RedisClusterInitializing, status.InitializingClusterLeaderReason, 0, 0, r.Dk8sClient)
		if err != nil {
			return ctrl.Result{}, err
		}
	}

	if leaderReplicas != 0 {
		err = k8sutils.CreateRedisLeaderService(instance, r.K8sClient)
		if err != nil {
			return ctrl.Result{}, err
		}
	}
	err = k8sutils.CreateRedisLeader(instance, r.K8sClient)
	if err != nil {
		return ctrl.Result{}, err
	}

	err = k8sutils.ReconcileRedisPodDisruptionBudget(instance, "leader", instance.Spec.RedisLeader.PodDisruptionBudget, r.K8sClient)
	if err != nil {
		return ctrl.Result{}, err
	}

	redisLeaderInfo, err := k8sutils.GetStatefulSet(instance.Namespace, instance.ObjectMeta.Name+"-leader", r.K8sClient)
	if err != nil {
		if errors.IsNotFound(err) {
			return ctrl.Result{RequeueAfter: time.Second * 60}, nil
		}
		return ctrl.Result{}, err
	}

	if redisLeaderInfo.Status.ReadyReplicas == leaderReplicas {
		// Mark the cluster status as initializing if there are no follower nodes
		if instance.Status.ReadyLeaderReplicas == 0 && instance.Status.ReadyFollowerReplicas == 0 {
			err = k8sutils.UpdateRedisClusterStatus(instance, status.RedisClusterInitializing, status.InitializingClusterFollowerReason, leaderReplicas, 0, r.Dk8sClient)
			if err != nil {
				return ctrl.Result{}, err
			}
		}
		// if we have followers create their service.
		if followerReplicas != 0 {
			err = k8sutils.CreateRedisFollowerService(instance, r.K8sClient)
			if err != nil {
				return ctrl.Result{}, err
			}
		}
		err = k8sutils.CreateRedisFollower(instance, r.K8sClient)
		if err != nil {
			return ctrl.Result{}, err
		}
		err = k8sutils.ReconcileRedisPodDisruptionBudget(instance, "follower", instance.Spec.RedisFollower.PodDisruptionBudget, r.K8sClient)
		if err != nil {
			return ctrl.Result{}, err
		}
	}
	redisFollowerInfo, err := k8sutils.GetStatefulSet(instance.Namespace, instance.ObjectMeta.Name+"-follower", r.K8sClient)
	if err != nil {
		if errors.IsNotFound(err) {
			return ctrl.Result{RequeueAfter: time.Second * 60}, nil
		}
		return ctrl.Result{}, err
	}

	if leaderReplicas == 0 {
		reqLogger.Info("Redis leaders Cannot be 0", "Ready.Replicas", strconv.Itoa(int(redisLeaderInfo.Status.ReadyReplicas)), "Expected.Replicas", leaderReplicas)
		return ctrl.Result{RequeueAfter: time.Second * 60}, nil
	}

	if !(redisLeaderInfo.Status.ReadyReplicas == leaderReplicas && redisFollowerInfo.Status.ReadyReplicas == followerReplicas) {
		reqLogger.Info("Redis leader and follower nodes are not ready yet", "Ready.Replicas", strconv.Itoa(int(redisLeaderInfo.Status.ReadyReplicas)), "Expected.Replicas", leaderReplicas)
		return ctrl.Result{RequeueAfter: time.Second * 60}, nil
	}

	// Mark the cluster status as bootstrapping if all the leader and follower nodes are ready
	if !(instance.Status.ReadyLeaderReplicas == leaderReplicas && instance.Status.ReadyFollowerReplicas == followerReplicas) {
		err = k8sutils.UpdateRedisClusterStatus(instance, status.RedisClusterBootstrap, status.BootstrapClusterReason, leaderReplicas, followerReplicas, r.Dk8sClient)
		if err != nil {
			return ctrl.Result{}, err
		}
	}

	if nc := k8sutils.CheckRedisNodeCount(ctx, r.K8sClient, r.Log, instance, ""); nc != totalReplicas {
		reqLogger.Info("Creating redis cluster by executing cluster creation commands", "Leaders.Ready", strconv.Itoa(int(redisLeaderInfo.Status.ReadyReplicas)), "Followers.Ready", strconv.Itoa(int(redisFollowerInfo.Status.ReadyReplicas)))
		leaderCount := k8sutils.CheckRedisNodeCount(ctx, r.K8sClient, r.Log, instance, "leader")
		if leaderCount != leaderReplicas {
			reqLogger.Info("Not all leader are part of the cluster...", "Leaders.Count", leaderCount, "Instance.Size", leaderReplicas)
			if leaderCount <= 2 {
				k8sutils.ExecuteRedisClusterCommand(ctx, r.K8sClient, r.Log, instance)
			} else {
				if leaderCount < leaderReplicas {
					// Scale up the cluster
					// Step 2 : Add Redis Node
					k8sutils.AddRedisNodeToCluster(ctx, r.K8sClient, r.Log, instance)
					// Step 3 Rebalance the cluster using the empty masters
					k8sutils.RebalanceRedisClusterEmptyMasters(r.K8sClient, r.Log, instance)
				}
			}
		} else {
			if followerReplicas > 0 && redisFollowerInfo.Status.ReadyReplicas == followerReplicas {
				reqLogger.Info("All leader are part of the cluster, adding follower/replicas", "Leaders.Count", leaderCount, "Instance.Size", leaderReplicas, "Follower.Replicas", followerReplicas)
				k8sutils.ExecuteRedisReplicationCommand(ctx, r.K8sClient, r.Log, instance)
			} else {
				reqLogger.Info("no follower/replicas configured, skipping replication configuration", "Leaders.Count", leaderCount, "Leader.Size", leaderReplicas, "Follower.Replicas", followerReplicas)
			}
		}
		reqLogger.Info("Redis cluster count is not desired", "Current.Count", nc, "Desired.Count", totalReplicas)
		return ctrl.Result{RequeueAfter: time.Second * 60}, nil
	}

	reqLogger.Info("Redis cluster count is desired")
	if int(totalReplicas) > 1 && k8sutils.CheckRedisClusterState(ctx, r.K8sClient, r.Log, instance) >= int(totalReplicas)-1 {
		reqLogger.Info("Redis leader is not desired, executing failover operation")
		err = k8sutils.ExecuteFailoverOperation(ctx, r.K8sClient, r.Log, instance)
		if err != nil {
			return ctrl.Result{}, err
		}
	}

	// Check If there is No Empty Master Node
	if k8sutils.CheckRedisNodeCount(ctx, r.K8sClient, r.Log, instance, "") == totalReplicas {
		k8sutils.CheckIfEmptyMasters(ctx, r.K8sClient, r.Log, instance)
	}
	reqLogger.Info("Will reconcile redis cluster operator in again 10 seconds")

	// Mark the cluster status as ready if all the leader and follower nodes are ready
	if instance.Status.ReadyLeaderReplicas == leaderReplicas && instance.Status.ReadyFollowerReplicas == followerReplicas {
		err = k8sutils.UpdateRedisClusterStatus(instance, status.RedisClusterReady, status.ReadyClusterReason, leaderReplicas, followerReplicas, r.Dk8sClient)
		if err != nil {
			return ctrl.Result{}, err
		}
	}

	return ctrl.Result{RequeueAfter: time.Second * 10}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *RedisClusterReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&redisv1beta2.RedisCluster{}).
		Complete(r)
}
