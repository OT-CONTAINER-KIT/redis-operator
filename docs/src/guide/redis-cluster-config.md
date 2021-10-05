# Redis Cluster

The redis setup cluster mode can be customized using custom configuration. If redis setup is done by **Helm**, in that case `values.yaml` can be updated.

- [Redis cluster values](https://github.com/OT-CONTAINER-KIT/helm-charts/blob/main/charts/redis-cluster/values.yaml)

But if the setup is not done via Helm, in that scenario we may have to customize the CRD parameters.

In this configuration section, we have these configuration parameters:-

- [Helm Parameters](configuration.html#helm-parameters)
- [CRD Parameters](configuration.html#crd-parameters)

## Helm Parameters

|**Name**|**Default Value**|**Description**|
|--------|-----------------|---------------|
|`imagePullSecrets` | [] | List of image pull secrets, in case redis image is getting pull from private registry |
|`redisCluster.clusterSize` | 3 | Size of the redis cluster leader and follower nodes |
|`redisCluster.secretName` | redis-secret | Name of the existing secret in Kubernetes |
|`redisCluster.secretKey` | password | Name of the existing secret key in Kubernetes |
|`redisCluster.image` | quay.io/opstree/redis | Name of the redis image |
|`redisCluster.tag` | v6.2 | Tag of the redis image |
|`redisCluster.imagePullPolicy` | IfNotPresent | Image Pull Policy of the redis image |
|`redisCluster.leader.affinity` | {} | Affinity for node and pods for redis leader statefulset |
|`redisCluster.follower.affinity` | {} | Affinity for node and pods for redis follower statefulset |
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
|`tolerations` | [] | Tolerations for redis statefulset |

# CRD Parameters

These are the CRD Parameters which is currently supported by Redis Exporter for standalone CRD.

**clusterSize**

`clusterSize` is size of the Redis leader and follower nodes.

```yaml
  clusterSize: 3
```

**redisLeader**

`redisLeader` is the field for Redis leader related configurations.

```yaml
  redisLeader:
    redisConfig:
      additionalRedisConfig: redis-external-config
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

**redisFollower**

`redisFollower` is the field for Redis follower related configurations.

```yaml
  redisFollower:
    redisConfig:
      additionalRedisConfig: redis-external-config
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

**tolerations**

Tolerations for nodes and pods in Kubernetes.

```yaml
  tolerations:
  - key: "key1"
    operator: "Equal"
    value: "value1"
    effect: "NoSchedule"
```
