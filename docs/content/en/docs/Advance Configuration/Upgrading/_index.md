---
title: "Upgrading Redis and RedisCluster"
linkTitle: "Upgrading Redis and RedisCluster"
weight: 20
date: 2022-11-02T00:19:19Z
description: >
  Instructions for upgrading the redis and redis cluster
---

The upgrade strategy for standalone Redis includes the downtime of the system but for cluster setup mode any application will not face any type of issues or downtime. The operator uses the rolling deployment strategy for cluster setup where it will upgrade the redis leader pods and once the leader is upgraded it will follow the same strategy for redis follower pods.

## Upgrade of standalone setup

For upgrading the standalone setup of Redis, first we need to identify the current version of it and that can be done via combination of `kubectl` and `redis-server` command.

```shell
$ kubectl exec -it redis-0 -n ot-operators -c redis -- redis-server --version
...
Redis server v=6.2.5 sha=00000000:0 malloc=jemalloc-5.1.0 bits=64 build=3e393a4e624651a3
```

Now let's say the new version, we want to migrate is `7.0.5`. In that case, we can simply use `helm upgrade` command to upgrade the redis cluster.

```shell
$ helm upgrade redis ot-helm/redis --namespace ot-operators \
  --set redisStandalone.tag=v7.0.5
...
Release "redis" has been upgraded. Happy Helming!
NAME:          redis
LAST DEPLOYED: Wed Nov  2 19:58:42 2022
NAMESPACE:     ot-operators
STATUS:        deployed
REVISION:      2
TEST SUITE:    None
```

Once the upgrade strategy is completed, we can verify the redis pod status by executing:

```shell
$ kubectl get pods -n ot-operators
...
NAME                              READY   STATUS    RESTARTS   AGE
redis-0                           2/2     Running   0          6m26s
```

Verify the version of redis pod by using the same cli command.

```shell
$ kubectl exec -it redis-0 -n ot-operators -c redis -- redis-server --version
...
Redis server v=7.0.5 sha=00000000:0 malloc=jemalloc-5.2.1 bits=64 build=90d2ef529791ba03
```

