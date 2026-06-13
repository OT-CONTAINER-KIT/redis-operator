package redisreplication

import (
	"context"
	"testing"

	commonapi "github.com/OT-CONTAINER-KIT/redis-operator/api/common/v1beta2"
	rrvb2 "github.com/OT-CONTAINER-KIT/redis-operator/api/redisreplication/v1beta2"
	rsvb2 "github.com/OT-CONTAINER-KIT/redis-operator/api/redissentinel/v1beta2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/fake"
	"k8s.io/utils/ptr"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	clientfake "sigs.k8s.io/controller-runtime/pkg/client/fake"
)

func TestReconcileRedisSkipsReplicationChangesWhenTopologyIsIncomplete(t *testing.T) {
	createCalled := false
	r := &Reconciler{
		K8sClient: fake.NewSimpleClientset(),
		RedisNodesByRole: func(_ context.Context, _ kubernetes.Interface, _ *rrvb2.RedisReplication, role string) ([]string, error) {
			if role == "master" {
				return []string{"example-replication-0"}, nil
			}
			return []string{"example-replication-1"}, nil
		},
		RedisReplicationRealMaster: func(context.Context, kubernetes.Interface, *rrvb2.RedisReplication, []string) string {
			return ""
		},
		CreateRedisReplicationLink: func(context.Context, kubernetes.Interface, *rrvb2.RedisReplication, []string, string) error {
			createCalled = true
			return nil
		},
	}
	result, err := r.reconcileRedis(context.Background(), newReplicationInstanceForTest())

	require.NoError(t, err)
	assert.Equal(t, ctrl.Result{}, result)
	assert.False(t, createCalled)
}

func TestReconcileRedisSkipsReplicationChangesWhenMultipleMastersAreObservedButTopologyIsIncomplete(t *testing.T) {
	createCalled := false
	r := &Reconciler{
		K8sClient: fake.NewSimpleClientset(),
		RedisNodesByRole: func(_ context.Context, _ kubernetes.Interface, _ *rrvb2.RedisReplication, role string) ([]string, error) {
			if role == "master" {
				return []string{"example-replication-0", "example-replication-1"}, nil
			}
			return nil, nil
		},
		RedisReplicationRealMaster: func(context.Context, kubernetes.Interface, *rrvb2.RedisReplication, []string) string {
			return "example-replication-1"
		},
		CreateRedisReplicationLink: func(context.Context, kubernetes.Interface, *rrvb2.RedisReplication, []string, string) error {
			createCalled = true
			return nil
		},
	}

	result, err := r.reconcileRedis(context.Background(), newReplicationInstanceForTest())

	require.NoError(t, err)
	assert.Equal(t, ctrl.Result{}, result)
	assert.False(t, createCalled)
}

func TestReconcileRedisKeepsHealthyBehaviorWhenTopologyIsComplete(t *testing.T) {
	createCalled := false
	var gotPods []string
	var gotMaster string
	r := &Reconciler{
		K8sClient: fake.NewSimpleClientset(),
		RedisNodesByRole: func(_ context.Context, _ kubernetes.Interface, _ *rrvb2.RedisReplication, role string) ([]string, error) {
			if role == "master" {
				return []string{"example-replication-0", "example-replication-1"}, nil
			}
			return []string{"example-replication-2"}, nil
		},
		RedisReplicationRealMaster: func(context.Context, kubernetes.Interface, *rrvb2.RedisReplication, []string) string {
			return "example-replication-1"
		},
		CreateRedisReplicationLink: func(_ context.Context, _ kubernetes.Interface, _ *rrvb2.RedisReplication, pods []string, realMaster string) error {
			createCalled = true
			gotPods = append([]string{}, pods...)
			gotMaster = realMaster
			return nil
		},
	}
	result, err := r.reconcileRedis(context.Background(), newReplicationInstanceForTest())

	require.NoError(t, err)
	assert.Equal(t, ctrl.Result{}, result)
	assert.True(t, createCalled)
	assert.ElementsMatch(t, []string{"example-replication-0", "example-replication-1"}, gotPods)
	assert.Equal(t, "example-replication-1", gotMaster)
}

func TestReconcileStatusStillRunsWhenOnePodIsUnobserved(t *testing.T) {
	scheme := runtime.NewScheme()
	require.NoError(t, rrvb2.AddToScheme(scheme))

	seedInstance := newReplicationInstanceForTest()
	ctrlClient := clientfake.NewClientBuilder().
		WithScheme(scheme).
		WithStatusSubresource(seedInstance).
		WithObjects(seedInstance.DeepCopy()).
		Build()

	instance := &rrvb2.RedisReplication{}
	require.NoError(t, ctrlClient.Get(context.Background(), client.ObjectKeyFromObject(seedInstance), instance))

	healer := &fakeHealer{}
	r := &Reconciler{
		Client:    ctrlClient,
		K8sClient: fake.NewSimpleClientset(),
		Healer:    healer,
		RedisNodesByRole: func(_ context.Context, _ kubernetes.Interface, _ *rrvb2.RedisReplication, role string) ([]string, error) {
			if role == "master" {
				return []string{"example-replication-1"}, nil
			}
			return nil, nil
		},
		RedisReplicationRealMaster: func(context.Context, kubernetes.Interface, *rrvb2.RedisReplication, []string) string {
			return ""
		},
	}

	result, err := r.reconcileStatus(context.Background(), instance)

	require.NoError(t, err)
	assert.Equal(t, ctrl.Result{}, result)
	assert.True(t, healer.updateCalled)

	updated := &rrvb2.RedisReplication{}
	require.NoError(t, ctrlClient.Get(context.Background(), client.ObjectKeyFromObject(instance), updated))
	assert.Equal(t, "example-replication-1", updated.Status.MasterNode)
}

