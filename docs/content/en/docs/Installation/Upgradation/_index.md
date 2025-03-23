---
title: "Upgrade"
linkTitle: "Upgrade"
weight: 10
date: 2022-11-02T00:19:19Z
description: >
  Instructions for upgrading Redis Operator
---

{{< alert color="info" title="Note" >}}
Whichever approach you take to upgrading Redis Operator, make sure to test it in your development environment
before applying it to production.
{{< /alert >}}

## Upgrading Operator

The following are strategies for safely upgrading Redis Operator from one version to another. They may require adjustment to your particular game architecture but should provide a solid foundation for updating Agones safely.

Ideally we should disable the reconcillation on all the Redis setup managed by operator. To disable the reconcillation, we need to add an annotation on all the `Redis` and `Redis Cluster` object.

For `Redis` standalone object:

```yaml
annotations:
  redis.opstreelabs.in/skip-reconcile: "true"
```

For `RedisCluster` object:

```yaml
annotations:
  rediscluster.opstreelabs.in/skip-reconcile: "true"
```

For `RedisReplication` object:

```yaml
annotations:
  redisReplication.opstreelabs.in/skip-reconcile: "true"
```

For `RedisSentinel` object:

```yaml
annotations:
  redisSentinel.opstreelabs.in/skip-reconcile: "true"
```

### Upgrading with Helm

Helm features capabilities for upgrading to newer versions of Agones without having to uninstall Redis Operator completely.

For details on how to use Helm for upgrades, see the [helm upgrade](https://v2.helm.sh/docs/helm/#helm-upgrade) documentation.

```shell
$ helm install redis-operator ot-helm/redis-operator \
  --namespace ot-operators --version <desired_version>
```

Once upgrading activity is completed, again validate the setup by steps defined in [Validation](../validation).

### Upgrading with YAML

If you installed Redis Operator with [install-operator.sh](https://github.com/OT-CONTAINER-KIT/redis-operator/blob/main/install-operator.sh), we need to update the image tag version inside the [deployment manifest](https://github.com/OT-CONTAINER-KIT/redis-operator/blob/main/config/manager/manager.yaml) of operator and again run the same script.

```yaml
    spec:
      securityContext:
        runAsNonRoot: true
      containers:
        - command:
            - /manager
          args:
            - --leader-elect
            - --zap-log-level=info
          image: quay.io/opstree/redis-operator:<desired_version>
          imagePullPolicy: Always
```

```shell
$ bash install-operator.sh
```
