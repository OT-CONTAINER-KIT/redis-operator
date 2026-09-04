package redisreplication

import (
	"context"
	"fmt"
	"net"
	"strings"

	rrvb2 "github.com/OT-CONTAINER-KIT/redis-operator/api/redisreplication/v1beta2"
	"github.com/OT-CONTAINER-KIT/redis-operator/internal/controller/common"
	"github.com/OT-CONTAINER-KIT/redis-operator/internal/controller/common/statefulset"
	"github.com/OT-CONTAINER-KIT/redis-operator/internal/envs"
	"github.com/OT-CONTAINER-KIT/redis-operator/internal/k8sutils"
	"github.com/OT-CONTAINER-KIT/redis-operator/internal/service/redis"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

func newSentinelService(rr *rrvb2.RedisReplication) corev1.Service {
	labels := common.GetRedisLabels(
		rr.SentinelStatefulSet(),
		common.SetupTypeSentinel,
		"sentinel",
		rr.GetLabels(),
	)

	return corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      rr.SentinelHLService(),
			Namespace: rr.Namespace,
			Labels:    labels,
		},
		Spec: corev1.ServiceSpec{
			ClusterIP: "None",
			Selector:  labels,
			Ports: []corev1.ServicePort{
				{
					Name:       "sentinel",
					Port:       26379,
					TargetPort: intstr.FromInt(26379),
					Protocol:   corev1.ProtocolTCP,
				},
			},
		},
	}
}

func newSentinelStatefulSet(rr *rrvb2.RedisReplication, svcName string) appsv1.StatefulSet {
	labels := common.GetRedisLabels(
		rr.SentinelStatefulSet(),
		common.SetupTypeSentinel,
		"sentinel",
		rr.GetLabels(),
	)
	return statefulset.New(statefulset.Params{
		Name:            rr.SentinelStatefulSet(),
		Namespace:       rr.Namespace,
		Replicas:        rr.Spec.Sentinel.Size,
		ServiceName:     svcName,
		PodTemplateSpec: buildSentinelPodTemplate(rr, labels),
	})
}

func buildSentinelPodTemplate(rr *rrvb2.RedisReplication, labels map[string]string) corev1.PodTemplateSpec {
	spec := corev1.PodSpec{
		Containers: []corev1.Container{
			buildSentinelContainer(rr),
		},
	}

	sentinel := rr.Spec.Sentinel
	if sentinel.Affinity != nil {
		spec.Affinity = sentinel.Affinity
	}
	if sentinel.Tolerations != nil {
		spec.Tolerations = *sentinel.Tolerations
	}
	if sentinel.NodeSelector != nil {
		spec.NodeSelector = sentinel.NodeSelector
	}
	if len(sentinel.TopologySpreadConstraints) > 0 {
		spec.TopologySpreadConstraints = sentinel.TopologySpreadConstraints
	}
	if sentinel.PodSecurityContext != nil {
		spec.SecurityContext = sentinel.PodSecurityContext
	}
	if sentinel.PriorityClassName != "" {
		spec.PriorityClassName = sentinel.PriorityClassName
	}
	if sentinel.TerminationGracePeriodSeconds != nil {
		spec.TerminationGracePeriodSeconds = sentinel.TerminationGracePeriodSeconds
	}
	if sentinel.ImagePullSecrets != nil {
		spec.ImagePullSecrets = *sentinel.ImagePullSecrets
	}
	if sentinel.ServiceAccountName != nil {
		spec.ServiceAccountName = *sentinel.ServiceAccountName
	}

	return corev1.PodTemplateSpec{
		ObjectMeta: metav1.ObjectMeta{
			Labels: labels,
		},
		Spec: spec,
	}
}

func buildSentinelContainer(rr *rrvb2.RedisReplication) corev1.Container {
	container := corev1.Container{
		Name:            "sentinel",
		Image:           rr.Spec.Sentinel.Image,
		ImagePullPolicy: rr.Spec.Sentinel.ImagePullPolicy,
		Ports: []corev1.ContainerPort{
			{
				Name:          "sentinel",
				ContainerPort: 26379,
				Protocol:      corev1.ProtocolTCP,
			},
		},
		Env: buildSentinelEnv(rr),
	}
	if rr.Spec.Sentinel.Resources != nil {
		container.Resources = *rr.Spec.Sentinel.Resources
	}
	if rr.Spec.Sentinel.SecurityContext != nil {
		container.SecurityContext = rr.Spec.Sentinel.SecurityContext
	}
	return container
}

