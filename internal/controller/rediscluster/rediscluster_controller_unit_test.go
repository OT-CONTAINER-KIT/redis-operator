package rediscluster

import (
	"context"
	"errors"
	"testing"

	common "github.com/OT-CONTAINER-KIT/redis-operator/api/common/v1beta2"
	rcvb2 "github.com/OT-CONTAINER-KIT/redis-operator/api/rediscluster/v1beta2"
	"github.com/OT-CONTAINER-KIT/redis-operator/internal/controller/common/redis"
	"github.com/stretchr/testify/assert"
	"k8s.io/client-go/kubernetes"
)

type fakeChecker struct {
	redis.Checker
	slotsAssigned bool
	err           error
	called        bool
}

func (f *fakeChecker) CheckClusterSlotsAssigned(ctx context.Context, cr *rcvb2.RedisCluster) (bool, error) {
	f.called = true
	return f.slotsAssigned, f.err
}

func TestShouldScaleUpExistingCluster(t *testing.T) {
	tests := []struct {
		name           string
		leaderCount    int32
		slotsAssigned  bool
		checkErr       error
		wantScaleUp    bool
		wantErr        bool
		wantCheckerHit bool
	}{
		{
			name:           "formed single-node cluster scales up via add-node (issue #1521)",
			leaderCount:    1,
			slotsAssigned:  true,
			wantScaleUp:    true,
			wantCheckerHit: true,
		},
		{
			name:           "fresh nodes without assigned slots fall back to cluster creation",
			leaderCount:    1,
			slotsAssigned:  false,
			wantScaleUp:    false,
			wantCheckerHit: true,
		},
		{
			name:           "formed two-leader cluster scales up via add-node",
			leaderCount:    2,
			slotsAssigned:  true,
			wantScaleUp:    true,
			wantCheckerHit: true,
		},
		{
			name:        "no leaders means initial creation, slot check skipped",
			leaderCount: 0,
			wantScaleUp: false,
		},
		{
			name:        "more than two leaders is always a formed cluster, slot check skipped",
			leaderCount: 3,
			wantScaleUp: true,
		},
		{
			name:           "slot check error is propagated",
			leaderCount:    1,
			checkErr:       errors.New("connection refused"),
			wantErr:        true,
			wantCheckerHit: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			checker := &fakeChecker{slotsAssigned: tt.slotsAssigned, err: tt.checkErr}
			r := &Reconciler{Checker: checker}

			scaleUp, err := r.shouldScaleUpExistingCluster(context.Background(), &rcvb2.RedisCluster{}, tt.leaderCount)

			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.wantScaleUp, scaleUp)
			}
			assert.Equal(t, tt.wantCheckerHit, checker.called)
		})
	}
}

func TestReapplyDynamicConfigIfReady(t *testing.T) {
	newCluster := func(state rcvb2.RedisClusterState, maxmemory string) *rcvb2.RedisCluster {
		return &rcvb2.RedisCluster{
			Spec: rcvb2.RedisClusterSpec{
				RedisConfig: &common.RedisConfig{
					DynamicConfig: []string{"maxmemory " + maxmemory},
				},
			},
			Status: rcvb2.RedisClusterStatus{State: state},
		}
	}

	t.Run("re-applies on successive reconciles once Ready, picking up spec edits", func(t *testing.T) {
		var applied [][]string
		r := &Reconciler{setDynamicConfig: func(_ context.Context, _ kubernetes.Interface, cr *rcvb2.RedisCluster) error {
			applied = append(applied, cr.Spec.RedisConfig.DynamicConfig)
			return nil
		}}

		assert.NoError(t, r.reapplyDynamicConfigIfReady(context.TODO(), newCluster(rcvb2.RedisClusterReady, "100mb")))
		assert.NoError(t, r.reapplyDynamicConfigIfReady(context.TODO(), newCluster(rcvb2.RedisClusterReady, "200mb")))
		assert.Equal(t, [][]string{{"maxmemory 100mb"}, {"maxmemory 200mb"}}, applied)
	})

	t.Run("skips clusters that are not Ready", func(t *testing.T) {
		for _, state := range []rcvb2.RedisClusterState{
			rcvb2.RedisClusterInitializing,
			rcvb2.RedisClusterBootstrap,
			rcvb2.RedisClusterFailed,
			"",
		} {
			called := false
			r := &Reconciler{setDynamicConfig: func(context.Context, kubernetes.Interface, *rcvb2.RedisCluster) error {
				called = true
				return nil
			}}
			assert.NoError(t, r.reapplyDynamicConfigIfReady(context.TODO(), newCluster(state, "100mb")))
			assert.False(t, called, "dynamic config applied in state %q", state)
		}
	})

	t.Run("returns the apply error", func(t *testing.T) {
		wantErr := errors.New("CONFIG SET failed")
		r := &Reconciler{setDynamicConfig: func(context.Context, kubernetes.Interface, *rcvb2.RedisCluster) error {
			return wantErr
		}}
		assert.ErrorIs(t, r.reapplyDynamicConfigIfReady(context.TODO(), newCluster(rcvb2.RedisClusterReady, "100mb")), wantErr)
	})
}
