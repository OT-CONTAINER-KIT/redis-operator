# API Reference

## Packages
- [redis.redis.opstreelabs.in/v1beta2](#redisredisopstreelabsinv1beta2)


## redis.redis.opstreelabs.in/v1beta2

Package v1beta2 contains API Schema definitions for the redis v1beta2 API group

### Resource Types
- [Redis](#redis)
- [RedisCluster](#rediscluster)
- [RedisReplication](#redisreplication)
- [RedisSentinel](#redissentinel)



#### ClusterStorage



Node-conf needs to be added only in redis cluster



_Appears in:_
- [RedisClusterSpec](#redisclusterspec)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `nodeConfVolume` _boolean_ |  | false |  |
| `nodeConfVolumeClaimTemplate` _[PersistentVolumeClaim](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.31/#persistentvolumeclaim-v1-core)_ |  |  |  |
| `keepAfterDelete` _boolean_ |  |  |  |
| `volumeClaimTemplate` _[PersistentVolumeClaim](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.31/#persistentvolumeclaim-v1-core)_ |  |  |  |
| `volumeMount` _[AdditionalVolume](#additionalvolume)_ |  |  |  |


#### Redis



Redis is the Schema for the redis API





| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `apiVersion` _string_ | `redis.redis.opstreelabs.in/v1beta2` | | |
| `kind` _string_ | `Redis` | | |
| `metadata` _[ObjectMeta](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.31/#objectmeta-v1-meta)_ | Refer to Kubernetes API documentation for fields of `metadata`. |  |  |
| `spec` _[RedisSpec](#redisspec)_ |  |  |  |


#### RedisCluster



RedisCluster is the Schema for the redisclusters API





| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `apiVersion` _string_ | `redis.redis.opstreelabs.in/v1beta2` | | |
| `kind` _string_ | `RedisCluster` | | |
| `metadata` _[ObjectMeta](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.31/#objectmeta-v1-meta)_ | Refer to Kubernetes API documentation for fields of `metadata`. |  |  |
| `spec` _[RedisClusterSpec](#redisclusterspec)_ |  |  |  |


#### RedisClusterSpec



RedisClusterSpec defines the desired state of RedisCluster



_Appears in:_
- [RedisCluster](#rediscluster)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `clusterSize` _integer_ |  |  |  |
| `kubernetesConfig` _[KubernetesConfig](#kubernetesconfig)_ |  |  |  |
| `hostNetwork` _boolean_ |  |  |  |
| `port` _integer_ |  | 6379 |  |
| `clusterVersion` _string_ |  | v7 |  |
| `redisConfig` _[RedisConfig](#redisconfig)_ |  |  |  |
| `redisLeader` _[RedisLeader](#redisleader)_ |  |  |  |
| `redisFollower` _[RedisFollower](#redisfollower)_ |  |  |  |
| `redisExporter` _[RedisExporter](#redisexporter)_ |  |  |  |
| `storage` _[ClusterStorage](#clusterstorage)_ |  |  |  |
| `podSecurityContext` _[PodSecurityContext](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.31/#podsecuritycontext-v1-core)_ |  |  |  |
| `priorityClassName` _string_ |  |  |  |
| `resources` _[ResourceRequirements](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.31/#resourcerequirements-v1-core)_ |  |  |  |
| `TLS` _[TLSConfig](#tlsconfig)_ |  |  |  |
| `acl` _[ACLConfig](#aclconfig)_ |  |  |  |
| `initContainer` _[InitContainer](#initcontainer)_ |  |  |  |
| `sidecars` _[Sidecar](#sidecar)_ |  |  |  |
| `serviceAccountName` _string_ |  |  |  |
| `persistenceEnabled` _boolean_ |  |  |  |
| `env` _[EnvVar](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.31/#envvar-v1-core)_ |  |  |  |
| `hostPort` _integer_ |  |  |  |




#### RedisFollower



RedisFollower interface will have the redis follower configuration



_Appears in:_
- [RedisClusterSpec](#redisclusterspec)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `replicas` _integer_ |  |  |  |
| `redisConfig` _[RedisConfig](#redisconfig)_ |  |  |  |
| `affinity` _[Affinity](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.31/#affinity-v1-core)_ |  |  |  |
| `pdb` _[RedisPodDisruptionBudget](#redispoddisruptionbudget)_ |  |  |  |
| `readinessProbe` _[Probe](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.31/#probe-v1-core)_ |  |  |  |
| `livenessProbe` _[Probe](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.31/#probe-v1-core)_ |  |  |  |
| `tolerations` _[Toleration](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.31/#toleration-v1-core)_ |  |  |  |
| `nodeSelector` _object (keys:string, values:string)_ |  |  |  |
| `topologySpreadConstraints` _[TopologySpreadConstraint](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.31/#topologyspreadconstraint-v1-core) array_ |  |  |  |
| `securityContext` _[SecurityContext](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.31/#securitycontext-v1-core)_ |  |  |  |
| `terminationGracePeriodSeconds` _integer_ |  |  |  |
| `resources` _[ResourceRequirements](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.31/#resourcerequirements-v1-core)_ |  |  |  |


#### RedisLeader



RedisLeader interface will have the redis leader configuration



_Appears in:_
- [RedisClusterSpec](#redisclusterspec)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `replicas` _integer_ |  |  |  |
| `redisConfig` _[RedisConfig](#redisconfig)_ |  |  |  |
| `affinity` _[Affinity](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.31/#affinity-v1-core)_ |  |  |  |
| `pdb` _[RedisPodDisruptionBudget](#redispoddisruptionbudget)_ |  |  |  |
| `readinessProbe` _[Probe](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.31/#probe-v1-core)_ |  |  |  |
| `livenessProbe` _[Probe](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.31/#probe-v1-core)_ |  |  |  |
| `tolerations` _[Toleration](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.31/#toleration-v1-core)_ |  |  |  |
| `nodeSelector` _object (keys:string, values:string)_ |  |  |  |
| `topologySpreadConstraints` _[TopologySpreadConstraint](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.31/#topologyspreadconstraint-v1-core) array_ |  |  |  |
| `securityContext` _[SecurityContext](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.31/#securitycontext-v1-core)_ |  |  |  |
| `terminationGracePeriodSeconds` _integer_ |  |  |  |
| `resources` _[ResourceRequirements](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.31/#resourcerequirements-v1-core)_ |  |  |  |


#### RedisReplication



Redis is the Schema for the redis API





| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `apiVersion` _string_ | `redis.redis.opstreelabs.in/v1beta2` | | |
| `kind` _string_ | `RedisReplication` | | |
| `metadata` _[ObjectMeta](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.31/#objectmeta-v1-meta)_ | Refer to Kubernetes API documentation for fields of `metadata`. |  |  |
| `spec` _[RedisReplicationSpec](#redisreplicationspec)_ |  |  |  |


#### RedisReplicationSpec







_Appears in:_
- [RedisReplication](#redisreplication)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `clusterSize` _integer_ |  |  |  |
| `kubernetesConfig` _[KubernetesConfig](#kubernetesconfig)_ |  |  |  |
| `redisExporter` _[RedisExporter](#redisexporter)_ |  |  |  |
| `redisConfig` _[RedisConfig](#redisconfig)_ |  |  |  |
| `storage` _[Storage](#storage)_ |  |  |  |
| `nodeSelector` _object (keys:string, values:string)_ |  |  |  |
| `podSecurityContext` _[PodSecurityContext](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.31/#podsecuritycontext-v1-core)_ |  |  |  |
| `securityContext` _[SecurityContext](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.31/#securitycontext-v1-core)_ |  |  |  |
| `priorityClassName` _string_ |  |  |  |
| `affinity` _[Affinity](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.31/#affinity-v1-core)_ |  |  |  |
| `tolerations` _[Toleration](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.31/#toleration-v1-core)_ |  |  |  |
| `TLS` _[TLSConfig](#tlsconfig)_ |  |  |  |
| `pdb` _[RedisPodDisruptionBudget](#redispoddisruptionbudget)_ |  |  |  |
| `acl` _[ACLConfig](#aclconfig)_ |  |  |  |
| `readinessProbe` _[Probe](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.31/#probe-v1-core)_ |  |  |  |
| `livenessProbe` _[Probe](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.31/#probe-v1-core)_ |  |  |  |
| `initContainer` _[InitContainer](#initcontainer)_ |  |  |  |
| `sidecars` _[Sidecar](#sidecar)_ |  |  |  |
| `serviceAccountName` _string_ |  |  |  |
| `terminationGracePeriodSeconds` _integer_ |  |  |  |
| `env` _[EnvVar](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.31/#envvar-v1-core)_ |  |  |  |
| `topologySpreadConstraints` _[TopologySpreadConstraint](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.31/#topologyspreadconstraint-v1-core) array_ |  |  |  |
| `hostPort` _integer_ |  |  |  |


#### RedisSentinel



Redis is the Schema for the redis API





| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `apiVersion` _string_ | `redis.redis.opstreelabs.in/v1beta2` | | |
| `kind` _string_ | `RedisSentinel` | | |
| `metadata` _[ObjectMeta](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.31/#objectmeta-v1-meta)_ | Refer to Kubernetes API documentation for fields of `metadata`. |  |  |
| `spec` _[RedisSentinelSpec](#redissentinelspec)_ |  |  |  |


#### RedisSentinelConfig







_Appears in:_
- [RedisSentinelSpec](#redissentinelspec)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `additionalSentinelConfig` _string_ |  |  |  |
| `redisReplicationName` _string_ |  |  |  |
| `redisReplicationPassword` _[EnvVarSource](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.31/#envvarsource-v1-core)_ |  |  |  |
| `masterGroupName` _string_ |  | myMaster |  |
| `redisPort` _string_ |  | 6379 |  |
| `quorum` _string_ |  | 2 |  |
| `parallelSyncs` _string_ |  | 1 |  |
| `failoverTimeout` _string_ |  | 180000 |  |
| `downAfterMilliseconds` _string_ |  | 30000 |  |
| `resolveHostnames` _string_ |  | no |  |
| `announceHostnames` _string_ |  | no |  |


#### RedisSentinelSpec







_Appears in:_
- [RedisSentinel](#redissentinel)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `clusterSize` _integer_ |  | 3 | Minimum: 1 <br /> |
| `kubernetesConfig` _[KubernetesConfig](#kubernetesconfig)_ |  |  |  |
| `redisExporter` _[RedisExporter](#redisexporter)_ |  |  |  |
| `redisSentinelConfig` _[RedisSentinelConfig](#redissentinelconfig)_ |  |  |  |
| `nodeSelector` _object (keys:string, values:string)_ |  |  |  |
| `podSecurityContext` _[PodSecurityContext](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.31/#podsecuritycontext-v1-core)_ |  |  |  |
| `securityContext` _[SecurityContext](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.31/#securitycontext-v1-core)_ |  |  |  |
| `priorityClassName` _string_ |  |  |  |
| `affinity` _[Affinity](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.31/#affinity-v1-core)_ |  |  |  |
| `tolerations` _[Toleration](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.31/#toleration-v1-core)_ |  |  |  |
| `TLS` _[TLSConfig](#tlsconfig)_ |  |  |  |
| `pdb` _[RedisPodDisruptionBudget](#redispoddisruptionbudget)_ |  |  |  |
| `readinessProbe` _[Probe](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.31/#probe-v1-core)_ |  |  |  |
| `livenessProbe` _[Probe](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.31/#probe-v1-core)_ |  |  |  |
| `initContainer` _[InitContainer](#initcontainer)_ |  |  |  |
| `sidecars` _[Sidecar](#sidecar)_ |  |  |  |
| `serviceAccountName` _string_ |  |  |  |
| `terminationGracePeriodSeconds` _integer_ |  |  |  |
| `env` _[EnvVar](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.31/#envvar-v1-core)_ |  |  |  |
| `volumeMount` _[AdditionalVolume](#additionalvolume)_ |  |  |  |
| `topologySpreadConstraints` _[TopologySpreadConstraint](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.31/#topologyspreadconstraint-v1-core) array_ |  |  |  |
| `hostPort` _integer_ |  |  |  |


#### RedisSpec



RedisSpec defines the desired state of Redis



_Appears in:_
- [Redis](#redis)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `kubernetesConfig` _[KubernetesConfig](#kubernetesconfig)_ |  |  |  |
| `redisExporter` _[RedisExporter](#redisexporter)_ |  |  |  |
| `redisConfig` _[RedisConfig](#redisconfig)_ |  |  |  |
| `storage` _[Storage](#storage)_ |  |  |  |
| `nodeSelector` _object (keys:string, values:string)_ |  |  |  |
| `podSecurityContext` _[PodSecurityContext](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.31/#podsecuritycontext-v1-core)_ |  |  |  |
| `securityContext` _[SecurityContext](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.31/#securitycontext-v1-core)_ |  |  |  |
| `priorityClassName` _string_ |  |  |  |
| `affinity` _[Affinity](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.31/#affinity-v1-core)_ |  |  |  |
| `tolerations` _[Toleration](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.31/#toleration-v1-core)_ |  |  |  |
| `TLS` _[TLSConfig](#tlsconfig)_ |  |  |  |
| `acl` _[ACLConfig](#aclconfig)_ |  |  |  |
| `readinessProbe` _[Probe](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.31/#probe-v1-core)_ |  |  |  |
| `livenessProbe` _[Probe](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.31/#probe-v1-core)_ |  |  |  |
| `initContainer` _[InitContainer](#initcontainer)_ |  |  |  |
| `sidecars` _[Sidecar](#sidecar)_ |  |  |  |
| `serviceAccountName` _string_ |  |  |  |
| `terminationGracePeriodSeconds` _integer_ |  |  |  |
| `env` _[EnvVar](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.31/#envvar-v1-core)_ |  |  |  |
| `hostPort` _integer_ |  |  |  |


