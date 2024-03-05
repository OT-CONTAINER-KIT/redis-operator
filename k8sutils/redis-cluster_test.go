package k8sutils

import (
	"os"
	"path/filepath"
	"testing"

	common "github.com/OT-CONTAINER-KIT/redis-operator/api"
	redisv1beta2 "github.com/OT-CONTAINER-KIT/redis-operator/api/v1beta2"
	"github.com/go-logr/logr/testr"
	"github.com/stretchr/testify/assert"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	"k8s.io/apimachinery/pkg/util/yaml"
	"k8s.io/client-go/kubernetes/fake"
	"k8s.io/utils/pointer"
)

func Test_generateRedisClusterParams(t *testing.T) {
	path := filepath.Join("..", "tests", "testdata", "redis-cluster.yaml")

	expectedLeaderSTS := statefulSetParameters{
		Replicas:       pointer.Int32(3),
		ClusterMode:    true,
		NodeConfVolume: true,
		PodSecurityContext: &corev1.PodSecurityContext{
			RunAsUser: pointer.Int64(1000),
			FSGroup:   pointer.Int64(1000),
		},
		PriorityClassName: "high-priority",
		Affinity: &corev1.Affinity{
			NodeAffinity: &corev1.NodeAffinity{
				RequiredDuringSchedulingIgnoredDuringExecution: &corev1.NodeSelector{
					NodeSelectorTerms: []corev1.NodeSelectorTerm{
						{
							MatchExpressions: []corev1.NodeSelectorRequirement{
								{
									Key:      "node-role.kubernetes.io/infra",
									Operator: corev1.NodeSelectorOpIn,
									Values:   []string{"redisLeader"},
								},
							},
						},
					},
				},
			},
		},
		Tolerations: &[]corev1.Toleration{
			{
				Key:      "node-role.kubernetes.io/infra",
				Operator: corev1.TolerationOpExists,
				Effect:   corev1.TaintEffectNoSchedule,
			},
			{
				Key:      "node-role.kubernetes.io/infra",
				Operator: corev1.TolerationOpExists,
				Effect:   corev1.TaintEffectNoExecute,
			},
		},
		PersistentVolumeClaim: corev1.PersistentVolumeClaim{
			Spec: corev1.PersistentVolumeClaimSpec{
				StorageClassName: pointer.String("standard"),
				AccessModes:      []corev1.PersistentVolumeAccessMode{corev1.ReadWriteOnce},
				Resources: corev1.ResourceRequirements{
					Requests: corev1.ResourceList{
						corev1.ResourceStorage: resource.MustParse("1Gi"),
					},
				},
			},
		},
		NodeConfPersistentVolumeClaim: corev1.PersistentVolumeClaim{
			Spec: corev1.PersistentVolumeClaimSpec{
				StorageClassName: pointer.String("standard"),
				AccessModes:      []corev1.PersistentVolumeAccessMode{corev1.ReadWriteOnce},
				Resources: corev1.ResourceRequirements{
					Requests: corev1.ResourceList{
						corev1.ResourceStorage: resource.MustParse("1Gi"),
					},
				},
			},
		},
		EnableMetrics:      true,
		ImagePullSecrets:   &[]corev1.LocalObjectReference{{Name: "mysecret"}},
		ExternalConfig:     pointer.String("redis-external-config-leader"),
		ServiceAccountName: pointer.String("redis-sa"),
		IgnoreAnnotations:  []string{"opstreelabs.in/ignore"},
	}
	expectedFollowerSTS := statefulSetParameters{
		Replicas:       pointer.Int32(3),
		ClusterMode:    true,
		NodeConfVolume: true,
		PodSecurityContext: &corev1.PodSecurityContext{
			RunAsUser: pointer.Int64(1000),
			FSGroup:   pointer.Int64(1000),
		},
		PriorityClassName: "high-priority",
		Affinity: &corev1.Affinity{
			NodeAffinity: &corev1.NodeAffinity{
				RequiredDuringSchedulingIgnoredDuringExecution: &corev1.NodeSelector{
					NodeSelectorTerms: []corev1.NodeSelectorTerm{
						{
							MatchExpressions: []corev1.NodeSelectorRequirement{
								{
									Key:      "node-role.kubernetes.io/infra",
									Operator: corev1.NodeSelectorOpIn,
									Values:   []string{"redisFollower"},
								},
							},
						},
					},
				},
			},
		},
		Tolerations: &[]corev1.Toleration{
			{
				Key:      "node-role.kubernetes.io/infra",
				Operator: corev1.TolerationOpExists,
				Effect:   corev1.TaintEffectNoSchedule,
			},
			{
				Key:      "node-role.kubernetes.io/infra",
				Operator: corev1.TolerationOpExists,
				Effect:   corev1.TaintEffectNoExecute,
			},
		},
		PersistentVolumeClaim: corev1.PersistentVolumeClaim{
			Spec: corev1.PersistentVolumeClaimSpec{
				StorageClassName: pointer.String("standard"),
				AccessModes:      []corev1.PersistentVolumeAccessMode{corev1.ReadWriteOnce},
				Resources: corev1.ResourceRequirements{
					Requests: corev1.ResourceList{
						corev1.ResourceStorage: resource.MustParse("1Gi"),
					},
				},
			},
		},
		NodeConfPersistentVolumeClaim: corev1.PersistentVolumeClaim{
			Spec: corev1.PersistentVolumeClaimSpec{
				StorageClassName: pointer.String("standard"),
				AccessModes:      []corev1.PersistentVolumeAccessMode{corev1.ReadWriteOnce},
				Resources: corev1.ResourceRequirements{
					Requests: corev1.ResourceList{
						corev1.ResourceStorage: resource.MustParse("1Gi"),
					},
				},
			},
		},
		EnableMetrics:      true,
		ImagePullSecrets:   &[]corev1.LocalObjectReference{{Name: "mysecret"}},
		ExternalConfig:     pointer.String("redis-external-config-follower"),
		ServiceAccountName: pointer.String("redis-sa"),
		IgnoreAnnotations:  []string{"opstreelabs.in/ignore"},
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("Failed to read file %s: %v", path, err)
	}

	input := &redisv1beta2.RedisCluster{}
	err = yaml.UnmarshalStrict(data, input)
	if err != nil {
		t.Fatalf("Failed to unmarshal file %s: %v", path, err)
	}

	actualLeaderSTS := generateRedisClusterParams(input, *input.Spec.Size, input.Spec.RedisLeader.RedisConfig.AdditionalRedisConfig, RedisClusterSTS{
		RedisStateFulType:             "leader",
		ExternalConfig:                input.Spec.RedisLeader.RedisConfig.AdditionalRedisConfig,
		SecurityContext:               input.Spec.RedisLeader.SecurityContext,
		Affinity:                      input.Spec.RedisLeader.Affinity,
		TerminationGracePeriodSeconds: input.Spec.RedisLeader.TerminationGracePeriodSeconds,
		ReadinessProbe:                input.Spec.RedisLeader.ReadinessProbe,
		LivenessProbe:                 input.Spec.RedisLeader.LivenessProbe,
		NodeSelector:                  input.Spec.RedisLeader.NodeSelector,
		Tolerations:                   input.Spec.RedisLeader.Tolerations,
	})
	assert.EqualValues(t, expectedLeaderSTS, actualLeaderSTS, "Expected %+v, got %+v", expectedLeaderSTS, actualLeaderSTS)

	actualFollowerSTS := generateRedisClusterParams(input, *input.Spec.Size, input.Spec.RedisFollower.RedisConfig.AdditionalRedisConfig, RedisClusterSTS{
		RedisStateFulType:             "follower",
		ExternalConfig:                input.Spec.RedisFollower.RedisConfig.AdditionalRedisConfig,
		SecurityContext:               input.Spec.RedisFollower.SecurityContext,
		Affinity:                      input.Spec.RedisFollower.Affinity,
		TerminationGracePeriodSeconds: input.Spec.RedisFollower.TerminationGracePeriodSeconds,
		ReadinessProbe:                input.Spec.RedisFollower.ReadinessProbe,
		LivenessProbe:                 input.Spec.RedisFollower.LivenessProbe,
		NodeSelector:                  input.Spec.RedisFollower.NodeSelector,
		Tolerations:                   input.Spec.RedisFollower.Tolerations,
	})
	assert.EqualValues(t, expectedFollowerSTS, actualFollowerSTS, "Expected %+v, got %+v", expectedFollowerSTS, actualFollowerSTS)
}

