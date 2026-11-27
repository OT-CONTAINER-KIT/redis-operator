package podtemplate

import (
	"testing"

	"github.com/stretchr/testify/assert"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func owner(generation int64, labels, annotations map[string]string) *metav1.ObjectMeta {
	return &metav1.ObjectMeta{Generation: generation, Labels: labels, Annotations: annotations}
}

func TestInputsKey(t *testing.T) {
	base := InputsKey(owner(1, map[string]string{"a": "1", "b": "2"}, map[string]string{"x": "y"}))

	assert.Equal(t, base, InputsKey(owner(1, map[string]string{"b": "2", "a": "1"}, map[string]string{"x": "y"})),
		"the key must not depend on map order")

	changed := map[string]string{
		"generation":       InputsKey(owner(2, map[string]string{"a": "1", "b": "2"}, map[string]string{"x": "y"})),
		"label":            InputsKey(owner(1, map[string]string{"a": "1", "b": "3"}, map[string]string{"x": "y"})),
		"annotation":       InputsKey(owner(1, map[string]string{"a": "1", "b": "2"}, map[string]string{"x": "z"})),
		"apply annotation": InputsKey(owner(1, map[string]string{"a": "1", "b": "2"}, map[string]string{"x": "y", AnnotationApply: "1"})),
		"extra input":      InputsKey(owner(1, map[string]string{"a": "1", "b": "2"}, map[string]string{"x": "y"}), "port=30001"),
		"label moved to annotation": InputsKey(owner(1, map[string]string{"a": "1"},
			map[string]string{"x": "y", "b": "2"})),
	}
	for name, key := range changed {
		assert.NotEqual(t, base, key, name)
	}
}

func template(image string) corev1.PodTemplateSpec {
	return corev1.PodTemplateSpec{
		ObjectMeta: metav1.ObjectMeta{Labels: map[string]string{"app": "redis"}},
		Spec: corev1.PodSpec{
			InitContainers: []corev1.Container{{Name: "init-config", Image: image}},
			Containers:     []corev1.Container{{Name: "redis", Image: "redis:7"}},
		},
	}
}

func sts(tmpl corev1.PodTemplateSpec, annotations map[string]string) *appsv1.StatefulSet {
	return &appsv1.StatefulSet{
		ObjectMeta: metav1.ObjectMeta{Name: "redis", Annotations: annotations},
		Spec:       appsv1.StatefulSetSpec{Template: tmpl},
	}
}

func TestReconcile(t *testing.T) {
	oldTmpl := template("operator:v1")
	newTmpl := template("operator:v2")
	oldHash := Hash(&oldTmpl)
	newHash := Hash(&newTmpl)
	key := InputsKey(owner(1, nil, nil))
	otherKey := InputsKey(owner(2, nil, nil))

	tests := []struct {
		name         string
		stored       *appsv1.StatefulSet
		applied      *corev1.PodTemplateSpec
		wantImage    string
		wantResult   Result
		wantHash     string
		wantPending  string
		wantNoStatus bool
	}{
		{
			name:        "operator changed the template, resource unchanged: keep it",
			stored:      sts(oldTmpl, map[string]string{AnnotationInputs: key, AnnotationHash: oldHash}),
			applied:     &oldTmpl,
			wantImage:   "operator:v1",
			wantResult:  Result{Deferred: true, Pending: true, PendingKnown: true},
			wantHash:    oldHash,
			wantPending: "true",
		},
		{
			name:        "resource changed: apply the generated template",
			stored:      sts(oldTmpl, map[string]string{AnnotationInputs: otherKey, AnnotationHash: oldHash}),
			applied:     &oldTmpl,
			wantImage:   "operator:v2",
			wantResult:  Result{PendingKnown: true},
			wantHash:    newHash,
			wantPending: "false",
		},
		{
			name:        "nothing changed",
			stored:      sts(newTmpl, map[string]string{AnnotationInputs: key, AnnotationHash: newHash}),
			applied:     &newTmpl,
			wantImage:   "operator:v2",
			wantResult:  Result{Deferred: true, PendingKnown: true},
			wantHash:    newHash,
			wantPending: "false",
		},
		{
			name:        "adopt a StatefulSet written before the gate was enabled",
			stored:      sts(oldTmpl, nil),
			applied:     &oldTmpl,
			wantImage:   "operator:v1",
			wantResult:  Result{Deferred: true, Pending: true, PendingKnown: true},
			wantHash:    oldHash,
			wantPending: "true",
		},
		{
			name:         "adopt without a copy of the applied template: keep stored, pending unknown",
			stored:       sts(oldTmpl, nil),
			wantImage:    "operator:v1",
			wantResult:   Result{Deferred: true},
			wantNoStatus: true,
		},
		{
			name:        "no copy of the applied template but a recorded hash",
			stored:      sts(oldTmpl, map[string]string{AnnotationInputs: key, AnnotationHash: oldHash}),
			wantImage:   "operator:v1",
			wantResult:  Result{Deferred: true, Pending: true, PendingKnown: true},
			wantHash:    oldHash,
			wantPending: "true",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			desired := sts(newTmpl, map[string]string{"keep": "me"})
			desired.Spec.Template = *newTmpl.DeepCopy()
			// Simulate SetCreated having run before, as the embedded Sentinel path does.
			if tt.wantNoStatus {
				SetCreated(desired, key)
			}

			got := Reconcile(desired, tt.stored, tt.applied, key)

			assert.Equal(t, tt.wantResult, got)
			assert.Equal(t, tt.wantImage, desired.Spec.Template.Spec.InitContainers[0].Image)
			assert.Equal(t, key, desired.Annotations[AnnotationInputs])
			assert.Equal(t, "me", desired.Annotations["keep"])
			if tt.wantNoStatus {
				assert.NotContains(t, desired.Annotations, AnnotationHash)
				assert.NotContains(t, desired.Annotations, AnnotationPending)
				return
			}
			assert.Equal(t, tt.wantHash, desired.Annotations[AnnotationHash])
			assert.Equal(t, tt.wantPending, desired.Annotations[AnnotationPending])
		})
	}
}

func TestSetCreated(t *testing.T) {
	tmpl := template("operator:v1")
	desired := sts(tmpl, nil)
	SetCreated(desired, "k")
	assert.Equal(t, map[string]string{
		AnnotationInputs:  "k",
		AnnotationHash:    Hash(&tmpl),
		AnnotationPending: "false",
	}, desired.Annotations)
}
