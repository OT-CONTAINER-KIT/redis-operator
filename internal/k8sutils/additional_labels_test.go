package k8sutils

import (
	"context"
	"testing"

	common "github.com/OT-CONTAINER-KIT/redis-operator/api/common/v1beta2"
	rvb2 "github.com/OT-CONTAINER-KIT/redis-operator/api/redis/v1beta2"
	rcvb2 "github.com/OT-CONTAINER-KIT/redis-operator/api/rediscluster/v1beta2"
	rrvb2 "github.com/OT-CONTAINER-KIT/redis-operator/api/redisreplication/v1beta2"
	rsvb2 "github.com/OT-CONTAINER-KIT/redis-operator/api/redissentinel/v1beta2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"
	"k8s.io/utils/ptr"
)

func TestRedisLabelsWithAdditionalLabels(t *testing.T) {
	labels := getRedisLabelsWithAdditional("redis", standalone, "standalone", map[string]string{
		"team": "platform",
		"tier": "cache",
		"app":  "user-app",
	}, map[string]string{
		"tier":             "critical",
		"environment":      "prod",
		"role":             "user-role",
		"redis_setup_type": "user-type",
	})

	assert.Equal(t, "platform", labels["team"])
	assert.Equal(t, "critical", labels["tier"])
	assert.Equal(t, "prod", labels["environment"])
	assert.Equal(t, "redis", labels["app"])
	assert.Equal(t, "standalone", labels["role"])
	assert.Equal(t, "standalone", labels["redis_setup_type"])
}

func TestStandaloneRedisAdditionalLabels(t *testing.T) {
	ctx := context.Background()
	client := fake.NewSimpleClientset()
	cr := &rvb2.Redis{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "redis",
			Namespace: "default",
			Labels:    map[string]string{"team": "platform"},
		},
		Spec: rvb2.RedisSpec{
			KubernetesConfig: common.KubernetesConfig{
				Image:            "redis:7",
				AdditionalLabels: map[string]string{"environment": "prod"},
			},
			Storage: redisStorage(),
		},
	}

	require.NoError(t, CreateStandaloneRedis(ctx, cr, client))
	require.NoError(t, CreateStandaloneService(ctx, cr, client))

	sts := getStatefulSetForTest(t, client, cr.Namespace, cr.Name)
	assertAdditionalLabels(t, sts.Labels)
	assertAdditionalLabels(t, sts.Spec.Template.Labels)
	require.Len(t, sts.Spec.VolumeClaimTemplates, 1)
	assertAdditionalLabels(t, sts.Spec.VolumeClaimTemplates[0].Labels)
	assert.NotContains(t, sts.Spec.Selector.MatchLabels, "environment")

	svc, err := client.CoreV1().Services(cr.Namespace).Get(ctx, cr.Name, metav1.GetOptions{})
	require.NoError(t, err)
	assertAdditionalLabels(t, svc.Labels)
	assertAdditionalLabels(t, svc.Spec.Selector)
}

func TestRedisClusterAdditionalLabels(t *testing.T) {
	ctx := context.Background()
	client := fake.NewSimpleClientset()
	cr := &rcvb2.RedisCluster{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "cluster",
			Namespace: "default",
			Labels:    map[string]string{"team": "platform"},
		},
		Spec: rcvb2.RedisClusterSpec{
			ClusterSize: ptr.To(int32(3)),
			Port:        ptr.To(6379),
			KubernetesConfig: common.KubernetesConfig{
				Image:            "redis:7",
				AdditionalLabels: map[string]string{"environment": "prod"},
			},
		},
	}

	require.NoError(t, CreateRedisLeader(ctx, cr, client))
	require.NoError(t, CreateRedisFollower(ctx, cr, client))
	require.NoError(t, CreateRedisLeaderService(ctx, cr, client))

	leader := getStatefulSetForTest(t, client, cr.Namespace, "cluster-leader")
	follower := getStatefulSetForTest(t, client, cr.Namespace, "cluster-follower")
	assertAdditionalLabels(t, leader.Labels)
	assertAdditionalLabels(t, leader.Spec.Template.Labels)
	assertAdditionalLabels(t, follower.Labels)
	assertAdditionalLabels(t, follower.Spec.Template.Labels)
	assert.NotContains(t, leader.Spec.Selector.MatchLabels, "environment")
	assert.NotContains(t, follower.Spec.Selector.MatchLabels, "environment")

	svc, err := client.CoreV1().Services(cr.Namespace).Get(ctx, "cluster-leader", metav1.GetOptions{})
	require.NoError(t, err)
	assertAdditionalLabels(t, svc.Labels)
	assertAdditionalLabels(t, svc.Spec.Selector)
}

