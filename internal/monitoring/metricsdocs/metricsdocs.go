package main

import (
	"bytes"
	"fmt"
	"sort"
	"text/template"

	"github.com/OT-CONTAINER-KIT/redis-operator/internal/monitoring"
)

func main() {
	replicationMetrics := monitoring.ListRedisReplicationMetrics()
	sort.Slice(replicationMetrics, func(i, j int) bool {
		return replicationMetrics[i].Name < replicationMetrics[j].Name
	})

	// redisCluster := monitoring.ListRedisClusterMetrics()
	clusterMetrics := monitoring.ListRedisClusterMetrics()
	sort.Slice(clusterMetrics, func(i, j int) bool {
		return clusterMetrics[i].Name < clusterMetrics[j].Name
	})

	type MetricsData struct {
		Replication []monitoring.MetricDescription
		Cluster     []monitoring.MetricDescription
	}

	data := MetricsData{
		Replication: replicationMetrics,
		Cluster:     clusterMetrics,
	}

	tmpl, err := template.New("Redis Operator metrics").Parse("# Operator Metrics\n" +
		"This document aims to help users that are not familiar with metrics exposed by this operator.\n" +
		"The metrics documentation is auto-generated by the utility tool \"monitoring/metricsdocs\" and reflects all of the metrics that are exposed by the operator.\n\n" +
		"## Operator Metrics List\n" +
		"\n" +
		"## Redis Replication Metrics" +
		"\n" +
		"{{range .Replication}}\n" +
		"### {{.Name}}\n" +
		"{{.Help}} " +
		"Type: {{.Type}}.\n" +
		"{{end}}" +
		"\n" +
		"## Redis Cluster Metrics" +
		"\n" +
		"{{range .Cluster}}\n" +
		"### {{.Name}}\n" +
		"{{.Help}} " +
		"Type: {{.Type}}.\n" +
		"{{end}}" +
		"\n" +
		"## Developing new metrics\n" +
		"After developing new metrics or changing old ones, please run \"make generate-metricsdocs\" to regenerate this document.\n\n" +
		"If you feel that the new metric doesn't follow these rules, please change \"monitoring/metricsdocs\" according to your needs.")
	if err != nil {
		panic(err)
	}

	// generate the template using the sorted list of metrics
	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, data); err != nil {
		panic(err)
	}

	// print the generated metrics documentation
	fmt.Println(buf.String())
}
