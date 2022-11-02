---
title: "Failover Testing"
linkTitle: "Failover Testing"
weight: 30
date: 2022-11-02T00:19:19Z
description: >
  Instructions for testing the failover of Redis cluster
---

For cluster setup, testing can be performed to validate the failover functionality of Redis. In the failover testing, we can set some random keys inside the redis cluster and then delete one or two pods from the redis cluster. At that particular time, we can make some calls to redis for fetching the key to observing its failover mechanism of it.

Before failover testing, we have to write some dummy data inside the Redis cluster, we can write the dummy data using the `redis-cli`.

```shell
$ kubectl exec -it redis-cluster-leader-0 -n ot-operators \
    -- redis-cli -c set tony stark
...
OK
```

Verify the key has been inserted properly inside the redis by fetching its value. Again we will use `redis-cli` for fetching the key from redis.

```shell
$ kubectl exec -it redis-cluster-leader-0 -n ot-operators \
    -- redis-cli -c get tony
...
"stark"
```

To validate the failover functionality, we need to delete few of the pods from the redis cluster. `kubectl` cli could be use for deleting pods from the cluster.

```shell
$ kubectl delete pod redis-cluster-leader-0 -n ot-operators
...
pod "redis-cluster-leader-0" deleted
```

Since we have restarted `redis-cluster-leader-0` pod, we will again list out the redis nodes using `redis-cli` to see if follower node attached to it is promoted as leader or not. Also, the leader role should have been changed to follower role.

```shell
$ kubectl exec -it redis-cluster-leader-0 -n ot-operators \
    -- redis-cli cluster nodes
...
eef84b7dada737051c32d592bd66652b9af0cb35 10.42.2.184:6379@16379 slave 0a36dc5064b0a61afa8bd850e93ff0a1c2267704 0 1619958171517 3 connected
a7c424b5ec0e696aa7be15a691846c8820e48cd1 10.42.1.181:6379@16379 master - 0 1619958172520 4 connected 0-5460
118dbe4f49fa224b7d48fbe71990d054c7e9e853 10.42.0.228:6379@16379 slave 85747fe5cabf96e00fd0365737996a93e05cf947 0 1619958173523 2 connected
50c3f58a1e2911a68b614f6a1a766cc4a7063e95 10.42.0.229:6379@16379 myself,slave a7c424b5ec0e696aa7be15a691846c8820e48cd1 0 1619958172000 4 connected
0a36dc5064b0a61afa8bd850e93ff0a1c2267704 10.42.1.183:6379@16379 master - 0 1619958173000 3 connected 10923-16383
85747fe5cabf96e00fd0365737996a93e05cf947 10.42.2.182:6379@16379 master - 0 1619958173523 2 connected 5461-10922
```

So if you notice the output of cluster nodes command, the node IP is updated, and it’s connected as a leader.

```shell
$ kubectl exec -it redis-cluster-follower-1 -n ot-operators \
    -- redis-cli -c get tony
...
"stark"
```
