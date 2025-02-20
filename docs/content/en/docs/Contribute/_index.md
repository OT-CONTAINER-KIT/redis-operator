---
title: "Contribute"
linkTitle: "Contribute"
weight: 9
date: 2022-11-02T00:19:19Z
description: >
  How to contribute to the Redis Operator
---

## Prerequisites

- [Kubernetes Cluster](https://kubernetes.io)
- [Git](https://git-scm.com/downloads)
- [Go](https://golang.org/dl/)
- [Docker](https://docs.docker.com/install/)
- [Operator SDK](https://github.com/operator-framework/operator-sdk/releases)
- [Make](https://www.gnu.org/software/make/manual/make.html)
- [Eksctl](https://eksctl.io/)

## Local Kubernetes Cluster

For development and testing of operator on local system, we need to set up a [Minikube](https://minikube.sigs.k8s.io/docs/start/) or local Kubernetes cluster.

Minikube is a single node Kubernetes cluster that generally gets used for the development and testing on Kubernetes. For creating a Minkube cluster we need to simply run:

```shell
$ minikube start --vm-driver virtualbox
...
😄  minikube v1.0.1 on linux (amd64)
🤹  Downloading Kubernetes v1.14.1 images in the background ...
🔥  Creating kvm2 VM (CPUs=2, Memory=2048MB, Disk=20000MB) ...
📶  "minikube" IP address is 192.168.39.240
🐳  Configuring Docker as the container runtime ...
🐳  Version of container runtime is 18.06.3-ce
⌛  Waiting for image downloads to complete ...
✨  Preparing Kubernetes environment ...
🚜  Pulling images required by Kubernetes v1.14.1 ...
🚀  Launching Kubernetes v1.14.1 using kubeadm ... 
⌛  Waiting for pods: apiserver proxy etcd scheduler controller dns
🔑  Configuring cluster permissions ...
🤔  Verifying component health .....
💗  kubectl is now configured to use "minikube"
🏄  Done! Thank you for using minikube!
```

## Cloud Kubernetes Cluster

For cloud based Kubernetes cluster we can use any type of platforms like [Amazon Web Service](https://aws.amazon.com/), [Azure Cloud](https://azure.microsoft.com/en-in/), or [Google Cloud Platform](https://cloud.google.com/). We have provided an [eks-cluster.yaml](./example/eks-cluster.yaml) file for creating an Elastic Kubernetes Service(EKS) using [eksctl](https://eksctl.io/).

`eksctl` is a cli tool to create a Kubernetes cluster on EKS by a single command. It supports creation of Ipv4 and Ipv6 based Kubernetes clusters for development.

```shell
$ eksctl create cluster -f example/eks-cluster.yaml
...
2022-10-30 19:47:44 [ℹ]  eksctl version 0.114.0
2022-10-30 19:47:44 [ℹ]  using region us-west-2
2022-10-30 19:47:45 [ℹ]  setting availability zones to [us-west-2d us-west-2c us-west-2a]
2022-10-30 19:47:45 [ℹ]  subnets for us-west-2d - public:192.168.0.0/19 private:192.168.96.0/19
2022-10-30 19:47:45 [ℹ]  subnets for us-west-2c - public:192.168.32.0/19 private:192.168.128.0/19
2022-10-30 19:47:45 [ℹ]  subnets for us-west-2a - public:192.168.64.0/19 private:192.168.160.0/19
2022-10-30 19:47:45 [ℹ]  nodegroup "ng-1" will use "" [AmazonLinux2/1.22]
2022-10-30 19:47:45 [ℹ]  using SSH public key "/Users/abhishekdubey/.ssh/id_rsa.pub" as "eksctl-operator-testing-nodegroup-ng-1-8b:2b:b2:fc:4c:7f:9c:0d:54:14:70:39:25:b5:6d:60"
2022-10-30 19:47:47 [ℹ]  using Kubernetes version 1.22
2022-10-30 19:47:47 [ℹ]  creating EKS cluster "operator-testing" in "us-west-2" region with managed nodes
2022-10-30 19:47:47 [ℹ]  1 nodegroup (ng-1) was included (based on the include/exclude rules)
2022-10-30 19:47:47 [ℹ]  will create a CloudFormation stack for cluster itself and 0 nodegroup stack(s)
2022-10-30 19:47:47 [ℹ]  will create a CloudFormation stack for cluster itself and 1 managed nodegroup stack(s)
2022-10-30 19:47:47 [ℹ]  if you encounter any issues, check CloudFormation console or try 'eksctl utils describe-stacks --region=us-west-2 --cluster=operator-testing'
2022-10-30 19:47:47 [ℹ]  Kubernetes API endpoint access will use default of {publicAccess=true, privateAccess=false} for cluster "operator-testing" in "us-west-2"
2022-10-30 19:47:47 [ℹ]  CloudWatch logging will not be enabled for cluster "operator-testing" in "us-west-2"
2022-10-30 19:47:47 [ℹ]  you can enable it with 'eksctl utils update-cluster-logging --enable-types={SPECIFY-YOUR-LOG-TYPES-HERE (e.g. all)} --region=us-west-2 --cluster=operator-testing'
2022-10-30 19:47:47 [ℹ]
2 sequential tasks: { create cluster control plane "operator-testing",
    2 sequential sub-tasks: {
        5 sequential sub-tasks: {
            wait for control plane to become ready,
            associate IAM OIDC provider,
            no tasks,
            restart daemonset "kube-system/aws-node",
            1 task: { create addons },
        },
        create managed nodegroup "ng-1",
    }
}
2022-10-30 19:47:47 [ℹ]  building cluster stack "eksctl-operator-testing-cluster"
2022-10-30 19:47:50 [ℹ]  deploying stack "eksctl-operator-testing-cluster"
2022-10-30 20:01:17 [ℹ]  daemonset "kube-system/aws-node" restarted
2022-10-30 20:01:18 [ℹ]  creating role using recommended policies
2022-10-30 20:01:20 [ℹ]  deploying stack "eksctl-operator-testing-addon-vpc-cni"
2022-10-30 20:01:20 [ℹ]  waiting for CloudFormation stack "eksctl-operator-testing-addon-vpc-cni"
2022-10-30 20:01:52 [ℹ]  waiting for CloudFormation stack "eksctl-operator-testing-addon-vpc-cni"
2022-10-30 20:02:24 [ℹ]  waiting for CloudFormation stack "eksctl-operator-testing-addon-vpc-cni"
2022-10-30 20:02:26 [ℹ]  creating addon
2022-10-30 20:02:37 [ℹ]  addon "vpc-cni" active
2022-10-30 20:02:39 [ℹ]  building managed nodegroup stack "eksctl-operator-testing-nodegroup-ng-1"
```

For setting up the Ipv4 or Ipv6 cluster with eksctl, we need to modify this configuration in the [eks-cluster.yaml](./example/eks-cluster.yaml):

```yaml
kubernetesNetworkConfig:
  ipFamily: IPv4
#  ipFamily: IPv6
```

## Operator structure

The structure for Redis operator includes different module's directory. The codebase include these major directories:

```shell
redis-operator/
|-- api
|   |-- v1beta2
|-- bin
|-- config
|   |-- certmanager
|   |-- crd
|   |   |-- bases
|   |   |-- patches
|   |-- default
|   |-- manager
|   |-- prometheus
|   |-- rbac
|   |-- samples
|   |-- scorecard
|       |-- bases
|       |-- patches
|-- controllers
|-- hack
|-- k8sutils
```

As part of the development, generally, we modify the codebase in API, controllers, and k8sutils. The API modules hold the interface and structure for CRD definition, the controllers are the watch controllers that create, update, and delete the resources. The k8sutils is a module in which all the Kubernetes resources(Statefulsets, Services, etc.) codebase is present.

### Building Operator

For building operator, we can execute make command to create binary and docker image:

```shell
$ make manager
$ make docker-build
```

For any change inside the `api` module, we need to recreate the CRD schema because of interface changes. To generate the CRD manifest and RBAC policies updated by operator:

```shell
$ make manifests
```

### Deploying Operator

The operator deployment can be done via `helm` cli, we just need to define the custom image name and tag for testing the operator functionality:

```shell
$ helm upgrade redis-operator ot-helm/redis-operator \
  --install --create-namespace --namespace ot-operators \
  --set redisOperator.imageName=<custom-url> \
  --set redisOperator.imageTag=<customTag>
```

```shell
# For deploying standalone redis
$ helm upgrade redis ot-helm/redis --namespace ot-operators

# For deploying cluster redis
$ helm upgrade redis-cluster ot-helm/redis-cluster \n 
  --set redisCluster.clusterSize=3 --install --namespace ot-operators \ 
  --set pdb.enabled=false --set redisCluster.tag=v7.0.5-beta
```

## Docker Image Development

Development of redis docker image is maintained inside a different repository - https://github.com/OT-CONTAINER-KIT/redis. To make any change or suggestion related to Redis docker image, please refer to this repository and make required changes.

In the repository, we have `Dockerfile` for [Redis](https://github.com/OT-CONTAINER-KIT/redis/blob/master/Dockerfile) and [Redis Exporter](https://github.com/OT-CONTAINER-KIT/redis/blob/master/Dockerfile.exporter)

For building the docker image for redis and redis exporter, there are simple make commands:

```shell
$ make build-redis
$ make build-redis-exporter
```
