<p align="left">
  <img src="https://github.com/OT-CONTAINER-KIT/redis-operator/raw/master/static/redis-operator-logo.svg" height="180" width="180">
</p>

[![CircleCI](https://circleci.com/gh/OT-CONTAINER-KIT/redis-operator.svg?style=shield)](https://circleci.com/gh/OT-CONTAINER-KIT/redis-operator)
[![Go Report Card](https://goreportcard.com/badge/github.com/OT-CONTAINER-KIT/redis-operator)](https://goreportcard.com/report/github.com/OT-CONTAINER-KIT/redis-operator)
[![Docker Repository on Quay](https://img.shields.io/badge/container-ready-green "Docker Repository on Quay")](https://quay.io/repository/opstree/redis-operator)
[![Maintainability](https://api.codeclimate.com/v1/badges/89dd2d6355e51d623068/maintainability)](https://codeclimate.com/github/OT-CONTAINER-KIT/redis-operator/maintainability)
[![Apache License](https://img.shields.io/badge/License-Apache%202.0-blue.svg)](LICENSE)

# Speculator: Redis Operator

A Golang based redis operator that will make/oversee Redis standalone/cluster mode setup on top of the Kubernetes. It can create a redis cluster setup with best practices on Cloud as well as the Bare metal environment. Also, it provides an in-built monitoring capability using redis-exporter.

## Architecture

<div align="center">
    <img src="https://github.com/OT-CONTAINER-KIT/redis-operator/raw/master/static/redis-operator.png">
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
