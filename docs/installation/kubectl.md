# Installing Redis Operator (Kubectl)

If you are beginner with Kubernetes and don't want to go in the complexity of Helm, you can use beloved tool of Kubernetes client like `kubectl`.

- Create the CRD in kubernetes cluster

```bash
kubectl apply -f deploy/crds/redis.opstreelabs.in_redis_crd.yaml
```

- Create role for redis operator

```bash
kubectl apply -f deploy/role.yaml
```

- Create the service-account for redis operator

```bash
kubectl apply -f deploy/service_account.yaml
```

- Create the role bindings for redis operator

```bash
kubectl apply -f deploy/role_binding.yaml
```

- Finally deploy the redis operator

```bash
kubectl apply -f deploy/operator.yaml
```
