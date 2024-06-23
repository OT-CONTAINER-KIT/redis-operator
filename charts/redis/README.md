# Redis

Redis is a key-value based distributed database, this helm chart is for only standalone setup. This helm chart needs [Redis Operator](../redis-operator) inside Kubernetes cluster. The redis definition can be modified or changed by [values.yaml](./values.yaml).

```shell
helm repo add ot-helm https://ot-container-kit.github.io/helm-charts/
helm install <my-release> ot-helm/redis --namespace <namespace>
```

Redis setup can be upgraded by using `helm upgrade` command:-

```shell
helm upgrade <my-release> ot-helm/redis --install --namespace <namespace>
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

| **Name**                          | **Value**                      | **Description**                                                                               |
|-----------------------------------|--------------------------------|-----------------------------------------------------------------------------------------------|
| `imagePullSecrets`                | []                             | List of image pull secrets, in case redis image is getting pull from private registry         |
| `redisStandalone.secretName`      | redis-secret                   | Name of the existing secret in Kubernetes                                                     |
| `redisStandalone.secretKey`       | password                       | Name of the existing secret key in Kubernetes                                                 |
| `redisStandalone.image`           | quay.io/opstree/redis          | Name of the redis image                                                                       |
| `redisStandalone.tag`             | v6.2                           | Tag of the redis image                                                                        |
| `redisStandalone.imagePullPolicy` | IfNotPresent                   | Image Pull Policy of the redis image                                                          |
| `redisStandalone.serviceType`     | ClusterIP                      | Kubernetes service type for Redis                                                             |
| `redisStandalone.resources`       | {}                             | Request and limits for redis statefulset                                                      |
| `redisStandalone.name`                            | ""                             | Overwrites the name for the charts resources instead of the Release name |
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
| `redisExporter.tag`               | v6.2                           | Tag of the redis exporter image                                                               |
| `redisExporter.imagePullPolicy`   | IfNotPresent                   | Image Pull Policy of the redis exporter image                                                 |
| `redisExporter.env`               | []                             | Extra environment variables which needs to be added in redis exporter                         |
| `sidecars`                        | []                             | Sidecar for redis pods                                                                        |
| `nodeSelector`                    | {}                             | NodeSelector for redis statefulset                                                            |
| `priorityClassName`               | ""                             | Priority class name for the redis statefulset                                                 |
| `storageSpec`                     | {}                             | Storage configuration for redis setup                                                         |
| `securityContext`                 | {}                             | Security Context for redis pods for changing system or kernel level parameters                |
| `affinity`                        | {}                             | Affinity for node and pod for redis statefulset                                               |
| `tolerations`                     | []                             | Tolerations for redis statefulset                                                             |
