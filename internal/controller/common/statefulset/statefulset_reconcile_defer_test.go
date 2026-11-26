package statefulset

import (
	"context"
	"fmt"
	"testing"

	"github.com/OT-CONTAINER-KIT/redis-operator/internal/controller/common/podtemplate"
	"github.com/OT-CONTAINER-KIT/redis-operator/internal/features"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

func TestReconcileDeferPodTemplateUpdates(t *testing.T) {
	original := features.Enabled(features.DeferPodTemplateUpdates)
	require.NoError(t, features.MutableFeatureGate.Set("DeferPodTemplateUpdates=true"))
	t.Cleanup(func() {
		require.NoError(t, features.MutableFeatureGate.Set(fmt.Sprintf("DeferPodTemplateUpdates=%t", original)))
	})

	ctx := context.Background()
	cl := fake.NewClientBuilder().WithScheme(scheme.Scheme).Build()
	// Any object works as the owner; only its metadata feeds the inputs key.
	owner := &corev1.ConfigMap{ObjectMeta: metav1.ObjectMeta{Name: "rr", Namespace: "ns", UID: "1", Generation: 1}}

	expected := func(image string, replicas int32) appsv1.StatefulSet {
		return New(Params{
			Name:      "rr-sentinel",
			Namespace: "ns",
			Replicas:  replicas,
			PodTemplateSpec: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{Labels: map[string]string{"app": "rr-sentinel"}},
				Spec:       corev1.PodSpec{Containers: []corev1.Container{{Name: "sentinel", Image: image}}},
			},
		})
	}
	get := func() appsv1.StatefulSet {
		var sts appsv1.StatefulSet
		require.NoError(t, cl.Get(ctx, types.NamespacedName{Namespace: "ns", Name: "rr-sentinel"}, &sts))
		return sts
	}

	_, err := Reconcile(ctx, cl, expected("sentinel:v1", 3), owner)
	require.NoError(t, err)
	assert.Equal(t, "false", get().Annotations[podtemplate.AnnotationPending])

	// New operator version renders a different template for the same owner.
	_, err = Reconcile(ctx, cl, expected("sentinel:v2", 3), owner)
	require.NoError(t, err)
	sts := get()
	assert.Equal(t, "sentinel:v1", sts.Spec.Template.Spec.Containers[0].Image)
	assert.Equal(t, "true", sts.Annotations[podtemplate.AnnotationPending])

	// The owner changed: the generated template is applied.
	owner.Generation = 2
	_, err = Reconcile(ctx, cl, expected("sentinel:v2", 5), owner)
	require.NoError(t, err)
	sts = get()
	assert.Equal(t, "sentinel:v2", sts.Spec.Template.Spec.Containers[0].Image)
	assert.Equal(t, int32(5), *sts.Spec.Replicas)
	assert.Equal(t, "false", sts.Annotations[podtemplate.AnnotationPending])
}
