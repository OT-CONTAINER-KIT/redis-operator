# Prometheus Monitoring

The redis-operator uses [redis-exporter](https://github.com/oliver006/redis_exporter) to expose metrics of redis setup in Prometheus format. This exporter captures metrics for both redis standalone and cluster setup.

This can be enabled and disabled from configuration. For example:-

```yaml
redisExporter:
  enabled: true
  image: quay.io/opstree/redis-exporter:1.0
  imagePullPolicy: Always
  resources:
    requests:
      cpu: 100m
      memory: 128Mi
    limits:
      cpu: 100m
      memory: 128Mi
  env:
  - name: REDIS_EXPORTER_INCL_SYSTEM_METRICS
    value: "true"
  - name: UI_PROPERTIES_FILE_NAME
    valueFrom:
      configMapKeyRef:
        name: game-demo
        key: ui_properties_file_name
  - name: SECRET_USERNAME
    valueFrom:
      secretKeyRef:
        name: mysecret
        key: username
```

Once the exporter is configured, we may have to update Prometheus to monitor this endpoint. For [Prometheus Operator](https://github.com/prometheus-operator/prometheus-operator), we have to create a CRD based object called **ServiceMonitor**.

### Redis Cluster

```yaml
---
apiVersion: monitoring.coreos.com/v1
kind: ServiceMonitor
metadata:
  name: redis-monitoring-cluster
  labels:
    redis-operator: "true"
    env: production
spec:
  selector:
    matchLabels:
      redis_setup_type: cluster
  endpoints:
  - port: redis-exporter
```

### Redis Standalone

```yaml
---
apiVersion: monitoring.coreos.com/v1
kind: ServiceMonitor
metadata:
  name: redis-monitoring-standalone
  labels:
    redis-operator: "true"
    env: production
spec:
  selector:
    matchLabels:
      redis_setup_type: standalone
  endpoints:
  - port: redis-exporter
```
