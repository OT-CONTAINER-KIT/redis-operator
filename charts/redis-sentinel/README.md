# redis

Redis is a key-value based distributed database, this helm chart is for redis cluster setup. This helm chart needs [Redis Operator](../redis-operator) inside Kubernetes cluster. The redis cluster definition can be modified or changed by [values.yaml](./values.yaml).

**Homepage:** <https://github.com/ot-container-kit/redis-operator>

## Maintainers

| Name | Email | Url |
| ---- | ------ | --- |
| iamabhishek-dubey |  |  |
| sandy724 |  |  |
| shubham-cmyk |  |  |

## Pre-Requisities

- Kubernetes 1.15+
- Helm 3.X
- Redis Operator 0.7.0

## Source Code

* <https://github.com/ot-container-kit/redis-operator>

```shell
helm repo add ot-helm https://ot-container-kit.github.io/helm-charts/

helm install <my-release> ot-helm/redis-sentinel --namespace <namespace>
```

Redis setup can be upgraded by using `helm upgrade` command:-

```shell
helm upgrade <my-release> ot-helm/redis-sentinel --install --namespace <namespace>
```

For uninstalling the chart:-

```shell
helm delete <my-release> --namespace <namespace>
```

## Values

| Key | Type | Default | Description |
|-----|------|---------|-------------|
| TLS.ca | string | `"ca.key"` |  |
| TLS.cert | string | `"tls.crt"` |  |
| TLS.key | string | `"tls.key"` |  |
| TLS.secret.secretName | string | `""` |  |
| acl.secret.secretName | string | `""` |  |
| affinity | object | `{}` |  |
| env | list | `[]` |  |
| externalConfig.data | string | `"tcp-keepalive 400\nslowlog-max-len 158\nstream-node-max-bytes 2048\n"` |  |
| externalConfig.enabled | bool | `false` |  |
| externalService.enabled | bool | `false` |  |
| externalService.port | int | `26379` |  |
| externalService.serviceType | string | `"NodePort"` |  |
| initContainer.args | list | `[]` |  |
| initContainer.command | list | `[]` |  |
| initContainer.enabled | bool | `false` |  |
| initContainer.env | list | `[]` |  |
| initContainer.image | string | `""` |  |
| initContainer.imagePullPolicy | string | `"IfNotPresent"` |  |
| initContainer.resources | object | `{}` |  |
| labels | object | `{}` |  |
| nodeSelector | object | `{}` |  |
| podSecurityContext.fsGroup | int | `1000` |  |
| podSecurityContext.runAsUser | int | `1000` |  |
| priorityClassName | string | `""` |  |
| redisExporter.enabled | bool | `false` |  |
| redisExporter.env | list | `[]` |  |
| redisExporter.image | string | `"quay.io/opstree/redis-exporter"` |  |
| redisExporter.imagePullPolicy | string | `"IfNotPresent"` |  |
| redisExporter.resources | object | `{}` |  |
| redisExporter.tag | string | `"v1.44.0"` |  |
| redisSentinel.clusterSize | int | `3` |  |
| redisSentinel.ignoreAnnotations | list | `[]` |  |
| redisSentinel.image | string | `"quay.io/opstree/redis-sentinel"` |  |
| redisSentinel.imagePullPolicy | string | `"IfNotPresent"` |  |
| redisSentinel.imagePullSecrets | list | `[]` |  |
| redisSentinel.minReadySeconds | int | `0` |  |
| redisSentinel.name | string | `""` |  |
| redisSentinel.redisSecret.secretKey | string | `""` |  |
| redisSentinel.redisSecret.secretName | string | `""` |  |
| redisSentinel.resources | object | `{}` |  |
| redisSentinel.serviceType | string | `"ClusterIP"` |  |
| redisSentinel.tag | string | `"v7.0.15"` |  |
| redisSentinelConfig.downAfterMilliseconds | string | `""` |  |
| redisSentinelConfig.failoverTimeout | string | `""` |  |
| redisSentinelConfig.masterGroupName | string | `""` |  |
| redisSentinelConfig.parallelSyncs | string | `""` |  |
| redisSentinelConfig.quorum | string | `""` |  |
| redisSentinelConfig.redisPort | string | `""` |  |
| redisSentinelConfig.redisReplicationName | string | `"redis-replication"` |  |
| redisSentinelConfig.redisReplicationPassword.secretKey | string | `""` |  |
| redisSentinelConfig.redisReplicationPassword.secretName | string | `""` |  |
| securityContext | object | `{}` |  |
| serviceAccountName | string | `""` |  |
| serviceMonitor.enabled | bool | `false` |  |
| serviceMonitor.interval | string | `"30s"` |  |
| serviceMonitor.namespace | string | `"monitoring"` |  |
| serviceMonitor.scrapeTimeout | string | `"10s"` |  |
| sidecars.env | list | `[]` |  |
| sidecars.image | string | `""` |  |
| sidecars.imagePullPolicy | string | `"IfNotPresent"` |  |
| sidecars.name | string | `""` |  |
| sidecars.resources.limits.cpu | string | `"100m"` |  |
| sidecars.resources.limits.memory | string | `"128Mi"` |  |
| sidecars.resources.requests.cpu | string | `"50m"` |  |
| sidecars.resources.requests.memory | string | `"64Mi"` |  |
| tolerations | list | `[]` |  |