# Redis Cluster

Redis is a key-value based distributed database, this helm chart is for redis cluster setup. This helm chart needs [Redis Operator](../redis-operator) inside Kubernetes cluster. The redis cluster definition can be modified or changed by [values.yaml](./values.yaml).

```shell
helm repo add ot-helm https://ot-container-kit.github.io/helm-charts/

helm install <my-release> ot-helm/redis-cluster \
    --set redisCluster.clusterSize=3 --namespace <namespace>
```

Redis setup can be upgraded by using `helm upgrade` command:-

```shell
helm upgrade <my-release> ot-helm/redis-cluster --install \
    --set redisCluster.clusterSize=5 --namespace <namespace>
```

For uninstalling the chart:-

```shell
helm delete <my-release> --namespace <namespace>
```

## Pre-Requisities

- Kubernetes 1.15+
- Helm 3.X
- Redis Operator 0.7.0

## Parameters

| **Name**                           | **Default Value**              | **Description**                                                                               |
|------------------------------------|--------------------------------|-----------------------------------------------------------------------------------------------|
| `imagePullSecrets`                 | []                             | List of image pull secrets, in case redis image is getting pull from private registry         |
| `redisCluster.clusterSize`         | 3                              | Size of the redis cluster leader and follower nodes                                           |
| `redisCluster.clusterVersion`      | v7                             | Major version of Redis setup, values can be v6 or v7                                          |
| `redisCluster.persistenceEnabled`  | true                           | Persistence should be enabled or not in the Redis cluster setup                               |
| `redisCluster.secretName`          | redis-secret                   | Name of the existing secret in Kubernetes                                                     |
| `redisCluster.secretKey`           | password                       | Name of the existing secret key in Kubernetes                                                 |
| `redisCluster.image`               | quay.io/opstree/redis          | Name of the redis image                                                                       |
| `redisCluster.tag`                 | v6.2                           | Tag of the redis image                                                                        |
| `redisCluster.imagePullPolicy`     | IfNotPresent                   | Image Pull Policy of the redis image                                                          |
| `redisCluster.leaderServiceType`   | ClusterIP                      | Kubernetes service type for Redis Leader                                                      |
| `redisCluster.followerServiceType` | ClusterIP                      | Kubernetes service type for Redis Follower                                                    |
| `redisCluster.name`                            | ""                             | Overwrites the name for the charts resources instead of the Release name |
| `externalService.enabled`          | false                          | If redis service needs to be exposed using LoadBalancer or NodePort                           |
| `externalService.annotations`      | {}                             | Kubernetes service related annotations                                                        |
| `externalService.serviceType`      | NodePort                       | Kubernetes service type for exposing service, values - ClusterIP, NodePort, and LoadBalancer  |
| `externalService.port`             | 6379                           | Port number on which redis external service should be exposed                                 |
| `serviceMonitor.enabled`           | false                          | Servicemonitor to monitor redis with Prometheus                                               |
| `serviceMonitor.interval`          | 30s                            | Interval at which metrics should be scraped.                                                  |
| `serviceMonitor.scrapeTimeout`     | 10s                            | Timeout after which the scrape is ended                                                       |
| `serviceMonitor.namespace`         | monitoring                     | Namespace in which Prometheus operator is running                                             |
| `redisExporter.enabled`            | true                           | Redis exporter should be deployed or not                                                      |
| `redisExporter.image`              | quay.io/opstree/redis-exporter | Name of the redis exporter image                                                              |
| `redisExporter.tag`                | v6.2                           | Tag of the redis exporter image                                                               |
| `redisExporter.imagePullPolicy`    | IfNotPresent                   | Image Pull Policy of the redis exporter image                                                 |
| `redisExporter.env`                | []                             | Extra environment variables which needs to be added in redis exporter                         |
| `sidecars`                         | []                             | Sidecar for redis pods                                                                        |
| `nodeSelector`                     | {}                             | NodeSelector for redis statefulset                                                            |
| `priorityClassName`                | ""                             | Priority class name for the redis statefulset                                                 |
| `storageSpec`                      | {}                             | Storage configuration for redis setup                                                         |
| `securityContext`                  | {}                             | Security Context for redis pods for changing system or kernel level parameters                |
| `affinity`                         | {}                             | Affinity for node and pods for redis statefulset                                              |
| `tolerations`                      | []                             | Tolerations for redis statefulset                                                             |
