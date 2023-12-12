package v1beta2

// SetDefault sets default values for the RedisCluster object.
func (r *RedisCluster) SetDefault() {
	if r.Spec.Port == 0 {
		r.Spec.Port = 6379
	}
}
