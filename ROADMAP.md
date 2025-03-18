# Phase 1: Feature Expansion and Advanced Configuration (0-6 months)

- Advance Backup and Restore
    - Automated Backup Policies: Users can define more complex backup strategies (e.g., time-based, event-triggered, incremental backups).
    - Backup on Redis Persistence Options: Integrate support for Redis persistence modes like RDB snapshots and AOF (Append-Only File).
 
- Redis Stream and Pub/Sub Support
    - Add support for Redis Streams and Pub/Sub messaging patterns, allowing users to manage Redis Stream-based applications directly through the operator.
 
- Valkey Support
		- Adding support for Valkey standalone.
		- Adding support for Valkey replication.
		- Adding support for Valkey cluster mode.
		- Adding TLS-based authentication.
		- Adding password-based authentication support.
		- Encryption support.
		- Persistence support.
		- Monitoring feature.
		- Different Kubernetes native support like PDB, sidecar, tolerations, upgrade strategy.
