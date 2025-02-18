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

package rediscluster

import (
	"context"
	"fmt"
	"time"

	"github.com/OT-CONTAINER-KIT/redis-operator/api/status"
	redisv1beta2 "github.com/OT-CONTAINER-KIT/redis-operator/api/v1beta2"
	"github.com/OT-CONTAINER-KIT/redis-operator/pkg/controllers/common/events"
	intctrlutil "github.com/OT-CONTAINER-KIT/redis-operator/pkg/controllerutil"
	"github.com/OT-CONTAINER-KIT/redis-operator/pkg/k8sutils"
	retry "github.com/avast/retry-go"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/record"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

// Reconciler reconciles a RedisCluster object
type Reconciler struct {
	client.Client
	k8sutils.StatefulSet
	K8sClient  kubernetes.Interface
	Dk8sClient dynamic.Interface
	Recorder   record.EventRecorder
}

func (r *Reconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := log.FromContext(ctx)
	instance := &redisv1beta2.RedisCluster{}

	err := r.Client.Get(context.TODO(), req.NamespacedName, instance)
	if err != nil {
		return intctrlutil.RequeueWithErrorChecking(ctx, err, "failed to get redis cluster instance")
	}
	if instance.ObjectMeta.GetDeletionTimestamp() != nil {
		if err = k8sutils.HandleRedisClusterFinalizer(ctx, r.Client, instance); err != nil {
			return intctrlutil.RequeueWithError(ctx, err, "failed to handle redis cluster finalizer")
		}
		return intctrlutil.Reconciled()
	}
	if value, found := instance.ObjectMeta.GetAnnotations()["rediscluster.opstreelabs.in/skip-reconcile"]; found && value == "true" {
		log.FromContext(ctx).Info("found skip reconcile annotation", "namespace", instance.Namespace, "name", instance.Name)
		return intctrlutil.RequeueAfter(ctx, time.Second*10, "found skip reconcile annotation")
	}
	instance.SetDefault()

	leaderReplicas := instance.Spec.GetReplicaCounts("leader")
	followerReplicas := instance.Spec.GetReplicaCounts("follower")
	totalReplicas := leaderReplicas + followerReplicas

	if err = k8sutils.AddFinalizer(ctx, instance, k8sutils.RedisClusterFinalizer, r.Client); err != nil {
		return intctrlutil.RequeueWithError(ctx, err, "failed to add finalizer")
	}

	// Check if the cluster is downscaled
	if leaderCount := r.GetStatefulSetReplicas(ctx, instance.Namespace, instance.Name+"-leader"); leaderReplicas < leaderCount {
		if !(r.IsStatefulSetReady(ctx, instance.Namespace, instance.Name+"-leader") && r.IsStatefulSetReady(ctx, instance.Namespace, instance.Name+"-follower")) {
			return intctrlutil.Reconciled()
		}
		if masterCount := k8sutils.CheckRedisNodeCount(ctx, r.K8sClient, instance, "leader"); masterCount == leaderCount {
			r.Recorder.Event(instance, corev1.EventTypeNormal, events.EventReasonRedisClusterDownscale, "Redis cluster is downscaling...")
			logger.Info("Redis cluster is downscaling...", "Current.LeaderReplicas", leaderCount, "Desired.LeaderReplicas", leaderReplicas)
			for shardIdx := leaderCount - 1; shardIdx >= leaderReplicas; shardIdx-- {
				logger.Info("Remove the shard", "Shard.Index", shardIdx)
				//  Imp if the last index of leader sts is not leader make it then
				// check whether the redis is leader or not ?
				// if not true then make it leader pod
				if !(k8sutils.VerifyLeaderPod(ctx, r.K8sClient, instance, shardIdx)) {
					// lastLeaderPod is slaving right now Make it the master Pod
					// We have to bring a manual failover here to make it a leaderPod
					// clusterFailover should also include the clusterReplicate since we have to map the followers to new leader
					logger.Info("Cluster Failover is initiated", "Shard.Index", shardIdx)
					if err = k8sutils.ClusterFailover(ctx, r.K8sClient, instance, shardIdx); err != nil {
						logger.Error(err, "Failed to initiate cluster failover")
						return intctrlutil.RequeueWithError(ctx, err, "")
					}
				}
				// Step 1 Remove the Follower Node
				k8sutils.RemoveRedisFollowerNodesFromCluster(ctx, r.K8sClient, instance, shardIdx)
				// Step 2 Reshard the Cluster
				k8sutils.ReshardRedisCluster(ctx, r.K8sClient, instance, shardIdx, true)
			}
			logger.Info("Redis cluster is downscaled... Rebalancing the cluster")
			// Step 3 Rebalance the cluster
			k8sutils.RebalanceRedisCluster(ctx, r.K8sClient, instance)
			logger.Info("Redis cluster is downscaled... Rebalancing the cluster is done")
			return intctrlutil.RequeueAfter(ctx, time.Second*10, "")
		} else {
			logger.Info("masterCount is not equal to leader statefulset replicas,skip downscale", "masterCount", masterCount, "leaderReplicas", leaderReplicas)
		}
	}

	// Mark the cluster status as initializing if there are no leader or follower nodes
	if (instance.Status.ReadyLeaderReplicas == 0 && instance.Status.ReadyFollowerReplicas == 0) ||
		instance.Status.ReadyLeaderReplicas != leaderReplicas {
		err = k8sutils.UpdateRedisClusterStatus(ctx, instance, status.RedisClusterInitializing, status.InitializingClusterLeaderReason, instance.Status.ReadyLeaderReplicas, instance.Status.ReadyFollowerReplicas, r.Dk8sClient)
		if err != nil {
			return intctrlutil.RequeueWithError(ctx, err, "")
		}
	}

	if leaderReplicas != 0 {
		err = k8sutils.CreateRedisLeaderService(ctx, instance, r.K8sClient)
		if err != nil {
			return intctrlutil.RequeueWithError(ctx, err, "")
		}
	}
	err = k8sutils.CreateRedisLeader(ctx, instance, r.K8sClient)
	if err != nil {
		return intctrlutil.RequeueWithError(ctx, err, "")
	}
	err = k8sutils.ReconcileRedisPodDisruptionBudget(ctx, instance, "leader", instance.Spec.RedisLeader.PodDisruptionBudget, r.K8sClient)
	if err != nil {
		return intctrlutil.RequeueWithError(ctx, err, "")
	}

	if r.IsStatefulSetReady(ctx, instance.Namespace, instance.Name+"-leader") {
		// Mark the cluster status as initializing if there are no follower nodes
		if (instance.Status.ReadyLeaderReplicas == 0 && instance.Status.ReadyFollowerReplicas == 0) ||
			instance.Status.ReadyFollowerReplicas != followerReplicas {
			err = k8sutils.UpdateRedisClusterStatus(ctx, instance, status.RedisClusterInitializing, status.InitializingClusterFollowerReason, leaderReplicas, instance.Status.ReadyFollowerReplicas, r.Dk8sClient)
			if err != nil {
				return intctrlutil.RequeueWithError(ctx, err, "")
			}
		}
		// if we have followers create their service.
		if followerReplicas != 0 {
			err = k8sutils.CreateRedisFollowerService(ctx, instance, r.K8sClient)
			if err != nil {
				return intctrlutil.RequeueWithError(ctx, err, "")
			}
		}
		err = k8sutils.CreateRedisFollower(ctx, instance, r.K8sClient)
		if err != nil {
			return intctrlutil.RequeueWithError(ctx, err, "")
		}
		err = k8sutils.ReconcileRedisPodDisruptionBudget(ctx, instance, "follower", instance.Spec.RedisFollower.PodDisruptionBudget, r.K8sClient)
		if err != nil {
			return intctrlutil.RequeueWithError(ctx, err, "")
		}
	}

	if !(r.IsStatefulSetReady(ctx, instance.Namespace, instance.Name+"-leader") && r.IsStatefulSetReady(ctx, instance.Namespace, instance.Name+"-follower")) {
		return intctrlutil.Reconciled()
	}

	// Mark the cluster status as bootstrapping if all the leader and follower nodes are ready
	if !(instance.Status.ReadyLeaderReplicas == leaderReplicas && instance.Status.ReadyFollowerReplicas == followerReplicas) {
		err = k8sutils.UpdateRedisClusterStatus(ctx, instance, status.RedisClusterBootstrap, status.BootstrapClusterReason, leaderReplicas, followerReplicas, r.Dk8sClient)
		if err != nil {
			return intctrlutil.RequeueWithError(ctx, err, "")
		}
	}

	if nc := k8sutils.CheckRedisNodeCount(ctx, r.K8sClient, instance, ""); nc != totalReplicas {
		logger.Info("Creating redis cluster by executing cluster creation commands")
		leaderCount := k8sutils.CheckRedisNodeCount(ctx, r.K8sClient, instance, "leader")
		if leaderCount != leaderReplicas {
			logger.Info("Not all leader are part of the cluster...", "Leaders.Count", leaderCount, "Instance.Size", leaderReplicas)
			if leaderCount <= 2 {
				k8sutils.ExecuteRedisClusterCommand(ctx, r.K8sClient, instance)
			} else {
				if leaderCount < leaderReplicas {
					// Scale up the cluster
					// Step 2 : Add Redis Node
					k8sutils.AddRedisNodeToCluster(ctx, r.K8sClient, instance)
					// Step 3 Rebalance the cluster using the empty masters
					k8sutils.RebalanceRedisClusterEmptyMasters(ctx, r.K8sClient, instance)
				}
			}
		} else {
			if followerReplicas > 0 {
				logger.Info("All leader are part of the cluster, adding follower/replicas", "Leaders.Count", leaderCount, "Instance.Size", leaderReplicas, "Follower.Replicas", followerReplicas)
				k8sutils.ExecuteRedisReplicationCommand(ctx, r.K8sClient, instance)
			} else {
				logger.Info("no follower/replicas configured, skipping replication configuration", "Leaders.Count", leaderCount, "Leader.Size", leaderReplicas, "Follower.Replicas", followerReplicas)
			}
		}
		return intctrlutil.RequeueAfter(ctx, time.Second*60, "Redis cluster count is not desired", "Current.Count", nc, "Desired.Count", totalReplicas)
	}

	logger.Info("Number of Redis nodes match desired")
	unhealthyNodeCount, err := k8sutils.UnhealthyNodesInCluster(ctx, r.K8sClient, instance)
	if err != nil {
		logger.Error(err, "failed to determine unhealthy node count in cluster")
	}
	if int(totalReplicas) > 1 && unhealthyNodeCount >= int(totalReplicas)-1 {
		err = k8sutils.UpdateRedisClusterStatus(ctx, instance, status.RedisClusterFailed, "RedisCluster has too many unhealthy nodes", leaderReplicas, followerReplicas, r.Dk8sClient)
		if err != nil {
			return intctrlutil.RequeueWithError(ctx, err, "")
		}

		logger.Info("healthy leader count does not match desired; attempting to repair disconnected masters")
		if err = k8sutils.RepairDisconnectedMasters(ctx, r.K8sClient, instance); err != nil {
			logger.Error(err, "failed to repair disconnected masters")
		}

		err = retry.Do(func() error {
			nc, nErr := k8sutils.UnhealthyNodesInCluster(ctx, r.K8sClient, instance)
			if nErr != nil {
				return nErr
			}
			if nc == 0 {
				return nil
			}
			return fmt.Errorf("%d unhealthy nodes", nc)
		}, retry.Attempts(3), retry.Delay(time.Second*5))

		if err == nil {
			logger.Info("repairing unhealthy masters successful, no unhealthy masters left")
			return intctrlutil.RequeueAfter(ctx, time.Second*30, "no unhealthy nodes found after repairing disconnected masters")
		}
		logger.Info("unhealthy nodes exist after attempting to repair disconnected masters; starting failover")
		if err = k8sutils.ExecuteFailoverOperation(ctx, r.K8sClient, instance); err != nil {
			return intctrlutil.RequeueWithError(ctx, err, "")
		}
	}

	// Check If there is No Empty Master Node
	if k8sutils.CheckRedisNodeCount(ctx, r.K8sClient, instance, "") == totalReplicas {
		k8sutils.CheckIfEmptyMasters(ctx, r.K8sClient, instance)
	}

	// Mark the cluster status as ready if all the leader and follower nodes are ready
	if instance.Status.ReadyLeaderReplicas == leaderReplicas && instance.Status.ReadyFollowerReplicas == followerReplicas {
		if k8sutils.RedisClusterStatusHealth(ctx, r.K8sClient, instance) {
			// Apply dynamic config to all Redis instances in the cluster
			if err = k8sutils.SetRedisClusterDynamicConfig(ctx, r.K8sClient, instance); err != nil {
				logger.Error(err, "Failed to set dynamic config")
				return intctrlutil.RequeueWithError(ctx, err, "failed to set dynamic config")
			}

			err = k8sutils.UpdateRedisClusterStatus(ctx, instance, status.RedisClusterReady, status.ReadyClusterReason, leaderReplicas, followerReplicas, r.Dk8sClient)
			if err != nil {
				return intctrlutil.RequeueWithError(ctx, err, "")
			}
		}
	}
	return intctrlutil.RequeueAfter(ctx, time.Second*10, "")
}

// SetupWithManager sets up the controller with the Manager.
func (r *Reconciler) SetupWithManager(mgr ctrl.Manager, opts controller.Options) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&redisv1beta2.RedisCluster{}).
		WithOptions(opts).
		Owns(&appsv1.StatefulSet{}).
		Complete(r)
}
