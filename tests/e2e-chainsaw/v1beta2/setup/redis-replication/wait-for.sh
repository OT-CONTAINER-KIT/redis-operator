#!/bin/sh
# wait-for.sh <seconds> <expected> <command>
#
# Polls <command> until its stdout equals <expected>, then prints "matched".
# The hostname-announcement steps assert values that live inside Redis and
# Sentinel rather than in the Kubernetes API, so they cannot be chainsaw
# asserts and need a bounded poll instead.
#
# Runs under /bin/sh (dash) via chainsaw's `sh -c`, so keep it POSIX.
set -u

deadline=$(($(date +%s) + $1))
expected=$2
command=$3

while :; do
    got=$(sh -c "$command" 2>/dev/null)
    if [ "$got" = "$expected" ]; then
        echo "matched: $got"
        exit 0
    fi
    if [ "$(date +%s)" -ge "$deadline" ]; then
        echo "timeout: got [$got], want [$expected]"
        exit 1
    fi
    sleep 5
done
