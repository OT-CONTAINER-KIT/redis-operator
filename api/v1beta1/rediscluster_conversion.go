package v1beta1

import (
	redisv1beta2 "github.com/OT-CONTAINER-KIT/redis-operator/api/v1beta2"
	"sigs.k8s.io/controller-runtime/pkg/conversion"
)

// ConvertTo converts this RedisCluster to the Hub version (v1beta2).
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
	dst.Spec.RedisLeader.CommonAttributes = src.Spec.RedisLeader.CommonAttributes
	// RedisFollower
	dst.Spec.RedisFollower.CommonAttributes = src.Spec.RedisFollower.CommonAttributes
	// RedisExporter
	if src.Spec.RedisExporter != nil {
		dst.Spec.RedisExporter.RedisExporter = src.Spec.RedisExporter.RedisExporter
	}
	// Storage
	if src.Spec.Storage != nil {
		// Note : Add the Check the creation of node-conf later
		dst.Spec.Storage.CommonAttributes = src.Spec.Storage.CommonAttributes
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
	dst.Spec.RedisLeader.CommonAttributes = src.Spec.RedisLeader.CommonAttributes
	// RedisFollower
	dst.Spec.RedisFollower.CommonAttributes = src.Spec.RedisFollower.CommonAttributes
	// RedisExporter
	if src.Spec.RedisExporter != nil {
		dst.Spec.RedisExporter.RedisExporter = src.Spec.RedisExporter.RedisExporter
	}
	// Storage
	if src.Spec.Storage != nil {
		dst.Spec.Storage.CommonAttributes = src.Spec.Storage.CommonAttributes
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
