# Webhook Configuration Migration Guide

## Overview

Webhook configurations now include the release name prefix to prevent naming conflicts with other operators.

**Only required if you have webhooks enabled (`redisOperator.webhook=true`).**

## Migration Steps

### Helm Installations

```bash
# 1. Delete old webhook configuration
kubectl delete mutatingwebhookconfiguration mutating-webhook-configuration

# 2. Upgrade
helm upgrade redis-operator ot-helm/redis-operator \
  --namespace <namespace> \
  --set redisOperator.webhook=true

# 3. Verify
kubectl get mutatingwebhookconfiguration | grep redis-operator
```
