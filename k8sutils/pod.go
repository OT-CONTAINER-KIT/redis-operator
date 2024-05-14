package k8sutils

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/go-logr/logr"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes"
)

type Pod interface {
	ListPods(ctx context.Context, namespace string, labels map[string]string) (*corev1.PodList, error)
	PatchPodLabels(ctx context.Context, namespace, name string, labels map[string]string) error
}

type PodService struct {
	kubeClient kubernetes.Interface
	log        logr.Logger
}

func NewPodService(kubeClient kubernetes.Interface, log logr.Logger) *PodService {
	log = log.WithValues("service", "k8s.pod")
	return &PodService{
		kubeClient: kubeClient,
		log:        log,
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
	s.log.Info("Patch pod labels", "namespace", namespace, "podName", podName, "labels", labels)

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
		s.log.Error(err, "Patch pod labels failed", "namespace", namespace, "podName", podName)
	}
	return err
}
