<p align="left">
  <img src="./static/redis-operator-logo.svg" height="180" width="180">
</p>

[![CircleCI](https://circleci.com/gh/OT-CONTAINER-KIT/redis-operator.svg?style=shield)](https://circleci.com/gh/OT-CONTAINER-KIT/redis-operator)
[![Go Report Card](https://goreportcard.com/badge/github.com/OT-CONTAINER-KIT/redis-operator)](https://goreportcard.com/report/github.com/OT-CONTAINER-KIT/redis-operator)
[![Docker Repository on Quay](https://img.shields.io/badge/container-ready-green "Docker Repository on Quay")](https://quay.io/repository/opstree/redis-operator)
[![Maintainability](https://api.codeclimate.com/v1/badges/89dd2d6355e51d623068/maintainability)](https://codeclimate.com/github/OT-CONTAINER-KIT/redis-operator/maintainability)
[![Apache License](https://img.shields.io/badge/License-Apache%202.0-blue.svg)](LICENSE)

# Speculator: Redis Operator

A Golang based redis operator that will make/oversee Redis standalone/cluster mode setup on top of the Kubernetes. It can create a redis cluster setup with best practices on Cloud as well as the Bare metal environment. Also, it provides an in-built monitoring capability using redis-exporter.

For documentation, please refer to https://docs.opstreelabs.in/redis-operator/

## Architecture

<div align="center">
    <img src="./static/redis-operator.png">
</div>

### Purpose

The purpose of creating this operator was to provide an easy and production grade setup of Redis on Kubernetes. It doesn't care if you have a plain on-prem Kubernetes or cloud-based.

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

If you want to deploy redis-operator from scratch to a local Minikube cluster, begin with the [Getting started](https://ot-container-kit.github.io/redis-operator/#/quickstart/quickstart) document. It will guide your through the setup step-by-step.

### Example

The configuration of Redis setup should be described in Redis CRD. You will find all the examples manifests in [example](./example) folder.

### Prerequisites

Redis operator requires a Kubernetes cluster of version `>=1.8.0`. If you have just started with Operators, its highly recommended to use latest version of Kubernetes.

### Quickstart

The setup can be done by using helm. If you want to see more example, please go through the [example](./example) folder.

But you can simply use the helm chart for installation.

```shell
# Deploy the redis-operator
helm upgrade redis-operator ./helm/redis-operator --install --namespace redis-operator
```

After deployment, verify the installation of operator

```shell
helm test redis-operator --namespace redis-operator
```

Creating redis cluster or standalone setup.

```shell
# Create redis cluster setup
helm upgrade redis-cluster ./helm/redis-setup -f ./helm/redis-setup/cluster-values.yaml \
  --set setupMode="cluster" --set cluster.size=3 \
  --install --namespace redis-operator
```

```shell
# Create redis standalone setup
helm upgrade redis ./helm/redis-setup -f ./helm/redis-setup/cluster-values.yaml \
  --set setupMode="standalone" \
  --install --namespace redis-operator
```

### Monitoring with Prometheus

To monitor redis performance we will be using prometheus. In any case, extra prometheus configuration will not be required because we will be using the Prometheus service discover pattern. For that we already have set these annotations:-

```yaml
  annotations:
    redis.opstreelabs.in: "true"
    prometheus.io/scrape: "true"
    prometheus.io/port: "9121"
```

### Development

Please see our [DEVELOPMENT.md](https://ot-container-kit.github.io/redis-operator/#/development/development) for details.

### Release History

Please see our [CHANGELOG.md](./CHANGELOG.md) for details.

### Documentation

Please see our [GETTING_STARTED.md](https://ot-container-kit.github.io/redis-operator/#/quickstart/quickstart) for details.