For YAML manifest based upgrade, please update the `spec` section of Redis Object. For further details check [here](../../crd-reference/redis-api/#kubernetesconfig).

```yaml
spec:
  kubernetesConfig:
    image: "quay.io/opstree/redis:v7.0.15"
    imagePullPolicy: "IfNotPresent"
```

**Things to keep in mind:**

{{< alert color="info" title="Note" >}}
- Standalone upgrade introduces downtime, so make sure you have strategy in-place. A better option could be deploying a new redis standalone alongside and point the application to it.
- Your applications should be compatible with the new version of Redis on which you are upgrading.
{{< /alert >}}

## Upgrading cluster setup

Similar to standalone setup upgrade, the cluster setup upgrade can also be performed by `helm` command. But again first we need to verify the version of existing cluster setup.

```shell
$ kubectl get pods -n ot-operators
...
NAME                              READY   STATUS    RESTARTS   AGE
redis-cluster-follower-0          2/2     Running   0          2m22s
redis-cluster-follower-1          2/2     Running   0          2m9s
redis-cluster-follower-2          2/2     Running   0          102s
redis-cluster-leader-0            2/2     Running   0          2m22s
redis-cluster-leader-1            2/2     Running   0          2m9s
redis-cluster-leader-2            2/2     Running   0          101s
```

As an initial step of Redis cluster upgrade, first we need to check the version of `redis` and also the health of cluster. Again this can be achieved by `kubectl` and `redis-cli`.

```shell
$ kubectl exec -it redis-cluster-leader-0 -c redis-cluster-leader \
  -n ot-operators -- redis-server --version
...
Redis server v=6.2.5 sha=00000000:0 malloc=jemalloc-5.1.0 bits=64 build=3e393a4e624651a3
```

```shell
$ kubectl exec -it redis-cluster-leader-0 -c redis-cluster-leader \
  -n ot-operators -- redis-cli cluster nodes
...
be337e56dbcbdcf47af569e53a0f0316f8e5cd28 192.168.94.6:6379@16379 slave af958687b0048c734b13cef5632ab2b46e386fb1 0 1667400526029 3 connected
2321a671fba363fe55821cd133d4b70fbdeba713 192.168.3.127:6379@16379 myself,master - 0 1667400527000 1 connected 0-5460
93eb534f0e748b04ff4be6fe984173cdc65703eb 192.168.37.210:6379@16379 slave 670c7c36d3d89c93d4bc546e6ee02b2b843d8801 0 1667400527536 2 connected
a00493797d566f2cb1f861962e94fee453d23857 192.168.2.131:6379@16379 slave 2321a671fba363fe55821cd133d4b70fbdeba713 0 1667400527033 1 connected
670c7c36d3d89c93d4bc546e6ee02b2b843d8801 192.168.33.178:6379@16379 master - 0 1667400526000 2 connected 5461-10922
af958687b0048c734b13cef5632ab2b46e386fb1 192.168.72.57:6379@16379 master - 0 1667400527000 3 connected 10923-16383
```

Once the version and cluster health is verified, we can trigger the upgrade of the cluster by using `helm` command. Let's upgrade the cluster version to v7.

```shell
$ helm upgrade redis-cluster ot-helm/redis-cluster \
  --set redisCluster.clusterSize=3 --install --namespace ot-operators \
  --set redisCluster.tag=v7.0.5 --set redisCluster.clusterVersion=v7
...
Release "redis-cluster" has been upgraded. Happy Helming!
NAME:          redis-cluster
LAST DEPLOYED: Wed Nov  2 20:21:35 2022
NAMESPACE:     ot-operators
STATUS:        deployed
REVISION:      2
TEST SUITE:    None
```

Once the upgrade using helm is completed, we need to verify the redis cluster pod status. Also, we need to check the version and health of redis cluster to verify the upgrade is successful or not.

```shell
$ kubectl get pods -n ot-operators
...
NAME                              READY   STATUS    RESTARTS   AGE
redis-cluster-follower-0          2/2     Running   0          78s
redis-cluster-follower-1          2/2     Running   0          2m5s
redis-cluster-follower-2          2/2     Running   0          2m49s
redis-cluster-leader-0            2/2     Running   0          77s
redis-cluster-leader-1            2/2     Running   0          2m4s
redis-cluster-leader-2            2/2     Running   0          2m50s
```

```shell
$ kubectl exec -it redis-cluster-leader-0 -c redis-cluster-leader \
  -n ot-operators -- redis-server --version
...
Redis server v=7.0.5 sha=00000000:0 malloc=jemalloc-5.2.1 bits=64 build=90d2ef529791ba03
```

```shell
$ kubectl exec -it redis-cluster-leader-0 -c redis-cluster-leader \
  -n ot-operators -- redis-cli cluster nodes
...
af958687b0048c734b13cef5632ab2b46e386fb1 192.168.90.191:6379@16379 master - 0 1667400984185 3 connected 10923-16383
be337e56dbcbdcf47af569e53a0f0316f8e5cd28 192.168.65.215:6379@16379 slave af958687b0048c734b13cef5632ab2b46e386fb1 0 1667400983000 3 connected
a00493797d566f2cb1f861962e94fee453d23857 192.168.8.246:6379@16379 slave 2321a671fba363fe55821cd133d4b70fbdeba713 0 1667400984184 1 connected
670c7c36d3d89c93d4bc546e6ee02b2b843d8801 192.168.32.65:6379@16379 master - 0 1667400983180 2 connected 5461-10922
2321a671fba363fe55821cd133d4b70fbdeba713 192.168.2.134:6379@16379 myself,master - 0 1667400983000 1 connected 0-5460
93eb534f0e748b04ff4be6fe984173cdc65703eb 192.168.52.229:6379@16379 slave 670c7c36d3d89c93d4bc546e6ee02b2b843d8801 0 1667400984687 2 connected
```

For YAML manifest based upgrade, please update the `spec` section of Redis Object. For further details check [here](../../crd-reference/redis-api/#kubernetesconfig).

```yaml
spec:
  kubernetesConfig:
    image: "quay.io/opstree/redis:v7.0.15"
    imagePullPolicy: "IfNotPresent"
```


**Things to keep in mind:**

{{< alert color="info" title="Note" >}}
- Cluster upgrade doesn't cause any kind of downtime because of rolling update strategy of Kubernetes, there will be always a redis available to serve application requests.
- If application is highly critical, in such scenarios it would make sense to create a new cluster and migrate the application pointing to it
{{< /alert >}}

