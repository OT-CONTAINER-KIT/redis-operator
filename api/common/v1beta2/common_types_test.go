package api

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"k8s.io/utils/ptr"
)

func TestKubernetesConfig_ShouldCreateAdditionalService(t *testing.T) {
	tests := []struct {
		name     string
		config   *KubernetesConfig
		expected bool
	}{
		{
			name:     "nil config",
			config:   nil,
			expected: true,
		},
		{
			name:     "empty config",
			config:   &KubernetesConfig{},
			expected: true,
		},
		{
			name: "nil service",
			config: &KubernetesConfig{
				Service: nil,
			},
			expected: true,
		},
		{
			name: "nil additional",
			config: &KubernetesConfig{
				Service: &ServiceConfig{
					Additional: nil,
				},
			},
			expected: true,
		},
		{
			name: "nil enabled",
			config: &KubernetesConfig{
				Service: &ServiceConfig{
					Additional: &Service{
						Enabled: nil,
					},
				},
			},
			expected: true,
		},
		{
			name: "enabled true",
			config: &KubernetesConfig{
				Service: &ServiceConfig{
					Additional: &Service{
						Enabled: ptr.To(true),
					},
				},
			},
			expected: true,
		},
		{
			name: "enabled false",
			config: &KubernetesConfig{
				Service: &ServiceConfig{
					Additional: &Service{
						Enabled: ptr.To(false),
					},
				},
			},
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.config == nil {
				assert.True(t, (&KubernetesConfig{}).ShouldCreateAdditionalService())
				return
			}
			assert.Equal(t, tt.expected, tt.config.ShouldCreateAdditionalService())
		})
	}
}
