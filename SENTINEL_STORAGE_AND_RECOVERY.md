# Sentinel storage, migration and recovery

This is a local design and rehearsal record, not permission to modify an environment.
It applies to three standalone, IP-addressed, non-TLS Redis 8.4 members and three Sentinels with intact persistent storage.

## Recommended storage settings

Use three independent filesystem claims, one per Sentinel ordinal, each requesting 1 GiB with ReadWriteOnce access.
Retain claims on deletion and scale-down.
Use the same explicitly configured user and fsGroup (1000) for initialization and runtime.
Do not share, clone, restore or roll back a live voter's configuration volume.

Use the deployment's existing Redis storage class explicitly; do not assume every cluster provides the same class.
The disposable rehearsal uses kind's `standard` provisioner.
The example below describes claim retention, not a storage-provider or live-environment qualification.
Before an authorized rollout, verify the live provisioner, filesystem durability, encryption-at-rest policy, topology and attach behavior.
Do not silently switch storage classes during an upgrade.

```yaml
storage:
  volumeClaimTemplate:
    spec:
      storageClassName: standard-rwo
      accessModes: [ReadWriteOnce]
      resources:
        requests:
          storage: 1Gi
kubernetesConfig:
  persistentVolumeClaimRetentionPolicy:
    whenDeleted: Retain
    whenScaled: Retain
podSecurityContext:
  runAsUser: 1000
  fsGroup: 1000
```

The file can contain authentication settings.
Restrict pod exec, PVC mounts and storage administration to the operator's operational owners.
Do not emit it into logs, diagnostics, Git, image contexts or ordinary incident attachments.
Require encrypted storage and encrypted, access-controlled forensic archives.
Retain state until its entire voter generation is explicitly retired; delete it only through the normal approved storage-retirement process.

No automatic restore of Sentinel snapshots is proposed.
A snapshot can restore the same voter identity while moving its epoch and vote history backwards.
Snapshot age, a healthy pod and a matching identity do not make such a restore safe.
Redis dataset backup policy is separate from Sentinel election-state retention.

## Legacy ephemeral-to-persistent migration

The StatefulSet cannot acquire claim templates in place.
Use an attended maintenance procedure, not the operator's generic invalid-update recreation annotation.

1. Verify the exact source and destination controller/init images and generated CRDs.
   Confirm one primary, two synchronized replicas, a healthy Sentinel majority, the master Service endpoint and a usable Redis dataset backup.
2. Hold both workload rollout partitions at three.
   Record Redis pod UIDs and all existing PVC identities.
3. Stop the old operator before adding fields its old API type cannot preserve.
   Confirm the old controller is actually gone, not merely scaled down in the desired spec.
4. Apply the new CRDs and persistent Sentinel settings while Redis is held in place.
   Terminate the entire old Sentinel set and confirm its voters are gone before introducing a fresh voter generation.
   Do not reuse deterministic legacy identities with empty configuration.
5. Start the new controller and persistent Sentinel set.
   Confirm three distinct voter IDs, three bound claims and agreement on the still-running primary.
   Confirm Redis UIDs and the dataset marker have not changed.
6. Advance Redis one member at a time.
   After each replacement, require healthy replication, Sentinel discovery, a verified master endpoint and retained data before proceeding.
7. Capture actual image digests, claim UIDs and the final health checks.

A legacy adoption rehearsal on an earlier implementation is not qualification of every later controller/init pair.
Repeat it on the exact proposed images before a live upgrade; see `STANDALONE_SENTINEL_VERIFICATION.md` for current test limits.

## Persistence-compatible rollback

Retain the newer CRDs and all state claims.
Never downgrade to stock v0.23 or an ephemeral initializer after adopting this protocol.
Never remove storage, alter the monitor group or retarget replication as part of an image rollback.

Use a previously tested persistence-aware controller/init pair, with both rollout partitions held before switching images.
Advance Sentinel ordinals first, then Redis ordinals, checking each step.
Require unchanged claim UIDs and voter IDs and nondecreasing configuration epochs.
For partial rollback, actually replace one candidate Sentinel before returning to the fallback image.
An image-tag change without replacement is not a rollback drill.

A fallback must preserve the persistent standalone protocol and the ownership/startup protections.
It must also preserve the controller-only replacement-recovery annotation without copying it into workload templates.
Do not assume an older image meets these requirements merely because it starts successfully.
Full and one-Sentinel partial rollback were tested with a separately built persistence-aware fallback, not two tags of identical code.
That fallback retained election state but did not implement automatic stranded-replacement recovery.
This does not certify arbitrary prior releases as rollback targets.
See `STANDALONE_SENTINEL_VERIFICATION.md` for the checked scope and limits.

## Missing monitors are not ordinary pod recovery

`SENTINEL REMOVE` deletes per-master vote history as well as the monitor.
Recreating that monitor under the same retained voter identity can permit a different vote in the same epoch.
A persistent volume alone does not undo this administrative loss.

