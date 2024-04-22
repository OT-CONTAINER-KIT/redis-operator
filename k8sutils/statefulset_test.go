package k8sutils

import (
	"context"
	"path"
	"testing"

	common "github.com/OT-CONTAINER-KIT/redis-operator/api"
	redisv1beta2 "github.com/OT-CONTAINER-KIT/redis-operator/api/v1beta2"
	"github.com/go-logr/logr"
	"github.com/stretchr/testify/assert"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	kerrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	k8sClientFake "k8s.io/client-go/kubernetes/fake"
	"k8s.io/utils/ptr"
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
			persistenceEnabled: ptr.To(true),
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
			persistenceEnabled: ptr.To(true),
			clusterMode:        true,
			nodeConfVolume:     false,
			externalConfig:     ptr.To("some-config"),
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
			persistenceEnabled: ptr.To(true),
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
			persistenceEnabled: ptr.To(true),
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
			externalConfig:     ptr.To("some-config"),
			mountpath:          []corev1.VolumeMount{},
			tlsConfig:          nil,
			aclConfig:          nil,
			expectedMounts:     []corev1.VolumeMount{{Name: "external-config", MountPath: "/etc/redis/external.conf.d"}},
		},
		{
			name:               "9. Persistence enabled, cluster mode true, node conf true, only acl enabled",
			persistenceEnabled: ptr.To(true),
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
			persistenceEnabled: ptr.To(true),
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

func Test_GetStatefulSet(t *testing.T) {
	logger := logr.Discard()

	tests := []struct {
		name         string
		sts          appsv1.StatefulSet
		stsName      string
		stsNamespace string
		present      bool
	}{
		{
			name: "StatefulSet present",
			sts: appsv1.StatefulSet{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-sts",
					Namespace: "test-ns",
				},
			},
			stsName:      "test-sts",
			stsNamespace: "test-ns",
			present:      true,
		},
		{
			name:         "StatefulSet not present",
			sts:          appsv1.StatefulSet{},
			stsName:      "test-sts",
			stsNamespace: "test-ns",
			present:      false,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			client := k8sClientFake.NewSimpleClientset(test.sts.DeepCopy())
			_, err := GetStatefulSet(client, logger, test.stsNamespace, test.stsName)
			if test.present {
				assert.Nil(t, err)
			} else {
				assert.NotNil(t, err)
			}
		})
	}
}

func Test_createStatefulSet(t *testing.T) {
	logger := logr.Discard()

	tests := []struct {
		name    string
		sts     appsv1.StatefulSet
		present bool
	}{
		{
			name: "StatefulSet present",
			sts: appsv1.StatefulSet{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-sts",
					Namespace: "test-ns",
				},
			},

			present: true,
		},
		{
			name: "StatefulSet not present",
			sts: appsv1.StatefulSet{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-sts",
					Namespace: "test-ns",
				},
			},
			present: false,
		},
	}

	for i := range tests {
		test := tests[i]
		t.Run(test.name, func(t *testing.T) {
			var client *k8sClientFake.Clientset
			if test.present {
				client = k8sClientFake.NewSimpleClientset(test.sts.DeepCopy())
			} else {
				client = k8sClientFake.NewSimpleClientset()
			}
			err := createStatefulSet(client, logger, test.sts.GetNamespace(), &test.sts)
			if test.present {
				assert.NotNil(t, err)
			} else {
				assert.Nil(t, err)
			}
		})
	}
}

