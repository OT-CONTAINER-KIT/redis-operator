---
title: "Exposing Redis Service"
linkTitle: "Exposing Redis Service"
weight: 10
date: 2022-11-02T00:19:19Z
description: >
  Instructions for exposing redis outside Kubernetes cluster
---

By default, the nature of Redis standalone/cluster setup is private and limited to the Kubernetes cluster only. But we do have a provision to expose it using the Kubernetes "Service" object. If we can expose the service by doing some configuration inside the helm values for redis standalone and cluster setup. This will create another service in parallel to the internal redis service to expose redis.

The service can be exposed with these service types:

- **NodePort:** Exposes the Service on each Node's IP at a static port (the NodePort). A ClusterIP Service, to which the NodePort Service routes, is automatically created. You'll be able to contact the NodePort Service, from outside the cluster, by requesting <NodeIP>:<NodePort>.
- **LoadBalancer:** Exposes the Service externally using a cloud provider's load balancer. NodePort and ClusterIP Services, to which the external load balancer routes, are automatically created.

{{< alert color="info" title="Context" >}}
Examples below are labeled as Helm values or CRD manifests. Use the CRD form if you install the operator once and apply Redis custom resources directly.
{{< /alert >}}

## Exposing Service

### Helm values (charts)

Customize or create the values file with the following content. The externalService configuration is a common method of exposing service for redis standalone and cluster setup:

```yaml
externalService:
  enabled: true
  annotations:
    service.beta.kubernetes.io/aws-load-balancer-internal: 0.0.0.0/0
  serviceType: LoadBalancer
  port: 6379
```

Once the values file is customized or created we can apply or upgrade the redis setup. We need to pass the created file as an argument to the `helm` command.

```shell
# redis standalone
$ helm upgrade redis ot-helm/redis -f custom-values.yaml \
    --install --namespace ot-operators

# redis cluster
$ helm upgrade redis-cluster ot-helm/redis-cluster -f custom-values.yaml \
  --set redisCluster.clusterSize=3 --install --namespace ot-operators
```

### CRD manifest (operator)

If you apply CRDs directly (no chart), configure the additional service on the custom resource. This creates a `-additional` Service for external access.

```yaml
apiVersion: redis.redis.opstreelabs.in/v1beta2
kind: RedisCluster
metadata:
  name: redis-cluster
spec:
  clusterSize: 3
  kubernetesConfig:
    image: quay.io/opstree/redis:v7.0.5
    imagePullPolicy: IfNotPresent
    service:
      serviceType: LoadBalancer
      annotations:
        service.beta.kubernetes.io/aws-load-balancer-internal: 0.0.0.0/0
      additional:
        enabled: true
```

For standalone Redis, use `kind: Redis` with the same `spec.kubernetesConfig.service` block.

Once applied, we can verify the external service by kubectl command. As we can see in the output, there is an IP in the "EXTERNAL-IP" column.

```shell
$ kubectl get svc -n ot-operators
...
NAME                            TYPE           CLUSTER-IP     EXTERNAL-IP      PORT(S)                         AGE
redis-external-service          LoadBalancer   10.103.9.171   164.52.207.101   6379:32247/TCP,9121:30708/TCP   4d20h
```
