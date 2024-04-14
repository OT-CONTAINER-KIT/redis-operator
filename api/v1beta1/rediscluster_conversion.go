package v1beta1

import (
	redisv1beta2 "github.com/teocns/redis-operator/api/v1beta2"
	"sigs.k8s.io/controller-runtime/pkg/conversion"
)

// ConvertTo converts this RedisCluster to the Hub version (v1beta2) from the current version (v1beta1)

func (src *RedisCluster) ConvertTo(dstRaw conversion.Hub) error {
	dst := dstRaw.(*redisv1beta2.RedisCluster)
	// ObjectMeta
	dst.ObjectMeta = src.ObjectMeta

	// Size
	if src.Spec.Size != nil {
		dst.Spec.Size = src.Spec.Size
	}
	// KubernetesConfig
	dst.Spec.KubernetesConfig.KubernetesConfig = src.Spec.KubernetesConfig.KubernetesConfig
	// ClusterVersion
	if src.Spec.ClusterVersion != nil {
		dst.Spec.ClusterVersion = src.Spec.ClusterVersion
	}
	// RedisLeader
	dst.Spec.RedisLeader = redisv1beta2.RedisLeader{}
	if src.Spec.RedisLeader.Replicas != nil {
		dst.Spec.RedisLeader.Replicas = src.Spec.RedisLeader.Replicas
	}
	if src.Spec.RedisLeader.RedisConfig != nil {
		dst.Spec.RedisLeader.RedisConfig = src.Spec.RedisLeader.RedisConfig
	}
	if src.Spec.RedisLeader.Affinity != nil {
		dst.Spec.RedisLeader.Affinity = src.Spec.RedisLeader.Affinity
	}
	if src.Spec.RedisLeader.PodDisruptionBudget != nil {
		dst.Spec.RedisLeader.PodDisruptionBudget = src.Spec.RedisLeader.PodDisruptionBudget
	}
	if src.Spec.RedisLeader.ReadinessProbe != nil {
		dst.Spec.RedisLeader.ReadinessProbe = src.Spec.RedisLeader.ReadinessProbe
	}
	if src.Spec.RedisLeader.LivenessProbe != nil {
		dst.Spec.RedisLeader.LivenessProbe = src.Spec.RedisLeader.LivenessProbe
	}
	if src.Spec.RedisLeader.Tolerations != nil {
		dst.Spec.RedisLeader.Tolerations = src.Spec.RedisLeader.Tolerations
	}
	if src.Spec.RedisLeader.NodeSelector != nil {
		dst.Spec.RedisLeader.NodeSelector = src.Spec.RedisLeader.NodeSelector
	}

	// RedisFollower
	dst.Spec.RedisFollower = redisv1beta2.RedisFollower{}
	if src.Spec.RedisFollower.Replicas != nil {
		dst.Spec.RedisFollower.Replicas = src.Spec.RedisFollower.Replicas
	}
	if src.Spec.RedisFollower.RedisConfig != nil {
		dst.Spec.RedisFollower.RedisConfig = src.Spec.RedisFollower.RedisConfig
	}
	if src.Spec.RedisFollower.Affinity != nil {
		dst.Spec.RedisFollower.Affinity = src.Spec.RedisFollower.Affinity
	}
	if src.Spec.RedisFollower.PodDisruptionBudget != nil {
		dst.Spec.RedisFollower.PodDisruptionBudget = src.Spec.RedisFollower.PodDisruptionBudget
	}
	if src.Spec.RedisFollower.ReadinessProbe != nil {
		dst.Spec.RedisFollower.ReadinessProbe = src.Spec.RedisFollower.ReadinessProbe
	}
	if src.Spec.RedisFollower.LivenessProbe != nil {
		dst.Spec.RedisFollower.LivenessProbe = src.Spec.RedisFollower.LivenessProbe
	}
	if src.Spec.RedisFollower.Tolerations != nil {
		dst.Spec.RedisFollower.Tolerations = src.Spec.RedisFollower.Tolerations
	}
	if src.Spec.RedisFollower.NodeSelector != nil {
		dst.Spec.RedisFollower.NodeSelector = src.Spec.RedisFollower.NodeSelector
	}
	// RedisExporter
	if src.Spec.RedisExporter != nil {
		dst.Spec.RedisExporter = &redisv1beta2.RedisExporter{}
		dst.Spec.RedisExporter.RedisExporter = src.Spec.RedisExporter.RedisExporter
	}
	// Storage-v1bet1 >> ClusterStorage-v1beta2
	if src.Spec.Storage != nil {
		// Note : Add the Check the creation of node-conf later
		dst.Spec.Storage = &redisv1beta2.ClusterStorage{}
		dst.Spec.Storage.VolumeClaimTemplate = src.Spec.Storage.VolumeClaimTemplate
		dst.Spec.Storage.VolumeMount = src.Spec.Storage.VolumeMount
	}
	// SecurityContext >> PodSecurityContext
	if src.Spec.SecurityContext != nil {
		dst.Spec.PodSecurityContext = src.Spec.SecurityContext
	}
	// PriorityClassName
	if src.Spec.PriorityClassName != "" {
		dst.Spec.PriorityClassName = src.Spec.PriorityClassName
	}
	// Resources
	if src.Spec.Resources != nil {
		dst.Spec.Resources = src.Spec.Resources
	}
	// TLS
	if src.Spec.TLS != nil {
		dst.Spec.TLS = &redisv1beta2.TLSConfig{}
		dst.Spec.TLS.TLSConfig = src.Spec.TLS.TLSConfig
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
	// PersistenceEnabled
	if src.Spec.PersistenceEnabled != nil {
		dst.Spec.PersistenceEnabled = src.Spec.PersistenceEnabled
	}
	return nil
}

// ConvertFrom converts from the Hub version (vbeta2) to this version.
func (dst *RedisCluster) ConvertFrom(srcRaw conversion.Hub) error {
	src := srcRaw.(*redisv1beta2.RedisCluster)

	// ObjectMeta
	dst.ObjectMeta = src.ObjectMeta

	// Size
	if src.Spec.Size != nil {
		dst.Spec.Size = src.Spec.Size
	}
	// KubernetesConfig
	dst.Spec.KubernetesConfig.KubernetesConfig = src.Spec.KubernetesConfig.KubernetesConfig
	// ClusterVersion
	if src.Spec.ClusterVersion != nil {
		dst.Spec.ClusterVersion = src.Spec.ClusterVersion
	}
	// RedisLeader
	dst.Spec.RedisLeader = RedisLeader{}
	if src.Spec.RedisLeader.Replicas != nil {
		dst.Spec.RedisLeader.Replicas = src.Spec.RedisLeader.Replicas
	}
	if src.Spec.RedisLeader.RedisConfig != nil {
		dst.Spec.RedisLeader.RedisConfig = src.Spec.RedisLeader.RedisConfig
	}
	if src.Spec.RedisLeader.Affinity != nil {
		dst.Spec.RedisLeader.Affinity = src.Spec.RedisLeader.Affinity
	}
	if src.Spec.RedisLeader.PodDisruptionBudget != nil {
		dst.Spec.RedisLeader.PodDisruptionBudget = src.Spec.RedisLeader.PodDisruptionBudget
	}
	if src.Spec.RedisLeader.ReadinessProbe != nil {
		dst.Spec.RedisLeader.ReadinessProbe = src.Spec.RedisLeader.ReadinessProbe
	}
	if src.Spec.RedisLeader.LivenessProbe != nil {
		dst.Spec.RedisLeader.LivenessProbe = src.Spec.RedisLeader.LivenessProbe
	}
	if src.Spec.RedisLeader.Tolerations != nil {
		dst.Spec.RedisLeader.Tolerations = src.Spec.RedisLeader.Tolerations
	}
	if src.Spec.RedisLeader.NodeSelector != nil {
		dst.Spec.RedisLeader.NodeSelector = src.Spec.RedisLeader.NodeSelector
	}

	// RedisFollower
	dst.Spec.RedisFollower = RedisFollower{}
	if src.Spec.RedisFollower.Replicas != nil {
		dst.Spec.RedisFollower.Replicas = src.Spec.RedisFollower.Replicas
	}
	if src.Spec.RedisFollower.RedisConfig != nil {
		dst.Spec.RedisFollower.RedisConfig = src.Spec.RedisFollower.RedisConfig
	}
	if src.Spec.RedisFollower.Affinity != nil {
		dst.Spec.RedisFollower.Affinity = src.Spec.RedisFollower.Affinity
	}
	if src.Spec.RedisFollower.PodDisruptionBudget != nil {
		dst.Spec.RedisFollower.PodDisruptionBudget = src.Spec.RedisFollower.PodDisruptionBudget
	}
	if src.Spec.RedisFollower.ReadinessProbe != nil {
		dst.Spec.RedisFollower.ReadinessProbe = src.Spec.RedisFollower.ReadinessProbe
	}
	if src.Spec.RedisFollower.LivenessProbe != nil {
		dst.Spec.RedisFollower.LivenessProbe = src.Spec.RedisFollower.LivenessProbe
	}
	if src.Spec.RedisFollower.Tolerations != nil {
		dst.Spec.RedisFollower.Tolerations = src.Spec.RedisFollower.Tolerations
	}
	if src.Spec.RedisFollower.NodeSelector != nil {
		dst.Spec.RedisFollower.NodeSelector = src.Spec.RedisFollower.NodeSelector
	}
	// RedisExporter
	if src.Spec.RedisExporter != nil {
		dst.Spec.RedisExporter = &RedisExporter{}
		dst.Spec.RedisExporter.RedisExporter = src.Spec.RedisExporter.RedisExporter
	}
	// ClusterStorage(v1beta2) >> Storage(v1beta1)
	if src.Spec.Storage != nil {
		dst.Spec.Storage = &Storage{}
		dst.Spec.Storage.VolumeClaimTemplate = src.Spec.Storage.VolumeClaimTemplate
		dst.Spec.Storage.VolumeMount = src.Spec.Storage.VolumeMount
	}
	//  PodSecurityContext >> SecurityContext
	if src.Spec.PodSecurityContext != nil {
		dst.Spec.SecurityContext = src.Spec.PodSecurityContext
	}
	// PriorityClassName
	if src.Spec.PriorityClassName != "" {
		dst.Spec.PriorityClassName = src.Spec.PriorityClassName
	}
	// Resources
	if src.Spec.Resources != nil {
		dst.Spec.Resources = src.Spec.Resources
	}
	// TLS
	if src.Spec.TLS != nil {
		dst.Spec.TLS = &TLSConfig{}
		dst.Spec.TLS.TLSConfig = src.Spec.TLS.TLSConfig
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
	// PersistenceEnabled
	if src.Spec.PersistenceEnabled != nil {
		dst.Spec.PersistenceEnabled = src.Spec.PersistenceEnabled
	}
	return nil
}
