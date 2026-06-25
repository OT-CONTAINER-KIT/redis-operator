#!/usr/bin/env bash
# soak.sh — rolling-restart soak test for the ForgetStaleNodes fix.
#
# Repeatedly rolling-restarts a pure-cache RedisCluster (one pod at a time) and
# samples cluster correctness to a CSV timeseries, to verify the operator keeps
# the stale-orphan backlog bounded and the cluster reconverges to a correct
# state under sustained churn.
#
#   Phase 1 (sequential): each round restarts all pods once, then waits for full
#     convergence; records convergence lag. Stops if a round never converges.
#   Phase 2 (aggressive): restarts pods continuously without waiting for
#     convergence for PHASE2_SECS, sampling the orphan backlog; then stops and
#     measures recovery time.
#
# Env (with defaults):
#   CTX, NS, NAME, EXPECTED_NODES, EXPECTED_MASTERS, ROUNDS, CONVERGE_TIMEOUT,
#   PHASE2_SECS, POD_READY_TIMEOUT, SAMPLE_SECS, TS (timeseries csv path)
set -uo pipefail

CTX="${CTX:-atla-prod-storage}"
NS="${NS:-ashwinr}"
NAME="${NAME:-redis-soak-50}"
EXPECTED_NODES="${EXPECTED_NODES:-50}"
EXPECTED_MASTERS="${EXPECTED_MASTERS:-25}"
LEADERS="${LEADERS:-25}"
FOLLOWERS="${FOLLOWERS:-25}"
ROUNDS="${ROUNDS:-3}"
CONVERGE_TIMEOUT="${CONVERGE_TIMEOUT:-720}"   # 12 min
PHASE2_SECS="${PHASE2_SECS:-2700}"            # 45 min
POD_READY_TIMEOUT="${POD_READY_TIMEOUT:-180}"
SAMPLE_SECS="${SAMPLE_SECS:-15}"
TS="${TS:-/tmp/soak_timeseries.csv}"

kc() { kubectl --context "$CTX" -n "$NS" "$@"; }
log() { echo "$(date -u +%H:%M:%S) $*"; }

# Optional ArgoCD-race helper (opt-in via PATCHED_IMG). In an environment where
# a GitOps controller self-heals the operator Deployment back to a stock image,
# set PATCHED_IMG=<your patched image> to have the driver re-assert it whenever it
# drifts, so the operator under test stays patched. Leave PATCHED_IMG unset on a
# cluster you control (the committed default) and this is a no-op. NOTE: each
# re-assert restarts the operator, so racing a self-healer means the operator
# cycles throughout the run — annotate results accordingly.
PATCHED_IMG="${PATCHED_IMG:-}"
OP_NS="${OP_NS:-redis-operator}"
reassert() {
  [ -z "$PATCHED_IMG" ] && return 0
  local cur
  cur=$(kubectl --context "$CTX" -n "$OP_NS" get deploy redis-operator -o jsonpath='{.spec.template.spec.containers[0].image}' 2>/dev/null)
  [ -n "$cur" ] && [ "$cur" != "$PATCHED_IMG" ] && kubectl --context "$CTX" -n "$OP_NS" set image deploy/redis-operator redis-operator="$PATCHED_IMG" >/dev/null 2>&1
  return 0
}

