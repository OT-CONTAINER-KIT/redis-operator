<p align="left">
  <img src="./static/redis-operator-logo.png">
</p>

# Speculator: Redis Operator

A golang based redis operator which will make/oversee Redis standalone/cluster mode setup on top of the Kubernetes.

### Purpose

The purpose of creating this operator was to provide an easy and production grade setup of Redis on Kubernetes. It doesn't care if you have a plan Kubernetes, a Cloud based.

### Supported Features

Here the features which are supported by this operator:-

- Redis cluster/standalone mode setup
- Inbuilt monitoring with prometheus exporter
- Dynamic storage provisioning with pvc template
- Resources restrictions with k8s requests and limits
- Password/Password-less setup
- Node selector and affinity
- Priority class to manage setup priority
- SecurityContext to manipulate kernel parameters

### Getting Started

<p align="center">
  <img src="./static/redis-operator.png">
</p>

If you want to deploy redis-operator from scratch to a local Minikube cluster, begin with the [Getting started](./GETTING_STARTED.md) document. It will guide your through the setup step-by-step.

### Example

The configuration of Redis setup should be described in Redis CRD. You will find all the examples manifests in [example](./example) folder.

### Prerequisites

Redis operator requires a Kubernetes cluster of version `>=1.8.0`. If you have just started with Operators, its highly recommended to use latest version of Kubernetes.

## To Do
- Add slave statefulsets in operator
- Add services for slave statefulsets in operator
- Nodeselector
- PriorityClass
- Affinity
- Dynamic Configuration Update
- SecurityContext
- Readiness and liveness probes
- Create Getting Started
- Create example folder and add examples
- Add unit test cases
- Add circle ci pipeline integration
