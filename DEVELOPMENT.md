## Reference Link
https://github.com/joatmon08/hello-stateful-operator/tree/master/pkg/apis/hello-stateful

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

### 4. Add Interface logic in your code

Updated interface logic in below file

```
redis-operator/redis-operator/pkg/apis/redis/v1alpha1/redis_types.go
```

### 5. Updated container information in controller

Updated the container definition in Redis controller

```
redis-operator/redis-operator/pkg/controller/redis/redis_controller.go
```

### 6. Created an object to generate statefulset definitions

```
redis-operator/redis-operator/pkg/controller/redis/redis_controller.go
```

### 7. Created an object to generate service definition

```
redis-operator/redis-operator/pkg/controller/redis/redis_controller.go
```

### 8. Created a common meta information function

```
redis-operator/redis-operator/pkg/controller/redis/redis_controller.go
```
