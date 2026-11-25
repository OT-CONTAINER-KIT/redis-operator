#!/usr/bin/env bash
# Renders the redis-operator Helm chart and asserts that fleetServiceMonitor
# produces a single ServiceMonitor selecting the operator-created metrics
# Services, with the expected namespace handling. Requires only `helm`.
set -euo pipefail

CHART_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/../../charts/redis-operator" && pwd)"
TEMPLATE="templates/fleet-servicemonitor.yaml"

fail() { echo "FAIL: $1" >&2; exit 1; }
pass() { echo "PASS: $1"; }

render() { helm template ro "$CHART_DIR" --namespace redis-operator --show-only "$TEMPLATE" "$@"; }
has_line() { echo "$1" | grep -qxF -- "$2"; }

# --- disabled by default ---
if skip_out="$(render 2>&1)"; then
  fail "fleet ServiceMonitor should not render by default"
fi
echo "$skip_out" | grep -q "could not find template $TEMPLATE" \
  || fail "unexpected helm error with default values: $skip_out"
pass "fleet ServiceMonitor is not rendered by default"

# --- enabled with defaults ---
out="$(render --set fleetServiceMonitor.enabled=true)"
[[ "$(echo "$out" | grep -c '^kind: ServiceMonitor$')" == 1 ]] || fail "enabled should render exactly one ServiceMonitor"
has_line "$out" "  name: ro-redis-fleet"                    || fail "default name should be <release>-redis-fleet"
has_line "$out" "  namespace: redis-operator"               || fail "ServiceMonitor should default to the release namespace"
has_line "$out" "      app.kubernetes.io/component: metrics" || fail "selector should match the operator's metrics Services"
has_line "$out" "    - key: redis_setup_type"               || fail "selector should filter on redis_setup_type"
has_line "$out" "      - cluster"                           || fail "setupTypes should default to cluster"
echo "$out" | grep -qxE '      - (standalone|replication|sentinel)' && fail "default setupTypes should only contain cluster" || true
has_line "$out" "    matchNames:"                           || fail "namespaceSelector should use matchNames by default"
has_line "$out" "    - redis-operator"                      || fail "namespaceSelector should default to the ServiceMonitor namespace"
has_line "$out" "  - port: redis-exporter"                  || fail "endpoint should scrape the redis-exporter port"
has_line "$out" "    interval: 30s"                         || fail "interval should default to 30s"
has_line "$out" "    scrapeTimeout: 10s"                    || fail "scrapeTimeout should default to 10s"
echo "$out" | grep -q 'any: true'                           && fail "default should not select all namespaces" || true
echo "$out" | grep -q 'relabelings:'                        && fail "relabelings should be omitted when empty" || true
pass "enabled renders one ServiceMonitor with the default selector and namespace"

# --- custom ServiceMonitor namespace is also the default scrape namespace ---
out="$(render --set fleetServiceMonitor.enabled=true --set fleetServiceMonitor.namespace=monitoring \
  --set fleetServiceMonitor.name=redis-fleet)"
has_line "$out" "  name: redis-fleet"     || fail "name override should be used"
has_line "$out" "  namespace: monitoring" || fail "namespace override should set metadata.namespace"
has_line "$out" "    - monitoring"        || fail "namespaceSelector should follow the namespace override"
pass "name and namespace overrides apply"

# --- explicit target namespaces ---
out="$(render --set fleetServiceMonitor.enabled=true --set 'fleetServiceMonitor.targetNamespaces={team-a,team-b}')"
has_line "$out" "    - team-a" || fail "targetNamespaces should be listed in matchNames"
has_line "$out" "    - team-b" || fail "targetNamespaces should be listed in matchNames"
has_line "$out" "    - redis-operator" && fail "targetNamespaces should replace the default namespace" || true
pass "targetNamespaces populate namespaceSelector.matchNames"

# --- allNamespaces wins over targetNamespaces ---
out="$(render --set fleetServiceMonitor.enabled=true --set fleetServiceMonitor.allNamespaces=true \
  --set 'fleetServiceMonitor.targetNamespaces={team-a}')"
has_line "$out" "    any: true" || fail "allNamespaces should render namespaceSelector.any"
echo "$out" | grep -q 'matchNames:' && fail "allNamespaces should not render matchNames" || true
pass "allNamespaces renders namespaceSelector.any and ignores targetNamespaces"

# --- multiple setup types, labels and relabelings ---
out="$(render --set fleetServiceMonitor.enabled=true \
  --set 'fleetServiceMonitor.setupTypes={cluster,sentinel}' \
  --set fleetServiceMonitor.extraLabels.release=kube-prometheus-stack \
  --set 'fleetServiceMonitor.relabelings[0].sourceLabels[0]=__meta_kubernetes_service_label_app' \
  --set 'fleetServiceMonitor.relabelings[0].targetLabel=redis_cluster' \
  --set 'fleetServiceMonitor.metricRelabelings[0].action=drop' \
  --set 'fleetServiceMonitor.metricRelabelings[0].regex=redis_commands_.*')"
has_line "$out" "      - cluster"                       || fail "setupTypes should include cluster"
has_line "$out" "      - sentinel"                      || fail "setupTypes should include sentinel"
has_line "$out" "    release: kube-prometheus-stack"    || fail "extraLabels should be added to metadata.labels"
has_line "$out" "    relabelings:"                      || fail "relabelings should be rendered on the endpoint"
has_line "$out" "      targetLabel: redis_cluster"      || fail "relabeling content should be rendered"
has_line "$out" "    metricRelabelings:"                || fail "metricRelabelings should be rendered on the endpoint"
has_line "$out" "    - action: drop"                    || fail "metricRelabeling content should be rendered"
pass "setupTypes, extraLabels, relabelings and metricRelabelings are rendered"

# --- empty setupTypes is rejected at template time ---
if err_out="$(render --set fleetServiceMonitor.enabled=true --set 'fleetServiceMonitor.setupTypes=null' 2>&1)"; then
  fail "empty setupTypes should make templating fail"
fi
echo "$err_out" | grep -q 'fleetServiceMonitor.setupTypes must list at least one' \
  || fail "unexpected helm error for empty setupTypes: $err_out"
pass "empty setupTypes is rejected"

echo "All fleet ServiceMonitor assertions passed."
