# `tests/e2e-rejoin` — physical multi-operator comparison for isolated-node auto-rejoin

This harness physically compares how **four** RedisCluster operator builds behave
when a node is isolated, on a real cluster. It exists because a single-operator
"does it rejoin?" test is not enough to justify the `RejoinIsolatedNodes` change:
older operators pass naive tests too. The goal is a **discriminating** test that
shows *which* operator recovers *which* failure, and why.

It also doubles as a worked example of how to design a rigorous operator test.
The methodology below is deliberately general so it can be reused for other
controller behaviours.

---

## Methodology playbook (reusable for any operator/controller test)

These are the principles this harness is built on. If you are designing a test
to prove a controller change, follow them in order:

1. **Never trust a test the baseline also passes.** Always run the *unpatched*
   baseline(s) against the exact same injection. If stock passes, the test
   proves nothing about your change. (Our first mistake: a `CLUSTER RESET SOFT`
   test that stock v0.22.2 also "passed" via plain gossip self-heal.)

2. **Find the real failure mechanism before designing the injection — read the
   reconcile code.** Here, the controller has a *node-count* path
   (`CheckRedisNodeCount != total` → `ExecuteRedisReplicationCommand` re-MEETs
   everyone) and a separate *repair* path. That means:
   - Injections that `CLUSTER FORGET` the node **self-defeat**: forgetting drops
     the count below target → the creation path re-MEETs everyone → even stock
     recovers. Discarded.
   - The **durable** repro is a pod delete on a *pure-cache* cluster
     (persistence off, no `nodeConfVolume`): the pod returns under a **new**
     node-id while the **old** id lingers as a stale `fail`/`noaddr` entry that
     **keeps the count at target**. The creation path never fires; the stale
     slave entry is invisible to `RepairDisconnectedMasters` → the live pod stays
     isolated forever.

3. **Prove the injection is durable (a control arm).** An injection only tests
   the operator if the cluster does *not* self-heal without it. Run it against
   stock (or operator-off) and confirm the victim stays broken for the full
   window. If it self-heals, your injection is leaky.

4. **Isolate the operator-under-test reliably — beware GitOps self-heal.** The
   shared operator here is ArgoCD-managed with `selfHeal:true`, which reverts any
   image/replica edit within ~20s. Fighting it (re-asserting the image in a loop)
   just causes perpetual rollout churn, so the "image under test" is barely
   running and the results are garbage. The robust pattern (see *Architecture*):
   annotate the test object so the shared operator **ignores** it, and run a
   **separate, non-GitOps-managed** operator instance (scoped to the test
   namespace, leader-election off) for the image under test.

5. **Capture rich per-object metrics, not a binary pass/fail.** "Rejoined?" is
   too coarse. We record `victim_role`, victim `cluster_known_nodes`, `leader-0`
   known-nodes, and `fail/noaddr` count, and classify into
   `REJOINED_CLEAN / REJOINED_DIRTY / EMPTYMASTER / ISOLATED`. The difference
   between "rejoined as a replica" and "rejoined as an empty master" is the whole
   point and a binary oracle hides it.

6. **Reset to a *verified-clean* state between cells.** We recreate the cluster
   and wait for `known=6, state:ok, 16384/16384, no stale` before every cell. A
   cheaper "heal in place" was tried and discarded — it could leave `leader-0`
   itself isolated and silently poison the next cell.

7. **Guard against confounders.** Real bugs we hit and defended against:
   - **Concurrent runs:** a stop/resume left an orphaned harness process
     injecting the same cluster + writing the same CSV → corruption. The harness
     now refuses to start if another instance is running.
   - **Cross-arch builds:** workstation is `arm64`, cluster is `amd64`; we
     cross-compile the Go binary on the host and package it (no qemu).
   - **Admission webhooks:** confirm no `failurePolicy: Fail` webhook depends on
     the operator you are about to disable.

8. **Test every role — leaders behave differently from followers.** A deleted
   leader triggers a failover (its follower is promoted, slots preserved); the
   ex-leader returns needing to become a *replica* of the new master — a
   different recovery than a follower. Don't generalize from followers only.

---

## Architecture

```
shared redis-operator (v0.22.2, ArgoCD-managed)   ── ignores ──▶  rejoin-test
   │  watches all namespaces                                       (annotated
   │  honours skip-reconcile annotation                            skip-reconcile=true)
   ▼                                                                     ▲
spotu/toast etc. (untouched)                                             │ sole reconciler
                                                                         │
redis-operator-test (NOT ArgoCD-managed, leader-election off,  ─────────┘
   WATCH_NAMESPACE=ashwinr, runs a *-noskip image under test)
```

