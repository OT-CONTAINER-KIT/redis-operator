package redis

import (
	"context"
	"fmt"
	"net"
	"strings"

	rrvb2 "github.com/OT-CONTAINER-KIT/redis-operator/api/redisreplication/v1beta2"
	rsvb2 "github.com/OT-CONTAINER-KIT/redis-operator/api/redissentinel/v1beta2"
	"github.com/OT-CONTAINER-KIT/redis-operator/internal/controller/common"
	"github.com/OT-CONTAINER-KIT/redis-operator/internal/envs"
	"github.com/OT-CONTAINER-KIT/redis-operator/internal/k8sutils"
	"github.com/OT-CONTAINER-KIT/redis-operator/internal/service/redis"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

// GetSentinelElectedMaster returns the pod of rr that a quorum of the
// Sentinels of rs reports as master, if that pod reports role:master itself.
// It returns an empty pod when no quorum names a pod of rr, so that callers can
// fall back to GetMasterFromReplication.
//
// Right after a Sentinel failover the promoted pod has no replica yet while
// the old master may still report role:master with replicas attached; asking
// Sentinel first keeps the operator from pointing Sentinel back at the old
// master.
func (c *checker) GetSentinelElectedMaster(ctx context.Context, rs *rsvb2.RedisSentinel, rr *rrvb2.RedisReplication) (corev1.Pod, error) {
	if rs.Spec.RedisSentinelConfig == nil {
		return corev1.Pod{}, nil
	}
	replicationPods, err := c.podsOfStatefulSet(ctx, rr.Namespace, rr.GetStatefulSetName())
	if err != nil {
		return corev1.Pod{}, fmt.Errorf("get replication pods: %w", err)
	}
	podByAddress := make(map[string]corev1.Pod, 2*len(replicationPods))
	for _, pod := range replicationPods {
		podByAddress[fmt.Sprintf("%s.%s.%s.svc.%s", pod.Name, common.GetHeadlessServiceNameFromPodName(pod.Name), rr.Namespace, envs.GetServiceDNSDomain())] = pod
		if pod.Status.PodIP != "" {
			podByAddress[pod.Status.PodIP] = pod
		}
	}

	sentinelPods, err := c.podsOfStatefulSet(ctx, rs.Namespace, rs.GetStatefulSetName())
	if err != nil {
		return corev1.Pod{}, fmt.Errorf("get sentinel pods: %w", err)
	}
	sentinelPassword, err := c.GetPassword(ctx, rs.Namespace, rs.Spec.KubernetesConfig.ExistingPasswordSecret)
	if err != nil {
		return corev1.Pod{}, fmt.Errorf("get sentinel password: %w", err)
	}

	votes := make(map[string]int, len(replicationPods))
	for _, sentinelPod := range sentinelPods {
		if !k8sutils.IsRedisPodProbeable(&sentinelPod) {
			continue
		}
		connInfo := createConnectionInfo(ctx, sentinelPod, sentinelPassword, rs.Spec.TLS, c.k8s, rs.Namespace, "26379")
		info, err := c.redis.Connect(connInfo).GetInfoSentinel(ctx)
		if err != nil {
			if ctxErr := ctx.Err(); ctxErr != nil {
				return corev1.Pod{}, ctxErr
			}
			log.FromContext(ctx).V(1).Info("Failed to read sentinel info, skipping sentinel pod", "pod", sentinelPod.Name, "error", err)
			continue
		}
		if info == nil {
			continue
		}
		for _, master := range info.Masters {
			if master.Name != rs.Spec.RedisSentinelConfig.MasterGroupName {
				continue
			}
			if pod, ok := podByAddress[addressHost(master.Address)]; ok {
				votes[pod.Name]++
			}
		}
	}

	size := int32(1)
	if rs.Spec.Size != nil {
		size = *rs.Spec.Size
	}
	quorum := int(size/2) + 1
	for _, pod := range replicationPods {
		if votes[pod.Name] < quorum {
			continue
		}
		password, err := c.GetPassword(ctx, rr.Namespace, rr.Spec.KubernetesConfig.ExistingPasswordSecret)
		if err != nil {
			return corev1.Pod{}, fmt.Errorf("get redis password: %w", err)
		}
		isMaster, err := c.redis.Connect(createConnectionInfo(ctx, pod, password, rr.Spec.TLS, c.k8s, rr.Namespace, "6379")).IsMaster(ctx)
		if err != nil {
			return corev1.Pod{}, fmt.Errorf("check role of %s: %w", pod.Name, err)
		}
		if !isMaster {
			log.FromContext(ctx).Info("Sentinel quorum names a pod that does not report role:master, not trusting it", "pod", pod.Name)
			return corev1.Pod{}, nil
		}
		return pod, nil
	}
	return corev1.Pod{}, nil
}

func (c *checker) podsOfStatefulSet(ctx context.Context, namespace, name string) ([]corev1.Pod, error) {
	sts, err := c.k8s.AppsV1().StatefulSets(namespace).Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		return nil, err
	}
	selector, err := metav1.LabelSelectorAsSelector(sts.Spec.Selector)
	if err != nil {
		return nil, err
	}
	pods, err := c.k8s.CoreV1().Pods(namespace).List(ctx, metav1.ListOptions{LabelSelector: selector.String()})
	if err != nil {
		return nil, err
	}
	return pods.Items, nil
}

// addressHost strips the port from a Sentinel master address. INFO sentinel
// prints IPv6 addresses without brackets, so the last colon is the separator.
func addressHost(address string) string {
	if host, _, err := net.SplitHostPort(address); err == nil {
		return host
	}
	if i := strings.LastIndex(address, ":"); i >= 0 {
		return strings.Trim(address[:i], "[]")
	}
	return address
}

// SentinelFailoverInProgress reports whether a Sentinel of rs is running a
// failover of its master group. Only the leader of a failover reports it, so
// one Sentinel is enough; unreachable Sentinels are skipped.
func (c *checker) SentinelFailoverInProgress(ctx context.Context, rs *rsvb2.RedisSentinel) (bool, error) {
	if rs.Spec.RedisSentinelConfig == nil {
		return false, nil
	}
	sentinelPods, err := c.podsOfStatefulSet(ctx, rs.Namespace, rs.GetStatefulSetName())
	if err != nil {
		return false, fmt.Errorf("get sentinel pods: %w", err)
	}
	sentinelPassword, err := c.GetPassword(ctx, rs.Namespace, rs.Spec.KubernetesConfig.ExistingPasswordSecret)
	if err != nil {
		return false, fmt.Errorf("get sentinel password: %w", err)
	}
	for _, sentinelPod := range sentinelPods {
		if !k8sutils.IsRedisPodProbeable(&sentinelPod) {
			continue
		}
		connInfo := createConnectionInfo(ctx, sentinelPod, sentinelPassword, rs.Spec.TLS, c.k8s, rs.Namespace, "26379")
		fields, err := c.redis.Connect(connInfo).SentinelMaster(ctx, rs.Spec.RedisSentinelConfig.MasterGroupName)
		if err != nil {
			if ctxErr := ctx.Err(); ctxErr != nil {
				return false, ctxErr
			}
			log.FromContext(ctx).V(1).Info("Failed to read sentinel master, skipping sentinel pod", "pod", sentinelPod.Name, "error", err)
			continue
		}
		if redis.SentinelFailoverInProgress(fields) {
			return true, nil
		}
	}
	return false, nil
}