func buildSentinelEnv(rr *rrvb2.RedisReplication) []corev1.EnvVar {
	envs := []corev1.EnvVar{
		{Name: "QUORUM", Value: fmt.Sprintf("%d", rr.Spec.Sentinel.Size/2+1)},
		{Name: "RESOLVE_HOSTNAMES", Value: resolveHostnamesOrDefault(rr.Spec.Sentinel.ResolveHostnames)},
		{Name: "ANNOUNCE_HOSTNAMES", Value: resolveHostnamesOrDefault(rr.Spec.Sentinel.AnnounceHostnames)},
	}
	passwordSecret := rr.Spec.KubernetesConfig.ExistingPasswordSecret
	if rr.Spec.Sentinel.ExistingPasswordSecret != nil {
		passwordSecret = rr.Spec.Sentinel.ExistingPasswordSecret
	}
	if passwordSecret != nil {
		envs = append(envs, corev1.EnvVar{
			Name: "MASTER_PASSWORD",
			ValueFrom: &corev1.EnvVarSource{
				SecretKeyRef: &corev1.SecretKeySelector{
					LocalObjectReference: corev1.LocalObjectReference{
						Name: *passwordSecret.Name,
					},
					Key: *passwordSecret.Key,
				},
			},
		})
	}

	return envs
}

func resolveHostnamesOrDefault(v string) string {
	if v == "" {
		return "no"
	}
	return v
}

func (r *Reconciler) sentinelMonitoredMaster(ctx context.Context, inst *rrvb2.RedisReplication, candidates []string) (string, error) {
	if r.SentinelMonitoredMaster != nil {
		return r.SentinelMonitoredMaster(ctx, inst, candidates)
	}
	return r.querySentinelMonitoredMaster(ctx, redis.NewClient(), inst, candidates)
}

func (r *Reconciler) querySentinelMonitoredMaster(ctx context.Context, redisClient redis.Client, inst *rrvb2.RedisReplication, candidates []string) (string, error) {
	// Sentinel reports the master by IP, or by hostname when hostnames are announced.
	podByAddress := make(map[string]string, 2*len(candidates))
	for _, podName := range candidates {
		pod, err := r.K8sClient.CoreV1().Pods(inst.Namespace).Get(ctx, podName, metav1.GetOptions{})
		if err != nil {
			return "", fmt.Errorf("get pod %s: %w", podName, err)
		}
		podByAddress[replicationPodHostname(inst, podName)] = podName
		if pod.Status.PodIP != "" {
			podByAddress[pod.Status.PodIP] = podName
		}
	}

	sentinelPassword, err := r.sentinelPassword(ctx, inst)
	if err != nil {
		return "", fmt.Errorf("get sentinel password secret: %w", err)
	}
	sentinelPods, err := r.getSentinelPods(ctx, inst)
	if err != nil {
		return "", fmt.Errorf("get sentinel pods: %w", err)
	}

	votes := make(map[string]int, len(candidates))
	for _, sentinelPod := range sentinelPods.Items {
		if !k8sutils.IsRedisPodProbeable(&sentinelPod) {
			continue
		}
		info, err := redisClient.Connect(&redis.ConnectionInfo{
			Host:     sentinelPod.Status.PodIP,
			Port:     "26379",
			Password: sentinelPassword,
		}).GetInfoSentinel(ctx)
		if err != nil {
			if ctxErr := ctx.Err(); ctxErr != nil {
				return "", ctxErr
			}
			log.FromContext(ctx).V(1).Info("Failed to read sentinel info, skipping sentinel pod", "pod", sentinelPod.Name, "error", err)
			continue
		}
		if podName := podByAddress[sentinelReportedMasterHost(info)]; podName != "" {
			votes[podName]++
		}
	}

	quorum := int(inst.Spec.Sentinel.Size/2) + 1
	for _, podName := range candidates {
		if votes[podName] >= quorum {
			return podName, nil
		}
	}
	return "", nil
}

func sentinelReportedMasterHost(info *redis.InfoSentinelResult) string {
	if info == nil {
		return ""
	}
	for _, master := range info.Masters {
		if master.Name == masterGroupName {
			return sentinelAddressHost(master.Address)
		}
	}
	return ""
}

func sentinelAddressHost(address string) string {
	if host, _, err := net.SplitHostPort(address); err == nil {
		return host
	}
	if i := strings.LastIndex(address, ":"); i >= 0 {
		return strings.Trim(address[:i], "[]")
	}
	return address
}

func (r *Reconciler) sentinelPassword(ctx context.Context, inst *rrvb2.RedisReplication) (string, error) {
	if inst.Spec.Sentinel.ExistingPasswordSecret == nil {
		return "", nil
	}
	secret, err := r.K8sClient.CoreV1().Secrets(inst.Namespace).Get(
		ctx,
		*inst.Spec.Sentinel.ExistingPasswordSecret.Name,
		metav1.GetOptions{},
	)
	if err != nil {
		return "", err
	}
	return string(secret.Data[*inst.Spec.Sentinel.ExistingPasswordSecret.Key]), nil
}

func replicationPodHostname(inst *rrvb2.RedisReplication, podName string) string {
	return fmt.Sprintf(
		"%s.%s.%s.svc.%s",
		podName,
		common.GetHeadlessServiceNameFromPodName(podName),
		inst.Namespace,
		envs.GetServiceDNSDomain(),
	)
}
