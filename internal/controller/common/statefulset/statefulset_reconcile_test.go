package statefulset

import (
	"context"
	"testing"

	"github.com/banzaicloud/k8s-objectmatcher/patch"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

const reloaderAnnotation = "reloader.stakater.com/last-reloaded-from"

func newTestStatefulSet(image string) appsv1.StatefulSet {
	return New(Params{
		Name:        "redis-replication-s",
		Namespace:   "default",
		Annotations: map[string]string{"redis.opstreelabs.in": "true"},
		Replicas:    3,
		ServiceName: "redis-replication-s-hl",
		PodTemplateSpec: corev1.PodTemplateSpec{
			ObjectMeta: metav1.ObjectMeta{Labels: map[string]string{"app": "redis-replication-s"}},
			Spec: corev1.PodSpec{
				Containers: []corev1.Container{{
					Name:  "sentinel",
					Image: image,
					Env:   []corev1.EnvVar{{Name: "SENTINEL_PORT", Value: "26379"}},
					Ports: []corev1.ContainerPort{{Name: "sentinel", ContainerPort: 26379}},
				}},
			},
		},
	})
}

// withAPIDefaults sets fields the API server defaults on a StatefulSet the
// operator created without them.
func withAPIDefaults(sts *appsv1.StatefulSet) {
	sts.Spec.RevisionHistoryLimit = ptr.To[int32](10)
	sts.Spec.PodManagementPolicy = appsv1.OrderedReadyPodManagement
	sts.Spec.UpdateStrategy = appsv1.StatefulSetUpdateStrategy{Type: appsv1.RollingUpdateStatefulSetStrategyType}
	pod := &sts.Spec.Template.Spec
	pod.RestartPolicy = corev1.RestartPolicyAlways
	pod.DNSPolicy = corev1.DNSClusterFirst
	pod.SchedulerName = corev1.DefaultSchedulerName
	pod.TerminationGracePeriodSeconds = ptr.To[int64](30)
	pod.SecurityContext = &corev1.PodSecurityContext{}
	for i := range pod.Containers {
		pod.Containers[i].ImagePullPolicy = corev1.PullIfNotPresent
		pod.Containers[i].TerminationMessagePath = corev1.TerminationMessagePathDefault
		pod.Containers[i].TerminationMessagePolicy = corev1.TerminationMessageReadFile
		for j := range pod.Containers[i].Ports {
			pod.Containers[i].Ports[j].Protocol = corev1.ProtocolTCP
		}
	}
}

func applied(t *testing.T, sts appsv1.StatefulSet) appsv1.StatefulSet {
	t.Helper()
	require.NoError(t, recordAppliedSpec(&sts))
	return sts
}

func TestNeedUpdate(t *testing.T) {
	expected := applied(t, newTestStatefulSet("redis-sentinel:v7"))

	tests := []struct {
		name         string
		expected     appsv1.StatefulSet
		existed      func() appsv1.StatefulSet
		wantMetadata bool
		wantSpec     bool
	}{
		{
			name:     "unchanged, with the fields the API server defaults",
			expected: expected,
			existed: func() appsv1.StatefulSet {
				sts := *expected.DeepCopy()
				withAPIDefaults(&sts)
				return sts
			},
		},
		{
			name:     "an environment variable Reloader added is kept",
			expected: expected,
			existed: func() appsv1.StatefulSet {
				sts := *expected.DeepCopy()
				withAPIDefaults(&sts)
				c := &sts.Spec.Template.Spec.Containers[0]
				c.Env = append(c.Env, corev1.EnvVar{Name: "STAKATER_REDIS_TLS_CERT_SECRET", Value: "c0ffee"})
				return sts
			},
		},
		{
			name:     "a pod template annotation added by Reloader or kubectl rollout restart is kept",
			expected: expected,
			existed: func() appsv1.StatefulSet {
				sts := *expected.DeepCopy()
				withAPIDefaults(&sts)
				sts.Spec.Template.Annotations = map[string]string{
					reloaderAnnotation:                  `{"type":"SECRET","name":"redis-tls-cert"}`,
					"kubectl.kubernetes.io/restartedAt": "2026-11-27T15:00:00Z",
				}
				return sts
			},
		},
		{
			name:     "an annotation someone else added to the StatefulSet is kept",
			expected: expected,
			existed: func() appsv1.StatefulSet {
				sts := *expected.DeepCopy()
				sts.Annotations["owner"] = "team"
				return sts
			},
		},
		{
			name:     "a field the operator sets, edited by someone else, is restored",
			expected: expected,
			existed: func() appsv1.StatefulSet {
				sts := *expected.DeepCopy()
				withAPIDefaults(&sts)
				sts.Spec.Template.Spec.Containers[0].Image = "redis-sentinel:edited"
				return sts
			},
			wantSpec: true,
		},
		{
			name:     "the operator wants another spec",
			expected: applied(t, newTestStatefulSet("redis-sentinel:v8")),
			existed: func() appsv1.StatefulSet {
				sts := *expected.DeepCopy()
				withAPIDefaults(&sts)
				return sts
			},
			wantSpec: true,
		},
		{
			// A StatefulSet created before the spec was recorded: the first
			// update records it.
			name:     "no recorded spec",
			expected: expected,
			existed: func() appsv1.StatefulSet {
				sts := *expected.DeepCopy()
				delete(sts.Annotations, patch.LastAppliedConfig)
				return sts
			},
			wantSpec: true,
		},
		{
			name:     "a missing annotation",
			expected: expected,
			existed: func() appsv1.StatefulSet {
				sts := *expected.DeepCopy()
				delete(sts.Annotations, "redis.opstreelabs.in")
				return sts
			},
			wantMetadata: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			existed := tt.existed()
			metadataChanged, specChanged, err := needUpdate(&tt.expected, &existed)
			require.NoError(t, err)
			assert.Equal(t, tt.wantMetadata, metadataChanged, "metadata")
			assert.Equal(t, tt.wantSpec, specChanged, "spec")
		})
	}
}

