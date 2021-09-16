# Prometheus Monitoring

The redis-operator uses [redis-exporter](https://github.com/oliver006/redis_exporter) to expose metrics of redis setup in Prometheus format. This exporter captures metrics for both redis standalone and cluster setup.

If we are using helm chart for the installation of redis setup, we can simply enable the redis exporter by creating a custom values file for helm chart. The content of the values file will look like this:-

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

When we have defined the redis-exporter related config in values file, we can apply or upgrade the redis setup. We need to pass the created file as an argument to the `helm` command.

```shell
# redis standalone
$ helm upgrade redis ot-helm/redis -f custom-values.yaml \
  --install --namespace ot-operators

# redis cluster
$ helm upgrade redis-cluster ot-helm/redis-cluster -f custom-values.yaml \
  --set redisCluster.clusterSize=3 --install --namespace ot-operators
```

## ServiceMonitor

Once the exporter is configured, we may have to update Prometheus to monitor this endpoint. For [Prometheus Operator](https://github.com/prometheus-operator/prometheus-operator), we have to create a CRD based object called **ServiceMonitor**. We can apply the CRD definition as well using the `helm` command.

```yaml
serviceMonitor:
  enabled: false
  interval: 30s
  scrapeTimeout: 10s
  namespace: monitoring
```

```shell
# redis standalone
$ helm upgrade redis ot-helm/redis -f custom-values.yaml \
  --install --namespace ot-operators

# redis cluster
$ helm upgrade redis-cluster ot-helm/redis-cluster -f custom-values.yaml \
  --set redisCluster.clusterSize=3 --install --namespace ot-operators
```

For kubectl related configuration, we may have to create `ServiceMonitor` definition in a yaml file and apply it using `kubectl` command.

**Redis Cluster**

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
    interval: 30s
    scrapeTimeout: 10s
  namespaceSelector:
    matchNames:
    - monitoring
```

**Redis Standalone**

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
    interval: 30s
    scrapeTimeout: 10s
  namespaceSelector:
    matchNames:
    - monitoring
```
