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

package env

import (
	"os"
	"strconv"
	"strings"
)

// Environment variable keys
const (
	// WatchNamespaceEnv defines the namespaces that the operator will watch for resources
	WatchNamespaceEnv = "WATCH_NAMESPACE"

	// MaxConcurrentReconcilesEnv defines the maximum number of concurrent reconciles
	MaxConcurrentReconcilesEnv = "MAX_CONCURRENT_RECONCILES"

	// EnableWebhooksEnv defines whether webhooks are enabled
	EnableWebhooksEnv = "ENABLE_WEBHOOKS"

	// FeatureGatesEnv defines feature gates for alpha/experimental features
	FeatureGatesEnv = "FEATURE_GATES"

	// OperatorImageEnv defines the image of the operator
	OperatorImageEnv = "OPERATOR_IMAGE"
)

// GetOperatorImage returns the image of the operator
func GetOperatorImage() string {
	return os.Getenv(OperatorImageEnv)
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

// IsWebhookEnabled returns true if webhooks are enabled
func IsWebhookEnabled() bool {
	return os.Getenv(EnableWebhooksEnv) != "false"
}

// GetFeatureGates returns feature gates string
func GetFeatureGates() string {
	return os.Getenv(FeatureGatesEnv)
}
