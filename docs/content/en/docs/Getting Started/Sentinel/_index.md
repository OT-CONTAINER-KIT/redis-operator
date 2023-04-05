---
title: "Sentinel"
linkTitle: "sentinel"
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

In redis sentinel mode, we deploy redis as a single StatefulSet pod that means ease of setup but no complexity, no high availability, and no resilience.

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

[Examples](https://github.com/OT-CONTAINER-KIT/redis-operator/tree/master/example) folder has different types of manifests for different scenarios and features. There are these YAML examples present in this directory:

- [additional_config](https://github.com/OT-CONTAINER-KIT/redis-operator/tree/master/example/additional_config)
- [advance_config](https://github.com/OT-CONTAINER-KIT/redis-operator/tree/master/example/advance_config)
- [affinity](https://github.com/OT-CONTAINER-KIT/redis-operator/tree/master/example/affinity)
- [disruption_budget](https://github.com/OT-CONTAINER-KIT/redis-operator/tree/master/example/disruption_budget)
- [external_service](https://github.com/OT-CONTAINER-KIT/redis-operator/tree/master/example/external_service)
- [password_protected](https://github.com/OT-CONTAINER-KIT/redis-operator/tree/master/example/password_protected)
- [private_registry](https://github.com/OT-CONTAINER-KIT/redis-operator/tree/master/example/private_registry)
- [probes](https://github.com/OT-CONTAINER-KIT/redis-operator/tree/master/example/probes)
- [redis_monitoring](https://github.com/OT-CONTAINER-KIT/redis-operator/tree/master/example/redis_monitoring)
- [tls_enabled](https://github.com/OT-CONTAINER-KIT/redis-operator/tree/master/example/tls_enabled)
- [upgrade_strategy](https://github.com/OT-CONTAINER-KIT/redis-operator/tree/master/example/upgrade-strategy)

A basic sample manifest for sentinel redis:

```yaml
---
apiVersion: redis.redis.opstreelabs.in/v1beta1
kind: RedisSentinel
metadata:
  name: redis-sentinel
spec:
  clusterSize: 3
  securityContext:
    runAsUser: 1000
    fsGroup: 1000
  redisSentinelConfig: 
    redisReplicationName : redis-replication
  kubernetesConfig:
    image: quay.io/opstree/redis-sentinel:v7.0.7 
    imagePullPolicy: IfNotPresent
    resources:
      requests:
        cpu: 101m
        memory: 128Mi
      limits:
        cpu: 101m
        memory: 128Mi
```

The yaml manifest can easily get applied by using `kubectl`.

```shell
$ kubectl apply -f sentinel.yaml
```
