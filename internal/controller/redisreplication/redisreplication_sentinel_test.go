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
