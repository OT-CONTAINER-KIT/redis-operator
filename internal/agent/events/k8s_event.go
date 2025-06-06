package events

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
)

const (
	RoleChangedReason    = "RoleChanged"
	InvolvedObjectPod    = "Pod"
	EventSourceComponent = "redis-role-detector"
)

// RoleChangeEventData represents the structured data for role change events
type RoleChangeEventData struct {
	PodName      string    `json:"podName"`
	Namespace    string    `json:"namespace"`
	PreviousRole string    `json:"previousRole"`
	CurrentRole  string    `json:"currentRole"`
	Timestamp    time.Time `json:"timestamp"`
}

// K8sEventSender sends Kubernetes events for role changes
type K8sEventSender struct {
	clientset *kubernetes.Clientset
	podName   string
	namespace string
}

// NewK8sEventSender creates a new Kubernetes event sender
func NewK8sEventSender() (*K8sEventSender, error) {
	config, err := rest.InClusterConfig()
	if err != nil {
		return nil, fmt.Errorf("failed to get in-cluster config: %w", err)
	}

	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		return nil, fmt.Errorf("failed to create clientset: %w", err)
	}

	podName := os.Getenv("POD_NAME")
	namespace := os.Getenv("POD_NAMESPACE")

	if podName == "" || namespace == "" {
		return nil, fmt.Errorf("POD_NAME and POD_NAMESPACE environment variables must be set")
	}

	return &K8sEventSender{
		clientset: clientset,
		podName:   podName,
		namespace: namespace,
	}, nil
}

// SendRoleChangeEvent sends a Kubernetes event for role change
func (s *K8sEventSender) SendRoleChangeEvent(ctx context.Context, previousRole, currentRole string) error {
	// Create structured event data
	eventData := RoleChangeEventData{
		PodName:      s.podName,
		Namespace:    s.namespace,
		PreviousRole: previousRole,
		CurrentRole:  currentRole,
		Timestamp:    time.Now().UTC(),
	}

	// Marshal to JSON for structured message
	messageBytes, err := json.Marshal(eventData)
	if err != nil {
		return fmt.Errorf("failed to marshal event data: %w", err)
	}

	event := &corev1.Event{
		ObjectMeta: metav1.ObjectMeta{
			GenerateName: "redis-role-change-",
			Namespace:    s.namespace,
		},
		InvolvedObject: corev1.ObjectReference{
			Kind:      InvolvedObjectPod,
			Name:      s.podName,
			Namespace: s.namespace,
		},
		Reason:  RoleChangedReason,
		Message: string(messageBytes),
		Type:    corev1.EventTypeNormal,
		Source: corev1.EventSource{
			Component: EventSourceComponent,
		},
		FirstTimestamp: metav1.Now(),
		LastTimestamp:  metav1.Now(),
		Count:          1,
	}

	_, err = s.clientset.CoreV1().Events(s.namespace).Create(ctx, event, metav1.CreateOptions{})
	if err != nil {
		return fmt.Errorf("failed to create event: %w", err)
	}

	return nil
}
