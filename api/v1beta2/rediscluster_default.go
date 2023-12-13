package v1beta2

import "k8s.io/utils/pointer"

// SetDefault sets default values for the RedisCluster object.
func (r *RedisCluster) SetDefault() {
	if r.Spec.Port == nil {
		r.Spec.Port = pointer.Int(6379)
	}
}