# Probe the cluster from the BEST-INFORMED leader (max cluster_known_nodes),
# sampling up to PROBE_N leaders so a single just-restarted/isolated leader can't
# poison the reading. echo "known size slots_assigned slots_ok state bad"
PROBE_N="${PROBE_N:-8}"
probe() {
  local i pod out tried=0
  local best_known=-1 best=""
  for i in $(seq 0 $((LEADERS-1))); do
    (( tried >= PROBE_N )) && break
    pod="${NAME}-leader-${i}"
    out=$(kc exec "$pod" -c "${NAME}-leader" -- sh -c '
      redis-cli cluster info 2>/dev/null
      echo "==NODES=="
      redis-cli cluster nodes 2>/dev/null' 2>/dev/null) || continue
    [[ "$out" == *cluster_state* ]] || continue
    tried=$((tried+1))
    local ci nodes known size sa so state bad
    ci="${out%%==NODES==*}"; nodes="${out#*==NODES==}"
    known=$(printf '%s\n' "$ci" | awk -F: '/cluster_known_nodes:/{gsub(/\r/,"");print $2}')
    size=$(printf '%s\n' "$ci"  | awk -F: '/cluster_size:/{gsub(/\r/,"");print $2}')
    sa=$(printf '%s\n' "$ci"    | awk -F: '/cluster_slots_assigned:/{gsub(/\r/,"");print $2}')
    so=$(printf '%s\n' "$ci"    | awk -F: '/cluster_slots_ok:/{gsub(/\r/,"");print $2}')
    state=$(printf '%s\n' "$ci" | awk -F: '/cluster_state:/{gsub(/\r/,"");print $2}')
    bad=$(printf '%s\n' "$nodes" | grep -cE 'fail|noaddr|handshake')
    if (( ${known:-0} > best_known )); then
      best_known=${known:-0}; best="${known:-?} ${size:-?} ${sa:-?} ${so:-?} ${state:-?} ${bad:-?}"
      (( best_known >= EXPECTED_NODES )) && break   # fully-informed view; stop early
    fi
  done
  if [[ -n "$best" ]]; then echo "$best"; return 0; fi
  echo "? ? ? ? unreachable ?"; return 1
}

ready_pods() { kc get pods -l "cluster=${NAME}" -o jsonpath='{range .items[*]}{.status.containerStatuses[0].ready}{"\n"}{end}' 2>/dev/null | grep -c true; }
op_tag() { kubectl --context "$CTX" -n redis-operator get deploy redis-operator -o jsonpath='{.spec.template.spec.containers[0].image}' 2>/dev/null | sed 's#.*:##'; }

sample() {
  reassert
  local phase="$1" round="$2" p rp ot
  p=$(probe); rp=$(ready_pods); ot=$(op_tag)
  echo "$(date -u +%s),$phase,$round,$p,$rp,$ot" | tr ' ' ',' >> "$TS"
  read -r known size sa so state bad <<<"$p"
  log "[$phase r$round] known=$known size=$size slots=$sa/$so state=$state bad=$bad ready=$rp/$EXPECTED_NODES op=$ot"
}

converged() {
  local p known size sa so state bad
  p=$(probe) || return 1
  read -r known size sa so state bad <<<"$p"
  [[ "$known" == "$EXPECTED_NODES" && "$bad" == "0" && "$sa" == "16384" && "$so" == "16384" && "$state" == "ok" && "$size" == "$EXPECTED_MASTERS" ]]
}

wait_converged() { # $1 timeout secs -> echo lag secs; return 0 if converged
  local t=0
  while (( t < $1 )); do
    if converged; then echo "$t"; return 0; fi
    sample "converge" "$2"
    sleep "$SAMPLE_SECS"; t=$((t+SAMPLE_SECS))
  done
  echo "$1"; return 1
}

roll_pod() { # $1 pod  $2 wait_ready(true/false)
  # Wait for the OLD pod to be fully deleted (respects terminationGracePeriod),
  # so we restart strictly one pod at a time.
  kc delete pod "$1" --wait=true --timeout="${POD_READY_TIMEOUT}s" >/dev/null 2>&1
  if [[ "$2" == "true" ]]; then
    # Then wait for the StatefulSet's replacement pod to come back Ready.
    local t=0
    while (( t < POD_READY_TIMEOUT )); do
      reassert
      if kc get pod "$1" -o jsonpath='{.status.containerStatuses[0].ready}' 2>/dev/null | grep -q true; then return 0; fi
      sleep 3; t=$((t+3))
    done
    log "WARN $1 not Ready in ${POD_READY_TIMEOUT}s"
  fi
}

echo "ts,phase,round,known,size,slots_assigned,slots_ok,state,bad,ready,op_tag" > "$TS"
log "soak start: $NAME ns=$NS ctx=$CTX expect nodes=$EXPECTED_NODES masters=$EXPECTED_MASTERS"
log "baseline:"; sample "baseline" 0

OVERALL_RESULT="PASS"

# ---- Phase 1: sequential rounds ----
for r in $(seq 1 "$ROUNDS"); do
  log "=== Phase1 round $r: rolling restart all pods (followers then leaders) ==="
  for i in $(seq 0 $((FOLLOWERS-1))); do roll_pod "${NAME}-follower-${i}" true; sample "p1-restart" "$r"; done
  for i in $(seq 0 $((LEADERS-1)));   do roll_pod "${NAME}-leader-${i}"   true; sample "p1-restart" "$r"; done
  log "round $r restarts done; waiting for convergence (<= ${CONVERGE_TIMEOUT}s)"
  if lag=$(wait_converged "$CONVERGE_TIMEOUT" "$r"); then
    log "round $r CONVERGED in ${lag}s after restarts"
  else
    log "round $r FAILED to converge within ${CONVERGE_TIMEOUT}s — STOP"
    OVERALL_RESULT="FAIL (round $r no convergence)"; break
  fi
done

# ---- Phase 2: aggressive overlapping (only if phase1 passed) ----
if [[ "$OVERALL_RESULT" == "PASS" ]]; then
  log "=== Phase2: aggressive overlapping restarts for ${PHASE2_SECS}s ==="
  end=$(( $(date +%s) + PHASE2_SECS )); idx=0
  while (( $(date +%s) < end )); do
    if (( idx % 2 == 0 )); then pod="${NAME}-follower-$(( (idx/2) % FOLLOWERS ))"; else pod="${NAME}-leader-$(( (idx/2) % LEADERS ))"; fi
    roll_pod "$pod" true     # wait only for pod-Ready, NOT cluster convergence
    sample "p2-churn" 0
    idx=$((idx+1))
  done
  log "Phase2 churn done; measuring recovery (<= ${CONVERGE_TIMEOUT}s)"
  if lag=$(wait_converged "$CONVERGE_TIMEOUT" 0); then
    log "Phase2 RECOVERED in ${lag}s after churn stopped"
  else
    log "Phase2 did NOT recover within ${CONVERGE_TIMEOUT}s"
    OVERALL_RESULT="FAIL (phase2 no recovery)"
  fi
fi

log "=== SOAK RESULT: $OVERALL_RESULT ==="
log "peak orphan backlog: $(awk -F, 'NR>1 && $4 ~ /^[0-9]+$/ {d=$4-'"$EXPECTED_NODES"'; if(d>m)m=d} END{print m+0}' "$TS")"
log "timeseries: $TS"