func TestUpdateStatefulSet(t *testing.T) {
	logger := logr.Discard()
	tests := []struct {
		name            string
		existingStsSpec appsv1.StatefulSetSpec
		updatedStsSpec  appsv1.StatefulSetSpec
		recreateSts     bool
		stsPresent      bool
		expectErr       error
	}{
		{
			name: "Update StatefulSet without recreate in existing Statefulset",
			existingStsSpec: appsv1.StatefulSetSpec{
				Replicas: ptr.To(int32(3)),
			},
			updatedStsSpec: appsv1.StatefulSetSpec{
				Replicas: ptr.To(int32(5)),
			},
			recreateSts: false,
			stsPresent:  true,
		},
		{
			name: "Update StatefulSet with recreate in existing Statefulset",
			existingStsSpec: appsv1.StatefulSetSpec{
				Replicas: ptr.To(int32(2)),
			},
			updatedStsSpec: appsv1.StatefulSetSpec{
				Replicas: ptr.To(int32(4)),
			},
			recreateSts: true,
			stsPresent:  true,
		},
		{
			name: "Update StatefulSet without recreate StatefulSet is not present",
			existingStsSpec: appsv1.StatefulSetSpec{
				Replicas: ptr.To(int32(2)),
			},
			updatedStsSpec: appsv1.StatefulSetSpec{
				Replicas: ptr.To(int32(4)),
			},
			recreateSts: false,
			stsPresent:  false,
			expectErr:   kerrors.NewNotFound(schema.GroupResource{Group: "apps", Resource: "statefulsets"}, "test-sts"),
		},
		{
			name: "Update StatefulSet without recreate StatefulSet",
			existingStsSpec: appsv1.StatefulSetSpec{
				Template: corev1.PodTemplateSpec{
					ObjectMeta: metav1.ObjectMeta{
						Labels: map[string]string{
							"name": "redis",
						},
					},
				},
			},
			updatedStsSpec: appsv1.StatefulSetSpec{
				Template: corev1.PodTemplateSpec{
					ObjectMeta: metav1.ObjectMeta{
						Labels: map[string]string{
							"name": "redis-standalone",
						},
					},
				},
			},
			recreateSts: false,
			stsPresent:  true,
		},
		{
			name: "Update StatefulSet failed with Invalid Reason",
			existingStsSpec: appsv1.StatefulSetSpec{
				Replicas: ptr.To(int32(2)),
			},
			updatedStsSpec: appsv1.StatefulSetSpec{
				Replicas: ptr.To(int32(4)),
			},
			recreateSts: true,
			stsPresent:  false,
			expectErr:   kerrors.NewNotFound(schema.GroupResource{Group: "apps", Resource: "statefulsets"}, "test-sts"),
		},
	}

	assert := assert.New(t)

	for i := range tests {
		test := tests[i]
		t.Run(test.name, func(t *testing.T) {
			existingSts := appsv1.StatefulSet{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-sts",
					Namespace: "test-ns",
				},
				Spec: *test.existingStsSpec.DeepCopy(),
			}
			updatedSts := appsv1.StatefulSet{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-sts",
					Namespace: "test-ns",
				},
				Spec: *test.updatedStsSpec.DeepCopy(),
			}
			var client *k8sClientFake.Clientset
			if test.stsPresent {
				client = k8sClientFake.NewSimpleClientset(existingSts.DeepCopyObject())
			} else {
				client = k8sClientFake.NewSimpleClientset()
			}
			err := updateStatefulSet(client, logger, updatedSts.GetNamespace(), &updatedSts, test.recreateSts)
			if test.expectErr != nil {
				assert.Error(err, "Expected Error while updating Statefulset")
				assert.Equal(test.expectErr, err)
			} else {
				assert.NoError(err, "Error while updating Statefulset")
			}
			if err == nil {
				getUpdatedSts, err := client.AppsV1().StatefulSets(updatedSts.GetNamespace()).Get(context.TODO(), updatedSts.GetName(), metav1.GetOptions{})
				assert.NoError(err, "Error getting Updted StatefulSet")
				assert.NotEqual(getUpdatedSts.DeepCopy(), existingSts.DeepCopy(), "StatefulSet not updated")
			}
		})
	}
}

