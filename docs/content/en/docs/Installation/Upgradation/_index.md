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

## Version-Specific Migration Guides

Before upgrading, check if there is a migration guide for your version jump. These guides cover breaking changes and required manual steps.

| From | To | Guide |
|------|----|-------|
| v0.23.0 | v0.24.0 | [Migration Guide](https://github.com/OT-CONTAINER-KIT/redis-operator/blob/main/upgrade/migartion-v0.23.0-to-v0.24.0.md) |

## Upgrading Operator

The following are strategies for safely upgrading Redis Operator from one version to another. They may require adjustment to your particular architecture but should provide a solid foundation for updating safely.

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

## Preserving existing PVC names

The name of the volume claim template in the generated StatefulSet defaults to the StatefulSet name, which gives PVCs such as `redis-redis-0` for a standalone `Redis` named `redis`, and `redis-cluster-leader-redis-cluster-leader-0` for a `RedisCluster`. If an older operator version created your PVCs under a different template name, a new StatefulSet will not adopt them and Redis starts on an empty volume.

Set `OPERATOR_STS_PVC_TEMPLATE_NAME` on the operator deployment to keep the previous template name:

```yaml
    spec:
      containers:
        - command:
            - /manager
          env:
            - name: OPERATOR_STS_PVC_TEMPLATE_NAME
              value: "<existing template name>"
```

The variable is global to the operator, so every managed setup uses the same template name. Leave it unset unless you are carrying PVCs over from an older layout.

### Upgrading with Helm

Helm features capabilities for upgrading to newer versions without having to uninstall Redis Operator completely.

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
