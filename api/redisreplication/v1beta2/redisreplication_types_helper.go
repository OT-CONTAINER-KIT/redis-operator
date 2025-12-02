package v1beta2

func (cr *RedisReplication) EnableSentinel() bool {
	return cr != nil && cr.Spec.Sentinel != nil && cr.Spec.Sentinel.Size > 0
}

func (cr *RedisReplication) SentinelStatefulSet() string {
	return cr.Name + "-s"
}

func (cr *RedisReplication) RedisStatefulSet() string {
	return cr.Name
}

func (cr *RedisReplication) SentinelHLService() string {
	return cr.Name + "-s-hl"
}