Automatic initialization is therefore limited to a wholly unconfigured first authority, with a direct read of the unchanged RedisReplication identity/specification and no historical ownership/startup marker.
Ownership is committed before initialization begins and is not reset afterward.
Any missing monitor after handoff, mixed preexisting/missing generation, or interrupted/ambiguous first initialization fails closed and requires explicit generation recovery.
This deliberately prefers stopping to silently recreating lost voting state, including when an interrupted first initialization might not actually have cast votes.

Normal Sentinel pod or StatefulSet replacement reuses complete configuration and remains a separate supported recovery path.
Do not use `SENTINEL REMOVE`/`RESET`, delete internal ownership/startup annotations, or recreate RedisReplication against old Sentinel state as a repair shortcut.
The ownership metadata must be retained alongside the Sentinel volumes.
Deleting/recreating authority CRs or rolling back their metadata is not qualified as ordinary replacement.

## Bounded recovery of an undiscovered replacement

A replacement can finish initialization against primary A, start only after Sentinel has promoted B, and then replicate behind A instead of directly from B.
Sentinel discovers replicas from its primary, so this new address can remain undiscovered and correctly fail readiness.
A container restart does not rerun completed init containers.

The local recovery implementation may gracefully recreate only that positively identified replacement, so protected initialization samples current authority again.
It sends no live `REPLICAOF`, `REMOVE`, `MONITOR` or `RESET`, never chooses a primary, and leaves readiness unchanged.
The required evidence includes:

- Recorded standalone ownership and startup activation, unchanged resource identities/specifications, three running Redis members, and three ready retained Sentinels with distinct voter identities.
- A verified serving primary and another healthy direct replica, with all three Sentinels agreeing at one nonzero configuration epoch.
- A protected, unready replacement following that direct replica, with its new address absent from every Sentinel replica table.
- Successful initialization and a main process running for at least 90 seconds, no main-container restart, retained bound claims, and no pod-owned claims or generic ephemeral claim volumes that deletion could collect.
- All actual workload pods at their desired StatefulSet revision. A readiness-dependent `currentRevision` lag alone is not an update still waiting to be applied.
- Unchanged positive observations at least 30 seconds apart, with no observation gap exceeding one minute. Elections, missing authority or contradictory evidence reset this observation interval.

The controller reserves each attempt before deletion in `redis.opstreelabs.in/sentinel-replacement-recovery` on RedisReplication.
This controller-only annotation is excluded from generated workload metadata/templates, so observation updates do not initiate rolling updates.
The budget is two automatic attempts for that RedisReplication UID, survives controller restarts, and is not silently replenished after success.
The observation interval is bound to the controller incarnation, not carried across a restart.
An intervening contradiction invalidates it in memory before attempting an API reset, so a failed reset read or write cannot resurrect an older interval; the next controller also starts a fresh interval.
Only one pending target is allowed; it is revalidated before a UID/resource-version-preconditioned Pod deletion using the normal grace period.
No force deletion, PVC deletion, Sentinel restart or voting-history reset is part of this recovery.

After the budget is exhausted or evidence becomes ambiguous, stop automatic recovery and investigate explicitly.
Resetting this budget is a separate attended action: stop the controller, establish three healthy directly attached Redis members and intact agreeing voters, confirm no pending termination, and retain the old metadata record before clearing only the replacement-recovery annotation.
Never clear ownership/startup annotations or Sentinel history to replenish this budget.
Live recovery or budget-reset operations still require environment-specific authorization.
Historical fallback images must not be assumed to understand this new annotation or guard; use only a fallback qualified with its mandatory metadata filtering and activation protections.
This is not a cold-start, split-brain fencing, divergent-data selection, authentication-transition or suspect-storage recovery policy.

## Full cold restart or suspect election storage

The protected initializer deliberately does not invent a primary when every Redis process is stopped.
Missing or zero-length Sentinel state is rejected; a nonzero corrupted file is not certified by initialization.
An intact-PVC replacement pass is not evidence of corruption recovery.

Conservative recovery rules:

1. Fence all old Redis writers and any suspect voter generation.
   Pod reachability is not physical fencing; verify process/node termination before recovery can create a new writer.
2. Preserve the original Redis datasets and Sentinel volumes without modification.
   Work on offline copies and do not run Redis against a suspect original just to inspect it.
3. Validate dataset copies with the matching Redis tools and compare replication lineage, available durable offsets and backups.
   Key count, an arbitrary ordinal and wall-clock modification time are not sufficient to select the authoritative dataset.
4. Select a recovery source only with evidence for its contents and a defined acceptable recovery point.
   If the evidence cannot distinguish divergent datasets, do not guess or automatically overwrite one with the other.
5. Use a new, isolated recovery generation and fresh voter identities if old election history is suspect.
   Keep the old generation fenced and retained; do not run restored vote history alongside it.
6. Validate the restored primary, replicas, routing and application reconnects before allowing writes or retiring originals.

The local cold-start test checks refusal to start a writer and retention of original state.
The companion restore rehearsal copies only a controlled, synchronously saved, identical one-key fixture into fresh data claims and fresh Sentinel identities in another disposable cluster.
It is not qualification of arbitrary dataset selection, divergent copies, a live voter-generation transition, or a general data-loss/RPO guarantee.
An actual ambiguous dataset choice requires incident-specific approval, not a standing automatic recovery policy.
