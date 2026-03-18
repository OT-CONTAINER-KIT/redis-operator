package k8sutils

import (
	"context"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

// IsPodRunning checks whether a pod exists and is in Running phase.
// Returns false if the pod does not exist, is not Running, or if there's an error.
func IsPodRunning(ctx context.Context, client kubernetes.Interface, namespace, podName string) bool {
	if podName == "" {
		return false
	}
	pod, err := client.CoreV1().Pods(namespace).Get(ctx, podName, metav1.GetOptions{})
	if err != nil {
		log.FromContext(ctx).V(1).Info("Pod not found or error fetching pod", "podName", podName, "error", err)
		return false
	}
	return pod.Status.Phase == corev1.PodRunning
}
