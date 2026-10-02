#!/usr/bin/env bash
# Verify that Redis charts can share a parent chart without overriding helpers.
set -euo pipefail

REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
test_dir="$(mktemp -d)"
trap 'rm -rf "$test_dir"' EXIT

charts=(redis redis-cluster redis-replication redis-sentinel)
values_keys=(redisStandalone redisCluster redisReplication redisSentinel)
kinds=(Redis RedisCluster RedisReplication RedisSentinel)

render_combination() {
  rm -rf "$test_dir/charts"
  mkdir -p "$test_dir/charts"
  cat > "$test_dir/Chart.yaml" <<'EOF'
apiVersion: v2
name: redis-helper-test
version: 0.1.0
dependencies:
EOF
  local args=() selected=() index chart name label_name
  for index in "$@"; do
    chart="${charts[$index]}"
    selected+=("$chart")
    cp -R "$REPO_ROOT/charts/$chart" "$test_dir/charts/$chart"
    cat >> "$test_dir/Chart.yaml" <<EOF
  - name: $chart
    version: '*'
EOF
    name="$chart-example"
    args+=(--set "$chart.${values_keys[$index]}.name=$name")
    args+=(--set "$chart.labels.example=$chart")
  done

  helm template helper-test "$test_dir" --namespace redis-test "${args[@]}" > "$test_dir/rendered.yaml"
  for index in "$@"; do
    chart="${charts[$index]}"
    name="$chart-example"
    label_name=helper-test
    if ((index < 2)); then
      label_name="$name"
    fi
    yq --exit-status "select(.kind == \"${kinds[$index]}\") |
      .metadata.name == \"$name\" and
      .metadata.labels.\"app.kubernetes.io/name\" == \"$label_name\" and
      .metadata.labels.example == \"$chart\"" "$test_dir/rendered.yaml" > /dev/null
  done
  echo "PASS: ${selected[*]} render with each chart's name and labels"
}

for ((first = 0; first < ${#charts[@]}; first++)); do
  for ((second = first + 1; second < ${#charts[@]}; second++)); do
    render_combination "$first" "$second"
  done
done
render_combination 0 1 2 3
