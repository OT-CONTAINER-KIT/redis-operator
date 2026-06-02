package statefulset

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	appsv1 "k8s.io/api/apps/v1"
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
	require.NoError(t, appsv1.AddToScheme(s))
	require.NoError(t, corev1.AddToScheme(s))
	return s
}

func newExpected() appsv1.StatefulSet {
	return appsv1.StatefulSet{
		ObjectMeta: metav1.ObjectMeta{Name: "redis-s", Namespace: "redis"},
		Spec: appsv1.StatefulSetSpec{
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{Labels: map[string]string{"app": "redis-s"}},
			},
		},
	}
}

// TestReconcile_DoesNotPanicOnUpdateError reproduces GH #1753: the prior code
// dereferenced a nil interface when reconciler.Reconcile returned (nil, err)
// from an Update failure, crash-looping the operator.
func TestReconcile_DoesNotPanicOnUpdateError(t *testing.T) {
	scheme := newScheme(t)
	existing := newExpected()
	existing.Spec.ServiceName = "old-svc"

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
	expected.Spec.ServiceName = "new-svc" // triggers update path

	// Must not panic; must return an error and a zero-value StatefulSet.
	var out appsv1.StatefulSet
	var err error
	require.NotPanics(t, func() {
		out, err = Reconcile(context.Background(), cli, expected, nil)
	})
	assert.Error(t, err)
	assert.Equal(t, appsv1.StatefulSet{}, out)
}

// TestReconcile_HappyPath ensures the non-error path still returns the
// reconciled StatefulSet correctly.
func TestReconcile_HappyPath(t *testing.T) {
	scheme := newScheme(t)
	cli := fake.NewClientBuilder().WithScheme(scheme).Build()

	expected := newExpected()

	out, err := Reconcile(context.Background(), cli, expected, nil)
	require.NoError(t, err)
	assert.Equal(t, "redis-s", out.Name)
	assert.Equal(t, "redis", out.Namespace)
}
