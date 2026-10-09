# Examples (v1beta2)

This directory contains ready-to-apply manifests for common Redis Operator scenarios.

## Core setups

- `redis-standalone.yaml`: Minimal Redis standalone instance.
- `redis-replication.yaml`: Redis replication setup.
- `redis-sentinel.yaml`: Redis Sentinel setup.
- `redis-cluster.yaml`: Redis Cluster setup.

## Configuration and tuning

- `additional_config/`: Additional Redis configuration via `redisConfig`.
- `advance_config/`: Advanced Redis configuration options.
- `env_vars/`: Inject environment variables into Redis containers.
- `probes/`: Custom readiness and liveness probes.
- `sidecar_features/`: Sidecar containers and related settings.
- `volume_mount/`: Additional volume mounts.

## Scheduling and placement

- `affinity/`: Pod affinity and anti-affinity rules.
- `node-selector/`: Node selector settings.
- `tolerations/`: Pod tolerations.
- `topology_spread_constraints/`: Topology spread constraints.
- `redis-cluster-deploy/`: Role-based anti-affinity example for cluster deployment.

## Security

- `password_protected/`: Password-protected Redis.
- `tls_enabled/`: TLS-enabled Redis.
- `acl_config/`: ACL configuration for Redis users.
- `acl-pvc/`: ACL configuration stored on PVC.
- `private_registry/`: Pull images from a private registry.

## Operations and lifecycle

- `backup_restore/`: Backup and restore workflows.
- `disruption_budget/`: PodDisruptionBudget configuration.
- `external_service/`: External service exposure options.
- `pvc_retention_policy/`: PVC retention policy settings.
- `recreate-statefulset/`: StatefulSet recreate strategy.
- `redis_monitoring/`: Monitoring and exporter configuration.
- `upgrade-strategy/`: Upgrade strategy examples.
