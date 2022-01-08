# Exposing Redis

By default, the nature of Redis standalone/cluster setup is private and limited to the Kubernetes cluster only. But we do have a provision to expose it using the Kubernetes "Service" object.
If we can expose the service by doing some configuration inside the helm values for redis standalone and cluster setup. This will create another service in parallel to the internal redis service to expose redis.

The service can be exposed with these service types:-

- **NodePort**: Exposes the Service on each Node's IP at a static port (the NodePort). A ClusterIP Service, to which the NodePort Service routes, is automatically created. You'll be able to contact the NodePort Service, from outside the cluster, by requesting `<NodeIP>:<NodePort>`.
- **LoadBalancer**: Exposes the Service externally using a cloud provider's load balancer. NodePort and ClusterIP Services, to which the external load balancer routes, are automatically created.

## Exposing Service

Customize or create the values file with the following content. The `externalService` configuration is a common method of exposing service for redis standalone and cluster setup.

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

If helm command is completed successfully, we can verify the external service by `kubectl` command. As we can see in the output, there is an IP in the "EXTERNAL-IP" coloumn.

```shell
$ kubectl get svc -n ot-operators
...
NAME                            TYPE           CLUSTER-IP     EXTERNAL-IP      PORT(S)                         AGE
redis-external-service          LoadBalancer   10.103.9.171   164.52.207.101   6379:32247/TCP,9121:30708/TCP   4d20h
```
