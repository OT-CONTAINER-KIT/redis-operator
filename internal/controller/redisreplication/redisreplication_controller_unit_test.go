package redisreplication

import (
	"context"
	"errors"
	"fmt"
	"testing"

	commonapi "github.com/OT-CONTAINER-KIT/redis-operator/api/common/v1beta2"
	rrvb2 "github.com/OT-CONTAINER-KIT/redis-operator/api/redisreplication/v1beta2"
	rsvb2 "github.com/OT-CONTAINER-KIT/redis-operator/api/redissentinel/v1beta2"
	"github.com/OT-CONTAINER-KIT/redis-operator/internal/controller/common"
	"github.com/OT-CONTAINER-KIT/redis-operator/internal/k8sutils"
	"github.com/OT-CONTAINER-KIT/redis-operator/internal/service/redis"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
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
		K8sClient:                fake.NewSimpleClientset(),
		RedisReplicationTopology: topologyOf([]string{"example-replication-0"}, []string{"example-replication-1"}, []string{"example-replication-2"}),
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
		K8sClient:                fake.NewSimpleClientset(),
		RedisReplicationTopology: topologyOf([]string{"example-replication-0", "example-replication-1"}, nil, []string{"example-replication-2"}),
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
		K8sClient:                fake.NewSimpleClientset(),
		RedisReplicationTopology: topologyOf([]string{"example-replication-0", "example-replication-1"}, []string{"example-replication-2"}, nil),
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

func TestReconcileRedisSkipsSentinelReconfigurationWhenTopologyIsIncompleteAndMasterIsAmbiguous(t *testing.T) {
	createCalled := false
	sentinelCalled := false
	r := &Reconciler{
		StatefulSet:              &fakeStatefulSetService{},
		K8sClient:                fake.NewSimpleClientset(),
		RedisReplicationTopology: topologyOf([]string{"example-replication-0", "example-replication-1"}, nil, []string{"example-replication-2"}),
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

// The master is unreachable and a replica pod has been recreated: it boots as a
// standalone master with no replicas, while the pod still replicating from the
// unreachable master reports as slave. The standalone node is the only master
// observed, but it is not the master; repointing Sentinel at it would replicate
// an empty dataset over the surviving replica.
func TestReconcileRedisSkipsSentinelReconfigurationForLoneStandaloneMasterWhenTopologyIsIncomplete(t *testing.T) {
	createCalled := false
	sentinelCalled := false
	var askedSentinelAbout []string
	r := &Reconciler{
		StatefulSet:              &fakeStatefulSetService{},
		K8sClient:                fake.NewSimpleClientset(),
		RedisReplicationTopology: topologyOf([]string{"example-replication-1"}, []string{"example-replication-2"}, []string{"example-replication-0"}),
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
		SentinelMonitoredMaster: func(_ context.Context, _ *rrvb2.RedisReplication, candidates []string) (string, error) {
			askedSentinelAbout = candidates
			return "", nil
		},
	}
	instance := newSentinelReplicationInstanceForTest()
	instance.Status.MasterNode = "example-replication-0"

	result, err := r.reconcileRedis(context.Background(), instance)

	require.NoError(t, err)
	assert.Equal(t, ctrl.Result{}, result)
	assert.False(t, createCalled)
	assert.False(t, sentinelCalled)
	assert.Equal(t, []string{"example-replication-1"}, askedSentinelAbout)
}

// Sentinel has failed over: the promoted pod has the surviving replica attached,
// which a standalone node cannot show. It is the master even though the old
// master is still unobserved, and Sentinel is kept in sync with it.
func TestReconcileRedisConfiguresSentinelForLoneMasterWithAttachedReplicasWhenTopologyIsIncomplete(t *testing.T) {
	sentinelCalled := false
	var gotMaster string
	r := &Reconciler{
		StatefulSet:              &fakeStatefulSetService{},
		K8sClient:                fake.NewSimpleClientset(),
		RedisReplicationTopology: topologyOf([]string{"example-replication-2"}, []string{"example-replication-1"}, []string{"example-replication-0"}),
		RedisReplicationRealMaster: func(_ context.Context, _ kubernetes.Interface, _ *rrvb2.RedisReplication, masterPods []string) string {
			require.Equal(t, []string{"example-replication-2"}, masterPods)
			return "example-replication-2"
		},
		ConfigureSentinel: func(_ context.Context, _ *rrvb2.RedisReplication, master string) error {
			sentinelCalled = true
			gotMaster = master
			return nil
		},
	}
	instance := newSentinelReplicationInstanceForTest()
	instance.Status.MasterNode = "example-replication-0"

	result, err := r.reconcileRedis(context.Background(), instance)

	require.NoError(t, err)
	assert.Equal(t, ctrl.Result{}, result)
	assert.True(t, sentinelCalled)
	assert.Equal(t, "example-replication-2", gotMaster)
}

// A two-pod replication has lost its master. Sentinel promoted the survivor,
// which has no replica left to prove itself with, and the recorded master is
// the pod that is gone. Sentinel's own view is what settles this: the promoted
// pod is recorded so that the old master, once back, is attached under it and
// not the other way round.
func TestReconcileRedisConfiguresSentinelForLoneMasterThatSentinelReports(t *testing.T) {
	sentinelCalled := false
	var gotMaster string
	r := &Reconciler{
		StatefulSet:              &fakeStatefulSetService{},
		K8sClient:                fake.NewSimpleClientset(),
		RedisReplicationTopology: topologyOf([]string{"example-replication-1"}, nil, []string{"example-replication-0"}),
		RedisReplicationRealMaster: func(context.Context, kubernetes.Interface, *rrvb2.RedisReplication, []string) string {
			return ""
		},
		ConfigureSentinel: func(_ context.Context, _ *rrvb2.RedisReplication, master string) error {
			sentinelCalled = true
			gotMaster = master
			return nil
		},
		SentinelMonitoredMaster: sentinelMonitoring("example-replication-1"),
	}
	instance := newSentinelReplicationInstanceForTest()
	instance.Spec.Size = ptr.To(int32(2))
	instance.Status.MasterNode = "example-replication-0"

	result, err := r.reconcileRedis(context.Background(), instance)

	require.NoError(t, err)
	assert.Equal(t, ctrl.Result{}, result)
	assert.True(t, sentinelCalled)
	assert.Equal(t, "example-replication-1", gotMaster)
}

// If Sentinel cannot be asked, the candidate stays unconfirmed; the reconcile
// itself still succeeds and retries on its normal cadence.
func TestReconcileRedisSkipsSentinelReconfigurationWhenSentinelCannotBeAsked(t *testing.T) {
	sentinelCalled := false
	r := &Reconciler{
		StatefulSet:              &fakeStatefulSetService{},
		K8sClient:                fake.NewSimpleClientset(),
		RedisReplicationTopology: topologyOf([]string{"example-replication-1"}, nil, []string{"example-replication-0", "example-replication-2"}),
		RedisReplicationRealMaster: func(context.Context, kubernetes.Interface, *rrvb2.RedisReplication, []string) string {
			return ""
		},
		ConfigureSentinel: func(context.Context, *rrvb2.RedisReplication, string) error {
			sentinelCalled = true
			return nil
		},
		SentinelMonitoredMaster: func(context.Context, *rrvb2.RedisReplication, []string) (string, error) {
			return "", errors.New("sentinel unreachable")
		},
	}

	result, err := r.reconcileRedis(context.Background(), newSentinelReplicationInstanceForTest())

	require.NoError(t, err)
	assert.Equal(t, ctrl.Result{}, result)
	assert.False(t, sentinelCalled)
}

// The recorded master was lost, Sentinel promoted the other pod, and the old
// master is back as an empty standalone master before the promotion was ever
// recorded. Two replica-less masters and a stale Status.MasterNode are exactly
// what the bootstrap election would resolve the wrong way round; Sentinel's
// pick takes precedence so the returning pod is attached under the promoted one.
func TestReconcileRedisAttachesReturningMasterUnderTheMasterSentinelMonitors(t *testing.T) {
	var gotPods []string
	var gotMaster, gotSentinelMaster string
	r := &Reconciler{
		StatefulSet:              &fakeStatefulSetService{},
		K8sClient:                fake.NewSimpleClientset(),
		RedisReplicationTopology: topologyOf([]string{"example-replication-0", "example-replication-1"}, nil, nil),
		RedisReplicationRealMaster: func(context.Context, kubernetes.Interface, *rrvb2.RedisReplication, []string) string {
			return ""
		},
		CreateRedisReplicationLink: func(_ context.Context, _ kubernetes.Interface, _ *rrvb2.RedisReplication, pods []string, realMaster string) error {
			gotPods = append([]string{}, pods...)
			gotMaster = realMaster
			return nil
		},
		ConfigureSentinel: func(_ context.Context, _ *rrvb2.RedisReplication, master string) error {
			gotSentinelMaster = master
			return nil
		},
		SentinelMonitoredMaster: sentinelMonitoring("example-replication-1"),
	}
	instance := newSentinelReplicationInstanceForTest()
	instance.Spec.Size = ptr.To(int32(2))
	instance.Status.MasterNode = "example-replication-0"

	result, err := r.reconcileRedis(context.Background(), instance)

	require.NoError(t, err)
	assert.Equal(t, ctrl.Result{}, result)
	assert.ElementsMatch(t, []string{"example-replication-0", "example-replication-1"}, gotPods)
	assert.Equal(t, "example-replication-1", gotMaster)
	assert.Equal(t, "example-replication-1", gotSentinelMaster)
}

// With every pod observed a lone master is conclusive on its own.
func TestReconcileRedisConfiguresSentinelForLoneMasterWhenTopologyIsComplete(t *testing.T) {
	sentinelCalled := false
	var gotMaster string
	r := &Reconciler{
		StatefulSet:              &fakeStatefulSetService{},
		K8sClient:                fake.NewSimpleClientset(),
		RedisReplicationTopology: topologyOf([]string{"example-replication-1"}, []string{"example-replication-0", "example-replication-2"}, nil),
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

// Every pod answered and every one of them is a replica: there is no master,
// and the recorded one is provably not it. Keeping it would feed a stale name
// to the bootstrap election and the sentinel controller's fallback.
func TestReconcileStatusClearsMasterWhenEveryPodReportsSlave(t *testing.T) {
	scheme := runtime.NewScheme()
	require.NoError(t, rrvb2.AddToScheme(scheme))

	seedInstance := newReplicationInstanceForTest()
	seedInstance.Status.MasterNode = "example-replication-0"
	ctrlClient := clientfake.NewClientBuilder().
		WithScheme(scheme).
		WithStatusSubresource(seedInstance).
		WithObjects(seedInstance.DeepCopy()).
		Build()

	instance := &rrvb2.RedisReplication{}
	require.NoError(t, ctrlClient.Get(context.Background(), client.ObjectKeyFromObject(seedInstance), instance))

	r := &Reconciler{
		Client:                   ctrlClient,
		K8sClient:                fake.NewSimpleClientset(),
		Healer:                   &fakeHealer{},
		RedisReplicationTopology: topologyOf(nil, []string{"example-replication-0", "example-replication-1", "example-replication-2"}, nil),
		RedisReplicationRealMaster: func(context.Context, kubernetes.Interface, *rrvb2.RedisReplication, []string) string {
			return ""
		},
	}

	result, err := r.reconcileStatus(context.Background(), instance)

	require.NoError(t, err)
	assert.Equal(t, ctrl.Result{}, result)

	updated := &rrvb2.RedisReplication{}
	require.NoError(t, ctrlClient.Get(context.Background(), client.ObjectKeyFromObject(instance), updated))
	assert.Empty(t, updated.Status.MasterNode)
}

// Several masters that cannot be told apart are an ambiguous view, not an
// absent master, so the recorded master survives it.
func TestReconcileStatusKeepsLastKnownMasterWhenMastersCannotBeToldApart(t *testing.T) {
	scheme := runtime.NewScheme()
	require.NoError(t, rrvb2.AddToScheme(scheme))

	seedInstance := newReplicationInstanceForTest()
	seedInstance.Status.MasterNode = "example-replication-0"
	ctrlClient := clientfake.NewClientBuilder().
		WithScheme(scheme).
		WithStatusSubresource(seedInstance).
		WithObjects(seedInstance.DeepCopy()).
		Build()

	instance := &rrvb2.RedisReplication{}
	require.NoError(t, ctrlClient.Get(context.Background(), client.ObjectKeyFromObject(seedInstance), instance))

	r := &Reconciler{
		Client:                   ctrlClient,
		K8sClient:                fake.NewSimpleClientset(),
		Healer:                   &fakeHealer{},
		RedisReplicationTopology: topologyOf([]string{"example-replication-0", "example-replication-1", "example-replication-2"}, nil, nil),
		RedisReplicationRealMaster: func(context.Context, kubernetes.Interface, *rrvb2.RedisReplication, []string) string {
			return ""
		},
	}

	result, err := r.reconcileStatus(context.Background(), instance)

	require.NoError(t, err)
	assert.Equal(t, ctrl.Result{}, result)

	updated := &rrvb2.RedisReplication{}
	require.NoError(t, ctrlClient.Get(context.Background(), client.ObjectKeyFromObject(instance), updated))
	assert.Equal(t, "example-replication-0", updated.Status.MasterNode)
}

// A probe blip that hides every master must not clear the recorded master: both
// this controller and the sentinel controller fall back to Status.MasterNode.
func TestReconcileStatusKeepsLastKnownMasterWhenNoMasterIsObserved(t *testing.T) {
	scheme := runtime.NewScheme()
	require.NoError(t, rrvb2.AddToScheme(scheme))

	seedInstance := newReplicationInstanceForTest()
	seedInstance.Status.MasterNode = "example-replication-0"
	ctrlClient := clientfake.NewClientBuilder().
		WithScheme(scheme).
		WithStatusSubresource(seedInstance).
		WithObjects(seedInstance.DeepCopy()).
		Build()

	instance := &rrvb2.RedisReplication{}
	require.NoError(t, ctrlClient.Get(context.Background(), client.ObjectKeyFromObject(seedInstance), instance))

	r := &Reconciler{
		Client:                   ctrlClient,
		K8sClient:                fake.NewSimpleClientset(),
		Healer:                   &fakeHealer{},
		RedisReplicationTopology: topologyOf(nil, nil, []string{"example-replication-0", "example-replication-1", "example-replication-2"}),
		RedisReplicationRealMaster: func(context.Context, kubernetes.Interface, *rrvb2.RedisReplication, []string) string {
			return ""
		},
	}

	result, err := r.reconcileStatus(context.Background(), instance)

	require.NoError(t, err)
	assert.Equal(t, ctrl.Result{}, result)

	updated := &rrvb2.RedisReplication{}
	require.NoError(t, ctrlClient.Get(context.Background(), client.ObjectKeyFromObject(instance), updated))
	assert.Equal(t, "example-replication-0", updated.Status.MasterNode)
}

// ... but a real promotion is still recorded, so the guard cannot pin the status
// to a master that has been replaced. Here the old master has rejoined as a
// replica, so every pod is observed and the lone master is conclusive.
func TestReconcileStatusUpdatesMasterWhenANewMasterIsObserved(t *testing.T) {
	scheme := runtime.NewScheme()
	require.NoError(t, rrvb2.AddToScheme(scheme))

	seedInstance := newReplicationInstanceForTest()
	seedInstance.Status.MasterNode = "example-replication-0"
	ctrlClient := clientfake.NewClientBuilder().
		WithScheme(scheme).
		WithStatusSubresource(seedInstance).
		WithObjects(seedInstance.DeepCopy()).
		Build()

	instance := &rrvb2.RedisReplication{}
	require.NoError(t, ctrlClient.Get(context.Background(), client.ObjectKeyFromObject(seedInstance), instance))

	r := &Reconciler{
		Client:                   ctrlClient,
		K8sClient:                fake.NewSimpleClientset(),
		Healer:                   &fakeHealer{},
		RedisReplicationTopology: topologyOf([]string{"example-replication-1"}, []string{"example-replication-0", "example-replication-2"}, nil),
		RedisReplicationRealMaster: func(context.Context, kubernetes.Interface, *rrvb2.RedisReplication, []string) string {
			return ""
		},
	}

	result, err := r.reconcileStatus(context.Background(), instance)

	require.NoError(t, err)
	assert.Equal(t, ctrl.Result{}, result)

	updated := &rrvb2.RedisReplication{}
	require.NoError(t, ctrlClient.Get(context.Background(), client.ObjectKeyFromObject(instance), updated))
	assert.Equal(t, "example-replication-1", updated.Status.MasterNode)
}

// A promotion is also recorded while the old master is still unobserved, as
// long as the promoted pod shows attached replicas; the role labels are
// refreshed in the same pass, so one unreachable pod never stalls them.
func TestReconcileStatusUpdatesMasterWhenANewMasterWithAttachedReplicasIsObservedInAnIncompleteTopology(t *testing.T) {
	scheme := runtime.NewScheme()
	require.NoError(t, rrvb2.AddToScheme(scheme))

	seedInstance := newReplicationInstanceForTest()
	seedInstance.Status.MasterNode = "example-replication-0"
	ctrlClient := clientfake.NewClientBuilder().
		WithScheme(scheme).
		WithStatusSubresource(seedInstance).
		WithObjects(seedInstance.DeepCopy()).
		Build()

	instance := &rrvb2.RedisReplication{}
	require.NoError(t, ctrlClient.Get(context.Background(), client.ObjectKeyFromObject(seedInstance), instance))

	healer := &fakeHealer{}
	r := &Reconciler{
		Client:                   ctrlClient,
		K8sClient:                fake.NewSimpleClientset(),
		Healer:                   healer,
		RedisReplicationTopology: topologyOf([]string{"example-replication-1"}, []string{"example-replication-2"}, []string{"example-replication-0"}),
		RedisReplicationRealMaster: func(context.Context, kubernetes.Interface, *rrvb2.RedisReplication, []string) string {
			return "example-replication-1"
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

// The master is unreachable and a recreated replica answers as a standalone
// master with no replicas. It is the only master observed, but recording it
// would make every later bootstrap election attach the real master under an
// empty node.
func TestReconcileStatusKeepsLastKnownMasterWhenOnlyAStandaloneMasterIsObserved(t *testing.T) {
	tests := []struct {
		name     string
		instance func() *rrvb2.RedisReplication
	}{
		{
			name:     "without sentinel the standalone pod is not the recorded master",
			instance: newReplicationInstanceForTest,
		},
		{
			name:     "with sentinel the standalone pod is not the monitored master",
			instance: newSentinelReplicationInstanceForTest,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			scheme := runtime.NewScheme()
			require.NoError(t, rrvb2.AddToScheme(scheme))

			seedInstance := tt.instance()
			seedInstance.Status.MasterNode = "example-replication-0"
			ctrlClient := clientfake.NewClientBuilder().
				WithScheme(scheme).
				WithStatusSubresource(seedInstance).
				WithObjects(seedInstance.DeepCopy()).
				Build()

			instance := &rrvb2.RedisReplication{}
			require.NoError(t, ctrlClient.Get(context.Background(), client.ObjectKeyFromObject(seedInstance), instance))

			healer := &fakeHealer{}
			r := &Reconciler{
				Client:                   ctrlClient,
				K8sClient:                fake.NewSimpleClientset(),
				Healer:                   healer,
				RedisReplicationTopology: topologyOf([]string{"example-replication-1"}, []string{"example-replication-2"}, []string{"example-replication-0"}),
				RedisReplicationRealMaster: func(context.Context, kubernetes.Interface, *rrvb2.RedisReplication, []string) string {
					return ""
				},
				SentinelMonitoredMaster: sentinelMonitoring(""),
			}

			result, err := r.reconcileStatus(context.Background(), instance)

			require.NoError(t, err)
			assert.Equal(t, ctrl.Result{}, result)
			assert.True(t, healer.updateCalled)
			// The kept Status.MasterNode is a fallback, not an identification; the
			// healer must not take it as proof against other master labels.
			assert.Empty(t, healer.master)

			updated := &rrvb2.RedisReplication{}
			require.NoError(t, ctrlClient.Get(context.Background(), client.ObjectKeyFromObject(instance), updated))
			assert.Equal(t, "example-replication-0", updated.Status.MasterNode)
		})
	}
}

// A two-pod replication lost its master and Sentinel promoted the survivor. It
// has no replica to prove itself with, so Sentinel's word is what records it.
func TestReconcileStatusRecordsPromotedMasterReportedBySentinel(t *testing.T) {
	scheme := runtime.NewScheme()
	require.NoError(t, rrvb2.AddToScheme(scheme))

	seedInstance := newSentinelReplicationInstanceForTest()
	seedInstance.Spec.Size = ptr.To(int32(2))
	seedInstance.Status.MasterNode = "example-replication-0"
	ctrlClient := clientfake.NewClientBuilder().
		WithScheme(scheme).
		WithStatusSubresource(seedInstance).
		WithObjects(seedInstance.DeepCopy()).
		Build()

	instance := &rrvb2.RedisReplication{}
	require.NoError(t, ctrlClient.Get(context.Background(), client.ObjectKeyFromObject(seedInstance), instance))

	healer := &fakeHealer{}
	r := &Reconciler{
		Client:                   ctrlClient,
		K8sClient:                fake.NewSimpleClientset(),
		Healer:                   healer,
		RedisReplicationTopology: topologyOf([]string{"example-replication-1"}, nil, []string{"example-replication-0"}),
		RedisReplicationRealMaster: func(context.Context, kubernetes.Interface, *rrvb2.RedisReplication, []string) string {
			return ""
		},
		SentinelMonitoredMaster: sentinelMonitoring("example-replication-1"),
	}

	result, err := r.reconcileStatus(context.Background(), instance)

	require.NoError(t, err)
	assert.Equal(t, ctrl.Result{}, result)
	// The healer is told which pod is the master, so it can drop the lost
	// master's label even though the survivor has no replicas to show.
	assert.Equal(t, "example-replication-1", healer.master)

	updated := &rrvb2.RedisReplication{}
	require.NoError(t, ctrlClient.Get(context.Background(), client.ObjectKeyFromObject(instance), updated))
	assert.Equal(t, "example-replication-1", updated.Status.MasterNode)
}

// A fresh replication has no recorded master to fall back on. Until every pod
// is observed, a lone standalone master is still only a candidate.
func TestReconcileStatusLeavesMasterUnsetWhenOnlyAStandaloneMasterIsObservedDuringBootstrap(t *testing.T) {
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

	r := &Reconciler{
		Client:                   ctrlClient,
		K8sClient:                fake.NewSimpleClientset(),
		Healer:                   &fakeHealer{},
		RedisReplicationTopology: topologyOf([]string{"example-replication-0"}, nil, []string{"example-replication-1", "example-replication-2"}),
		RedisReplicationRealMaster: func(context.Context, kubernetes.Interface, *rrvb2.RedisReplication, []string) string {
			return ""
		},
	}

	result, err := r.reconcileStatus(context.Background(), instance)

	require.NoError(t, err)
	assert.Equal(t, ctrl.Result{}, result)

	updated := &rrvb2.RedisReplication{}
	require.NoError(t, ctrlClient.Get(context.Background(), client.ObjectKeyFromObject(instance), updated))
	assert.Empty(t, updated.Status.MasterNode)
}

// The rule itself, branch by branch, including which checks settle the answer
// before Sentinel has to be asked.
func TestObservedRedisReplicationMaster(t *testing.T) {
	const (
		pod0 = "example-replication-0"
		pod1 = "example-replication-1"
		pod2 = "example-replication-2"
	)
	tests := []struct {
		name           string
		sentinel       bool
		recorded       string
		topology       k8sutils.RedisReplicationTopology
		realMaster     string
		sentinelMaster string
		sentinelErr    error
		wantMaster     string
		wantConfirmed  bool
		wantAsked      bool
	}{
		{
			name:     "no master observed",
			topology: k8sutils.RedisReplicationTopology{Slaves: []string{pod1, pod2}, Unobserved: []string{pod0}},
		},
		{
			name:          "lone master in a complete topology",
			topology:      k8sutils.RedisReplicationTopology{Masters: []string{pod1}, Slaves: []string{pod0, pod2}},
			wantMaster:    pod1,
			wantConfirmed: true,
		},
		{
			name:          "lone master with attached replicas in a partial view",
			recorded:      pod0,
			topology:      k8sutils.RedisReplicationTopology{Masters: []string{pod1}, Slaves: []string{pod2}, Unobserved: []string{pod0}},
			realMaster:    pod1,
			wantMaster:    pod1,
			wantConfirmed: true,
		},
		{
			name:          "without sentinel the recorded master is still the master",
			recorded:      pod0,
			topology:      k8sutils.RedisReplicationTopology{Masters: []string{pod0}, Unobserved: []string{pod1, pod2}},
			wantMaster:    pod0,
			wantConfirmed: true,
		},
		{
			name:     "without sentinel a standalone pod that is not the recorded master is a candidate only",
			recorded: pod0,
			topology: k8sutils.RedisReplicationTopology{Masters: []string{pod1}, Slaves: []string{pod2}, Unobserved: []string{pod0}},
		},
		{
			name:     "without sentinel a standalone pod during bootstrap is a candidate only",
			topology: k8sutils.RedisReplicationTopology{Masters: []string{pod0}, Unobserved: []string{pod1, pod2}},
		},
		{
			name:          "with sentinel a complete topology is conclusive without asking",
			sentinel:      true,
			topology:      k8sutils.RedisReplicationTopology{Masters: []string{pod1}, Slaves: []string{pod0, pod2}},
			wantMaster:    pod1,
			wantConfirmed: true,
		},
		{
			name:          "with sentinel attached replicas settle it without asking",
			sentinel:      true,
			recorded:      pod0,
			topology:      k8sutils.RedisReplicationTopology{Masters: []string{pod1}, Slaves: []string{pod2}, Unobserved: []string{pod0}},
			realMaster:    pod1,
			wantMaster:    pod1,
			wantConfirmed: true,
		},
		{
			name:           "with sentinel the pod sentinel monitors is the master",
			sentinel:       true,
			recorded:       pod0,
			topology:       k8sutils.RedisReplicationTopology{Masters: []string{pod1}, Unobserved: []string{pod0}},
			sentinelMaster: pod1,
			wantMaster:     pod1,
			wantConfirmed:  true,
			wantAsked:      true,
		},
		{
			// The recorded master was replaced by a failover this controller
			// never saw and came back as an empty standalone master; being
			// recorded proves nothing under Sentinel.
			name:      "with sentinel the recorded master proves nothing on its own",
			sentinel:  true,
			recorded:  pod0,
			topology:  k8sutils.RedisReplicationTopology{Masters: []string{pod0}, Slaves: []string{pod1}, Unobserved: []string{pod2}},
			wantAsked: true,
		},
		{
			name:        "with sentinel an unanswered question leaves the candidate unconfirmed",
			sentinel:    true,
			recorded:    pod0,
			topology:    k8sutils.RedisReplicationTopology{Masters: []string{pod0}, Unobserved: []string{pod1, pod2}},
			sentinelErr: errors.New("sentinel unreachable"),
			wantAsked:   true,
		},
		{
			name:          "several masters resolve through attached replicas",
			topology:      k8sutils.RedisReplicationTopology{Masters: []string{pod0, pod1}, Slaves: []string{pod2}},
			realMaster:    pod1,
			wantMaster:    pod1,
			wantConfirmed: true,
		},
		{
			name:     "several masters without replicas are unresolved",
			recorded: pod0,
			topology: k8sutils.RedisReplicationTopology{Masters: []string{pod0, pod1, pod2}},
		},
		{
			name:          "with sentinel several masters with replicas settle without asking",
			sentinel:      true,
			topology:      k8sutils.RedisReplicationTopology{Masters: []string{pod0, pod1}, Slaves: []string{pod2}},
			realMaster:    pod1,
			wantMaster:    pod1,
			wantConfirmed: true,
		},
		{
			name:           "with sentinel several replica-less masters resolve through sentinel",
			sentinel:       true,
			recorded:       pod0,
			topology:       k8sutils.RedisReplicationTopology{Masters: []string{pod0, pod1}},
			sentinelMaster: pod1,
			wantMaster:     pod1,
			wantConfirmed:  true,
			wantAsked:      true,
		},
		{
			name:      "with sentinel several replica-less masters stay unresolved when sentinel monitors none of them",
			sentinel:  true,
			recorded:  pod0,
			topology:  k8sutils.RedisReplicationTopology{Masters: []string{pod0, pod1}, Unobserved: []string{pod2}},
			wantAsked: true,
		},
		{
			name:        "with sentinel several replica-less masters stay unresolved when sentinel cannot be asked",
			sentinel:    true,
			recorded:    pod0,
			topology:    k8sutils.RedisReplicationTopology{Masters: []string{pod0, pod1}},
			sentinelErr: errors.New("sentinel unreachable"),
			wantAsked:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sentinelAsked := false
			r := &Reconciler{
				K8sClient: fake.NewSimpleClientset(),
				RedisReplicationRealMaster: func(context.Context, kubernetes.Interface, *rrvb2.RedisReplication, []string) string {
					return tt.realMaster
				},
				SentinelMonitoredMaster: func(_ context.Context, _ *rrvb2.RedisReplication, candidates []string) (string, error) {
					sentinelAsked = true
					assert.Equal(t, tt.topology.Masters, candidates)
					return tt.sentinelMaster, tt.sentinelErr
				},
			}
			instance := newReplicationInstanceForTest()
			if tt.sentinel {
				instance = newSentinelReplicationInstanceForTest()
			}
			instance.Status.MasterNode = tt.recorded

			master, confirmed := r.observedRedisReplicationMaster(context.Background(), instance, tt.topology)

			assert.Equal(t, tt.wantMaster, master)
			assert.Equal(t, tt.wantConfirmed, confirmed)
			assert.Equal(t, master != "", confirmed, "a master is returned exactly when it is confirmed")
			assert.Equal(t, tt.wantAsked, sentinelAsked, "sentinel is asked only when completeness and attached replicas cannot settle it")
		})
	}
}

func TestQuerySentinelMonitoredMaster(t *testing.T) {
	const (
		pod0 = "example-replication-0"
		pod1 = "example-replication-1"
	)
	instance := newSentinelReplicationInstanceForTest()
	pod1Hostname := replicationPodHostname(instance, pod1)

	tests := []struct {
		name         string
		candidates   []string
		pod1IP       string
		sentinelInfo map[string]*redis.InfoSentinelResult
		sentinelErr  map[string]error
		want         string
	}{
		{
			name:       "a quorum reports the candidate by IP",
			candidates: []string{pod1},
			sentinelInfo: map[string]*redis.InfoSentinelResult{
				"10.0.1.10": sentinelInfoMonitoring("10.0.0.11:6379"),
				"10.0.1.11": sentinelInfoMonitoring("10.0.0.11:6379"),
				"10.0.1.12": sentinelInfoMonitoring("10.0.0.10:6379"),
			},
			want: pod1,
		},
		{
			name:       "a single sentinel is not a quorum",
			candidates: []string{pod1},
			sentinelInfo: map[string]*redis.InfoSentinelResult{
				"10.0.1.10": sentinelInfoMonitoring("10.0.0.11:6379"),
				"10.0.1.11": sentinelInfoMonitoring("10.0.0.10:6379"),
				"10.0.1.12": sentinelInfoMonitoring("10.0.0.10:6379"),
			},
			want: "",
		},
		{
			name:       "a quorum reports the candidate by hostname",
			candidates: []string{pod1},
			sentinelInfo: map[string]*redis.InfoSentinelResult{
				"10.0.1.10": sentinelInfoMonitoring(pod1Hostname + ":6379"),
				"10.0.1.11": sentinelInfoMonitoring(pod1Hostname + ":6379"),
				"10.0.1.12": sentinelInfoMonitoring(pod1Hostname + ":6379"),
			},
			want: pod1,
		},
		{
			// INFO sentinel prints IPv6 addresses without brackets.
			name:       "a quorum reports the candidate by unbracketed IPv6",
			candidates: []string{pod1},
			pod1IP:     "fd00::11",
			sentinelInfo: map[string]*redis.InfoSentinelResult{
				"10.0.1.10": sentinelInfoMonitoring("fd00::11:6379"),
				"10.0.1.11": sentinelInfoMonitoring("fd00::11:6379"),
				"10.0.1.12": sentinelInfoMonitoring("fd00::11:6379"),
			},
			want: pod1,
		},
		{
			name:       "another master group is ignored",
			candidates: []string{pod1},
			sentinelInfo: map[string]*redis.InfoSentinelResult{
				"10.0.1.10": {Masters: []redis.SentinelMasterInfo{{Name: "othermaster", Address: "10.0.0.11:6379"}}},
				"10.0.1.11": {Masters: []redis.SentinelMasterInfo{{Name: "othermaster", Address: "10.0.0.11:6379"}}},
				"10.0.1.12": {Masters: []redis.SentinelMasterInfo{{Name: "othermaster", Address: "10.0.0.11:6379"}}},
			},
			want: "",
		},
		{
			name:       "an unreachable sentinel is skipped and the rest can still form a quorum",
			candidates: []string{pod1},
			sentinelInfo: map[string]*redis.InfoSentinelResult{
				"10.0.1.10": sentinelInfoMonitoring("10.0.0.11:6379"),
				"10.0.1.11": sentinelInfoMonitoring("10.0.0.11:6379"),
			},
			sentinelErr: map[string]error{"10.0.1.12": errors.New("dial tcp: i/o timeout")},
			want:        pod1,
		},
		{
			name:       "among several candidates the one with a quorum is returned",
			candidates: []string{pod0, pod1},
			sentinelInfo: map[string]*redis.InfoSentinelResult{
				"10.0.1.10": sentinelInfoMonitoring("10.0.0.11:6379"),
				"10.0.1.11": sentinelInfoMonitoring("10.0.0.11:6379"),
				"10.0.1.12": sentinelInfoMonitoring("10.0.0.10:6379"),
			},
			want: pod1,
		},
		{
			name:       "a monitored pod that is not a candidate elects nobody",
			candidates: []string{pod0},
			sentinelInfo: map[string]*redis.InfoSentinelResult{
				"10.0.1.10": sentinelInfoMonitoring("10.0.0.11:6379"),
				"10.0.1.11": sentinelInfoMonitoring("10.0.0.11:6379"),
				"10.0.1.12": sentinelInfoMonitoring("10.0.0.11:6379"),
			},
			want: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pod1IP := tt.pod1IP
			if pod1IP == "" {
				pod1IP = "10.0.0.11"
			}
			objects := []runtime.Object{
				newRunningPodForTest(instance.Namespace, pod0, "10.0.0.10", nil, true),
				newRunningPodForTest(instance.Namespace, pod1, pod1IP, nil, true),
			}
			sentinelLabels := common.GetRedisLabels(instance.SentinelStatefulSet(), common.SetupTypeSentinel, "sentinel", instance.GetLabels())
			for i, ip := range []string{"10.0.1.10", "10.0.1.11", "10.0.1.12"} {
				objects = append(objects, newRunningPodForTest(instance.Namespace, fmt.Sprintf("%s-%d", instance.SentinelStatefulSet(), i), ip, sentinelLabels, true))
			}
			// A sentinel pod that is not Ready is never asked.
			objects = append(objects, newRunningPodForTest(instance.Namespace, instance.SentinelStatefulSet()+"-3", "10.0.1.13", sentinelLabels, false))

			redisClient := &fakeSentinelInfoClient{infoByHost: tt.sentinelInfo, errByHost: tt.sentinelErr}
			r := &Reconciler{K8sClient: fake.NewSimpleClientset(objects...)}

			got, err := r.querySentinelMonitoredMaster(context.Background(), redisClient, instance, tt.candidates)

			require.NoError(t, err)
			assert.Equal(t, tt.want, got)
			assert.ElementsMatch(t, []string{"10.0.1.10", "10.0.1.11", "10.0.1.12"}, redisClient.connectedHosts)
		})
	}
}

func TestQuerySentinelMonitoredMasterFailsWhenACandidateCannotBeFetched(t *testing.T) {
	r := &Reconciler{K8sClient: fake.NewSimpleClientset()}

	_, err := r.querySentinelMonitoredMaster(context.Background(), &fakeSentinelInfoClient{}, newSentinelReplicationInstanceForTest(), []string{"example-replication-1"})

	require.Error(t, err)
	assert.ErrorContains(t, err, "example-replication-1")
}

func TestSentinelAddressHost(t *testing.T) {
	tests := map[string]string{
		"10.0.0.11:6379":  "10.0.0.11",
		"fd00::11:6379":   "fd00::11",
		"[fd00::11]:6379": "fd00::11",
		"example-replication-1.example-replication-headless.default.svc.cluster.local:6379": "example-replication-1.example-replication-headless.default.svc.cluster.local",
		"10.0.0.11": "10.0.0.11",
	}
	for address, want := range tests {
		assert.Equal(t, want, sentinelAddressHost(address), address)
	}
}

func sentinelInfoMonitoring(address string) *redis.InfoSentinelResult {
	return &redis.InfoSentinelResult{
		Masters: []redis.SentinelMasterInfo{{Name: masterGroupName, Status: "ok", Address: address, Slaves: 1, Sentinels: 3}},
	}
}

// sentinelMonitoring stands in for a Sentinel quorum that monitors master, or
// nobody when master is empty; it only ever answers with one of the candidates.
func sentinelMonitoring(master string) func(context.Context, *rrvb2.RedisReplication, []string) (string, error) {
	return func(_ context.Context, _ *rrvb2.RedisReplication, candidates []string) (string, error) {
		for _, candidate := range candidates {
			if candidate == master {
				return master, nil
			}
		}
		return "", nil
	}
}

func newRunningPodForTest(namespace, name, podIP string, labels map[string]string, ready bool) *corev1.Pod {
	readyStatus := corev1.ConditionFalse
	if ready {
		readyStatus = corev1.ConditionTrue
	}
	return &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: namespace, Labels: labels},
		Status: corev1.PodStatus{
			Phase:      corev1.PodRunning,
			PodIP:      podIP,
			Conditions: []corev1.PodCondition{{Type: corev1.PodReady, Status: readyStatus}},
		},
	}
}

type fakeSentinelInfoClient struct {
	connectedHosts []string
	infoByHost     map[string]*redis.InfoSentinelResult
	errByHost      map[string]error
}

func (f *fakeSentinelInfoClient) Connect(info *redis.ConnectionInfo) redis.Service {
	f.connectedHosts = append(f.connectedHosts, info.Host)
	return &fakeSentinelInfoService{info: f.infoByHost[info.Host], err: f.errByHost[info.Host]}
}

type fakeSentinelInfoService struct {
	info *redis.InfoSentinelResult
	err  error
}

func (f *fakeSentinelInfoService) IsMaster(context.Context) (bool, error) { return false, nil }

func (f *fakeSentinelInfoService) GetAttachedReplicaCount(context.Context) (int, error) {
	return 0, nil
}

func (f *fakeSentinelInfoService) SentinelMonitor(context.Context, *redis.ConnectionInfo, string, string) error {
	return nil
}

func (f *fakeSentinelInfoService) SentinelSet(context.Context, string, string, string) error {
	return nil
}

func (f *fakeSentinelInfoService) SentinelReset(context.Context, string) error { return nil }

func (f *fakeSentinelInfoService) GetInfoSentinel(context.Context) (*redis.InfoSentinelResult, error) {
	return f.info, f.err
}

func (f *fakeSentinelInfoService) GetClusterInfo(context.Context) (*redis.ClusterStatus, error) {
	return &redis.ClusterStatus{}, nil
}

func TestReconcileStatusRequeuesWhenTopologyCannotBeObserved(t *testing.T) {
	healer := &fakeHealer{}
	r := &Reconciler{
		K8sClient: fake.NewSimpleClientset(),
		Healer:    healer,
		RedisReplicationTopology: func(context.Context, kubernetes.Interface, *rrvb2.RedisReplication) (k8sutils.RedisReplicationTopology, error) {
			return k8sutils.RedisReplicationTopology{}, context.Canceled
		},
	}

	_, err := r.reconcileStatus(context.Background(), newReplicationInstanceForTest())

	require.ErrorIs(t, err, context.Canceled)
	assert.False(t, healer.updateCalled)
}

// topologyOf stands in for a role sweep that observed the given masters and
// slaves and could not observe the remaining pods.
func topologyOf(masters, slaves, unobserved []string) func(context.Context, kubernetes.Interface, *rrvb2.RedisReplication) (k8sutils.RedisReplicationTopology, error) {
	return func(context.Context, kubernetes.Interface, *rrvb2.RedisReplication) (k8sutils.RedisReplicationTopology, error) {
		return k8sutils.RedisReplicationTopology{
			Masters:    masters,
			Slaves:     slaves,
			Unobserved: unobserved,
		}, nil
	}
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

type fakeHealer struct {
	updateCalled bool
	master       string
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

func (f *fakeHealer) UpdateRedisRoleLabel(_ context.Context, _ string, _ map[string]string, _ *commonapi.ExistingPasswordSecret, _ *commonapi.TLSConfig, masterPod string) error {
	f.updateCalled = true
	f.master = masterPod
	return nil
}
