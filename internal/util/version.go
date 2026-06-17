package util

import (
	"strconv"
	"strings"
)

// IsRedisVersion7OrNewer reports whether the supplied Redis major-version
// string denotes Redis 7 or any newer major (v7, v8, ...).
//
// The hostname-based cluster discovery features the operator relies on
// (cluster-announce-hostname, FQDN cluster endpoints,
// cluster-preferred-endpoint-type hostname and CLUSTER ADDSLOTSRANGE) were
// introduced in Redis 7 and remain available in every later major. The
// operator must therefore gate them on "7 or newer" rather than an exact
// "v7" match; otherwise a perfectly valid clusterVersion such as "v8"
// silently regresses to the legacy Redis 6 IP-based behaviour.
//
// The value is expected to look like "v7" — the form stored in
// RedisCluster.spec.clusterVersion and propagated to the agent through the
// REDIS_MAJOR_VERSION environment variable — but a leading "v" is optional and
// any trailing minor/patch component (e.g. "v7.2.1") is ignored. An empty or
// unparseable value is treated as the default (v7), so callers fall back to
// the modern behaviour instead of silently regressing.
func IsRedisVersion7OrNewer(version string) bool {
	major, ok := parseRedisMajorVersion(version)
	if !ok {
		return true
	}
	return major >= 7
}

// parseRedisMajorVersion extracts the leading integer major component from a
// version string, tolerating an optional "v"/"V" prefix and ignoring any
// ".minor.patch" suffix. The bool is false when no leading major number can be
// parsed, so callers can apply their own default for empty/garbage input.
func parseRedisMajorVersion(version string) (int, bool) {
	v := strings.TrimSpace(version)
	v = strings.TrimPrefix(v, "v")
	v = strings.TrimPrefix(v, "V")

	end := 0
	for end < len(v) && v[end] >= '0' && v[end] <= '9' {
		end++
	}
	if end == 0 {
		return 0, false
	}
	major, err := strconv.Atoi(v[:end])
	if err != nil {
		return 0, false
	}
	return major, true
}
