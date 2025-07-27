---
title: "API Reference"
linkTitle: "API Reference"
weight: 20
date: 2025-01-27
description: >
  API reference
---

This section contains the API reference documentation generated using the `elastic/crd-ref-docs` tool, which provides improved documentation quality and maintenance.

## API Groups Covered

- `redis.redis.opstreelabs.in/v1beta2`
- `rediscluster.redis.opstreelabs.in/v1beta2`
- `redisreplication.redis.opstreelabs.in/v1beta2`
- `redissentinel.redis.opstreelabs.in/v1beta2`
- `common.redis.opstreelabs.in/v1beta2`

## How to Generate

To regenerate this documentation, run:

```bash
make generate-api-docs
```

Or directly:

```bash
hack/api-docs/build.sh
```