package k8sutils

import (
	"context"
	"testing"

	common "github.com/OT-CONTAINER-KIT/redis-operator/api/common/v1beta2"
	rrvb2 "github.com/OT-CONTAINER-KIT/redis-operator/api/redisreplication/v1beta2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	k8sClientFake "k8s.io/client-go/kubernetes/fake"
	"k8s.io/utils/ptr"
)

func Test_generateReplicationSentinelContainerParams(t *testing.T) {
	cr := &rrvb2.RedisReplication{
		Spec: rrvb2.RedisReplicationSpec{
			TLS: &common.TLSConfig{},
			Sentinel: &rrvb2.Sentinel{
				Size: 3,
				KubernetesConfig: common.KubernetesConfig{
					Image: "quay.io/opstree/redis-sentinel:v7.0.15",
				},
				SentinelConfig: common.SentinelConfig{
					ResolveHostnames:      "yes",
					AnnounceHostnames:     "yes",
					DownAfterMilliseconds: "5000",
					FailoverTimeout:       "60000",
					ParallelSyncs:         "1",
				},
			},
		},
	}

	p := generateReplicationSentinelContainerParams(cr)

	assert.Equal(t, "sentinel", p.Role, "container role must be sentinel so the agent runs bootstrap --sentinel")
	assert.Equal(t, "quay.io/opstree/redis-sentinel:v7.0.15", p.Image)
	assert.Same(t, cr.Spec.TLS, p.TLSConfig, "embedded sentinel must inherit the RedisReplication TLS config")

	if assert.NotNil(t, p.AdditionalEnvVariable, "sentinel env must be set") {
		env := map[string]string{}
		for _, e := range *p.AdditionalEnvVariable {
			env[e.Name] = e.Value
		}
		assert.Equal(t, "yes", env["RESOLVE_HOSTNAMES"])
		assert.Equal(t, "yes", env["ANNOUNCE_HOSTNAMES"])
		assert.Equal(t, "mymaster", env["MASTER_GROUP_NAME"])
		assert.Equal(t, "2", env["QUORUM"], "quorum = size/2+1 = 3/2+1 = 2")
		assert.Equal(t, "5000", env["DOWN_AFTER_MILLISECONDS"])
		assert.Equal(t, "60000", env["FAILOVER_TIMEOUT"])
		assert.Equal(t, "1", env["PARALLEL_SYNCS"])
	}
}

func Test_CreateReplicationSentinel_statefulsetIdentity(t *testing.T) {
	cr := &rrvb2.RedisReplication{
		ObjectMeta: metav1.ObjectMeta{Name: "redis-replication", Namespace: "redis"},
		Spec: rrvb2.RedisReplicationSpec{
			Sentinel: &rrvb2.Sentinel{
				Size: 3,
				KubernetesConfig: common.KubernetesConfig{
					Image: "quay.io/opstree/redis-sentinel:v7.0.15",
				},
			},
		},
	}
	client := k8sClientFake.NewSimpleClientset()

	require.NoError(t, CreateReplicationSentinel(context.TODO(), cr, client))

	sts, err := GetStatefulSet(context.TODO(), client, cr.Namespace, cr.SentinelStatefulSet())
	require.NoError(t, err)

	assert.Equal(t, cr.SentinelStatefulSet(), sts.Name)
	assert.Equal(t, cr.SentinelHLService(), sts.Spec.ServiceName, "sts ServiceName must match the headless service name for stable pod DNS")
	assert.Equal(t, "sentinel", sts.Spec.Selector.MatchLabels["role"])
}

func Test_generateReplicationSentinelParams_mapsSentinelKubernetesConfig(t *testing.T) {
	cr := &rrvb2.RedisReplication{
		Spec: rrvb2.RedisReplicationSpec{
			Sentinel: &rrvb2.Sentinel{
				Size: 3,
				KubernetesConfig: common.KubernetesConfig{
					Image:            "quay.io/opstree/redis-sentinel:v7.0.15",
					ImagePullSecrets: &[]corev1.LocalObjectReference{{Name: "regcred"}},
					MinReadySeconds:  ptr.To(int32(7)),
				},
				SentinelConfig: common.SentinelConfig{
					AdditionalSentinelConfig: ptr.To("sentinel resolve-hostnames yes"),
				},
			},
		},
	}

	p := generateReplicationSentinelParams(cr)

	assert.Equal(t, int32(3), *p.Replicas)
	assert.True(t, p.RecreateStatefulSet, "recreate handles immutable selector/serviceName migration")
	assert.Equal(t, int32(7), p.MinReadySeconds)
	if assert.NotNil(t, p.ImagePullSecrets) {
		assert.Equal(t, "regcred", (*p.ImagePullSecrets)[0].Name)
	}
	if assert.NotNil(t, p.ExternalConfig, "additionalSentinelConfig must reach the StatefulSet as ExternalConfig") {
		assert.Equal(t, "sentinel resolve-hostnames yes", *p.ExternalConfig)
	}
}
