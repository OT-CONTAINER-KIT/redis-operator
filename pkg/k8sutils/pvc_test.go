package k8sutils

import (
	"context"
	"fmt"
	"strconv"
	"testing"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes/fake"
	ktesting "k8s.io/client-go/testing"
)

// TestHandlePVCResizing_NoUpdateNeeded verifies that if the target template spec hasn't changed,
// HandlePVCResizing returns nil without updating any PVC.
func TestHandlePVCResizing_NoUpdateNeeded(t *testing.T) {
	ctx := context.Background()

	// Define a stored PVC spec with 5Gi capacity.
	quantity := resource.MustParse("5Gi")
	pvcSpec := corev1.PersistentVolumeClaimSpec{
		Resources: corev1.VolumeResourceRequirements{
			Requests: corev1.ResourceList{
				corev1.ResourceStorage: quantity,
			},
		},
	}

	// Create two VolumeClaimTemplates: one for node-conf and one for redis-data.
	storedTemplates := []corev1.PersistentVolumeClaim{
		{
			ObjectMeta: metav1.ObjectMeta{Name: "node-conf"},
			Spec:       pvcSpec,
		},
		{
			ObjectMeta: metav1.ObjectMeta{Name: "redis-data"},
			Spec:       pvcSpec,
		},
	}

	// Set annotation storageCapacity to 5Gi.
	annotations := map[string]string{
		"storageCapacity": strconv.FormatInt(quantity.Value(), 10),
	}

	storedStateful := &appsv1.StatefulSet{
		ObjectMeta: metav1.ObjectMeta{
			Name:        "redis",
			Namespace:   "default",
			Annotations: annotations,
		},
		Spec: appsv1.StatefulSetSpec{
			VolumeClaimTemplates: storedTemplates,
		},
	}

	// newStateful is identical to storedStateful (no spec change).
	newStateful := storedStateful.DeepCopy()

	cl := fake.NewSimpleClientset()

	err := HandlePVCResizing(ctx, storedStateful, newStateful, cl)
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}

	// Ensure no PVC update action occurred.
	actions := cl.Fake.Actions()
	for _, action := range actions {
		if action.GetVerb() == "update" {
			t.Errorf("Unexpected PVC update action: %#v", action)
		}
	}
}

// TestHandlePVCResizing_UpdatePVC verifies that when the desired capacity differs for the target template,
// the matching PVC is updated and the annotation is updated accordingly.
func TestHandlePVCResizing_UpdatePVC(t *testing.T) {
	ctx := context.Background()

	// Stored PVC spec with 5Gi and new spec with 10Gi for the target (redis-data).
	storedQuantity := resource.MustParse("5Gi")
	desiredQuantity := resource.MustParse("10Gi")
	storedPVCSpec := corev1.PersistentVolumeClaimSpec{
		Resources: corev1.VolumeResourceRequirements{
			Requests: corev1.ResourceList{
				corev1.ResourceStorage: storedQuantity,
			},
		},
	}
	newPVCSpec := corev1.PersistentVolumeClaimSpec{
		Resources: corev1.VolumeResourceRequirements{
			Requests: corev1.ResourceList{
				corev1.ResourceStorage: desiredQuantity,
			},
		},
	}

	// Define two VolumeClaimTemplates.
	storedTemplates := []corev1.PersistentVolumeClaim{
		{
			ObjectMeta: metav1.ObjectMeta{Name: "node-conf"},
			Spec:       storedPVCSpec,
		},
		{
			ObjectMeta: metav1.ObjectMeta{Name: "redis-data"},
			Spec:       storedPVCSpec,
		},
	}

	annotations := map[string]string{
		"storageCapacity": strconv.FormatInt(storedQuantity.Value(), 10),
	}

	storedStateful := &appsv1.StatefulSet{
		ObjectMeta: metav1.ObjectMeta{
			Name:        "redis",
			Namespace:   "default",
			Annotations: annotations,
		},
		Spec: appsv1.StatefulSetSpec{
			VolumeClaimTemplates: storedTemplates,
		},
	}

	// newStateful: update the "redis-data" template spec to have 10Gi.
	newTemplates := []corev1.PersistentVolumeClaim{
		{
			ObjectMeta: metav1.ObjectMeta{Name: "node-conf"},
			Spec:       storedPVCSpec,
		},
		{
			ObjectMeta: metav1.ObjectMeta{Name: "redis-data"},
			Spec:       newPVCSpec,
		},
	}
	newStateful := storedStateful.DeepCopy()
	newStateful.Spec.VolumeClaimTemplates = newTemplates

	// Create a fake PVC corresponding to the redis-data template.
	// Its name is expected to start with "redis-data-".
	existingPVC := &corev1.PersistentVolumeClaim{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "redis-data-0",
			Namespace: "default",
			Labels: map[string]string{
				"app":                         "redis",
				"app.kubernetes.io/component": "middleware",
			},
		},
		Spec: storedPVCSpec,
	}
	cl := fake.NewSimpleClientset(existingPVC)

	err := HandlePVCResizing(ctx, storedStateful, newStateful, cl)
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}

	// Verify the annotation was updated to desired capacity.
	expectedAnnotation := strconv.FormatInt(desiredQuantity.Value(), 10)
	if storedStateful.Annotations["storageCapacity"] != expectedAnnotation {
		t.Errorf("Expected annotation storageCapacity to be %s, got %s", expectedAnnotation, storedStateful.Annotations["storageCapacity"])
	}

	// Verify the PVC was updated.
	updatedPVC, err := cl.CoreV1().PersistentVolumeClaims("default").Get(ctx, "redis-data-0", metav1.GetOptions{})
	if err != nil {
		t.Fatalf("Failed to get PVC: %v", err)
	}
	updatedCapacity := updatedPVC.Spec.Resources.Requests.Storage().Value()
	if updatedCapacity != desiredQuantity.Value() {
		t.Errorf("Expected PVC capacity to be %d, got %d", desiredQuantity.Value(), updatedCapacity)
	}
}

