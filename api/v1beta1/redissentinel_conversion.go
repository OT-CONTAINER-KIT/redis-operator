package v1beta1

import (
	redisv1beta2 "github.com/OT-CONTAINER-KIT/redis-operator/api/v1beta2"
	"sigs.k8s.io/controller-runtime/pkg/conversion"
)

// ConvertTo converts this RedisSentinel to the Hub version (v1beta2).
func (src *RedisSentinel) ConvertTo(dstRaw conversion.Hub) error {
	dst := dstRaw.(*redisv1beta2.RedisSentinel)

	// ObjectMeta
	dst.ObjectMeta = src.ObjectMeta

	// Size
	dst.Spec.Size = src.Spec.Size
	// KubernetesConfig
	dst.Spec.KubernetesConfig.KubernetesConfig = src.Spec.KubernetesConfig.KubernetesConfig
	// RedisSentinelConfig
	if src.Spec.RedisSentinelConfig != nil {
		dst.Spec.RedisSentinelConfig = &redisv1beta2.RedisSentinelConfig{}
		dst.Spec.RedisSentinelConfig.RedisSentinelConfig = src.Spec.RedisSentinelConfig.RedisSentinelConfig
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
		dst.Spec.ReadinessProbe = &redisv1beta2.Probe{}
		dst.Spec.ReadinessProbe.Probe = src.Spec.ReadinessProbe.Probe
	}
	// LivenessProbe
	if src.Spec.LivenessProbe != nil {
		dst.Spec.ReadinessProbe = &redisv1beta2.Probe{}
		dst.Spec.LivenessProbe.Probe = src.Spec.LivenessProbe.Probe
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
func (dst *RedisSentinel) ConvertFrom(srcRaw conversion.Hub) error {
	src := srcRaw.(*redisv1beta2.RedisSentinel)

	// ObjectMeta
	dst.ObjectMeta = src.ObjectMeta

	// Size
	dst.Spec.Size = src.Spec.Size
	// KubernetesConfig
	dst.Spec.KubernetesConfig.KubernetesConfig = src.Spec.KubernetesConfig.KubernetesConfig
	// RedisSentinelConfig
	if src.Spec.RedisSentinelConfig != nil {
		dst.Spec.RedisSentinelConfig = &RedisSentinelConfig{}
		dst.Spec.RedisSentinelConfig.RedisSentinelConfig = src.Spec.RedisSentinelConfig.RedisSentinelConfig
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
		dst.Spec.ReadinessProbe = &Probe{}
		dst.Spec.ReadinessProbe.Probe = src.Spec.ReadinessProbe.Probe
	}
	// LivenessProbe
	if src.Spec.LivenessProbe != nil {
		dst.Spec.LivenessProbe = &Probe{}
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
