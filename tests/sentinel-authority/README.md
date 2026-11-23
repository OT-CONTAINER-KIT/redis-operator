# Local standalone Sentinel verification

These are opt-in destructive checks for disposable resources, not a staging or production runbook.

## Deterministic monitor-recovery race

```sh
go run ./tests/sentinel-authority/recovery-race
```

This starts a synthetic RESP server on an ephemeral loopback port.
The caller observes the same old report twice, then the server returns a newer election to the monitor helper.
Success requires that the newer election is not replaced.
Exit status 1 with `overwrote_newer_election: true` reproduces the old blocker.
The create-only implementation must return zero REMOVE/MONITOR commands in this schedule.
No real Redis, Kubernetes cluster, or credentials are used.

## Vote-state retention gate

`vote_reset.py` checks a different property: a restarted logical voter must not switch candidates within one epoch.
It requires the same explicit disposable cluster/kubeconfig and owned-namespace guards.
It requests vote responses only, not failovers or promotions.
Exit status 1 with `same_epoch_vote_changed_after_replacement: true` reproduces the historical loss of election history under a retained voter identity.
Do not treat a healthy pod/reconnection check as a substitute for this gate.
See `STANDALONE_SENTINEL_VERIFICATION.md` for the verified scope and current limits.

The persistent-state implementation requires retention of the same PVC, voter identity and vote epoch.
Use `--allow-ephemeral` only to reproduce the historical broken build.
See `STANDALONE_SENTINEL_VERIFICATION.md` for checks and `SENTINEL_STORAGE_AND_RECOVERY.md` for the storage and recovery design.

## Disposable kind convergence harness

`verify.py` requires an explicit kubeconfig and a context named `kind-redis-authority-verify-*`.
It verifies the node names and uses only a namespace named `authority-test-*` with its own ownership label.
It refuses to initialize an existing namespace.
Do not change these guards to target a shared environment.

After installing the operator and CRDs in a matching disposable four-node kind cluster:

```sh
python3 tests/sentinel-authority/verify.py \
  --kubeconfig /path/to/disposable-kind-kubeconfig \
  --cluster redis-authority-verify-example setup
python3 tests/sentinel-authority/verify.py \
  --kubeconfig /path/to/disposable-kind-kubeconfig \
  --cluster redis-authority-verify-example --rounds 3 regressions
```

The setup uses synthetic Redis data, local PVCs, and three standalone Sentinels.
Each Sentinel now has its own 1 GiB `standard` configuration claim in this local fixture.
Use `--ephemeral-sentinels` only for a stock legacy baseline before migration.
The regression mode includes delayed Sentinel reconfiguration, ordinary failover,
failover overlapping replica replacement, and two/all Sentinel replacements.
Network injection uses node-local Docker/iptables and cleanup blocks.
Each round checks full replica health, Sentinel agreement, routing, marker retention, and the relevant pod identities.
A missing two-primary injection window is a failure, not a successful race test.

The recorded candidate completed three rounds of these five scenarios.
Those results do not qualify untested failures or later source changes.

Additional modes:

- `--namespace authority-test-auth --authentication setup`, then `recovery`, uses separate random disposable Redis/Sentinel credentials. Values are never emitted or passed in argv.
- `recovery` tests deleted Sentinel StatefulSet recovery with routing status cleared first, followed by failover. It does not delete established monitors.
- `clients` uses `client-probe` to check persistent direct-Service/Sentinel pools, Pub/Sub and blocking list consumers through failover.
- `combined-loss` replaces all Sentinel pods and a Redis replica together. Persistent Sentinel configuration is retained. A timeout is a failure, not a passed availability test.

### Failover after protected initialization

`replacement_start.py` recreates a replica, waits for its protected initializer to finish, and deliberately delays its Redis process while Sentinel promotes another member.
The delayed member then starts behind the former primary, reproducing the undiscovered cascading-replica case.
Success requires automatic recreation of that exact stranded UID followed by direct attachment, Sentinel discovery and readiness.
Healthy-member UIDs, all claims, voter identities, the post-promotion epoch and the synthetic dataset marker must remain unchanged.

Build `Dockerfile.replacement-delay` with a context containing only `replacement-delay.sh`, and load the resulting `redis-authority-replacement-delay:local` image into the disposable cluster before setup.
The wrapper activates only when the test explicitly creates its marker on the replica's synthetic data claim; it is not a deployment image.

```sh
python3 tests/sentinel-authority/replacement_start.py \
  --kubeconfig /path/to/disposable-kind-kubeconfig \
  --cluster redis-authority-verify-example \
  --namespace authority-test-replacement setup
python3 tests/sentinel-authority/replacement_start.py \
  --kubeconfig /path/to/disposable-kind-kubeconfig \
  --cluster redis-authority-verify-example \
  --namespace authority-test-replacement recovery
```

Use `--authentication` for the static-authentication fixture.
The initial construction timeout is 480 seconds for this fixture; it is not an outage-recovery bound.
`--unavailable-voter` blocks only the operator's access to one Sentinel, requires the stranded UID to remain for 150 seconds, then requires recovery after the block is removed.
`--exhausted-budget` preloads the spent two-attempt budget, restarts the disposable operator and requires the stranded UID and budget to remain unchanged for 150 seconds.
`--generic-ephemeral-claim` adds a Pod-owned auxiliary claim and requires no automatic deletion or spent attempt for 150 seconds; the test's initial replica deletion intentionally replaces that auxiliary claim, after which its new UID and marker must remain intact.
`--restart-during-observation` restarts the actual operator after its first unreserved observation and requires a new controller incarnation and fresh interval before deletion. A fixture-owned finalizer preserves the exact deletion-request timestamp for the assertion and is removed in cleanup.
Run the exhausted-budget case after other checks that should not overlap an operator restart, and use a fresh owned namespace for each scenario.

