# Failover Testing

Before failover testing, we have to write some dummy data inside the Redis cluster, we can write the dummy data using the `redis-cli`.

```shell
$ kubectl exec -it redis-leader-0 -n redis-operator \
    -- redis-cli -a Opstree@1234 -c set tony stark
...
Defaulting container name to redis-leader.
Use 'kubectl describe pod/redis-leader-0 -n redis-operator' to see all of the containers in this pod.
Warning: Using a password with '-a' or '-u' option on the command line interface may not be safe.
OK
```

Verify the key has been inserted properly by fetching its value.

```shell
$ kubectl exec -it redis-leader-0 -n redis-operator \
    -- redis-cli -a Opstree@1234 -c get tony
...
Use 'kubectl describe pod/redis-leader-0 -n redis-operator' to see all of the containers in this pod.
Warning: Using a password with '-a' or '-u' option on the command line interface may not be safe.
"stark"
```

Let’s restart the pod name `redis-leader-0` and see the redis node behavior.

```shell
$ kubectl delete pod redis-leader-0 -n redis-operator
...
pod "redis-leader-0" deleted
```

Now we can again try to list redis cluster nodes from `redis-leader-0` pod and from some other pod as well like:- `redis-follower-2`

```shell
$ kubectl exec -it redis-leader-0 -n redis-operator \
    -- redis-cli -a Opstree@1234 cluster nodes
...
Defaulting container name to redis-leader.
Use 'kubectl describe pod/redis-leader-0 -n redis-operator' to see all of the containers in this pod.
Warning: Using a password with '-a' or '-u' option on the command line interface may not be safe.
eef84b7dada737051c32d592bd66652b9af0cb35 10.42.2.184:6379@16379 slave 0a36dc5064b0a61afa8bd850e93ff0a1c2267704 0 1619958171517 3 connected
a7c424b5ec0e696aa7be15a691846c8820e48cd1 10.42.1.181:6379@16379 master - 0 1619958172520 4 connected 0-5460
118dbe4f49fa224b7d48fbe71990d054c7e9e853 10.42.0.228:6379@16379 slave 85747fe5cabf96e00fd0365737996a93e05cf947 0 1619958173523 2 connected
50c3f58a1e2911a68b614f6a1a766cc4a7063e95 10.42.0.229:6379@16379 myself,slave a7c424b5ec0e696aa7be15a691846c8820e48cd1 0 1619958172000 4 connected
0a36dc5064b0a61afa8bd850e93ff0a1c2267704 10.42.1.183:6379@16379 master - 0 1619958173000 3 connected 10923-16383
85747fe5cabf96e00fd0365737996a93e05cf947 10.42.2.182:6379@16379 master - 0 1619958173523 2 connected 5461-10922
```

So if you notice the output of cluster nodes command, the node IP is updated and it’s connected as a leader.

Let's try to get value of key from some other pod

```shell
$ kubectl exec -it redis-follower-1 -n redis-operator \
    -- redis-cli -a Opstree@1234 -c get tony
...
Defaulting container name to redis-follower.
Use 'kubectl describe pod/redis-follower-1 -n redis-operator' to see all of the containers in this pod.
Warning: Using a password with '-a' or '-u' option on the command line interface may not be safe.
"stark"
```
