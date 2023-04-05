---
title: "Custom Resource Object API"
linkTitle: "Custom Resource Object API"
weight: 10
date: 2022-11-02T00:19:19Z
description: >
  CRD Schema details for Redis and Redis Cluster Reference API
---

# API Reference

## Packages

- [redis.redis.opstreelabs.in/v1beta1](#redisredisopstreelabsinv1beta1)

## redis.redis.opstreelabs.in/v1beta1

Package v1beta1 contains API Schema definitions for the redis v1beta1 API group

### Resource Types

- [Redis](#redis)
- [RedisCluster](#rediscluster)
- [RedisReplication](#redisreplication)
- [RedisSentinel](#redissentinel)

#### ExistingPasswordSecret

ExistingPasswordSecret is the struct to access the existing secret

_Appears in:_

- [KubernetesConfig](#kubernetesconfig)

| Field | Description |
| --- | --- |
| `name` _string_ |  |
| `key` _string_ |  |

#### KubernetesConfig

KubernetesConfig will be the JSON struct for Basic Redis Config

_Appears in:_

- [RedisClusterSpec](#redisclusterspec)
- [RedisSpec](#redisspec)
- [RedisReplicationSpec](#redisreplicationspec)
- [RedisSentinel](#redissentinelspec)

| Field | Description |
| --- | --- |
| `image` _string_ |  |
| `imagePullPolicy` _[PullPolicy](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.22/#pullpolicy-v1-core)_ |  |
| `resources` _[ResourceRequirements](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.22/#resourcerequirements-v1-core)_ |  |
| `redisSecret` _[ExistingPasswordSecret](#existingpasswordsecret)_ |  |
| `imagePullSecrets` _[LocalObjectReference](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.22/#localobjectreference-v1-core)_ |  |
| `updateStrategy` _[StatefulSetUpdateStrategy](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.22/#statefulsetupdatestrategy-v1-apps)_ |  |

#### Probe

Probe is a interface for ReadinessProbe and LivenessProbe

_Appears in:_

- [RedisFollower](#redisfollower)
- [RedisLeader](#redisleader)
- [RedisSpec](#redisspec)
- [RedisReplicationSpec](#redisreplicationspec)
- [RedisSentinel](#redissentinelspec)

| Field | Description |
| --- | --- |
| `initialDelaySeconds` _integer_ |  |
| `timeoutSeconds` _integer_ |  |
| `periodSeconds` _integer_ |  |
| `successThreshold` _integer_ |  |
| `failureThreshold` _integer_ |  |

#### Redis

Redis is the Schema for the redis API

| Field | Description |
| --- | --- |
| `apiVersion` _string_ | `redis.redis.opstreelabs.in/v1beta1`
| `kind` _string_ | `Redis`
| `metadata` _[ObjectMeta](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.22/#objectmeta-v1-meta)_ | Refer to Kubernetes API documentation for fields of `metadata`. |
| `spec` _[RedisSpec](#redisspec)_ |  |

#### RedisCluster

RedisCluster is the Schema for the redisclusters API

| Field | Description |
| --- | --- |
| `apiVersion` _string_ | `redis.redis.opstreelabs.in/v1beta1`
| `kind` _string_ | `RedisCluster`
| `metadata` _[ObjectMeta](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.22/#objectmeta-v1-meta)_ | Refer to Kubernetes API documentation for fields of `metadata`. |
| `spec` _[RedisClusterSpec](#redisclusterspec)_ |  |

#### RedisReplication

RedisReplication is the Schema for the redisreplication API

| Field | Description |
| --- | --- |
| `apiVersion` _string_ | `redis.redis.opstreelabs.in/v1beta1`
| `kind` _string_ | `RedisReplication`
| `metadata` _[ObjectMeta](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.22/#objectmeta-v1-meta)_ | Refer to Kubernetes API documentation for fields of `metadata`. |
| `spec` _[RedisReplicationSpec](#redisreplicationspec)_ |  |

#### RedisSentinel

RedisSentinel is the Schema for the redissentinel API

| Field | Description |
| --- | --- |
| `apiVersion` _string_ | `redis.redis.opstreelabs.in/v1beta1`
| `kind` _string_ | `RedisSentinel`
| `metadata` _[ObjectMeta](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.22/#objectmeta-v1-meta)_ | Refer to Kubernetes API documentation for fields of `metadata`. |
| `spec` _[RedisSentinelSpec](#redissentinelspec)_ |  |

#### RedisClusterSpec

RedisClusterSpec defines the desired state of RedisCluster

_Appears in:_

- [RedisCluster](#rediscluster)

| Field | Description |
| --- | --- |
| `clusterSize` _integer_ |  |
| `kubernetesConfig` _[KubernetesConfig](#kubernetesconfig)_ |  |
| `clusterVersion` _string_ |  |
| `redisLeader` _[RedisLeader](#redisleader)_ |  |
| `redisFollower` _[RedisFollower](#redisfollower)_ |  |
| `redisExporter` _[RedisExporter](#redisexporter)_ |  |
| `storage` _[Storage](#storage)_ |  |
| `nodeSelector` _object (keys:string, values:string)_ |  |
| `securityContext` _[PodSecurityContext](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.22/#podsecuritycontext-v1-core)_ |  |
| `priorityClassName` _string_ |  |
| `tolerations` _[Toleration](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.22/#toleration-v1-core)_ |  |
| `resources` _[ResourceRequirements](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.22/#resourcerequirements-v1-core)_ |  |
| `TLS` _[TLSConfig](#tlsconfig)_ |  |
| `sidecars` _[Sidecar](#sidecar)_ |  |
| `serviceAccountName` _string_ |  |
| `persistenceEnabled` _boolean_ |  |

#### RedisSpec

RedisSpec defines the desired state of Redis

_Appears in:_

- [Redis](#redis)

| Field | Description |
| --- | --- |
| `kubernetesConfig` _[KubernetesConfig](#kubernetesconfig)_ |  |
| `redisExporter` _[RedisExporter](#redisexporter)_ |  |
| `redisConfig` _[RedisConfig](#redisconfig)_ |  |
| `storage` _[Storage](#storage)_ |  |
| `nodeSelector` _object (keys:string, values:string)_ |  |
| `securityContext` _[PodSecurityContext](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.22/#podsecuritycontext-v1-core)_ |  |
| `priorityClassName` _string_ |  |
| `affinity` _[Affinity](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.22/#affinity-v1-core)_ |  |
| `tolerations` _[Toleration](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.22/#toleration-v1-core)_ |  |
| `TLS` _[TLSConfig](#tlsconfig)_ |  |
| `readinessProbe` _[Probe](#probe)_ |  |
| `livenessProbe` _[Probe](#probe)_ |  |
| `sidecars` _[Sidecar](#sidecar)_ |  |
| `serviceAccountName` _string_ |  |

#### RedisReplicationSpec

RedisReplicationSpec defines the desired state of RedisReplication

_Appears in:_

- [RedisReplication](#redisreplication)

| Field | Description |
| --- | --- |
| `clusterSize` _integer_ |  |
| `kubernetesConfig` _[KubernetesConfig](#kubernetesconfig)_ |  |
| `redisExporter` _[RedisExporter](#redisexporter)_ |  |
| `redisConfig` _[RedisConfig](#redisconfig)_ |  |
| `storage` _[Storage](#storage)_ |  |
| `nodeSelector` _object (keys:string, values:string)_ |  |
| `securityContext` _[PodSecurityContext](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.22/#podsecuritycontext-v1-core)_ |  |
| `priorityClassName` _string_ |  |
| `affinity` _[Affinity](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.22/#affinity-v1-core)_ |  |
| `tolerations` _[Toleration](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.22/#toleration-v1-core)_ |  |
| `TLS` _[TLSConfig](#tlsconfig)_ |  |
| `readinessProbe` _[Probe](#probe)_ |  |
| `livenessProbe` _[Probe](#probe)_ |  |
| `sidecars` _[Sidecar](#sidecar)_ |  |
| `serviceAccountName` _string_ |  |

#### RedisSentinelSpec

RedisSentinelSpec defines the desired state of RedisSentinel

_Appears in:_

- [RedisSentinel](#redissentinel)

| Field | Description |
| --- | --- |
| `clusterSize` _integer_ |  |
| `kubernetesConfig` _[KubernetesConfig](#kubernetesconfig)_ |  |
| `redisSentinelConfig` _[RedisSentinelConfig](#redissentinelconfig)_ |  |
| `nodeSelector` _object (keys:string, values:string)_ |  |
| `securityContext` _[PodSecurityContext](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.22/#podsecuritycontext-v1-core)_ |  |
| `priorityClassName` _string_ |  |
| `affinity` _[Affinity](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.22/#affinity-v1-core)_ |  |
| `tolerations` _[Toleration](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.22/#toleration-v1-core)_ |  |
| `TLS` _[TLSConfig](#tlsconfig)_ |  |
| `readinessProbe` _[Probe](#probe)_ |  |
| `livenessProbe` _[Probe](#probe)_ |  |
| `sidecars` _[Sidecar](#sidecar)_ |  |
| `serviceAccountName` _string_ |  |

#### RedisConfig

RedisConfig defines the external configuration of Redis

_Appears in:_

- [RedisFollower](#redisfollower)
- [RedisLeader](#redisleader)
- [RedisSpec](#redisspec)
- [RedisReplicationSpec](#redisreplicationspec)

| Field | Description |
| --- | --- |
| `additionalRedisConfig` _string_ |  |

#### RedisSentinelConfig

RedisSentinelConfig defines the external configuration of RedisSentinel

_Appears in:_

- [RedisSentinelSpec](#redissentinelspec)

| Field | Description |
| --- | --- |
| `additionalRedisConfig` _string_ |  |
| `masterGroupName` _string_ |  |
| `redisPort` _string_ |  |
| `quorum` _string_ |  |
| `parallelSyncs` _string_ |  |
| `failoverTimeout` _string_ |  |
| `downAfterMilliseconds` _string_ |  |

#### RedisExporter

RedisExporter interface will have the information for redis exporter related stuff

_Appears in:_

- [RedisClusterSpec](#redisclusterspec)
- [RedisSpec](#redisspec)
- [RedisReplicationSpec](#redisreplicationspec)

| Field | Description |
| --- | --- |
| `enabled` _boolean_ |  |
| `image` _string_ |  |
| `resources` _[ResourceRequirements](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.22/#resourcerequirements-v1-core)_ |  |
| `imagePullPolicy` _[PullPolicy](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.22/#pullpolicy-v1-core)_ |  |
| `env` _[EnvVar](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.22/#envvar-v1-core)_ |  |

#### RedisFollower

RedisFollower interface will have the redis follower configuration

_Appears in:_

- [RedisClusterSpec](#redisclusterspec)

| Field | Description |
| --- | --- |
| `replicas` _integer_ |  |
| `redisConfig` _[RedisConfig](#redisconfig)_ |  |
| `affinity` _[Affinity](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.22/#affinity-v1-core)_ |  |
| `pdb` _[RedisPodDisruptionBudget](#redispoddisruptionbudget)_ |  |
| `readinessProbe` _[Probe](#probe)_ |  |
| `livenessProbe` _[Probe](#probe)_ |  |

#### RedisLeader

RedisLeader interface will have the redis leader configuration

_Appears in:_

- [RedisClusterSpec](#redisclusterspec)

| Field | Description |
| --- | --- |
| `replicas` _integer_ |  |
| `redisConfig` _[RedisConfig](#redisconfig)_ |  |
| `affinity` _[Affinity](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.22/#affinity-v1-core)_ |  |
| `pdb` _[RedisPodDisruptionBudget](#redispoddisruptionbudget)_ |  |
| `readinessProbe` _[Probe](#probe)_ |  |
| `livenessProbe` _[Probe](#probe)_ |  |

#### RedisPodDisruptionBudget

RedisPodDisruptionBudget configure a PodDisruptionBudget on the resource (leader/follower)

_Appears in:_

- [RedisFollower](#redisfollower)
- [RedisLeader](#redisleader)
- [RedisReplication](#redisreplicationspec)
- [RedisSentinel](#redissentinelspec)
  
| Field | Description |
| --- | --- |
| `enabled` _boolean_ |  |
| `minAvailable` _integer_ |  |
| `maxUnavailable` _integer_ |  |

#### Sidecar

Sidecar for each Redis pods

_Appears in:_

- [RedisClusterSpec](#redisclusterspec)
- [RedisSpec](#redisspec)
- [RedisReplicationSpec](#redisreplicationspec)
- [RedisSentinel](#redissentinelspec)

| Field | Description |
| --- | --- |
| `name` _string_ |  |
| `image` _string_ |  |
| `imagePullPolicy` _[PullPolicy](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.22/#pullpolicy-v1-core)_ |  |
| `resources` _[ResourceRequirements](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.22/#resourcerequirements-v1-core)_ |  |
| `env` _[EnvVar](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.22/#envvar-v1-core)_ |  |

#### Storage

Storage is the inteface to add pvc and pv support in redis

_Appears in:_

- [RedisClusterSpec](#redisclusterspec)
- [RedisSpec](#redisspec)
- [RedisReplicationSpec](#redisreplicationspec)

| Field | Description |
| --- | --- |
| `volumeClaimTemplate` _[PersistentVolumeClaim](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.22/#persistentvolumeclaim-v1-core)_ |  |

#### TLSConfig

TLS Configuration for redis instances

_Appears in:_

- [RedisClusterSpec](#redisclusterspec)
- [RedisSpec](#redisspec)
- [RedisReplicationSpec](#redisreplicationspec)
- [RedisSentinel](#redissentinelspec)

| Field | Description |
| --- | --- |
| `ca` _string_ |  |
| `cert` _string_ |  |
| `key` _string_ |  |
| `secret` _[SecretVolumeSource](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.22/#secretvolumesource-v1-core)_ | Reference to secret which contains the certificates |
