package redisreplication

import (
	"context"
	"testing"

	commonapi "github.com/OT-CONTAINER-KIT/redis-operator/api/common/v1beta2"
	rrvb2 "github.com/OT-CONTAINER-KIT/redis-operator/api/redisreplication/v1beta2"
	"github.com/OT-CONTAINER-KIT/redis-operator/internal/service/redis"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"
	"k8s.io/utils/ptr"
)

// TestSentinelExistingPasswordSecretResolvesToEmbeddedRedisSecret guards against the
// shadowing regression: the Sentinel struct must not declare its own outer
// ExistingPasswordSecret field, otherwise it shadows the embedded
// KubernetesConfig.ExistingPasswordSecret (json "redisSecret") and the
// controller silently drops the configured Sentinel password.
func TestSentinelExistingPasswordSecretResolvesToEmbeddedRedisSecret(t *testing.T) {
	secret := &commonapi.ExistingPasswordSecret{
		Name: ptr.To("sentinel-secret"),
		Key:  ptr.To("password"),
	}
	// Set only the embedded KubernetesConfig.ExistingPasswordSecret (json
	// "redisSecret") via a composite literal...
	sentinel := &rrvb2.Sentinel{
		KubernetesConfig: commonapi.KubernetesConfig{ExistingPasswordSecret: secret},
	}

	// ...and require that the promoted selector resolves to it, rather than to a
	// separate (nil) outer field.
	assert.Same(t, secret, sentinel.ExistingPasswordSecret)
}

func TestBuildSentinelPodTemplate(t *testing.T) {
	affinity := &corev1.Affinity{
		PodAntiAffinity: &corev1.PodAntiAffinity{
			RequiredDuringSchedulingIgnoredDuringExecution: []corev1.PodAffinityTerm{
				{TopologyKey: "kubernetes.io/hostname"},
			},
		},
	}
	tolerations := []corev1.Toleration{{Key: "dedicated", Operator: corev1.TolerationOpExists}}
	nodeSelector := map[string]string{"disktype": "ssd"}
	topology := []corev1.TopologySpreadConstraint{{
		MaxSkew:           1,
		TopologyKey:       "kubernetes.io/hostname",
		WhenUnsatisfiable: corev1.DoNotSchedule,
	}}
	podSecurityContext := &corev1.PodSecurityContext{FSGroup: ptr.To(int64(1000))}
	imagePullSecrets := []corev1.LocalObjectReference{{Name: "registry-creds"}}
	graceperiod := int64(42)

	rr := &rrvb2.RedisReplication{
		ObjectMeta: metav1.ObjectMeta{Name: "example-replication", Namespace: "default"},
		Spec: rrvb2.RedisReplicationSpec{
			Sentinel: &rrvb2.Sentinel{
				Size:                          3,
				Affinity:                      affinity,
				Tolerations:                   &tolerations,
				NodeSelector:                  nodeSelector,
				TopologySpreadConstraints:     topology,
				PodSecurityContext:            podSecurityContext,
				PriorityClassName:             "high-priority",
				TerminationGracePeriodSeconds: ptr.To(graceperiod),
				ServiceAccountName:            ptr.To("sentinel-sa"),
			},
		},
	}
	rr.Spec.Sentinel.ImagePullSecrets = &imagePullSecrets

	labels := map[string]string{"app": "sentinel"}
	template := buildSentinelPodTemplate(rr, labels)
	spec := template.Spec

	assert.Equal(t, labels, template.Labels)
	assert.Same(t, affinity, spec.Affinity)
	assert.Equal(t, tolerations, spec.Tolerations)
	assert.Equal(t, nodeSelector, spec.NodeSelector)
	assert.Equal(t, topology, spec.TopologySpreadConstraints)
	assert.Same(t, podSecurityContext, spec.SecurityContext)
	assert.Equal(t, "high-priority", spec.PriorityClassName)
	require.NotNil(t, spec.TerminationGracePeriodSeconds)
	assert.Equal(t, graceperiod, *spec.TerminationGracePeriodSeconds)
	assert.Equal(t, imagePullSecrets, spec.ImagePullSecrets)
	assert.Equal(t, "sentinel-sa", spec.ServiceAccountName)
	require.Len(t, spec.Containers, 1)
	assert.Equal(t, "sentinel", spec.Containers[0].Name)
}

