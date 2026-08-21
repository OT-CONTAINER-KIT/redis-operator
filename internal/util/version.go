package util

import "strconv"

// redisDefaultMajorVersion is the major version assumed when the configured
// value cannot be parsed. It matches the `v7` default of
// RedisCluster.spec.clusterVersion, so unset or malformed values keep the
// behaviour existing clusters already rely on.
const redisDefaultMajorVersion = 7

// RedisMajorVersion extracts the major version number from a value of the form
// `v7`, `7`, `v7.2` or `v8.0.1`. Values that do not start with a digit (after an
// optional leading `v`) fall back to redisDefaultMajorVersion.
func RedisMajorVersion(version string) int {
	v := version
	if len(v) > 0 && (v[0] == 'v' || v[0] == 'V') {
		v = v[1:]
	}
	i := 0
	for i < len(v) && v[i] >= '0' && v[i] <= '9' {
		i++
	}
	if i == 0 {
		return redisDefaultMajorVersion
	}
	major, err := strconv.Atoi(v[:i])
	if err != nil {
		// Only reachable for absurdly long digit runs; treat as the default.
		return redisDefaultMajorVersion
	}
	return major
}

// IsRedisVersionAtLeastV7 reports whether the given version string denotes
// Redis 7 or newer. Features introduced in Redis 7.0 — cluster-announce-hostname,
// cluster-preferred-endpoint-type and CLUSTER ADDSLOTSRANGE — are available for
// every later major version too, so they must be gated on `>= 7` rather than on
// an exact `v7` match. Otherwise a user honestly setting `clusterVersion: v8`
// would silently get the Redis 6 code path.
func IsRedisVersionAtLeastV7(version string) bool {
	return RedisMajorVersion(version) >= 7
}
