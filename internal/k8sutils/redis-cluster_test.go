package k8sutils

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strconv"
	"testing"

	common "github.com/OT-CONTAINER-KIT/redis-operator/api/common/v1beta2"
	rcvb2 "github.com/OT-CONTAINER-KIT/redis-operator/api/rediscluster/v1beta2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/yaml"
	"k8s.io/client-go/kubernetes/fake"
	k8stesting "k8s.io/client-go/testing"
	"k8s.io/utils/ptr"
)

func Test_generateRedisClusterParams(t *testing.T) {
	path := filepath.Join("..", "..", "tests", "testdata", "redis-cluster.yaml")

	expectedLeaderSTS := statefulSetParameters{
		Replicas:       ptr.To(int32(3)),
		ClusterMode:    true,
		NodeConfVolume: true,
		PodSecurityContext: &corev1.PodSecurityContext{
			RunAsUser: ptr.To(int64(1000)),
			FSGroup:   ptr.To(int64(1000)),
		},
		PriorityClassName:             "high-priority",
		MinReadySeconds:               5,
		TerminationGracePeriodSeconds: ptr.To(int64(60)),
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
				StorageClassName: ptr.To("standard"),
				AccessModes:      []corev1.PersistentVolumeAccessMode{corev1.ReadWriteOnce},
				Resources: corev1.VolumeResourceRequirements{
					Requests: corev1.ResourceList{
						corev1.ResourceStorage: resource.MustParse("1Gi"),
					},
				},
			},
		},
		NodeConfPersistentVolumeClaim: corev1.PersistentVolumeClaim{
			Spec: corev1.PersistentVolumeClaimSpec{
				StorageClassName: ptr.To("standard"),
				AccessModes:      []corev1.PersistentVolumeAccessMode{corev1.ReadWriteOnce},
				Resources: corev1.VolumeResourceRequirements{
					Requests: corev1.ResourceList{
						corev1.ResourceStorage: resource.MustParse("1Gi"),
					},
				},
			},
		},
		EnableMetrics:      true,
		ImagePullSecrets:   &[]corev1.LocalObjectReference{{Name: "mysecret"}},
		ExternalConfig:     ptr.To("redis-external-config-leader"),
		ServiceAccountName: ptr.To("redis-sa"),
		IgnoreAnnotations:  []string{"opstreelabs.in/ignore"},
	}
	expectedFollowerSTS := statefulSetParameters{
		Replicas:       ptr.To(int32(3)),
		ClusterMode:    true,
		NodeConfVolume: true,
		PodSecurityContext: &corev1.PodSecurityContext{
			RunAsUser: ptr.To(int64(1000)),
			FSGroup:   ptr.To(int64(1000)),
		},
		PriorityClassName:             "high-priority",
		MinReadySeconds:               5,
		TerminationGracePeriodSeconds: ptr.To(int64(45)),
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
				StorageClassName: ptr.To("standard"),
				AccessModes:      []corev1.PersistentVolumeAccessMode{corev1.ReadWriteOnce},
				Resources: corev1.VolumeResourceRequirements{
					Requests: corev1.ResourceList{
						corev1.ResourceStorage: resource.MustParse("1Gi"),
					},
				},
			},
		},
		NodeConfPersistentVolumeClaim: corev1.PersistentVolumeClaim{
			Spec: corev1.PersistentVolumeClaimSpec{
				StorageClassName: ptr.To("standard"),
				AccessModes:      []corev1.PersistentVolumeAccessMode{corev1.ReadWriteOnce},
				Resources: corev1.VolumeResourceRequirements{
					Requests: corev1.ResourceList{
						corev1.ResourceStorage: resource.MustParse("1Gi"),
					},
				},
			},
		},
		EnableMetrics:      true,
		ImagePullSecrets:   &[]corev1.LocalObjectReference{{Name: "mysecret"}},
		ExternalConfig:     ptr.To("redis-external-config-follower"),
		ServiceAccountName: ptr.To("redis-sa"),
		IgnoreAnnotations:  []string{"opstreelabs.in/ignore"},
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("Failed to read file %s: %v", path, err)
	}

	input := &rcvb2.RedisCluster{}
	err = yaml.UnmarshalStrict(data, input)
	if err != nil {
		t.Fatalf("Failed to unmarshal file %s: %v", path, err)
	}

	actualLeaderSTS := generateRedisClusterParams(context.TODO(), input, *input.Spec.ClusterSize, input.Spec.RedisLeader.RedisConfig.AdditionalRedisConfig, RedisClusterSTS{
		RedisStateFulType:             "leader",
		ExternalConfig:                input.Spec.RedisLeader.RedisConfig.AdditionalRedisConfig,
		SecurityContext:               input.Spec.RedisLeader.SecurityContext,
		Affinity:                      input.Spec.RedisLeader.Affinity,
		TerminationGracePeriodSeconds: input.Spec.RedisLeader.TerminationGracePeriodSeconds,
		ReadinessProbe:                input.Spec.RedisLeader.ReadinessProbe,
		LivenessProbe:                 input.Spec.RedisLeader.LivenessProbe,
		NodeSelector:                  input.Spec.RedisLeader.NodeSelector,
		TopologySpreadConstraints:     input.Spec.RedisLeader.TopologySpreadConstraints,
		Tolerations:                   input.Spec.RedisLeader.Tolerations,
	})
	assert.EqualValues(t, expectedLeaderSTS, actualLeaderSTS, "Expected %+v, got %+v", expectedLeaderSTS, actualLeaderSTS)

	actualFollowerSTS := generateRedisClusterParams(context.TODO(), input, *input.Spec.ClusterSize, input.Spec.RedisFollower.RedisConfig.AdditionalRedisConfig, RedisClusterSTS{
		RedisStateFulType:             "follower",
		ExternalConfig:                input.Spec.RedisFollower.RedisConfig.AdditionalRedisConfig,
		SecurityContext:               input.Spec.RedisFollower.SecurityContext,
		Affinity:                      input.Spec.RedisFollower.Affinity,
		TerminationGracePeriodSeconds: input.Spec.RedisFollower.TerminationGracePeriodSeconds,
		ReadinessProbe:                input.Spec.RedisFollower.ReadinessProbe,
		LivenessProbe:                 input.Spec.RedisFollower.LivenessProbe,
		NodeSelector:                  input.Spec.RedisFollower.NodeSelector,
		TopologySpreadConstraints:     input.Spec.RedisFollower.TopologySpreadConstraints,
		Tolerations:                   input.Spec.RedisFollower.Tolerations,
	})
	assert.EqualValues(t, expectedFollowerSTS, actualFollowerSTS, "Expected %+v, got %+v", expectedFollowerSTS, actualFollowerSTS)
}

