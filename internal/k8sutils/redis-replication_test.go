package k8sutils

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	common "github.com/OT-CONTAINER-KIT/redis-operator/api/common/v1beta2"
	rrvb2 "github.com/OT-CONTAINER-KIT/redis-operator/api/redisreplication/v1beta2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/yaml"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	"k8s.io/utils/ptr"
	clientfake "sigs.k8s.io/controller-runtime/pkg/client/fake"
)

func Test_generateRedisReplicationParams(t *testing.T) {
	path := filepath.Join("..", "..", "tests", "testdata", "redis-replication.yaml")
	expected := statefulSetParameters{
		Replicas:       ptr.To(int32(3)),
		ClusterMode:    false,
		NodeConfVolume: false,
		NodeSelector: map[string]string{
			"node-role.kubernetes.io/infra": "worker",
		},
		TopologySpreadConstraints: []corev1.TopologySpreadConstraint{
			{
				MaxSkew:           1,
				TopologyKey:       "kubernetes.io/hostname",
				WhenUnsatisfiable: corev1.ScheduleAnyway,
				LabelSelector: &metav1.LabelSelector{
					MatchLabels: map[string]string{
						"role": "replication",
						"app":  "redis-replication",
					},
				},
			},
		},
		PodSecurityContext: &corev1.PodSecurityContext{
			RunAsUser: ptr.To(int64(1000)),
			FSGroup:   ptr.To(int64(1000)),
		},
		PriorityClassName: "high-priority",
		MinReadySeconds:   5,
		Affinity: &corev1.Affinity{
			NodeAffinity: &corev1.NodeAffinity{
				RequiredDuringSchedulingIgnoredDuringExecution: &corev1.NodeSelector{
					NodeSelectorTerms: []corev1.NodeSelectorTerm{
						{
							MatchExpressions: []corev1.NodeSelectorRequirement{
								{
									Key:      "node-role.kubernetes.io/infra",
									Operator: corev1.NodeSelectorOpIn,
									Values:   []string{"worker"},
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
		EnableMetrics:                 true,
		ImagePullSecrets:              &[]corev1.LocalObjectReference{{Name: "mysecret"}},
		ExternalConfig:                ptr.To("redis-external-config"),
		ServiceAccountName:            ptr.To("redis-sa"),
		TerminationGracePeriodSeconds: ptr.To(int64(30)),
		IgnoreAnnotations:             []string{"opstreelabs.in/ignore"},
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("Failed to read file %s: %v", path, err)
	}

	input := &rrvb2.RedisReplication{}
	err = yaml.UnmarshalStrict(data, input)
	if err != nil {
		t.Fatalf("Failed to unmarshal file %s: %v", path, err)
	}
	actual := generateRedisReplicationParams(input)
	assert.EqualValues(t, expected, actual, "Expected %+v, got %+v", expected, actual)
}

func Test_generateRedisReplicationContainerParams(t *testing.T) {
	path := filepath.Join("..", "..", "tests", "testdata", "redis-replication.yaml")
	expected := containerParameters{
		Image:           "quay.io/opstree/redis:v7.0.12",
		Port:            ptr.To(6379),
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
		Role:               "replication",
		EnabledPassword:    ptr.To(true),
		SecretName:         ptr.To("redis-secret"),
		SecretKey:          ptr.To("password"),
		PersistenceEnabled: ptr.To(true),
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
			// Always present so the bootstrap agent can gate replica-announce-ip
			// on whether the monitoring Sentinel resolves hostnames.
			{
				Name:  "RESOLVE_HOSTNAMES",
				Value: "no",
			},
			{
				Name:  "ANNOUNCE_HOSTNAMES",
				Value: "no",
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

	input := &rrvb2.RedisReplication{}
	err = yaml.UnmarshalStrict(data, input)
	if err != nil {
		t.Fatalf("Failed to unmarshal file %s: %v", path, err)
	}

	actual := generateRedisReplicationContainerParams(context.TODO(), input, nil)
	assert.EqualValues(t, expected, actual, "Expected %+v, got %+v", expected, actual)
}

func Test_generateRedisReplicationContainerParams_SentinelPreStop(t *testing.T) {
	base := func() *rrvb2.RedisReplication {
		return &rrvb2.RedisReplication{
			ObjectMeta: metav1.ObjectMeta{Name: "my-replication"},
			Spec: rrvb2.RedisReplicationSpec{
				Size: ptr.To(int32(3)),
			},
		}
	}

	t.Run("sentinel disabled leaves preStop fields empty", func(t *testing.T) {
		got := generateRedisReplicationContainerParams(context.TODO(), base(), nil)
		assert.Empty(t, got.SentinelService)
		assert.Empty(t, got.SentinelMasterName)
		assert.Zero(t, got.SentinelPort)
		assert.Zero(t, got.PreStopWaitSeconds)
	})

	t.Run("embedded sentinel wires preStop from CR config", func(t *testing.T) {
		cr := base()
		cr.Spec.Sentinel = &rrvb2.Sentinel{Size: 3}
		cr.Spec.TerminationGracePeriodSeconds = ptr.To(int64(60))

		got := generateRedisReplicationContainerParams(context.TODO(), cr, nil)
		assert.Equal(t, "my-replication-s-hl", got.SentinelService)
		assert.Equal(t, "mymaster", got.SentinelMasterName)
		assert.Equal(t, 26379, got.SentinelPort)
		// grace(60) - headroom(10) => 50s wait, leaving headroom before SIGKILL.
		assert.Equal(t, 50, got.PreStopWaitSeconds)
	})

	t.Run("embedded sentinel with size 0 stays disabled", func(t *testing.T) {
		cr := base()
		cr.Spec.Sentinel = &rrvb2.Sentinel{Size: 0}

		got := generateRedisReplicationContainerParams(context.TODO(), cr, nil)
		assert.Empty(t, got.SentinelService)
	})
}

func Test_passwordSecretChecksum(t *testing.T) {
	scheme := runtime.NewScheme()
	require.NoError(t, clientgoscheme.AddToScheme(scheme))

	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{Name: "redis-secret", Namespace: "default"},
		Data:       map[string][]byte{"password": []byte("old-password")},
	}
	ctrlClient := clientfake.NewClientBuilder().WithScheme(scheme).WithObjects(secret).Build()

	checksum := passwordSecretChecksum(context.TODO(), ctrlClient, "default", "redis-secret")
	assert.NotEmpty(t, checksum)

	rotated := secret.DeepCopy()
	rotated.Data["password"] = []byte("new-password")
	require.NoError(t, ctrlClient.Update(context.TODO(), rotated))

	rotatedChecksum := passwordSecretChecksum(context.TODO(), ctrlClient, "default", "redis-secret")
	assert.NotEmpty(t, rotatedChecksum)
	assert.NotEqual(t, checksum, rotatedChecksum, "checksum must change when the secret's password rotates")

	assert.Empty(t, passwordSecretChecksum(context.TODO(), ctrlClient, "default", "missing-secret"), "unreadable secret yields an empty checksum instead of failing reconcile")
	assert.Empty(t, passwordSecretChecksum(context.TODO(), nil, "default", "redis-secret"), "nil ctrlClient yields an empty checksum instead of panicking")
}

func Test_generateRedisReplicationContainerParams_SecretChecksum(t *testing.T) {
	scheme := runtime.NewScheme()
	require.NoError(t, clientgoscheme.AddToScheme(scheme))

	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{Name: "redis-secret", Namespace: "default"},
		Data:       map[string][]byte{"password": []byte("s3cr3t")},
	}
	ctrlClient := clientfake.NewClientBuilder().WithScheme(scheme).WithObjects(secret).Build()

	cr := &rrvb2.RedisReplication{
		ObjectMeta: metav1.ObjectMeta{Name: "example-replication", Namespace: "default"},
		Spec: rrvb2.RedisReplicationSpec{
			Size: ptr.To(int32(3)),
		},
	}
	cr.Spec.KubernetesConfig.ExistingPasswordSecret = &common.ExistingPasswordSecret{
		Name: ptr.To("redis-secret"),
		Key:  ptr.To("password"),
	}

	got := generateRedisReplicationContainerParams(context.TODO(), cr, ctrlClient)
	want := passwordSecretChecksum(context.TODO(), ctrlClient, "default", "redis-secret")
	assert.NotEmpty(t, want)
	assert.Equal(t, want, got.SecretChecksum)
}

func Test_generateRedisReplicationInitContainerParams(t *testing.T) {
	path := filepath.Join("..", "..", "tests", "testdata", "redis-replication.yaml")
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
		Role:               "replication",
		Command:            []string{"/bin/bash", "-c", "/app/restore.bash"},
		Arguments:          []string{"--restore-from", "redis-replication-restore"},
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
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("Failed to read file %s: %v", path, err)
	}

	input := &rrvb2.RedisReplication{}
	err = yaml.UnmarshalStrict(data, input)
	if err != nil {
		t.Fatalf("Failed to unmarshal file %s: %v", path, err)
	}

	actual := generateRedisReplicationInitContainerParams(input)
	assert.EqualValues(t, expected, actual, "Expected %+v, got %+v", expected, actual)
}
