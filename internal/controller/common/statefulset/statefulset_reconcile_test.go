package statefulset

import (
	"context"
	"testing"

	"github.com/banzaicloud/k8s-objectmatcher/patch"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/utils/ptr"
	clientfake "sigs.k8s.io/controller-runtime/pkg/client/fake"
)

func newExpected(t *testing.T) *appsv1.StatefulSet {
	t.Helper()
	sts := New(Params{
		Name:        "redis",
		Namespace:   "default",
		Replicas:    3,
		ServiceName: "redis-headless",
		PodTemplateSpec: corev1.PodTemplateSpec{
			Spec: corev1.PodSpec{
				Containers: []corev1.Container{{Name: "redis", Image: "redis:7"}},
				Tolerations: []corev1.Toleration{{
					Key:      "dedicated",
					Operator: corev1.TolerationOpExists,
				}},
			},
		},
	})
	require.NoError(t, patch.DefaultAnnotator.SetLastAppliedAnnotation(&sts))
	return &sts
}

// asStored returns sts as the API server stores it, with its defaults filled in.
func asStored(sts *appsv1.StatefulSet) *appsv1.StatefulSet {
	stored := sts.DeepCopy()
	stored.Spec.PodManagementPolicy = appsv1.OrderedReadyPodManagement
	stored.Spec.RevisionHistoryLimit = ptr.To[int32](10)
	stored.Spec.UpdateStrategy = appsv1.StatefulSetUpdateStrategy{
		Type: appsv1.RollingUpdateStatefulSetStrategyType,
		RollingUpdate: &appsv1.RollingUpdateStatefulSetStrategy{
			Partition: ptr.To[int32](0),
		},
	}
	podSpec := &stored.Spec.Template.Spec
	podSpec.RestartPolicy = corev1.RestartPolicyAlways
	podSpec.DNSPolicy = corev1.DNSClusterFirst
	podSpec.SchedulerName = corev1.DefaultSchedulerName
	podSpec.TerminationGracePeriodSeconds = ptr.To[int64](30)
	podSpec.SecurityContext = &corev1.PodSecurityContext{}
	for i := range podSpec.Containers {
		podSpec.Containers[i].ImagePullPolicy = corev1.PullIfNotPresent
		podSpec.Containers[i].TerminationMessagePath = corev1.TerminationMessagePathDefault
		podSpec.Containers[i].TerminationMessagePolicy = corev1.TerminationMessageReadFile
	}
	stored.Status = appsv1.StatefulSetStatus{Replicas: 3, ReadyReplicas: 3}
	return stored
}

func TestNeedUpdate(t *testing.T) {
	t.Run("server defaults alone do not need an update", func(t *testing.T) {
		expected := newExpected(t)
		assert.False(t, needUpdate(expected, asStored(expected)))
	})

	t.Run("a changed field is detected", func(t *testing.T) {
		existed := asStored(newExpected(t))
		expected := newExpected(t)
		expected.Spec.Template.Spec.Containers[0].Image = "redis:8"
		require.NoError(t, patch.DefaultAnnotator.SetLastAppliedAnnotation(expected))
		assert.True(t, changed(expected, existed))
	})

	t.Run("a removed field is detected", func(t *testing.T) {
		existed := asStored(newExpected(t))
		expected := newExpected(t)
		expected.Spec.Template.Spec.Tolerations = nil
		require.NoError(t, patch.DefaultAnnotator.SetLastAppliedAnnotation(expected))
		assert.True(t, changed(expected, existed))
	})

	t.Run("a field edited on the live object needs an update", func(t *testing.T) {
		expected := newExpected(t)
		existed := asStored(expected)
		existed.Spec.Template.Spec.Containers[0].Image = "redis:6"
		assert.True(t, needUpdate(expected, existed))
	})

	t.Run("an object without the last-applied annotation needs one update", func(t *testing.T) {
		expected := newExpected(t)
		existed := asStored(expected)
		delete(existed.Annotations, patch.LastAppliedConfig)
		assert.True(t, needUpdate(expected, existed))

		update(expected, existed)
		assert.False(t, needUpdate(expected, asStored(existed)))
	})
}

func TestReconcile(t *testing.T) {
	ctx := context.Background()
	cli := clientfake.NewClientBuilder().WithScheme(scheme.Scheme).Build()
	expected := func() appsv1.StatefulSet {
		sts := *newExpected(t)
		delete(sts.Annotations, patch.LastAppliedConfig)
		return sts
	}

	created, err := Reconcile(ctx, cli, expected(), nil)
	require.NoError(t, err)
	assert.Contains(t, created.Annotations, patch.LastAppliedConfig)

	again, err := Reconcile(ctx, cli, expected(), nil)
	require.NoError(t, err)
	assert.Equal(t, created.ResourceVersion, again.ResourceVersion)

	withoutTolerations := expected()
	withoutTolerations.Spec.Template.Spec.Tolerations = nil
	updated, err := Reconcile(ctx, cli, withoutTolerations, nil)
	require.NoError(t, err)
	assert.Empty(t, updated.Spec.Template.Spec.Tolerations)
}