// TestBuildSentinelPodTemplateOmitsUnsetPlacement ensures nil/empty placement
// fields do not leak into the PodSpec (defaults stay zero-valued).
func TestBuildSentinelPodTemplateOmitsUnsetPlacement(t *testing.T) {
	rr := &rrvb2.RedisReplication{
		Spec: rrvb2.RedisReplicationSpec{
			Sentinel: &rrvb2.Sentinel{Size: 3},
		},
	}

	spec := buildSentinelPodTemplate(rr, nil).Spec

	assert.Nil(t, spec.Affinity)
	assert.Nil(t, spec.Tolerations)
	assert.Nil(t, spec.NodeSelector)
	assert.Nil(t, spec.TopologySpreadConstraints)
	assert.Nil(t, spec.SecurityContext)
	assert.Empty(t, spec.PriorityClassName)
	assert.Nil(t, spec.TerminationGracePeriodSeconds)
	assert.Nil(t, spec.ImagePullSecrets)
	assert.Empty(t, spec.ServiceAccountName)
}

func TestBuildSentinelContainer(t *testing.T) {
	resources := &corev1.ResourceRequirements{
		Limits: corev1.ResourceList{corev1.ResourceCPU: resource.MustParse("100m")},
	}
	securityContext := &corev1.SecurityContext{RunAsNonRoot: ptr.To(true)}

	rr := &rrvb2.RedisReplication{
		Spec: rrvb2.RedisReplicationSpec{
			Sentinel: &rrvb2.Sentinel{
				Size:            3,
				SecurityContext: securityContext,
			},
		},
	}
	rr.Spec.Sentinel.Image = "quay.io/opstree/redis-sentinel:v7.0.15"
	rr.Spec.Sentinel.ImagePullPolicy = corev1.PullAlways
	rr.Spec.Sentinel.Resources = resources

	container := buildSentinelContainer(rr)

	assert.Equal(t, "sentinel", container.Name)
	assert.Equal(t, "quay.io/opstree/redis-sentinel:v7.0.15", container.Image)
	assert.Equal(t, corev1.PullAlways, container.ImagePullPolicy)
	assert.Equal(t, *resources, container.Resources)
	assert.Same(t, securityContext, container.SecurityContext)
	require.Len(t, container.Ports, 1)
	assert.Equal(t, int32(26379), container.Ports[0].ContainerPort)
}

func TestBuildSentinelEnv(t *testing.T) {
	redisSecret := &commonapi.ExistingPasswordSecret{
		Name: ptr.To("redis-secret"),
		Key:  ptr.To("redis-password"),
	}
	sentinelSecret := &commonapi.ExistingPasswordSecret{
		Name: ptr.To("sentinel-secret"),
		Key:  ptr.To("sentinel-password"),
	}

	newRR := func(redisPwd, sentinelPwd *commonapi.ExistingPasswordSecret) *rrvb2.RedisReplication {
		rr := &rrvb2.RedisReplication{
			Spec: rrvb2.RedisReplicationSpec{
				Sentinel: &rrvb2.Sentinel{Size: 3},
			},
		}
		rr.Spec.KubernetesConfig.ExistingPasswordSecret = redisPwd
		rr.Spec.Sentinel.ExistingPasswordSecret = sentinelPwd
		return rr
	}

	t.Run("no password secret only sets quorum", func(t *testing.T) {
		envs := buildSentinelEnv(newRR(nil, nil))
		require.Len(t, envs, 1)
		assert.Equal(t, "QUORUM", envs[0].Name)
		assert.Equal(t, "2", envs[0].Value) // size 3 -> 3/2+1
	})

	t.Run("falls back to top-level redis secret", func(t *testing.T) {
		envs := buildSentinelEnv(newRR(redisSecret, nil))
		assertMasterPassword(t, envs, "redis-secret", "redis-password")
	})

	t.Run("sentinel redisSecret overrides top-level", func(t *testing.T) {
		envs := buildSentinelEnv(newRR(redisSecret, sentinelSecret))
		assertMasterPassword(t, envs, "sentinel-secret", "sentinel-password")
	})

	t.Run("sentinel-only redisSecret is honoured", func(t *testing.T) {
		envs := buildSentinelEnv(newRR(nil, sentinelSecret))
		assertMasterPassword(t, envs, "sentinel-secret", "sentinel-password")
	})
}

func assertMasterPassword(t *testing.T, envs []corev1.EnvVar, wantName, wantKey string) {
	t.Helper()
	for _, e := range envs {
		if e.Name != "MASTER_PASSWORD" {
			continue
		}
		require.NotNil(t, e.ValueFrom)
		require.NotNil(t, e.ValueFrom.SecretKeyRef)
		assert.Equal(t, wantName, e.ValueFrom.SecretKeyRef.Name)
		assert.Equal(t, wantKey, e.ValueFrom.SecretKeyRef.Key)
		return
	}
	t.Fatalf("MASTER_PASSWORD env var not found in %+v", envs)
}

