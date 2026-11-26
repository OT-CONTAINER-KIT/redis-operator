// Package podtemplate implements the DeferPodTemplateUpdates feature gate: it
// keeps the pod template of a Redis StatefulSet unchanged when only the
// operator, not the owning custom resource, changed what the template would be.
package podtemplate

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"sort"
	"strconv"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	// AnnotationApply is set by the resource owner on the custom resource. Any
	// new value applies the pod template the running operator generates, which
	// rolls the pods. It is never copied to the StatefulSet or the pods.
	AnnotationApply = "redis.opstreelabs.in/apply-pod-template"

	// AnnotationInputs is set by the operator on the StatefulSet. It records the
	// custom resource inputs (generation, labels, annotations and resources the
	// template is rendered from) the current pod template belongs to.
	AnnotationInputs = "redis.opstreelabs.in/pod-template-inputs"

	// AnnotationHash is set by the operator on the StatefulSet. It is a hash of
	// the pod template the operator last wrote to it.
	AnnotationHash = "redis.opstreelabs.in/pod-template-hash"

	// AnnotationPending is set by the operator on the StatefulSet to "true" when
	// the running operator would generate a different pod template than the one
	// the StatefulSet carries, and to "false" otherwise.
	AnnotationPending = "redis.opstreelabs.in/pod-template-update-pending"
)

// InputsKey returns a key for the custom resource inputs a pod template is
// rendered from. It changes when the spec changes (metadata.generation), when
// a label or annotation changes, including AnnotationApply, and when one of the
// extra inputs changes. extra carries values the template is rendered from that
// do not live in the custom resource, such as allocated NodePorts.
//
// The key must not depend on anything the operator version decides, otherwise
// an operator upgrade would change it and roll the pods.
func InputsKey(owner metav1.Object, extra ...string) string {
	h := sha256.New()
	write := func(s string) {
		h.Write([]byte(strconv.Itoa(len(s))))
		h.Write([]byte{':'})
		h.Write([]byte(s))
	}
	write(strconv.FormatInt(owner.GetGeneration(), 10))
	for _, m := range []map[string]string{owner.GetLabels(), owner.GetAnnotations()} {
		keys := make([]string, 0, len(m))
		for k := range m {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		write(strconv.Itoa(len(keys)))
		for _, k := range keys {
			write(k)
			write(m[k])
		}
	}
	write(strconv.Itoa(len(extra)))
	for _, e := range extra {
		write(e)
	}
	return hex.EncodeToString(h.Sum(nil))[:16]
}

// Hash returns a hash of a pod template as the operator renders it, before the
// API server adds defaults.
func Hash(template *corev1.PodTemplateSpec) string {
	b, err := json.Marshal(template)
	if err != nil {
		return ""
	}
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:])[:16]
}

// Result reports what Reconcile decided.
type Result struct {
	// Deferred is true when the desired StatefulSet keeps the stored pod template.
	Deferred bool
	// Pending is true when the kept template differs from the generated one.
	Pending bool
	// PendingKnown is false when the operator cannot tell whether the kept
	// template differs, because the StatefulSet was written before the feature
	// was enabled and no copy of the template the operator wrote is available.
	PendingKnown bool
}

// Reconcile decides which pod template desired carries and sets the
// annotations above on it.
//
//   - desired is the StatefulSet the running operator generated. It is changed
//     in place.
//   - stored is the StatefulSet in the cluster.
//   - applied is the pod template the operator last wrote to stored, without
//     API server defaults, or nil when it is not known.
//   - inputsKey is InputsKey of the owning custom resource.
//
// When the inputs changed since the stored template was written, desired keeps
// the generated template. Otherwise only the operator changed, and desired
// gets the template from applied, or from stored when applied is nil, so the
// pods are not re-created. A StatefulSet without AnnotationInputs is treated as
// unchanged: it is adopted, so that enabling the gate together with an operator
// upgrade does not roll the pods.
func Reconcile(desired, stored *appsv1.StatefulSet, applied *corev1.PodTemplateSpec, inputsKey string) Result {
	generatedHash := Hash(&desired.Spec.Template)
	storedInputs := stored.GetAnnotations()[AnnotationInputs]

	annotations := desired.GetAnnotations()
	if annotations == nil {
		annotations = map[string]string{}
	}
	annotations[AnnotationInputs] = inputsKey
	defer desired.SetAnnotations(annotations)

	if storedInputs != "" && storedInputs != inputsKey {
		annotations[AnnotationHash] = generatedHash
		annotations[AnnotationPending] = "false"
		return Result{PendingKnown: true}
	}

	appliedHash := stored.GetAnnotations()[AnnotationHash]
	if applied != nil {
		appliedHash = Hash(applied)
		desired.Spec.Template = *applied.DeepCopy()
	} else {
		desired.Spec.Template = *stored.Spec.Template.DeepCopy()
	}
	if appliedHash == "" {
		delete(annotations, AnnotationHash)
		delete(annotations, AnnotationPending)
		return Result{Deferred: true}
	}
	pending := appliedHash != generatedHash
	annotations[AnnotationHash] = appliedHash
	annotations[AnnotationPending] = strconv.FormatBool(pending)
	return Result{Deferred: true, Pending: pending, PendingKnown: true}
}

// SetCreated sets the annotations on a StatefulSet that is about to be created
// with the generated template.
func SetCreated(desired *appsv1.StatefulSet, inputsKey string) {
	annotations := desired.GetAnnotations()
	if annotations == nil {
		annotations = map[string]string{}
	}
	annotations[AnnotationInputs] = inputsKey
	annotations[AnnotationHash] = Hash(&desired.Spec.Template)
	annotations[AnnotationPending] = "false"
	desired.SetAnnotations(annotations)
}
