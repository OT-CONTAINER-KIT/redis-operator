package k8sutils

import (
	"path"
	"testing"

	common "github.com/OT-CONTAINER-KIT/redis-operator/api"
	redisv1beta2 "github.com/OT-CONTAINER-KIT/redis-operator/api/v1beta2"
	"github.com/stretchr/testify/assert"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/utils/pointer"
)

func TestGetVolumeMount(t *testing.T) {
	tests := []struct {
		name               string
		persistenceEnabled *bool
		clusterMode        bool
		nodeConfVolume     bool
		externalConfig     *string
		mountpath          []corev1.VolumeMount
		tlsConfig          *redisv1beta2.TLSConfig
		aclConfig          *redisv1beta2.ACLConfig
		expectedMounts     []corev1.VolumeMount
	}{
		{
			name:               "1. All false or nil",
			persistenceEnabled: nil,
			clusterMode:        false,
			nodeConfVolume:     false,
			externalConfig:     nil,
			mountpath:          []corev1.VolumeMount{},
			tlsConfig:          nil,
			aclConfig:          nil,
			expectedMounts:     []corev1.VolumeMount{},
		},
		{
			name:               "2. Persistence enabled with cluster mode and node conf",
			persistenceEnabled: pointer.Bool(true),
			clusterMode:        true,
			nodeConfVolume:     true,
			externalConfig:     nil,
			mountpath:          []corev1.VolumeMount{},
			tlsConfig:          nil,
			aclConfig:          nil,
			expectedMounts: []corev1.VolumeMount{
				{
					Name:      "persistent-volume",
					MountPath: "/data",
				},
				{
					Name:      "node-conf",
					MountPath: "/node-conf",
				},
			},
		},
		{
			name:               "3. Persistence enabled with cluster mode and external config",
			persistenceEnabled: pointer.Bool(true),
			clusterMode:        true,
			nodeConfVolume:     false,
			externalConfig:     pointer.String("some-config"),
			mountpath:          []corev1.VolumeMount{},
			tlsConfig:          nil,
			aclConfig:          nil,
			expectedMounts: []corev1.VolumeMount{
				{
					Name:      "persistent-volume",
					MountPath: "/data",
				},
				{
					Name:      "external-config",
					MountPath: "/etc/redis/external.conf.d",
				},
			},
		},
		{
			name:               "4. Persistence enabled, cluster mode false, node conf true, no tls/acl, with mountpath",
			persistenceEnabled: pointer.Bool(true),
			clusterMode:        false,
			nodeConfVolume:     true,
			externalConfig:     nil,
			mountpath: []corev1.VolumeMount{
				{
					Name:      "additional-mount",
					MountPath: "/additional",
				},
			},
			tlsConfig:      nil,
			aclConfig:      nil,
			expectedMounts: []corev1.VolumeMount{{Name: "persistent-volume", MountPath: "/data"}, {Name: "additional-mount", MountPath: "/additional"}},
		},
		{
			name:               "5. Only tls enabled",
			persistenceEnabled: nil,
			clusterMode:        false,
			nodeConfVolume:     false,
			externalConfig:     nil,
			mountpath:          []corev1.VolumeMount{},
			tlsConfig:          &redisv1beta2.TLSConfig{},
			aclConfig:          nil,
			expectedMounts:     []corev1.VolumeMount{{Name: "tls-certs", MountPath: "/tls", ReadOnly: true}},
		},
		{
			name:               "6. Only acl enabled",
			persistenceEnabled: nil,
			clusterMode:        false,
			nodeConfVolume:     false,
			externalConfig:     nil,
			mountpath:          []corev1.VolumeMount{},
			tlsConfig:          nil,
			aclConfig:          &redisv1beta2.ACLConfig{},
			expectedMounts:     []corev1.VolumeMount{{Name: "acl-secret", MountPath: "/etc/redis/user.acl", SubPath: "user.acl"}},
		},
		{
			name:               "7. Everything enabled except externalConfig",
			persistenceEnabled: pointer.Bool(true),
			clusterMode:        true,
			nodeConfVolume:     true,
			externalConfig:     nil,
			mountpath: []corev1.VolumeMount{
				{
					Name:      "additional-mount",
					MountPath: "/additional",
				},
			},
			tlsConfig: &redisv1beta2.TLSConfig{},
			aclConfig: &redisv1beta2.ACLConfig{},
			expectedMounts: []corev1.VolumeMount{
				{Name: "persistent-volume", MountPath: "/data"},
				{Name: "node-conf", MountPath: "/node-conf"},
				{Name: "tls-certs", MountPath: "/tls", ReadOnly: true},
				{Name: "acl-secret", MountPath: "/etc/redis/user.acl", SubPath: "user.acl"},
				{Name: "additional-mount", MountPath: "/additional"},
			},
		},
		{
			name:               "8. Only externalConfig enabled",
			persistenceEnabled: nil,
			clusterMode:        false,
			nodeConfVolume:     false,
			externalConfig:     pointer.String("some-config"),
			mountpath:          []corev1.VolumeMount{},
			tlsConfig:          nil,
			aclConfig:          nil,
			expectedMounts:     []corev1.VolumeMount{{Name: "external-config", MountPath: "/etc/redis/external.conf.d"}},
		},
		{
			name:               "9. Persistence enabled, cluster mode true, node conf true, only acl enabled",
			persistenceEnabled: pointer.Bool(true),
			clusterMode:        true,
			nodeConfVolume:     true,
			externalConfig:     nil,
			mountpath:          []corev1.VolumeMount{},
			tlsConfig:          nil,
			aclConfig:          &redisv1beta2.ACLConfig{},
			expectedMounts: []corev1.VolumeMount{
				{Name: "persistent-volume", MountPath: "/data"},
				{Name: "node-conf", MountPath: "/node-conf"},
				{Name: "acl-secret", MountPath: "/etc/redis/user.acl", SubPath: "user.acl"},
			},
		},
		{
			name:               "10. Persistence enabled, cluster mode false, node conf false, only tls enabled with mountpath",
			persistenceEnabled: pointer.Bool(true),
			clusterMode:        false,
			nodeConfVolume:     false,
			externalConfig:     nil,
			mountpath: []corev1.VolumeMount{
				{
					Name:      "additional-mount",
					MountPath: "/additional",
				},
			},
			tlsConfig:      &redisv1beta2.TLSConfig{},
			aclConfig:      nil,
			expectedMounts: []corev1.VolumeMount{{Name: "persistent-volume", MountPath: "/data"}, {Name: "tls-certs", MountPath: "/tls", ReadOnly: true}, {Name: "additional-mount", MountPath: "/additional"}},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := getVolumeMount("persistent-volume", tt.persistenceEnabled, tt.clusterMode, tt.nodeConfVolume, tt.externalConfig, tt.mountpath, tt.tlsConfig, tt.aclConfig)
			assert.ElementsMatch(t, tt.expectedMounts, got)
		})
	}
}

