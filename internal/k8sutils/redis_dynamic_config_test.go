package k8sutils

import (
	"context"
	"errors"
	"testing"

	common "github.com/OT-CONTAINER-KIT/redis-operator/api/common/v1beta2"
	rvb2 "github.com/OT-CONTAINER-KIT/redis-operator/api/redis/v1beta2"
	rcvb2 "github.com/OT-CONTAINER-KIT/redis-operator/api/rediscluster/v1beta2"
	rrvb2 "github.com/OT-CONTAINER-KIT/redis-operator/api/redisreplication/v1beta2"
	"github.com/go-redis/redismock/v9"
	redis "github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/assert"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"
)

func TestApplyDynamicConfig(t *testing.T) {
	ctx := context.Background()

	t.Run("applies all entries when reachable", func(t *testing.T) {
		client, mock := redismock.NewClientMock()
		mock.ExpectPing().SetVal("PONG")
		mock.ExpectConfigSet("maxmemory-policy", "allkeys-lru").SetVal("OK")
		mock.ExpectConfigSet("slowlog-log-slower-than", "5000").SetVal("OK")

		applied, err := applyDynamicConfig(ctx, client, "redis-0", []string{
			"maxmemory-policy allkeys-lru",
			"slowlog-log-slower-than 5000",
		})
		assert.NoError(t, err)
		assert.True(t, applied)
		assert.NoError(t, mock.ExpectationsWereMet())
	})

	t.Run("preserves spaces in the value", func(t *testing.T) {
		client, mock := redismock.NewClientMock()
		mock.ExpectPing().SetVal("PONG")
		// Only the first space splits key from value; the value may contain spaces.
		mock.ExpectConfigSet("save", "900 1 300 10").SetVal("OK")

		applied, err := applyDynamicConfig(ctx, client, "redis-0", []string{"save 900 1 300 10"})
		assert.NoError(t, err)
		assert.True(t, applied)
		assert.NoError(t, mock.ExpectationsWereMet())
	})

	t.Run("skips pod and reports not-applied when ping fails", func(t *testing.T) {
		client, mock := redismock.NewClientMock()
		mock.ExpectPing().SetErr(errors.New("connection refused"))

		applied, err := applyDynamicConfig(ctx, client, "redis-0", []string{"maxmemory-policy allkeys-lru"})
		assert.NoError(t, err)
		assert.False(t, applied, "an unreachable pod must report not-applied so the caller can retry")
		assert.NoError(t, mock.ExpectationsWereMet())
	})

	t.Run("skips pod and reports not-applied when not PONG", func(t *testing.T) {
		client, mock := redismock.NewClientMock()
		mock.ExpectPing().SetVal("LOADING Redis is loading the dataset in memory")

		applied, err := applyDynamicConfig(ctx, client, "redis-0", []string{"maxmemory-policy allkeys-lru"})
		assert.NoError(t, err)
		assert.False(t, applied)
		assert.NoError(t, mock.ExpectationsWereMet())
	})

	t.Run("skips malformed entries but applies valid ones", func(t *testing.T) {
		client, mock := redismock.NewClientMock()
		mock.ExpectPing().SetVal("PONG")
		mock.ExpectConfigSet("maxmemory-policy", "allkeys-lru").SetVal("OK")

		applied, err := applyDynamicConfig(ctx, client, "redis-0", []string{
			"missing-value",
			"maxmemory-policy allkeys-lru",
		})
		assert.NoError(t, err)
		assert.True(t, applied)
		assert.NoError(t, mock.ExpectationsWereMet())
	})

	t.Run("returns error when CONFIG SET fails", func(t *testing.T) {
		client, mock := redismock.NewClientMock()
		mock.ExpectPing().SetVal("PONG")
		mock.ExpectConfigSet("maxmemory-policy", "allkeys-lru").SetErr(errors.New("CONFIG SET failed"))

		applied, err := applyDynamicConfig(ctx, client, "redis-0", []string{"maxmemory-policy allkeys-lru"})
		assert.Error(t, err)
		assert.True(t, applied, "the pod was reachable, so it is considered applied even though a CONFIG SET failed")
		assert.NoError(t, mock.ExpectationsWereMet())
	})
}

