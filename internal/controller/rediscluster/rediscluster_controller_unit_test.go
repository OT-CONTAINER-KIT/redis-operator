package rediscluster

import (
	"context"
	"errors"
	"testing"

	commonapi "github.com/OT-CONTAINER-KIT/redis-operator/api/common/v1beta2"
	rcvb2 "github.com/OT-CONTAINER-KIT/redis-operator/api/rediscluster/v1beta2"
	"github.com/OT-CONTAINER-KIT/redis-operator/internal/controller/common/redis"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	k8sfake "k8s.io/client-go/kubernetes/fake"
	"k8s.io/client-go/tools/record"
	"k8s.io/utils/ptr"
	ctrl "sigs.k8s.io/controller-runtime"
	clientfake "sigs.k8s.io/controller-runtime/pkg/client/fake"
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

type fakeStatefulSet struct {
	replicas int32
	ready    bool
}

func (f *fakeStatefulSet) IsStatefulSetReady(context.Context, string, string) bool {
	return f.ready
}

func (f *fakeStatefulSet) GetStatefulSetReplicas(context.Context, string, string) int32 {
	return f.replicas
}

// A RedisCluster whose desired leader count is 0 while its leader StatefulSet
// still has pods must not panic with "integer divide by zero" (issue #1942).
func TestReconcileRefusesDownscaleToZeroLeaders(t *testing.T) {
	tests := []struct {
		name string
		spec rcvb2.RedisClusterSpec
	}{
		{
			name: "clusterSize 0",
			spec: rcvb2.RedisClusterSpec{ClusterSize: ptr.To(int32(0))},
		},
		{
			name: "redisLeader.replicas 0",
			spec: rcvb2.RedisClusterSpec{
				ClusterSize: ptr.To(int32(3)),
				RedisLeader: rcvb2.RedisLeader{RedisLeader: commonapi.RedisLeader{Replicas: ptr.To(int32(0))}},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			scheme := runtime.NewScheme()
			require.NoError(t, rcvb2.AddToScheme(scheme))

			instance := &rcvb2.RedisCluster{
				ObjectMeta: metav1.ObjectMeta{Name: "example", Namespace: "default"},
				Spec:       tt.spec,
			}
			recorder := record.NewFakeRecorder(10)
			r := &Reconciler{
				Client:      clientfake.NewClientBuilder().WithScheme(scheme).WithObjects(instance).Build(),
				StatefulSet: &fakeStatefulSet{replicas: 3, ready: true},
				K8sClient:   k8sfake.NewSimpleClientset(),
				Checker:     &fakeChecker{},
				Recorder:    recorder,
			}

			var result ctrl.Result
			var err error
			require.NotPanics(t, func() {
				result, err = r.Reconcile(context.Background(), ctrl.Request{
					NamespacedName: types.NamespacedName{Name: instance.Name, Namespace: instance.Namespace},
				})
			})
			require.NoError(t, err)
			assert.Equal(t, ctrl.Result{}, result)

			require.Len(t, recorder.Events, 1)
			event := <-recorder.Events
			assert.Contains(t, event, corev1.EventTypeWarning)
			assert.Contains(t, event, "Refusing to downscale Redis cluster from 3 to 0 leaders")
		})
	}
}
