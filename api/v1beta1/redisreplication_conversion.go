package v1beta1

import (
	redisv1beta2 "github.com/OT-CONTAINER-KIT/redis-operator/api/v1beta2"
	"sigs.k8s.io/controller-runtime/pkg/conversion"
)

// ConvertTo converts this RedisReplication to the Hub version (vbeta2).
func (src *RedisReplication) ConvertTo(dstRaw conversion.Hub) error {
	dst := dstRaw.(*redisv1beta2.RedisReplication)

	// ObjectMeta
	dst.ObjectMeta = src.ObjectMeta

	// Size
	dst.Spec.Size = src.Spec.Size
	// KubernetesConfig
	dst.Spec.KubernetesConfig.KubernetesConfig = src.Spec.KubernetesConfig.KubernetesConfig
	// RedisExporter
	if src.Spec.RedisExporter != nil {
		dst.Spec.RedisExporter.RedisExporter = src.Spec.RedisExporter.RedisExporter
	}
	// RedisConfig
	if src.Spec.RedisConfig != nil {
		dst.Spec.RedisConfig.RedisConfig = src.Spec.RedisConfig.RedisConfig
	}
	// Storage
	if src.Spec.Storage != nil {
		dst.Spec.Storage.CommonAttributes = src.Spec.Storage.CommonAttributes
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
		dst.Spec.TLS.TLSConfig = src.Spec.TLS.TLSConfig
	}
	// ReadinessProbe
	if src.Spec.ReadinessProbe != nil {
		dst.Spec.ReadinessProbe.Probe = src.Spec.ReadinessProbe.Probe
	}
	// LivenessProbe
	if src.Spec.LivenessProbe != nil {
		dst.Spec.LivenessProbe.Probe = src.Spec.LivenessProbe.Probe
	}
	// Sidecars
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
func (dst *RedisReplication) ConvertFrom(srcRaw conversion.Hub) error {
	src := srcRaw.(*redisv1beta2.RedisReplication)

	// ObjectMeta
	dst.ObjectMeta = src.ObjectMeta

	// Size
	dst.Spec.Size = src.Spec.Size
	// KubernetesConfig
	dst.Spec.KubernetesConfig.KubernetesConfig = src.Spec.KubernetesConfig.KubernetesConfig
	// RedisExporter
	if src.Spec.RedisExporter != nil {
		dst.Spec.RedisExporter.RedisExporter = src.Spec.RedisExporter.RedisExporter
	}
	// RedisConfig
	if src.Spec.RedisConfig != nil {
		dst.Spec.RedisConfig.RedisConfig = src.Spec.RedisConfig.RedisConfig
	}
	// Storage
	if src.Spec.Storage != nil {
		dst.Spec.Storage.CommonAttributes = src.Spec.Storage.CommonAttributes
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
		dst.Spec.TLS.TLSConfig = src.Spec.TLS.TLSConfig
	}
	// ReadinessProbe
	if src.Spec.ReadinessProbe != nil {
		dst.Spec.ReadinessProbe.Probe = src.Spec.ReadinessProbe.Probe
	}
	// LivenessProbe
	if src.Spec.LivenessProbe != nil {
		dst.Spec.LivenessProbe.Probe = src.Spec.LivenessProbe.Probe
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
