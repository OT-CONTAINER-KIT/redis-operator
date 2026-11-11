package statefulset

import (
	"context"

	"github.com/OT-CONTAINER-KIT/redis-operator/internal/controller/common/reconciler"
	"github.com/OT-CONTAINER-KIT/redis-operator/internal/util/annotations"
	"github.com/OT-CONTAINER-KIT/redis-operator/internal/util/maps"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/equality"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type Params struct {
	Name            string
	Namespace       string
	Replicas        int32
	ServiceName     string
	PodTemplateSpec corev1.PodTemplateSpec
}

func New(params Params) appsv1.StatefulSet {
	return appsv1.StatefulSet{
		ObjectMeta: metav1.ObjectMeta{
			Name:      params.Name,
			Namespace: params.Namespace,
		},
		Spec: appsv1.StatefulSetSpec{
			ServiceName: params.ServiceName,
			Replicas:    ptr.To(params.Replicas),
			Selector: &metav1.LabelSelector{
				MatchLabels: params.PodTemplateSpec.Labels,
			},
			Template: params.PodTemplateSpec,
		},
	}
}

func Reconcile(ctx context.Context, k8sClient client.Client, expected appsv1.StatefulSet, owner client.Object) (appsv1.StatefulSet, error) {
	reconciled, err := reconciler.Reconcile(ctx, reconciler.Params{
		Client:   k8sClient,
		Owner:    owner,
		Expected: &expected,
		NeedUpdate: func(existed client.Object) bool {
			return needUpdate(&expected, existed.(*appsv1.StatefulSet))
		},
		Update: func(existed client.Object) {
			update(&expected, existed.(*appsv1.StatefulSet))
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

func needUpdate(expected, existed *appsv1.StatefulSet) bool {
	preserveRestartAnnotation(expected, existed)
	return !maps.IsSubset(expected.Labels, existed.Labels) ||
		!maps.IsSubset(expected.Annotations, existed.Annotations) ||
		!equality.Semantic.DeepEqual(expected.Spec, existed.Spec)
}

func update(expected, existed *appsv1.StatefulSet) {
	preserveRestartAnnotation(expected, existed)
	existed.Labels = maps.Merge(existed.Labels, expected.Labels)
	existed.Annotations = maps.Merge(existed.Annotations, expected.Annotations)
	existed.Spec = expected.Spec
}

// preserveRestartAnnotation carries the `kubectl rollout restart`
// pod-template annotation from the live StatefulSet onto the expected one.
// The expected template is rebuilt from the CR and cannot source this
// annotation, so without preserving it the reconciler would strip it
// mid-rollout and issue a competing revision, which can leave pods relabeled
// to the new revision without being recreated.
func preserveRestartAnnotation(expected, existed *appsv1.StatefulSet) {
	value, ok := existed.Spec.Template.Annotations[annotations.KubectlRestartedAt]
	if !ok {
		return
	}
	if expected.Spec.Template.Annotations == nil {
		expected.Spec.Template.Annotations = map[string]string{}
	}
	if _, exists := expected.Spec.Template.Annotations[annotations.KubectlRestartedAt]; !exists {
		expected.Spec.Template.Annotations[annotations.KubectlRestartedAt] = value
	}
}
