---
title: "RedisReplication"
linkTitle: "RedisReplication"
weight: 10
date: 2023-04-05T19:00:00Z
description: >
  Configurations and parameters for Redis replication
---

Redis replication configuration can be customized by [values.yaml](https://github.com/OT-CONTAINER-KIT/redis-operator/blob/main/charts/redis-replication/values.yaml). The recommended way of managing the setup is using `helm` but if the setup is not maintained by it, `YAML` CRD parameters can be modified in the manifest.

## Helm Configuration Parameters

| Key | Type | Default | Description |
|-----|------|---------|-------------|
| redisReplication.name | string | `""` | Name override for the RedisReplication resource |
| redisReplication.clusterSize | int | `3` | Number of Redis nodes in the replication setup |
| redisReplication.image | string | `"quay.io/opstree/redis"` | Redis container image |
| redisReplication.tag | string | `"v7.0.15"` | Redis image version tag |
| redisReplication.imagePullPolicy | string | `"IfNotPresent"` | Image pull policy for the Redis image |
| redisReplication.imagePullSecrets | list | `[]` | Image pull secrets for private registries |
| redisReplication.redisSecret.secretName | string | `""` | Secret containing Redis password |
| redisReplication.redisSecret.secretKey | string | `""` | Key in the secret containing Redis password |
| redisReplication.serviceType | string | `"ClusterIP"` | Kubernetes service type used by Redis |
| redisReplication.resources | object | `{}` | Resource requests and limits for Redis pods |
| redisReplication.ignoreAnnotations | list | `[]` | List of annotations ignored by the operator |
| redisReplication.minReadySeconds | int | `0` | Minimum number of seconds for a pod to be ready before it is considered available |
| redisReplication.recreateStatefulSetOnUpdateInvalid | bool | `false` | Recreates the StatefulSet when immutable fields need to be updated |
| redisReplication.maxMemoryPercentOfLimit | int | `0` | Sets Redis maxmemory as a percentage of container memory limit |
| externalConfig.enabled | bool | `false` | Enables custom Redis configuration from ConfigMap data |
| externalConfig.data | string | multiline config | Additional Redis configuration parameters |
| externalService.enabled | bool | `false` | Enables external access to Redis |
| externalService.serviceType | string | `"NodePort"` | Service type for external Redis access |
| externalService.port | int | `6379` | Port used for external Redis access |
| serviceMonitor.enabled | bool | `false` | Enables Prometheus ServiceMonitor |
| serviceMonitor.interval | string | `"30s"` | Prometheus scrape interval |
| serviceMonitor.scrapeTimeout | string | `"10s"` | Prometheus scrape timeout |
| serviceMonitor.namespace | string | `""` | Namespace where the ServiceMonitor is created |
| redisExporter.enabled | bool | `false` | Enables Redis exporter |
| redisExporter.image | string | `"quay.io/opstree/redis-exporter"` | Redis exporter image |
| redisExporter.tag | string | `"v1.44.0"` | Redis exporter image tag |
| redisExporter.imagePullPolicy | string | `"IfNotPresent"` | Image pull policy for the exporter |
| redisExporter.resources | object | `{}` | Resource requests and limits for the exporter |
| initContainer.enabled | bool | `false` | Enables init container for Redis pods |
| initContainer.image | string | `""` | Init container image |
| initContainer.imagePullPolicy | string | `"IfNotPresent"` | Image pull policy for the init container |
| sidecars | list | `[]` | Additional sidecar containers for Redis pods |
| priorityClassName | string | `""` | Priority class for Redis pods |
| nodeSelector | object | `{}` | Node selector for Redis pods |
| storageSpec.volumeClaimTemplate.spec.accessModes[0] | string | `"ReadWriteOnce"` | Access mode for persistent storage |
| storageSpec.volumeClaimTemplate.spec.resources.requests.storage | string | `"1Gi"` | Requested persistent storage size |
| podSecurityContext.runAsUser | int | `1000` | User ID used to run Redis containers |
| podSecurityContext.fsGroup | int | `1000` | Group ID used for mounted volumes |
| securityContext | object | `{}` | Pod/container security context settings |
| affinity | object | `{}` | Affinity rules for pod scheduling |
| tolerations | list | `[]` | Tolerations for pod scheduling |
| topologySpreadConstraints | list | `[]` | Topology spread constraints for Redis pods |
| serviceAccountName | string | `""` | Service account used by Redis pods |
| TLS.ca | string | `"ca.key"` | Name of the TLS CA file |
| TLS.cert | string | `"tls.crt"` | Name of the TLS certificate file |
| TLS.key | string | `"tls.key"` | Name of the TLS private key file |
| TLS.secret.secretName | string | `""` | Secret containing TLS materials |
| acl.secret.secretName | string | `""` | Secret containing ACL configuration |
| env | list | `[]` | Additional environment variables for Redis pods |
| pdb.enabled | bool | `false` | Enables PodDisruptionBudget |
| pdb.minAvailable | int | `1` | Minimum number of pods that must remain available |
| pdb.maxUnavailable | null | `null` | Maximum number of pods that can be unavailable |
| sentinel.enabled | bool | `false` | Enables Redis Sentinel for automatic failover |
| sentinel.image | string | `"quay.io/opstree/redis-sentinel"` | Redis Sentinel image |
| sentinel.tag | string | `"v7.0.15"` | Redis Sentinel image tag |
| sentinel.imagePullPolicy | string | `"IfNotPresent"` | Image pull policy for Sentinel |
| sentinel.size | int | `3` | Number of Sentinel instances |
| sentinel.resources | object | `{}` | Resource requests and limits for Sentinel |
| sentinel.ignoreAnnotations | list | `[]` | List of annotations ignored by Sentinel |
| sentinel.minReadySeconds | int | `0` | Minimum number of seconds for a Sentinel pod to be ready |
| sentinel.parallelSyncs | string | `"1"` | Number of replicas reconfigured in parallel during failover |
| sentinel.failoverTimeout | string | `"10000"` | Sentinel failover timeout in milliseconds |
| sentinel.downAfterMilliseconds | string | `"5000"` | Time before Sentinel considers the master down |
| sentinel.resolveHostnames | string | `"no"` | Whether Sentinel resolves hostnames instead of IPs |
| sentinel.announceHostnames | string | `"no"` | Whether Sentinel announces hostnames to clients |

## RedisReplication Instance Configuration

### Dynamic Configuration

Redis Operator supports dynamic configuration for Redis instances in a replication setup through the top-level `redisConfig` field. You can set Redis configuration parameters that can be modified at runtime without requiring a restart.

#### Example Configuration

```yaml
apiVersion: redis.redis.opstreelabs.in/v1beta2
kind: RedisReplication
metadata:
  name: redis-replication
spec:
  redisConfig:
    dynamicConfig:
      - "maxmemory-policy allkeys-lru"
      - "slowlog-log-slower-than 5000"
```

#### Configuration Application

- Dynamic configurations are applied to all Redis instances in the replication setup
- The operator ensures all accessible instances receive the configuration
- If an instance is not ready or accessible, it will be skipped and retried in the next reconciliation
- Configuration changes are applied only when the replication setup is in a ready state

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

3. **Best Practices**
   - Use dynamic configuration for parameters that need to be consistent across the replication setup
   - Test configuration changes in non-production environments first

4. **Limitations**
   - Only supports parameters that can be modified at runtime