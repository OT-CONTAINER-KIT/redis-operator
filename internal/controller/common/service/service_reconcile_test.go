package service

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/client/interceptor"
)

func newScheme(t *testing.T) *runtime.Scheme {
	t.Helper()
	s := runtime.NewScheme()
	require.NoError(t, corev1.AddToScheme(s))
	return s
}

func newExpected() corev1.Service {
	return corev1.Service{
		ObjectMeta: metav1.ObjectMeta{Name: "redis-master", Namespace: "redis"},
		Spec: corev1.ServiceSpec{
			Selector: map[string]string{"app": "redis", "redis-role": "master"},
			Ports:    []corev1.ServicePort{{Port: 6379}},
		},
	}
}

// TestReconcile_DoesNotPanicOnUpdateError mirrors the StatefulSet panic guard
// for Service reconcile path (same GH #1753 root cause).
func TestReconcile_DoesNotPanicOnUpdateError(t *testing.T) {
	scheme := newScheme(t)
	existing := newExpected()
	existing.Spec.Ports[0].Port = 9999 // forces update

	cli := fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(&existing).
		WithInterceptorFuncs(interceptor.Funcs{
			Update: func(ctx context.Context, c client.WithWatch, obj client.Object, opts ...client.UpdateOption) error {
				return errors.New("simulated update conflict")
			},
		}).
		Build()

	expected := newExpected()

	var out corev1.Service
	var err error
	require.NotPanics(t, func() {
		out, err = Reconcile(context.Background(), cli, expected, nil)
	})
	assert.Error(t, err)
	assert.Equal(t, corev1.Service{}, out)
}

func TestReconcile_HappyPath(t *testing.T) {
	scheme := newScheme(t)
	cli := fake.NewClientBuilder().WithScheme(scheme).Build()

	out, err := Reconcile(context.Background(), cli, newExpected(), nil)
	require.NoError(t, err)
	assert.Equal(t, "redis-master", out.Name)
	assert.Equal(t, "redis", out.Namespace)
}
