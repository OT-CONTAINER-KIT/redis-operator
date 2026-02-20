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
| TLS.ca | string | `"ca.crt"` |  |
| TLS.cert | string | `"tls.crt"` |  |
| TLS.key | string | `"tls.key"` |  |
| TLS.secret.secretName | string | `""` |  |
| acl.secret.secretName | string | `""` |  |
| affinity | object | `{}` |  |
| annotations | object | `{}` |  |
| announceHostnames | string | `""` |  |
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
| pdb.enabled | bool | `false` |  |
| pdb.maxUnavailable | string | `nil` |  |
| pdb.minAvailable | int | `1` |  |
| podManagementPolicy | string | `"OrderedReady"` |  |
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
| redisReplication.livenessProbe | object | `{}` |  |
| redisReplication.maxMemoryPercentOfLimit | int | `0` | MaxMemoryPercentOfLimit is the percentage of the Redis container memory limit to be used as maxmemory.    When a memory limit exists, the operator also exposes the computed value via the REDIS_MAX_MEMORY env var.    Default is 0 (disabled). |
| redisReplication.minReadySeconds | int | `0` |  |
| redisReplication.name | string | `""` |  |
| redisReplication.persistentVolumeClaimRetentionPolicy | object | `{}` |  |
| redisReplication.readinessProbe | object | `{}` |  |
| redisReplication.recreateStatefulSetOnUpdateInvalid | bool | `false` | Some fields of statefulset are immutable, such as volumeClaimTemplates. When set to true, the operator will delete the statefulset and recreate it. Default is false. |
| redisReplication.redisSecret.secretKey | string | `""` |  |
| redisReplication.redisSecret.secretName | string | `""` |  |
| redisReplication.resources | object | `{}` |  |
| redisReplication.serviceType | string | `"ClusterIP"` |  |
| redisReplication.tag | string | `"v7.0.15"` |  |
| resolveHostnames | string | `""` | Set to "yes" to resolve and announce hostnames instead of IPs. Required when running with service mesh (e.g. Istio) where pod IPs may be loopback addresses. When enabled, replica-announce-ip will use the pod's FQDN instead of its IP. |
| securityContext | object | `{}` |  |
| sentinel | object | `{"affinity":{},"announceHostnames":"no","downAfterMilliseconds":"5000","enabled":false,"failoverTimeout":"10000","ignoreAnnotations":[],"image":"quay.io/opstree/redis-sentinel","imagePullPolicy":"IfNotPresent","minReadySeconds":0,"nodeSelector":{},"parallelSyncs":"1","persistentVolumeClaimRetentionPolicy":{},"podSecurityContext":{},"priorityClassName":"","redisSecret":{"secretKey":"","secretName":""},"resolveHostnames":"no","resources":{},"securityContext":{},"serviceAccountName":"","size":3,"tag":"v7.0.15","terminationGracePeriodSeconds":null,"tolerations":[],"topologySpreadConstraints":[]}` | Sentinel configuration for automatic failover. When enabled, the operator creates a Sentinel StatefulSet alongside the replication pods. The operator queries Sentinel for the current master instead of forcing master-by-ordinal. |
| sentinel.affinity | object | `{}` | Affinity rules for Sentinel pods, e.g. anti-affinity to keep them off the Redis nodes. |
| sentinel.announceHostnames | string | `"no"` | Whether Sentinel announces hostnames instead of IPs to clients |
| sentinel.downAfterMilliseconds | string | `"5000"` | Time in milliseconds before master is considered down |
| sentinel.failoverTimeout | string | `"10000"` | Failover timeout in milliseconds |
| sentinel.nodeSelector | object | `{}` | Node selector for scheduling Sentinel pods. |
| sentinel.parallelSyncs | string | `"1"` | Number of replicas to reconfigure in parallel during failover |
| sentinel.podSecurityContext | object | `{}` | Pod-level security context for Sentinel pods. |
| sentinel.priorityClassName | string | `""` | PriorityClass name for Sentinel pods. |
| sentinel.redisSecret | object | `{"secretKey":"","secretName":""}` | Secret holding the password Sentinel uses to authenticate to Redis. Leave empty to fall back to redisReplication.redisSecret. |
| sentinel.resolveHostnames | string | `"no"` | Use hostnames instead of IPs for Sentinel monitoring. WARNING: the operator does not pass RESOLVE_HOSTNAMES env var to sentinel pods, so setting this to "yes" will cause SENTINEL MONITOR to fail. Keep as "no". |
| sentinel.securityContext | object | `{}` | Container-level security context for the Sentinel container. |
| sentinel.serviceAccountName | string | `""` | ServiceAccount name for Sentinel pods. |
| sentinel.terminationGracePeriodSeconds | string | `nil` | Termination grace period (in seconds) for Sentinel pods. |
| sentinel.tolerations | list | `[]` | Tolerations for Sentinel pods. |
| sentinel.topologySpreadConstraints | list | `[]` | Topology spread constraints for Sentinel pods. |
| serviceAccountName | string | `""` |  |
| serviceMonitor.enabled | bool | `false` |  |
| serviceMonitor.extraLabels | object | `{}` | extraLabels are added to the servicemonitor when enabled set to true |
| serviceMonitor.interval | string | `"30s"` |  |
| serviceMonitor.namespace | string | `""` | Namespace where servicemonitor resource will be created, if empty it will be created in the same namespace as the redis-replication |
| serviceMonitor.scrapeTimeout | string | `"10s"` |  |
| sidecars | list | `[]` |  |
| storageSpec.volumeClaimTemplate.spec.accessModes[0] | string | `"ReadWriteOnce"` |  |
| storageSpec.volumeClaimTemplate.spec.resources.requests.storage | string | `"1Gi"` |  |
| tolerations | list | `[]` |  |
| topologySpreadConstraints | list | `[]` |  |