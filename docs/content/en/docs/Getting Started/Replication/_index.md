---
title: "Replication"
linkTitle: "Replication"
weight: 10
date: 2023-04-05T19:00:00Z
description: >
  Instructions for setting up Redis Replication
---

## Architecture

Redis is an in-memory key-value store that can be used as a database, cache, and message broker. Redis replication is the process of synchronizing data from a Redis leader node to one or more Redis follower nodes.

In Redis replication, the leader node is responsible for receiving write requests and propagating the changes to one or more follower nodes. The follower nodes receive the data changes from the leader and apply them locally, thereby creating a replica of the leader's dataset.

Redis replication uses asynchronous replication, which means that the leader node does not wait for the follower nodes to apply the changes before sending new updates. Instead, the follower nodes catch up with the leader node as soon as they can, based on the available network bandwidth and the capacity of the hardware.

Redis replication is a powerful feature that enhances the durability and scalability of Redis applications. By using Redis replication, you can distribute the workload across multiple nodes, improve the read performance, and ensure data availability in case of a node failure.

<div align="center" class="mb-0">
    <img src="../../../images/replication-redis.png">
</div>

> **Note:** By using Redis Sentinel, you can ensure that your Redis Replication remains available even if one or more nodes go down, improving the resilience and reliability of your application.

## Helm Installation

For redis replication setup we can use `helm` command with the reference of replication helm chart and additional properties:

```shell
$ helm install redis-replication ot-helm/redis-replication \
  --set redisreplication.clusterSize=3 --namespace ot-operators
...
NAME: redis-replication
LAST DEPLOYED: Tue Mar 21 22:47:44 2023
NAMESPACE: ot-operators
STATUS: deployed
REVISION: 1
TEST SUITE: None
```

Verify the replication-cluster by checking the pod status of pods.

```shell
$ kubectl get pods -n ot-operators
...
NAME                  READY   STATUS    RESTARTS   AGE
redis-replication-0   1/1     Running   0          2m23s
redis-replication-1   1/1     Running   0          99s
redis-replication-2   1/1     Running   0          59s
```

If all the pods are in the running, then we can check the health of the redis replication-cluster by using redis-cli. Here by default the 0th index redis pod is promoted to master.

## YAML Installation

[Examples](https://github.com/OT-CONTAINER-KIT/redis-operator/tree/master/example/v1beta2) folder has different types of manifests for different scenarios and features. There are these YAML examples present in this directory:

- [additional_config](https://github.com/OT-CONTAINER-KIT/redis-operator/tree/master/example/v1beta2/additional_config)
- [advance_config](https://github.com/OT-CONTAINER-KIT/redis-operator/tree/master/example/v1beta2/advance_config)
- [affinity](https://github.com/OT-CONTAINER-KIT/redis-operator/tree/master/example/v1beta2/affinity)
- [disruption_budget](https://github.com/OT-CONTAINER-KIT/redis-operator/tree/master/example/v1beta2/disruption_budget)
- [external_service](https://github.com/OT-CONTAINER-KIT/redis-operator/tree/master/example/v1beta2/external_service)
- [password_protected](https://github.com/OT-CONTAINER-KIT/redis-operator/tree/master/example/v1beta2/password_protected)
- [private_registry](https://github.com/OT-CONTAINER-KIT/redis-operator/tree/master/example/v1beta2/private_registry)
- [probes](https://github.com/OT-CONTAINER-KIT/redis-operator/tree/master/example/v1beta2/probes)
- [redis_monitoring](https://github.com/OT-CONTAINER-KIT/redis-operator/tree/master/example/v1beta2/redis_monitoring)
- [tls_enabled](https://github.com/OT-CONTAINER-KIT/redis-operator/tree/master/example/v1beta2/tls_enabled)
- [upgrade_strategy](https://github.com/OT-CONTAINER-KIT/redis-operator/tree/master/example/v1beta2/upgrade-strategy)

A sample manifest for deploying redis replication-cluster:

```yaml
---
apiVersion: redis.redis.opstreelabs.in/v1beta2
kind: RedisReplication
metadata:
  name:  redis-replication
spec:
  clusterSize: 3
  securityContext:
    runAsUser: 1000
    fsGroup: 1000
  kubernetesConfig: 
    image: quay.io/opstree/redis:v7.0.15
    imagePullPolicy: IfNotPresent
    resources:
      requests:
        cpu: 101m
        memory: 128Mi
      limits:
        cpu: 101m
        memory: 128Mi
  storage:
    volumeClaimTemplate:
      spec:
        # storageClassName: standard
        accessModes: ["ReadWriteOnce"]
        resources:
          requests:
            storage: 1Gi
```

The yaml manifest can easily get applied by using `kubectl`.

```shell
kubectl apply -f replication.yaml
```