func Test_generateRedisClusterContainerParams(t *testing.T) {
	path := filepath.Join("..", "..", "tests", "testdata", "redis-cluster.yaml")
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
			RunAsUser:              ptr.To(int64(1000)),
			RunAsGroup:             ptr.To(int64(1000)),
			RunAsNonRoot:           ptr.To(true),
			ReadOnlyRootFilesystem: ptr.To(true),
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
		EnabledPassword:    ptr.To(true),
		SecretName:         ptr.To("redis-secret"),
		SecretKey:          ptr.To("password"),
		PersistenceEnabled: ptr.To(true),
		PreStopWaitSeconds: 50, // redisLeader.terminationGracePeriodSeconds 60 - 10s headroom
		TLSConfig: &common.TLSConfig{
			CaCertFile:  "ca.crt",
			CertKeyFile: "tls.crt",
			KeyFile:     "tls.key",
			Secret: corev1.SecretVolumeSource{
				SecretName: "redis-tls-cert",
			},
		},
		ACLConfig: &common.ACLConfig{
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
			RunAsUser:              ptr.To(int64(1000)),
			RunAsGroup:             ptr.To(int64(1000)),
			RunAsNonRoot:           ptr.To(true),
			ReadOnlyRootFilesystem: ptr.To(true),
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
		EnabledPassword:    ptr.To(true),
		SecretName:         ptr.To("redis-secret"),
		SecretKey:          ptr.To("password"),
		PersistenceEnabled: ptr.To(true),
		PreStopWaitSeconds: 35, // redisFollower.terminationGracePeriodSeconds 45 - 10s headroom
		TLSConfig: &common.TLSConfig{
			CaCertFile:  "ca.crt",
			CertKeyFile: "tls.crt",
			KeyFile:     "tls.key",
			Secret: corev1.SecretVolumeSource{
				SecretName: "redis-tls-cert",
			},
		},
		ACLConfig: &common.ACLConfig{
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

	input := &rcvb2.RedisCluster{}
	err = yaml.UnmarshalStrict(data, input)
	if err != nil {
		t.Fatalf("Failed to unmarshal file %s: %v", path, err)
	}

	actualLeaderContainer, err := generateRedisClusterContainerParams(context.TODO(), fake.NewSimpleClientset(), input, input.Spec.RedisLeader.SecurityContext, input.Spec.RedisLeader.ReadinessProbe, input.Spec.RedisLeader.LivenessProbe, "leader", input.Spec.GetRedisLeaderResources())
	require.NoError(t, err)
	assert.EqualValues(t, expectedLeaderContainer, actualLeaderContainer, "Expected %+v, got %+v", expectedLeaderContainer, actualLeaderContainer)

	actualFollowerContainer, err := generateRedisClusterContainerParams(context.TODO(), fake.NewSimpleClientset(), input, input.Spec.RedisFollower.SecurityContext, input.Spec.RedisFollower.ReadinessProbe, input.Spec.RedisFollower.LivenessProbe, "follower", input.Spec.GetRedisFollowerResources())
	require.NoError(t, err)
	assert.EqualValues(t, expectedFollowerContainer, actualFollowerContainer, "Expected %+v, got %+v", expectedFollowerContainer, actualFollowerContainer)
}

func Test_generateRedisClusterInitContainerParams(t *testing.T) {
	path := filepath.Join("..", "..", "tests", "testdata", "redis-cluster.yaml")
	expected := initContainerParameters{
		Enabled:         ptr.To(true),
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
		PersistenceEnabled: ptr.To(true),
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
		SecurityContext: &corev1.SecurityContext{
			RunAsUser:                ptr.To(int64(1000)),
			RunAsGroup:               ptr.To(int64(1000)),
			AllowPrivilegeEscalation: ptr.To(false),
			ReadOnlyRootFilesystem:   ptr.To(true),
			Capabilities: &corev1.Capabilities{
				Drop: []corev1.Capability{"ALL"},
				Add:  []corev1.Capability{"NET_BIND_SERVICE"},
			},
		},
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("Failed to read file %s: %v", path, err)
	}

	input := &rcvb2.RedisCluster{}
	err = yaml.UnmarshalStrict(data, input)
	if err != nil {
		t.Fatalf("Failed to unmarshal file %s: %v", path, err)
	}

	actual := generateRedisClusterInitContainerParams(input)
	assert.EqualValues(t, expected, actual, "Expected %+v, got %+v", expected, actual)
}

func Test_generateRedisClusterInitContainerParams_PersistenceDisabled(t *testing.T) {
	enabled := true
	persistenceEnabled := false

	input := &rcvb2.RedisCluster{
		Spec: rcvb2.RedisClusterSpec{
			PersistenceEnabled: &persistenceEnabled,
			Storage:            &rcvb2.ClusterStorage{},
			InitContainer: &common.InitContainer{
				Enabled: &enabled,
				Image:   "busybox:latest",
			},
		},
	}

	actual := generateRedisClusterInitContainerParams(input)
	if actual.PersistenceEnabled != nil {
		t.Fatalf("Expected PersistenceEnabled to be nil when persistence is disabled, got %v", *actual.PersistenceEnabled)
	}
}

func TestEnsureRedisClusterNodePortServices(t *testing.T) {
	newRedisCluster := func(serviceType string, replicas int32) *rcvb2.RedisCluster {
		cluster := &rcvb2.RedisCluster{
			TypeMeta: metav1.TypeMeta{
				APIVersion: "redis.redis.opstreelabs.in/v1beta2",
				Kind:       "RedisCluster",
			},
			ObjectMeta: metav1.ObjectMeta{
				Name:      "redis-cluster",
				Namespace: "redis",
			},
			Spec: rcvb2.RedisClusterSpec{
				ClusterSize: ptr.To(replicas),
				KubernetesConfig: common.KubernetesConfig{
					Service: &common.ServiceConfig{ServiceType: serviceType},
				},
			},
		}
		cluster.SetDefault()
		return cluster
	}

	t.Run("does nothing for a non-NodePort cluster", func(t *testing.T) {
		client := fake.NewSimpleClientset()
		err := EnsureRedisClusterNodePortServices(t.Context(), newRedisCluster("ClusterIP", 3), "leader", client)

		require.NoError(t, err)
		assert.Empty(t, client.Actions())
	})

	t.Run("creates every missing per-pod service", func(t *testing.T) {
		client := fake.NewSimpleClientset()
		err := EnsureRedisClusterNodePortServices(t.Context(), newRedisCluster("NodePort", 3), "follower", client)

		require.NoError(t, err)
		services, err := client.CoreV1().Services("redis").List(t.Context(), metav1.ListOptions{})
		require.NoError(t, err)
		require.Len(t, services.Items, 3)
		for i := 0; i < 3; i++ {
			name := "redis-cluster-follower-" + strconv.Itoa(i)
			service, err := client.CoreV1().Services("redis").Get(t.Context(), name, metav1.GetOptions{})
			require.NoError(t, err)
			assert.Equal(t, corev1.ServiceTypeNodePort, service.Spec.Type)
			assert.Equal(t, name, service.Spec.Selector["statefulset.kubernetes.io/pod-name"])

			ports := map[string]int32{}
			for _, port := range service.Spec.Ports {
				ports[port.Name] = port.Port
			}
			assert.Equal(t, map[string]int32{"redis-client": 6379, "redis-bus": 16379}, ports)

			require.Len(t, service.OwnerReferences, 1)
			assert.Equal(t, "RedisCluster", service.OwnerReferences[0].Kind)
			assert.Equal(t, "redis-cluster", service.OwnerReferences[0].Name)
		}
	})

	t.Run("creates only the service added by scale-up", func(t *testing.T) {
		existingServices := make([]runtime.Object, 0, 3)
		for i := 0; i < 3; i++ {
			existingServices = append(existingServices, &corev1.Service{
				ObjectMeta: metav1.ObjectMeta{
					Name:        "redis-cluster-leader-" + strconv.Itoa(i),
					Namespace:   "redis",
					Annotations: map[string]string{"preserve": "true"},
				},
				Spec: corev1.ServiceSpec{Selector: map[string]string{"preserve": "true"}},
			})
		}
		client := fake.NewSimpleClientset(existingServices...)

		err := EnsureRedisClusterNodePortServices(t.Context(), newRedisCluster("NodePort", 4), "leader", client)

		require.NoError(t, err)
		services, err := client.CoreV1().Services("redis").List(t.Context(), metav1.ListOptions{})
		require.NoError(t, err)
		require.Len(t, services.Items, 4)
		for i := 0; i < 3; i++ {
			service, err := client.CoreV1().Services("redis").Get(t.Context(), "redis-cluster-leader-"+strconv.Itoa(i), metav1.GetOptions{})
			require.NoError(t, err)
			assert.Equal(t, "true", service.Annotations["preserve"])
			assert.Equal(t, "true", service.Spec.Selector["preserve"])
		}
		var createActions, updateActions int
		for _, action := range client.Actions() {
			switch action.GetVerb() {
			case "create":
				createActions++
			case "update":
				updateActions++
			}
		}
		assert.Equal(t, 1, createActions)
		assert.Zero(t, updateActions)
	})

	t.Run("returns a service lookup error", func(t *testing.T) {
		client := fake.NewSimpleClientset()
		client.PrependReactor("get", "services", func(k8stesting.Action) (bool, runtime.Object, error) {
			return true, nil, errors.New("service lookup failed")
		})

		err := EnsureRedisClusterNodePortServices(t.Context(), newRedisCluster("NodePort", 3), "leader", client)

		require.EqualError(t, err, "service lookup failed")
	})

	t.Run("never updates a service that appears between the lookup and the create", func(t *testing.T) {
		client := fake.NewSimpleClientset()
		// The lookup misses but the Service exists by the time the create is issued,
		// which must not turn the preflight into an update of a live Service.
		client.PrependReactor("get", "services", func(action k8stesting.Action) (bool, runtime.Object, error) {
			return true, nil, apierrors.NewNotFound(corev1.Resource("services"), action.(k8stesting.GetAction).GetName())
		})
		client.PrependReactor("create", "services", func(action k8stesting.Action) (bool, runtime.Object, error) {
			service := action.(k8stesting.CreateAction).GetObject().(*corev1.Service)
			return true, nil, apierrors.NewAlreadyExists(corev1.Resource("services"), service.Name)
		})

		err := EnsureRedisClusterNodePortServices(t.Context(), newRedisCluster("NodePort", 3), "leader", client)

		require.NoError(t, err)
		for _, action := range client.Actions() {
			assert.NotEqual(t, "update", action.GetVerb())
			assert.NotEqual(t, "patch", action.GetVerb())
		}
	})
}

func Test_generateRedisClusterContainerParams_NodePort(t *testing.T) {
	newNodePortCluster := func(replicas int32, envVars *[]corev1.EnvVar) *rcvb2.RedisCluster {
		cluster := &rcvb2.RedisCluster{
			ObjectMeta: metav1.ObjectMeta{Name: "redis-cluster", Namespace: "redis"},
			Spec: rcvb2.RedisClusterSpec{
				ClusterSize: ptr.To(replicas),
				EnvVars:     envVars,
				KubernetesConfig: common.KubernetesConfig{
					Service: &common.ServiceConfig{ServiceType: "NodePort"},
				},
			},
		}
		cluster.SetDefault()
		return cluster
	}
	nodePortService := func(name string, ports ...corev1.ServicePort) *corev1.Service {
		return &corev1.Service{
			ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: "redis"},
			Spec:       corev1.ServiceSpec{Type: corev1.ServiceTypeNodePort, Ports: ports},
		}
	}
	clientPort := func(nodePort int32) corev1.ServicePort {
		return corev1.ServicePort{Name: "redis-client", Port: 6379, NodePort: nodePort}
	}
	busPort := func(nodePort int32) corev1.ServicePort {
		return corev1.ServicePort{Name: "redis-bus", Port: 16379, NodePort: nodePort}
	}
	envValues := func(params containerParameters) map[string]string {
		values := map[string]string{}
		require.NotNil(t, params.EnvVars)
		for _, env := range *params.EnvVars {
			values[env.Name] = env.Value
		}
		return values
	}

	t.Run("fails instead of rendering a template without announce variables", func(t *testing.T) {
		client := fake.NewSimpleClientset()

		_, err := generateRedisClusterContainerParams(t.Context(), client, newNodePortCluster(1, nil), nil, nil, nil, "leader", nil)

		require.Error(t, err)
		assert.Contains(t, err.Error(), "redis/redis-cluster-leader-0")
	})

	t.Run("fails when the node ports are not allocated yet", func(t *testing.T) {
		client := fake.NewSimpleClientset(nodePortService("redis-cluster-leader-0", clientPort(0), busPort(0)))

		_, err := generateRedisClusterContainerParams(t.Context(), client, newNodePortCluster(1, nil), nil, nil, nil, "leader", nil)

		require.Error(t, err)
		assert.Contains(t, err.Error(), "no allocated")
	})

	t.Run("fails when the cluster bus port is missing from the service", func(t *testing.T) {
		client := fake.NewSimpleClientset(nodePortService("redis-cluster-leader-0", clientPort(30000)))

		_, err := generateRedisClusterContainerParams(t.Context(), client, newNodePortCluster(1, nil), nil, nil, nil, "leader", nil)

		require.Error(t, err)
		assert.Contains(t, err.Error(), "no allocated")
	})

	t.Run("matches ports by name rather than by position", func(t *testing.T) {
		client := fake.NewSimpleClientset(nodePortService("redis-cluster-leader-0", busPort(31000), clientPort(30000)))

		params, err := generateRedisClusterContainerParams(t.Context(), client, newNodePortCluster(1, nil), nil, nil, nil, "leader", nil)

		require.NoError(t, err)
		values := envValues(params)
		assert.Equal(t, "30000", values["announce_port_redis_cluster_leader_0"])
		assert.Equal(t, "31000", values["announce_bus_port_redis_cluster_leader_0"])
	})

	t.Run("does not leak announce variables between the leader and follower render", func(t *testing.T) {
		client := fake.NewSimpleClientset(
			nodePortService("redis-cluster-leader-0", clientPort(30000), busPort(31000)),
			nodePortService("redis-cluster-follower-0", clientPort(30001), busPort(31001)),
		)
		// A user supplied env slice is shared by both renders of the same reconciliation.
		cr := newNodePortCluster(1, &[]corev1.EnvVar{{Name: "USER_DEFINED", Value: "yes"}})

		leader, err := generateRedisClusterContainerParams(t.Context(), client, cr, nil, nil, nil, "leader", nil)
		require.NoError(t, err)
		follower, err := generateRedisClusterContainerParams(t.Context(), client, cr, nil, nil, nil, "follower", nil)
		require.NoError(t, err)

		leaderValues := envValues(leader)
		assert.Equal(t, "30000", leaderValues["announce_port_redis_cluster_leader_0"])
		assert.NotContains(t, leaderValues, "announce_port_redis_cluster_follower_0")

		followerValues := envValues(follower)
		assert.Equal(t, "30001", followerValues["announce_port_redis_cluster_follower_0"])
		assert.NotContains(t, followerValues, "announce_port_redis_cluster_leader_0")

		var nodePortEnvCount int
		for _, env := range *follower.EnvVars {
			if env.Name == "NODEPORT" {
				nodePortEnvCount++
			}
		}
		assert.Equal(t, 1, nodePortEnvCount, "NODEPORT must not be appended twice")

		require.NotNil(t, cr.Spec.EnvVars)
		assert.Len(t, *cr.Spec.EnvVars, 1, "the cluster spec must not be mutated by rendering")
	})
}
