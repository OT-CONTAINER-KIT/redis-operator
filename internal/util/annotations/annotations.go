// Package annotations holds well-known annotation keys that the operator
// needs to preserve across reconciliation but does not own.
package annotations

// KubectlRestartedAt is injected into a workload's pod template by
// `kubectl rollout restart` to trigger a rolling update. The operator must
// carry it across reconciles instead of overwriting the template, otherwise
// a reconcile competes with the in-progress rollout and can leave pods
// relabeled to the new revision without being recreated.
const KubectlRestartedAt = "kubectl.kubernetes.io/restartedAt"
