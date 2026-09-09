package monitoring

import "github.com/prometheus/client_golang/prometheus"

var RedisSentinelDescription = map[string]MetricDescription{
	"RedisSentinelSkipReconcile": {
		Name:   "redissentinel_skipreconcile",
		Help:   "Whether skip-reconcile of RedisSentinel enabled or not",
		Type:   "Gauge",
		labels: []string{"namespace", "instance"},
	},
	"RedisSentinelHealthy": {
		Name:   "redissentinel_healthy",
		Help:   "Whether or not to check Redis Sentinel Health status.",
		Type:   "Gauge",
		labels: []string{"namespace", "instance"},
	},
	"RedisSentinelMonitorTotal": {
		Name:   "redissentinel_monitor_total",
		Help:   "Total number of Redis Sentinel monitor event.",
		Type:   "Counter",
		labels: []string{"namespace", "instance"},
	},
	"RedisSentinelResetTotal": {
		Name:   "redissentinel_reset_total",
		Help:   "Total number of Redis Sentinel reset event.",
		Type:   "Counter",
		labels: []string{"namespace", "instance"},
	},
	"RedisSentinelSetTotal": {
		Name:   "redissentinel_set_total",
		Help:   "Total number of Redis Sentinel set event.",
		Type:   "Counter",
		labels: []string{"namespace", "instance"},
	},
}

var (
	RedisSentinelSkipReconcile = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: RedisSentinelDescription["RedisSentinelSkipReconcile"].Name,
			Help: RedisSentinelDescription["RedisSentinelSkipReconcile"].Help,
		},
		RedisSentinelDescription["RedisSentinelSkipReconcile"].labels,
	)

	RedisSentinelHealthy = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: RedisSentinelDescription["RedisSentinelHealthy"].Name,
			Help: RedisSentinelDescription["RedisSentinelHealthy"].Help,
		},
		RedisSentinelDescription["RedisSentinelHealthy"].labels,
	)

	RedisSentinelMonitorTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: RedisSentinelDescription["RedisSentinelMonitorTotal"].Name,
			Help: RedisSentinelDescription["RedisSentinelMonitorTotal"].Help,
		},
		RedisSentinelDescription["RedisSentinelMonitorTotal"].labels,
	)

	RedisSentinelResetTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: RedisSentinelDescription["RedisSentinelResetTotal"].Name,
			Help: RedisSentinelDescription["RedisSentinelResetTotal"].Help,
		},
		RedisSentinelDescription["RedisSentinelResetTotal"].labels,
	)

	RedisSentinelSetTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: RedisSentinelDescription["RedisSentinelSetTotal"].Name,
			Help: RedisSentinelDescription["RedisSentinelSetTotal"].Help,
		},
		RedisSentinelDescription["RedisSentinelSetTotal"].labels,
	)
)

func ListRedisSentinelMetrics() []MetricDescription {
	metrics := make([]MetricDescription, 0, len(RedisSentinelDescription))
	for _, desc := range RedisSentinelDescription {
		metrics = append(metrics, desc)
	}
	return metrics
}
