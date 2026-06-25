#!/usr/bin/env bash
# inject.sh — isolation injections for the rejoin matrix.
#   ./inject.sh <INJ-id> [victim-pod ...]
# Prints the victim pod(s) it isolated, one per line.
#
# Design note (validated empirically): injections that CLUSTER FORGET the old
# node id are self-defeating — forgetting drops leader-0's node count below the
# desired total, which makes the operator's cluster-creation path
# (ExecuteRedisReplicationCommand) re-MEET every pod, so even stock v0.22.2
# recovers. The durable, discriminating repro is a pod delete on a pure-cache
# cluster (no nodeConfVolume): the pod returns isolated under a NEW node id while
# the OLD id lingers as a stale fail/noaddr entry that keeps leader-0's count at
# the expected total — so the creation path never fires and the live isolated
# pod is only recovered by per-pod probing (the fix).
set -uo pipefail
HERE="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
source "$HERE/lib.sh"

del_isolate() { # delete a pod and wait for its isolated replacement to be Ready
  local v="$1"
  kc delete pod "$v" --wait=true --timeout=120s >/dev/null 2>&1
  wait_pod_ready "$v" 180
  echo "$v"
}

inj() {
  local id="$1"; shift
  case "$id" in
    INJ-1) # reset-soft a follower — the current PR injection (negative control:
           # gossip/operator re-MEET recovers it on every image, incl. v0.22.2)
      local v="${1:-${NAME}-follower-2}"
      redis "$v" cluster reset soft >/dev/null 2>&1
      echo "$v" ;;
    INJ-2) # delete one follower (pure-cache) — durable isolated-after-delete
      del_isolate "${1:-${NAME}-follower-2}" ;;
    INJ-3) # delete two followers at once — batch robustness
      local vs=("$@"); [[ ${#vs[@]} -eq 0 ]] && vs=("${NAME}-follower-1" "${NAME}-follower-2")
      local v
      for v in "${vs[@]}"; do kc delete pod "$v" --wait=false >/dev/null 2>&1; done
      for v in "${vs[@]}"; do wait_pod_ready "$v" 180; echo "$v"; done ;;
    INJ-5) # delete a NON-SEED LEADER (pure-cache): its follower is promoted and
           # takes the slots; the returned leader pod comes back isolated. Tests
           # the leader path (the fix MEETs leaders but does not REPLICATE them),
           # and whether the returned ex-leader rejoins as a replica of the new
           # master or strands as an empty master (split shard).
      del_isolate "${1:-${NAME}-leader-1}" ;;
    INJ-4) # CLUSTER RESET HARD a follower in place, no forget: new node id, knows
           # nobody, but peers keep the OLD id (no count drop). No stale entry is
           # left for forget-stale to act on, and the victim is not auto-rejoined
           # as a proper slave — isolating the case where direct per-pod rejoin
           # (CLUSTER MEET + REPLICATE) is the only thing that restores it.
      local v="${1:-${NAME}-follower-2}"
      redis "$v" cluster reset hard >/dev/null 2>&1
      echo "$v" ;;
    *) echo "unknown injection: $id" >&2; return 2 ;;
  esac
}

inj "$@"
