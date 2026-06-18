---
title: "Redis"
linkTitle: "Redis"
weight: 10
date: 2022-11-02T00:19:19Z
description: >
  Configurations and parameters for Redis standalone
---

Redis standalone configuration can be customized by [values.yaml](https://github.com/OT-CONTAINER-KIT/redis-operator/blob/main/charts/redis/values.yaml). The recommended way of managing the setup is using `helm` but if the setup is not maintained by it, `YAML` CRD parameters can be modified in the manifest.

## Helm Configuration Parameters

| Key                                                             | Type   | Default                                                                  | Description                                                                                                                                                           |
|-----------------------------------------------------------------|--------|--------------------------------------------------------------------------|-----------------------------------------------------------------------------------------------------------------------------------------------------------------------|
| TLS.ca                                                          | string | `"ca.crt"`                                                               |                                                                                                                                                                       |
| TLS.cert                                                        | string | `"tls.crt"`                                                              |                                                                                                                                                                       |
| TLS.key                                                         | string | `"tls.key"`                                                              |                                                                                                                                                                       |
| TLS.secret.secretName                                           | string | `""`                                                                     |                                                                                                                                                                       |
| acl.secret.secretName                                           | string | `""`                                                                     |                                                                                                                                                                       |
| affinity                                                        | object | `{}`                                                                     |                                                                                                                                                                       |
| env                                                             | list   | `[]`                                                                     |                                                                                                                                                                       |
| externalConfig.data                                             | string | `"tcp-keepalive 400\nslowlog-max-len 158\nstream-node-max-bytes 2048\n"` |                                                                                                                                                                       |
| externalConfig.enabled                                          | bool   | `false`                                                                  |                                                                                                                                                                       |
| externalService.enabled                                         | bool   | `false`                                                                  |                                                                                                                                                                       |
| externalService.port                                            | int    | `6379`                                                                   |                                                                                                                                                                       |
| externalService.serviceType                                     | string | `"NodePort"`                                                             |                                                                                                                                                                       |
| initContainer.args                                              | list   | `[]`                                                                     |                                                                                                                                                                       |
| initContainer.command                                           | list   | `[]`                                                                     |                                                                                                                                                                       |
| initContainer.enabled                                           | bool   | `false`                                                                  |                                                                                                                                                                       |
| initContainer.env                                               | list   | `[]`                                                                     |                                                                                                                                                                       |
| initContainer.image                                             | string | `""`                                                                     |                                                                                                                                                                       |
| initContainer.imagePullPolicy                                   | string | `"IfNotPresent"`                                                         |                                                                                                                                                                       |
| initContainer.resources                                         | object | `{}`                                                                     |                                                                                                                                                                       |
| labels                                                          | object | `{}`                                                                     |                                                                                                                                                                       |
| nodeSelector                                                    | object | `{}`                                                                     |                                                                                                                                                                       |
| podSecurityContext.fsGroup                                      | int    | `1000`                                                                   |                                                                                                                                                                       |
| podSecurityContext.runAsUser                                    | int    | `1000`                                                                   |                                                                                                                                                                       |
| priorityClassName                                               | string | `""`                                                                     |                                                                                                                                                                       |
| redisExporter.enabled                                           | bool   | `false`                                                                  |                                                                                                                                                                       |
| redisExporter.env                                               | list   | `[]`                                                                     |                                                                                                                                                                       |
| redisExporter.image                                             | string | `"quay.io/opstree/redis-exporter"`                                       |                                                                                                                                                                       |
| redisExporter.imagePullPolicy                                   | string | `"IfNotPresent"`                                                         |                                                                                                                                                                       |
| redisExporter.resources                                         | object | `{}`                                                                     |                                                                                                                                                                       |
| redisExporter.tag                                               | string | `"v1.44.0"`                                                              |                                                                                                                                                                       |
| redisStandalone.ignoreAnnotations                               | list   | `[]`                                                                     |                                                                                                                                                                       |
| redisStandalone.image                                           | string | `"quay.io/opstree/redis"`                                                |                                                                                                                                                                       |
| redisStandalone.imagePullPolicy                                 | string | `"IfNotPresent"`                                                         |                                                                                                                                                                       |
| redisStandalone.imagePullSecrets                                | list   | `[]`                                                                     |                                                                                                                                                                       |
| redisStandalone.minReadySeconds                                 | int    | `0`                                                                      |                                                                                                                                                                       |
| redisStandalone.name                                            | string | `""`                                                                     |                                                                                                                                                                       |
| redisStandalone.recreateStatefulSetOnUpdateInvalid              | bool   | `false`                                                                  | Some fields of statefulset are immutable, such as volumeClaimTemplates. When set to true, the operator will delete the statefulset and recreate it. Default is false. |
| redisStandalone.redisSecret.secretKey                           | string | `""`                                                                     |                                                                                                                                                                       |
| redisStandalone.redisSecret.secretName                          | string | `""`                                                                     |                                                                                                                                                                       |
| redisStandalone.resources                                       | object | `{}`                                                                     |                                                                                                                                                                       |
| redisStandalone.serviceType                                     | string | `"ClusterIP"`                                                            |                                                                                                                                                                       |
| redisStandalone.tag                                             | string | `"v7.0.15"`                                                              |                                                                                                                                                                       |
| securityContext                                                 | object | `{}`                                                                     |                                                                                                                                                                       |
| serviceAccountName                                              | string | `""`                                                                     |                                                                                                                                                                       |
| serviceMonitor.enabled                                          | bool   | `false`                                                                  |                                                                                                                                                                       |
| serviceMonitor.interval                                         | string | `"30s"`                                                                  |                                                                                                                                                                       |
| serviceMonitor.namespace                                        | string | `"monitoring"`                                                           |                                                                                                                                                                       |
| serviceMonitor.scrapeTimeout                                    | string | `"10s"`                                                                  |                                                                                                                                                                       |
| sidecars.env                                                    | list   | `[]`                                                                     |                                                                                                                                                                       |
| sidecars.image                                                  | string | `""`                                                                     |                                                                                                                                                                       |
| sidecars.imagePullPolicy                                        | string | `"IfNotPresent"`                                                         |                                                                                                                                                                       |
| sidecars.name                                                   | string | `""`                                                                     |                                                                                                                                                                       |
| sidecars.resources.limits.cpu                                   | string | `"100m"`                                                                 |                                                                                                                                                                       |
| sidecars.resources.limits.memory                                | string | `"128Mi"`                                                                |                                                                                                                                                                       |
| sidecars.resources.requests.cpu                                 | string | `"50m"`                                                                  |                                                                                                                                                                       |
| sidecars.resources.requests.memory                              | string | `"64Mi"`                                                                 |                                                                                                                                                                       |
| storageSpec.volumeClaimTemplate.spec.accessModes[0]             | string | `"ReadWriteOnce"`                                                        |                                                                                                                                                                       |
| storageSpec.volumeClaimTemplate.spec.resources.requests.storage | string | `"1Gi"`                                                                  |                                                                                                                                                                       |
| tolerations                                                     | list   | `[]`                                                                     |                                                                                                                                                                       |

## Redis Standalone Instance Configuration

### Dynamic Configuration

Redis Operator supports dynamic configuration for the standalone Redis instance through the top-level `redisConfig` field. You can set Redis configuration parameters that can be modified at runtime without requiring a restart.

#### Example Configuration

```yaml
apiVersion: redis.redis.opstreelabs.in/v1beta2
kind: Redis
metadata:
  name: redis-standalone
spec:
  redisConfig:
    dynamicConfig:
      - "maxmemory-policy allkeys-lru"
      - "slowlog-log-slower-than 5000"
```

#### Configuration Application

- Dynamic configurations are applied to the standalone Redis instance via `CONFIG SET`
- Configuration is applied only after the instance's StatefulSet is in a ready state
- If the instance is not ready or accessible, it will be skipped and retried in the next reconciliation

#### Important Notes

1. **Configuration Validation**
   - Ensure the configuration parameters are supported by your Redis version
   - Use proper format: "parameter value" (e.g., "maxmemory-policy allkeys-lru")
   - Invalid configurations will be logged and skipped

2. **Monitoring**
   - Configuration changes are logged at the pod level
   - Check pod logs for configuration status and any errors
   - Use `kubectl exec` to verify configurations:

   ```bash
   kubectl exec -it <pod-name> -- redis-cli CONFIG GET <parameter>
   ```

3. **Limitations**
   - Only supports parameters that can be modified at runtime
   - `CONFIG SET` is not persisted to disk, so values supplied through `dynamicConfig` are **not retained across pod restarts** unless they are also provided through `externalConfig` (`additionalRedisConfig`). `dynamicConfig` is applied at runtime only and intentionally does not rewrite the ConfigMap, so that runtime-tunable parameters do not trigger a StatefulSet rolling restart.