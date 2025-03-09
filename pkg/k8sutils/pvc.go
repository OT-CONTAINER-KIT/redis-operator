package k8sutils

import (
	"context"
	"fmt"
	"strconv"

	appsv1 "k8s.io/api/apps/v1"
	"k8s.io/apimachinery/pkg/api/equality"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/client-go/kubernetes"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

// HandlePVCResizing checks and updates the PVC storage capacity.
// If the storage capacity defined in the new StatefulSet's VolumeClaimTemplate differs from the actual PVC capacity,
// it attempts to update the PVC and adjusts the annotation accordingly.
// Returns an error if any update fails.
func HandlePVCResizing(ctx context.Context, storedStateful, newStateful *appsv1.StatefulSet, cl kubernetes.Interface) error {
	// If the VolumeClaimTemplate Spec hasn't changed, no update is required.
	if equality.Semantic.DeepEqual(newStateful.Spec.VolumeClaimTemplates[0].Spec, storedStateful.Spec.VolumeClaimTemplates[0].Spec) {
		return nil
	}

	// Retrieve or initialize the storage capacity recorded in annotations.
	annotations := storedStateful.Annotations
	if annotations == nil {
		annotations = map[string]string{"storageCapacity": "0"}
	}

	storedCapacity, _ := strconv.ParseInt(annotations["storageCapacity"], 0, 64)
	desiredCapacity := newStateful.Spec.VolumeClaimTemplates[0].Spec.Resources.Requests.Storage().Value()

	// If the stored capacity matches the desired capacity, no update is needed.
	if storedCapacity == desiredCapacity {
		return nil
	}

	// Create a label selector to list all related PVCs.
	labelSelector := labels.FormatLabels(map[string]string{
		"app":                         storedStateful.Name,
		"app.kubernetes.io/component": "redis",
	})
	listOpt := metav1.ListOptions{LabelSelector: labelSelector}

	pvcs, err := cl.CoreV1().PersistentVolumeClaims(storedStateful.Namespace).List(context.Background(), listOpt)
	if err != nil {
		return err
	}

	updateFailed := false
	realUpdate := false

	// Iterate over PVCs and update the resource requests if their capacity does not match the desired capacity.
	for i := range pvcs.Items {
		pvc := &pvcs.Items[i]
		currentCapacity := pvc.Spec.Resources.Requests.Storage().Value()
		if currentCapacity != desiredCapacity {
			realUpdate = true
			pvc.Spec.Resources.Requests = newStateful.Spec.VolumeClaimTemplates[0].Spec.Resources.Requests
			if _, err := cl.CoreV1().PersistentVolumeClaims(storedStateful.Namespace).Update(context.Background(), pvc, metav1.UpdateOptions{}); err != nil {
				updateFailed = true
				log.FromContext(ctx).Error(fmt.Errorf("redis:%s resize pvc failed: %s", storedStateful.Name, err.Error()), "")
			}
		}
	}

	// If any update failed, return an error.
	if updateFailed {
		return fmt.Errorf("one or more PVC updates failed")
	}

	// If updates were successful, update the annotation with the new storage capacity.
	if len(pvcs.Items) != 0 {
		annotations["storageCapacity"] = fmt.Sprintf("%d", desiredCapacity)
		storedStateful.Annotations = annotations
		if realUpdate {
			log.FromContext(ctx).V(1).Info(fmt.Sprintf("redis:%s resized pvc from %d to %d", storedStateful.Name, storedCapacity, desiredCapacity))
		} else {
			log.FromContext(ctx).V(1).Info(fmt.Sprintf("redis:%s updated annotations for storage capacity", storedStateful.Name))
		}
	}

	return nil
}
