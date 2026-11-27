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
| redisReplication.maxMemoryPercentOfLimit | int | `0` | Sets Redis maxmemory as a percentage of container memory limit. Requires the `GenerateConfigInInitContainer` feature gate on the operator; otherwise maxmemory stays 0 |
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
| sentinel.redisSecret.secretName | string | `""` | Secret containing the Sentinel password (`requirepass` on port 26379) |
| sentinel.redisSecret.secretKey | string | `""` | Key in the secret containing the Sentinel password |
| sentinel.pdb.enabled | bool | `false` | Creates a PodDisruptionBudget for the Sentinel pods, named after the Sentinel StatefulSet. The top-level `pdb` covers only the Redis pods |
| sentinel.pdb.minAvailable | int | `null` | Minimum number of Sentinel pods that must stay available. With neither this nor `maxUnavailable` set, a majority of the Sentinels |
| sentinel.pdb.maxUnavailable | int | `null` | Maximum number of Sentinel pods that can be unavailable. Takes precedence over `minAvailable` |

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
   - `CONFIG SET` is not persisted to disk, so values supplied through `dynamicConfig` are **not retained across pod restarts** unless they are also provided through `externalConfig` (`additionalRedisConfig`). `dynamicConfig` is applied at runtime only and intentionally does not rewrite the ConfigMap, so that runtime-tunable parameters do not trigger a StatefulSet rolling restart.

## Embedded Sentinel Authentication

When `spec.sentinel` is set, two secrets control authentication:

- `spec.kubernetesConfig.redisSecret` is the Redis password. The embedded Sentinel uses it as `auth-pass` towards the monitored master; it is passed to the Sentinel pod as `MASTER_PASSWORD`, and the operator sets it again with `SENTINEL SET mymaster auth-pass` whenever it points Sentinel at a new master address.
- `spec.sentinel.redisSecret` is the Sentinel's own password (`requirepass` on port 26379), the same role `kubernetesConfig.redisSecret` plays on a standalone `RedisSentinel`. It is passed to the Sentinel pod as `REDIS_PASSWORD`; the operator authenticates to Sentinel with it, and Sentinel-aware clients must send it as well. When it is omitted, the Sentinel port accepts unauthenticated connections.
- The preStop hook of the Redis pods runs `SENTINEL FAILOVER` when the master is terminated. It authenticates to Sentinel with `spec.sentinel.redisSecret`, which is passed to the Redis pods as `SENTINEL_REDISCLI_AUTH`, and sends no password when it is omitted. The two secrets may hold different passwords.

Previously `spec.sentinel.redisSecret` only changed the boot-time `MASTER_PASSWORD` and the Sentinel port was always unauthenticated. If you already set it, upgrading enables `requirepass` on the Sentinel pods, so clients that discover the master through Sentinel must start sending that password.

```yaml
apiVersion: redis.redis.opstreelabs.in/v1beta2
kind: RedisReplication
metadata:
  name: redis-replication
spec:
  clusterSize: 3
  kubernetesConfig:
    image: quay.io/opstree/redis:latest
    redisSecret:
      name: redis-secret
      key: password
  sentinel:
    size: 3
    image: quay.io/opstree/redis-sentinel:latest
    redisSecret:
      name: redis-sentinel-secret
      key: password
```

## Embedded Sentinel Probes

The Sentinel container has readiness and liveness probes that run `redis-cli PING` against port 26379 and expect `PONG`, authenticating with `spec.sentinel.redisSecret` when it is set and using TLS when `spec.TLS` is set. A Sentinel pod is ready only once it answers. The operator waits for all Sentinel pods to be ready before configuring replication and asks only ready Sentinels which master they monitor.

## Embedded Sentinel TLS

When `spec.TLS` is set, the embedded Sentinel uses the same TLS secret as the Redis pods:

