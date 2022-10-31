<p align="center">
  <img src="./static/redis-operator-logo.svg" height="440" width="400">
</p>

<p align="center">
  <a href="https://dev.azure.com/opstreedevops/DevOps/_apis/build/status/redis-operator/redis-operator?repoName=OT-CONTAINER-KIT%2Fredis-operator&branchName=master">
    <img src="https://dev.azure.com/opstreedevops/DevOps/_apis/build/status/redis-operator/redis-operator?repoName=OT-CONTAINER-KIT%2Fredis-operator&branchName=master" alt="Azure Pipelines">
  </a>
  <a href="https://goreportcard.com/report/github.com/OT-CONTAINER-KIT/redis-operator">
    <img src="https://goreportcard.com/badge/github.com/OT-CONTAINER-KIT/redis-operator" alt="GoReportCard">
  </a>
  <a href="http://golang.org">
    <img src="https://img.shields.io/github/go-mod/go-version/OT-CONTAINER-KIT/redis-operator" alt="GitHub go.mod Go version (subdirectory of monorepo)">
  </a>
  <a href="http://golang.org">
    <img src="https://img.shields.io/badge/Made%20with-Go-1f425f.svg" alt="made-with-Go">
  </a>
  <a href="https://quay.io/repository/opstree/redis-operator">
    <img src="https://img.shields.io/badge/container-ready-green" alt="Docker">
  </a>
  <a href="https://github.com/OT-CONTAINER-KIT/redis-operator/master/LICENSE">
    <img src="https://img.shields.io/badge/License-Apache%202.0-blue.svg" alt="License">
  </a>
</p>

A Golang based redis operator that will make/oversee Redis standalone and cluster mode setup on top of the Kubernetes. It can create a redis cluster setup with best practices on Cloud as well as the Bare metal environment. Also, it provides an in-built monitoring capability using redis-exporter.

For documentation, please refer to https://ot-container-kit.github.io/redis-operator/

Organizations that are using Redis Operator to manage their redis workload can be found [here](./USED_BY_ORGANIZATIONS.md). If your organization is also using Redis Operator, please free to add by creating a PR https://github.com/OT-CONTAINER-KIT/redis-operator/pulls.

This operator only supports versions of redis `=>6`.

## Architecture

<div align="center">
    <img src="./static/redis-operator-architecture.png">
</div>

## Purpose

There are multiple problems that people face while setting up redis setup on Kubernetes, specially cluster type setup. The purpose of creating this opperator is to provide an easy and production ready interface for redis setup that include best-practices, security controls, monitoring, and management.

## Supported Features

Here the features which are supported by this operator:-

- Redis cluster and standalone mode setup
- Redis cluster failover and recovery
- Inbuilt monitoring with redis exporter
- Password and password-less setup of redis
- TLS support for additional security layer
- Ipv4 and Ipv6 support for redis setup
- Detailed monitoring grafana dashboard

## Getting Started

If you want to deploy redis-operator from scratch to a local Minikube cluster, begin with the [Getting started](https://ot-container-kit.github.io/redis-operator/#/quickstart/quickstart) document. It will guide your through the setup step-by-step.

The configuration of Redis setup should be described in [CRD definitions](config/crd/bases). All the examples related to redis standalone and cluster setup can be found inside [example](./example) folder.

## Prerequisites

Redis operator requires a Kubernetes cluster of version `>=1.18.0`. If you have just started with Operators, it's highly recommended using the latest version of Kubernetes.

## Quickstart

The setup can be done by using helm. If you want to see more example, please go through the [example](./example) folder.

But you can simply use the helm chart for installation.

```shell
# Add the helm chart
$ helm repo add ot-helm https://ot-container-kit.github.io/helm-charts/
```

```shell
# Deploy the redis-operator
$ helm upgrade redis-operator ot-helm/redis-operator --install --namespace ot-operators
```

After deployment, verify the installation of operator

```shell
$ helm test redis-operator --namespace ot-operators
```

Creating redis cluster or standalone setup.

```shell
# Create redis cluster setup
$ helm upgrade redis-cluster ot-helm/redis-cluster \
  --set redisCluster.clusterSize=3 --install \ 
  --namespace ot-operators
```

```shell
# Create redis standalone setup
$ helm upgrade redis ot-helm/redis \
  --install --namespace ot-operators
```

If you want to customize the value file by yourself while initializing the helm command, the values files for reference are present [here](https://github.com/OT-CONTAINER-KIT/helm-charts/tree/main/charts/redis-setup).

## Monitoring with Prometheus

To monitor redis performance we will be using prometheus. In any case, extra prometheus configuration will not be required because we will be using the Prometheus service discover pattern. For that we already have set these annotations:-

```yaml
  annotations:
    redis.opstreelabs.in: "true"
    prometheus.io/scrape: "true"
    prometheus.io/port: "9121"
```

## Contribution

Please see our [CONTRIBUTING.md](./CONTRIBUTING.md) for details.

## Release History

Please see our [CHANGELOG.md](./CHANGELOG.md) for details.

## Contact Information

This project is managed by [OpsTree Solutions](http://opstree.com). If you have any queries or suggestions, mail us at [opensource@opstree.com](opensource@opstree.com).
