#!/usr/bin/env bash
# lib.sh — shared helpers for the rejoin-isolated-nodes e2e matrix.
# Pure-cache cluster, no auth/TLS. Sourced by inject.sh and run-matrix.sh.
set -uo pipefail

CTX="${CTX:-atla-prod-storage}"
NS="${NS:-ashwinr}"
NAME="${NAME:-rejoin-test}"
LEADERS="${LEADERS:-3}"
FOLLOWERS="${FOLLOWERS:-3}"
EXPECTED_NODES="${EXPECTED_NODES:-6}"
PORT="${PORT:-6379}"
OP_NS="${OP_NS:-redis-operator}"
OP_DEPLOY="${OP_DEPLOY:-redis-operator}"
OP_CONTAINER="${OP_CONTAINER:-redis-operator}"

log() { echo "$(date -u +%H:%M:%S) $*"; }
kc() { kubectl --context "$CTX" -n "$NS" "$@"; }

ctr() { case "$1" in *-follower-*) echo "${NAME}-follower";; *) echo "${NAME}-leader";; esac; }
# redis <pod> <args...>  — run redis-cli inside a pod (best effort)
redis() { kc exec "$1" -c "$(ctr "$1")" -- redis-cli "${@:2}" 2>/dev/null; }

pod_ip() { kc get pod "$1" -o jsonpath='{.status.podIP}' 2>/dev/null; }
myid()   { redis "$1" cluster myid | tr -d '[:space:]'; }
ci_field() { redis "$1" cluster info | awk -F: -v k="$2" '$1==k{gsub(/\r/,"");print $2}'; }
known()  { ci_field "$1" cluster_known_nodes; }
cstate() { ci_field "$1" cluster_state; }
slots_ok()       { ci_field "$1" cluster_slots_ok; }
slots_assigned() { ci_field "$1" cluster_slots_assigned; }
ri_field() { redis "$1" info replication | awk -F: -v k="$2" '$1==k{gsub(/\r/,"");print $2}'; }
role() { ri_field "$1" role; }
link() { ri_field "$1" master_link_status; }

leader_pods()   { for i in $(seq 0 $((LEADERS-1)));   do echo "${NAME}-leader-$i";   done; }
follower_pods() { for i in $(seq 0 $((FOLLOWERS-1))); do echo "${NAME}-follower-$i"; done; }
all_pods()      { leader_pods; follower_pods; }
# expected master pod for a follower index: follower-i replicates leader-(i % LEADERS)
master_pod_for() { local fidx="${1##*-}"; echo "${NAME}-leader-$(( fidx % LEADERS ))"; }

# stale node IDs visible from leader-0 (fail/noaddr/handshake, not myself)
stale_ids() {
  redis "${NAME}-leader-0" cluster nodes \
    | awk '$3 !~ /myself/ && ($3 ~ /fail/ || $3 ~ /noaddr/ || $3 ~ /handshake/){print $1}'
}

# count fail/noaddr/handshake entries from leader-0's view
bad_count() {
  redis "${NAME}-leader-0" cluster nodes | grep -cE 'fail|noaddr|handshake' || true
}

# Wait until the whole cluster is healthy (6 nodes, ok, full slots, no bad entries).
wait_healthy() { # $1 timeout
  local t=0 to="${1:-240}"
  while (( t < to )); do
    local k s sa so b
    k=$(known "${NAME}-leader-0"); s=$(cstate "${NAME}-leader-0")
    sa=$(slots_assigned "${NAME}-leader-0"); so=$(slots_ok "${NAME}-leader-0")
    b=$(bad_count)
    if [[ "$k" == "$EXPECTED_NODES" && "$s" == "ok" && "$sa" == "16384" && "$so" == "16384" && "$b" == "0" ]]; then
      return 0
    fi
    sleep 5; t=$((t+5))
  done
  return 1
}

wait_pod_ready() { # $1 pod $2 timeout
  kc wait pod/"$1" --for=condition=Ready --timeout="${2:-180}s" >/dev/null 2>&1
}
