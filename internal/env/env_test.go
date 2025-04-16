package env

import (
	"os"
	"reflect"
	"testing"
)

func TestGetWatchNamespaces(t *testing.T) {
	tests := []struct {
		name          string
		envValue      string
		expectedValue []string
	}{
		{
			name:          "empty namespace",
			envValue:      "",
			expectedValue: nil,
		},
		{
			name:          "single namespace",
			envValue:      "default",
			expectedValue: []string{"default"},
		},
		{
			name:          "multiple namespaces",
			envValue:      "default,kube-system",
			expectedValue: []string{"default", "kube-system"},
		},
		{
			name:          "namespaces with spaces",
			envValue:      "  default , kube-system  ",
			expectedValue: []string{"default", "kube-system"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Set environment variable
			if tt.envValue != "" {
				os.Setenv(WatchNamespaceEnv, tt.envValue)
				defer os.Unsetenv(WatchNamespaceEnv)
			} else {
				os.Unsetenv(WatchNamespaceEnv)
			}

			// Get actual namespaces
			actualNamespaces := GetWatchNamespaces()

			// Compare results
			if !reflect.DeepEqual(actualNamespaces, tt.expectedValue) {
				t.Errorf("GetWatchNamespaces() = %v, want %v", actualNamespaces, tt.expectedValue)
			}
		})
	}
}

func TestGetMaxConcurrentReconciles(t *testing.T) {
	tests := []struct {
		name          string
		envValue      string
		defaultValue  int
		expectedValue int
	}{
		{
			name:          "empty value with default",
			envValue:      "",
			defaultValue:  1,
			expectedValue: 1,
		},
		{
			name:          "valid value",
			envValue:      "5",
			defaultValue:  1,
			expectedValue: 5,
		},
		{
			name:          "invalid value with default",
			envValue:      "invalid",
			defaultValue:  3,
			expectedValue: 3,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Set environment variable
			if tt.envValue != "" {
				os.Setenv(MaxConcurrentReconcilesEnv, tt.envValue)
				defer os.Unsetenv(MaxConcurrentReconcilesEnv)
			} else {
				os.Unsetenv(MaxConcurrentReconcilesEnv)
			}

			// Get actual value
			actualValue := GetMaxConcurrentReconciles(tt.defaultValue)

			// Compare results
			if actualValue != tt.expectedValue {
				t.Errorf("GetMaxConcurrentReconciles() = %v, want %v", actualValue, tt.expectedValue)
			}
		})
	}
}

func TestIsWebhookEnabled(t *testing.T) {
	tests := []struct {
		name          string
		envValue      string
		expectedValue bool
	}{
		{
			name:          "empty value (default enabled)",
			envValue:      "",
			expectedValue: true,
		},
		{
			name:          "explicitly disabled",
			envValue:      "false",
			expectedValue: false,
		},
		{
			name:          "explicitly enabled",
			envValue:      "true",
			expectedValue: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Set environment variable
			if tt.envValue != "" {
				os.Setenv(EnableWebhooksEnv, tt.envValue)
				defer os.Unsetenv(EnableWebhooksEnv)
			} else {
				os.Unsetenv(EnableWebhooksEnv)
			}

			// Get actual value
			actualValue := IsWebhookEnabled()

			// Compare results
			if actualValue != tt.expectedValue {
				t.Errorf("IsWebhookEnabled() = %v, want %v", actualValue, tt.expectedValue)
			}
		})
	}
}

func TestGetFeatureGates(t *testing.T) {
	tests := []struct {
		name          string
		envValue      string
		expectedValue string
	}{
		{
			name:          "empty value",
			envValue:      "",
			expectedValue: "",
		},
		{
			name:          "with feature gates",
			envValue:      "GenerateConfigInInitContainer=true",
			expectedValue: "GenerateConfigInInitContainer=true",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Set environment variable
			if tt.envValue != "" {
				os.Setenv(FeatureGatesEnv, tt.envValue)
				defer os.Unsetenv(FeatureGatesEnv)
			} else {
				os.Unsetenv(FeatureGatesEnv)
			}

			// Get actual value
			actualValue := GetFeatureGates()

			// Compare results
			if actualValue != tt.expectedValue {
				t.Errorf("GetFeatureGates() = %v, want %v", actualValue, tt.expectedValue)
			}
		})
	}
}