func TestReconcileRedisSkipsSentinelReconfigurationWhenTopologyIsIncompleteAndMasterIsAmbiguous(t *testing.T) {
	createCalled := false
	sentinelCalled := false
	r := &Reconciler{
		StatefulSet: &fakeStatefulSetService{},
		K8sClient:   fake.NewSimpleClientset(),
		RedisNodesByRole: func(_ context.Context, _ kubernetes.Interface, _ *rrvb2.RedisReplication, role string) ([]string, error) {
			if role == "master" {
				return []string{"example-replication-0", "example-replication-1"}, nil
			}
			return nil, nil
		},
		RedisReplicationRealMaster: func(context.Context, kubernetes.Interface, *rrvb2.RedisReplication, []string) string {
			return ""
		},
		CreateRedisReplicationLink: func(context.Context, kubernetes.Interface, *rrvb2.RedisReplication, []string, string) error {
			createCalled = true
			return nil
		},
		ConfigureSentinel: func(context.Context, *rrvb2.RedisReplication, string) error {
			sentinelCalled = true
			return nil
		},
	}

	result, err := r.reconcileRedis(context.Background(), newSentinelReplicationInstanceForTest())

	require.NoError(t, err)
	assert.Equal(t, ctrl.Result{}, result)
	assert.False(t, createCalled)
	assert.False(t, sentinelCalled)
}

func TestReconcileRedisConfiguresSentinelForSingleObservedMaster(t *testing.T) {
	sentinelCalled := false
	var gotMaster string
	r := &Reconciler{
		StatefulSet: &fakeStatefulSetService{},
		K8sClient:   fake.NewSimpleClientset(),
		RedisNodesByRole: func(_ context.Context, _ kubernetes.Interface, _ *rrvb2.RedisReplication, role string) ([]string, error) {
			if role == "master" {
				return []string{"example-replication-1"}, nil
			}
			return nil, nil
		},
		RedisReplicationRealMaster: func(context.Context, kubernetes.Interface, *rrvb2.RedisReplication, []string) string {
			return ""
		},
		ConfigureSentinel: func(_ context.Context, _ *rrvb2.RedisReplication, master string) error {
			sentinelCalled = true
			gotMaster = master
			return nil
		},
	}

	result, err := r.reconcileRedis(context.Background(), newSentinelReplicationInstanceForTest())

	require.NoError(t, err)
	assert.Equal(t, ctrl.Result{}, result)
	assert.True(t, sentinelCalled)
	assert.Equal(t, "example-replication-1", gotMaster)
}

func newReplicationInstanceForTest() *rrvb2.RedisReplication {
	size := int32(3)
	return &rrvb2.RedisReplication{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "example-replication",
			Namespace: "default",
		},
		Spec: rrvb2.RedisReplicationSpec{
			Size: ptr.To(size),
			KubernetesConfig: commonapi.KubernetesConfig{
				Image: "redis:7",
			},
		},
	}
}

func newSentinelReplicationInstanceForTest() *rrvb2.RedisReplication {
	instance := newReplicationInstanceForTest()
	instance.Spec.Sentinel = &rrvb2.Sentinel{Size: 3}
	return instance
}

type fakeStatefulSetService struct{}

func (f *fakeStatefulSetService) IsStatefulSetReady(context.Context, string, string) bool {
	return true
}

func (f *fakeStatefulSetService) GetStatefulSetReplicas(context.Context, string, string) int32 {
	return 0
}

func (f *fakeStatefulSetService) GetStatefulSetReadyReplicas(context.Context, string, string) int32 {
	return 0
}

type fakeHealer struct {
	updateCalled bool
}

func (f *fakeHealer) SentinelMonitor(context.Context, *rsvb2.RedisSentinel, string) error {
	return nil
}

func (f *fakeHealer) SentinelSet(context.Context, *rsvb2.RedisSentinel, string) error {
	return nil
}

func (f *fakeHealer) SentinelReset(context.Context, *rsvb2.RedisSentinel) error {
	return nil
}

func (f *fakeHealer) UpdateRedisRoleLabel(context.Context, string, map[string]string, *commonapi.ExistingPasswordSecret, *commonapi.TLSConfig) error {
	f.updateCalled = true
	return nil
}

