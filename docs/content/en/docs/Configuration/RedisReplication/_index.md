---
title: "RedisReplication"
linkTitle: "RedisReplication"
weight: 10
date: 2023-04-05T19:00:00Z
description: >
  Configurations and parameters for Redis replication
---

Redis replication configuration can be customized by [values.yaml](https://github.com/OT-CONTAINER-KIT/redis-operator/blob/master/charts/redis-replication/values.yaml). The recommended way of managing the setup is using `helm` but if the setup is not maintained by it, `YAML` CRD parameters can be modified in the manifest.

## Helm Configuration Parameters

| Key | Type | Default | Description |
|-----|------|---------|-------------|
| TLS.ca | string | `"ca.key"` |  |
| TLS.cert | string | `"tls.crt"` |  |
| TLS.key | string | `"tls.key"` |  |
| TLS.secret.secretName | string | `""` |  |
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
| livenessProbe.failureThreshold | int | `3` |  |
| livenessProbe.initialDelaySeconds | int | `1` |  |
| livenessProbe.periodSeconds | int | `10` |  |
| livenessProbe.successThreshold | int | `1` |  |
| livenessProbe.timeoutSeconds | int | `1` |  |
| nodeSelector | object | `{}` |  |
| pdb.enabled | bool | `false` |  |
| pdb.maxUnavailable | string | `nil` |  |
| pdb.minAvailable | int | `1` |  |
| podSecurityContext.fsGroup | int | `1000` |  |
| podSecurityContext.runAsUser | int | `1000` |  |
| priorityClassName | string | `""` |  |
| readinessProbe.failureThreshold | int | `3` |  |
| readinessProbe.initialDelaySeconds | int | `1` |  |
| readinessProbe.periodSeconds | int | `10` |  |
| readinessProbe.successThreshold | int | `1` |  |
| readinessProbe.timeoutSeconds | int | `1` |  |
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
| redisSentinel.recreateStatefulSetOnUpdateInvalid | bool | `false` | Some fields of statefulset are immutable, such as volumeClaimTemplates. When set to true, the operator will delete the statefulset and recreate it. Default is false. |
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
| redisSentinelConfig.resolveHostnames | string | `"no"` |  |
| redisSentinelConfig.announceHostnames | string | `"no"` |  |
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