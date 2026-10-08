package redissentinel

import (
	"context"
	"testing"

	commonapi "github.com/OT-CONTAINER-KIT/redis-operator/api/common/v1beta2"
	rcvb2 "github.com/OT-CONTAINER-KIT/redis-operator/api/rediscluster/v1beta2"
	rrvb2 "github.com/OT-CONTAINER-KIT/redis-operator/api/redisreplication/v1beta2"
	rsvb2 "github.com/OT-CONTAINER-KIT/redis-operator/api/redissentinel/v1beta2"
	intctrlutil "github.com/OT-CONTAINER-KIT/redis-operator/internal/controllerutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes/fake"
	"k8s.io/utils/ptr"
	clientfake "sigs.k8s.io/controller-runtime/pkg/client/fake"
)

func TestReconcileSentinelRejectsExternalMasterReplication(t *testing.T) {
	scheme := runtime.NewScheme()
	require.NoError(t, rrvb2.AddToScheme(scheme))
	require.NoError(t, rsvb2.AddToScheme(scheme))

	rr := &rrvb2.RedisReplication{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "my-replication",
			Namespace: "default",
		},
		Spec: rrvb2.RedisReplicationSpec{
			Size: ptr.To(int32(3)),
			KubernetesConfig: commonapi.KubernetesConfig{
				Image: "redis:7",
			},
			ExternalMaster: &rrvb2.ExternalMaster{
				Host: "redis.primary.example.com",
				Port: ptr.To(int32(6380)),
			},
		},
	}

	ctrlClient := clientfake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(rr).
		Build()

	r := &RedisSentinelReconciler{
		Client:             ctrlClient,
		K8sClient:          fake.NewSimpleClientset(),
		ReplicationWatcher: intctrlutil.NewResourceWatcher(),
	}

	sentinel := &rsvb2.RedisSentinel{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "my-sentinel",
			Namespace: "default",
		},
		Spec: rsvb2.RedisSentinelSpec{
			Size: ptr.To(int32(3)),
			KubernetesConfig: commonapi.KubernetesConfig{
				Image: "redis:7",
			},
			RedisSentinelConfig: &rsvb2.RedisSentinelConfig{
				RedisSentinelConfig: commonapi.RedisSentinelConfig{
					RedisReplicationName: "my-replication",
				},
			},
		},
	}

	_, err := r.reconcileSentinel(context.Background(), sentinel)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "externalMaster configured")
	assert.Contains(t, err.Error(), "sentinel cannot monitor a passive replication")
}

func TestReconcileSentinelAllowsNormalReplication(t *testing.T) {
	scheme := runtime.NewScheme()
	require.NoError(t, rrvb2.AddToScheme(scheme))
	require.NoError(t, rsvb2.AddToScheme(scheme))

	rr := &rrvb2.RedisReplication{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "my-replication",
			Namespace: "default",
		},
		Spec: rrvb2.RedisReplicationSpec{
			Size: ptr.To(int32(3)),
			KubernetesConfig: commonapi.KubernetesConfig{
				Image: "redis:7",
			},
		},
	}

	masterPod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "my-replication-0",
			Namespace: "default",
		},
		Status: corev1.PodStatus{
			PodIP: "10.0.0.1",
		},
	}

	ctrlClient := clientfake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(rr).
		Build()

	r := &RedisSentinelReconciler{
		Client:             ctrlClient,
		K8sClient:          fake.NewSimpleClientset(masterPod),
		ReplicationWatcher: intctrlutil.NewResourceWatcher(),
		Checker:            &fakeChecker{masterPod: *masterPod},
		Healer:             &fakeHealer{},
	}

	sentinel := &rsvb2.RedisSentinel{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "my-sentinel",
			Namespace: "default",
		},
		Spec: rsvb2.RedisSentinelSpec{
			Size: ptr.To(int32(3)),
			KubernetesConfig: commonapi.KubernetesConfig{
				Image: "redis:7",
			},
			RedisSentinelConfig: &rsvb2.RedisSentinelConfig{
				RedisSentinelConfig: commonapi.RedisSentinelConfig{
					RedisReplicationName: "my-replication",
				},
			},
		},
	}

	_, err := r.reconcileSentinel(context.Background(), sentinel)

	require.NoError(t, err)
}

type fakeChecker struct {
	masterPod corev1.Pod
}

func (f *fakeChecker) GetMasterFromReplication(context.Context, *rrvb2.RedisReplication) (corev1.Pod, error) {
	return f.masterPod, nil
}

func (f *fakeChecker) GetPassword(context.Context, string, *commonapi.ExistingPasswordSecret) (string, error) {
	return "", nil
}

func (f *fakeChecker) CheckClusterSlotsAssigned(context.Context, *rcvb2.RedisCluster) (bool, error) {
	return true, nil
}

type fakeHealer struct{}

func (f *fakeHealer) SentinelMonitor(context.Context, *rsvb2.RedisSentinel, string) error {
	return nil
}

func (f *fakeHealer) SentinelSet(context.Context, *rsvb2.RedisSentinel, string) error {
	return nil
}

func (f *fakeHealer) SentinelReset(context.Context, *rsvb2.RedisSentinel) error {
	return nil
}

func (f *fakeHealer) UpdateRedisRoleLabel(context.Context, string, map[string]string, *commonapi.ExistingPasswordSecret, *commonapi.TLSConfig, string) error {
	return nil
}
