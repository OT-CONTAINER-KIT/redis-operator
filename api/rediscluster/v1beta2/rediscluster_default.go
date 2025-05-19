package v1beta2

import "k8s.io/utils/ptr"

// SetDefault sets default values for the RedisCluster object.
func (r *RedisCluster) SetDefault() {
	if r.Spec.Port == nil {
		r.Spec.Port = ptr.To(6379)
	}
	if r.Spec.RedisExporter != nil && r.Spec.RedisExporter.Port == nil {
		r.Spec.RedisExporter.Port = ptr.To(9121)
	}
}
