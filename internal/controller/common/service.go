package common

import (
	"strconv"
	"strings"
)

// GetHeadlessServiceNameFromPodName trims the trailing ordinal (e.g. "-0", "-1") from a pod name to
// derive the headless service name. If the pod name does not contain a trailing
// ordinal segment the original name is returned unchanged.
//
// Examples for different Redis modes:
// - RedisReplication: "redis-replication-0" -> "redis-replication-headless"
// - RedisCluster Leader: "redis-cluster-leader-0" -> "redis-cluster-leader-headless"
// - RedisCluster Follower: "redis-cluster-follower-1" -> "redis-cluster-follower-headless"
// - RedisSentinel: "redis-sentinel-sentinel-0" -> "redis-sentinel-sentinel-headless"
func GetHeadlessServiceNameFromPodName(podName string) string {
	// Find the last dash in the pod name. If there is none, return the whole name.
	idx := strings.LastIndex(podName, "-")
	if idx == -1 {
		return podName
	}
	// Check whether the suffix after the last dash is a number (the StatefulSet
	// ordinal). If it is, trim it to get the service name; otherwise return the
	// original pod name.
	if _, err := strconv.Atoi(podName[idx+1:]); err == nil {
		return podName[:idx] + "-headless"
	}
	return podName
}
