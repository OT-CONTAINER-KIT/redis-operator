package monitoring

import (
	"sigs.k8s.io/controller-runtime/pkg/metrics"
)

type MetricDescription struct {
	Name   string
	Help   string
	Type   string
	labels []string
}

func RegisterRedisReplicationMetrics() {
	metrics.Registry.MustRegister(
		RedisReplicationSkipReconcile,
		RedisReplicationReplicasSizeDesired,
		RedisReplicationReplicasSizeCurrent,
		RedisReplicationReplicasSizeMismatch,
		RedisReplicationHasMaster,
		RedisReplicationMasterRoleChangesTotal,
		RedisReplicationConnectedSlavesTotal,
	)
}

func RegisterRedisClusterMetrics() {
	metrics.Registry.MustRegister(
		RedisClusterHealthy,
		RedisClusterSkipReconcile,
		RedisClusterReplicasSizeDesired,
		RedisClusterAddingNodeAttempt,
		RedisClusterRebalanceTotal,
		RedisClusterRemoveFollowerAttempt,
		RedisClusterReshardTotal,
	)
}
