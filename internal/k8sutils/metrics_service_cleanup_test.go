package k8sutils

import (
	"context"
	"testing"

	rvb2 "github.com/OT-CONTAINER-KIT/redis-operator/api/redis/v1beta2"
	rcvb2 "github.com/OT-CONTAINER-KIT/redis-operator/api/rediscluster/v1beta2"
	rrvb2 "github.com/OT-CONTAINER-KIT/redis-operator/api/redisreplication/v1beta2"
	rsvb2 "github.com/OT-CONTAINER-KIT/redis-operator/api/redissentinel/v1beta2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	k8sClientFake "k8s.io/client-go/kubernetes/fake"
	"k8s.io/utils/ptr"
)

// Every CR kind must remove an orphaned <cr>-metrics Service on reconcile once
// the exporter is disabled (or absent from the spec).
func TestMetricsServiceCleanupOnExporterDisabled(t *testing.T) {
	ctx := context.Background()
	namespace := "redis"

	orphanService := func(name string) *corev1.Service {
		return &corev1.Service{
			ObjectMeta: metav1.ObjectMeta{
				Name:      name,
				Namespace: namespace,
				Labels:    map[string]string{"app.kubernetes.io/component": "metrics"},
			},
		}
	}

	assertMetricsServiceGone := func(t *testing.T, client *k8sClientFake.Clientset, name string) {
		t.Helper()
		_, err := client.CoreV1().Services(namespace).Get(ctx, name, metav1.GetOptions{})
		assert.True(t, apierrors.IsNotFound(err), "metrics Service %q should have been deleted", name)
	}

	t.Run("RedisReplication deletes orphaned metrics Service", func(t *testing.T) {
		cr := &rrvb2.RedisReplication{
			ObjectMeta: metav1.ObjectMeta{Name: "test-replication", Namespace: namespace},
		}
		metricsName := "test-replication-metrics"
		client := k8sClientFake.NewSimpleClientset(orphanService(metricsName))

		require.NoError(t, CreateReplicationService(ctx, cr, client))
		assertMetricsServiceGone(t, client, metricsName)
	})

	t.Run("standalone Redis deletes orphaned metrics Service", func(t *testing.T) {
		cr := &rvb2.Redis{
			ObjectMeta: metav1.ObjectMeta{Name: "test-standalone", Namespace: namespace},
		}
		metricsName := "test-standalone-metrics"
		client := k8sClientFake.NewSimpleClientset(orphanService(metricsName))

		require.NoError(t, CreateStandaloneService(ctx, cr, client))
		assertMetricsServiceGone(t, client, metricsName)
	})

	t.Run("RedisSentinel deletes orphaned metrics Service", func(t *testing.T) {
		cr := &rsvb2.RedisSentinel{
			ObjectMeta: metav1.ObjectMeta{Name: "test-sentinel", Namespace: namespace},
		}
		metricsName := "test-sentinel-sentinel-metrics"
		client := k8sClientFake.NewSimpleClientset(orphanService(metricsName))

		require.NoError(t, CreateRedisSentinelService(ctx, cr, client))
		assertMetricsServiceGone(t, client, metricsName)
	})

	t.Run("RedisCluster leader deletes orphaned metrics Service", func(t *testing.T) {
		cr := &rcvb2.RedisCluster{
			ObjectMeta: metav1.ObjectMeta{Name: "test-cluster", Namespace: namespace},
			Spec: rcvb2.RedisClusterSpec{
				ClusterSize: ptr.To(int32(3)),
			},
		}
		cr.SetDefault()
		metricsName := "test-cluster-leader-metrics"
		client := k8sClientFake.NewSimpleClientset(orphanService(metricsName))

		require.NoError(t, CreateRedisLeaderService(ctx, cr, client))
		assertMetricsServiceGone(t, client, metricsName)
	})
}
