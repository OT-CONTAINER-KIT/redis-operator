package statefulset

import (
	"bytes"
	"context"
	"fmt"

	"github.com/OT-CONTAINER-KIT/redis-operator/internal/controller/common/reconciler"
	"github.com/OT-CONTAINER-KIT/redis-operator/internal/util/maps"
	"github.com/banzaicloud/k8s-objectmatcher/patch"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

type Params struct {
	Name            string
	Namespace       string
	Annotations     map[string]string
	Replicas        int32
	MinReadySeconds int32
	ServiceName     string
	PodTemplateSpec corev1.PodTemplateSpec
}

func New(params Params) appsv1.StatefulSet {
	return appsv1.StatefulSet{
		ObjectMeta: metav1.ObjectMeta{
			Name:        params.Name,
			Namespace:   params.Namespace,
			Annotations: maps.Copy(params.Annotations),
		},
		Spec: appsv1.StatefulSetSpec{
			ServiceName:     params.ServiceName,
			Replicas:        ptr.To(params.Replicas),
			MinReadySeconds: params.MinReadySeconds,
			Selector: &metav1.LabelSelector{
				MatchLabels: params.PodTemplateSpec.Labels,
			},
			Template: params.PodTemplateSpec,
		},
	}
}

func Reconcile(ctx context.Context, k8sClient client.Client, expected appsv1.StatefulSet, owner client.Object) (appsv1.StatefulSet, error) {
	if err := recordAppliedSpec(&expected); err != nil {
		return expected, fmt.Errorf("record the applied spec of statefulset %s/%s: %w", expected.Namespace, expected.Name, err)
	}
	var replaceSpec bool
	reconciled, err := reconciler.Reconcile(ctx, reconciler.Params{
		Client:   k8sClient,
		Owner:    owner,
		Expected: &expected,
		NeedUpdate: func(existed client.Object) bool {
			metadataChanged, specChanged, err := needUpdate(&expected, existed.(*appsv1.StatefulSet))
			if err != nil {
				log.FromContext(ctx).Error(err, "Could not compare the statefulset spec, replacing it",
					"namespace", expected.Namespace, "name", expected.Name)
				specChanged = true
			}
			replaceSpec = specChanged
			return metadataChanged || specChanged
		},
		Update: func(existed client.Object) {
			update(&expected, existed.(*appsv1.StatefulSet), replaceSpec)
		},
	})
	// reconciler.Reconcile can return (nil, err) when the downstream
	// Create/Update failed; the previous unconditional type assertion
	// panicked the reconciler with 'interface conversion: client.Object
	// is nil, not *v1.StatefulSet' and crash-looped the operator. Fall
	// back to the expected object so the controller receives a valid
	// StatefulSet alongside the error and schedules a requeue.
	if err != nil || reconciled == nil {
		return expected, err
	}
	sts, ok := reconciled.(*appsv1.StatefulSet)
	if !ok || sts == nil {
		return expected, err
	}
	return *sts, err
}

// recordAppliedSpec stores the spec of expected in its last-applied
// annotation, so creating and updating the StatefulSet records it as well.
func recordAppliedSpec(expected *appsv1.StatefulSet) error {
	applied := &appsv1.StatefulSet{Spec: *expected.Spec.DeepCopy()}
	if err := patch.DefaultAnnotator.SetLastAppliedAnnotation(applied); err != nil {
		return err
	}
	if expected.Annotations == nil {
		expected.Annotations = map[string]string{}
	}
	expected.Annotations[patch.LastAppliedConfig] = applied.Annotations[patch.LastAppliedConfig]
	return nil
}

// needUpdate reports whether existed lacks a label or annotation of expected,
// and whether its spec has to be replaced by the one of expected: when the
// operator wants another spec than it last applied, or when someone changed a
// field the operator sets.
//
// The spec is compared three-way, as k8sutils.patchStatefulSet does for the
// Redis StatefulSets: the spec the operator last applied, recorded in the
// patch.LastAppliedConfig annotation, against the wanted and the live one. A
// field someone else adds to the spec is then neither a change nor reverted,
// such as the pod template annotation or environment variable Reloader sets to
// restart the pods after a Secret changes. Comparing the specs in full would
// revert it on the next reconcile, in the middle of the rollout it started.
//
// Only the spec is recorded and compared that way. Labels and annotations are
// merged into the live ones rather than replaced, so they are checked as a
// subset.
func needUpdate(expected, existed *appsv1.StatefulSet) (metadataChanged, specChanged bool, err error) {
	wantedAnnotations := maps.Copy(expected.Annotations)
	delete(wantedAnnotations, patch.LastAppliedConfig)
	metadataChanged = !maps.IsSubset(expected.Labels, existed.Labels) ||
		!maps.IsSubset(wantedAnnotations, existed.Annotations)

	wanted, err := patch.DefaultAnnotator.GetOriginalConfiguration(expected)
	if err != nil {
		return metadataChanged, true, fmt.Errorf("read the wanted spec: %w", err)
	}
	applied, err := patch.DefaultAnnotator.GetOriginalConfiguration(existed)
	if err != nil {
		return metadataChanged, true, fmt.Errorf("read the last applied spec: %w", err)
	}
	if !bytes.Equal(wanted, applied) {
		return metadataChanged, true, nil
	}

	live := &appsv1.StatefulSet{
		ObjectMeta: metav1.ObjectMeta{Annotations: map[string]string{patch.LastAppliedConfig: existed.Annotations[patch.LastAppliedConfig]}},
		Spec:       existed.Spec,
	}
	result, err := patch.DefaultPatchMaker.Calculate(live, &appsv1.StatefulSet{Spec: expected.Spec},
		patch.IgnoreStatusFields(),
		patch.IgnoreField("metadata"),
	)
	if err != nil {
		return metadataChanged, true, fmt.Errorf("compare the spec: %w", err)
	}
	return metadataChanged, !result.IsEmpty(), nil
}

// update merges the labels and annotations of expected into existed and, when
// replaceSpec is set, replaces its spec. Fields others added to the spec are
// dropped then, as with the Redis StatefulSets; the operator's own change
// restarts the pods anyway.
func update(expected, existed *appsv1.StatefulSet, replaceSpec bool) {
	existed.Labels = maps.Merge(existed.Labels, expected.Labels)
	existed.Annotations = maps.Merge(existed.Annotations, expected.Annotations)
	if replaceSpec {
		existed.Spec = expected.Spec
	}
}
