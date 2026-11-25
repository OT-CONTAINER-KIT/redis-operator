package rediscluster

import (
	"context"
	"errors"
	"testing"

	commonapi "github.com/OT-CONTAINER-KIT/redis-operator/api/common/v1beta2"
	rcvb2 "github.com/OT-CONTAINER-KIT/redis-operator/api/rediscluster/v1beta2"
	"github.com/OT-CONTAINER-KIT/redis-operator/internal/controller/common/redis"
	"github.com/OT-CONTAINER-KIT/redis-operator/internal/k8sutils"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes"
	k8sfake "k8s.io/client-go/kubernetes/fake"
	k8stesting "k8s.io/client-go/testing"
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

func TestReapplyDynamicConfigIfReady(t *testing.T) {
	newCluster := func(state rcvb2.RedisClusterState, maxmemory string) *rcvb2.RedisCluster {
		return &rcvb2.RedisCluster{
			Spec: rcvb2.RedisClusterSpec{
				RedisConfig: &commonapi.RedisConfig{
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

func TestReconcileKeepsFollowersWhenRedisMembershipCannotBeRead(t *testing.T) {
	ctx := context.Background()
	scheme := runtime.NewScheme()
	require.NoError(t, rcvb2.AddToScheme(scheme))
	cluster := &rcvb2.RedisCluster{
		ObjectMeta: metav1.ObjectMeta{Name: "example", Namespace: "default", Finalizers: []string{RedisClusterFinalizer}},
		Spec: rcvb2.RedisClusterSpec{
			ClusterSize:   ptr.To(int32(3)),
			RedisFollower: rcvb2.RedisFollower{RedisFollower: commonapi.RedisFollower{Replicas: ptr.To(int32(1))}},
		},
		Status: rcvb2.RedisClusterStatus{
			State:                 rcvb2.RedisClusterInitializing,
			Reason:                rcvb2.InitializingClusterFollowerReason,
			ReadyLeaderReplicas:   3,
			ReadyFollowerReplicas: 2,
		},
	}
	leader := &appsv1.StatefulSet{
		ObjectMeta: metav1.ObjectMeta{Name: "example-leader", Namespace: cluster.Namespace},
		Spec:       appsv1.StatefulSetSpec{Replicas: ptr.To(int32(3))},
		Status:     appsv1.StatefulSetStatus{ReadyReplicas: 3, UpdatedReplicas: 3},
	}
	follower := &appsv1.StatefulSet{
		ObjectMeta: metav1.ObjectMeta{Name: "example-follower", Namespace: cluster.Namespace},
		Spec:       appsv1.StatefulSetSpec{Replicas: ptr.To(int32(2))},
	}
	kube := k8sfake.NewSimpleClientset(leader, follower)
	var followerUpdateAttempted bool
	kube.PrependReactor("update", "statefulsets", func(action k8stesting.Action) (bool, runtime.Object, error) {
		sts := action.(k8stesting.UpdateAction).GetObject().(*appsv1.StatefulSet)
		if sts.Name == follower.Name {
			followerUpdateAttempted = true
			return true, nil, errors.New("follower StatefulSet update attempted")
		}
		// Match the API server's separation of StatefulSet spec and status.
		sts.Status = leader.Status
		return false, nil, nil
	})
	r := &Reconciler{
		Client:      clientfake.NewClientBuilder().WithScheme(scheme).WithStatusSubresource(cluster).WithObjects(cluster).Build(),
		StatefulSet: k8sutils.NewStatefulSetService(kube),
		K8sClient:   kube,
		Recorder:    record.NewFakeRecorder(10),
	}

	_, err := r.Reconcile(ctx, ctrl.Request{NamespacedName: types.NamespacedName{Name: cluster.Name, Namespace: cluster.Namespace}})

	assert.ErrorContains(t, err, "no Redis address")
	assert.False(t, followerUpdateAttempted, "follower StatefulSet was updated without verifying Redis membership")
}
