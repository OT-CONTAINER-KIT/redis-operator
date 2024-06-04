# E2E Testing of the Redis Cluster with Kuttl

> ***Kuttl is depracated!*** now we use chainsaw to run e2e tests. Please refer to the [chainsaw](https://github.com/kyverno/chainsaw).

This guide will walk you through the process of end-to-end (E2E) testing a Redis cluster using the `kuttl` utility.

## **Prerequisites**

Ensure you have the following tools installed:

- **kuttl**: For testing. [Installation Guide](https://kuttl.dev/docs/installation/)
- **kind**: For local Kubernetes clusters. [Installation Guide](https://kind.sigs.k8s.io/docs/user/quick-start/#installation)
- **kubectl**: Kubernetes command-line tool. [Installation Guide](https://kubernetes.io/docs/tasks/tools/install-kubectl/)
- **helm**: Package manager for Kubernetes. [Installation Guide](https://helm.sh/docs/intro/install/)

## **Steps**

## Steps to Follow

### 1. Set Up a 3-node Kind Cluster

Create a 3-node kind cluster using the provided configuration:

```bash
kind create cluster --config tests/_config/kind-config.yaml
```

### 2. Install the Redis Operator

To install the Redis operator, utilize the Helm chart from the repository provided:

- [OT-CONTAINER-KIT Redis Operator Helm Chart](https://github.com/OT-CONTAINER-KIT/helm-charts/tree/main/charts/redis-operator#readme)

Please refer to the repository's README for detailed instructions on installing the operator using Helm.

### 3. Execute Kuttl Test

Execute the kuttl test using the following command:

To run all default tests ( \_config/kuttl-test.yaml is the default config file )

```bash
kubectl kuttl test --config tests/_config/kuttl-test.yaml
```

To run a test at specified path

```bash
kubectl kuttl test tests/e2e/v1beta2 --config tests/_config/kuttl-test.yaml --timeout 600
```
