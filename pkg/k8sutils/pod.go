package k8sutils

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

type Pod interface {
	ListPods(ctx context.Context, namespace string, labels map[string]string) (*corev1.PodList, error)
	PatchPodLabels(ctx context.Context, namespace, name string, labels map[string]string) error
}

type PodService struct {
	kubeClient kubernetes.Interface
}

func NewPodService(kubeClient kubernetes.Interface) *PodService {
	return &PodService{
		kubeClient: kubeClient,
	}
}

func (s *PodService) ListPods(ctx context.Context, namespace string, labels map[string]string) (*corev1.PodList, error) {
	selector := make([]string, 0, len(labels))
	for key, value := range labels {
		selector = append(selector, fmt.Sprintf("%s=%s", key, value))
	}
	return s.kubeClient.CoreV1().Pods(namespace).List(ctx, metav1.ListOptions{
		LabelSelector: strings.Join(selector, ","),
	})
}

type patchStringValue struct {
	Op    string      `json:"op"`
	Path  string      `json:"path"`
	Value interface{} `json:"value"`
}

func (s *PodService) PatchPodLabels(ctx context.Context, namespace, podName string, labels map[string]string) error {
	log.FromContext(ctx).V(1).Info("Patch pod labels", "namespace", namespace, "podName", podName, "labels", labels)

	var payloads []interface{}
	for labelKey, labelValue := range labels {
		payload := patchStringValue{
			Op:    "replace",
			Path:  "/metadata/labels/" + labelKey,
			Value: labelValue,
		}
		payloads = append(payloads, payload)
	}
	payloadBytes, _ := json.Marshal(payloads)

	_, err := s.kubeClient.CoreV1().Pods(namespace).Patch(ctx, podName, types.JSONPatchType, payloadBytes, metav1.PatchOptions{})
	if err != nil {
		log.FromContext(ctx).Error(err, "Patch pod labels failed", "namespace", namespace, "podName", podName)
	}
	return err
}
