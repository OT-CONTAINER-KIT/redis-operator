#!/usr/bin/env bash
# Renders the redis-operator Helm chart and asserts that the rbac.scope value
# produces valid RBAC resources for both the default/cluster scope and the
# namespace scope. Requires `helm` and either `kubectl` or `kustomize`
# (no cluster needed).
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
CHART_DIR="$ROOT_DIR/charts/redis-operator"

fail() { echo "FAIL: $1" >&2; exit 1; }
pass() { echo "PASS: $1"; }

# --- default scope (rbac.scope unset) -> cluster-wide RBAC ---
default_out="$(helm template ro "$CHART_DIR" --namespace redis-operator \
  --show-only templates/role.yaml --show-only templates/role-binding.yaml)"

echo "$default_out" | grep -q '^kind: ClusterRole$'        || fail "default scope should render a ClusterRole"
echo "$default_out" | grep -q '^kind: ClusterRoleBinding$' || fail "default scope should render a ClusterRoleBinding"
echo "$default_out" | grep -q 'customresourcedefinitions'  || fail "default ClusterRole should keep the CRD rule"
echo "$default_out" | grep -q 'aggregate-to-'              && fail "operator ClusterRole must not carry any aggregate-to-* label" || true
# nonResourceURLs break namespaced Role copies of aggregated admin (#1858).
if echo "$default_out" | grep -qE '^[[:space:]]*-?[[:space:]]*nonResourceURLs:'; then
  fail "operator ClusterRole must not contain nonResourceURLs"
fi
if grep -q 'nonResourceURLs' "$ROOT_DIR/config/rbac/role.yaml"; then
  fail "generated manager-role must not contain nonResourceURLs"
fi
if grep -q 'kubebuilder:rbac:urls=' "$ROOT_DIR/api/rbac.go"; then
  fail "RBAC markers must not regenerate nonResourceURLs"
fi

if command -v kubectl >/dev/null 2>&1; then
  kustomize_out="$(kubectl kustomize "$ROOT_DIR/config/rbac")"
elif command -v kustomize >/dev/null 2>&1; then
  kustomize_out="$(kustomize build "$ROOT_DIR/config/rbac")"
else
  fail "kubectl or kustomize is required to validate generated RBAC"
fi
manager_role="$(echo "$kustomize_out" | awk 'BEGIN { RS = "---" } /kind: ClusterRole/ && /name: manager-role/ { print }')"
echo "$manager_role" | grep -q 'name: manager-role' || fail "kustomize output should include manager-role"
echo "$manager_role" | grep -q 'aggregate-to-' && fail "kustomize manager-role must not carry any aggregate-to-* label" || true
echo "$manager_role" | grep -q 'nonResourceURLs' && fail "kustomize manager-role must not contain nonResourceURLs" || true
# aggregate-to-admin ClusterRoles are the view/edit roles; they must stay free of nonResourceURLs
agg_roles="$(echo "$kustomize_out" | awk 'BEGIN { RS = "---" } /kind: ClusterRole/ && /aggregate-to-admin/ { print }')"
echo "$agg_roles" | grep -q 'aggregate-to-admin' || fail "kustomize should include aggregate-to-admin ClusterRoles"
echo "$agg_roles" | grep -q 'nonResourceURLs' && fail "aggregate-to-admin ClusterRoles must not contain nonResourceURLs" || true
pass "default scope renders ClusterRole/ClusterRoleBinding"

# --- aggregation roles: default cluster scope aggregates into view, edit and admin ---
agg_out="$(helm template ro "$CHART_DIR" --show-only templates/aggregate-roles.yaml)"
echo "$agg_out" | grep -q 'aggregate-to-view: "true"'  || fail "default should render an aggregate-to-view ClusterRole"
echo "$agg_out" | grep -q 'aggregate-to-edit: "true"'  || fail "default should render an aggregate-to-edit ClusterRole"
echo "$agg_out" | grep -q 'aggregate-to-admin: "true"' || fail "default should render an aggregate-to-admin label"
echo "$agg_out" | grep -q 'finalizers'                 && fail "aggregation roles must not grant finalizers" || true
echo "$agg_out" | grep -q 'nonResourceURLs'            && fail "aggregation roles must not contain nonResourceURLs" || true
# view role: 3 labels, read verbs only; edit role: 2 labels, write verbs only
view_doc="$(echo "$agg_out" | awk -v pat="\n  name: [^\n]*-view\n" '/^---$/{if(d~pat)print d; d=""; next}{d=d $0 "\n"}END{if(d~pat)print d}')"
edit_doc="$(echo "$agg_out" | awk -v pat="\n  name: [^\n]*-edit\n" '/^---$/{if(d~pat)print d; d=""; next}{d=d $0 "\n"}END{if(d~pat)print d}')"
for l in view edit admin; do
  echo "$view_doc" | grep -q "aggregate-to-$l: \"true\"" || fail "view ClusterRole must carry aggregate-to-$l"
done
echo "$edit_doc" | grep -q 'aggregate-to-view' && fail "edit ClusterRole must not carry aggregate-to-view" || true
for l in edit admin; do
  echo "$edit_doc" | grep -q "aggregate-to-$l: \"true\"" || fail "edit ClusterRole must carry aggregate-to-$l"
done
for v in create update patch delete deletecollection; do
  echo "$view_doc" | grep -qx "  - $v" && fail "view ClusterRole must not grant $v" || true
  echo "$edit_doc" | grep -qx "  - $v" || fail "edit ClusterRole must grant $v"
done
for v in get list watch; do
  echo "$view_doc" | grep -qx "  - $v" || fail "view ClusterRole must grant $v"
  echo "$edit_doc" | grep -qx "  - $v" && fail "edit ClusterRole must not duplicate read verb $v" || true
done
pass "default scope renders view and edit/admin aggregation ClusterRoles"

# --- aggregation roles are rendered nowhere else ---
for args in "--set rbac.aggregate=false" "--set rbac.enabled=false" "--set rbac.scope=namespace"; do
  # shellcheck disable=SC2086
  if skip_out="$(helm template ro "$CHART_DIR" $args --show-only templates/aggregate-roles.yaml 2>&1)"; then
    fail "aggregation roles should not render with: $args"
  fi
  # Only "template not rendered" counts as skipped; any other error is a real failure.
  echo "$skip_out" | grep -q 'could not find template templates/aggregate-roles.yaml' \
    || fail "unexpected helm error with: $args: $skip_out"
done
pass "aggregation roles are skipped for rbac.aggregate=false, rbac.enabled=false and namespace scope"

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
echo "$ns_out" | grep -q 'aggregate-to-'            && fail "namespaced Role must not carry any aggregate-to-* label" || true
echo "$ns_out" | grep -q 'customresourcedefinitions' && fail "namespaced Role must not grant cluster-scoped CRDs" || true
echo "$ns_out" | grep -qE '^[[:space:]]*-[[:space:]]*namespaces$' && fail "namespaced Role must not grant the cluster-scoped namespaces resource" || true
pass "namespace scope renders a clean Role/RoleBinding with no cluster-only rules"

# --- invalid scope is rejected at template time ---
if helm template ro "$CHART_DIR" --set rbac.scope=invalid >/dev/null 2>&1; then
  fail "an invalid rbac.scope should make templating fail"
fi
pass "invalid scope is rejected"

echo "All RBAC scope assertions passed."