func TestRedisReplicationAdditionalLabels(t *testing.T) {
	ctx := context.Background()
	client := fake.NewSimpleClientset()
	cr := &rrvb2.RedisReplication{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "replication",
			Namespace: "default",
			Labels:    map[string]string{"team": "platform"},
		},
		Spec: rrvb2.RedisReplicationSpec{
			Size: ptr.To(int32(3)),
			KubernetesConfig: common.KubernetesConfig{
				Image:            "redis:7",
				AdditionalLabels: map[string]string{"environment": "prod"},
			},
			PodDisruptionBudget: &common.RedisPodDisruptionBudget{Enabled: true},
		},
	}

	require.NoError(t, CreateReplicationRedis(ctx, cr, client))
	require.NoError(t, CreateReplicationService(ctx, cr, client))
	require.NoError(t, ReconcileReplicationPodDisruptionBudget(ctx, cr, cr.Spec.PodDisruptionBudget, client))

	sts := getStatefulSetForTest(t, client, cr.Namespace, cr.Name)
	assertAdditionalLabels(t, sts.Labels)
	assertAdditionalLabels(t, sts.Spec.Template.Labels)
	assert.NotContains(t, sts.Spec.Selector.MatchLabels, "environment")

	svc, err := client.CoreV1().Services(cr.Namespace).Get(ctx, cr.Name, metav1.GetOptions{})
	require.NoError(t, err)
	assertAdditionalLabels(t, svc.Labels)
	assertAdditionalLabels(t, svc.Spec.Selector)

	pdb, err := client.PolicyV1().PodDisruptionBudgets(cr.Namespace).Get(ctx, cr.Name+"-replication", metav1.GetOptions{})
	require.NoError(t, err)
	assertAdditionalLabels(t, pdb.Labels)
	assert.NotContains(t, pdb.Spec.Selector.MatchLabels, "environment")
}

func TestRedisSentinelAdditionalLabels(t *testing.T) {
	ctx := context.Background()
	client := fake.NewSimpleClientset()
	cr := &rsvb2.RedisSentinel{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "sentinel",
			Namespace: "default",
			Labels:    map[string]string{"team": "platform"},
		},
		Spec: rsvb2.RedisSentinelSpec{
			Size: ptr.To(int32(3)),
			KubernetesConfig: common.KubernetesConfig{
				Image:            "redis:7",
				AdditionalLabels: map[string]string{"environment": "prod"},
			},
			PodDisruptionBudget: &common.RedisPodDisruptionBudget{Enabled: true},
		},
	}

	require.NoError(t, CreateRedisSentinel(ctx, client, cr, client, nil))
	require.NoError(t, CreateRedisSentinelService(ctx, cr, client))
	require.NoError(t, ReconcileSentinelPodDisruptionBudget(ctx, cr, cr.Spec.PodDisruptionBudget, client))

	sts := getStatefulSetForTest(t, client, cr.Namespace, "sentinel-sentinel")
	assertAdditionalLabels(t, sts.Labels)
	assertAdditionalLabels(t, sts.Spec.Template.Labels)
	assert.NotContains(t, sts.Spec.Selector.MatchLabels, "environment")

	svc, err := client.CoreV1().Services(cr.Namespace).Get(ctx, "sentinel-sentinel", metav1.GetOptions{})
	require.NoError(t, err)
	assertAdditionalLabels(t, svc.Labels)
	assertAdditionalLabels(t, svc.Spec.Selector)

	pdb, err := client.PolicyV1().PodDisruptionBudgets(cr.Namespace).Get(ctx, "sentinel-sentinel", metav1.GetOptions{})
	require.NoError(t, err)
	assertAdditionalLabels(t, pdb.Labels)
	assert.NotContains(t, pdb.Spec.Selector.MatchLabels, "environment")
}

func redisStorage() *common.Storage {
	return &common.Storage{
		VolumeClaimTemplate: corev1.PersistentVolumeClaim{
			Spec: corev1.PersistentVolumeClaimSpec{
				AccessModes: []corev1.PersistentVolumeAccessMode{corev1.ReadWriteOnce},
				Resources: corev1.VolumeResourceRequirements{
					Requests: corev1.ResourceList{
						corev1.ResourceStorage: resource.MustParse("1Gi"),
					},
				},
			},
		},
	}
}

func getStatefulSetForTest(t *testing.T, client *fake.Clientset, namespace, name string) *appsv1.StatefulSet {
	t.Helper()
	sts, err := client.AppsV1().StatefulSets(namespace).Get(context.Background(), name, metav1.GetOptions{})
	require.NoError(t, err)
	return sts
}

func assertAdditionalLabels(t *testing.T, labels map[string]string) {
	t.Helper()
	assert.Equal(t, "platform", labels["team"])
	assert.Equal(t, "prod", labels["environment"])
}
