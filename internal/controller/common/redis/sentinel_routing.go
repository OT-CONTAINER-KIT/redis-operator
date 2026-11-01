package redis

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	commonapi "github.com/OT-CONTAINER-KIT/redis-operator/api/common/v1beta2"
	"github.com/OT-CONTAINER-KIT/redis-operator/internal/controller/common"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
)

// SetRedisMasterLabel removes old endpoints before advertising the verified
// primary. An empty selection or a failed role check deliberately fails closed.
func (h *healer) SetRedisMasterLabel(ctx context.Context, ns string, labels map[string]string, secret *commonapi.ExistingPasswordSecret, tlsConfig *commonapi.TLSConfig, masterPod string) error {
	var selector []string
	for key, value := range labels {
		selector = append(selector, key+"="+value)
	}
	pods, err := h.k8s.CoreV1().Pods(ns).List(ctx, metav1.ListOptions{LabelSelector: strings.Join(selector, ",")})
	if err != nil {
		return err
	}
	var chosen *v1.Pod
	var validationErr error
	if masterPod != "" {
		for i := range pods.Items {
			if pods.Items[i].Name == masterPod {
				chosen = &pods.Items[i]
			}
		}
		if !isPodReady(chosen) {
			validationErr = fmt.Errorf("sentinel-selected primary is not a ready pod")
		} else {
			password, err := NewChecker(h.k8s).GetPassword(ctx, ns, secret)
			if err != nil {
				validationErr = err
			} else if master, err := h.redis.Connect(createConnectionInfo(ctx, *chosen, password, tlsConfig, h.k8s, ns, "6379")).IsMaster(ctx); err != nil || !master {
				validationErr = fmt.Errorf("sentinel-selected pod does not confirm role:master")
			}
		}
	}
	if validationErr != nil {
		masterPod = ""
	}
	patchRole := func(pod v1.Pod, role string) error {
		if pod.Labels[common.RedisRoleLabelKey] == role {
			return nil
		}
		operations := []map[string]interface{}{}
		if role == common.RedisRoleLabelMaster {
			if pod.UID == "" {
				return fmt.Errorf("cannot advertise a pod without a verified UID")
			}
			operations = append(operations, map[string]interface{}{"op": "test", "path": "/metadata/uid", "value": pod.UID})
			if pod.ResourceVersion != "" {
				operations = append(operations, map[string]interface{}{"op": "test", "path": "/metadata/resourceVersion", "value": pod.ResourceVersion})
			}
		}
		operations = append(operations, map[string]interface{}{"op": "add", "path": "/metadata/labels/" + common.RedisRoleLabelKey, "value": role})
		patch, err := json.Marshal(operations)
		if err != nil {
			return err
		}
		_, err = h.k8s.CoreV1().Pods(ns).Patch(ctx, pod.Name, types.JSONPatchType, patch, metav1.PatchOptions{})
		return err
	}
	for _, pod := range pods.Items {
		if pod.Name != masterPod {
			if err := patchRole(pod, common.RedisRoleLabelSlave); err != nil {
				return err
			}
		}
	}
	if masterPod != "" {
		if err := patchRole(*chosen, common.RedisRoleLabelMaster); err != nil {
			return err
		}
	}
	return validationErr
}

func isPodReady(pod *v1.Pod) bool {
	if pod == nil || pod.Status.PodIP == "" || pod.DeletionTimestamp != nil {
		return false
	}
	for _, condition := range pod.Status.Conditions {
		if condition.Type == v1.PodReady && condition.Status == v1.ConditionTrue {
			return true
		}
	}
	return false
}
