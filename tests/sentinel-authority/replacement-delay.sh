#!/bin/sh
# Disposable fixture only: model an image/startup delay after protected init.
set -eu
if [ "${SENTINEL_BOOTSTRAP_REQUIRED:-false}" = true ] && [ -f /data/.authority-delay-arm ]; then
    mv /data/.authority-delay-arm /data/.authority-delay-consumed
    while [ ! -f /data/.authority-delay-release ]; do
        sleep 1
    done
fi
exec /usr/local/bin/redis-server.real "$@"
