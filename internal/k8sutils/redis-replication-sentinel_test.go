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

func Test_generateReplicationSentinelParams_podPlacement(t *testing.T) {
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
	grace := int64(42)

	cr := &rrvb2.RedisReplication{
		Spec: rrvb2.RedisReplicationSpec{
			Sentinel: &rrvb2.Sentinel{
				Size:                          3,
				Affinity:                      affinity,
				Tolerations:                   &tolerations,
				NodeSelector:                  nodeSelector,
				TopologySpreadConstraints:     topology,
				PodSecurityContext:            podSecurityContext,
				PriorityClassName:             "high-priority",
				TerminationGracePeriodSeconds: ptr.To(grace),
				ServiceAccountName:            ptr.To("sentinel-sa"),
			},
		},
	}

	p := generateReplicationSentinelParams(cr)

	assert.Same(t, affinity, p.Affinity)
	assert.Equal(t, &tolerations, p.Tolerations)
	assert.Equal(t, nodeSelector, p.NodeSelector)
	assert.Equal(t, topology, p.TopologySpreadConstraints)
	assert.Same(t, podSecurityContext, p.PodSecurityContext)
	assert.Equal(t, "high-priority", p.PriorityClassName)
	if assert.NotNil(t, p.TerminationGracePeriodSeconds) {
		assert.Equal(t, grace, *p.TerminationGracePeriodSeconds)
	}
	if assert.NotNil(t, p.ServiceAccountName, "Sentinel-specific serviceAccountName must win") {
		assert.Equal(t, "sentinel-sa", *p.ServiceAccountName)
	}
}

func Test_generateReplicationSentinelParams_omitsUnsetPlacement(t *testing.T) {
	cr := &rrvb2.RedisReplication{
		Spec: rrvb2.RedisReplicationSpec{
			Sentinel: &rrvb2.Sentinel{Size: 3},
		},
	}

	p := generateReplicationSentinelParams(cr)

	assert.Nil(t, p.Affinity)
	assert.Nil(t, p.Tolerations)
	assert.Nil(t, p.NodeSelector)
	assert.Nil(t, p.TopologySpreadConstraints)
	assert.Nil(t, p.PodSecurityContext)
	assert.Empty(t, p.PriorityClassName)
	assert.Nil(t, p.TerminationGracePeriodSeconds)
	// With no Sentinel-specific serviceAccountName, it falls back to the
	// replication-level one (nil in this minimal CR).
	assert.Nil(t, p.ServiceAccountName)
}

func Test_generateReplicationSentinelParams_serviceAccountNameFallsBack(t *testing.T) {
	cr := &rrvb2.RedisReplication{
		Spec: rrvb2.RedisReplicationSpec{
			ServiceAccountName: ptr.To("replication-sa"),
			Sentinel:           &rrvb2.Sentinel{Size: 3},
		},
	}

	p := generateReplicationSentinelParams(cr)

	if assert.NotNil(t, p.ServiceAccountName) {
		assert.Equal(t, "replication-sa", *p.ServiceAccountName)
	}
}

func Test_generateReplicationSentinelContainerParams_securityContext(t *testing.T) {
	sc := &corev1.SecurityContext{RunAsNonRoot: ptr.To(true)}
	cr := &rrvb2.RedisReplication{
		Spec: rrvb2.RedisReplicationSpec{
			Sentinel: &rrvb2.Sentinel{
				Size:            3,
				SecurityContext: sc,
			},
		},
	}

	p := generateReplicationSentinelContainerParams(cr)

	assert.Same(t, sc, p.SecurityContext)
}

func Test_getReplicationSentinelEnvVariable_masterPassword(t *testing.T) {
	redisSecret := &common.ExistingPasswordSecret{Name: ptr.To("redis-secret"), Key: ptr.To("redis-password")}
	sentinelSecret := &common.ExistingPasswordSecret{Name: ptr.To("sentinel-secret"), Key: ptr.To("sentinel-password")}

	newCR := func(top, sentinel *common.ExistingPasswordSecret) *rrvb2.RedisReplication {
		cr := &rrvb2.RedisReplication{
			Spec: rrvb2.RedisReplicationSpec{Sentinel: &rrvb2.Sentinel{Size: 3}},
		}
		cr.Spec.KubernetesConfig.ExistingPasswordSecret = top
		cr.Spec.Sentinel.ExistingPasswordSecret = sentinel
		return cr
	}

	masterPassword := func(cr *rrvb2.RedisReplication) *corev1.EnvVar {
		for i, e := range *getReplicationSentinelEnvVariable(cr) {
			if e.Name == "MASTER_PASSWORD" {
				return &(*getReplicationSentinelEnvVariable(cr))[i]
			}
		}
		return nil
	}

	t.Run("no secret omits MASTER_PASSWORD", func(t *testing.T) {
		assert.Nil(t, masterPassword(newCR(nil, nil)))
	})

	t.Run("falls back to replication-level redisSecret", func(t *testing.T) {
		e := masterPassword(newCR(redisSecret, nil))
		require.NotNil(t, e)
		require.NotNil(t, e.ValueFrom)
		require.NotNil(t, e.ValueFrom.SecretKeyRef)
		assert.Equal(t, "redis-secret", e.ValueFrom.SecretKeyRef.Name)
		assert.Equal(t, "redis-password", e.ValueFrom.SecretKeyRef.Key)
	})

	t.Run("Sentinel redisSecret overrides replication-level", func(t *testing.T) {
		e := masterPassword(newCR(redisSecret, sentinelSecret))
		require.NotNil(t, e)
		assert.Equal(t, "sentinel-secret", e.ValueFrom.SecretKeyRef.Name)
		assert.Equal(t, "sentinel-password", e.ValueFrom.SecretKeyRef.Key)
	})

	t.Run("Sentinel-only redisSecret is honoured", func(t *testing.T) {
		e := masterPassword(newCR(nil, sentinelSecret))
		require.NotNil(t, e)
		assert.Equal(t, "sentinel-secret", e.ValueFrom.SecretKeyRef.Name)
		assert.Equal(t, "sentinel-password", e.ValueFrom.SecretKeyRef.Key)
	})
}
