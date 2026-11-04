#!/bin/sh
# wait-for-recovery.sh <seconds> <keys>
#
# Polls until exactly one replication pod is a master, it is the recorded
# status.masterNode, it has both other pods attached and holds <keys> keys, and
# every Sentinel monitors it and knows at least both replicas. Later steps find
# the master through status.masterNode. Prints "recovered" on success.
#
# Runs under /bin/sh (dash) via chainsaw's `sh -c`, so keep it POSIX.
set -u

deadline=$(($(date +%s) + $1))
keys=$2
rr=redis-replication
rs=redis-sentinel-sentinel

info() {
    kubectl -n "$NAMESPACE" exec "$1" -c "$2" -- redis-cli $3 2>/dev/null | tr -d '\r'
}

while :; do
    masters=0
    master=""
    for i in 0 1 2; do
        if info "$rr-$i" "$rr" "info replication" | grep -q '^role:master$'; then
            masters=$((masters + 1))
            master="$rr-$i"
        fi
    done
    state="masters=$masters"
    if [ "$masters" -eq 1 ]; then
        ip=$(kubectl -n "$NAMESPACE" get pod "$master" -o jsonpath='{.status.podIP}')
        replicas=$(info "$master" "$rr" "info replication" | sed -n 's/^connected_slaves://p')
        dbsize=$(info "$master" "$rr" "dbsize")
        sentinels=0
        for i in 0 1 2; do
            line=$(info "$rs-$i" "$rs" "-p 26379 info sentinel" | grep '^master0:')
            known=$(echo "$line" | sed -n 's/.*,slaves=\([0-9]*\),.*/\1/p')
            # After a failover Sentinel lists the old master as a replica until
            # it is reset, so more than two known replicas is fine.
            case "$line" in
            *",address=$ip:6379,"*) [ "${known:-0}" -ge 2 ] && sentinels=$((sentinels + 1)) ;;
            esac
        done
        recorded=$(kubectl -n "$NAMESPACE" get redisreplication "$rr" -o jsonpath='{.status.masterNode}')
        state="master=$master recorded=$recorded replicas=$replicas keys=$dbsize sentinels=$sentinels"
        if [ "$recorded" = "$master" ] && [ "$replicas" = 2 ] && [ "$dbsize" = "$keys" ] && [ "$sentinels" -eq 3 ]; then
            echo "recovered: $state"
            exit 0
        fi
    fi
    if [ "$(date +%s)" -ge "$deadline" ]; then
        echo "timeout: $state"
        exit 1
    fi
    sleep 5
done
