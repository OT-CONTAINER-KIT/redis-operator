package v1beta2

import (
	"testing"

	"github.com/stretchr/testify/assert"
	corev1 "k8s.io/api/core/v1"
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

func TestACLConfig_PersistentVolumeClaim(t *testing.T) {
	tests := []struct {
		name                     string
		aclConfig                *ACLConfig
		expectedPVCName          *string
		expectedSecretConfigured bool
	}{
		{
			name:                     "nil ACL config",
			aclConfig:                nil,
			expectedPVCName:          nil,
			expectedSecretConfigured: false,
		},
		{
			name:                     "empty ACL config",
			aclConfig:                &ACLConfig{},
			expectedPVCName:          nil,
			expectedSecretConfigured: false,
		},
		{
			name: "ACL with secret only",
			aclConfig: &ACLConfig{
				Secret: &corev1.SecretVolumeSource{
					SecretName: "redis-acl-secret",
				},
			},
			expectedPVCName:          nil,
			expectedSecretConfigured: true,
		},
		{
			name: "ACL with PVC only",
			aclConfig: &ACLConfig{
				PersistentVolumeClaim: ptr.To("redis-acl-pvc"),
			},
			expectedPVCName:          ptr.To("redis-acl-pvc"),
			expectedSecretConfigured: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.aclConfig == nil {
				assert.Nil(t, tt.aclConfig)
				return
			}

			// Verify PVC configuration
			if tt.expectedPVCName != nil {
				assert.NotNil(t, tt.aclConfig.PersistentVolumeClaim)
				assert.Equal(t, *tt.expectedPVCName, *tt.aclConfig.PersistentVolumeClaim)
			} else {
				assert.Nil(t, tt.aclConfig.PersistentVolumeClaim)
			}

			// Verify Secret configuration
			if tt.expectedSecretConfigured {
				assert.NotNil(t, tt.aclConfig.Secret)
			} else {
				assert.Nil(t, tt.aclConfig.Secret)
			}
		})
	}
}

func TestACLConfig_BothSourcesConfigured(t *testing.T) {
	// Test scenario where both Secret and PVC are configured
	// Validation should fail
	aclConfig := &ACLConfig{
		Secret: &corev1.SecretVolumeSource{
			SecretName: "redis-acl-secret",
		},
		PersistentVolumeClaim: ptr.To("redis-acl-pvc"),
	}

	// Both should be settable (struct level)
	assert.NotNil(t, aclConfig.Secret)
	assert.NotNil(t, aclConfig.PersistentVolumeClaim)
	assert.Equal(t, "redis-acl-secret", aclConfig.Secret.SecretName)
	assert.Equal(t, "redis-acl-pvc", *aclConfig.PersistentVolumeClaim)

	// But validation should fail
	err := aclConfig.Validate()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "only one of 'secret' or 'persistentVolumeClaim' can be specified")
}

func TestACLConfig_Validate(t *testing.T) {
	tests := []struct {
		name      string
		aclConfig *ACLConfig
		wantErr   bool
		errMsg    string
	}{
		{
			name:      "nil config is valid",
			aclConfig: nil,
			wantErr:   false,
		},
		{
			name:      "empty config is valid",
			aclConfig: &ACLConfig{},
			wantErr:   false,
		},
		{
			name: "only secret is valid",
			aclConfig: &ACLConfig{
				Secret: &corev1.SecretVolumeSource{
					SecretName: "redis-acl-secret",
				},
			},
			wantErr: false,
		},
		{
			name: "only PVC is valid",
			aclConfig: &ACLConfig{
				PersistentVolumeClaim: ptr.To("redis-acl-pvc"),
			},
			wantErr: false,
		},
		{
			name: "both secret and PVC is invalid",
			aclConfig: &ACLConfig{
				Secret: &corev1.SecretVolumeSource{
					SecretName: "redis-acl-secret",
				},
				PersistentVolumeClaim: ptr.To("redis-acl-pvc"),
			},
			wantErr: true,
			errMsg:  "only one of 'secret' or 'persistentVolumeClaim' can be specified",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.aclConfig.Validate()
			if tt.wantErr {
				assert.Error(t, err)
				if tt.errMsg != "" {
					assert.Contains(t, err.Error(), tt.errMsg)
				}
			} else {
				assert.NoError(t, err)
			}
		})
	}
}