func TestUpdateRedisReplicationMasterStateClassification(t *testing.T) {
	scheme := runtime.NewScheme()
	require.NoError(t, rrvb2.AddToScheme(scheme))

	tests := []struct {
		name          string
		size          *int32
		masterNode    string
		readyReplicas int32
		wantState     rrvb2.RedisReplicationState
		wantReason    string
	}{
		{
			name:          "no ready replicas is initializing even without a master",
			size:          ptr.To(int32(3)),
			masterNode:    "",
			readyReplicas: 0,
			wantState:     rrvb2.RedisReplicationInitializing,
			wantReason:    rrvb2.InitializingReplicationReason,
		},
		{
			name:          "ready replicas but no elected master is failed",
			size:          ptr.To(int32(3)),
			masterNode:    "",
			readyReplicas: 2,
			wantState:     rrvb2.RedisReplicationFailed,
			wantReason:    rrvb2.FailedReplicationReason,
		},
		{
			name:          "partial replicas with a master is initializing",
			size:          ptr.To(int32(3)),
			masterNode:    "example-replication-0",
			readyReplicas: 2,
			wantState:     rrvb2.RedisReplicationInitializing,
			wantReason:    rrvb2.InitializingReplicationReason,
		},
		{
			name:          "all replicas with a master is ready",
			size:          ptr.To(int32(3)),
			masterNode:    "example-replication-0",
			readyReplicas: 3,
			wantState:     rrvb2.RedisReplicationReady,
			wantReason:    rrvb2.ReadyReplicationReason,
		},
		{
			name:          "nil size with a master does not panic and is ready",
			size:          nil,
			masterNode:    "example-replication-0",
			readyReplicas: 1,
			wantState:     rrvb2.RedisReplicationReady,
			wantReason:    rrvb2.ReadyReplicationReason,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			instance := newReplicationInstanceForTest()
			instance.Spec.Size = tt.size

			ctrlClient := clientfake.NewClientBuilder().
				WithScheme(scheme).
				WithStatusSubresource(instance).
				WithObjects(instance.DeepCopy()).
				Build()

			r := &Reconciler{
				Client:    ctrlClient,
				K8sClient: fake.NewSimpleClientset(),
			}

			fresh := &rrvb2.RedisReplication{}
			require.NoError(t, ctrlClient.Get(context.Background(), client.ObjectKeyFromObject(instance), fresh))

			require.NoError(t, r.UpdateRedisReplicationMaster(context.Background(), fresh, tt.masterNode, tt.readyReplicas))

			updated := &rrvb2.RedisReplication{}
			require.NoError(t, ctrlClient.Get(context.Background(), client.ObjectKeyFromObject(instance), updated))
			assert.Equal(t, tt.wantState, updated.Status.State)
			assert.Equal(t, tt.wantReason, updated.Status.Reason)
			assert.Equal(t, tt.readyReplicas, updated.Status.ReadyReplicas)
			assert.Equal(t, tt.masterNode, updated.Status.MasterNode)
		})
	}
}

func TestReplicationUpdateStatusSkipsWriteWhenUnchanged(t *testing.T) {
	scheme := runtime.NewScheme()
	require.NoError(t, rrvb2.AddToScheme(scheme))

	instance := newReplicationInstanceForTest()
	ctrlClient := clientfake.NewClientBuilder().
		WithScheme(scheme).
		WithStatusSubresource(instance).
		WithObjects(instance.DeepCopy()).
		Build()

	r := &Reconciler{
		Client:    ctrlClient,
		K8sClient: fake.NewSimpleClientset(),
	}

	status := rrvb2.RedisReplicationStatus{
		State:         rrvb2.RedisReplicationReady,
		Reason:        rrvb2.ReadyReplicationReason,
		ReadyReplicas: 3,
		MasterNode:    "example-replication-0",
	}

	// First update writes the status and bumps the resourceVersion.
	fresh := &rrvb2.RedisReplication{}
	require.NoError(t, ctrlClient.Get(context.Background(), client.ObjectKeyFromObject(instance), fresh))
	require.NoError(t, r.updateStatus(context.Background(), fresh, status))

	written := &rrvb2.RedisReplication{}
	require.NoError(t, ctrlClient.Get(context.Background(), client.ObjectKeyFromObject(instance), written))
	assert.Equal(t, rrvb2.RedisReplicationReady, written.Status.State)
	rvAfterWrite := written.ResourceVersion

	// Second update with an identical status is a no-op (reflect.DeepEqual guard),
	// so the resourceVersion must not change.
	require.NoError(t, r.updateStatus(context.Background(), written, status))
	noop := &rrvb2.RedisReplication{}
	require.NoError(t, ctrlClient.Get(context.Background(), client.ObjectKeyFromObject(instance), noop))
	assert.Equal(t, rvAfterWrite, noop.ResourceVersion, "identical status update should be skipped")
}
