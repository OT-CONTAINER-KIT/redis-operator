# Redis Cluster Setup

For setting up redis cluster we can use `helm` and `kubectl`. The only thing which needs to be taken care is that minimum count for cluster is 3.

#### Helm

```shell
# Create redis cluster setup
helm upgrade redis-cluster ./helm/redis-setup -f ./helm/redis-setup/cluster-values.yaml \
  --set setupMode="cluster" --set cluster.size=3 \
  --install --namespace redis-operator
```

#### Kubectl

```shell
# Standalone redis deployment
kubectl apply -f example/redis-cluster-example.yaml
```