func Test_generateRedisClusterContainerParams(t *testing.T) {
	path := filepath.Join("..", "tests", "testdata", "redis-cluster.yaml")
	expectedLeaderContainer := containerParameters{
		Image:           "quay.io/opstree/redis:v7.0.12",
		ImagePullPolicy: corev1.PullPolicy("IfNotPresent"),
		Resources: &corev1.ResourceRequirements{
			Requests: corev1.ResourceList{
				corev1.ResourceCPU:    resource.MustParse("101m"),
				corev1.ResourceMemory: resource.MustParse("128Mi"),
			},
			Limits: corev1.ResourceList{
				corev1.ResourceCPU:    resource.MustParse("101m"),
				corev1.ResourceMemory: resource.MustParse("128Mi"),
			},
		},
		SecurityContext: &corev1.SecurityContext{
			RunAsUser:              pointer.Int64(1000),
			RunAsGroup:             pointer.Int64(1000),
			RunAsNonRoot:           pointer.Bool(true),
			ReadOnlyRootFilesystem: pointer.Bool(true),
			Capabilities: &corev1.Capabilities{
				Drop: []corev1.Capability{"ALL"},
				Add:  []corev1.Capability{"NET_BIND_SERVICE"},
			},
		},
		RedisExporterImage:           "quay.io/opstree/redis-exporter:v1.44.0",
		RedisExporterImagePullPolicy: corev1.PullPolicy("Always"),
		RedisExporterResources: &corev1.ResourceRequirements{
			Requests: corev1.ResourceList{
				corev1.ResourceCPU:    resource.MustParse("100m"),
				corev1.ResourceMemory: resource.MustParse("128Mi"),
			},
			Limits: corev1.ResourceList{
				corev1.ResourceCPU:    resource.MustParse("100m"),
				corev1.ResourceMemory: resource.MustParse("128Mi"),
			},
		},
		RedisExporterEnv: &[]corev1.EnvVar{
			{
				Name:  "REDIS_EXPORTER_INCL_SYSTEM_METRICS",
				Value: "true",
			},
			{
				Name: "UI_PROPERTIES_FILE_NAME",
				ValueFrom: &corev1.EnvVarSource{
					ConfigMapKeyRef: &corev1.ConfigMapKeySelector{
						LocalObjectReference: corev1.LocalObjectReference{
							Name: "game-demo",
						},
						Key: "ui_properties_file_name",
					},
				},
			},
			{
				Name: "SECRET_USERNAME",
				ValueFrom: &corev1.EnvVarSource{
					SecretKeyRef: &corev1.SecretKeySelector{
						LocalObjectReference: corev1.LocalObjectReference{
							Name: "mysecret",
						},
						Key: "username",
					},
				},
			},
		},
		Role:               "cluster",
		EnabledPassword:    pointer.Bool(true),
		SecretName:         pointer.String("redis-secret"),
		SecretKey:          pointer.String("password"),
		PersistenceEnabled: pointer.Bool(true),
		TLSConfig: &redisv1beta2.TLSConfig{
			TLSConfig: common.TLSConfig{
				CaKeyFile:   "ca.key",
				CertKeyFile: "tls.crt",
				KeyFile:     "tls.key",
				Secret: corev1.SecretVolumeSource{
					SecretName: "redis-tls-cert",
				},
			},
		},
		ACLConfig: &redisv1beta2.ACLConfig{
			Secret: &corev1.SecretVolumeSource{
				SecretName: "acl-secret",
			},
		},
		EnvVars: &[]corev1.EnvVar{
			{
				Name:  "CUSTOM_ENV_VAR_1",
				Value: "custom_value_1",
			},
			{
				Name:  "CUSTOM_ENV_VAR_2",
				Value: "custom_value_2",
			},
		},
		AdditionalVolume: []corev1.Volume{
			{
				Name: "example-config",
				VolumeSource: corev1.VolumeSource{
					ConfigMap: &corev1.ConfigMapVolumeSource{
						LocalObjectReference: corev1.LocalObjectReference{
							Name: "example-configmap",
						},
					},
				},
			},
		},
		AdditionalMountPath: []corev1.VolumeMount{
			{
				MountPath: "/config",
				Name:      "example-config",
			},
		},
	}

	expectedFollowerContainer := containerParameters{
		Image:           "quay.io/opstree/redis:v7.0.12",
		ImagePullPolicy: corev1.PullPolicy("IfNotPresent"),
		Resources: &corev1.ResourceRequirements{
			Requests: corev1.ResourceList{
				corev1.ResourceCPU:    resource.MustParse("101m"),
				corev1.ResourceMemory: resource.MustParse("128Mi"),
			},
			Limits: corev1.ResourceList{
				corev1.ResourceCPU:    resource.MustParse("101m"),
				corev1.ResourceMemory: resource.MustParse("128Mi"),
			},
		},
		SecurityContext: &corev1.SecurityContext{
			RunAsUser:              pointer.Int64(1000),
			RunAsGroup:             pointer.Int64(1000),
			RunAsNonRoot:           pointer.Bool(true),
			ReadOnlyRootFilesystem: pointer.Bool(true),
			Capabilities: &corev1.Capabilities{
				Drop: []corev1.Capability{"ALL"},
				Add:  []corev1.Capability{"NET_BIND_SERVICE"},
			},
		},
		RedisExporterImage:           "quay.io/opstree/redis-exporter:v1.44.0",
		RedisExporterImagePullPolicy: corev1.PullPolicy("Always"),
		RedisExporterResources: &corev1.ResourceRequirements{
			Requests: corev1.ResourceList{
				corev1.ResourceCPU:    resource.MustParse("100m"),
				corev1.ResourceMemory: resource.MustParse("128Mi"),
			},
			Limits: corev1.ResourceList{
				corev1.ResourceCPU:    resource.MustParse("100m"),
				corev1.ResourceMemory: resource.MustParse("128Mi"),
			},
		},
		RedisExporterEnv: &[]corev1.EnvVar{
			{
				Name:  "REDIS_EXPORTER_INCL_SYSTEM_METRICS",
				Value: "true",
			},
			{
				Name: "UI_PROPERTIES_FILE_NAME",
				ValueFrom: &corev1.EnvVarSource{
					ConfigMapKeyRef: &corev1.ConfigMapKeySelector{
						LocalObjectReference: corev1.LocalObjectReference{
							Name: "game-demo",
						},
						Key: "ui_properties_file_name",
					},
				},
			},
			{
				Name: "SECRET_USERNAME",
				ValueFrom: &corev1.EnvVarSource{
					SecretKeyRef: &corev1.SecretKeySelector{
						LocalObjectReference: corev1.LocalObjectReference{
							Name: "mysecret",
						},
						Key: "username",
					},
				},
			},
		},
		Role:               "cluster",
		EnabledPassword:    pointer.Bool(true),
		SecretName:         pointer.String("redis-secret"),
		SecretKey:          pointer.String("password"),
		PersistenceEnabled: pointer.Bool(true),
		TLSConfig: &redisv1beta2.TLSConfig{
			TLSConfig: common.TLSConfig{
				CaKeyFile:   "ca.key",
				CertKeyFile: "tls.crt",
				KeyFile:     "tls.key",
				Secret: corev1.SecretVolumeSource{
					SecretName: "redis-tls-cert",
				},
			},
		},
		ACLConfig: &redisv1beta2.ACLConfig{
			Secret: &corev1.SecretVolumeSource{
				SecretName: "acl-secret",
			},
		},
		EnvVars: &[]corev1.EnvVar{
			{
				Name:  "CUSTOM_ENV_VAR_1",
				Value: "custom_value_1",
			},
			{
				Name:  "CUSTOM_ENV_VAR_2",
				Value: "custom_value_2",
			},
		},
		AdditionalVolume: []corev1.Volume{
			{
				Name: "example-config",
				VolumeSource: corev1.VolumeSource{
					ConfigMap: &corev1.ConfigMapVolumeSource{
						LocalObjectReference: corev1.LocalObjectReference{
							Name: "example-configmap",
						},
					},
				},
			},
		},
		AdditionalMountPath: []corev1.VolumeMount{
			{
				MountPath: "/config",
				Name:      "example-config",
			},
		},
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("Failed to read file %s: %v", path, err)
	}

	input := &redisv1beta2.RedisCluster{}
	err = yaml.UnmarshalStrict(data, input)
	if err != nil {
		t.Fatalf("Failed to unmarshal file %s: %v", path, err)
	}
	logger := testr.New(t)

	actualLeaderContainer := generateRedisClusterContainerParams(fake.NewSimpleClientset(), logger, input, input.Spec.RedisLeader.SecurityContext, input.Spec.RedisLeader.ReadinessProbe, input.Spec.RedisLeader.LivenessProbe, "leader")
	assert.EqualValues(t, expectedLeaderContainer, actualLeaderContainer, "Expected %+v, got %+v", expectedLeaderContainer, actualLeaderContainer)

	actualFollowerContainer := generateRedisClusterContainerParams(fake.NewSimpleClientset(), logger, input, input.Spec.RedisFollower.SecurityContext, input.Spec.RedisFollower.ReadinessProbe, input.Spec.RedisFollower.LivenessProbe, "follower")
	assert.EqualValues(t, expectedFollowerContainer, actualFollowerContainer, "Expected %+v, got %+v", expectedFollowerContainer, actualFollowerContainer)
}

