package k8sutils

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/equality"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/client-go/kubernetes"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

// HandlePVCResizing checks and updates the PVC storage capacity for the target VolumeClaimTemplate.
// It filters out any template with name "node-conf", and only processes the first template
// that does not have that name. If the storage capacity defined in the new StatefulSet's target template
// (i.e. for Redis RDB data) differs from the actual PVC capacity, it attempts to update the PVC
// and adjusts the annotation accordingly. Returns an error if any update fails.
func HandlePVCResizing(ctx context.Context, storedStateful, newStateful *appsv1.StatefulSet, cl kubernetes.Interface) error {
	// Find the target VolumeClaimTemplate that is not "node-conf".
	targetIndex := -1
	for i, tmpl := range newStateful.Spec.VolumeClaimTemplates {
		if tmpl.Name != "node-conf" {
			targetIndex = i
			break
		}
	}
	// If no target template is found, nothing to do.
	if targetIndex == -1 {
		return nil
	}

	// Find the corresponding stored template by matching the name.
	var storedTemplate *corev1.PersistentVolumeClaim
	for i, tmpl := range storedStateful.Spec.VolumeClaimTemplates {
		if tmpl.Name == newStateful.Spec.VolumeClaimTemplates[targetIndex].Name {
			storedTemplate = &storedStateful.Spec.VolumeClaimTemplates[i]
			break
		}
	}
	if storedTemplate == nil {
		return fmt.Errorf("matching stored VolumeClaimTemplate not found for template %s", newStateful.Spec.VolumeClaimTemplates[targetIndex].Name)
	}

	// If the target VolumeClaimTemplate Spec hasn't changed, no update is required.
	if equality.Semantic.DeepEqual(newStateful.Spec.VolumeClaimTemplates[targetIndex].Spec, storedTemplate.Spec) {
		return nil
	}

	// Retrieve or initialize the storage capacity recorded in annotations.
	annotations := storedStateful.Annotations
	if annotations == nil {
		annotations = map[string]string{"storageCapacity": "0"}
	}

	storedCapacity, _ := strconv.ParseInt(annotations["storageCapacity"], 10, 64)
	desiredCapacity := newStateful.Spec.VolumeClaimTemplates[targetIndex].Spec.Resources.Requests.Storage().Value()

	// If the stored capacity matches the desired capacity, no update is needed.
	if storedCapacity == desiredCapacity {
		// TODO: Handle scale-out scenario:
		// When scaling out, new PVCs are created with the original capacity from the VolumeClaimTemplate.
		// Since the annotation is not cleared, the current logic might incorrectly assume that the desired capacity is already applied.
		// In a future update, we should detect new PVCs and update them accordingly.
		return nil
	}

	// Create a label selector to list all related PVCs.
	labelSelector := labels.FormatLabels(map[string]string{
		"app": storedStateful.Name,
	})
	listOpt := metav1.ListOptions{LabelSelector: labelSelector}

	pvcs, err := cl.CoreV1().PersistentVolumeClaims(storedStateful.Namespace).List(context.Background(), listOpt)
	if err != nil {
		return err
	}

	updateFailed := false

	// Determine the prefix for PVC names based on the target template name.
	// PVC names are typically formatted as "<templateName>-<podName>".
	targetTemplateName := newStateful.Spec.VolumeClaimTemplates[targetIndex].Name
	pvcPrefix := targetTemplateName + "-"

	// Iterate over PVCs and update those that match the target template.
	for i := range pvcs.Items {
		pvc := &pvcs.Items[i]
		if !strings.HasPrefix(pvc.Name, pvcPrefix) {
			continue // Skip PVCs that do not belong to the target template.
		}
		currentCapacity := pvc.Spec.Resources.Requests.Storage().Value()
		if currentCapacity != desiredCapacity {
			pvc.Spec.Resources.Requests = newStateful.Spec.VolumeClaimTemplates[targetIndex].Spec.Resources.Requests
			if _, err := cl.CoreV1().PersistentVolumeClaims(storedStateful.Namespace).Update(context.Background(), pvc, metav1.UpdateOptions{}); err != nil {
				updateFailed = true
				log.FromContext(ctx).Error(fmt.Errorf("sts:%s resize pvc [%s] failed: %s", storedStateful.Name, pvc.Name, err.Error()), "")
			}
			log.FromContext(ctx).Info(fmt.Sprintf("sts:%s resized pvc [%s] from %d to %d", storedStateful.Name, pvc.Name, currentCapacity, desiredCapacity))
		}
	}

	// If any update failed, return an error.
	if updateFailed {
		return fmt.Errorf("one or more PVC updates failed")
	}

	// Update the annotation with the new storage capacity unconditionally.
	annotations["storageCapacity"] = fmt.Sprintf("%d", desiredCapacity)
	storedStateful.Annotations = annotations
	log.FromContext(ctx).V(1).Info(fmt.Sprintf("sts:%s updated storageCapacity annotation to %d", storedStateful.Name, desiredCapacity))

	return nil
}
