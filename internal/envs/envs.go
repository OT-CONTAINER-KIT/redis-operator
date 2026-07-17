/*
Copyright 2023 Opstree Solutions.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package envs

import (
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/OT-CONTAINER-KIT/redis-operator/internal/util"
)

// Environment variable keys
const (
	// WatchNamespaceEnv defines the namespaces that the operator will watch for resources
	WatchNamespaceEnv = "WATCH_NAMESPACE"

	// MaxConcurrentReconcilesEnv defines the maximum number of concurrent reconciles
	MaxConcurrentReconcilesEnv = "MAX_CONCURRENT_RECONCILES"

	// ExecCommandTimeoutEnv defines the timeout for commands executed inside redis pods via the Kubernetes exec API
	ExecCommandTimeoutEnv = "EXEC_COMMAND_TIMEOUT"

	// EnableWebhooksEnv defines whether webhooks are enabled
	EnableWebhooksEnv = "ENABLE_WEBHOOKS"

	// FeatureGatesEnv defines feature gates for alpha/experimental features
	FeatureGatesEnv = "FEATURE_GATES"

	// InitContainerImageEnv defines the image used for init containers
	InitContainerImageEnv = "INIT_CONTAINER_IMAGE"

	// ServiceDNSDomain defines the DNS domain suffix for Kubernetes services
	ServiceDNSDomain = "SERVICE_DNS_DOMAIN"

	// RedisPodReachAttemptsEnv defines the number of attempts to reach a Redis pod during replication reconciliation
	RedisPodReachAttemptsEnv = "REDIS_POD_REACH_ATTEMPTS"
)

var (
	initContainerImage     string
	initContainerImageOnce sync.Once
)

// GetInitContainerImage returns the image to use for init containers.
func GetInitContainerImage() string {
	initContainerImageOnce.Do(func() {
		val := os.Getenv(InitContainerImageEnv)
		initContainerImage = util.Coalesce(val, "quay.io/opstree/redis-operator:latest")
	})
	return initContainerImage
}

// GetServiceDNSDomain returns the Kubernetes service DNS domain suffix.
func GetServiceDNSDomain() string {
	return util.Coalesce(os.Getenv(ServiceDNSDomain), "cluster.local")
}

// GetWatchNamespaces returns a list of namespaces that the operator should watch
func GetWatchNamespaces() []string {
	namespaceEnvValue := strings.TrimSpace(os.Getenv(WatchNamespaceEnv))
	if namespaceEnvValue == "" {
		return nil
	}

	var namespaces []string
	for _, ns := range strings.Split(namespaceEnvValue, ",") {
		if ns = strings.TrimSpace(ns); ns != "" {
			namespaces = append(namespaces, ns)
		}
	}
	return namespaces
}

// GetMaxConcurrentReconciles returns the maximum number of concurrent reconciles
func GetMaxConcurrentReconciles(defaultValue int) int {
	if valueStr := os.Getenv(MaxConcurrentReconcilesEnv); valueStr != "" {
		if value, err := strconv.Atoi(valueStr); err == nil {
			return value
		}
	}
	return defaultValue
}

// GetExecCommandTimeout returns the timeout applied to commands executed inside redis pods
// via the Kubernetes exec API.
func GetExecCommandTimeout(defaultValue time.Duration) time.Duration {
	if valueStr := os.Getenv(ExecCommandTimeoutEnv); valueStr != "" {
		if value, err := time.ParseDuration(valueStr); err == nil && value > 0 {
			return value
		}
	}
	return defaultValue
}

// GetRedisPodReachAttempts returns the number of attempts to reach a Redis pod during replication reconciliation.
func GetRedisPodReachAttempts(defaultValue int) int {
	if valueStr := strings.TrimSpace(os.Getenv(RedisPodReachAttemptsEnv)); valueStr != "" {
		if value, err := strconv.Atoi(valueStr); err == nil && value > 0 {
			return value
		}
	}
	return defaultValue
}

// IsWebhookEnabled returns true if webhooks are enabled
func IsWebhookEnabled() bool {
	return os.Getenv(EnableWebhooksEnv) != "false"
}

// GetFeatureGates returns feature gates string
func GetFeatureGates() string {
	return os.Getenv(FeatureGatesEnv)
}
