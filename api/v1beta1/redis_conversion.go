package v1beta1

import (
	redisv1beta2 "github.com/OT-CONTAINER-KIT/redis-operator/api/v1beta2"
	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/conversion"
)

// ConvertTo converts this Redis to the Hub version (vbeta2).
func (src *Redis) ConvertTo(dstRaw conversion.Hub) error {
	dst := dstRaw.(*redisv1beta2.Redis)

	// if src == nil {
	// 	return errors.New("source is nil")
	// }
	// ObjectMeta
	dst.ObjectMeta = src.ObjectMeta

	// KubernetesConfig
	dst.Spec.KubernetesConfig.KubernetesConfig = src.Spec.KubernetesConfig.KubernetesConfig
	// RedisExporter
	if src.Spec.RedisExporter != nil {
		dst.Spec.RedisExporter = &redisv1beta2.RedisExporter{}
		dst.Spec.RedisExporter.RedisExporter = src.Spec.RedisExporter.RedisExporter
	}
	// RedisConfig
	if src.Spec.RedisConfig != nil {
		dst.Spec.RedisConfig = &redisv1beta2.RedisConfig{}
		dst.Spec.RedisConfig.RedisConfig = src.Spec.RedisConfig.RedisConfig
	}
	// Storage
	if src.Spec.Storage != nil {
		dst.Spec.Storage = &redisv1beta2.Storage{}
		dst.Spec.Storage.VolumeClaimTemplate = src.Spec.Storage.VolumeClaimTemplate
		dst.Spec.Storage.VolumeMount = src.Spec.Storage.VolumeMount
	}
	// NodeSelector
	if src.Spec.NodeSelector != nil {
		dst.Spec.NodeSelector = src.Spec.NodeSelector
	}
	// SecurityContext >> PodSecurityContext
	if src.Spec.SecurityContext != nil {
		dst.Spec.PodSecurityContext = src.Spec.SecurityContext
	}
	// PriorityClassName
	if src.Spec.PriorityClassName != "" {
		dst.Spec.PriorityClassName = src.Spec.PriorityClassName
	}
	// Affinity
	if src.Spec.Affinity != nil {
		dst.Spec.Affinity = src.Spec.Affinity
	}
	// Tolerations
	if src.Spec.Tolerations != nil {
		dst.Spec.Tolerations = src.Spec.Tolerations
	}
	// TLS
	if src.Spec.TLS != nil {
		dst.Spec.TLS = &redisv1beta2.TLSConfig{}
		dst.Spec.TLS.TLSConfig = src.Spec.TLS.TLSConfig
	}
	// ReadinessProbe
	if src.Spec.ReadinessProbe != nil {
		dst.Spec.ReadinessProbe = &corev1.Probe{}
		dst.Spec.ReadinessProbe = src.Spec.ReadinessProbe
	}
	// LivenessProbe
	if src.Spec.LivenessProbe != nil {
		dst.Spec.LivenessProbe = &corev1.Probe{}
		dst.Spec.LivenessProbe = src.Spec.LivenessProbe
	}
	// Sidecars
	if src.Spec.Sidecars != nil {
		var sidecars []redisv1beta2.Sidecar
		for _, sidecar := range *src.Spec.Sidecars {
			sidecars = append(sidecars, redisv1beta2.Sidecar{
				Sidecar: sidecar.Sidecar,
			})
		}
		dst.Spec.Sidecars = &sidecars
	}
	// ServiceAccountName
	if src.Spec.ServiceAccountName != nil {
		dst.Spec.ServiceAccountName = src.Spec.ServiceAccountName
	}

	return nil
}

// ConvertFrom converts from the Hub version (vbeta2) to this version.
func (dst *Redis) ConvertFrom(srcRaw conversion.Hub) error {
	src := srcRaw.(*redisv1beta2.Redis)

	// if src == nil {
	// 	return errors.New("source is nil")
	// }
	// ObjectMeta
	dst.ObjectMeta = src.ObjectMeta

	// KubernetesConfig
	dst.Spec.KubernetesConfig.KubernetesConfig = src.Spec.KubernetesConfig.KubernetesConfig
	// RedisExporter
	if src.Spec.RedisExporter != nil {
		dst.Spec.RedisExporter = &RedisExporter{}
		dst.Spec.RedisExporter.RedisExporter = src.Spec.RedisExporter.RedisExporter
	}
	// RedisConfig
	if src.Spec.RedisConfig != nil {
		dst.Spec.RedisConfig = &RedisConfig{}
		dst.Spec.RedisConfig.RedisConfig = src.Spec.RedisConfig.RedisConfig
	}
	// Storage
	if src.Spec.Storage != nil {
		dst.Spec.Storage = &Storage{}
		dst.Spec.Storage.VolumeClaimTemplate = src.Spec.Storage.VolumeClaimTemplate
		dst.Spec.Storage.VolumeMount = src.Spec.Storage.VolumeMount
	}
	// NodeSelector
	if src.Spec.NodeSelector != nil {
		dst.Spec.NodeSelector = src.Spec.NodeSelector
	}
	//  PodSecurityContext >> SecurityContext
	if src.Spec.PodSecurityContext != nil {
		dst.Spec.SecurityContext = src.Spec.PodSecurityContext
	}
	// PriorityClassName
	if src.Spec.PriorityClassName != "" {
		dst.Spec.PriorityClassName = src.Spec.PriorityClassName
	}
	// Affinity
	if src.Spec.Affinity != nil {
		dst.Spec.Affinity = src.Spec.Affinity
	}
	// Tolerations
	if src.Spec.Tolerations != nil {
		dst.Spec.Tolerations = src.Spec.Tolerations
	}
	// TLS
	if src.Spec.TLS != nil {
		dst.Spec.TLS = &TLSConfig{}
		dst.Spec.TLS.TLSConfig = src.Spec.TLS.TLSConfig
	}
	// ReadinessProbe
	if src.Spec.ReadinessProbe != nil {
		dst.Spec.ReadinessProbe = &corev1.Probe{}
		dst.Spec.ReadinessProbe = src.Spec.ReadinessProbe
	}
	// LivenessProbe
	if src.Spec.LivenessProbe != nil {
		dst.Spec.LivenessProbe = &corev1.Probe{}
		dst.Spec.LivenessProbe = src.Spec.LivenessProbe
	}
	// Sidecars
	if src.Spec.Sidecars != nil {
		var sidecars []Sidecar
		for _, sidecar := range *src.Spec.Sidecars {
			sidecars = append(sidecars, Sidecar{
				Sidecar: sidecar.Sidecar,
			})
		}
		dst.Spec.Sidecars = &sidecars
	}
	// ServiceAccountName
	if src.Spec.ServiceAccountName != nil {
		dst.Spec.ServiceAccountName = src.Spec.ServiceAccountName
	}
	return nil
}