func TestGenerateTLSEnvironmentVariables(t *testing.T) {
	tlsConfig := &redisv1beta2.TLSConfig{
		TLSConfig: common.TLSConfig{
			CaKeyFile:   "test_ca.crt",
			CertKeyFile: "test_tls.crt",
			KeyFile:     "test_tls.key",
		},
	}

	envVars := GenerateTLSEnvironmentVariables(tlsConfig)

	expectedEnvVars := []corev1.EnvVar{
		{
			Name:  "TLS_MODE",
			Value: "true",
		},
		{
			Name:  "REDIS_TLS_CA_KEY",
			Value: path.Join("/tls/", "test_ca.crt"),
		},
		{
			Name:  "REDIS_TLS_CERT",
			Value: path.Join("/tls/", "test_tls.crt"),
		},
		{
			Name:  "REDIS_TLS_CERT_KEY",
			Value: path.Join("/tls/", "test_tls.key"),
		},
	}
	assert.ElementsMatch(t, envVars, expectedEnvVars, "EnvVars generated for TLS config are not as expected")
}

func TestGetEnvironmentVariables(t *testing.T) {
	tests := []struct {
		name                string
		role                string
		enabledPassword     *bool
		secretName          *string
		secretKey           *string
		persistenceEnabled  *bool
		tlsConfig           *redisv1beta2.TLSConfig
		aclConfig           *redisv1beta2.ACLConfig
		envVar              *[]corev1.EnvVar
		port                *int
		clusterVersion      *string
		expectedEnvironment []corev1.EnvVar
	}{
		{
			name:               "Test with role sentinel, metrics true, password true, persistence true, exporter env, tls enabled, acl enabled and env var",
			role:               "sentinel",
			enabledPassword:    pointer.Bool(true),
			secretName:         pointer.String("test-secret"),
			secretKey:          pointer.String("test-key"),
			persistenceEnabled: pointer.Bool(true),
			tlsConfig: &redisv1beta2.TLSConfig{
				TLSConfig: common.TLSConfig{
					CaKeyFile:   "test_ca.crt",
					CertKeyFile: "test_tls.crt",
					KeyFile:     "test_tls.key",
					Secret: corev1.SecretVolumeSource{
						SecretName: "tls-secret",
					},
				},
			},
			aclConfig: &redisv1beta2.ACLConfig{
				Secret: &corev1.SecretVolumeSource{
					SecretName: "acl-secret",
				},
			},
			envVar: &[]corev1.EnvVar{
				{Name: "TEST_ENV", Value: "test-value"},
			},
			clusterVersion: pointer.String("v6"),
			expectedEnvironment: []corev1.EnvVar{
				{Name: "ACL_MODE", Value: "true"},
				{Name: "PERSISTENCE_ENABLED", Value: "true"},
				{Name: "REDIS_ADDR", Value: "redis://localhost:26379"},
				{Name: "TLS_MODE", Value: "true"},
				{Name: "REDIS_TLS_CA_KEY", Value: path.Join("/tls/", "test_ca.crt")},
				{Name: "REDIS_TLS_CERT", Value: path.Join("/tls/", "test_tls.crt")},
				{Name: "REDIS_TLS_CERT_KEY", Value: path.Join("/tls/", "test_tls.key")},
				{Name: "REDIS_PASSWORD", ValueFrom: &corev1.EnvVarSource{
					SecretKeyRef: &corev1.SecretKeySelector{
						LocalObjectReference: corev1.LocalObjectReference{
							Name: "test-secret",
						},
						Key: "test-key",
					},
				}},
				{Name: "SERVER_MODE", Value: "sentinel"},
				{Name: "SETUP_MODE", Value: "sentinel"},
				{Name: "TEST_ENV", Value: "test-value"},
				{Name: "REDIS_MAJOR_VERSION", Value: "v6"},
			},
		},
		{
			name:               "Test with role redis, metrics false, password nil, persistence nil, exporter nil, tls nil, acl nil and nil env var",
			role:               "redis",
			enabledPassword:    nil,
			secretName:         nil,
			secretKey:          nil,
			persistenceEnabled: nil,
			tlsConfig:          nil,
			aclConfig:          nil,
			envVar:             nil,
			port:               nil,
			clusterVersion:     nil,
			expectedEnvironment: []corev1.EnvVar{
				{Name: "REDIS_ADDR", Value: "redis://localhost:6379"},
				{Name: "SERVER_MODE", Value: "redis"},
				{Name: "SETUP_MODE", Value: "redis"},
			},
		},
		{
			name:               "Test with role redis, metrics false, password nil, persistence false, exporter nil, tls nil, acl nil and nil env var",
			role:               "sentinel",
			enabledPassword:    nil,
			secretName:         nil,
			secretKey:          nil,
			persistenceEnabled: pointer.Bool(false),
			tlsConfig:          nil,
			aclConfig:          nil,
			envVar:             nil,
			expectedEnvironment: []corev1.EnvVar{
				{Name: "REDIS_ADDR", Value: "redis://localhost:26379"},
				{Name: "SERVER_MODE", Value: "sentinel"},
				{Name: "SETUP_MODE", Value: "sentinel"},
			},
		},
		{
			name:               "Test with role cluster, metrics true, password true, persistence true, exporter env, tls nil, acl enabled and env var",
			role:               "cluster",
			enabledPassword:    pointer.Bool(true),
			secretName:         pointer.String("test-secret"),
			secretKey:          pointer.String("test-key"),
			persistenceEnabled: pointer.Bool(true),
			tlsConfig:          nil,
			aclConfig:          &redisv1beta2.ACLConfig{},
			envVar: &[]corev1.EnvVar{
				{Name: "TEST_ENV", Value: "test-value"},
			},
			port: pointer.Int(6380),
			expectedEnvironment: []corev1.EnvVar{
				{Name: "ACL_MODE", Value: "true"},
				{Name: "PERSISTENCE_ENABLED", Value: "true"},
				{Name: "REDIS_ADDR", Value: "redis://localhost:6379"},
				{Name: "REDIS_PASSWORD", ValueFrom: &corev1.EnvVarSource{
					SecretKeyRef: &corev1.SecretKeySelector{
						LocalObjectReference: corev1.LocalObjectReference{
							Name: "test-secret",
						},
						Key: "test-key",
					},
				}},
				{Name: "SERVER_MODE", Value: "cluster"},
				{Name: "SETUP_MODE", Value: "cluster"},
				{Name: "TEST_ENV", Value: "test-value"},
				{Name: "REDIS_PORT", Value: "6380"},
			},
		},
		{
			name:               "Test with cluster role and only metrics enabled",
			role:               "cluster",
			enabledPassword:    nil,
			secretName:         nil,
			secretKey:          nil,
			persistenceEnabled: nil,
			tlsConfig:          nil,
			aclConfig:          nil,
			envVar:             nil,
			expectedEnvironment: []corev1.EnvVar{
				{Name: "REDIS_ADDR", Value: "redis://localhost:6379"},
				{Name: "SERVER_MODE", Value: "cluster"},
				{Name: "SETUP_MODE", Value: "cluster"},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			actualEnvironment := getEnvironmentVariables(tt.role, tt.enabledPassword, tt.secretName,
				tt.secretKey, tt.persistenceEnabled, tt.tlsConfig, tt.aclConfig, tt.envVar, tt.port, tt.clusterVersion)

			assert.ElementsMatch(t, tt.expectedEnvironment, actualEnvironment)
		})
	}
}

