# Installing Redis Operator (Helm)

The easiest way to install redis operator is using Helm chart. The operator helm chart is developed on helm `=>3.0.0` version.

```bash
# Deploy the redis-operator
helm upgrade redis-operator ./helm/redis-operator --install --namespace redis-operator
```

After deployment, verify the installation of operator

```shell
# Testing the redis operator
helm test redis-operator --namespace redis-operator
```
