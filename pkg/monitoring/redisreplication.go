package monitoring

import (
	"github.com/prometheus/client_golang/prometheus"
	"sigs.k8s.io/controller-runtime/pkg/metrics"
)

type MetricDescription struct {
	Name   string
	Help   string
	Type   string
	labels []string
}

// metricsDescription is a map of string keys (metrics) to MetricDescription values (Name, Help).
var metricDescription = map[string]MetricDescription{
	"RedisReplicationReplicasSizeDesired": {
		Name:   "redisreplication_replicas_size_desired",
		Help:   "Total desired number of redisreplication replicas.",
		Type:   "Gauge",
		labels: []string{"namespace", "instance"},
	},
	"RedisReplicationReplicasSizeMismatch": {
		Name:   "redisreplication_replicas_size_mismatch",
		Help:   "Total number of times the redisreplication size mismatches.",
		Type:   "Gauge",
		labels: []string{"namespace", "instance"},
	},
	"RedisReplicationReplicasSizeCurrent": {
		Name:   "redisreplication_replicas_size_current",
		Help:   "Total current number of redisreplication replicas.",
		Type:   "Gauge",
		labels: []string{"namespace", "instance"},
	},
	"RedisReplicationHasMaster": {
		Name:   "redisreplication_has_master",
		Help:   "Indicates whether the master of a Redis instance was found.",
		Type:   "Gauge",
		labels: []string{"namespace", "instance"},
	},
	"RedisReplicationMasterRoleChangesTotal": {
		Name:   "redisreplication_master_role_changes_total",
		Help:   "Total number of master role changes",
		Type:   "Counter",
		labels: []string{"namespace", "instance"},
	},
	"RedisReplicationConnectedSlavesTotal": {
		Name:   "redisreplication_connected_slaves_total",
		Help:   "Total number of connected slaves",
		Type:   "Counter",
		labels: []string{"namespace", "instance"},
	},
}

var (
	RedisReplicationReplicasSizeDesired = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: metricDescription["RedisReplicationReplicasSizeDesired"].Name,
			Help: metricDescription["RedisReplicationReplicasSizeDesired"].Help,
		},
		metricDescription["RedisReplicationReplicasSizeDesired"].labels,
	)
	RedisReplicationReplicasSizeCurrent = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: metricDescription["RedisReplicationReplicasSizeCurrent"].Name,
			Help: metricDescription["RedisReplicationReplicasSizeCurrent"].Help,
		},
		metricDescription["RedisReplicationReplicasSizeCurrent"].labels,
	)
	RedisReplicationReplicasSizeMismatch = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: metricDescription["RedisReplicationReplicasSizeMismatch"].Name,
			Help: metricDescription["RedisReplicationReplicasSizeMismatch"].Help,
		},
		metricDescription["RedisReplicationReplicasSizeMismatch"].labels,
	)

	RedisReplicationHasMaster = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: metricDescription["RedisReplicationHasMaster"].Name,
			Help: metricDescription["RedisReplicationHasMaster"].Help,
		},
		metricDescription["RedisReplicationHasMaster"].labels,
	)
	RedisReplicationMasterRoleChangesTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: metricDescription["RedisReplicationMasterRoleChangesTotal"].Name,
			Help: metricDescription["RedisReplicationMasterRoleChangesTotal"].Help,
		},
		metricDescription["RedisReplicationMasterRoleChangesTotal"].labels,
	)
	RedisReplicationConnectedSlavesTotal = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: metricDescription["RedisReplicationConnectedSlavesTotal"].Name,
			Help: metricDescription["RedisReplicationConnectedSlavesTotal"].Help,
		},
		metricDescription["RedisReplicationConnectedSlavesTotal"].labels,
	)
)

// RegisterMetrics will register metrics with the global prometheus registry
func RegisterRedisReplicationMetrics() {
	metrics.Registry.MustRegister(
		RedisReplicationReplicasSizeDesired,
		RedisReplicationReplicasSizeCurrent,
		RedisReplicationReplicasSizeMismatch,
		RedisReplicationHasMaster,
		RedisReplicationMasterRoleChangesTotal,
		RedisReplicationConnectedSlavesTotal,
	)
}

// ListMetrics will create a slice with the metrics available in metricDescription
func ListMetrics() []MetricDescription {
	v := make([]MetricDescription, 0, len(metricDescription))
	// Insert value (Name, Help) for each metric
	for _, value := range metricDescription {
		v = append(v, value)
	}

	return v
}
