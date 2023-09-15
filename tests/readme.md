# E2E Testing of the Redis Cluster with Kuttl

This guide will walk you through the process of end-to-end (E2E) testing a Redis cluster using the `kuttl` utility.

## **Prerequisites**

Ensure you have the following tools installed:

- **kuttl**: For testing. [Installation Guide](https://kuttl.dev/docs/installation/)
- **kind**: For local Kubernetes clusters. [Installation Guide](https://kind.sigs.k8s.io/docs/user/quick-start/#installation)
- **kubectl**: Kubernetes command-line tool. [Installation Guide](https://kubernetes.io/docs/tasks/tools/install-kubectl/)
- **helm**: Package manager for Kubernetes. [Installation Guide](https://helm.sh/docs/intro/install/)

## **Steps**

### **1. Set Up a 3-node Kind Cluster**

## Steps to Follow

### 1. Set Up a 3-node Kind Cluster

Create a 3-node kind cluster using the provided configuration:

```bash
kind create cluster --config /redis-operator/tests/_config/kind-example-config.yaml
```

### 2. Install the Redis Operator

To install the Redis operator, utilize the Helm chart from the repository provided:

- [OT-CONTAINER-KIT Redis Operator Helm Chart](https://github.com/OT-CONTAINER-KIT/helm-charts/tree/main/charts/redis-operator#readme)

Please refer to the repository's README for detailed instructions on installing the operator using Helm.

### 3. Execute Kuttl Test

Execute the kuttl test using the following command:

```bash
kubectl kuttl test redis-operator/tests/e2e/v1beta2 --config /redis-operator/tests/_config/kuttl-test.yaml --timeout 10m
```
