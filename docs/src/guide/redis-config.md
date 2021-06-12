# Redis Standalone

The redis setup in standalone mode can be customized using custom configuration. If redis setup is done by **Helm**, in that case `values.yaml` can be updated.

- [Redis standalone values](https://github.com/OT-CONTAINER-KIT/helm-charts/blob/main/charts/redis/values.yaml) 

But if the setup is not done via Helm, in that scenario we may have to customize the CRD parameters.

In this configuration section, we have these configuration parameters:-

- [Helm Parameters](configuration.html#helm-parameters)
- [CRD Parameters](configuration.html#crd-parameters)

## Helm Parameters

|**Name**|**Default Value**|**Required**|**Description**|
|--------|-----------------|------------|---------------|
|`redisStandalone.secretName` | redis-secret | false | Name of the existing secret in Kubernetes |
|`redisStandalone.secretKey` | password | false | Name of the existing secret key in Kubernetes |
|`redisStandalone.image` | quay.io/opstree/redis | true | Name of the redis image |
|`redisStandalone.tag` | v6.2 | true | Tag of the redis image |
|`redisStandalone.imagePullPolicy` | IfNotPresent | true | Image Pull Policy of the redis image |
|`redisStandalone.serviceType` | ClusterIP | false | Kubernetes service type for Redis |
|`redisExporter.enabled` | true | true | Redis exporter should be deployed or not |
|`redisExporter.image` | quay.io/opstree/redis-exporter | true | Name of the redis exporter image |
|`redisExporter.tag` | v6.2 | true | Tag of the redis exporter image |
|`redisExporter.imagePullPolicy` | IfNotPresent | true | Image Pull Policy of the redis exporter image |
|`nodeSelector` | {} | false | NodeSelector for redis pods |
|`storageSpec` | {} | false | Storage configuration for redis setup |
|`securityContext` | {} | false | Security Context for redis pods |
|`affinity` | {} | false | Affinity for node and pod for redis pods |
|`tolerations` | {} | false | Tolerations for redis pods |

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
    serviceType: LoadBalancer
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

