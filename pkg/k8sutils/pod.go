package k8sutils

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/OT-CONTAINER-KIT/redis-operator/pkg/util"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/util/retry"
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

	if len(labels) == 0 {
		return fmt.Errorf("empty labels, nothing to patch")
	}

	var payloads []interface{}
	for k, v := range labels {
		payloads = append(payloads, patchStringValue{
			Op:    "add",
			Path:  "/metadata/labels/" + util.EscapeJSONPointer(k),
			Value: v,
		})
	}
	payloadBytes, err := json.Marshal(payloads)
	if err != nil {
		return fmt.Errorf("failed to marshal patch payload: %w", err)
	}

	retryErr := retry.RetryOnConflict(retry.DefaultRetry, func() error {
		_, updateErr := s.kubeClient.CoreV1().Pods(namespace).Patch(
			ctx,
			podName,
			types.JSONPatchType,
			payloadBytes,
			metav1.PatchOptions{},
		)
		return updateErr
	})

	if retryErr != nil {
		log.FromContext(ctx).Error(retryErr, "Patch pod labels failed after retries",
			"namespace", namespace,
			"podName", podName,
			"labels", labels)
		return fmt.Errorf("failed to patch labels after retries: %w", retryErr)
	}
	return nil
}
