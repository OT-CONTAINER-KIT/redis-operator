package k8sutils

import (
	"context"
	"fmt"
	"testing"

	"github.com/OT-CONTAINER-KIT/redis-operator/internal/controller/common/podtemplate"
	"github.com/OT-CONTAINER-KIT/redis-operator/internal/features"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	appsv1 "k8s.io/api/apps/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	k8sClientFake "k8s.io/client-go/kubernetes/fake"
	"k8s.io/utils/ptr"
)

func setDeferPodTemplateUpdates(t *testing.T, enabled bool) {
	t.Helper()
	original := features.Enabled(features.DeferPodTemplateUpdates)
	require.NoError(t, features.MutableFeatureGate.Set(fmt.Sprintf("DeferPodTemplateUpdates=%t", enabled)))
	t.Cleanup(func() {
		require.NoError(t, features.MutableFeatureGate.Set(fmt.Sprintf("DeferPodTemplateUpdates=%t", original)))
	})
}

func countStatefulSetUpdates(client *k8sClientFake.Clientset) int {
	n := 0
	for _, a := range client.Actions() {
		if a.GetVerb() == "update" && a.GetResource().Resource == "statefulsets" {
			n++
		}
	}
	return n
}

// TestCreateOrUpdateStateFulDeferPodTemplateUpdates simulates an operator
// upgrade by rendering a different init container image for an unchanged
// custom resource (same PodTemplateInputs).
func TestCreateOrUpdateStateFulDeferPodTemplateUpdates(t *testing.T) {
	objMeta := metav1.ObjectMeta{Name: "redis", Namespace: "ns"}
	ownerDef := metav1.OwnerReference{Name: "redis", Kind: "Redis", APIVersion: "redis.redis.opstreelabs.in/v1beta2", UID: "1"}
	containerParams := containerParameters{Image: "redis:7"}
	initParams := func(image string) initContainerParameters {
		return initContainerParameters{Enabled: ptr.To(true), Image: image}
	}
	params := func(inputs string, replicas int32) statefulSetParameters {
		return statefulSetParameters{Replicas: ptr.To(replicas), PodTemplateInputs: inputs}
	}
	apply := func(t *testing.T, client *k8sClientFake.Clientset, p statefulSetParameters, image string) *appsv1.StatefulSet {
		t.Helper()
		require.NoError(t, CreateOrUpdateStateFul(context.TODO(), client, objMeta.Namespace, objMeta, p, ownerDef, initParams(image), containerParams, nil))
		sts, err := client.AppsV1().StatefulSets(objMeta.Namespace).Get(context.TODO(), objMeta.Name, metav1.GetOptions{})
		require.NoError(t, err)
		return sts
	}
	initImage := func(sts *appsv1.StatefulSet) string {
		return sts.Spec.Template.Spec.InitContainers[0].Image
	}

	t.Run("operator change is deferred until the resource changes", func(t *testing.T) {
		setDeferPodTemplateUpdates(t, true)
		client := k8sClientFake.NewSimpleClientset()

		sts := apply(t, client, params("gen1", 1), "operator:v1")
		assert.Equal(t, "gen1", sts.Annotations[podtemplate.AnnotationInputs])
		assert.Equal(t, "false", sts.Annotations[podtemplate.AnnotationPending])

		// New operator version, same resource: the template is kept.
		sts = apply(t, client, params("gen1", 1), "operator:v2")
		assert.Equal(t, "operator:v1", initImage(sts))
		assert.Equal(t, "true", sts.Annotations[podtemplate.AnnotationPending])

		// Further reconciles do not update the StatefulSet again.
		updates := countStatefulSetUpdates(client)
		sts = apply(t, client, params("gen1", 1), "operator:v2")
		assert.Equal(t, updates, countStatefulSetUpdates(client), "a deferred template must not cause an update on every reconcile")
		assert.Equal(t, "operator:v1", initImage(sts))

		// The resource changed: the generated template is applied.
		sts = apply(t, client, params("gen2", 2), "operator:v2")
		assert.Equal(t, "operator:v2", initImage(sts))
		assert.Equal(t, int32(2), *sts.Spec.Replicas)
		assert.Equal(t, "gen2", sts.Annotations[podtemplate.AnnotationInputs])
		assert.Equal(t, "false", sts.Annotations[podtemplate.AnnotationPending])
	})

	t.Run("fields outside the pod template are still updated", func(t *testing.T) {
		setDeferPodTemplateUpdates(t, true)
		client := k8sClientFake.NewSimpleClientset()

		apply(t, client, params("gen1", 1), "operator:v1")
		p := params("gen1", 1)
		p.MinReadySeconds = 10
		sts := apply(t, client, p, "operator:v2")
		assert.Equal(t, "operator:v1", initImage(sts))
		assert.Equal(t, int32(10), sts.Spec.MinReadySeconds)
	})

	t.Run("StatefulSet written before the gate was enabled is adopted", func(t *testing.T) {
		setDeferPodTemplateUpdates(t, false)
		client := k8sClientFake.NewSimpleClientset()
		sts := apply(t, client, params("gen1", 1), "operator:v1")
		assert.NotContains(t, sts.Annotations, podtemplate.AnnotationInputs)

		setDeferPodTemplateUpdates(t, true)
		sts = apply(t, client, params("gen1", 1), "operator:v2")
		assert.Equal(t, "operator:v1", initImage(sts))
		assert.Equal(t, "gen1", sts.Annotations[podtemplate.AnnotationInputs])
		assert.Equal(t, "true", sts.Annotations[podtemplate.AnnotationPending])
	})

	t.Run("gate disabled keeps the current behaviour", func(t *testing.T) {
		setDeferPodTemplateUpdates(t, false)
		client := k8sClientFake.NewSimpleClientset()
		apply(t, client, params("gen1", 1), "operator:v1")
		sts := apply(t, client, params("gen1", 1), "operator:v2")
		assert.Equal(t, "operator:v2", initImage(sts))
	})
}

func TestApplyPodTemplateAnnotationIsNotPropagated(t *testing.T) {
	meta := metav1.ObjectMeta{Name: "redis", Annotations: map[string]string{
		podtemplate.AnnotationApply: "2026-11-26",
		"other":                     "value",
	}}
	anots := generateStatefulSetsAnots(meta, nil)
	assert.NotContains(t, anots, podtemplate.AnnotationApply)
	assert.Equal(t, "value", anots["other"])
}
