package redis

import (
	"context"
	"errors"
	"fmt"
	"strings"

	commonapi "github.com/OT-CONTAINER-KIT/redis-operator/api/common/v1beta2"
	rr "github.com/OT-CONTAINER-KIT/redis-operator/api/redisreplication/v1beta2"
	"github.com/OT-CONTAINER-KIT/redis-operator/internal/service/redis"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

type Checker interface {
	GetMasterFromReplication(ctx context.Context, rr *rr.RedisReplication) (corev1.Pod, error)
	GetPassword(ctx context.Context, ns string, secret *commonapi.ExistingPasswordSecret) (string, error)
}

type checker struct {
	redis redis.Client
	k8s   kubernetes.Interface
}

func NewChecker(clientset kubernetes.Interface) Checker {
	return &checker{
		k8s:   clientset,
		redis: redis.NewClient(),
	}
}

func (c *checker) GetPassword(ctx context.Context, ns string, secret *commonapi.ExistingPasswordSecret) (string, error) {
	if secret == nil {
		return "", nil
	}
	secretName, err := c.k8s.CoreV1().Secrets(ns).Get(ctx, *secret.Name, metav1.GetOptions{})
	if err != nil {
		return "", err
	}
	for key, value := range secretName.Data {
		if key == *secret.Key {
			return strings.TrimSpace(string(value)), nil
		}
	}
	return "", errors.New("secret key not found")
}

func (c *checker) GetMasterFromReplication(ctx context.Context, rr *rr.RedisReplication) (corev1.Pod, error) {
	sts, err := c.k8s.AppsV1().StatefulSets(rr.Namespace).Get(ctx, rr.GetStatefulSetName(), metav1.GetOptions{})
	if err != nil {
		return corev1.Pod{}, err
	}

	var labels []string
	for k, v := range sts.Spec.Selector.MatchLabels {
		labels = append(labels, fmt.Sprintf("%s=%s", k, v))
	}
	pods, err := c.k8s.CoreV1().Pods(rr.Namespace).List(ctx, metav1.ListOptions{
		LabelSelector: strings.Join(labels, ","),
	})
	if err != nil {
		return corev1.Pod{}, err
	}

	password, err := c.GetPassword(ctx, rr.Namespace, rr.Spec.KubernetesConfig.ExistingPasswordSecret)
	if err != nil {
		return corev1.Pod{}, err
	}

	var masterPods []corev1.Pod
	for _, pod := range pods.Items {
		connInfo := createConnectionInfo(ctx, pod, password, rr.Spec.TLS, c.k8s, rr.Namespace, "6379")
		isMaster, err := c.redis.Connect(connInfo).IsMaster(ctx)
		if err != nil {
			return corev1.Pod{}, err
		}
		if isMaster {
			masterPods = append(masterPods, pod)
		}
	}

	var realMasterPod corev1.Pod
	for _, pod := range masterPods {
		connInfo := createConnectionInfo(ctx, pod, password, rr.Spec.TLS, c.k8s, rr.Namespace, "6379")
		count, err := c.redis.Connect(connInfo).GetAttachedReplicaCount(ctx)
		if err != nil {
			continue
		}
		if count != 0 {
			realMasterPod = pod
			break
		} else {
			replicaNum := *sts.Spec.Replicas
			if replicaNum == 1 {
				realMasterPod = pod
				break
			}
		}
	}
	return realMasterPod, nil
}
