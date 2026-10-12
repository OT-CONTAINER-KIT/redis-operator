#!/bin/sh
# delete-pods.sh <seconds> <pod>...
#
# Deletes the pods at once and waits until every one of them is gone. The
# StatefulSets recreate them under the same names, so they are matched by UID.
# Without the wait the next step can still reach a pod that is shutting down
# and take it for the recovered set. Prints "deleted" on success.
#
# Runs under /bin/sh (dash) via chainsaw's `sh -c`, so keep it POSIX.
set -u

deadline=$(($(date +%s) + $1))
shift

uids=""
for pod in "$@"; do
    uids="$uids $(kubectl -n "$NAMESPACE" get pod "$pod" -o jsonpath='{.metadata.uid}')"
done
kubectl -n "$NAMESPACE" delete pod --wait=false "$@" || exit 1

while :; do
    live=" $(kubectl -n "$NAMESPACE" get pods -o jsonpath='{.items[*].metadata.uid}') "
    left=""
    for uid in $uids; do
        case "$live" in
        *" $uid "*) left="$left $uid" ;;
        esac
    done
    if [ -z "$left" ]; then
        echo "deleted: $*"
        exit 0
    fi
    if [ "$(date +%s)" -ge "$deadline" ]; then
        echo "timeout: still running:$left"
        exit 1
    fi
    sleep 2
done
