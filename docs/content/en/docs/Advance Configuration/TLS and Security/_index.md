---
title: "TLS and Security"
linkTitle: "TLS and Security"
weight: 30
date: 2022-11-02T00:19:19Z
description: >
  Instructions for configuring TLS and security by Redis Operator
---

## Securing redis setup with password

If we want to use password based authentication inside Redis, we need to create a secret for it. By default, operator doesn't apply password on standalone or cluster. We need to enable the password login via `helm` command or `YAML` manifests.

```shell
$ kubectl create secret generic redis-secret \
    --from-literal=password=password -n ot-operators
```

For users that are managing Redis setup using `YAML` manifest, they need to define `redisSecret` inside the object of Redis and RedisCluster. For further details, please check [here](../../crd-reference/redis-api/#existingpasswordsecret).

```yaml
spec:
  kubernetesConfig:
    redisSecret:
      name: redis-secret
      key: password
```

With `helm`, the password configuration can be defined inside the values file and also can be passed using `helm --set` command.

Password configuration for Redis standalone:

```shell
$ helm install redis ot-helm/redis --namespace ot-operators \
  --set redisStandalone.redisSecret.secretName=redis-secret \
  --set redisStandalone.redisSecret.secretKey=password
```

Password configuration for Redis cluster:

```shell
$ helm install redis ot-helm/redis-cluster --namespace ot-operators \
  --set redisCluster.redisSecret.secretName=redis-secret \
  --set redisCluster.redisSecret.secretKey=password
```

Once the password configuration is applied to the redis setup, we can perform the validation using the combination of `kubectl` and `redis-cli`.

```shell
$ kubectl exec -it redis-0 -n ot-operators -c redis -- redis-cli info
...
NOAUTH Authentication required.
```

Try passing the password to the redis setup:

```shell
$ kubectl exec -it redis-0 -n ot-operators -c redis -- redis-cli -a password info
...
# Server
redis_version:7.0.5
redis_git_sha1:00000000
redis_git_dirty:0
redis_build_id:90d2ef529791ba03
redis_mode:standalone
os:Linux 5.4.209-116.367.amzn2.x86_64 x86_64
arch_bits:64
.......
```

## TLS configuration for redis setup

TLS is a security protocol that makes packet and network transfer encrypted between server and client architecture. In the redis setup, we can add TLS as a part of an additional security layer, and along with username and password, the TLS parameters also need to be passed to the client for server authentication.

The architecture with TLS setup looks like this:

<div align="center">
    <img src="../../../static/images/redis-tls.png">
</div>

### Certificate creation

TLS certificates can be purchased or self-signed, but both can be integrated with the redis setup. In Kubernetes, we can install [cert-manager,](https://cert-manager.io/docs/) and it can be used for generating the certificates inside Kubernetes for redis and different other applications.

First, we need to create an issuer inside the Kubernetes to issue the certificates for redis TLS integration.

```yaml
---
apiVersion: cert-manager.io/v1
kind: Issuer
metadata:
  name: selfsigned-issuer
spec:
  selfSigned: {}
---
apiVersion: cert-manager.io/v1
kind: Issuer
metadata:
  name: redis-tls-ca
spec:
  ca:
    secretName: redis-tls-ca-cert
```

```shell
$ kubectl apply -f issuer.yaml -n ot-operators
...
issuer.cert-manager.io/redis-tls-ca created
```

Verify the issuer by using `kubectl` command:

```shell
$ kubectl get issuers -n ot-operators
```

Once the issuer configuration is done, we need to create a `Certificate` object to create certificate for Redis.

```yaml
---
apiVersion: cert-manager.io/v1
kind: Certificate
metadata:
  name: redis-tls-ca
spec:
  isCA: true
  commonName: redis
  secretName: redis-tls-ca-cert
  issuerRef:
    name: selfsigned-issuer
    kind: Issuer
    group: cert-manager.io
---
apiVersion: cert-manager.io/v1
kind: Certificate
metadata:
  name: redis-tls  # this name should match the one appeared in kustomizeconfig.yaml
spec:
  dnsNames:
    - redis-headless.ot-operators.svc.cluster.local
    - redis-headless.ot-operators.svc
    - redis-headless
  issuerRef:
    kind: Issuer
    name: redis-tls-ca
    group: cert-manager.io
  secretName: redis-tls-cert
```

Create the defined `YAML` object inside the Kubernetes cluster using `kubectl` command:

```shell
$ kubectl apply -f certificate.yaml -n ot-operators
...
certificate.cert-manager.io/redis-tls-ca created
certificate.cert-manager.io/redis-tls created
```

Once the certificates are in ready, we can verify if the TLS secret is created or not by cert-manager.

```shell
$ kubectl get certificates -n ot-operators
...
NAME           READY   SECRET              AGE
redis-tls      True    redis-tls-cert      108s
redis-tls-ca   True    redis-tls-ca-cert   109s
```

```shell
$ kubectl get secrets -n ot-operators
...
NAME             TYPE                DATA   AGE
redis-tls-cert   kubernetes.io/tls   3      18m
```

### Redis TLS configuration

Redis TLS configuration can be done via `YAML` manifests and `helm` values file. We need to add details about the certificate to the Redis and RedisCluster objects.

For `YAML` manifest configuration, we need to define the TLS block like this:

```yaml
spec:
  TLS:
    secret:
      secretName: redis-tls-cert
      optional: false
```

For `helm upgrade` method we need to update the values file of `Redis` and `RedisCluster`.
