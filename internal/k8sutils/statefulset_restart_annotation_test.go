package k8sutils

import (
	"context"
	"testing"

	utilannotations "github.com/OT-CONTAINER-KIT/redis-operator/internal/util/annotations"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	k8sClientFake "k8s.io/client-go/kubernetes/fake"
	"k8s.io/utils/ptr"
)

func restartAnnotationSTS(templateAnnotations map[string]string, image string) *appsv1.StatefulSet {
	return &appsv1.StatefulSet{
		TypeMeta: metav1.TypeMeta{Kind: "StatefulSet", APIVersion: "apps/v1"},
		ObjectMeta: metav1.ObjectMeta{
			Name:            "test-sts",
			Namespace:       "redis",
			ResourceVersion: "1",
		},
		Spec: appsv1.StatefulSetSpec{
			Replicas: ptr.To(int32(3)),
			Selector: &metav1.LabelSelector{MatchLabels: map[string]string{"app": "redis"}},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels:      map[string]string{"app": "redis"},
					Annotations: templateAnnotations,
				},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{{Name: "redis", Image: image}},
				},
			},
		},
	}
}

func TestPatchStatefulSetPreservesRestartAnnotation(t *testing.T) {
	ctx := context.Background()
	const restartedAt = "2026-11-11T03:05:00Z"

	t.Run("no update when only the restart annotation differs", func(t *testing.T) {
		stored := restartAnnotationSTS(map[string]string{
			utilannotations.KubectlRestartedAt: restartedAt,
		}, "redis:7.4")
		desired := restartAnnotationSTS(nil, "redis:7.4")
		client := k8sClientFake.NewSimpleClientset(stored.DeepCopy())

		require.NoError(t, patchStatefulSet(ctx, stored, desired, "redis", false, nil, client))

		for _, action := range client.Actions() {
			assert.NotEqual(t, "update", action.GetVerb(),
				"reconcile must not fight an in-progress rollout restart")
		}
		assert.Equal(t, restartedAt,
			desired.Spec.Template.Annotations[utilannotations.KubectlRestartedAt])
	})

	t.Run("real spec change still updates and carries the annotation forward", func(t *testing.T) {
		stored := restartAnnotationSTS(map[string]string{
			utilannotations.KubectlRestartedAt: restartedAt,
		}, "redis:7.4")
		desired := restartAnnotationSTS(nil, "redis:7.4.4")
		client := k8sClientFake.NewSimpleClientset(stored.DeepCopy())

		require.NoError(t, patchStatefulSet(ctx, stored, desired, "redis", false, nil, client))

		updated, err := client.AppsV1().StatefulSets("redis").Get(ctx, "test-sts", metav1.GetOptions{})
		require.NoError(t, err)
		assert.Equal(t, "redis:7.4.4", updated.Spec.Template.Spec.Containers[0].Image)
		assert.Equal(t, restartedAt,
			updated.Spec.Template.Annotations[utilannotations.KubectlRestartedAt])
	})

	t.Run("no annotation on the live object leaves the desired template untouched", func(t *testing.T) {
		stored := restartAnnotationSTS(nil, "redis:7.4")
		desired := restartAnnotationSTS(nil, "redis:7.4")
		preservePodTemplateAnnotations(stored, desired)
		assert.Nil(t, desired.Spec.Template.Annotations)
	})
}
