---
title: "Monitoring"
linkTitle: "Monitoring"
weight: 8
date: 2022-11-02T00:19:19Z
description: >
  Monitoring of Redis standalone and cluster setup using Prometheus
---

The redis-operator uses [redis-exporter](https://github.com/oliver006/redis_exporter) to expose metrics of redis setup in Prometheus format. This exporter captures metrics for both redis standalone and cluster setup.

The monitoring architecture is illustrated in the diagram:

![redis_operator_architecture](../../../images/redis-operator-architecture.png)

For the helm chart installation of redis setup, we can simply enable the redis exporter by creating a custom values file for helm chart. The content of the values file will look like this:

```yaml
redisExporter:
  enabled: true
  image: quay.io/opstree/redis-exporter:1.0
  imagePullPolicy: Always
```

When we have defined the redis-exporter related config in values file, we can apply or upgrade the redis setup. We need to pass the created file as an argument to the `helm` command.

Enabling monitoring for standalone setup:

```shell
$ helm upgrade redis ot-helm/redis -f monitoring-values.yaml \
  --install --namespace ot-operators
```

Enabling monitoring for cluster setup:

```shell
$ helm upgrade redis-cluster ot-helm/redis-cluster -f monitoring-values.yaml \
  --set redisCluster.clusterSize=3 --install --namespace ot-operators
```

## ServiceMonitor

Once the exporter is configured, we may have to update Prometheus to monitor this endpoint. For [Prometheus Operator](https://github.com/prometheus-operator/prometheus-operator), we have to create a CRD based object called ServiceMonitor. We can apply the CRD definition as well using the helm command.

```yaml
serviceMonitor:
  enabled: false
  interval: 30s
  scrapeTimeout: 10s
  namespace: monitoring
```

For kubectl related configuration, we may have to create `ServiceMonitor` definition in a YAML manifest and apply it using `kubectl` command.

ServiceMonitor for Redis cluster setup:

```yaml
---
apiVersion: monitoring.coreos.com/v1
kind: ServiceMonitor
metadata:
  name: redis-cluster
  labels:
    redis-operator: "true"
    env: production
spec:
  selector:
    matchLabels:
      redis_setup_type: cluster
  endpoints:
  - port: redis-exporter
    interval: 30s
    scrapeTimeout: 10s
  namespaceSelector:
    matchNames:
    - monitoring
```

ServiceMonitor for Redis standalone setup:

```yaml
---
apiVersion: monitoring.coreos.com/v1
kind: ServiceMonitor
metadata:
  name: redis-standalone
  labels:
    redis-operator: "true"
    env: production
spec:
  selector:
    matchLabels:
      redis_setup_type: standalone
  endpoints:
  - port: redis-exporter
    interval: 30s
    scrapeTimeout: 10s
  namespaceSelector:
    matchNames:
    - monitoring
```

## Grafana Dashboards

There is detailed dashboard created for Redis cluster monitoring setup. Refer to that dashboard once the metrics are available inside Prometheus setup.

[Redis Operator Cluster Dashboard for Prometheus](https://github.com/OT-CONTAINER-KIT/redis-operator/blob/main/dashboards/redis-operator-cluster.json)

![redis_grafana_dashboard](../../../images/grafana1.3b7d307c.png)
