---
title: "Redis"
linkTitle: "Redis"
weight: 10
date: 2022-11-02T00:19:19Z
description: >
  Configurations and parameters for Redis standalone
---

Redis standalone configuration can be customized by [values.yaml](https://github.com/OT-CONTAINER-KIT/helm-charts/blob/main/charts/redis/values.yaml). The recommended way of managing the setup is using `helm` but if the setup is not maintained by it, `YAML` CRD parameters can be modified in the manifest.

## Helm Configuration Parameters

| **Name**                          | **Value**                      | **Description**                                                                               |
|-----------------------------------|--------------------------------|-----------------------------------------------------------------------------------------------|
| `imagePullSecrets`                | []                             | List of image pull secrets, in case redis image is getting pull from private registry         |
| `redisStandalone.secretName`      | redis-secret                   | Name of the existing secret in Kubernetes                                                     |
| `redisStandalone.secretKey`       | password                       | Name of the existing secret key in Kubernetes                                                 |
| `redisStandalone.image`           | quay.io/opstree/redis          | Name of the redis image                                                                       |
| `redisStandalone.tag`             | v7.0.5                         | Tag of the redis image                                                                        |
| `redisStandalone.imagePullPolicy` | IfNotPresent                   | Image Pull Policy of the redis image                                                          |
| `redisStandalone.resources`       | {}                             | Request and limits for redis statefulset                                                      |
| `externalService.enabled`         | false                          | If redis service needs to be exposed using LoadBalancer or NodePort                           |
| `externalService.annotations`     | {}                             | Kubernetes service related annotations                                                        |
| `externalService.serviceType`     | NodePort                       | Kubernetes service type for exposing service, values - ClusterIP, NodePort, and LoadBalancer  |
| `externalService.port`            | 6379                           | Port number on which redis external service should be exposed                                 |
| `serviceMonitor.enabled`          | false                          | Servicemonitor to monitor redis with Prometheus                                               |
| `serviceMonitor.interval`         | 30s                            | Interval at which metrics should be scraped.                                                  |
| `serviceMonitor.scrapeTimeout`    | 10s                            | Timeout after which the scrape is ended                                                       |
| `serviceMonitor.namespace`        | monitoring                     | Namespace in which Prometheus operator is running                                             |
| `redisExporter.enabled`           | true                           | Redis exporter should be deployed or not                                                      |
| `redisExporter.image`             | quay.io/opstree/redis-exporter | Name of the redis exporter image                                                              |
| `redisExporter.tag`               | v1.44.0                        | Tag of the redis exporter image                                                               |
| `redisExporter.imagePullPolicy`   | IfNotPresent                   | Image Pull Policy of the redis exporter image                                                 |
| `redisExporter.env`               | []                             | Extra environment variables which needs to be added in redis exporter                         |
| `nodeSelector`                    | {}                             | NodeSelector for redis statefulset                                                            |
| `priorityClassName`               | ""                             | Priority class name for the redis statefulset                                                 |
| `storageSpec`                     | {}                             | Storage configuration for redis setup                                                         |
| `securityContext`                 | {}                             | Security Context for redis pods for changing system or kernel level parameters                |
| `affinity`                        | {}                             | Affinity for node and pod for redis statefulset                                               |
| `tolerations`                     | []                             | Tolerations for redis statefulset                                                             |
| `sidecars`                        | []                             | Sidecar containers to run alongside Redis pods                                                |
