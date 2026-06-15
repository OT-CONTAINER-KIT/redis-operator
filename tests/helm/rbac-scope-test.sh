#!/usr/bin/env bash
# Renders the redis-operator Helm chart and asserts that the rbac.scope value
# produces valid RBAC resources for both the default/cluster scope and the
# namespace scope. Requires only `helm` (no cluster needed).
set -euo pipefail

CHART_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/../../charts/redis-operator" && pwd)"

fail() { echo "FAIL: $1" >&2; exit 1; }
pass() { echo "PASS: $1"; }

# --- default scope (rbac.scope unset) -> cluster-wide RBAC ---
default_out="$(helm template ro "$CHART_DIR" --namespace redis-operator \
  --show-only templates/role.yaml --show-only templates/role-binding.yaml)"

echo "$default_out" | grep -q '^kind: ClusterRole$'        || fail "default scope should render a ClusterRole"
echo "$default_out" | grep -q '^kind: ClusterRoleBinding$' || fail "default scope should render a ClusterRoleBinding"
echo "$default_out" | grep -q 'nonResourceURLs'            || fail "default ClusterRole should keep the nonResourceURLs rule"
echo "$default_out" | grep -q 'aggregate-to-admin'         || fail "default ClusterRole should keep the aggregate-to-admin label"
echo "$default_out" | grep -q 'customresourcedefinitions'  || fail "default ClusterRole should keep the CRD rule"
pass "default scope renders ClusterRole/ClusterRoleBinding"

# --- explicit cluster scope behaves like the default ---
cluster_out="$(helm template ro "$CHART_DIR" --set rbac.scope=cluster \
  --show-only templates/role.yaml)"
echo "$cluster_out" | grep -q '^kind: ClusterRole$' || fail "scope=cluster should render a ClusterRole"
pass "scope=cluster renders ClusterRole"

# --- namespace scope -> namespaced Role/RoleBinding ---
ns_out="$(helm template ro "$CHART_DIR" --namespace my-redis --set rbac.scope=namespace \
  --show-only templates/role.yaml --show-only templates/role-binding.yaml)"

echo "$ns_out" | grep -q '^kind: Role$'          || fail "namespace scope should render a Role"
echo "$ns_out" | grep -q '^kind: RoleBinding$'   || fail "namespace scope should render a RoleBinding"
echo "$ns_out" | grep -q '^  namespace: my-redis$' || fail "namespaced Role/RoleBinding should set metadata.namespace"

# A namespaced Role is rejected by the API server if it carries any cluster-only construct.
echo "$ns_out" | grep -q 'nonResourceURLs'           && fail "namespaced Role must not contain nonResourceURLs" || true
echo "$ns_out" | grep -q 'aggregate-to-admin'        && fail "namespaced Role must not carry the aggregate-to-admin label" || true
echo "$ns_out" | grep -q 'customresourcedefinitions' && fail "namespaced Role must not grant cluster-scoped CRDs" || true
echo "$ns_out" | grep -qE '^[[:space:]]*-[[:space:]]*namespaces$' && fail "namespaced Role must not grant the cluster-scoped namespaces resource" || true
pass "namespace scope renders a clean Role/RoleBinding with no cluster-only rules"

# --- invalid scope is rejected at template time ---
if helm template ro "$CHART_DIR" --set rbac.scope=invalid >/dev/null 2>&1; then
  fail "an invalid rbac.scope should make templating fail"
fi
pass "invalid scope is rejected"

echo "All RBAC scope assertions passed."
