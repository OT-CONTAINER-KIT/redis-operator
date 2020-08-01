# Redis Standalone Setup

We can use `helm` and `kubectl` for deploying redis standalone server.

#### Helm

```shell
# Create redis standalone setup
helm upgrade redis ./helm/redis-setup -f ./helm/redis-setup/cluster-values.yaml \
  --set setupMode="standalone" \
  --install --namespace redis-operator
```

#### Kubectl

```shell
# Standalone redis deployment
kubectl apply -f example/redis-standalone-example.yaml
```