func Test_getExporterEnvironmentVariables(t *testing.T) {
	tests := []struct {
		name                string
		params              containerParameters
		tlsConfig           *redisv1beta2.TLSConfig
		envVar              *[]corev1.EnvVar
		expectedEnvironment []corev1.EnvVar
	}{
		{
			name: "Test with tls enabled and env var",
			params: containerParameters{
				TLSConfig: &redisv1beta2.TLSConfig{
					TLSConfig: common.TLSConfig{
						CaKeyFile:   "test_ca.crt",
						CertKeyFile: "test_tls.crt",
						KeyFile:     "test_tls.key",
						Secret: corev1.SecretVolumeSource{
							SecretName: "tls-secret",
						},
					},
				},
				RedisExporterEnv: &[]corev1.EnvVar{
					{Name: "TEST_ENV", Value: "test-value"},
				},
			},
			expectedEnvironment: []corev1.EnvVar{
				{Name: "REDIS_EXPORTER_TLS_CLIENT_KEY_FILE", Value: "/tls/tls.key"},
				{Name: "REDIS_EXPORTER_TLS_CLIENT_CERT_FILE", Value: "/tls/tls.crt"},
				{Name: "REDIS_EXPORTER_TLS_CA_CERT_FILE", Value: "/tls/ca.crt"},
				{Name: "REDIS_EXPORTER_SKIP_TLS_VERIFICATION", Value: "true"},
				{Name: "TEST_ENV", Value: "test-value"},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			actualEnvironment := getExporterEnvironmentVariables(tt.params)

			assert.ElementsMatch(t, tt.expectedEnvironment, actualEnvironment)
		})
	}
}
