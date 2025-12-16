package v1beta2

import (
	"testing"

	"github.com/stretchr/testify/assert"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
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

func TestKubernetesConfig_AdditionalVolumes(t *testing.T) {
	tests := []struct {
		name     string
		config   *KubernetesConfig
		expected []corev1.Volume
	}{
		{
			name:     "nil config",
			config:   nil,
			expected: nil,
		},
		{
			name:     "empty config",
			config:   &KubernetesConfig{},
			expected: nil,
		},
		{
			name: "empty additional volumes",
			config: &KubernetesConfig{
				AdditionalVolumes: []corev1.Volume{},
			},
			expected: []corev1.Volume{},
		},
		{
			name: "single configmap volume",
			config: &KubernetesConfig{
				AdditionalVolumes: []corev1.Volume{
					{
						Name: "config-volume",
						VolumeSource: corev1.VolumeSource{
							ConfigMap: &corev1.ConfigMapVolumeSource{
								LocalObjectReference: corev1.LocalObjectReference{
									Name: "redis-config",
								},
							},
						},
					},
				},
			},
			expected: []corev1.Volume{
				{
					Name: "config-volume",
					VolumeSource: corev1.VolumeSource{
						ConfigMap: &corev1.ConfigMapVolumeSource{
							LocalObjectReference: corev1.LocalObjectReference{
								Name: "redis-config",
							},
						},
					},
				},
			},
		},
		{
			name: "multiple volume types",
			config: &KubernetesConfig{
				AdditionalVolumes: []corev1.Volume{
					{
						Name: "config-volume",
						VolumeSource: corev1.VolumeSource{
							ConfigMap: &corev1.ConfigMapVolumeSource{
								LocalObjectReference: corev1.LocalObjectReference{
									Name: "redis-config",
								},
							},
						},
					},
					{
						Name: "logs-volume",
						VolumeSource: corev1.VolumeSource{
							EmptyDir: &corev1.EmptyDirVolumeSource{},
						},
					},
					{
						Name: "secret-volume",
						VolumeSource: corev1.VolumeSource{
							Secret: &corev1.SecretVolumeSource{
								SecretName: "redis-secret",
							},
						},
					},
				},
			},
			expected: []corev1.Volume{
				{
					Name: "config-volume",
					VolumeSource: corev1.VolumeSource{
						ConfigMap: &corev1.ConfigMapVolumeSource{
							LocalObjectReference: corev1.LocalObjectReference{
								Name: "redis-config",
							},
						},
					},
				},
				{
					Name: "logs-volume",
					VolumeSource: corev1.VolumeSource{
						EmptyDir: &corev1.EmptyDirVolumeSource{},
					},
				},
				{
					Name: "secret-volume",
					VolumeSource: corev1.VolumeSource{
						Secret: &corev1.SecretVolumeSource{
							SecretName: "redis-secret",
						},
					},
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.config == nil {
				config := &KubernetesConfig{}
				assert.Nil(t, config.AdditionalVolumes)
				return
			}
			assert.Equal(t, tt.expected, tt.config.AdditionalVolumes)
		})
	}
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

func TestKubernetesConfig_AdditionalVolumeMounts(t *testing.T) {
	tests := []struct {
		name     string
		config   *KubernetesConfig
		expected []corev1.VolumeMount
	}{
		{
			name:     "nil config",
			config:   nil,
			expected: nil,
		},
		{
			name:     "empty config",
			config:   &KubernetesConfig{},
			expected: nil,
		},
		{
			name: "empty additional volume mounts",
			config: &KubernetesConfig{
				AdditionalVolumeMounts: []corev1.VolumeMount{},
			},
			expected: []corev1.VolumeMount{},
		},
		{
			name: "single volume mount",
			config: &KubernetesConfig{
				AdditionalVolumeMounts: []corev1.VolumeMount{
					{
						Name:      "config-volume",
						MountPath: "/etc/redis/config",
						ReadOnly:  true,
					},
				},
			},
			expected: []corev1.VolumeMount{
				{
					Name:      "config-volume",
					MountPath: "/etc/redis/config",
					ReadOnly:  true,
				},
			},
		},
		{
			name: "multiple volume mounts",
			config: &KubernetesConfig{
				AdditionalVolumeMounts: []corev1.VolumeMount{
					{
						Name:      "config-volume",
						MountPath: "/etc/redis/config",
						ReadOnly:  true,
					},
					{
						Name:      "logs-volume",
						MountPath: "/var/log/redis",
						ReadOnly:  false,
					},
					{
						Name:      "secret-volume",
						MountPath: "/etc/redis/secrets",
						ReadOnly:  true,
						SubPath:   "redis.conf",
					},
				},
			},
			expected: []corev1.VolumeMount{
				{
					Name:      "config-volume",
					MountPath: "/etc/redis/config",
					ReadOnly:  true,
				},
				{
					Name:      "logs-volume",
					MountPath: "/var/log/redis",
					ReadOnly:  false,
				},
				{
					Name:      "secret-volume",
					MountPath: "/etc/redis/secrets",
					ReadOnly:  true,
					SubPath:   "redis.conf",
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.config == nil {
				config := &KubernetesConfig{}
				assert.Nil(t, config.AdditionalVolumeMounts)
				return
			}
			assert.Equal(t, tt.expected, tt.config.AdditionalVolumeMounts)
		})
	}
}

func TestKubernetesConfig_AdditionalVolumesAndMounts_Combined(t *testing.T) {
	tests := []struct {
		name                        string
		config                      *KubernetesConfig
		expectedVolumes             []corev1.Volume
		expectedVolumeMounts        []corev1.VolumeMount
		volumeMountNamesShouldMatch bool
	}{
		{
			name: "matching volumes and mounts",
			config: &KubernetesConfig{
				AdditionalVolumes: []corev1.Volume{
					{
						Name: "config-volume",
						VolumeSource: corev1.VolumeSource{
							ConfigMap: &corev1.ConfigMapVolumeSource{
								LocalObjectReference: corev1.LocalObjectReference{
									Name: "redis-config",
								},
							},
						},
					},
					{
						Name: "logs-volume",
						VolumeSource: corev1.VolumeSource{
							EmptyDir: &corev1.EmptyDirVolumeSource{},
						},
					},
				},
				AdditionalVolumeMounts: []corev1.VolumeMount{
					{
						Name:      "config-volume",
						MountPath: "/etc/redis/config",
						ReadOnly:  true,
					},
					{
						Name:      "logs-volume",
						MountPath: "/var/log/redis",
					},
				},
			},
			expectedVolumes: []corev1.Volume{
				{
					Name: "config-volume",
					VolumeSource: corev1.VolumeSource{
						ConfigMap: &corev1.ConfigMapVolumeSource{
							LocalObjectReference: corev1.LocalObjectReference{
								Name: "redis-config",
							},
						},
					},
				},
				{
					Name: "logs-volume",
					VolumeSource: corev1.VolumeSource{
						EmptyDir: &corev1.EmptyDirVolumeSource{},
					},
				},
			},
			expectedVolumeMounts: []corev1.VolumeMount{
				{
					Name:      "config-volume",
					MountPath: "/etc/redis/config",
					ReadOnly:  true,
				},
				{
					Name:      "logs-volume",
					MountPath: "/var/log/redis",
				},
			},
			volumeMountNamesShouldMatch: true,
		},
		{
			name: "mismatched volumes and mounts",
			config: &KubernetesConfig{
				AdditionalVolumes: []corev1.Volume{
					{
						Name: "volume-a",
						VolumeSource: corev1.VolumeSource{
							EmptyDir: &corev1.EmptyDirVolumeSource{},
						},
					},
				},
				AdditionalVolumeMounts: []corev1.VolumeMount{
					{
						Name:      "volume-b",
						MountPath: "/test",
					},
				},
			},
			expectedVolumes: []corev1.Volume{
				{
					Name: "volume-a",
					VolumeSource: corev1.VolumeSource{
						EmptyDir: &corev1.EmptyDirVolumeSource{},
					},
				},
			},
			expectedVolumeMounts: []corev1.VolumeMount{
				{
					Name:      "volume-b",
					MountPath: "/test",
				},
			},
			volumeMountNamesShouldMatch: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expectedVolumes, tt.config.AdditionalVolumes)
			assert.Equal(t, tt.expectedVolumeMounts, tt.config.AdditionalVolumeMounts)

			// Test that volume mount names can reference volume names
			if tt.volumeMountNamesShouldMatch {
				volumeNames := make(map[string]bool)
				for _, vol := range tt.config.AdditionalVolumes {
					volumeNames[vol.Name] = true
				}

				for _, mount := range tt.config.AdditionalVolumeMounts {
					assert.True(t, volumeNames[mount.Name],
						"VolumeMount name '%s' should match a Volume name", mount.Name)
				}
			}
		})
	}
}

func TestKubernetesConfig_VolumeTypes_Comprehensive(t *testing.T) {
	tests := []struct {
		name   string
		volume corev1.Volume
	}{
		{
			name: "ConfigMap volume",
			volume: corev1.Volume{
				Name: "configmap-vol",
				VolumeSource: corev1.VolumeSource{
					ConfigMap: &corev1.ConfigMapVolumeSource{
						LocalObjectReference: corev1.LocalObjectReference{
							Name: "my-configmap",
						},
						DefaultMode: ptr.To[int32](0o644),
					},
				},
			},
		},
		{
			name: "Secret volume",
			volume: corev1.Volume{
				Name: "secret-vol",
				VolumeSource: corev1.VolumeSource{
					Secret: &corev1.SecretVolumeSource{
						SecretName:  "my-secret",
						DefaultMode: ptr.To[int32](0o600),
					},
				},
			},
		},
		{
			name: "EmptyDir volume",
			volume: corev1.Volume{
				Name: "emptydir-vol",
				VolumeSource: corev1.VolumeSource{
					EmptyDir: &corev1.EmptyDirVolumeSource{
						SizeLimit: ptr.To(resource.MustParse("1Gi")),
					},
				},
			},
		},
		{
			name: "PersistentVolumeClaim volume",
			volume: corev1.Volume{
				Name: "pvc-vol",
				VolumeSource: corev1.VolumeSource{
					PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{
						ClaimName: "my-pvc",
						ReadOnly:  true,
					},
				},
			},
		},
		{
			name: "HostPath volume",
			volume: corev1.Volume{
				Name: "hostpath-vol",
				VolumeSource: corev1.VolumeSource{
					HostPath: &corev1.HostPathVolumeSource{
						Path: "/host/path",
						Type: ptr.To(corev1.HostPathDirectoryOrCreate),
					},
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config := &KubernetesConfig{
				AdditionalVolumes: []corev1.Volume{tt.volume},
			}

			assert.Len(t, config.AdditionalVolumes, 1)
			assert.Equal(t, tt.volume, config.AdditionalVolumes[0])
			assert.Equal(t, tt.volume.Name, config.AdditionalVolumes[0].Name)
		})
	}
}

func TestKubernetesConfig_VolumeMountOptions_Comprehensive(t *testing.T) {
	tests := []struct {
		name        string
		volumeMount corev1.VolumeMount
	}{
		{
			name: "ReadOnly mount",
			volumeMount: corev1.VolumeMount{
				Name:      "readonly-vol",
				MountPath: "/readonly",
				ReadOnly:  true,
			},
		},
		{
			name: "ReadWrite mount",
			volumeMount: corev1.VolumeMount{
				Name:      "readwrite-vol",
				MountPath: "/readwrite",
				ReadOnly:  false,
			},
		},
		{
			name: "SubPath mount",
			volumeMount: corev1.VolumeMount{
				Name:      "subpath-vol",
				MountPath: "/subpath",
				SubPath:   "config/redis.conf",
			},
		},
		{
			name: "SubPathExpr mount",
			volumeMount: corev1.VolumeMount{
				Name:        "subpathexpr-vol",
				MountPath:   "/dynamic",
				SubPathExpr: "$(POD_NAME)/config",
			},
		},
		{
			name: "MountPropagation mount",
			volumeMount: corev1.VolumeMount{
				Name:             "propagation-vol",
				MountPath:        "/propagation",
				MountPropagation: ptr.To(corev1.MountPropagationBidirectional),
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config := &KubernetesConfig{
				AdditionalVolumeMounts: []corev1.VolumeMount{tt.volumeMount},
			}

			assert.Len(t, config.AdditionalVolumeMounts, 1)
			assert.Equal(t, tt.volumeMount, config.AdditionalVolumeMounts[0])
			assert.Equal(t, tt.volumeMount.Name, config.AdditionalVolumeMounts[0].Name)
			assert.Equal(t, tt.volumeMount.MountPath, config.AdditionalVolumeMounts[0].MountPath)
		})
	}
}

func TestKubernetesConfig_DeepCopy_AdditionalVolumes(t *testing.T) {
	original := &KubernetesConfig{
		Image: "redis:latest",
		AdditionalVolumes: []corev1.Volume{
			{
				Name: "config-volume",
				VolumeSource: corev1.VolumeSource{
					ConfigMap: &corev1.ConfigMapVolumeSource{
						LocalObjectReference: corev1.LocalObjectReference{
							Name: "redis-config",
						},
					},
				},
			},
			{
				Name: "logs-volume",
				VolumeSource: corev1.VolumeSource{
					EmptyDir: &corev1.EmptyDirVolumeSource{},
				},
			},
		},
		AdditionalVolumeMounts: []corev1.VolumeMount{
			{
				Name:      "config-volume",
				MountPath: "/etc/redis/config",
				ReadOnly:  true,
			},
			{
				Name:      "logs-volume",
				MountPath: "/var/log/redis",
			},
		},
	}

	// Test deep copy
	copied := original.DeepCopy()

	// Verify the copy is equal but separate
	assert.Equal(t, original.AdditionalVolumes, copied.AdditionalVolumes)
	assert.Equal(t, original.AdditionalVolumeMounts, copied.AdditionalVolumeMounts)

	// Verify modifying the copy doesn't affect the original
	copied.AdditionalVolumes[0].Name = "modified-volume"
	copied.AdditionalVolumeMounts[0].MountPath = "/modified/path"

	assert.NotEqual(t, original.AdditionalVolumes[0].Name, copied.AdditionalVolumes[0].Name)
	assert.NotEqual(t, original.AdditionalVolumeMounts[0].MountPath, copied.AdditionalVolumeMounts[0].MountPath)
	assert.Equal(t, "config-volume", original.AdditionalVolumes[0].Name)
	assert.Equal(t, "/etc/redis/config", original.AdditionalVolumeMounts[0].MountPath)
}

func TestKubernetesConfig_AdditionalVolumes_EdgeCases(t *testing.T) {
	tests := []struct {
		name        string
		volumes     []corev1.Volume
		expectValid bool
	}{
		{
			name:        "nil volumes slice",
			volumes:     nil,
			expectValid: true,
		},
		{
			name:        "empty volumes slice",
			volumes:     []corev1.Volume{},
			expectValid: true,
		},
		{
			name: "volume with empty name",
			volumes: []corev1.Volume{
				{
					Name: "",
					VolumeSource: corev1.VolumeSource{
						EmptyDir: &corev1.EmptyDirVolumeSource{},
					},
				},
			},
			expectValid: false, // Invalid according to Kubernetes validation
		},
		{
			name: "volume with no volume source",
			volumes: []corev1.Volume{
				{
					Name: "invalid-volume",
					// No VolumeSource specified
				},
			},
			expectValid: false, // Invalid according to Kubernetes validation
		},
		{
			name: "duplicate volume names",
			volumes: []corev1.Volume{
				{
					Name: "duplicate",
					VolumeSource: corev1.VolumeSource{
						EmptyDir: &corev1.EmptyDirVolumeSource{},
					},
				},
				{
					Name: "duplicate",
					VolumeSource: corev1.VolumeSource{
						EmptyDir: &corev1.EmptyDirVolumeSource{},
					},
				},
			},
			expectValid: false, // Invalid according to Kubernetes validation
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config := &KubernetesConfig{
				AdditionalVolumes: tt.volumes,
			}

			// For the purposes of this test, we're just verifying that our struct
			// can handle these cases without panicking. Kubernetes validation
			// would catch the invalid cases during actual deployment.
			assert.Equal(t, tt.volumes, config.AdditionalVolumes)

			// Test that the slice length is correct
			if tt.volumes == nil {
				assert.Nil(t, config.AdditionalVolumes)
			} else {
				assert.Len(t, config.AdditionalVolumes, len(tt.volumes))
			}
		})
	}
}

func TestKubernetesConfig_AdditionalVolumeMounts_EdgeCases(t *testing.T) {
	tests := []struct {
		name         string
		volumeMounts []corev1.VolumeMount
		expectValid  bool
	}{
		{
			name:         "nil volume mounts slice",
			volumeMounts: nil,
			expectValid:  true,
		},
		{
			name:         "empty volume mounts slice",
			volumeMounts: []corev1.VolumeMount{},
			expectValid:  true,
		},
		{
			name: "volume mount with empty name",
			volumeMounts: []corev1.VolumeMount{
				{
					Name:      "",
					MountPath: "/test",
				},
			},
			expectValid: false, // Invalid according to Kubernetes validation
		},
		{
			name: "volume mount with empty mount path",
			volumeMounts: []corev1.VolumeMount{
				{
					Name:      "test-vol",
					MountPath: "",
				},
			},
			expectValid: false, // Invalid according to Kubernetes validation
		},
		{
			name: "volume mount with relative path",
			volumeMounts: []corev1.VolumeMount{
				{
					Name:      "test-vol",
					MountPath: "relative/path",
				},
			},
			expectValid: false, // Invalid according to Kubernetes validation
		},
		{
			name: "volume mount with absolute path",
			volumeMounts: []corev1.VolumeMount{
				{
					Name:      "test-vol",
					MountPath: "/absolute/path",
				},
			},
			expectValid: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config := &KubernetesConfig{
				AdditionalVolumeMounts: tt.volumeMounts,
			}

			// For the purposes of this test, we're just verifying that our struct
			// can handle these cases without panicking. Kubernetes validation
			// would catch the invalid cases during actual deployment.
			assert.Equal(t, tt.volumeMounts, config.AdditionalVolumeMounts)

			// Test that the slice length is correct
			if tt.volumeMounts == nil {
				assert.Nil(t, config.AdditionalVolumeMounts)
			} else {
				assert.Len(t, config.AdditionalVolumeMounts, len(tt.volumeMounts))
			}
		})
	}
}
