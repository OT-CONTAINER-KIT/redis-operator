---
title: "Architecture"
linkTitle: "Architecture"
weight: 2
date: 2026-02-16T00:00:00Z
description: >
  High-level architecture and reconciliation flow for Redis Operator.
---

This page provides a high-level view of how Redis Operator turns a CR into Kubernetes resources.

```mermaid
flowchart TB
  CR["Redis / RedisCluster / RedisSentinel CR"] --> Controller["Controller / Reconciler"]
  Controller --> STS["StatefulSet"]
  Controller --> SVC["Service"]
  Controller --> PVC["PVCs"]
  Controller --> CM["ConfigMaps / Secrets"]
  Controller --> PDB["PodDisruptionBudget"]
  STS --> Exporter["redis-exporter (optional)"]
```

## Reconciliation flow

- You apply or update a Redis custom resource.
- The controller reconciles desired state into Kubernetes primitives.
- StatefulSets manage Redis pods and their storage.
- Services and PDBs provide networking and disruption protection.
- ConfigMaps and Secrets provide configuration and credentials.
