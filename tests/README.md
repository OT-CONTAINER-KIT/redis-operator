# E2E Testing of the Redis Cluster with Chainsaw

This guide will walk you through the process of end-to-end (E2E) testing a Redis cluster using the `chainsaw` utility.

## **Prerequisites**

Ensure you have the following tools installed:

- **chainsaw**: For testing. [Installation Guide](https://kyverno.github.io/chainsaw/latest/quick-start/install/)
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

### 3. Execute Chainsaw Test

Execute the chainsaw test using the following command:

To run all default tests ( \_config/chainsaw-configuration.yaml is the default config file )

```bash
chainsaw test tests/e2e-chainsaw/v1beta2 --config tests/_config/chainsaw-configuration.yaml
```

## **Data Assert**

We assert the data in the redis cluster && redis replication test cases.

The data assert have two actions:

1. **Put data to redis**
2. **Check data in redis**