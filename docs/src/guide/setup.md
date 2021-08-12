# Redis Setup

This redis operator supports below deployment strategies for redis:-

- Redis cluster setup (in-built leader follower with sharding and replication mode)
- Redis standalone setup

Here we will see how we can leverage these strategies.

If we want to use password based authentication inside Redis, we need to create a secret for it. By default the name of the secret is `redis-secret` and key name is `password`, but it can be overidden in helm charts.

```shell
$ kubectl create secret generic redis-secret \ 
    --from-literal=password=password -n ot-operators
```

## Redis Standalone

<div align="center">
    <img src="./images/redis-standalone.png" height="400" width="400">
</div>

In redis standalone mode, we deploy redis as a single Stateful pod which means ease of setup, no complexity, no high availability, and no resilience.

```shell
$ helm upgrade redis ot-helm/redis --install --namespace ot-operators
...
Release "redis" does not exist. Installing it now.
NAME: redis
LAST DEPLOYED: Sun May  2 15:59:48 2021
NAMESPACE: ot-operators
STATUS: deployed
REVISION: 1
TEST SUITE: None
```

Verify the standalone redis setup by `kubectl` command line.

```shell
$ kubectl get pods -n ot-operators
...
NAME                              READY   STATUS    RESTARTS   AGE
redis-operator-74b6cbf5c5-td8t7   1/1     Running   0          81m
redis-standalone-0                2/2     Running   0          56s
```

## Redis Cluster

<div align="center">
    <img src="./images/redis-cluster-setup.png" height="450" width="400">
</div>

A Redis cluster is simply a [data sharding strategy](https://www.digitalocean.com/community/tutorials/understanding-database-sharding). It automatically partitions data across multiple Redis nodes. It is an advanced feature of Redis which achieves distributed storage and prevents a single point of failure.

For redis cluster setup we can use same helm command but with different parameters.

```shell
$ helm upgrade redis-cluster ot-helm/redis-cluster \
  --set redisCluster.clusterSize=3 --install --namespace ot-operators
...
Release "redis-cluster" does not exist. Installing it now.
NAME: redis-cluster
LAST DEPLOYED: Sun May  2 16:11:38 2021
NAMESPACE: ot-operators
STATUS: deployed
REVISION: 1
TEST SUITE: None
```

Verify the cluster by checking the pod status of leader and follower pods.

```shell
$ kubectl get pods -n ot-operators
...
NAME                                 READY   STATUS    RESTARTS   AGE
redis-cluster-follower-0             1/1     Running   0          149m
redis-cluster-follower-1             1/1     Running   0          150m
redis-cluster-follower-2             1/1     Running   0          151m
redis-cluster-leader-0               1/1     Running   0          149m
redis-cluster-leader-1               1/1     Running   0          150m
redis-cluster-leader-2               1/1     Running   0          151m
redis-operator-5944ffd957-pt57s      1/1     Running   0          156m
```

If all the pods are in the running state of leader and follower Statefulsets, then we can check the health of the redis cluster by using `redis-cli`.

```shell
$ kubectl exec -it redis-leader-0 -n ot-operators -- redis-cli -a Opstree@1234 cluster nodes
...
Defaulting container name to redis-leader.
Use 'kubectl describe pod/redis-leader-0 -n ot-operators' to see all of the containers in this pod.
Warning: Using a password with '-a' or '-u' option on the command line interface may not be safe.
528438a759cee4528c3071d17d75b27b0818555d 10.42.0.219:6379@16379 myself,master - 0 1619952294000 1 connected 0-5460
8ec7812903b7e046bec2f2a7bce4a9ccadfa4188 10.42.0.221:6379@16379 slave d0ff3892d2eba0b2707199cb5df57adbba214bcd 0 1619952297241 3 connected
60f932272322bafbd8c3e16328d26af676aeb8d6 10.42.0.220:6379@16379 slave 6e80da4902802ebffa94cbac9b7d98e9fd74121f 0 1619952297000 2 connected
6e80da4902802ebffa94cbac9b7d98e9fd74121f 10.42.2.178:6379@16379 master - 0 1619952297000 2 connected 5461-10922
d0ff3892d2eba0b2707199cb5df57adbba214bcd 10.42.1.178:6379@16379 master - 0 1619952298245 3 connected 10923-16383
c2b74bd2a360068db01dfc8f00b8d0b012e21215 10.42.1.177:6379@16379 slave 528438a759cee4528c3071d17d75b27b0818555d 0 1619952297000 1 connected
```
