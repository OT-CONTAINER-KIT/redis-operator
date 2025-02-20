---
title: "Standalone"
linkTitle: "Standalone"
weight: 10
date: 2022-11-02T00:19:19Z
description: >
  Instructions for setting up Redis standalone
---

## Architecture

Redis standalone is a single process-based redis pod that can manage your keys inside it. Multiple applications can consume this redis with a Kubernetes endpoint or service. Since this standalone setup is running inside Kubernetes, the auto-heal feature will be automatically part of it. The only drawback of a standalone setup is that it doesn't stand on the high availability principle.

<div align="center" class="mb-0">
    <img src="../../../images/standalone-redis.png">
</div>

## Helm Installation

In redis standalone mode, we deploy redis as a single StatefulSet pod that means ease of setup but no complexity, no high availability, and no resilience.

Installation can be easily done via `helm` command:

```shell
$ helm install redis ot-helm/redis --namespace ot-operators
...
Release "redis" does not exist. Installing it now.
NAME:          redis
LAST DEPLOYED: Sun May  2 15:59:48 2021
NAMESPACE:     ot-operators
STATUS:        deployed
REVISION:      1
TEST SUITE:    None
```

Verify the standalone redis setup by kubectl command line.

```shell
$ kubectl get pods -n ot-operators
...
NAME                              READY   STATUS    RESTARTS   AGE
redis-0                           1/1     Running   0          56s
```

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

A basic sample manifest for standalone redis:

```yaml
---
apiVersion: redis.redis.opstreelabs.in/v1beta2
kind: Redis
metadata:
  name: redis-standalone
spec:
  kubernetesConfig:
    image: quay.io/opstree/redis:v7.0.15
    imagePullPolicy: IfNotPresent
  storage:
    volumeClaimTemplate:
      spec:
        # storageClassName: standard
        accessModes: ["ReadWriteOnce"]
        resources:
          requests:
            storage: 1Gi
  securityContext:
    runAsUser: 1000
    fsGroup: 1000
```

The yaml manifest can easily get applied by using `kubectl`.

```shell
$ kubectl apply -f standalone.yaml
```