`probe_timeout.py` uses a separate isolated Redis process and an endpoint that accepts connections without answering them.
It checks failed readiness, repeated probe deadlines/cancellation, process counts and recovery after replies resume.
For a native-probe build, provide a test Redis image containing the exact operator binary at `/etc/redis/sentinel-ready` using `--probe-image`.
This image has no dataset or credentials and must not be used for deployment.

`cold_start.py` stops the disposable operator and all Redis writers, then checks that replacement initialization explicitly refuses missing authority and advertises no writable endpoint.
It finishes with the original Redis set fenced at zero and the operator stopped.
Run it last in that cluster.
With `--backup-dir`, it exports all three datasets through read-only mounts, but only after verifying this is the controlled, synchronously saved, one-key fixture.
Archives remain in a new private directory under the approved local artifact root; Sentinel configuration is never exported.

`cold_restore.py` imports that controlled dataset into fresh data claims and a new voter generation in a separate disposable cluster.
It requires an explicit source kubeconfig and verifies that the original writers/controller remain stopped.
It never overwrites the original claims or restores Sentinel identity/vote snapshots.
This tests a known synthetic dataset restore, not selection among divergent real datasets or a zero-loss disaster recovery guarantee.

## Building local-only images

Build into an artifact directory, not a tracked source directory:

```sh
mkdir -p "$ARTIFACTS/build-operator" "$ARTIFACTS/build-client"
CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o "$ARTIFACTS/build-operator/operator" ./cmd/main.go
docker build -f tests/sentinel-authority/Dockerfile.candidate -t redis-operator:local-verification "$ARTIFACTS/build-operator"
CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o "$ARTIFACTS/build-client/client-probe" ./tests/sentinel-authority/client-probe
docker build -f tests/sentinel-authority/Dockerfile.probe -t redis-authority-client-probe:local "$ARTIFACTS/build-client"
```

Load images into the explicit disposable cluster with `kind load docker-image`.
Keep build contexts binary-only, separate from kubeconfigs and test receipts.
The chart defaults the init image tag to the operator tag.
Both images must implement the `--standalone-authority` capability flag; stock v0.27 rejects it before generating configuration.
The Sentinel init image must also implement `--persistent-sentinel-state`, preserving a saved runtime file instead of regenerating identity or votes.
These Dockerfiles are test conveniences, not image-publication or production supply-chain approval.

## Gated upgrade and rollback

Export old source with `git archive v0.23.0 charts/redis-operator config/crd/bases` into a local artifact directory.
Start a separate disposable cluster with the old CRDs and stock v0.23 operator, then run `setup`.
`upgrade.py` takes the same explicit cluster/kubeconfig guard plus `--helm`, `--candidate-tag` and mode `upgrade`, `rollback` or `partial-rollback`.
Legacy storage adoption holds Redis, freezes the old operator, updates CRDs, sets the local claims, terminates the old Sentinel set and creates the persistent set with fresh voter identities.
Redis then advances one gated member at a time.
For an already persistent set, compatible image changes advance one Sentinel and then one Redis member at a time.
CRDs and claims are retained during rollback rather than downgraded.
Its CRD field-ownership override is permitted only by the disposable-node guard; production needs its own ownership/migration plan.

The first full upgrade rehearsal passed.
Rollback to stock v0.23 stopped at its known placeholder-monitor initialization problem; it is not a passed rollback drill.
The October 19 storage-adoption rehearsal passed; later builds require separately identified receipts.
`rollback` now requires `--compatible-tag` identifying a persistence-aware local build; stock v0.23 is not an image-only fallback for this protocol.
Using two tags of identical code tests procedure/state retention only, not compatibility with earlier source.
See `STANDALONE_SENTINEL_VERIFICATION.md` for results and unresolved qualification limits.

The Go readiness agent uses bounded network operations and no child CLI/watchdog processes.
The initializer copies the executable into the Redis configuration volume and the existing probe script `exec`s it.
The rollback harness now checks claim and voter identities and nondecreasing configuration epochs throughout each transition, not merely pod health.

## Lost monitor history

`vote_reset.py --loss-mode monitor` reproduces the unsafe behavior in the older builds: deleting a group can erase the same voter's same-epoch refusal.
For the guarded candidate, use `--loss-mode monitor --expect-blocked-monitor-loss`.
That acceptance case requires the monitor to remain absent, routing to close, and the voter not to grant a second candidate.
It leaves the fixture in a state requiring explicit recovery, so run it last in its namespace.
It is not a claim that the deleted history has been restored.
First initialization may create absent monitors, but lost or ambiguous state after ownership handoff must not be silently reseeded.

A rollback test must supply a separately built, persistence-aware compatible image with retained ownership and metadata protections.
Two tags of identical code do not qualify source compatibility; stock v0.23 remains unsupported as a rollback target.

`chart_matrix.py` runs chart install/Helm-test commands in its own disposable namespace and cleans up its releases.
No hook manifest means no Helm hook assertion, not runtime failover qualification.
The native client adapter under `native-clients/` uses a pinned ioredis dependency and a lockfile; `npm test` checks syntax only.
`native_clients.py` provides its separate election/reconnect exercise, not a full application/BullMQ test.
`operator_restart.py` checks an authenticated fixture through an actual operator restart without changing its workload pods.
