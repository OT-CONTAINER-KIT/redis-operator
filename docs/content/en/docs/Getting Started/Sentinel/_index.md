---
title: "Sentinel"
linkTitle: "Sentinel"
weight: 20
date: 2023-04-05T19:00:00Z
description: >
  Instructions for setting up Redis sentinel
---

## Architecture

Redis Sentinel is a tool that provides automatic failover and monitoring for Redis nodes. It works by running separate processes that communicate with each other and with Redis nodes to detect failures, elect a new master node, and configure the other nodes to replicate from the new master. Sentinel can also perform additional tasks such as sending notifications and managing configuration changes. Redis Sentinel is a flexible and robust solution for implementing high availability in Redis.

<div align="center" class="mb-0">
    <img src="../../../images/sentinel-redis.png">
</div>

## Helm Installation

In redis sentinel mode, we deploy redis sentinel as a highly available monitoring solution that provides automatic failover, monitoring, and configuration management for Redis instances.

Installation can be easily done via `helm` command:

```shell
$ helm install redis-sentinel ot-helm/redis-sentinel \
  --set redissentinel.clusterSize=3  --namespace ot-operators \
  --set redisSentinelConfig.redisReplicationName="redis-replication"
...
NAME: redis-sentinel
LAST DEPLOYED: Tue Mar 21 23:11:57 2023
NAMESPACE: ot-operators
STATUS: deployed
REVISION: 1
TEST SUITE: None
```

**Note**: The `redisReplicationName` parameter must reference an existing RedisReplication resource. Make sure to deploy a RedisReplication resource first before installing the sentinel.

Verify the sentinel redis setup by kubectl command line.

```shell
$ kubectl get pods -n ot-operators
...
NAME                  READY   STATUS    RESTARTS   AGE
redis-sentinel-0      1/1     Running   0          3m40s
redis-sentinel-1      1/1     Running   0          2m55s
redis-sentinel-2      1/1     Running   0          2m10s
```

## YAML Installation

[Examples](https://github.com/OT-CONTAINER-KIT/redis-operator/tree/main/example/v1beta2) folder has different types of manifests for different scenarios and features. There are these YAML examples present in this directory:

- [additional_config](https://github.com/OT-CONTAINER-KIT/redis-operator/tree/main/example/v1beta2/additional_config)
- [advance_config](https://github.com/OT-CONTAINER-KIT/redis-operator/tree/main/example/v1beta2/advance_config)
- [affinity](https://github.com/OT-CONTAINER-KIT/redis-operator/tree/main/example/v1beta2/affinity)
- [disruption_budget](https://github.com/OT-CONTAINER-KIT/redis-operator/tree/main/example/v1beta2/disruption_budget)
- [external_service](https://github.com/OT-CONTAINER-KIT/redis-operator/tree/main/example/v1beta2/external_service)
- [password_protected](https://github.com/OT-CONTAINER-KIT/redis-operator/tree/main/example/v1beta2/password_protected)
- [private_registry](https://github.com/OT-CONTAINER-KIT/redis-operator/tree/main/example/v1beta2/private_registry)
- [probes](https://github.com/OT-CONTAINER-KIT/redis-operator/tree/main/example/v1beta2/probes)
- [redis_monitoring](https://github.com/OT-CONTAINER-KIT/redis-operator/tree/main/example/v1beta2/redis_monitoring)
- [tls_enabled](https://github.com/OT-CONTAINER-KIT/redis-operator/tree/main/example/v1beta2/tls_enabled)
- [upgrade_strategy](https://github.com/OT-CONTAINER-KIT/redis-operator/tree/main/example/v1beta2/upgrade-strategy)

A basic sample manifest for sentinel redis:

```yaml
---
apiVersion: redis.redis.opstreelabs.in/v1beta2
kind: RedisSentinel
metadata:
  name: redis-sentinel
spec:
  clusterSize: 3
  podSecurityContext:
    runAsUser: 1000
    fsGroup: 1000
  redisSentinelConfig:
    redisReplicationName : redis-replication  # Must match the name of an existing RedisReplication resource
  kubernetesConfig:
    image: quay.io/opstree/redis-sentinel:v7.0.15
    imagePullPolicy: IfNotPresent
    resources:
      requests:
        cpu: 101m
        memory: 128Mi
      limits:
        cpu: 101m
        memory: 128Mi
```

**Important**: The `redisReplicationName` field must reference an existing RedisReplication resource. RedisSentinel monitors and manages Redis instances created by RedisReplication, so ensure you have deployed a RedisReplication resource with the same name before applying this manifest.

The yaml manifest can easily get applied by using `kubectl`.

```shell
$ kubectl apply -f sentinel.yaml
```
