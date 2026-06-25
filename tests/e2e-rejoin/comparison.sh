#!/usr/bin/env bash
# comparison.sh — rigorous 3-image comparison for isolated-node auto-rejoin.
#
# For each operator image and each injection, resets the cluster to a clean
# state, injects, then classifies the outcome from rich metrics:
#   ISOLATED        victim never rejoins (known<=1, role master)
#   REJOINED_DIRTY  victim rejoins but cluster left with stale residue
#                   (leader-0 known>6 or fail/noaddr entries persist)
#   REJOINED_CLEAN  victim rejoins and cluster is fully clean (known=6, no bad)
#
# Each image runs via a dedicated, non-ArgoCD-managed test operator
# (redis-operator-test, a *-noskip build scoped to the test namespace). The
# shared v0.22.2 operator ignores the skip-reconcile-annotated test cluster, so
# there is no ArgoCD fight and the operator under test is the sole, stable actor.
set -uo pipefail
HERE="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
source "$HERE/lib.sh"

REG="${REG:-docker-local.artifactory.twitter.biz/storage}"
TESTOP="${TESTOP:-redis-operator-test}"
img_ref() { # map image key -> the matching *-noskip image run by the test operator
  echo "$REG/redis-operator:$1-noskip"
}
IMAGES=(${IMAGES:-v0.22.2 upstream-main forget-stale fix-rejoin})
INJECTIONS=(${INJECTIONS:-INJ-1 INJ-2 INJ-3 INJ-4})
WINDOW="${WINDOW:-180}"
CSV="${CSV:-$HERE/results.csv}"

# Refuse to run if another comparison is already active (prevents two runs from
# fighting over the same cluster + CSV, which silently corrupts both).
_others=$(pgrep -f 'comparison.sh' | grep -v "^$$\$" | wc -l | tr -d ' ')
if [[ "${_others:-0}" -gt 1 ]]; then
  echo "ERROR: another comparison.sh appears to be running (pids: $(pgrep -f comparison.sh | tr '\n' ' ')). Aborting." >&2
  exit 1
fi

# Point the dedicated (non-ArgoCD-managed) test operator at the image under test
# and wait for it to be the sole running pod. No pinner / no ArgoCD fight: the
# shared operator ignores the skip-reconcile-annotated cluster, and this one
# (a *-noskip build scoped to the test namespace) reconciles it.
set_operator() {
  local img; img="$(img_ref "$1")"
  log "switching test operator -> $1 ($img)"
  sed "s#IMAGE_PLACEHOLDER#$img#" "$HERE/testoperator.yaml" | kubectl --context "$CTX" apply -f - >/dev/null
  kubectl --context "$CTX" -n "$OP_NS" set image deploy/"$TESTOP" "$TESTOP=$img" >/dev/null 2>&1
  kubectl --context "$CTX" -n "$OP_NS" rollout status deploy/"$TESTOP" --timeout=150s >/dev/null \
    || { log "ERROR: test operator rollout failed for $1"; return 1; }
}

forget_all_stale() {
  local p sid
  for p in $(all_pods); do
    redis "$p" cluster nodes 2>/dev/null \
      | awk '$3 !~ /myself/ && ($3 ~ /fail/||$3 ~ /noaddr/||$3 ~ /handshake/){print $1}' \
      | while read -r sid; do redis "$p" cluster forget "$sid" >/dev/null 2>&1 || true; done
  done
}
clean_reset() { # ALWAYS recreate the cluster fresh — most reliable known-good
  # start for each cell (manual heal proved fragile: it could leave leader-0
  # isolated). The pinned operator under test rebuilds the cluster.
  kc delete -f "$HERE/testcluster.yaml" --wait=true --timeout=120s >/dev/null 2>&1 || true
  kc wait --for=delete rediscluster/"$NAME" --timeout=120s >/dev/null 2>&1 || true
  local i
  for i in $(seq 0 $((LEADERS-1))); do
    kc delete pod "${NAME}-leader-$i" --force --grace-period=0 >/dev/null 2>&1 || true; done
  for i in $(seq 0 $((FOLLOWERS-1))); do
    kc delete pod "${NAME}-follower-$i" --force --grace-period=0 >/dev/null 2>&1 || true; done
  kc apply -f "$HERE/testcluster.yaml" >/dev/null
  for i in $(seq 0 $((LEADERS-1))); do wait_pod_ready "${NAME}-leader-$i" 240; done
  for i in $(seq 0 $((FOLLOWERS-1))); do wait_pod_ready "${NAME}-follower-$i" 240; done
  wait_healthy 300
}