func TestEnableRedisMonitoring(t *testing.T) {
	tests := []struct {
		name                  string
		redisExporterParams   containerParameters
		expectedRedisExporter corev1.Container
	}{
		{
			name: "Redis Monitoring",
			redisExporterParams: containerParameters{
				RedisExporterImage:           "redis-exporter:latest",
				RedisExporterImagePullPolicy: corev1.PullIfNotPresent,
				RedisExporterPort:            ptr.To(9121),
			},
			expectedRedisExporter: corev1.Container{
				Name:            "redis-exporter",
				Image:           "redis-exporter:latest",
				ImagePullPolicy: corev1.PullIfNotPresent,
				Env: []corev1.EnvVar{
					{
						Name:  "REDIS_EXPORTER_WEB_LISTEN_ADDRESS",
						Value: ":9121",
					},
				},
				Ports: []corev1.ContainerPort{
					{
						Name:          "redis-exporter",
						ContainerPort: 9121,
						Protocol:      corev1.ProtocolTCP,
					},
				},
			},
		},
	}

	for i := range tests {
		test := tests[i]
		t.Run(test.name, func(t *testing.T) {
			redisExporter := enableRedisMonitoring(test.redisExporterParams)
			assert.Equal(t, redisExporter, test.expectedRedisExporter, "Redis Exporter Configuration")
		})
	}
}

func TestGenerateInitContainerDef(t *testing.T) {
	tests := []struct {
		name                     string
		initContainerDef         initContainerParameters
		expectedInitContainerDef []corev1.Container
		mountPaths               []corev1.VolumeMount
	}{
		{
			name: "Redis",
			initContainerDef: initContainerParameters{
				Image:              "redis-init-container:latest",
				ImagePullPolicy:    corev1.PullAlways,
				Command:            []string{"/bin/bash", "-c", "/app/restore.bash"},
				PersistenceEnabled: ptr.To(false),
			},
			expectedInitContainerDef: []corev1.Container{
				{
					Name:            "initRedis",
					Image:           "redis-init-container:latest",
					Command:         []string{"/bin/bash", "-c", "/app/restore.bash"},
					ImagePullPolicy: corev1.PullAlways,
					VolumeMounts:    getVolumeMount("redisVolume", ptr.To(false), false, false, nil, []corev1.VolumeMount{}, nil, nil),
				},
			},
			mountPaths: []corev1.VolumeMount{},
		},
		{
			name: "Redis-1",
			initContainerDef: initContainerParameters{
				Image:              "redis-init-container:latest",
				ImagePullPolicy:    corev1.PullAlways,
				Command:            []string{"/bin/bash", "-c", "/app/restore.bash"},
				PersistenceEnabled: ptr.To(true),
			},
			expectedInitContainerDef: []corev1.Container{
				{
					Name:            "initRedis-1",
					Image:           "redis-init-container:latest",
					Command:         []string{"/bin/bash", "-c", "/app/restore.bash"},
					ImagePullPolicy: corev1.PullAlways,
					VolumeMounts: getVolumeMount("Redis-1", ptr.To(true), false, false, nil, []corev1.VolumeMount{
						{
							Name:      "Redis-1",
							MountPath: "/data",
						},
					}, nil, nil),
				},
			},
			mountPaths: []corev1.VolumeMount{
				{
					Name:      "Redis-1",
					MountPath: "/data",
				},
			},
		},
	}

	for i := range tests {
		test := tests[i]
		t.Run(test.name, func(t *testing.T) {
			initContainer := generateInitContainerDef(test.name, test.initContainerDef, test.mountPaths)
			assert.Equal(t, initContainer, test.expectedInitContainerDef, "Init Container Configuration")
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
			enabledPassword:    ptr.To(true),
			secretName:         ptr.To("test-secret"),
			secretKey:          ptr.To("test-key"),
			persistenceEnabled: ptr.To(true),
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
			clusterVersion: ptr.To("v6"),
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
			persistenceEnabled: ptr.To(false),
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
			enabledPassword:    ptr.To(true),
			secretName:         ptr.To("test-secret"),
			secretKey:          ptr.To("test-key"),
			persistenceEnabled: ptr.To(true),
			tlsConfig:          nil,
			aclConfig:          &redisv1beta2.ACLConfig{},
			envVar: &[]corev1.EnvVar{
				{Name: "TEST_ENV", Value: "test-value"},
			},
			port: ptr.To(6380),
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
