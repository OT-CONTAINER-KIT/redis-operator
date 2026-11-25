package redisreplication

import (
	"context"
	"fmt"
	"net"
	"strings"

	commonapi "github.com/OT-CONTAINER-KIT/redis-operator/api/common/v1beta2"
	rrvb2 "github.com/OT-CONTAINER-KIT/redis-operator/api/redisreplication/v1beta2"
	rsvb2 "github.com/OT-CONTAINER-KIT/redis-operator/api/redissentinel/v1beta2"
	"github.com/OT-CONTAINER-KIT/redis-operator/internal/controller/common"
	redishealer "github.com/OT-CONTAINER-KIT/redis-operator/internal/controller/common/redis"
	"github.com/OT-CONTAINER-KIT/redis-operator/internal/controller/common/statefulset"
	"github.com/OT-CONTAINER-KIT/redis-operator/internal/envs"
	"github.com/OT-CONTAINER-KIT/redis-operator/internal/k8sutils"
	"github.com/OT-CONTAINER-KIT/redis-operator/internal/service/redis"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"
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
		MinReadySeconds: ptr.Deref(rr.Spec.Sentinel.MinReadySeconds, 0),
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
	if secret := rr.Spec.KubernetesConfig.ExistingPasswordSecret; secret != nil {
		envs = append(envs, secretEnvVar("MASTER_PASSWORD", secret))
	}
	if secret := rr.Spec.Sentinel.ExistingPasswordSecret; secret != nil {
		envs = append(envs, secretEnvVar("REDIS_PASSWORD", secret))
	}

	return envs
}

