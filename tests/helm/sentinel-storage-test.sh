#!/usr/bin/env bash
set -euo pipefail

ROOT=$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)

helm template storage-test "$ROOT/charts/redis-sentinel" | python3 -c '
import sys, yaml
sentinel = next(item for item in yaml.safe_load_all(sys.stdin) if item and item.get("kind") == "RedisSentinel")
spec = sentinel["spec"]
claim = spec["storage"]["volumeClaimTemplate"]["spec"]
assert claim["accessModes"] == ["ReadWriteOnce"]
assert claim["resources"]["requests"]["storage"] == "1Gi"
assert "storageClassName" not in claim
assert spec["podSecurityContext"]["runAsUser"] == 1000
assert spec["podSecurityContext"]["fsGroup"] == 1000
print("PASS: default Sentinel chart renders one configuration claim template and matching pod ownership")
'

helm template storage-test "$ROOT/charts/redis-sentinel" \
  --set storage.volumeClaimTemplate.spec.storageClassName=local-test \
  --set storage.volumeClaimTemplate.spec.resources.requests.storage=2Gi | python3 -c '
import sys, yaml
sentinel = next(item for item in yaml.safe_load_all(sys.stdin) if item and item.get("kind") == "RedisSentinel")
claim = sentinel["spec"]["storage"]["volumeClaimTemplate"]["spec"]
assert claim["storageClassName"] == "local-test"
assert claim["resources"]["requests"]["storage"] == "2Gi"
print("PASS: explicit Sentinel capacity and storage class are rendered")
'
