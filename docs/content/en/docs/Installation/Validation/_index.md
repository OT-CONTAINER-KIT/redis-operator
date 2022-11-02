---
title: "Installation Validation"
linkTitle: "Installation Validation"
weight: 10
date: 2022-11-02T00:19:19Z
description: >
  Instructions for validating installation of Operator
---

To confirm Redis Operator is up and running, run the following command:

```shell
$ kubectl describe --namespace ot-operators pods
```

It should describe one pod created in the ot-operators namespace, with no error messages or status. All Conditions sections should look like this:

```yaml
Conditions:
  Type              Status
  Initialized       True
  Ready             True
  ContainersReady   True
  PodScheduled      True
```

The operator pod should be in a RUNNING state:

```shell
$ kubectl get pods -n ot-operators
...
NAME                              READY   STATUS    RESTARTS   AGE
redis-operator-74b6cbf5c5-td8t7   1/1     Running   0          2m11s
```

That’s it!

Now with Redis Operator installed, you can utilise its [Custom Resource Definitions](https://kubernetes.io/docs/concepts/api-extension/custom-resources/) to create resources of type Redis, RedisCluster and more!


