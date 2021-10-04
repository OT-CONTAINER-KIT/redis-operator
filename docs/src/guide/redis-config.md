# Redis Standalone

The redis setup in standalone mode can be customized using custom configuration. If redis setup is done by **Helm**, in that case `values.yaml` can be updated.

- [Redis standalone values](https://github.com/OT-CONTAINER-KIT/helm-charts/blob/main/charts/redis/values.yaml)

But if the setup is not done via Helm, in that scenario we may have to customize the CRD parameters.

In this configuration section, we have these configuration parameters:-

- [Helm Parameters](redis-config.html#helm-parameters)
- [CRD Parameters](redis-config.html#crd-parameters)

# Helm Parameters

|**Name**|**Value**|**Description**|
|--------|-----------------|-------|
|`imagePullSecrets` | [] | List of image pull secrets, in case redis image is getting pull from private registry |
|`redisStandalone.secretName` | redis-secret | Name of the existing secret in Kubernetes |
|`redisStandalone.secretKey` | password |  Name of the existing secret key in Kubernetes |
|`redisStandalone.image` | quay.io/opstree/redis | Name of the redis image |
|`redisStandalone.tag` | v6.2 | Tag of the redis image |
|`redisStandalone.imagePullPolicy` | IfNotPresent | Image Pull Policy of the redis image |
|`redisStandalone.resources` | {} | Request and limits for redis statefulset |
|`externalService.enabled`| false | If redis service needs to be exposed using LoadBalancer or NodePort |
|`externalService.annotations`| {} | Kubernetes service related annotations |
|`externalService.serviceType` | NodePort | Kubernetes service type for exposing service, values - ClusterIP, NodePort, and LoadBalancer |
|`externalService.port` | 6379 | Port number on which redis external service should be exposed |
|`serviceMonitor.enabled` | false | Servicemonitor to monitor redis with Prometheus |
|`serviceMonitor.interval` | 30s | Interval at which metrics should be scraped. |
|`serviceMonitor.scrapeTimeout` | 10s | Timeout after which the scrape is ended |
|`serviceMonitor.namespace` | monitoring | 	Namespace in which Prometheus operator is running |
|`redisExporter.enabled` | true | Redis exporter should be deployed or not |
|`redisExporter.image` | quay.io/opstree/redis-exporter | Name of the redis exporter image |
|`redisExporter.tag` | v6.2 | Tag of the redis exporter image |
|`redisExporter.imagePullPolicy` | IfNotPresent | Image Pull Policy of the redis exporter image |
|`redisExporter.env` | [] | Extra environment variables which needs to be added in redis exporter|
|`nodeSelector` | {} | NodeSelector for redis statefulset |
|`priorityClassName`| "" | Priority class name for the redis statefulset |
|`storageSpec` | {} | Storage configuration for redis setup |
|`securityContext` | {} | Security Context for redis pods for changing system or kernel level parameters |
|`affinity` | {} | Affinity for node and pod for redis statefulset |
|`tolerations` | [] | Tolerations for redis statefulset |

# CRD Parameters

These are the CRD Parameters which is currently supported by Redis Exporter for standalone CRD.

**kubernetesConfig**

In the `kubernetesConfig` section, we define configuration related to Kubernetes.

```yaml
  kubernetesConfig:
    image: quay.io/opstree/redis:v6.2
    imagePullPolicy: IfNotPresent
    resources:
      requests:
        cpu: 101m
        memory: 128Mi
      limits:
        cpu: 101m
        memory: 128Mi
    redisSecret:
      name: redis-secret
      key: password
    imagePullSecrets:
      - name: regcred
```


**redisExporter**

`redisExporter` configuration which enable the metrics for Redis Database to get monitored by Prometheus.

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

**storage**

`storage` configuration for Redis Statefulset pods.

```yaml
  storage:
    volumeClaimTemplate:
      spec:
        storageClassName: standard
        accessModes: ["ReadWriteOnce"]
        resources:
          requests:
            storage: 1Gi
```

**priorityClassName**

Name of the Kubernetes priority class which you want to associate with redis setup.

```yaml
priorityClassName: priority-100
```

**nodeSelector**

Map of the labels which you want to use as nodeSelector.

```yaml
  nodeSelector:
    kubernetes.io/hostname: minikube
```

**securityContext**

Kubernetes security context for redis pods.

```yaml
  securityContext:
    runAsUser: 1000
```

**affinity**

Affinity for node and pod for redis setup.

```yaml
  affinity:
    nodeAffinity:
      requiredDuringSchedulingIgnoredDuringExecution:
        nodeSelectorTerms:
        - matchExpressions:
          - key: disktype
            operator: In
            values:
            - ssd
```

**tolerations**

Tolerations for nodes and pods in Kubernetes.

```yaml
  tolerations:
  - key: "key1"
    operator: "Equal"
    value: "value1"
    effect: "NoSchedule"
```
