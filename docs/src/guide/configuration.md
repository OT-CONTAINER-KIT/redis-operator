# Configuration

The redis setup in standalone or cluster mode can be customized using custom configuration. If redis setup is done by **Helm**, in that case [values.yaml](https://github.com/OT-CONTAINER-KIT/helm-charts/blob/main/charts/redis-setup/values.yaml) can be updated.

But if the setup is not done via Helm, in that scenario we may have to customize the CRD parameters.

In this configuration section, we have these configuration parameters:-

- [Helm Parameters](configuration.html#helm-parameters)
- [CRD Parameters](configuration.html#crd-parameters)

## Helm Parameters

|**Name**|**Default Value**|**Required**|**Description**|
|--------|-----------------|------------|---------------|
|`name` | redis | true | Name of the redis setup whether it is standalone or cluster |
|`setupMode` | standalone | true | Mode of the redis setup, expected values:- `standalone` or `cluster` |
|`cluster.size` | 3 | false | The number of master and slaves in redis cluster mode setup |
|`cluster.master` | | false | Custom configurations for redis master |
|`cluster.slave` | | false | Custom configurations for redis slave |
|`existingPasswordSecret.enabled` | false | false | To use existing created password secret in Kubernetes |
|`existingPasswordSecret.name` | redis-secret | false | Name of the existing secret in Kubernetes |
|`existingPasswordSecret.key` | password | false | Name of the existing secret key in Kubernetes |
|`global.image` | quay.io/opstree/redis | true | Name of the redis image |
|`global.tag` | v6.2 | true | Tag of the redis image |
|`global.imagePullPolicy` | IfNotPresent | true | Image Pull Policy of the redis image |
|`global.password` | Opstree@1234 | false | Password for the redis setup, leave it blank in case you don't want password |
|`exporter.enabled` | true | true | Redis exporter should be deployed or not |
|`exporter.image` | quay.io/opstree/redis-exporter | true | Name of the redis exporter image |
|`exporter.tag` | v6.2 | true | Tag of the redis exporter image |
|`exporter.imagePullPolicy` | IfNotPresent | true | Image Pull Policy of the redis exporter image |
|`nodeSelector` | {} | false | NodeSelector for redis pods |
|`storageSpec` | {} | false | Storage configuration for redis setup |
|`securityContext` | {} | false | Security Context for redis pods |
|`affinity` | {} | false | Affinity for node and pod for redis pods |
|`tolerations` | {} | false | Tolerations for redis pods |

## CRD Parameters

These are the CRD Parameters which is currently supported by Redis Exporter.

**Mode**

Mode of the redis setup. Available Options:-

- cluster - For cluster mode setup of redis
- standalone - For standalone setup of redis

```yaml
mode: cluster
```

**Size**

Size of the redis cluster pods.

```yaml
size: 3
```

**Global**

In the global section, we define similar configurations across the redis nodes.

```yaml
global:
  image: quay.io/opstree/redis:v6.2
  imagePullPolicy: IfNotPresent
  existingPasswordSecret:
    name: redis-secret
    key: password
  resources:
    requests:
      cpu: 100m
      memory: 128Mi
    limits:
      cpu: 100m
      memory: 128Mi
```

**Master**

Configuration specific to master nodes of Redis, like:- redis configuration parameters and type of service for master.

```yaml
master:
  service:
    type: ClusterIP
```

**Slave**

Configuration specific to slave nodes of Redis, like:- redis configuration parameters and type of service for slave.

```yaml
slave:
  service:
    type: ClusterIP
```

**Redis Exporter**

Redis Exporter configuration which enable the metrics for Redis Database to get monitored by Prometheus.

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

**Storage**

Storage configuration for Redis Statefulset pods.

```yaml
storage:
  volumeClaimTemplate:
    spec:
      storageClassName: csi-cephfs-sc
      accessModes: ["ReadWriteOnce"]
      resources:
        requests:
          storage: 1Gi
    selector: {}
```

**Priority Class**

Name of the Kubernetes priority class which you want to associate with redis setup.

```yaml
priorityClassName: priority-100
```

**Node Selector**

Map of the labels which you want to use as nodeSelector.

```yaml
nodeSelector:
  memory: medium
```

**Security Context**

Kubernetes security context for redis pods.

```yaml
securityContext:
  runAsUser: 1000
```

**Affinity**

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