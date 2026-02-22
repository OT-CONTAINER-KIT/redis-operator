<p align="center">
  <img src="./static/redis-operator-logo.svg" height="330" width="330">
</p>

<p align="center">
  <a href="https://github.com/OT-CONTAINER-KIT/redis-operator/actions/workflows/ci.yaml">
    <img src="https://github.com/OT-CONTAINER-KIT/redis-operator/actions/workflows/ci.yaml/badge.svg" alt="CI Pipeline">
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
  <a href="https://github.com/OT-CONTAINER-KIT/redis-operator/main/LICENSE">
    <img src="https://img.shields.io/badge/License-Apache%202.0-blue.svg" alt="License">
  </a>
</p>

A Golang-based Redis operator that will make/oversee Redis standalone and cluster mode setup on top of Kubernetes. It can create a Redis cluster setup with best practices on Cloud as well as the bare metal environment. Also, it provides an in-built monitoring capability using redis-exporter.

For documentation, please refer to <https://redis-operator.opstree.dev/>

Organizations that are using Redis Operator to manage their Redis workload can be found [here](./USED_BY_ORGANIZATIONS.md). If your organization is also using Redis Operator, please feel free to add by creating a [pull request](https://github.com/OT-CONTAINER-KIT/redis-operator/pulls)

This operator only supports versions of Redis `>=6`.

## Architecture

<div align="center">
    <img src="./static/updated-redis-operator-architecture-using-meshery.jpg">
</div>

## Purpose

There are multiple problems that people face while setting up Redis setup on Kubernetes, especially cluster type setup. The purpose of creating this operator is to provide an easy and production-ready interface for Redis setup that includes best-practices, security controls, monitoring, and management.

## Supported Features

Here are the features which are supported by this operator:

- Redis cluster and standalone mode setup
- Redis cluster failover and recovery
- Inbuilt monitoring with redis exporter
- Password and password-less setup of Redis
- TLS support for additional security layer
- IPv4 and IPv6 support for Redis setup
- Detailed monitoring Grafana dashboard

Check the [Installation](https://redis-operator.opstree.dev/docs/installation/) to deploy your first cluster with operator.

## Image Compatibility

The operator supports Redis versions `>=6.x`. However, **it is strongly recommended to use the latest stable version** to ensure you have the latest security fixes and bug patches from upstream.

**Container Images:**
- **Redis**: `quay.io/opstree/redis`
- **Sentinel**: `quay.io/opstree/redis-sentinel`
- **Exporter**: `quay.io/opstree/redis-exporter`

## Monitoring with Prometheus

To monitor Redis performance we will be using Prometheus. In any case, extra Prometheus configuration will not be required because we will be using the Prometheus service discovery pattern. For that we already have set these annotations:

```yaml
  annotations:
    redis.opstreelabs.in: "true"
    prometheus.io/scrape: "true"
    prometheus.io/port: "9121"
```

In addition to the annotations you have the possibility to deploy a `ServiceMonitor` for each of the Redis installations (configurable via Helm values file).

## Contribution

Please see our [CONTRIBUTING.md](./CONTRIBUTING.md) for details.

## Release History

Please see our [Release History](https://redis-operator.opstree.dev/docs/release-history/) for details.

## Contact Information

This project is managed by [OpsTree Solutions](http://opstree.com). For any queries or suggestions, you can reach out to us at [opensource@opstree.com](mailto:opensource@opstree.com).

Join our Slack Channel: [#redis-operator](https://join.slack.com/t/opstree/shared_invite/zt-3o8jp35x-UGMU2Cy0WSBk3Lbzqa2wVw).
