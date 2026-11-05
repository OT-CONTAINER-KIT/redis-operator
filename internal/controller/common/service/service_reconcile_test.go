package service

import (
	"context"
	"testing"

	"github.com/banzaicloud/k8s-objectmatcher/patch"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/utils/ptr"
	clientfake "sigs.k8s.io/controller-runtime/pkg/client/fake"
)

func newExpected(t *testing.T) *corev1.Service {
	t.Helper()
	svc := &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{Name: "redis", Namespace: "default"},
		Spec: corev1.ServiceSpec{
			Selector: map[string]string{"app": "redis"},
			Ports:    []corev1.ServicePort{{Name: "redis", Port: 6379}},
		},
	}
	require.NoError(t, patch.DefaultAnnotator.SetLastAppliedAnnotation(svc))
	return svc
}

// asStored returns svc as the API server stores it, with its defaults filled in.
func asStored(svc *corev1.Service) *corev1.Service {
	stored := svc.DeepCopy()
	stored.Spec.Type = corev1.ServiceTypeClusterIP
	stored.Spec.ClusterIP = "10.96.0.10"
	stored.Spec.ClusterIPs = []string{"10.96.0.10"}
	stored.Spec.SessionAffinity = corev1.ServiceAffinityNone
	stored.Spec.IPFamilies = []corev1.IPFamily{corev1.IPv4Protocol}
	stored.Spec.IPFamilyPolicy = ptr.To(corev1.IPFamilyPolicySingleStack)
	stored.Spec.InternalTrafficPolicy = ptr.To(corev1.ServiceInternalTrafficPolicyCluster)
	for i := range stored.Spec.Ports {
		stored.Spec.Ports[i].Protocol = corev1.ProtocolTCP
		stored.Spec.Ports[i].TargetPort = intstr.FromInt32(stored.Spec.Ports[i].Port)
	}
	return stored
}

func TestNeedUpdate(t *testing.T) {
	t.Run("server defaults alone do not need an update", func(t *testing.T) {
		expected := newExpected(t)
		assert.False(t, needUpdate(expected, asStored(expected)))
	})

	t.Run("a changed port is detected", func(t *testing.T) {
		existed := asStored(newExpected(t))
		expected := newExpected(t)
		expected.Spec.Ports[0].Port = 6380
		require.NoError(t, patch.DefaultAnnotator.SetLastAppliedAnnotation(expected))
		assert.True(t, changed(expected, existed))
	})

	t.Run("a field edited on the live object needs an update", func(t *testing.T) {
		expected := newExpected(t)
		existed := asStored(expected)
		existed.Spec.Selector = map[string]string{"app": "other"}
		assert.True(t, needUpdate(expected, existed))
	})
}

func TestReconcile(t *testing.T) {
	ctx := context.Background()
	cli := clientfake.NewClientBuilder().WithScheme(scheme.Scheme).Build()
	expected := func() corev1.Service {
		svc := *newExpected(t)
		delete(svc.Annotations, patch.LastAppliedConfig)
		return svc
	}

	created, err := Reconcile(ctx, cli, expected(), nil)
	require.NoError(t, err)
	assert.Contains(t, created.Annotations, patch.LastAppliedConfig)

	again, err := Reconcile(ctx, cli, expected(), nil)
	require.NoError(t, err)
	assert.Equal(t, created.ResourceVersion, again.ResourceVersion)
}
