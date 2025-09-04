package k8sutils

import (
	"context"
	"reflect"

	rvb2 "github.com/OT-CONTAINER-KIT/redis-operator/api/redis/v1beta2"
	rcvb2 "github.com/OT-CONTAINER-KIT/redis-operator/api/rediscluster/v1beta2"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

// UpdateRedisClusterStatus will update the status of the RedisCluster
func UpdateRedisClusterStatus(ctx context.Context, cr *rcvb2.RedisCluster, state rcvb2.RedisClusterState, reason string, readyLeaderReplicas, readyFollowerReplicas int32, k8sClient client.Client) error {
	newStatus := rcvb2.RedisClusterStatus{
		State:                 state,
		Reason:                reason,
		ReadyLeaderReplicas:   readyLeaderReplicas,
		ReadyFollowerReplicas: readyFollowerReplicas,
	}
	if reflect.DeepEqual(cr.Status, newStatus) {
		return nil
	}
	cr.Status = newStatus

	if err := k8sClient.Status().Update(ctx, cr); err != nil {
		log.FromContext(ctx).Error(err, "Failed to update status")
		return err
	}
	return nil
}

// UpdateRedisStatus will update the status of the Redis
func UpdateRedisStatus(ctx context.Context, cr *rvb2.Redis, state rvb2.RedisState, reason string, k8sClient client.Client) error {
	newStatus := rvb2.RedisStatus{
		State:  state,
		Reason: reason,
	}
	if reflect.DeepEqual(cr.Status, newStatus) {
		return nil
	}
	cr.Status = newStatus

	if err := k8sClient.Status().Update(ctx, cr); err != nil {
		log.FromContext(ctx).Error(err, "Failed to update Redis status")
		return err
	}
	return nil
}