func TestSetRedisReplicationDynamicConfig(t *testing.T) {
	ctx := context.Background()

	newReplication := func(size int32) *rrvb2.RedisReplication {
		return &rrvb2.RedisReplication{
			ObjectMeta: metav1.ObjectMeta{Name: "redis-replication", Namespace: "default"},
			Spec: rrvb2.RedisReplicationSpec{
				Size: ptr.To(size),
				RedisConfig: &common.RedisConfig{
					DynamicConfig: []string{"maxmemory-policy allkeys-lru"},
				},
			},
		}
	}

	t.Run("applies config to every replica pod", func(t *testing.T) {
		cr := newReplication(3)
		mocks := map[string]redismock.ClientMock{}
		makeClient := func(podName string) *redis.Client {
			c, m := redismock.NewClientMock()
			m.ExpectPing().SetVal("PONG")
			m.ExpectConfigSet("maxmemory-policy", "allkeys-lru").SetVal("OK")
			mocks[podName] = m
			return c
		}

		err := setRedisReplicationDynamicConfig(ctx, cr, makeClient)
		assert.NoError(t, err)
		assert.Len(t, mocks, 3)
		for _, name := range []string{"redis-replication-0", "redis-replication-1", "redis-replication-2"} {
			m, ok := mocks[name]
			assert.True(t, ok, "expected a client for pod %s", name)
			assert.NoError(t, m.ExpectationsWereMet())
		}
	})

	t.Run("no-op when dynamic config is empty", func(t *testing.T) {
		cr := newReplication(3)
		cr.Spec.RedisConfig = nil
		called := false
		err := setRedisReplicationDynamicConfig(ctx, cr, func(podName string) *redis.Client {
			called = true
			return nil
		})
		assert.NoError(t, err)
		assert.False(t, called, "client factory must not be called without dynamic config")
	})

	t.Run("continues past unreachable pods", func(t *testing.T) {
		cr := newReplication(2)
		mocks := map[string]redismock.ClientMock{}
		makeClient := func(podName string) *redis.Client {
			c, m := redismock.NewClientMock()
			if podName == "redis-replication-0" {
				m.ExpectPing().SetErr(errors.New("connection refused"))
			} else {
				m.ExpectPing().SetVal("PONG")
				m.ExpectConfigSet("maxmemory-policy", "allkeys-lru").SetVal("OK")
			}
			mocks[podName] = m
			return c
		}

		err := setRedisReplicationDynamicConfig(ctx, cr, makeClient)
		assert.NoError(t, err)
		assert.Len(t, mocks, 2, "every pod should be visited even if an earlier one is unreachable")
		for _, m := range mocks {
			assert.NoError(t, m.ExpectationsWereMet())
		}
	})

	t.Run("returns error when a CONFIG SET fails", func(t *testing.T) {
		cr := newReplication(2)
		makeClient := func(podName string) *redis.Client {
			c, m := redismock.NewClientMock()
			m.ExpectPing().SetVal("PONG")
			if podName == "redis-replication-0" {
				m.ExpectConfigSet("maxmemory-policy", "allkeys-lru").SetErr(errors.New("CONFIG SET failed"))
			} else {
				m.ExpectConfigSet("maxmemory-policy", "allkeys-lru").SetVal("OK")
			}
			return c
		}

		err := setRedisReplicationDynamicConfig(ctx, cr, makeClient)
		assert.Error(t, err)
	})
}

func TestSetRedisStandaloneDynamicConfig(t *testing.T) {
	ctx := context.Background()

	newStandalone := func() *rvb2.Redis {
		return &rvb2.Redis{
			ObjectMeta: metav1.ObjectMeta{Name: "redis-standalone", Namespace: "default"},
			Spec: rvb2.RedisSpec{
				RedisConfig: &common.RedisConfig{
					DynamicConfig: []string{"maxmemory-policy allkeys-lru"},
				},
			},
		}
	}

	t.Run("applies config to the single pod", func(t *testing.T) {
		cr := newStandalone()
		var requestedPod string
		c, mock := redismock.NewClientMock()
		mock.ExpectPing().SetVal("PONG")
		mock.ExpectConfigSet("maxmemory-policy", "allkeys-lru").SetVal("OK")

		applied, err := setRedisStandaloneDynamicConfig(ctx, cr, func(podName string) *redis.Client {
			requestedPod = podName
			return c
		})
		assert.NoError(t, err)
		assert.True(t, applied)
		assert.Equal(t, "redis-standalone-0", requestedPod)
		assert.NoError(t, mock.ExpectationsWereMet())
	})

	t.Run("reports not-applied when the pod is unreachable", func(t *testing.T) {
		cr := newStandalone()
		c, mock := redismock.NewClientMock()
		mock.ExpectPing().SetErr(errors.New("connection refused"))

		applied, err := setRedisStandaloneDynamicConfig(ctx, cr, func(podName string) *redis.Client {
			return c
		})
		assert.NoError(t, err)
		assert.False(t, applied, "an unreachable pod must report not-applied so the controller requeues and retries")
		assert.NoError(t, mock.ExpectationsWereMet())
	})

	t.Run("no-op when dynamic config is empty", func(t *testing.T) {
		cr := newStandalone()
		cr.Spec.RedisConfig = nil
		called := false
		applied, err := setRedisStandaloneDynamicConfig(ctx, cr, func(podName string) *redis.Client {
			called = true
			return nil
		})
		assert.NoError(t, err)
		assert.True(t, applied, "an empty config has nothing to apply and should not force a requeue")
		assert.False(t, called, "client factory must not be called without dynamic config")
	})

	t.Run("returns error when CONFIG SET fails", func(t *testing.T) {
		cr := newStandalone()
		c, mock := redismock.NewClientMock()
		mock.ExpectPing().SetVal("PONG")
		mock.ExpectConfigSet("maxmemory-policy", "allkeys-lru").SetErr(errors.New("CONFIG SET failed"))

		_, err := setRedisStandaloneDynamicConfig(ctx, cr, func(podName string) *redis.Client {
			return c
		})
		assert.Error(t, err)
		assert.NoError(t, mock.ExpectationsWereMet())
	})
}

