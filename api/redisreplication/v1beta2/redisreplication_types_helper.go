package v1beta2

import (
	"fmt"
	"strconv"
)

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

// SentinelMasterName returns the master group name monitored by the embedded
// Sentinel. The embedded Sentinel always monitors a single group named
// "mymaster" (see internal/agent/bootstrap/sentinel/config.go, which falls back
// to "mymaster" because the embedded Sentinel env never sets MASTER_GROUP_NAME).
func (cr *RedisReplication) SentinelMasterName() string {
	return "mymaster"
}

func (cr *RedisReplication) MasterService() string {
	return cr.Name + "-master"
}

// UseExternalMaster returns true when the deployment is configured as a passive
// replica of an external Redis master.
func (cr *RedisReplication) UseExternalMaster() bool {
	return cr != nil && cr.Spec.ExternalMaster != nil && cr.Spec.ExternalMaster.Host != ""
}

// GetExternalMasterPort returns the configured external master port, defaulting to 6379.
func (cr *RedisReplication) GetExternalMasterPort() int32 {
	if cr.Spec.ExternalMaster != nil && cr.Spec.ExternalMaster.Port != nil {
		return *cr.Spec.ExternalMaster.Port
	}
	return 6379
}

// GetExternalMasterEndpoint returns the "host:port" string of the external master.
func (cr *RedisReplication) GetExternalMasterEndpoint() string {
	return cr.Spec.ExternalMaster.Host + ":" + strconv.Itoa(int(cr.GetExternalMasterPort()))
}

// GetConnectionInfo returns connection info for clients based on the mode.
// The dnsDomain parameter should be the cluster DNS domain (e.g., "cluster.local").
func (cr *RedisReplication) GetConnectionInfo(dnsDomain string) *ConnectionInfo {
	if cr.UseExternalMaster() {
		return &ConnectionInfo{
			Host: cr.Spec.ExternalMaster.Host,
			Port: int(cr.GetExternalMasterPort()),
		}
	}
	if cr.EnableSentinel() {
		return &ConnectionInfo{
			Host:       fmt.Sprintf("%s.%s.svc.%s", cr.SentinelHLService(), cr.Namespace, dnsDomain),
			Port:       26379,
			MasterName: cr.SentinelMasterName(),
		}
	}
	return &ConnectionInfo{
		Host: fmt.Sprintf("%s.%s.svc.%s", cr.MasterService(), cr.Namespace, dnsDomain),
		Port: 6379,
	}
}
