---
title: "Overview"
linkTitle: "Overview"
weight: 1
date: 2022-11-02T00:19:19Z
description: >
  Redis Operator is a software to set up and manage Redis on [Kubernetes](https://kubernetes.io).
---

A Golang based redis operator that will make/oversee Redis standalone/cluster/replication/sentinel mode setup on top of the Kubernetes. It can create a redis cluster setup with best practices on Cloud as well as the Bare-metal environment. Also, it provides an in-built monitoring capability using redis-exporter.

Documentation is available here:- https://ot-container-kit.github.io/redis-operator/

The type of Redis setup which is currently supported:-

- Redis Cluster
- Redis Standalone
- Redis Replication
- Redis Sentinel

This operator only supports versions of redis `=>6`.

## Purpose

There are multiple problems that people face while setting up redis setup on Kubernetes, specially cluster type setup. The purpose of creating this opperator is to provide an easy and production ready interface for redis setup that include best-practices, security controls, monitoring, and management.

## Why Redis Operator?

Here the features which are supported by this operator:-

- Redis cluster, replication and standalone mode setup
- Redis cluster and replication failover and recovery
- Inbuilt monitoring with redis exporter
- Password and password-less setup of redis
- TLS support for additional security layer
- Ipv4 and Ipv6 support for redis setup
- Detailed monitoring grafana dashboard

## Architecture

![redis_operator_architecture](../../../images/redis-operator-architecture.png)

## Code of Conduct

Participation in this project comes under the [CONTRIBUTING.md](https://github.com/OT-CONTAINER-KIT/redis-operator/blob/main/CONTRIBUTING.md)

## What's Next

- Installation of Redis Operator
- Setup of Redis standalone, cluster, replication and sentinel mode
- Monitoring of Redis setup
- Configuration and advance cofiguration of Operator