func TestSetRedisClusterDynamicConfig(t *testing.T) {
	ctx := context.Background()

	newCluster := func(dynamicConfig []string) *rcvb2.RedisCluster {
		return &rcvb2.RedisCluster{
			ObjectMeta: metav1.ObjectMeta{Name: "redis-cluster", Namespace: "default"},
			Spec: rcvb2.RedisClusterSpec{
				ClusterSize: ptr.To(int32(2)),
				RedisConfig: &common.RedisConfig{
					DynamicConfig: dynamicConfig,
				},
			},
		}
	}

	t.Run("applies config to every leader and follower pod", func(t *testing.T) {
		cr := newCluster([]string{"maxmemory-policy allkeys-lru"})
		mocks := map[string]redismock.ClientMock{}
		makeClient := func(podName string) *redis.Client {
			c, m := redismock.NewClientMock()
			m.ExpectPing().SetVal("PONG")
			m.ExpectConfigSet("maxmemory-policy", "allkeys-lru").SetVal("OK")
			mocks[podName] = m
			return c
		}

		err := setRedisClusterDynamicConfig(ctx, cr, makeClient)
		assert.NoError(t, err)
		assert.Len(t, mocks, 4)
		for _, name := range []string{"redis-cluster-leader-0", "redis-cluster-leader-1", "redis-cluster-follower-0", "redis-cluster-follower-1"} {
			m, ok := mocks[name]
			assert.True(t, ok, "expected a client for pod %s", name)
			assert.NoError(t, m.ExpectationsWereMet())
		}
	})

	t.Run("no-op when dynamic config is empty", func(t *testing.T) {
		cr := newCluster(nil)
		called := false
		err := setRedisClusterDynamicConfig(ctx, cr, func(podName string) *redis.Client {
			called = true
			return nil
		})
		assert.NoError(t, err)
		assert.False(t, called, "client factory must not be called without dynamic config")
	})

	t.Run("re-applies an updated value on a later call, simulating a later reconcile", func(t *testing.T) {
		// This is the behavior issue #1757 relied on: once the cluster reaches
		// Ready, the operator must keep re-applying dynamicConfig on subsequent
		// reconciles so a spec change (e.g. maxmemory-samples 4 -> 11) actually
		// reaches the running pods instead of only being applied once.
		cr := newCluster([]string{"maxmemory-samples 4"})
		mocks := map[string]redismock.ClientMock{}
		makeClientWith := func(value string) func(string) *redis.Client {
			return func(podName string) *redis.Client {
				c, m := redismock.NewClientMock()
				m.ExpectPing().SetVal("PONG")
				m.ExpectConfigSet("maxmemory-samples", value).SetVal("OK")
				mocks[podName] = m
				return c
			}
		}

		err := setRedisClusterDynamicConfig(ctx, cr, makeClientWith("4"))
		assert.NoError(t, err)
		for _, m := range mocks {
			assert.NoError(t, m.ExpectationsWereMet())
		}

		mocks = map[string]redismock.ClientMock{}
		cr.Spec.RedisConfig.DynamicConfig = []string{"maxmemory-samples 11"}
		err = setRedisClusterDynamicConfig(ctx, cr, makeClientWith("11"))
		assert.NoError(t, err)
		for _, m := range mocks {
			assert.NoError(t, m.ExpectationsWereMet())
		}
	})

	t.Run("continues past unreachable pods", func(t *testing.T) {
		cr := newCluster([]string{"maxmemory-policy allkeys-lru"})
		mocks := map[string]redismock.ClientMock{}
		makeClient := func(podName string) *redis.Client {
			c, m := redismock.NewClientMock()
			if podName == "redis-cluster-leader-0" {
				m.ExpectPing().SetErr(errors.New("connection refused"))
			} else {
				m.ExpectPing().SetVal("PONG")
				m.ExpectConfigSet("maxmemory-policy", "allkeys-lru").SetVal("OK")
			}
			mocks[podName] = m
			return c
		}

		err := setRedisClusterDynamicConfig(ctx, cr, makeClient)
		assert.NoError(t, err)
		assert.Len(t, mocks, 4, "every pod should be visited even if an earlier one is unreachable")
		for _, m := range mocks {
			assert.NoError(t, m.ExpectationsWereMet())
		}
	})

	t.Run("returns error when a CONFIG SET fails", func(t *testing.T) {
		cr := newCluster([]string{"maxmemory-policy allkeys-lru"})
		makeClient := func(podName string) *redis.Client {
			c, m := redismock.NewClientMock()
			m.ExpectPing().SetVal("PONG")
			if podName == "redis-cluster-leader-0" {
				m.ExpectConfigSet("maxmemory-policy", "allkeys-lru").SetErr(errors.New("CONFIG SET failed"))
			} else {
				m.ExpectConfigSet("maxmemory-policy", "allkeys-lru").SetVal("OK")
			}
			return c
		}

		err := setRedisClusterDynamicConfig(ctx, cr, makeClient)
		assert.Error(t, err)
	})
}
