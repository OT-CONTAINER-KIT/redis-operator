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

helm install <my-release> ot-helm/redis-replication --namespace <namespace>
```

Redis setup can be upgraded by using `helm upgrade` command:-

```shell
helm upgrade <my-release> ot-helm/redis-replication --install --namespace <namespace>
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
| externalService.port | int | `6379` |  |
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
| redisExporter.securityContext | object | `{}` |  |
| redisExporter.tag | string | `"v1.44.0"` |  |
| redisReplication.clusterSize | int | `3` |  |
| redisReplication.ignoreAnnotations | list | `[]` |  |
| redisReplication.image | string | `"quay.io/opstree/redis"` |  |
| redisReplication.imagePullPolicy | string | `"IfNotPresent"` |  |
| redisReplication.imagePullSecrets | list | `[]` |  |
| redisReplication.minReadySeconds | int | `0` |  |
| redisReplication.name | string | `""` |  |
| redisReplication.recreateStatefulSetOnUpdateInvalid | bool | `false` | Some fields of statefulset are immutable, such as volumeClaimTemplates. When set to true, the operator will delete the statefulset and recreate it. Default is false. |
| redisReplication.redisSecret.secretKey | string | `""` |  |
| redisReplication.redisSecret.secretName | string | `""` |  |
| redisReplication.resources | object | `{}` |  |
| redisReplication.serviceType | string | `"ClusterIP"` |  |
| redisReplication.tag | string | `"v7.0.15"` |  |
| securityContext | object | `{}` |  |
| serviceAccountName | string | `""` |  |
| serviceMonitor.enabled | bool | `false` |  |
| serviceMonitor.extraLabels | object | `{}` | extraLabels are added to the servicemonitor when enabled set to true |
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
| storageSpec.volumeClaimTemplate.spec.accessModes[0] | string | `"ReadWriteOnce"` |  |
| storageSpec.volumeClaimTemplate.spec.resources.requests.storage | string | `"1Gi"` |  |
| tolerations | list | `[]` |  |