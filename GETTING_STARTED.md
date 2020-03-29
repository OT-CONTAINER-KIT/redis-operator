## Getting started

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

From now on your local Kubernetes client (kubectl) is configured to use your just started Minikube cluster.

#### Create a new namespace

First, we need to create a namespace for our resources to be deployed in. This is for the sake of separation and keeping order:

```shell
kubectl create namespace redis-operator
```

Redis operator by default watches for every change in Redis Configuration.

#### Configuration Overview

Here are the configuration overview

| **Field Name** | **Possible Values** | **Description** |
|----------------|---------------------|-----------------|
| mode | `cluster` or `standalone` | Setup mode for redis |
| exporter | `true` or `false` | Whether exporter should be enabled for monitoring or not |
| size | `Any odd integer value` | Size of the redis cluster nodes |
| imageName | `Valid image name` | Name of the redis image |
| redisExporterImage | `Valid image name` | Name of the redis exporter image |
| imagePullPolicy | `Always` or `IfNotPresent` | Image Pull Policy for pulling image in cluster |
| redisConfig | `Map values` | Extra redis configuration if needs to pass |
| redisPassword | `string` | Password for redis server or cluster |
| resources | `k8s resource` | Request and limit resource for Kubernetes cluster |
| storage | `k8s storage` | Storage definition for creating pvc and pv |
| nodeSelector | `map value` | k8s nodeselector to run on a specific node |
| securityContext | `k8s securityContext` | Security context to manipulate kernel parameters |
| priorityClassName | `k8s priorityClass` | Priority class name to define redis priority |
| affinity | `k8s affinity` | Affinity to distribute the redis replicas accross the nodes |

### Deploy redis standalone setup

To deploy redis standalone setup, a simple helm chart can be used.

```shell
# Create redis standalone setup
helm upgrade redis ./helm/redis-setup --set redisSetup.setupMode="standalone" \
--install --namespace redis-operator
```

Using kubectl

```shell
kubectl apply -f example/redis-standalone-example.yaml -n redis-operator
```

### Deploy redis cluster setup

To deploy redis standalone setup, a simple helm chart can be used.

```shell
# Create redis cluster setup
helm upgrade redis-cluster ./helm/redis-setup --set redisSetup.setupMode="cluster" \
--set redisSetup.clusterSize=3 \
--install --namespace redis-operator
```

Using kubectl

```shell
kubectl apply -f example/redis-cluster-example.yaml -n redis-operator
```

## Cleanup

For cleanup, follow this procedure

Using Helm

```shell
# To delete standalone setup
helm delete redis --namespace redis-operator
# To delete cluster setup
helm delete redis-cluster --namespace redis-operator
```

Using Kubectl

```shell
# To delete standalone setup
kubectl delete -f example/redis-standalone-example.yaml -n redis-operator
# To delete cluster setup
kubectl delete -f example/redis-cluster-example.yaml -n redis-operator
```