// TestHandlePVCResizing_UpdateFailure simulates a failure during PVC update and verifies that an error is returned.
func TestHandlePVCResizing_UpdateFailure(t *testing.T) {
	ctx := context.Background()

	// Stored PVC spec with 5Gi and new spec with 10Gi for the target (redis-data).
	storedQuantity := resource.MustParse("5Gi")
	desiredQuantity := resource.MustParse("10Gi")
	storedPVCSpec := corev1.PersistentVolumeClaimSpec{
		Resources: corev1.VolumeResourceRequirements{
			Requests: corev1.ResourceList{
				corev1.ResourceStorage: storedQuantity,
			},
		},
	}
	newPVCSpec := corev1.PersistentVolumeClaimSpec{
		Resources: corev1.VolumeResourceRequirements{
			Requests: corev1.ResourceList{
				corev1.ResourceStorage: desiredQuantity,
			},
		},
	}

	// Define two VolumeClaimTemplates.
	storedTemplates := []corev1.PersistentVolumeClaim{
		{
			ObjectMeta: metav1.ObjectMeta{Name: "node-conf"},
			Spec:       storedPVCSpec,
		},
		{
			ObjectMeta: metav1.ObjectMeta{Name: "redis-data"},
			Spec:       storedPVCSpec,
		},
	}

	annotations := map[string]string{
		"storageCapacity": strconv.FormatInt(storedQuantity.Value(), 10),
	}

	storedStateful := &appsv1.StatefulSet{
		ObjectMeta: metav1.ObjectMeta{
			Name:        "redis",
			Namespace:   "default",
			Annotations: annotations,
		},
		Spec: appsv1.StatefulSetSpec{
			VolumeClaimTemplates: storedTemplates,
		},
	}

	// newStateful: update the "redis-data" template spec to have 10Gi.
	newTemplates := []corev1.PersistentVolumeClaim{
		{
			ObjectMeta: metav1.ObjectMeta{Name: "node-conf"},
			Spec:       storedPVCSpec,
		},
		{
			ObjectMeta: metav1.ObjectMeta{Name: "redis-data"},
			Spec:       newPVCSpec,
		},
	}
	newStateful := storedStateful.DeepCopy()
	newStateful.Spec.VolumeClaimTemplates = newTemplates

	// Create a fake PVC corresponding to the redis-data template.
	existingPVC := &corev1.PersistentVolumeClaim{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "redis-data-0",
			Namespace: "default",
			Labels: map[string]string{
				"app":                         "redis",
				"app.kubernetes.io/component": "middleware",
			},
		},
		Spec: storedPVCSpec,
	}

	cl := fake.NewSimpleClientset(existingPVC)
	// Prepend a reactor to simulate an update failure.
	cl.PrependReactor("update", "persistentvolumeclaims", func(action ktesting.Action) (bool, runtime.Object, error) {
		return true, nil, fmt.Errorf("simulated update error")
	})

	err := HandlePVCResizing(ctx, storedStateful, newStateful, cl)
	if err == nil {
		t.Fatalf("Expected error due to simulated update failure, got nil")
	}
}
