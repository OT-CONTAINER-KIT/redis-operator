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

| Key                                                             | Type   | Default                                                                  | Description |
|-----------------------------------------------------------------|--------|--------------------------------------------------------------------------|-------------|
| TLS.ca                                                          | string | `"ca.key"`                                                               |             |
| TLS.cert                                                        | string | `"tls.crt"`                                                              |             |
| TLS.key                                                         | string | `"tls.key"`                                                              |             |
| TLS.secret.secretName                                           | string | `""`                                                                     |             |
| acl.secret.secretName                                           | string | `""`                                                                     |             |
| affinity                                                        | object | `{}`                                                                     |             |
| env                                                             | list   | `[]`                                                                     |             |
| externalConfig.data                                             | string | `"tcp-keepalive 400\nslowlog-max-len 158\nstream-node-max-bytes 2048\n"` |             |
| externalConfig.enabled                                          | bool   | `false`                                                                  |             |
| externalService.enabled                                         | bool   | `false`                                                                  |             |
| externalService.port                                            | int    | `6379`                                                                   |             |
| externalService.serviceType                                     | string | `"NodePort"`                                                             |             |
| initContainer.args                                              | list   | `[]`                                                                     |             |
| initContainer.command                                           | list   | `[]`                                                                     |             |
| initContainer.enabled                                           | bool   | `false`                                                                  |             |
| initContainer.env                                               | list   | `[]`                                                                     |             |
| initContainer.image                                             | string | `""`                                                                     |             |
| initContainer.imagePullPolicy                                   | string | `"IfNotPresent"`                                                         |             |
| initContainer.resources                                         | object | `{}`                                                                     |             |
| labels                                                          | object | `{}`                                                                     |             |
| nodeSelector                                                    | object | `{}`                                                                     |             |
| podSecurityContext.fsGroup                                      | int    | `1000`                                                                   |             |
| podSecurityContext.runAsUser                                    | int    | `1000`                                                                   |             |
| priorityClassName                                               | string | `""`                                                                     |             |
| redisExporter.enabled                                           | bool   | `false`                                                                  |             |
| redisExporter.env                                               | list   | `[]`                                                                     |             |
| redisExporter.image                                             | string | `"quay.io/opstree/redis-exporter"`                                       |             |
| redisExporter.imagePullPolicy                                   | string | `"IfNotPresent"`                                                         |             |
| redisExporter.resources                                         | object | `{}`                                                                     |             |
| redisExporter.tag                                               | string | `"v1.44.0"`                                                              |             |
| redisReplication.clusterSize                                    | int    | `3`                                                                      |             |
| redisReplication.ignoreAnnotations                              | list   | `[]`                                                                     |             |
| redisReplication.image                                          | string | `"quay.io/opstree/redis"`                                                |             |
| redisReplication.imagePullPolicy                                | string | `"IfNotPresent"`                                                         |             |
| redisReplication.imagePullSecrets                               | list   | `[]`                                                                     |             |
| redisReplication.minReadySeconds                                | int    | `0`                                                                      |             |
| redisReplication.name                                           | string | `""`                                                                     |             |
| redisReplication.redisSecret.secretKey                          | string | `""`                                                                     |             |
| redisReplication.redisSecret.secretName                         | string | `""`                                                                     |             |
| redisReplication.resources                                      | object | `{}`                                                                     |             |
| redisReplication.serviceType                                    | string | `"ClusterIP"`                                                            |             |
| redisReplication.tag                                            | string | `"v7.0.15"`                                                              |             |
| securityContext                                                 | object | `{}`                                                                     |             |
| serviceAccountName                                              | string | `""`                                                                     |             |
| serviceMonitor.enabled                                          | bool   | `false`                                                                  |             |
| serviceMonitor.interval                                         | string | `"30s"`                                                                  |             |
| serviceMonitor.namespace                                        | string | `"monitoring"`                                                           |             |
| serviceMonitor.scrapeTimeout                                    | string | `"10s"`                                                                  |             |
| sidecars.env                                                    | list   | `[]`                                                                     |             |
| sidecars.image                                                  | string | `""`                                                                     |             |
| sidecars.imagePullPolicy                                        | string | `"IfNotPresent"`                                                         |             |
| sidecars.name                                                   | string | `""`                                                                     |             |
| sidecars.resources.limits.cpu                                   | string | `"100m"`                                                                 |             |
| sidecars.resources.limits.memory                                | string | `"128Mi"`                                                                |             |
| sidecars.resources.requests.cpu                                 | string | `"50m"`                                                                  |             |
| sidecars.resources.requests.memory                              | string | `"64Mi"`                                                                 |             |
| storageSpec.volumeClaimTemplate.spec.accessModes[0]             | string | `"ReadWriteOnce"`                                                        |             |
| storageSpec.volumeClaimTemplate.spec.resources.requests.storage | string | `"1Gi"`                                                                  |             |
| tolerations                                                     | list   | `[]`                                                                     |             |