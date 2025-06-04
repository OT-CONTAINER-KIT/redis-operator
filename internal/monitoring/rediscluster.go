package monitoring

import (
	"github.com/prometheus/client_golang/prometheus"
)

// metricsDescription is a map of string keys (metrics) to MetricDescription values (Name, Help).
var RedisClusterDescription = map[string]MetricDescription{
	"RedisClusterSkipReconcile": {
		Name:   "rediscluster_skipreconcile",
		Help:   "Whether or not to skip the reconcile of RedisCluster.",
		Type:   "Gauge",
		labels: []string{"namespace", "instance"},
	},
	"RedisClusterHealthy": {
		Name:   "rediscluster_healthy",
		Help:   "Whether or not to check Redis Cluster Health status.",
		Type:   "Gauge",
		labels: []string{"namespace", "instance"},
	},
	"RedisClusterReplicasSizeDesired": {
		Name:   "rediscluster_replicas_size_desired",
		Help:   "Total desired number of rediscluster replicas.",
		Type:   "Gauge",
		labels: []string{"namespace", "instance"},
	},
	"RedisClusterRebalanceTotal": {
		Name:   "rediscluster_rebalance_total",
		Help:   "Total number of rediscluster rebalance operations.",
		Type:   "Counter",
		labels: []string{"namespace", "instance"},
	},
	"RedisClusterReshardTotal": {
		Name:   "rediscluster_reshard_total",
		Help:   "Total number of rediscluster reshard operations.",
		Type:   "Counter",
		labels: []string{"namespace", "instance"},
	},
	"RedisClusterRemoveFollowerAttempt": {
		Name:   "rediscluster_remove_follower_attempt",
		Help:   "Number of times to remove follower attempts.",
		Type:   "Counter",
		labels: []string{"namespace", "instance"},
	},
	"RedisClusterAddingNodeAttempt": {
		Name:   "rediscluster_adding_node_attempt",
		Help:   "Number of times to add a node to the cluster.",
		Type:   "Counter",
		labels: []string{"namespace", "instance"},
	},
}

var (
	RedisClusterSkipReconcile = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: RedisClusterDescription["RedisClusterSkipReconcile"].Name,
			Help: RedisClusterDescription["RedisClusterSkipReconcile"].Help,
		},
		RedisClusterDescription["RedisClusterSkipReconcile"].labels,
	)

	RedisClusterHealthy = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: RedisClusterDescription["RedisClusterHealthy"].Name,
			Help: RedisClusterDescription["RedisClusterHealthy"].Help,
		},
		RedisClusterDescription["RedisClusterHealthy"].labels,
	)

	RedisClusterUnhealthy = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: RedisClusterDescription["RedisClusterUnhealthy"].Name,
			Help: RedisClusterDescription["RedisClusterUnhealthy"].Help,
		},
		RedisClusterDescription["RedisClusterUnhealthy"].labels,
	)

	RedisClusterReplicasSizeDesired = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: RedisClusterDescription["RedisClusterReplicasSizeDesired"].Name,
			Help: RedisClusterDescription["RedisClusterReplicasSizeDesired"].Help,
		},
		RedisClusterDescription["RedisClusterReplicasSizeDesired"].labels,
	)

	RedisClusterRebalanceTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: RedisClusterDescription["RedisClusterRebalanceTotal"].Name,
			Help: RedisClusterDescription["RedisClusterRebalanceTotal"].Help,
		},
		RedisClusterDescription["RedisClusterRebalanceTotal"].labels,
	)

	RedisClusterReshardTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: RedisClusterDescription["RedisClusterReshardTotal"].Name,
			Help: RedisClusterDescription["RedisClusterReshardTotal"].Help,
		},
		RedisClusterDescription["RedisClusterReshardTotal"].labels,
	)

	RedisClusterRemoveFollowerAttempt = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: RedisClusterDescription["RedisClusterRemoveFollowerAttempt"].Name,
			Help: RedisClusterDescription["RedisClusterRemoveFollowerAttempt"].Help,
		},
		RedisClusterDescription["RedisClusterRemoveFollowerAttempt"].labels,
	)

	RedisClusterAddingNodeAttempt = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: RedisClusterDescription["RedisClusterAddingNodeAttempt"].Name,
			Help: RedisClusterDescription["RedisClusterAddingNodeAttempt"].Help,
		},
		RedisClusterDescription["RedisClusterAddingNodeAttempt"].labels,
	)
)

// ListMetrics will create a slice with the metrics available in metricDescription
func ListRedisClusterMetrics() []MetricDescription {
	v := make([]MetricDescription, 0, len(RedisClusterDescription))
	// Insert value (Name, Help) for each metric
	for _, value := range RedisClusterDescription {
		v = append(v, value)
	}

	return v
}
