---
title: "Create Cluster"
linkTitle: "Create Cluster"
weight: 10
date: 2022-11-02T00:19:19Z
description: >
  Instructions for creating a Kubernetes cluster and installing Redis Operator on it
---

Redis Operator needs a Kubernetes or Openshift cluster for provisioning a Redis setup. This guide helps in setting up a Kubernetes cluster from a quickstart perspective.

Tools involved in this kind of setup:

- [Eksctl](https://eksctl.io/)
- [Minikube](https://minikube.sigs.k8s.io/docs/start/)

## Amazon EKS Cluster

To create a Kubernetes cluster on AWS, we need to download and install the [eksctl](https://eksctl.io/) on the local system and then [eks-cluster.yaml](https://github.com/OT-CONTAINER-KIT/redis-operator/blob/main/example/eks-cluster.yaml) can be executed with it for cluster creation.

The content of [eks-cluster.yaml](https://github.com/OT-CONTAINER-KIT/redis-operator/blob/main/example/eks-cluster.yaml) looks like:

```yaml
apiVersion: eksctl.io/v1alpha5
kind: ClusterConfig
metadata:
  name: operator-testing
  region: us-west-2
  version: "1.22"
managedNodeGroups:
  - name: ng-1
    instanceType: t3a.medium
    desiredCapacity: 3
    volumeSize: 30
    ssh:
      allow: true
    volumeType: gp3
kubernetesNetworkConfig:
  ipFamily: IPv4
# ipFamily: IPv6
addons:
  - name: vpc-cni
  - name: coredns
  - name: kube-proxy
iam:
  withOIDC: true
```

```shell
$ eksctl create cluster -f example/eks-cluster.yaml
...
2022-11-01 12:49:15 [ℹ]  eksctl version 0.114.0
2022-11-01 12:49:15 [ℹ]  using region us-west-2
2022-11-01 12:49:16 [ℹ]  setting availability zones to [us-west-2b us-west-2a us-west-2d]
2022-11-01 12:49:16 [ℹ]  subnets for us-west-2b - public:192.168.0.0/19 private:192.168.96.0/19
2022-11-01 12:49:16 [ℹ]  subnets for us-west-2a - public:192.168.32.0/19 private:192.168.128.0/19
2022-11-01 12:49:16 [ℹ]  subnets for us-west-2d - public:192.168.64.0/19 private:192.168.160.0/19
2022-11-01 12:49:16 [ℹ]  nodegroup "ng-1" will use "" [AmazonLinux2/1.22]
2022-11-01 12:49:16 [ℹ]  using SSH public key "/Users/abhishekdubey/.ssh/id_rsa.pub" as "eksctl-operator-testing-nodegroup-ng-1-8b:2b:b2:fc:4c:7f:9c:0d:54:14:70:39:25:b5:6d:60"
2022-11-01 12:49:18 [ℹ]  using Kubernetes version 1.22
2022-11-01 12:49:18 [ℹ]  creating EKS cluster "operator-testing" in "us-west-2" region with managed nodes
2022-11-01 12:49:18 [ℹ]  1 nodegroup (ng-1) was included (based on the include/exclude rules)
2022-11-01 12:49:18 [ℹ]  will create a CloudFormation stack for cluster itself and 0 nodegroup stack(s)
2022-11-01 12:49:18 [ℹ]  will create a CloudFormation stack for cluster itself and 1 managed nodegroup stack(s)
2022-11-01 12:49:18 [ℹ]  if you encounter any issues, check CloudFormation console or try 'eksctl utils describe-stacks --region=us-west-2 --cluster=operator-testing'
2022-11-01 12:49:18 [ℹ]  Kubernetes API endpoint access will use default of {publicAccess=true, privateAccess=false} for cluster "operator-testing" in "us-west-2"
2022-11-01 12:49:18 [ℹ]  CloudWatch logging will not be enabled for cluster "operator-testing" in "us-west-2"
2022-11-01 12:49:18 [ℹ]  you can enable it with 'eksctl utils update-cluster-logging --enable-types={SPECIFY-YOUR-LOG-TYPES-HERE (e.g. all)} --region=us-west-2 --cluster=operator-testing'
2022-11-01 13:08:05 [ℹ]  waiting for CloudFormation stack "eksctl-operator-testing-nodegroup-ng-1"
2022-11-01 13:08:05 [ℹ]  waiting for the control plane to become ready
2022-11-01 13:08:06 [✔]  saved kubeconfig as "/Users/abhishekdubey/.kube/lab-config"
2022-11-01 13:08:06 [ℹ]  no tasks
2022-11-01 13:08:06 [✔]  all EKS cluster resources for "operator-testing" have been created
2022-11-01 13:08:08 [ℹ]  nodegroup "ng-1" has 3 node(s)
2022-11-01 13:08:08 [ℹ]  node "ip-192-168-25-130.us-west-2.compute.internal" is ready
2022-11-01 13:08:08 [ℹ]  node "ip-192-168-38-199.us-west-2.compute.internal" is ready
2022-11-01 13:08:08 [ℹ]  node "ip-192-168-89-35.us-west-2.compute.internal" is ready
2022-11-01 13:08:08 [ℹ]  waiting for at least 3 node(s) to become ready in "ng-1"
2022-11-01 13:08:08 [ℹ]  nodegroup "ng-1" has 3 node(s)
2022-11-01 13:08:08 [ℹ]  node "ip-192-168-25-130.us-west-2.compute.internal" is ready
2022-11-01 13:08:08 [ℹ]  node "ip-192-168-38-199.us-west-2.compute.internal" is ready
2022-11-01 13:08:08 [ℹ]  node "ip-192-168-89-35.us-west-2.compute.internal" is ready
2022-11-01 13:08:11 [ℹ]  no recommended policies found, proceeding without any IAM
```

## Minikube

Minikube is a tool for creation of Kubernetes on local system for Development purpose. It requires a [Docker](https://docker.com) compatible system or virtual machine environment.

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
