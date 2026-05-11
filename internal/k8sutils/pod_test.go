package k8sutils

import (
	"context"
	"testing"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	k8sClientFake "k8s.io/client-go/kubernetes/fake"
)

func TestIsPodRunning(t *testing.T) {
	fakeClient := k8sClientFake.NewSimpleClientset(
		&corev1.Pod{
			ObjectMeta: metav1.ObjectMeta{Name: "running-pod", Namespace: "default"},
			Status:     corev1.PodStatus{Phase: corev1.PodRunning},
		},
		&corev1.Pod{
			ObjectMeta: metav1.ObjectMeta{Name: "pending-pod", Namespace: "default"},
			Status:     corev1.PodStatus{Phase: corev1.PodPending},
		},
	)

	tests := []struct {
		name      string
		podName   string
		namespace string
		want      bool
	}{
		{"running pod", "running-pod", "default", true},
		{"pending pod", "pending-pod", "default", false},
		{"non-existent pod", "no-such-pod", "default", false},
		{"empty pod name", "", "default", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := IsPodRunning(context.Background(), fakeClient, tt.namespace, tt.podName)
			if got != tt.want {
				t.Errorf("IsPodRunning(%q) = %v, want %v", tt.podName, got, tt.want)
			}
		})
	}
}
