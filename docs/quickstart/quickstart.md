## Quickstart

In this document you will find a step-by-step guide on how to get redis-operator running in a local Minikube cluster.
You will run a simple standalone and cluster mode of Redis.

### Prerequisites

In order to setup redis-operator, you'll need access to a Kubernetes cluster:-

- [Virtualbox](https://www.virtualbox.org/wiki/Downloads) - hypervisor to run a kubernetes cluster
- [Minikube](https://kubernetes.io/docs/setup/minikube/) - for kubernetes cluster creation on local machine
- [Kubectl](https://kubernetes.io/docs/tasks/tools/install-kubectl/) - to interact with kubernetes cluster

### Deploying Redis Operator to a Minikube cluster

#### Start a local minikube cluster

Minikube is a minimal Kubernetes cluster run in a virtual machine (here in VirtualBox).

```shell
minikube start --vm-driver virtualbox
```

From now on your local Kubernetes client `kubectl` is configured to use your just started Minikube cluster.

#### Create a new namespace

First, we need to create a namespace for our resources to be deployed in. This is for the sake of separation and keeping order:

```shell
kubectl create namespace redis-operator
```

Redis operator by default watches for every change in Redis Configuration.

### Deploy redis standalone setup

```shell
kubectl apply -f example/redis-standalone-example.yaml -n redis-operator
```

### Deploy redis cluster setup


```shell
kubectl apply -f example/redis-cluster-example.yaml -n redis-operator
```

## Cleanup

```shell
# To delete standalone setup
kubectl delete -f example/redis-standalone-example.yaml -n redis-operator
# To delete cluster setup
kubectl delete -f example/redis-cluster-example.yaml -n redis-operator
```