func Test_generateRedisClusterInitContainerParams(t *testing.T) {
	path := filepath.Join("..", "tests", "testdata", "redis-cluster.yaml")
	expected := initContainerParameters{
		Enabled:         pointer.Bool(true),
		Image:           "quay.io/opstree/redis-operator-restore:latest",
		ImagePullPolicy: corev1.PullPolicy("Always"),
		Resources: &corev1.ResourceRequirements{
			Requests: corev1.ResourceList{
				corev1.ResourceCPU:    resource.MustParse("100m"),
				corev1.ResourceMemory: resource.MustParse("128Mi"),
			},
			Limits: corev1.ResourceList{
				corev1.ResourceCPU:    resource.MustParse("100m"),
				corev1.ResourceMemory: resource.MustParse("128Mi"),
			},
		},
		Role:               "cluster",
		Command:            []string{"/bin/bash", "-c", "/app/restore.bash"},
		Arguments:          []string{"--restore-from", "redis-cluster-restore"},
		PersistenceEnabled: pointer.Bool(true),
		AdditionalEnvVariable: &[]corev1.EnvVar{
			{
				Name: "CLUSTER_NAME",
				ValueFrom: &corev1.EnvVarSource{
					SecretKeyRef: &corev1.SecretKeySelector{
						LocalObjectReference: corev1.LocalObjectReference{
							Name: "env-secrets",
						},
						Key: "CLUSTER_NAME",
					},
				},
			},
			{
				Name: "CLUSTER_NAMESPACE",
				ValueFrom: &corev1.EnvVarSource{
					SecretKeyRef: &corev1.SecretKeySelector{
						LocalObjectReference: corev1.LocalObjectReference{
							Name: "env-secrets",
						},
						Key: "CLUSTER_NAMESPACE",
					},
				},
			},
		},
		AdditionalVolume: []corev1.Volume{
			{
				Name: "example-config",
				VolumeSource: corev1.VolumeSource{
					ConfigMap: &corev1.ConfigMapVolumeSource{
						LocalObjectReference: corev1.LocalObjectReference{
							Name: "example-configmap",
						},
					},
				},
			},
		},
		AdditionalMountPath: []corev1.VolumeMount{
			{
				MountPath: "/config",
				Name:      "example-config",
			},
		},
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("Failed to read file %s: %v", path, err)
	}

	input := &redisv1beta2.RedisCluster{}
	err = yaml.UnmarshalStrict(data, input)
	if err != nil {
		t.Fatalf("Failed to unmarshal file %s: %v", path, err)
	}

	actual := generateRedisClusterInitContainerParams(input)
	assert.EqualValues(t, expected, actual, "Expected %+v, got %+v", expected, actual)
}
