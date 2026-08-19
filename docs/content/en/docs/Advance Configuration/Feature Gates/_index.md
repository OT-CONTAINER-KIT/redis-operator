---
title: "Feature Gates"
linkTitle: "Feature Gates"
weight: 40
date: 2024-03-20T00:19:19Z
description: >
  Configuration guide for Redis Operator feature gates
---

Redis Operator supports feature gates to enable alpha/experimental features. These can be configured in the Helm chart values.

## Configuration

Feature gates can be configured in the Helm chart values:

```yaml
featureGates:
  # Enable generating Redis configuration using an init container instead of a regular container
  GenerateConfigInInitContainer: false
  # Never execute redis-cli -a <password>, even if authentication cannot succeed without it
  AvoidCommandLinePassword: false
```

## Available Feature Gates

### AvoidCommandLinePassword

When enabled, Redis Operator will never execute `redis-cli -a <password>`, which can leak passwords. The Operator sets the
`REDISCLI_AUTH` variable on all Redis pods, so the password does not need to be provided on the command line and it is normally
safe to turn this on unless you are simultaneously upgrading the operator. This is an alpha feature and may change in future releases.

However, if you upgrade from a version that does not add `REDISCLI_AUTH` to the pods (a behavior introduced in the same version that
added `AvoidCommandLinePassword`), simultaneously enabling `AvoidCommandLinePassword` will make Redis Operator unable to manage
your current pods, since `-a <password>` is still needed on them. Hence, to guarantee that the Redis password will never be included
on a command line, you must either risk an operator downtime or upgrade in two steps:

1. Upgrade to a version that adds `REDISCLI_AUTH` to the pods (which was introduced at the same time as `AvoidCommandLinePassword`).
2. Turn on `AvoidCommandLinePassword`.

**Default**: `false`

**Usage**:
```yaml
featureGates:
  AvoidCommandLinePassword: true
```

### GenerateConfigInInitContainer

When enabled, Redis configuration will be generated using an init container instead of a regular container. This is an alpha feature and may change in future releases.

**Default**: `false`

**Usage**:
```yaml
featureGates:
  GenerateConfigInInitContainer: true
```

## Feature Gate Lifecycle

Feature gates follow a standard lifecycle:

1. **Alpha**: Features are disabled by default and may be changed in incompatible ways in a later software release without notice.
2. **Beta**: Features are enabled by default and may be changed in incompatible ways in a later software release.
3. **GA**: Features are enabled by default and will not be changed in incompatible ways in a later software release.

{{< alert color="warning" title="Warning" >}}
Alpha features are experimental and may be changed or removed in future releases. Use them with caution in production environments.
{{< /alert >}}

## Adding New Feature Gates

When adding new feature gates to Redis Operator:

1. Define the feature gate in `internal/features/features.go`
2. Add the feature gate to `DefaultRedisOperatorFeatureGates`
3. Update the Helm chart values to include the new feature gate
4. Update this documentation with the new feature gate details