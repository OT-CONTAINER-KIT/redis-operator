---
title: "API Reference Documentation"
linkTitle: "API Docs"
weight: 10
date: 2025-01-27
description: >
  Complete API reference documentation for Redis Operator CRDs
---

# API Reference

## Packages
- [redis.redis.opstreelabs.in/v1beta2](#redisredisopstreelabsinv1beta2)


## redis.redis.opstreelabs.in/v1beta2

Package v1beta2 contains common types used by Redis Operator APIs.
These types are shared across different Redis resource types.


### Resource Types
- [Redis](#redis)
- [RedisCluster](#rediscluster)
- [RedisReplication](#redisreplication)
- [RedisSentinel](#redissentinel)



#### ACLConfig







_Appears in:_
- [RedisClusterSpec](#redisclusterspec)
- [RedisReplicationSpec](#redisreplicationspec)
- [RedisSpec](#redisspec)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `secret` _[SecretVolumeSource](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.31/#secretvolumesource-v1-core)_ |  |  |  |


#### AdditionalVolume



Additional Volume is provided by user that is mounted on the pods



_Appears in:_
- [ClusterStorage](#clusterstorage)
- [RedisSentinelSpec](#redissentinelspec)
- [Storage](#storage)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `volume` _[Volume](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.31/#volume-v1-core) array_ |  |  |  |
| `mountPath` _[VolumeMount](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.31/#volumemount-v1-core) array_ |  |  |  |


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


#### ExistingPasswordSecret



ExistingPasswordSecret is the struct to access the existing secret



_Appears in:_
- [KubernetesConfig](#kubernetesconfig)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `name` _string_ |  |  |  |
| `key` _string_ |  |  |  |


#### InitContainer



InitContainer for each Redis pods



_Appears in:_
- [RedisClusterSpec](#redisclusterspec)
- [RedisReplicationSpec](#redisreplicationspec)
- [RedisSentinelSpec](#redissentinelspec)
- [RedisSpec](#redisspec)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `enabled` _boolean_ |  |  |  |
| `image` _string_ |  |  |  |
| `imagePullPolicy` _[PullPolicy](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.31/#pullpolicy-v1-core)_ |  |  |  |
| `resources` _[ResourceRequirements](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.31/#resourcerequirements-v1-core)_ |  |  |  |
| `env` _[EnvVar](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.31/#envvar-v1-core)_ |  |  |  |
| `command` _string array_ |  |  |  |
| `args` _string array_ |  |  |  |
| `securityContext` _[SecurityContext](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.31/#securitycontext-v1-core)_ |  |  |  |


#### KubernetesConfig



KubernetesConfig will be the JSON struct for Basic Redis Config



_Appears in:_
- [RedisClusterSpec](#redisclusterspec)
- [RedisReplicationSpec](#redisreplicationspec)
- [RedisSentinelSpec](#redissentinelspec)
- [RedisSpec](#redisspec)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `image` _string_ |  |  |  |
| `imagePullPolicy` _[PullPolicy](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.31/#pullpolicy-v1-core)_ |  |  |  |
| `resources` _[ResourceRequirements](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.31/#resourcerequirements-v1-core)_ |  |  |  |
| `redisSecret` _[ExistingPasswordSecret](#existingpasswordsecret)_ |  |  |  |
| `imagePullSecrets` _[LocalObjectReference](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.31/#localobjectreference-v1-core)_ |  |  |  |
| `updateStrategy` _[StatefulSetUpdateStrategy](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.31/#statefulsetupdatestrategy-v1-apps)_ |  |  |  |
| `persistentVolumeClaimRetentionPolicy` _[StatefulSetPersistentVolumeClaimRetentionPolicy](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.31/#statefulsetpersistentvolumeclaimretentionpolicy-v1-apps)_ |  |  |  |
| `service` _[ServiceConfig](#serviceconfig)_ |  |  |  |
| `ignoreAnnotations` _string array_ |  |  |  |
| `minReadySeconds` _integer_ |  |  |  |


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
| `clusterSize` _integer_ | ClusterSize defines the default number of replicas for both leader and follower when not explicitly set |  |  |
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




#### RedisConfig



RedisConfig defines the external configuration of Redis



_Appears in:_
- [RedisClusterSpec](#redisclusterspec)
- [RedisFollower](#redisfollower)
- [RedisFollower](#redisfollower)
- [RedisLeader](#redisleader)
- [RedisLeader](#redisleader)
- [RedisReplicationSpec](#redisreplicationspec)
- [RedisSpec](#redisspec)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `maxMemoryPercentOfLimit` _integer_ | MaxMemoryPercentOfLimit is the percentage of redis container memory limit to be used as maxmemory. |  | Maximum: 100 <br />Minimum: 1 <br /> |
| `dynamicConfig` _string array_ |  |  |  |
| `additionalRedisConfig` _string_ |  |  |  |


#### RedisExporter



RedisExporter interface will have the information for redis exporter related stuff



_Appears in:_
- [RedisClusterSpec](#redisclusterspec)
- [RedisReplicationSpec](#redisreplicationspec)
- [RedisSentinelSpec](#redissentinelspec)
- [RedisSpec](#redisspec)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `enabled` _boolean_ |  |  |  |
| `port` _integer_ |  | 9121 |  |
| `image` _string_ |  |  |  |
| `resources` _[ResourceRequirements](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.31/#resourcerequirements-v1-core)_ |  |  |  |
| `imagePullPolicy` _[PullPolicy](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.31/#pullpolicy-v1-core)_ |  |  |  |
| `env` _[EnvVar](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.31/#envvar-v1-core)_ |  |  |  |
| `securityContext` _[SecurityContext](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.31/#securitycontext-v1-core)_ |  |  |  |


#### RedisFollower



RedisFollower interface will have the redis follower configuration



_Appears in:_
- [RedisClusterSpec](#redisclusterspec)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `replicas` _integer_ | Replicas overrides clusterSize for follower nodes count. If not set, uses clusterSize value |  |  |
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
| `replicas` _integer_ | Replicas overrides clusterSize for leader nodes count. If not set, uses clusterSize value |  |  |
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


#### RedisPodDisruptionBudget



RedisPodDisruptionBudget configure a PodDisruptionBudget on the resource (leader/follower)



_Appears in:_
- [RedisFollower](#redisfollower)
- [RedisFollower](#redisfollower)
- [RedisLeader](#redisleader)
- [RedisLeader](#redisleader)
- [RedisReplicationSpec](#redisreplicationspec)
- [RedisSentinelSpec](#redissentinelspec)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `enabled` _boolean_ |  |  |  |
| `minAvailable` _integer_ |  |  |  |
| `maxUnavailable` _integer_ |  |  |  |


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


#### Service



Service is the struct to define the service type and its annotations



_Appears in:_
- [ServiceConfig](#serviceconfig)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `type` _string_ |  | ClusterIP | Enum: [LoadBalancer NodePort ClusterIP] <br /> |
| `additionalAnnotations` _object (keys:string, values:string)_ |  |  |  |
| `includeBusPort` _boolean_ | IncludeBusPort when set to true, it will add bus port to the service, such as 16379.<br />This field is only used for Redis cluster mode. |  |  |
| `enabled` _boolean_ |  | true |  |


#### ServiceConfig



ServiceConfig define the type of service to be created and its annotations



_Appears in:_
- [KubernetesConfig](#kubernetesconfig)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `serviceType` _string_ |  |  | Enum: [LoadBalancer NodePort ClusterIP] <br /> |
| `annotations` _object (keys:string, values:string)_ |  |  |  |
| `includeBusPort` _boolean_ | IncludeBusPort when set to true, it will add bus port to the service, such as 16379.<br />This field is only used for Redis cluster mode. |  |  |
| `headless` _[Service](#service)_ | Headless config for which suffix is -headless service |  |  |
| `additional` _[Service](#service)_ | Additional config for which suffix is -additional service |  |  |


#### Sidecar



Sidecar for each Redis pods



_Appears in:_
- [RedisClusterSpec](#redisclusterspec)
- [RedisReplicationSpec](#redisreplicationspec)
- [RedisSentinelSpec](#redissentinelspec)
- [RedisSpec](#redisspec)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `name` _string_ |  |  |  |
| `image` _string_ |  |  |  |
| `imagePullPolicy` _[PullPolicy](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.31/#pullpolicy-v1-core)_ |  |  |  |
| `resources` _[ResourceRequirements](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.31/#resourcerequirements-v1-core)_ |  |  |  |
| `env` _[EnvVar](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.31/#envvar-v1-core)_ |  |  |  |
| `mountPath` _[VolumeMount](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.31/#volumemount-v1-core)_ |  |  |  |
| `command` _string array_ |  |  |  |
| `ports` _[ContainerPort](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.31/#containerport-v1-core)_ |  |  |  |
| `securityContext` _[SecurityContext](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.31/#securitycontext-v1-core)_ |  |  |  |


#### Storage



Storage is the inteface to add pvc and pv support in redis



_Appears in:_
- [ClusterStorage](#clusterstorage)
- [RedisReplicationSpec](#redisreplicationspec)
- [RedisSpec](#redisspec)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `keepAfterDelete` _boolean_ |  |  |  |
| `volumeClaimTemplate` _[PersistentVolumeClaim](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.31/#persistentvolumeclaim-v1-core)_ |  |  |  |
| `volumeMount` _[AdditionalVolume](#additionalvolume)_ |  |  |  |


#### TLSConfig



TLS Configuration for redis instances



_Appears in:_
- [RedisClusterSpec](#redisclusterspec)
- [RedisReplicationSpec](#redisreplicationspec)
- [RedisSentinelSpec](#redissentinelspec)
- [RedisSpec](#redisspec)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `ca` _string_ |  |  |  |
| `cert` _string_ |  |  |  |
| `key` _string_ |  |  |  |
| `secret` _[SecretVolumeSource](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.31/#secretvolumesource-v1-core)_ | Reference to secret which contains the certificates |  |  |


