package v1beta2

import (
	"fmt"
	"strconv"
)

func (cr *RedisReplication) EnableSentinel() bool {
	return cr != nil && cr.Spec.Sentinel != nil && cr.Spec.Sentinel.Size > 0
}

// UseExternalMaster returns true when slave-only mode with an external master is active.
// Presence of a non-nil ExternalMaster spec field activates this mode.
func (cr *RedisReplication) UseExternalMaster() bool {
	return cr != nil && cr.Spec.ExternalMaster != nil
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

func (cr *RedisReplication) SentinelStatefulSet() string {
	return cr.Name + "-s"
}

func (cr *RedisReplication) RedisStatefulSet() string {
	return cr.Name
}

func (cr *RedisReplication) SentinelHLService() string {
	return cr.Name + "-s-hl"
}

func (cr *RedisReplication) MasterService() string {
	return cr.Name + "-master"
}

// GetConnectionInfo returns connection info for clients based on the mode.
// The dnsDomain parameter should be the cluster DNS domain (e.g., "cluster.local").
func (cr *RedisReplication) GetConnectionInfo(dnsDomain string) *ConnectionInfo {
	if cr.UseExternalMaster() {
		// In slave-only mode return the external master endpoint so ConnectionInfo
		// is consistent with the "write endpoint" semantics used in other modes.
		return &ConnectionInfo{
			Host: cr.Spec.ExternalMaster.Host,
			Port: int(cr.GetExternalMasterPort()),
		}
	}
	if cr.EnableSentinel() {
		return &ConnectionInfo{
			Host:       fmt.Sprintf("%s.%s.svc.%s", cr.SentinelHLService(), cr.Namespace, dnsDomain),
			Port:       26379,
			MasterName: "mymaster",
		}
	}
	return &ConnectionInfo{
		Host: fmt.Sprintf("%s.%s.svc.%s", cr.MasterService(), cr.Namespace, dnsDomain),
		Port: 6379,
	}
}
