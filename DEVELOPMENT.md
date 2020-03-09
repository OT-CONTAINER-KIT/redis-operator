### 1. Generated Operator directory structure

This will generate you a directory structure for you
```shell
operator-sdk new redis-operator
```

### 2. Add api to the directory structure

```shell
operator-sdk add api --api-version=redis.opstreelabs.in/v1alpha1 --kind=Redis
```

### 3. Add controller to the directory structure

```shell
operator-sdk add controller --api-version=redis.opstreelabs.in/v1alpha1 --kind=Redis
```
