## Configuration

There is few configurations which is available in Redis operator which can be used as per use case.

## Mode

Mode of the redis setup

```yaml
mode: cluster
```

Available Options:-

- cluster - For cluster mode setup of redis
- standalone - For standalone setup of redis

## Size

Size of the redis cluster pods

```yaml
size: 3
```

Available Options:-

- An valid integer

## Global

In global section, we define the similar configurations accross the redis nodes.

```yaml
  global:
    image: opstree/redis:v2.0
    imagePullPolicy: IfNotPresent
    password: "Opstree@1234"
    resources:
      requests:
        cpu: 100m
        memory: 128Mi
      limits:
        cpu: 100m
        memory: 128Mi
```

## Master

Configuration specific to master nodes of Redis

```yaml
  master:
    service:
      type: ClusterIP
```

## Slave

Configuration specific to slave nodes of Redis

```yaml
  slave:
    service:
      type: ClusterIP
```

## RedisExporter

Redis Exporter Configurations

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

## Storage

Storage definition for redis nodes

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

## Priority Class

Name of the kubernetes priorityclass which you want to associate with redis setup

```yaml
priorityClassName: priority-100
```

## Node Selector

Map of the labels which you want to use as as nodeSelector

```yaml
nodeSelector:
  memory: medium
```

## Security Context

Kubernetes security context for redis pods

```yaml
securityContext:
  runAsUser: 1000
```

## Affinity

Node and pod affinity for redis pods

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