// TestConfigureSentinelPodUsesSentinelRedisSecret exercises the controller path
// that regressed when an outer ExistingPasswordSecret shadowed the embedded
// redisSecret: the Sentinel connection must authenticate with the password read
// from the embedded redisSecret.
func TestConfigureSentinelPodUsesSentinelRedisSecret(t *testing.T) {
	clientset := fake.NewSimpleClientset(&corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{Name: "sentinel-secret", Namespace: "default"},
		Data:       map[string][]byte{"password": []byte("s3cr3t")},
	})

	inst := &rrvb2.RedisReplication{
		ObjectMeta: metav1.ObjectMeta{Name: "example-replication", Namespace: "default"},
		Spec: rrvb2.RedisReplicationSpec{
			Size:     ptr.To(int32(3)),
			Sentinel: &rrvb2.Sentinel{Size: 3},
		},
	}
	inst.Spec.Sentinel.ExistingPasswordSecret = &commonapi.ExistingPasswordSecret{
		Name: ptr.To("sentinel-secret"),
		Key:  ptr.To("password"),
	}

	r := &Reconciler{K8sClient: clientset}
	redisClient := &fakeSentinelRedisClient{
		svc: &fakeSentinelRedisService{slaves: 2, sentinels: 3},
	}
	pod := corev1.Pod{Status: corev1.PodStatus{PodIP: "10.0.0.10"}}

	err := r.configureSentinelPod(context.Background(), redisClient, inst, pod, "10.0.0.20", "")

	require.NoError(t, err)
	require.Len(t, redisClient.connections, 1)
	assert.Equal(t, "10.0.0.10", redisClient.connections[0].Host)
	assert.Equal(t, "s3cr3t", redisClient.connections[0].Password)
}

func TestConfigureSentinelPodWithoutSecretSendsEmptyPassword(t *testing.T) {
	inst := &rrvb2.RedisReplication{
		ObjectMeta: metav1.ObjectMeta{Name: "example-replication", Namespace: "default"},
		Spec: rrvb2.RedisReplicationSpec{
			Size:     ptr.To(int32(3)),
			Sentinel: &rrvb2.Sentinel{Size: 3},
		},
	}

	r := &Reconciler{K8sClient: fake.NewSimpleClientset()}
	redisClient := &fakeSentinelRedisClient{
		svc: &fakeSentinelRedisService{slaves: 2, sentinels: 3},
	}
	pod := corev1.Pod{Status: corev1.PodStatus{PodIP: "10.0.0.10"}}

	err := r.configureSentinelPod(context.Background(), redisClient, inst, pod, "10.0.0.20", "")

	require.NoError(t, err)
	require.Len(t, redisClient.connections, 1)
	assert.Empty(t, redisClient.connections[0].Password)
}

type fakeSentinelRedisClient struct {
	connections []*redis.ConnectionInfo
	svc         *fakeSentinelRedisService
}

func (f *fakeSentinelRedisClient) Connect(info *redis.ConnectionInfo) redis.Service {
	f.connections = append(f.connections, info)
	return f.svc
}

type fakeSentinelRedisService struct {
	slaves    int
	sentinels int
}

func (f *fakeSentinelRedisService) IsMaster(context.Context) (bool, error) { return false, nil }

func (f *fakeSentinelRedisService) GetAttachedReplicaCount(context.Context) (int, error) {
	return 0, nil
}

func (f *fakeSentinelRedisService) SentinelMonitor(context.Context, *redis.ConnectionInfo, string, string) error {
	return nil
}

func (f *fakeSentinelRedisService) SentinelSet(context.Context, string, string, string) error {
	return nil
}

func (f *fakeSentinelRedisService) SentinelReset(context.Context, string) error { return nil }

func (f *fakeSentinelRedisService) GetInfoSentinel(context.Context) (*redis.InfoSentinelResult, error) {
	return &redis.InfoSentinelResult{
		Masters: []redis.SentinelMasterInfo{
			{Name: masterGroupName, Slaves: f.slaves, Sentinels: f.sentinels},
		},
	}, nil
}

func (f *fakeSentinelRedisService) GetClusterInfo(context.Context) (*redis.ClusterStatus, error) {
	return &redis.ClusterStatus{}, nil
}