- The secret is mounted at `/tls` in the Sentinel pods, which listen only on TLS (port 26379) and require a client certificate.
- Sentinel connects to Redis and to the other Sentinels over TLS, presenting the same certificate.
- The operator connects to Sentinel over TLS with that certificate as well. It verifies the certificate chain against the CA in the secret but not the hostname, as it does for the Redis pods.

Because one certificate is used as both server and client certificate, it must be valid for both: either it has no extended key usage, or it lists both `server auth` and `client auth`. The secret must contain the CA certificate (`ca.crt`, or the key named in `spec.TLS.ca`). Sentinel needs it to verify Redis and its peers, and the operator verifies Sentinel against it, so unlike the Redis connections neither falls back to the system trust store. While the CA is missing from the secret, for example during a rotation, the operator does not connect to Sentinel and retries on the next reconcile.

The sentinel image only configures the TLS listener, so the operator adds `tls-replication yes` and `tls-auth-clients yes` through an owned ConfigMap named `<name>-s-tls`. The image includes that file as the last line of `sentinel.conf`. If a ConfigMap with that name already exists and is not controlled by the RedisReplication, the operator leaves it untouched and reports an error instead of taking it over. `spec.sentinel.additionalSentinelConfig` is not applied to the embedded Sentinel.

Sentinel-aware clients must connect to the Sentinel service over TLS and present a client certificate signed by the same CA.

When the certificate is reissued, the operator uses the new one from its next reconcile on, because it reads the secret every time it connects. The secret is mounted as a directory, so the files in the Sentinel pods are updated too, but a running Sentinel keeps the certificate it loaded at startup: Sentinel has no `CONFIG` command, and `SENTINEL CONFIG SET` does not accept the TLS settings. Restart the Sentinel pods one at a time, each after the previous one is ready again, before the old certificate expires. Sentinels with the old and the new certificate keep working together as long as both are signed by the same CA. The Redis pods likewise keep the certificate they loaded until they restart.

```yaml
apiVersion: redis.redis.opstreelabs.in/v1beta2
kind: RedisReplication
metadata:
  name: redis-replication
spec:
  clusterSize: 3
  TLS:
    ca: ca.crt
    cert: tls.crt
    key: tls.key
    secret:
      secretName: redis-tls-cert
  kubernetesConfig:
    image: quay.io/opstree/redis:latest
  sentinel:
    size: 3
    image: quay.io/opstree/redis-sentinel:latest
```

## ACL Configuration

Enabling `spec.acl` (either `acl.secret` or `acl.persistentVolumeClaim`) makes Redis load users from the ACL file via the `aclfile` directive. Once `aclfile` is set, the ACL file is the sole authority on users: `requirepass` is ignored, and when the file does not define `default`, Redis creates a `default` user without a password.

The operator, the health probes, replicas (`masterauth`) and Sentinel (`auth-pass`) all authenticate as the `default` user with the password from `spec.kubernetesConfig.redisSecret`. When both `redisSecret` and `acl` are configured, the ACL file must therefore define the `default` user with the same password, either in plain text (`>password`) or as a SHA-256 hash (`#<sha256>`):

```yaml
apiVersion: v1
kind: Secret
metadata:
  name: redis-secret
stringData:
  password: Opstree1234
  user.acl: |
    user default on >Opstree1234 ~* &* +@all
    user app on >app-password ~app:* &* +@all
---
apiVersion: redis.redis.opstreelabs.in/v1beta2
kind: RedisReplication
metadata:
  name: redisreplication
spec:
  kubernetesConfig:
    redisSecret:
      name: redis-secret
      key: password
  acl:
    secret:
      secretName: redis-secret
```

With the `GenerateConfigInInitContainer` feature gate enabled, the `init-config` container validates this before writing the configuration. It fails with a descriptive error when the ACL file is missing, does not define `default`, leaves it disabled (no `on`), sets it to `nopass`, or defines it with a different password, instead of letting the pod crash-loop on `NOAUTH`.
