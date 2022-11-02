---
title: "RedisCluster"
linkTitle: "RedisCluster"
weight: 20
date: 2022-11-02T00:19:19Z
description: >
  Configurations and parameters for Redis cluster
---

Redis cluster can be customized by [values.yaml](https://github.com/OT-CONTAINER-KIT/helm-charts/blob/main/charts/redis-cluster/values.yaml). The recommended way of managing the setup is using `helm` but if the setup is not maintained by it, `YAML` CRD parameters can be modified in the manifest.

## Helm Configuration Parameters

| **Name**                           | **Default Value**              | **Description**                                                                              |
|------------------------------------|--------------------------------|----------------------------------------------------------------------------------------------|
| `imagePullSecrets`                 | []                             | List of image pull secrets, in case redis image is getting pull from private registry        |
| `redisCluster.clusterSize`         | 3                              | Size of the redis cluster leader and follower nodes                                          |
| `redisCluster.clusterVersion`      | v7                             | Major version of Redis setup, values can be v6 or v7                                         |
| `redisCluster.persistenceEnabled`  | true                           | Persistence should be enabled or not in the Redis cluster setup                              |
| `redisCluster.secretName`          | redis-secret                   | Name of the existing secret in Kubernetes                                                    |
| `redisCluster.secretKey`           | password                       | Name of the existing secret key in Kubernetes                                                |
| `redisCluster.image`               | quay.io/opstree/redis          | Name of the redis image                                                                      |
| `redisCluster.tag`                 | v7.0.5                         | Tag of the redis image                                                                       |
| `redisCluster.imagePullPolicy`     | IfNotPresent                   | Image Pull Policy of the redis image                                                         |
| `redisCluster.leaderServiceType`   | ClusterIP                      | Kubernetes service type for Redis Leader                                                     |
| `redisCluster.followerServiceType` | ClusterIP                      | Kubernetes service type for Redis Follower                                                   |
| `externalService.enabled`          | false                          | If redis service needs to be exposed using LoadBalancer or NodePort                          |
| `externalService.annotations`      | {}                             | Kubernetes service related annotations                                                       |
| `externalService.serviceType`      | NodePort                       | Kubernetes service type for exposing service, values - ClusterIP, NodePort, and LoadBalancer |
| `externalService.port`             | 6379                           | Port number on which redis external service should be exposed                                |
| `serviceMonitor.enabled`           | false                          | Servicemonitor to monitor redis with Prometheus                                              |
| `serviceMonitor.interval`          | 30s                            | Interval at which metrics should be scraped.                                                 |
| `serviceMonitor.scrapeTimeout`     | 10s                            | Timeout after which the scrape is ended                                                      |
| `serviceMonitor.namespace`         | monitoring                     | Namespace in which Prometheus operator is running                                            |
| `redisExporter.enabled`            | true                           | Redis exporter should be deployed or not                                                     |
| `redisExporter.image`              | quay.io/opstree/redis-exporter | Name of the redis exporter image                                                             |
| `redisExporter.tag`                | v1.44.0                        | Tag of the redis exporter image                                                              |
| `redisExporter.imagePullPolicy`    | IfNotPresent                   | Image Pull Policy of the redis exporter image                                                |
| `redisExporter.env`                | []                             | Extra environment variables which needs to be added in redis exporter                        |
| `sidecars`                         | []                             | Sidecar container to run alongside Redis pods                                                |
| `nodeSelector`                     | {}                             | NodeSelector for redis statefulset                                                           |
| `priorityClassName`                | ""                             | Priority class name for the redis statefulset                                                |
| `storageSpec`                      | {}                             | Storage configuration for redis setup                                                        |
| `securityContext`                  | {}                             | Security Context for redis pods for changing system or kernel level parameters               |
| `affinity`                         | {}                             | Affinity for node and pods for redis statefulset                                             |
| `tolerations`                      | []                             | Tolerations for redis statefulset management                                                 |
