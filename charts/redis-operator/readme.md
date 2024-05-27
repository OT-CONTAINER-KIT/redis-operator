# Redis Operator Helm Chart

## Introduction

This Helm chart deploys the redis-operator into your Kubernetes cluster. The operator facilitates the deployment, scaling, and management of Redis clusters and other Redis resources provided by the OpsTree Solutions team.

## Pre-requisites

- Helm v3+
- Kubernetes v1.16+
- If you intend to use the cert-manager, ensure that the cert-manager CRDs are installed before deploying the redis-operator.

## Installation Steps

### 1. Add Helm Repository

```bash
helm repo add ot-helm https://ot-container-kit.github.io/helm-charts
```

### 2. Install Cert-Manager CRDs (if using cert-manager)

If you plan to use cert-manager with the redis-operator, you need to install the cert-manager CRDs before deploying the operator.

```bash
kubectl apply -f https://github.com/cert-manager/cert-manager/releases/download/v1.12.4/cert-manager.crds.yaml
```

### 3. Install Redis Operator

Replace `<YourCertSecretName>` and `<YourPrivateKey>` with your specific values.

```bash
helm install <redis-operator> ot-helm/redis-operator --version=0.15.5 --appVersion=0.15.1 --set certificate.secretName=<YourCertSecretName> --set certmanager.enabled=true --set redisOperator.webhook=true --namespace <redis-operator> --create-namespace
```

> Note: If `certificate.secretName` is not provided, the operator will generate a self-signed certificate and use it for webhook server.
---
> Note : If you want to disable the webhook you have to pass the `--set webhook=false` and `--set certmanager.enabled=false`  while installing the redis-operator.

### 4. Patch the CA Bundle (if using cert-manager)

Cert-manager injects the CA bundle into the webhook configuration.

```bash
kubectl patch crd redis.redis.redis.opstreelabs.in -p '{"metadata":{"annotations":{"cert-manager.io/inject-ca-from":"<redis-operator>/<serving-cert>"}}}'

kubectl patch crd redisclusters.redis.redis.opstreelabs.in -p '{"metadata":{"annotations":{"cert-manager.io/inject-ca-from":"<redis-operator>/<serving-cert>"}}}'

kubectl patch crd redisreplications.redis.redis.opstreelabs.in -p '{"metadata":{"annotations":{"cert-manager.io/inject-ca-from":"<redis-operator>/<serving-cert>"}}}'

kubectl patch crd redissentinels.redis.redis.opstreelabs.in -p '{"metadata":{"annotations":{"cert-manager.io/inject-ca-from":"<redis-operator>/<serving-cert>"}}}'
```

> Note: Replace `<redis-operator>` and `<serving-cert>` with your specific values i.e. release name and certificate name.

#### You can verify the patch by running the following commands

```bash
kubectl get crd redis.redis.redis.opstreelabs.in -o=jsonpath='{.metadata.annotations}'
kubectl get crd redisclusters.redis.redis.opstreelabs.in -o=jsonpath='{.metadata.annotations}'
kubectl get crd redisreplications.redis.redis.opstreelabs.in -o=jsonpath='{.metadata.annotations}'
kubectl get crd redissentinels.redis.redis.opstreelabs.in -o=jsonpath='{.metadata.annotations}'
```

### How to generate private key( Optional )

```bash
openssl req -x509 -nodes -days 365 -newkey rsa:2048 -keyout tls.key -out tls.crt
kubectl create secret tls <webhook-server-cert> --key tls.key --cert tls.crt -n <redis-operator>
```

> Note: This secret will be used for webhook server certificate so generate it before installing the redis-operator.

## Default Values

| Parameter                           | Description                        | Default                                                      |
|-------------------------------------|------------------------------------|--------------------------------------------------------------|
| `redisOperator.name`                | Operator name                      | `redis-operator`                                             |
| `redisOperator.imageName`           | Image repository                   | `quay.io/opstree/redis-operator`                             |
| `redisOperator.imageTag`            | Image tag                          |  `{{appVersion}}`                                                        |
| `redisOperator.imagePullPolicy`     | Image pull policy                  | `Always`                                                     |
| `redisOperator.podAnnotations`        | Additional pod annotations         | `{}`                                                         |
| `redisOperator.podLabels`             | Additional Pod labels             | `{}`                                                         |
| `redisOperator.extraArgs`             | Additional arguments for the operator | `{}`                                                         |
| `redisOperator.watch_namespace`       | Namespace for the operator to watch  | `""`                                                         |
| `redisOperator.env`                  | Environment variables for the operator | `{}`                                                         |
| `redisOperator.webhook`              | Enable webhook                     | `false`                                                     |
| `resources.limits.cpu`              | CPU limit                          | `500m`                                                      |
| `resources.limits.memory`           | Memory limit                       | `500Mi`                                                     |
| `resources.requests.cpu`            | CPU request                        | `500m`                                                      |
| `resources.requests.memory`         | Memory request                     | `500Mi`                                                     |
| `replicas`                          | Number of replicas                 | `1`                                                         |
| `serviceAccountName`                | Service account name               | `redis-operator`                                             |
| `certificate.name`                  | Certificate name                   | `serving-cert`                                               |
| `certificate.secretName`            | Certificate secret name            | `webhook-server-cert`                                      |
| `issuer.type`                      | Issuer type                       | `selfSigned`                                                   |
| `issuer.name`                       | Issuer name                        | `redis-operator-issuer`                                           |
| `issuer.email`                      | Issuer email                       | `shubham.gupta@opstree.com`                                  |
| `issuer.server`                     | Issuer server URL                  | `https://acme-v02.api.letsencrypt.org/directory`            |
| `issuer.privateKeySecretName`       | Private key secret name            | `letsencrypt-prod`                                           |
| `certManager.enabled`              | Enable cert-manager                | `false`                                                       |

## Scheduling Parameters

| Parameter               | Description                                | Default  |
|-------------------------|--------------------------------------------|----------|
| `priorityClassName`     | Priority class name for the pods           | `""`     |
| `nodeSelector`          | Labels for pod assignment                  | `{}`     |
| `tolerateAllTaints`     | Whether to tolerate all node taints         | `false`  |
| `tolerations`           | Taints to tolerate                         | `[]`     |
| `affinity`              | Affinity rules for pod assignment          | `{}`     |
