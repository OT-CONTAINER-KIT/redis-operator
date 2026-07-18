package redisreplication

import (
	"fmt"
	"path"

	commonapi "github.com/OT-CONTAINER-KIT/redis-operator/api/common/v1beta2"
	rrvb2 "github.com/OT-CONTAINER-KIT/redis-operator/api/redisreplication/v1beta2"
	"github.com/OT-CONTAINER-KIT/redis-operator/internal/controller/common"
	"github.com/OT-CONTAINER-KIT/redis-operator/internal/controller/common/statefulset"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
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

	// Add TLS certificate volume if TLS is enabled
	if rr.Spec.TLS != nil {
		spec.Volumes = []corev1.Volume{
			{
				Name: "tls-certs",
				VolumeSource: corev1.VolumeSource{
					Secret: &rr.Spec.TLS.Secret,
				},
			},
		}
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

	// Add TLS volume mount if TLS is enabled
	if rr.Spec.TLS != nil {
		container.VolumeMounts = []corev1.VolumeMount{
			{
				Name:      "tls-certs",
				ReadOnly:  true,
				MountPath: "/tls",
			},
		}
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
	}

	// Add TLS environment variables if TLS is enabled
	if rr.Spec.TLS != nil {
		envs = append(envs, generateSentinelTLSEnv(rr.Spec.TLS)...)
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

// generateSentinelTLSEnv creates TLS-related environment variables for Sentinel
func generateSentinelTLSEnv(tlsConfig *commonapi.TLSConfig) []corev1.EnvVar {
	root := "/tls/"

	// Get and set defaults
	caCert := "ca.crt"
	tlsCert := "tls.crt"
	tlsCertKey := "tls.key"

	if tlsConfig.CaCertFile != "" {
		caCert = tlsConfig.CaCertFile
	}
	if tlsConfig.CertKeyFile != "" {
		tlsCert = tlsConfig.CertKeyFile
	}
	if tlsConfig.KeyFile != "" {
		tlsCertKey = tlsConfig.KeyFile
	}

	caPath := path.Join(root, caCert)

	return []corev1.EnvVar{
		{
			Name:  "TLS_MODE",
			Value: "true",
		},
		{
			Name:  "REDIS_TLS_CA_CERT",
			Value: caPath,
		},
		// REDIS_TLS_CA_KEY: the sentinel/redis image entrypoints read this name instead of REDIS_TLS_CA_CERT
		{
			Name:  "REDIS_TLS_CA_KEY",
			Value: caPath,
		},
		{
			Name:  "REDIS_TLS_CERT",
			Value: path.Join(root, tlsCert),
		},
		{
			Name:  "REDIS_TLS_CERT_KEY",
			Value: path.Join(root, tlsCertKey),
		},
	}
}
