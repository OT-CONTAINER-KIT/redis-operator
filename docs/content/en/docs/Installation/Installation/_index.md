---
title: "Installation"
linkTitle: "Installation"
weight: 10
date: 2022-11-02T00:19:19Z
description: >
  Instructions for installation of Redis Operator.
---

Redis Operator is developed as CRD(Custom Resource Definition) to deploy and manage Redis in standalone/cluster mode. So CRD is an amazing feature of Kubernetes which allows us to create our own resources and APIs in Kubernetes. For further information about CRD, please go through the [official documentation](https://kubernetes.io/docs/concepts/extend-kubernetes/api-extension/custom-resources/).

There are four different Objects available under `redis.redis.opstreelabs.in/v1beta2`:

- Redis
- Redis Cluster
- Redis Replication
- Redis Sentinel

For [OperatorHub](https://operatorhub.io) installation:

https://operatorhub.io/operator/redis-operator

So for deploying the redis-operator and setup we need a Kubernetes cluster 1.18+ and that's it. Let's deploy the redis operator first.

The easiest way to install a redis operator is using Helm chart. The operator helm chart is developed on the `helm=>3.0.0` version. The [values.yaml](https://github.com/OT-CONTAINER-KIT/helm-charts/blob/main/charts/redis-operator/values.yaml) can be modified.

## Helm Installation

```shell
$ helm repo add ot-helm https://ot-container-kit.github.io/helm-charts/
$ helm install redis-operator ot-helm/redis-operator --namespace ot-operators
...
Release "redis-operator" does not exist. Installing it now.
NAME:          redis-operator
LAST DEPLOYED: Sun May  2 14:42:23 2021
NAMESPACE:     ot-operators
STATUS:        deployed
REVISION:      1
TEST SUITE:    None
```

## YAML Installation

{{< alert title="Warning" color="warning">}}
YAML installation is not a recommended way for installation, this can only be used for development practices only.
{{< /alert >}}

```shell
$ bash install-operator.sh
...
customresourcedefinition.apiextensions.k8s.io/redis.redis.redis.opstreelabs.in created
customresourcedefinition.apiextensions.k8s.io/redisclusters.redis.redis.opstreelabs.in created
namespace/ot-operators created
deployment.apps/redis-operator created
serviceaccount/redis-operator created
clusterrole.rbac.authorization.k8s.io/redis-operator created
clusterrolebinding.rbac.authorization.k8s.io/redis-operator created
```
