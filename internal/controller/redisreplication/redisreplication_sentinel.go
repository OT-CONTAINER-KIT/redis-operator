package redisreplication

import (
	"context"
	"crypto/tls"
	"fmt"
	"net"
	"strconv"
	"strings"

	commonapi "github.com/OT-CONTAINER-KIT/redis-operator/api/common/v1beta2"
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
	"k8s.io/utils/ptr"
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

const (
	// sentinelExternalConfigKey is the file the sentinel image's entrypoint
	// includes as the last line of sentinel.conf when it exists, so its lines
	// override everything the entrypoint rendered before it.
	sentinelExternalConfigKey  = "redis-sentinel-additional.conf"
	sentinelExternalConfigPath = "/etc/redis/external.conf.d"
	sentinelTLSVolumeName      = "tls-certs"
	sentinelTLSMountPath       = "/tls"
	sentinelTLSConfigVolume    = "external-config"
)

// sentinelTLSConfigLines completes the TLS listener the entrypoint configures.
// Without tls-replication Sentinel talks to Redis and to the other Sentinels in
// plain text, so a TLS-only master shows up as disconnected; and the
// entrypoint's "tls-auth-clients optional" would let any TLS client without a
// certificate run SENTINEL FAILOVER, SET or REMOVE.
const sentinelTLSConfigLines = "tls-replication yes\ntls-auth-clients yes\n"

func sentinelTLSConfigMapName(rr *rrvb2.RedisReplication) string {
	return rr.SentinelStatefulSet() + "-tls"
}

// newSentinelTLSConfigMap returns the ConfigMap the Sentinel pods include as
// their external config when the replication runs with TLS.
func newSentinelTLSConfigMap(rr *rrvb2.RedisReplication) corev1.ConfigMap {
	return corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      sentinelTLSConfigMapName(rr),
			Namespace: rr.Namespace,
			Labels: common.GetRedisLabels(
				rr.SentinelStatefulSet(),
				common.SetupTypeSentinel,
				"sentinel",
				rr.GetLabels(),
			),
		},
		Data: map[string]string{
			sentinelExternalConfigKey: sentinelTLSConfigLines,
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
	if tlsConfig := rr.Spec.TLS; tlsConfig != nil {
		spec.Volumes = append(spec.Volumes,
			corev1.Volume{
				Name: sentinelTLSVolumeName,
				VolumeSource: corev1.VolumeSource{
					Secret: tlsConfig.Secret.DeepCopy(),
				},
			},
			corev1.Volume{
				Name: sentinelTLSConfigVolume,
				VolumeSource: corev1.VolumeSource{
					ConfigMap: &corev1.ConfigMapVolumeSource{
						LocalObjectReference: corev1.LocalObjectReference{Name: sentinelTLSConfigMapName(rr)},
					},
				},
			},
		)
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
		// The same redis-cli PING probe the operator renders for its other
		// pods. Readiness gates the operator: it waits for the Sentinel
		// StatefulSet to be ready and only asks ready Sentinels for the master.
		ReadinessProbe: k8sutils.GetProbeInfo(nil, true, rr.Spec.TLS != nil),
		LivenessProbe:  k8sutils.GetProbeInfo(nil, true, rr.Spec.TLS != nil),
	}
	if rr.Spec.Sentinel.Resources != nil {
		container.Resources = *rr.Spec.Sentinel.Resources
	}
	if rr.Spec.Sentinel.SecurityContext != nil {
		container.SecurityContext = rr.Spec.Sentinel.SecurityContext
	}
	if rr.Spec.TLS != nil {
		// Whole-directory mounts, not subPath, so the files follow a reissued
		// certificate. The probes and the next Sentinel start pick it up; a
		// running Sentinel keeps the certificate it loaded at startup.
		container.VolumeMounts = []corev1.VolumeMount{
			{Name: sentinelTLSVolumeName, MountPath: sentinelTLSMountPath, ReadOnly: true},
			{Name: sentinelTLSConfigVolume, MountPath: sentinelExternalConfigPath, ReadOnly: true},
		}
	}
	return container
}

func buildSentinelEnv(rr *rrvb2.RedisReplication) []corev1.EnvVar {
	envs := []corev1.EnvVar{
		{Name: "QUORUM", Value: fmt.Sprintf("%d", rr.Spec.Sentinel.Size/2+1)},
		{Name: "RESOLVE_HOSTNAMES", Value: resolveHostnamesOrDefault(rr.Spec.Sentinel.ResolveHostnames)},
		{Name: "ANNOUNCE_HOSTNAMES", Value: resolveHostnamesOrDefault(rr.Spec.Sentinel.AnnounceHostnames)},
		// Read by the probes. The image defaults to the same port; setting it
		// keeps the probes correct with images that do not.
		{Name: "SENTINEL_PORT", Value: strconv.Itoa(common.SentinelPort)},
	}
	if secret := rr.Spec.KubernetesConfig.ExistingPasswordSecret; secret != nil {
		envs = append(envs, secretEnvVar("MASTER_PASSWORD", secret))
	}
	if secret := rr.Spec.Sentinel.ExistingPasswordSecret; secret != nil {
		// REDISCLI_AUTH lets the probes authenticate without a password on
		// the command line, as on the Redis pods.
		envs = append(envs, secretEnvVar("REDIS_PASSWORD", secret), secretEnvVar("REDISCLI_AUTH", secret))
	}
	if rr.Spec.TLS != nil {
		envs = append(envs, k8sutils.GenerateSentinelTLSEnvironmentVariables(rr.Spec.TLS)...)
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
	sentinelTLS, err := r.sentinelTLSConfig(ctx, inst)
	if err != nil {
		return "", err
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
			Host:      sentinelPod.Status.PodIP,
			Port:      "26379",
			Password:  sentinelPassword,
			TLSConfig: sentinelTLS,
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
	return r.secretPassword(ctx, inst.Namespace, inst.Spec.Sentinel.ExistingPasswordSecret)
}

// sentinelTLSConfig returns the client TLS config for operator connections to
// the embedded Sentinel, or nil when the replication does not use TLS. With
// spec.TLS the Sentinel pods only listen on TLS and require a client
// certificate, so a config that cannot be built is an error, not a reason to
// fall back to plain TCP. The peer must chain to the secret's CA: a secret
// without one is an error, never a fallback to the system trust store. The
// secret is read on every call, so a reissued certificate is used from the
// next reconcile on.
func (r *Reconciler) sentinelTLSConfig(ctx context.Context, inst *rrvb2.RedisReplication) (*tls.Config, error) {
	tlsConfig, err := k8sutils.GetSentinelTLSConfig(ctx, r.K8sClient, inst.Namespace, inst.Spec.TLS)
	if err != nil {
		return nil, fmt.Errorf("build sentinel TLS config: %w", err)
	}
	return tlsConfig, nil
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
