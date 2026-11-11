package statefulset

import (
	"testing"

	"github.com/OT-CONTAINER-KIT/redis-operator/internal/util/annotations"
	"github.com/stretchr/testify/assert"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func reconcileSTS(templateAnnotations map[string]string, image string) *appsv1.StatefulSet {
	return &appsv1.StatefulSet{
		ObjectMeta: metav1.ObjectMeta{Name: "test-sts", Namespace: "redis"},
		Spec: appsv1.StatefulSetSpec{
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

func TestNeedUpdatePreservesRestartAnnotation(t *testing.T) {
	const restartedAt = "2026-11-11T03:05:00Z"

	t.Run("no update needed when only the restart annotation differs", func(t *testing.T) {
		existed := reconcileSTS(map[string]string{annotations.KubectlRestartedAt: restartedAt}, "redis:7.4")
		expected := reconcileSTS(nil, "redis:7.4")

		assert.False(t, needUpdate(expected, existed))
		assert.Equal(t, restartedAt,
			expected.Spec.Template.Annotations[annotations.KubectlRestartedAt])
	})

	t.Run("real spec difference still needs an update and keeps the annotation", func(t *testing.T) {
		existed := reconcileSTS(map[string]string{annotations.KubectlRestartedAt: restartedAt}, "redis:7.4")
		expected := reconcileSTS(nil, "redis:7.4.4")

		assert.True(t, needUpdate(expected, existed))
		assert.Equal(t, restartedAt,
			expected.Spec.Template.Annotations[annotations.KubectlRestartedAt])
	})

	t.Run("nothing to preserve when the live template has no annotation", func(t *testing.T) {
		existed := reconcileSTS(nil, "redis:7.4")
		expected := reconcileSTS(nil, "redis:7.4")

		assert.False(t, needUpdate(expected, existed))
		assert.Nil(t, expected.Spec.Template.Annotations)
	})
}

func TestUpdatePreservesRestartAnnotation(t *testing.T) {
	const restartedAt = "2026-11-11T03:05:00Z"

	existed := reconcileSTS(map[string]string{annotations.KubectlRestartedAt: restartedAt}, "redis:7.4")
	expected := reconcileSTS(nil, "redis:7.4.4")

	update(expected, existed)

	assert.Equal(t, "redis:7.4.4", existed.Spec.Template.Spec.Containers[0].Image)
	assert.Equal(t, restartedAt,
		existed.Spec.Template.Annotations[annotations.KubectlRestartedAt])
}