classify() { # victims... -> echoes "OUTCOME lag l0known bad"
  # OUTCOME taxonomy (most-to-least healed):
  #   REJOINED_CLEAN  victim a proper slave (known=6, link up) AND cluster clean
  #                   (leader-0 known=6, no fail/noaddr)
  #   REJOINED_DIRTY  victim a proper slave but cluster left with stale residue
  #   WRONGROLE       victim back in gossip (known=6) but stuck as empty master
  #                   (rejoined but not re-replicated)
  #   ISOLATED        victim never rejoined gossip (known<=1)
  local t=0 rejoined=0 lag=-1
  while (( t<WINDOW )); do
    local ok=1 v
    for v in "$@"; do
      local vk vr vl; vk=$(known "$v"); vr=$(role "$v"); vl=$(link "$v")
      if [[ "$v" == *-follower-* ]]; then
        [[ "$vk" == "$EXPECTED_NODES" && "$vr" == "slave" && "$vl" == "up" ]] || ok=0
      else
        [[ "$vk" == "$EXPECTED_NODES" && "$vr" == "master" ]] || ok=0
      fi
    done
    if (( ok==1 )); then rejoined=1; lag=$t; break; fi
    sleep 12; t=$((t+12))
  done
  sleep 15  # settle, then read cluster cleanliness + victim's final role
  local l0 bad v1 vrole vknown vlink; l0=$(known "${NAME}-leader-0"); bad=$(bad_count)
  v1="$1"; vrole=$(role "$v1"); vknown=$(known "$v1"); vlink=$(link "$v1")
  # outcome + 5 metrics: lag, leader0_known, bad, victim_role, victim_known
  if (( rejoined==1 )); then
    if [[ "$l0" == "$EXPECTED_NODES" && "$bad" == "0" ]]; then echo "REJOINED_CLEAN $lag $l0 $bad $vrole $vknown"
    else echo "REJOINED_DIRTY $lag $l0 $bad $vrole $vknown"; fi
  else
    if [[ "${vknown:-1}" -le 1 ]] 2>/dev/null; then echo "ISOLATED $WINDOW $l0 $bad $vrole $vknown"
    elif [[ "$vrole" == "slave" && "$vlink" == "up" ]]; then echo "REJOINED_DIRTY $WINDOW $l0 $bad $vrole $vknown"
    else echo "EMPTYMASTER $WINDOW $l0 $bad $vrole $vknown"; fi
  fi
}

echo "ts,image,injection,victims,outcome,lag_s,leader0_known,bad_entries,victim_role,victim_known" > "$CSV"
for imgkey in "${IMAGES[@]}"; do
  log "############## IMAGE: $imgkey ##############"
  set_operator "$imgkey" || { log "skip $imgkey (rollout failed)"; continue; }
  for inj in "${INJECTIONS[@]}"; do
    log "[$imgkey] reset -> clean"; clean_reset
    log "[$imgkey] inject $inj"
    victims=(); while IFS= read -r _line; do [[ -n "$_line" ]] && victims+=("$_line"); done < <("$HERE/inject.sh" "$inj")
    read -r outcome lag l0 bad vrole vknown < <(classify "${victims[@]}")
    log "[$imgkey] $inj => $outcome lag=${lag}s leader0_known=$l0 bad=$bad victim=$vrole/known=$vknown (${victims[*]})"
    echo "$(date -u +%FT%TZ),$imgkey,$inj,${victims[*]// /+},$outcome,$lag,$l0,$bad,$vrole,$vknown" >> "$CSV"
  done
done
log "=== RESULTS ==="; column -t -s, "$CSV"
