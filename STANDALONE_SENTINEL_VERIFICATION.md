# Standalone Sentinel verification

## Scope

The local runtime checks used three Redis 8.4 members and three separate, IP-addressed, non-TLS Sentinels with intact persistent storage.
Static authentication was exercised separately.
These results do not establish general production safety or qualify every supported deployment mode.

## Verified behavior

- Sentinel owns running promotions and demotions; operator routing requires verified authority rather than attached-replica counts.
- Each Sentinel retains its complete configuration, voter identity, configuration epochs and voting history on its own retained claim.
- Missing monitors after handoff remain absent and routing closes instead of silently rebuilding lost election history.
- A replacement starts as a replica after checking current authority and local-data safeguards.
- An undiscovered replacement stranded behind a demoted primary can be recreated through protected initialization, without a live role command.
- Automatic recreation is limited to two durable attempts per RedisReplication UID, with fresh observation intervals after controller restart or failed evidence invalidation.
- Controller-only recovery annotations do not enter workload templates or initiate rolling updates.

## Checks performed

| Check | Result |
| --- | --- |
| Full uncached Go suite with race detection and envtest | Passed |
| Go vet and pinned golangci-lint 2.9.0 | Passed |
| Six final-image replacement/restart/refusal scenarios | Passed |
| Three rounds of five existing fault scenarios | All fifteen passed |
| Combined Sentinel/Redis-replica loss | Passed |
| Persistent Service/Sentinel pools, Pub/Sub and blocking consumers | Required workloads passed a 90-second run |
| Same-voter/same-epoch retention across Pod recreation | Passed |
| Established monitor loss | Passed: routing closed, no monitor recreation or second-candidate vote |
| Full persistence-compatible rollback, one-Sentinel partial rollback and return | Passed with retained claims/voters, nondecreasing epochs and preserved recovery ledger |
| Candidate and compatible fallback image vulnerability scans | Zero HIGH/CRITICAL findings with a refreshed October 28 database |

The six replacement scenarios covered plain and authenticated recovery, one unreachable voter, a generic ephemeral auxiliary claim, controller restart during an observation interval, and an exhausted budget across restart.
The unreachable-voter test confirmed intercepted traffic and required no action until access resumed.
The generic-ephemeral test required preservation of the current auxiliary claim and zero automatic attempts.
The restart test observed a new controller incarnation and a fresh interval before the deletion request.

The passing local Go checks used Go 1.26.10; a separate scoped unit-test pass also used Go 1.26.6.
Envtest used Kubernetes 1.31.0 assets and the disposable runtime cluster used Kubernetes 1.33.12.
All test clusters were removed after results were saved.
No registry image publication or live-environment rollout was performed.

## Limits

One fresh Service connection was refused during the passing persistent-client run.
This is not a zero-downtime guarantee, an outage-duration bound, or a replay guarantee for missed Pub/Sub broadcasts.
The client probes do not qualify actual application or BullMQ behavior.

The full upstream release/Chainsaw matrix and documentation site build are not represented as locally passing.
Static-security-analysis debt remains; an image vulnerability result is not a clean static-analysis verdict.
Legacy migration and controlled cold-start/export/restore were rehearsed on an earlier implementation, not repeated on this final runtime.
TLS/hostnames, authentication transitions, physical fencing, corrupt or rolled-back election storage, divergent-data selection and general production RPO remain unqualified.

See `SENTINEL_STORAGE_AND_RECOVERY.md` for required storage, migration, rollback and explicit-recovery boundaries.