func secretEnvVar(name string, secret *commonapi.ExistingPasswordSecret) corev1.EnvVar {
	return corev1.EnvVar{
		Name: name,
		ValueFrom: &corev1.EnvVarSource{
			SecretKeyRef: &corev1.SecretKeySelector{
				LocalObjectReference: corev1.LocalObjectReference{
					Name: *secret.Name,
				},
				Key: *secret.Key,
			},
		},
	}
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

func (r *Reconciler) sentinelFailoverInProgress(ctx context.Context, inst *rrvb2.RedisReplication) (bool, error) {
	if r.SentinelFailoverInProgress != nil {
		return r.SentinelFailoverInProgress(ctx, inst)
	}
	return r.querySentinelFailoverInProgress(ctx, redis.NewClient(), inst)
}

// querySentinelFailoverInProgress reports whether a Sentinel monitoring inst
// runs a failover. Only the leader of a failover reports it, so one Sentinel
// is enough; unreachable Sentinels are skipped.
func (r *Reconciler) querySentinelFailoverInProgress(ctx context.Context, redisClient redis.Client, inst *rrvb2.RedisReplication) (bool, error) {
	voters, err := r.linkedSentinelVoters(ctx, inst)
	if err != nil || voters == nil {
		return false, err
	}
	for _, sentinelPod := range voters.pods {
		if !k8sutils.IsRedisPodProbeable(&sentinelPod) {
			continue
		}
		fields, err := redisClient.Connect(r.sentinelConnectionInfo(ctx, inst, voters, sentinelPod)).SentinelMaster(ctx, voters.masterName)
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

// sentinelVoters are the Sentinels that monitor a RedisReplication: the
// embedded spec.sentinel, or a standalone RedisSentinel that references it.
type sentinelVoters struct {
	pods       []corev1.Pod
	password   string
	masterName string
	quorum     int
	// tls is the TLS configuration of a standalone RedisSentinel; the
	// embedded Sentinel does not support TLS.
	tls *commonapi.TLSConfig
}

func (r *Reconciler) sentinelConnectionInfo(ctx context.Context, inst *rrvb2.RedisReplication, voters *sentinelVoters, sentinelPod corev1.Pod) *redis.ConnectionInfo {
	return redishealer.SentinelConnectionInfo(ctx, r.K8sClient, sentinelPod, voters.password, voters.tls, inst.Namespace)
}

func (r *Reconciler) linkedSentinelVoters(ctx context.Context, inst *rrvb2.RedisReplication) (*sentinelVoters, error) {
	if inst.EnableSentinel() {
		password, err := r.sentinelPassword(ctx, inst)
		if err != nil {
			return nil, fmt.Errorf("get sentinel password secret: %w", err)
		}
		pods, err := r.getSentinelPods(ctx, inst)
		if err != nil {
			return nil, fmt.Errorf("get sentinel pods: %w", err)
		}
		return &sentinelVoters{
			pods:       pods.Items,
			password:   password,
			masterName: masterGroupName,
			quorum:     int(inst.Spec.Sentinel.Size/2) + 1,
		}, nil
	}

	rs, err := r.linkedStandaloneSentinel(ctx, inst)
	if err != nil || rs == nil {
		return nil, err
	}
	password, err := r.secretPassword(ctx, inst.Namespace, rs.Spec.KubernetesConfig.ExistingPasswordSecret)
	if err != nil {
		return nil, fmt.Errorf("get sentinel password secret of %s: %w", rs.Name, err)
	}
	sts, err := r.K8sClient.AppsV1().StatefulSets(rs.Namespace).Get(ctx, rs.GetStatefulSetName(), metav1.GetOptions{})
	if err != nil {
		return nil, fmt.Errorf("get sentinel statefulset of %s: %w", rs.Name, err)
	}
	selector, err := metav1.LabelSelectorAsSelector(sts.Spec.Selector)
	if err != nil {
		return nil, fmt.Errorf("parse sentinel selector of %s: %w", rs.Name, err)
	}
	pods, err := r.K8sClient.CoreV1().Pods(rs.Namespace).List(ctx, metav1.ListOptions{LabelSelector: selector.String()})
	if err != nil {
		return nil, fmt.Errorf("get sentinel pods of %s: %w", rs.Name, err)
	}
	size := int32(1)
	if rs.Spec.Size != nil {
		size = *rs.Spec.Size
	}
	return &sentinelVoters{
		pods:       pods.Items,
		password:   password,
		masterName: rs.Spec.RedisSentinelConfig.MasterGroupName,
		tls:        rs.Spec.TLS,
		quorum:     int(size/2) + 1,
	}, nil
}

// linkedStandaloneSentinel returns the RedisSentinel that monitors inst, or nil.
func (r *Reconciler) linkedStandaloneSentinel(ctx context.Context, inst *rrvb2.RedisReplication) (*rsvb2.RedisSentinel, error) {
	var sentinels rsvb2.RedisSentinelList
	if err := r.List(ctx, &sentinels, client.InNamespace(inst.Namespace)); err != nil {
		return nil, fmt.Errorf("list redis sentinels: %w", err)
	}
	for i := range sentinels.Items {
		rs := &sentinels.Items[i]
		if rs.Spec.RedisSentinelConfig != nil && rs.Spec.RedisSentinelConfig.RedisReplicationName == inst.Name {
			return rs, nil
		}
	}
	return nil, nil
}

// querySentinelMonitoredMaster returns the candidate that a quorum of the
// Sentinels monitoring inst reports as master, or "" if there is none or no
// Sentinel monitors inst.
func (r *Reconciler) querySentinelMonitoredMaster(ctx context.Context, redisClient redis.Client, inst *rrvb2.RedisReplication, candidates []string) (string, error) {
	voters, err := r.linkedSentinelVoters(ctx, inst)
	if err != nil {
		return "", err
	}
	if voters == nil {
		return "", nil
	}

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

	votes := make(map[string]int, len(candidates))
	for _, sentinelPod := range voters.pods {
		if !k8sutils.IsRedisPodProbeable(&sentinelPod) {
			continue
		}
		info, err := redisClient.Connect(r.sentinelConnectionInfo(ctx, inst, voters, sentinelPod)).GetInfoSentinel(ctx)
		if err != nil {
			if ctxErr := ctx.Err(); ctxErr != nil {
				return "", ctxErr
			}
			log.FromContext(ctx).V(1).Info("Failed to read sentinel info, skipping sentinel pod", "pod", sentinelPod.Name, "error", err)
			continue
		}
		if podName := podByAddress[sentinelReportedMasterHost(info, voters.masterName)]; podName != "" {
			votes[podName]++
		}
	}

	for _, podName := range candidates {
		if votes[podName] >= voters.quorum {
			return podName, nil
		}
	}
	return "", nil
}

func sentinelReportedMasterHost(info *redis.InfoSentinelResult, masterName string) string {
	if info == nil {
		return ""
	}
	for _, master := range info.Masters {
		if master.Name == masterName {
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
	return r.secretPassword(ctx, inst.Namespace, inst.Spec.Sentinel.ExistingPasswordSecret)
}

func (r *Reconciler) masterPassword(ctx context.Context, inst *rrvb2.RedisReplication) (string, error) {
	return r.secretPassword(ctx, inst.Namespace, inst.Spec.KubernetesConfig.ExistingPasswordSecret)
}

func (r *Reconciler) secretPassword(ctx context.Context, namespace string, ref *commonapi.ExistingPasswordSecret) (string, error) {
	if ref == nil {
		return "", nil
	}
	secret, err := r.K8sClient.CoreV1().Secrets(namespace).Get(ctx, *ref.Name, metav1.GetOptions{})
	if err != nil {
		return "", err
	}
	return string(secret.Data[*ref.Key]), nil
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
