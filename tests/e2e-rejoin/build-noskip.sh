#!/usr/bin/env bash
# build-noskip.sh — build operator images that IGNORE the skip-reconcile
# annotation, so a dedicated (non-ArgoCD-managed) test operator can reconcile a
# test cluster that is annotated skip-reconcile=true (which makes the shared
# ArgoCD-managed v0.22.2 operator leave it alone). One image per branch.
set -euo pipefail
REPO="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
REG="${REG:-docker-local.artifactory.twitter.biz/storage}"
SKIP_FILE="internal/controller/common/skip_reconcile.go"
START="$(cd "$REPO" && git rev-parse --abbrev-ref HEAD)"

build() { # $1 branch  $2 tag
  cd "$REPO"
  git checkout "$1" >/dev/null 2>&1
  # point the RedisCluster skip-reconcile check at an annotation we never set
  sed -i '' 's#rediscluster.opstreelabs.in/skip-reconcile"#rediscluster.opstreelabs.in/skip-reconcile-NEVER"#' "$SKIP_FILE"
  grep -q 'skip-reconcile-NEVER' "$SKIP_FILE" || { echo "patch failed on $1"; exit 1; }
  GOOS=linux GOARCH=amd64 CGO_ENABLED=0 go build -ldflags "-s -w" -o /tmp/op-noskip ./cmd/main.go
  git checkout -- "$SKIP_FILE"
  local W; W=$(mktemp -d); cp /tmp/op-noskip "$W/operator"; cp tests/e2e-rejoin/Dockerfile.package "$W/Dockerfile"
  docker buildx build --builder xbuilder --platform=linux/amd64 --push -t "$REG/redis-operator:$2" -f "$W/Dockerfile" "$W" >/dev/null 2>&1
  rm -rf "$W"
  echo "pushed $REG/redis-operator:$2"
}

build v0.22.2 v0.22.2-noskip
build upstream/main upstream-main-noskip
build fix/forget-stale-nodes forget-stale-noskip
build fix/rejoin-isolated-nodes fix-rejoin-noskip
cd "$REPO" && git checkout "$START" >/dev/null 2>&1
echo "done; back on $START"
