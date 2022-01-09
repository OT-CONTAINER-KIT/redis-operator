# Installation

We have developed our in-house CRD(Custom Resource Definition) to deploy and manage Redis in standalone/cluster mode. So CRD is an amazing feature of Kubernetes which allows us to create our own resources and APIs in Kubernetes. We are not going in the depth of the CRD but soon we will write a blog on CRD as well. Till that time you guys can read about CRD from the [official documentation](https://kubernetes.io/docs/concepts/extend-kubernetes/api-extension/custom-resources/).

The API name which we have created is `redis.redis.opstreelabs.in/v1beta1` and this operator is also published under the OperatorHub catalog.

[https://operatorhub.io/operator/redis-operator](https://operatorhub.io/operator/redis-operator)

So for deploying the redis-operator and setup we need a Kubernetes cluster 1.11+ and that’s it. Let’s deploy the redis operator first.

The easiest way to install a redis operator is using Helm chart. The operator helm chart is developed on the `helm=>3.0.0` version. Also, you can customize the [values.yaml](https://github.com/OT-CONTAINER-KIT/helm-charts/blob/main/charts/redis-operator/values.yaml) file as per the need.

```shell
$ helm repo add ot-helm https://ot-container-kit.github.io/helm-charts/
$ helm upgrade redis-operator ot-helm/redis-operator \
    --install --namespace redis-operator
...
Release "redis-operator" does not exist. Installing it now.
NAME: redis-operator
LAST DEPLOYED: Sun May  2 14:42:23 2021
NAMESPACE: redis-operator
STATUS: deployed
REVISION: 1
TEST SUITE: None
```

Check the state of the pod is running or not.

```shell
$ kubectl get pods -n redis-operator
...
NAME                              READY   STATUS    RESTARTS   AGE
redis-operator-74b6cbf5c5-td8t7   1/1     Running   0          2m11s
```

If you are beginner to Kubernetes and don't want to go inside the complexities of helm, in that case, you can use `kubectl` to install the operator.

```shell
$ kubectl apply -f https://raw.githubusercontent.com/OT-CONTAINER-KIT/redis-operator/master/config/crd/bases/redis.redis.opstreelabs.in_redis.yaml
$ kubectl apply -f https://raw.githubusercontent.com/OT-CONTAINER-KIT/redis-operator/master/config/crd/bases/redis.redis.opstreelabs.in_redisclusters.yaml
$ kubectl apply -f https://raw.githubusercontent.com/OT-CONTAINER-KIT/redis-operator/master/config/manager/manager.yaml
$ kubectl apply -f https://raw.githubusercontent.com/OT-CONTAINER-KIT/redis-operator/master/config/rbac/serviceaccount.yaml
$ kubectl apply -f https://raw.githubusercontent.com/OT-CONTAINER-KIT/redis-operator/master/config/rbac/role.yaml
$ kubectl apply -f https://raw.githubusercontent.com/OT-CONTAINER-KIT/redis-operator/master/config/rbac/role_binding.yaml
```
