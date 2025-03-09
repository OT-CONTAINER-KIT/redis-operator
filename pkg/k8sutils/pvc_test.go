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
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes/fake"
	ktesting "k8s.io/client-go/testing"
)

// TestHandlePVCResizing_NoUpdateNeeded verifies that if the VolumeClaimTemplate spec hasn't changed,
// HandlePVCResizing returns nil without performing any update.
func TestHandlePVCResizing_NoUpdateNeeded(t *testing.T) {
	ctx := context.Background()

	// Create a dummy PVC spec (5Gi).
	quantity := resource.MustParse("5Gi")
	pvcSpec := corev1.PersistentVolumeClaimSpec{
		// After
		Resources: corev1.VolumeResourceRequirements{
			Requests: corev1.ResourceList{
				corev1.ResourceStorage: quantity,
			},
		},
	}

	// Both stored and new StatefulSets have identical VolumeClaimTemplate spec.
	storedStateful := &appsv1.StatefulSet{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "redis",
			Namespace: "default",
			Annotations: map[string]string{
				"storageCapacity": strconv.FormatInt(quantity.Value(), 10),
			},
		},
		Spec: appsv1.StatefulSetSpec{
			VolumeClaimTemplates: []corev1.PersistentVolumeClaim{
				{Spec: pvcSpec},
			},
		},
	}

	newStateful := storedStateful.DeepCopy()

	cl := fake.NewSimpleClientset()

	err := HandlePVCResizing(ctx, storedStateful, newStateful, cl)
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}
}

// TestHandlePVCResizing_UpdatePVC verifies that when the desired capacity differs from the stored capacity,
// the PVC is updated and the annotation is revised.
func TestHandlePVCResizing_UpdatePVC(t *testing.T) {
	ctx := context.Background()

	// Define stored PVC spec (5Gi) and new PVC spec (10Gi).
	quantity := resource.MustParse("5Gi")
	storedPVCSpec := corev1.PersistentVolumeClaimSpec{
		Resources: corev1.VolumeResourceRequirements{
			Requests: corev1.ResourceList{
				corev1.ResourceStorage: quantity,
			},
		},
	}
	newPVCSpec := corev1.PersistentVolumeClaimSpec{
		Resources: corev1.VolumeResourceRequirements{
			Requests: corev1.ResourceList{
				corev1.ResourceStorage: resource.MustParse("10Gi"),
			},
		},
	}

	storedStateful := &appsv1.StatefulSet{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "redis",
			Namespace: "default",
			Annotations: map[string]string{
				"storageCapacity": strconv.FormatInt(quantity.Value(), 10),
			},
		},
		Spec: appsv1.StatefulSetSpec{
			VolumeClaimTemplates: []corev1.PersistentVolumeClaim{
				{
					ObjectMeta: metav1.ObjectMeta{Name: "redis-data"},
					Spec:       storedPVCSpec,
				},
			},
		},
	}

	newStateful := storedStateful.DeepCopy()
	// Update the spec in newStateful to 10Gi.
	newStateful.Spec.VolumeClaimTemplates[0].Spec = newPVCSpec

	// Create a fake PVC representing an existing PVC with 5Gi capacity.
	existingPVC := &corev1.PersistentVolumeClaim{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "redis-data-0",
			Namespace: "default",
			Labels: map[string]string{
				"app":                         "redis",
				"app.kubernetes.io/component": "redis",
			},
		},
		Spec: corev1.PersistentVolumeClaimSpec{
			Resources: corev1.VolumeResourceRequirements{
				Requests: corev1.ResourceList{
					corev1.ResourceStorage: quantity,
				},
			},
		},
	}

	cl := fake.NewSimpleClientset(existingPVC)

	err := HandlePVCResizing(ctx, storedStateful, newStateful, cl)
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}

	// Verify the annotation was updated to the desired capacity.
	desiredCapacity := newStateful.Spec.VolumeClaimTemplates[0].Spec.Resources.Requests.Storage().Value()
	expectedAnnotation := fmt.Sprintf("%d", desiredCapacity)
	if storedStateful.Annotations["storageCapacity"] != expectedAnnotation {
		t.Errorf("Expected annotation storageCapacity to be %s, got %s", expectedAnnotation, storedStateful.Annotations["storageCapacity"])
	}

	// Verify the PVC was updated.
	pvcList, err := cl.CoreV1().PersistentVolumeClaims("default").List(ctx, metav1.ListOptions{
		LabelSelector: labels.FormatLabels(map[string]string{
			"app":                         "redis",
			"app.kubernetes.io/component": "redis",
		}),
	})
	if err != nil {
		t.Fatalf("Failed to list PVCs: %v", err)
	}
	if len(pvcList.Items) != 1 {
		t.Fatalf("Expected 1 PVC, got: %d", len(pvcList.Items))
	}

	updatedPVC := pvcList.Items[0]
	updatedCapacity := updatedPVC.Spec.Resources.Requests.Storage().Value()
	if updatedCapacity != desiredCapacity {
		t.Errorf("Expected PVC capacity to be %d, got %d", desiredCapacity, updatedCapacity)
	}
}

// TestHandlePVCResizing_UpdateFailure simulates a failure during PVC update and verifies that an error is returned.
func TestHandlePVCResizing_UpdateFailure(t *testing.T) {
	ctx := context.Background()

	quantity := resource.MustParse("5Gi")
	storedPVCSpec := corev1.PersistentVolumeClaimSpec{
		Resources: corev1.VolumeResourceRequirements{
			Requests: corev1.ResourceList{
				corev1.ResourceStorage: quantity,
			},
		},
	}
	newPVCSpec := corev1.PersistentVolumeClaimSpec{
		Resources: corev1.VolumeResourceRequirements{
			Requests: corev1.ResourceList{
				corev1.ResourceStorage: resource.MustParse("10Gi"),
			},
		},
	}

	storedStateful := &appsv1.StatefulSet{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "redis",
			Namespace: "default",
			Annotations: map[string]string{
				"storageCapacity": strconv.FormatInt(quantity.Value(), 10),
			},
		},
		Spec: appsv1.StatefulSetSpec{
			VolumeClaimTemplates: []corev1.PersistentVolumeClaim{
				{
					ObjectMeta: metav1.ObjectMeta{Name: "redis-data"},
					Spec:       storedPVCSpec,
				},
			},
		},
	}

	newStateful := storedStateful.DeepCopy()
	newStateful.Spec.VolumeClaimTemplates[0].Spec = newPVCSpec

	// Create a fake PVC representing an existing PVC with 5Gi capacity.
	existingPVC := &corev1.PersistentVolumeClaim{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "redis-data-0",
			Namespace: "default",
			Labels: map[string]string{
				"app":                         "redis",
				"app.kubernetes.io/component": "redis",
			},
		},
		Spec: corev1.PersistentVolumeClaimSpec{
			Resources: corev1.VolumeResourceRequirements{
				Requests: corev1.ResourceList{
					corev1.ResourceStorage: quantity,
				},
			},
		},
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