- `*-noskip` images are normal operator builds with a one-line patch so they
  **ignore** the `skip-reconcile` annotation (built by `build-noskip.sh`,
  reverted after build). The shared operator still honours it, so it leaves the
  test cluster alone while the test operator drives it. No ArgoCD fight, no churn,
  zero blast-radius on other namespaces.

## Files

| file | purpose |
|---|---|
| `lib.sh` | shared helpers (redis exec, probes, `wait_healthy`, pod/role/known queries) |
| `inject.sh` | the isolation injections (INJ-1/2/3/4/5) |
| `comparison.sh` | orchestrator: per image → per injection → recreate, inject, classify, record |
| `build-noskip.sh` | cross-compile + push the `*-noskip` image for each branch |
| `Dockerfile.package` | trivial distroless wrapper around a host-built binary |
| `testcluster.yaml` | pure-cache 6-pod RedisCluster, annotated `skip-reconcile=true` |
| `testoperator.yaml` | the dedicated `redis-operator-test` deployment (image templated) |
| `results-followers.csv`, `results-leader.csv` | recorded results (evidence) |

## Injections (`inject.sh`)

| id | what | why |
|---|---|---|
| INJ-1 | `CLUSTER RESET SOFT` a follower | same id, no stale entry; the original PR injection (control) |
| INJ-2 | delete a follower pod | new id, stale old id lingers — the real prod repro |
| INJ-3 | delete two followers | batch robustness |
| INJ-4 | `CLUSTER RESET HARD` a follower | new id in place |
| INJ-5 | delete a non-seed **leader** | failover + ex-leader returns isolated |

## Reproduce

```bash
# build + push the four *-noskip images (v0.22.2, upstream-main, forget-stale, fix-rejoin)
tests/e2e-rejoin/build-noskip.sh

kubectl apply -f tests/e2e-rejoin/testcluster.yaml

CSV=tests/e2e-rejoin/results-followers.csv \
  IMAGES="v0.22.2 upstream-main forget-stale fix-rejoin" \
  INJECTIONS="INJ-1 INJ-2 INJ-4" WINDOW=240 \
  tests/e2e-rejoin/comparison.sh

CSV=tests/e2e-rejoin/results-leader.csv \
  INJECTIONS="INJ-5" WINDOW=240 \
  tests/e2e-rejoin/comparison.sh
```

Env knobs: `CTX` (kube context, default `atla-prod-storage`), `NS` (`ashwinr`),
`NAME` (`rejoin-test`), `REG`, `WINDOW`, `IMAGES`, `INJECTIONS`.

## Results (240s window, victim = `follower-2`, or `leader-1` for INJ-5)

Followers:

| injection | v0.22.2 | upstream main | forget-stale | **fix-rejoin** |
|---|---|---|---|---|
| INJ-1 reset-soft | ISOLATED | ISOLATED | ISOLATED | **REJOINED_CLEAN** (slave, known=6) |
| INJ-2 delete | ISOLATED +stale | EMPTYMASTER known=7 +stale | EMPTYMASTER known=6 clean | **REJOINED** (slave) known=7 +stale |
| INJ-4 reset-hard | ISOLATED +stale | EMPTYMASTER known=7 +stale | EMPTYMASTER known=6 clean | EMPTYMASTER known=7 +stale* |

Leader (INJ-5, delete `leader-1`; failover preserves slots, `cluster_state:ok`):

| v0.22.2 | upstream main | forget-stale | fix-rejoin |
|---|---|---|---|
| split shard +stale | split shard +stale | split shard, stale pruned | split shard +stale |

## Conclusions

- **`fix/rejoin-isolated-nodes` is required.** It is the only operator that
  rejoins an isolated node back as a *working replica*, and the only one that
  recovers the reset-soft case at all.
- **`fix/forget-stale-nodes` alone is not sufficient.** It has no MEET/rejoin
  path; it only prunes stale gossip entries, leaving the victim an empty master.
- **They are complementary.** On new-id (delete) rejoins, `rejoin-isolated`
  brings the follower back as a slave but leaves a stale old-id (`known=7`);
  `forget-stale` is what prunes it. Cleanest end-state (`slave` + `known=6`)
  needs both.

### Limitations surfaced (tracked, not blockers)

- *(\*)* **Re-replication is race-prone for new-id rejoins.** INJ-2 ended as a
  proper slave but INJ-4 as an empty master: `CLUSTER REPLICATE` is best-effort
  in one reconcile and is not retried once the node is no longer isolated
  (`known>1`). Consider retrying REPLICATE across reconciles for joined-but-
  role-incorrect nodes.
- **Leaders are not fully recovered.** The ex-leader after a failover should
  become a replica of the promoted node, but `leader-N` pods are intentionally
  not REPLICATEd, so the shard is left split. Orthogonal to isolated-follower
  rejoin, but worth a follow-up.