func TestUpdate(t *testing.T) {
	expected := applied(t, newTestStatefulSet("redis-sentinel:v8"))
	reloaded := func() *appsv1.StatefulSet {
		sts := applied(t, newTestStatefulSet("redis-sentinel:v7"))
		sts.Annotations["owner"] = "team"
		sts.Spec.Template.Annotations = map[string]string{reloaderAnnotation: "secret"}
		return &sts
	}

	t.Run("metadata only keeps the live spec", func(t *testing.T) {
		existed := reloaded()
		update(&expected, existed, false)
		assert.Equal(t, "redis-sentinel:v7", existed.Spec.Template.Spec.Containers[0].Image)
		assert.Contains(t, existed.Spec.Template.Annotations, reloaderAnnotation)
		assert.Equal(t, "team", existed.Annotations["owner"])
	})

	t.Run("replacing the spec records it", func(t *testing.T) {
		existed := reloaded()
		update(&expected, existed, true)
		assert.Equal(t, expected.Spec, existed.Spec)
		assert.Equal(t, expected.Annotations[patch.LastAppliedConfig], existed.Annotations[patch.LastAppliedConfig])
		assert.Equal(t, "team", existed.Annotations["owner"])
	})
}

// TestReconcileKeepsTheChangesOfOthers plays the Reloader scenario against a
// fake API server: the operator must not revert the pod template annotation
// Reloader sets, nor update the StatefulSet because of it.
func TestReconcileKeepsTheChangesOfOthers(t *testing.T) {
	ctx := context.Background()
	// Any namespaced kind registered in the client-go scheme can stand in for
	// the RedisReplication: only its UID decides control.
	owner := &corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: "redis-replication", Namespace: "default", UID: "owner-uid"}}
	cl := fake.NewClientBuilder().WithScheme(scheme.Scheme).Build()
	key := types.NamespacedName{Namespace: "default", Name: "redis-replication-s"}

	_, err := Reconcile(ctx, cl, newTestStatefulSet("redis-sentinel:v7"), owner)
	require.NoError(t, err)
	var live appsv1.StatefulSet
	require.NoError(t, cl.Get(ctx, key, &live))
	require.Contains(t, live.Annotations, patch.LastAppliedConfig, "creating the StatefulSet records its spec")

	// Reloader restarts the pods after the TLS secret was reissued.
	live.Spec.Template.Annotations = map[string]string{reloaderAnnotation: `{"type":"SECRET","name":"redis-tls-cert"}`}
	require.NoError(t, cl.Update(ctx, &live))
	reloadedVersion := live.ResourceVersion

	_, err = Reconcile(ctx, cl, newTestStatefulSet("redis-sentinel:v7"), owner)
	require.NoError(t, err)
	require.NoError(t, cl.Get(ctx, key, &live))
	assert.Contains(t, live.Spec.Template.Annotations, reloaderAnnotation, "the restart Reloader started is not reverted")
	assert.Equal(t, reloadedVersion, live.ResourceVersion, "nothing to update")

	// An upgrade of the operator changes the spec: it is applied as a whole.
	_, err = Reconcile(ctx, cl, newTestStatefulSet("redis-sentinel:v8"), owner)
	require.NoError(t, err)
	require.NoError(t, cl.Get(ctx, key, &live))
	assert.Equal(t, "redis-sentinel:v8", live.Spec.Template.Spec.Containers[0].Image)
	assert.NotContains(t, live.Spec.Template.Annotations, reloaderAnnotation)
}
