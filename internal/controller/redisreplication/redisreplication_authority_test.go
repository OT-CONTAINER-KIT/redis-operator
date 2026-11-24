package redisreplication

import (
	"context"
	"errors"
	"testing"
	"time"

	rrvb2 "github.com/OT-CONTAINER-KIT/redis-operator/api/redisreplication/v1beta2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/fake"
)

// emptyMasterReconciler is the state of #1938: the master came back empty
// with a new IP, the replicas still hold the data on a dead link, and the
// Sentinels restarted without state.
type emptyMasterRun struct {
	promoted   string
	linkMaster string
	linkPods   []string
}

func emptyMasterReconciler(run *emptyMasterRun, keyCounts map[string]int64, sentinelCanDecide bool) *Reconciler {
	return &Reconciler{
		Client:                   emptyCtrlClient(),
		StatefulSet:              &fakeStatefulSetService{},
		K8sClient:                fake.NewSimpleClientset(),
		RedisReplicationTopology: topologyOf([]string{"example-replication-0"}, []string{"example-replication-1", "example-replication-2"}, nil),
		RedisReplicationRealMaster: func(context.Context, kubernetes.Interface, *rrvb2.RedisReplication, []string) string {
			return ""
		},
		Checker: keyCountsOf(keyCounts),
		CreateRedisReplicationLink: func(_ context.Context, _ kubernetes.Interface, _ *rrvb2.RedisReplication, pods []string, realMaster string) error {
			run.linkMaster = realMaster
			run.linkPods = append([]string{}, pods...)
			return nil
		},
		RedisReplicationBestMaster: func(_ context.Context, _ kubernetes.Interface, _ *rrvb2.RedisReplication, pods []string) string {
			return pods[len(pods)-1]
		},
		PromoteRedisReplicationMaster: func(_ context.Context, _ kubernetes.Interface, _ *rrvb2.RedisReplication, pod string) error {
			run.promoted = pod
			return nil
		},
		SentinelMonitoredMaster:    sentinelMonitoring(""),
		SentinelFailoverInProgress: func(context.Context, *rrvb2.RedisReplication) (bool, error) { return false, nil },
		SentinelCanDecide:          func(context.Context, *rrvb2.RedisReplication) (bool, error) { return sentinelCanDecide, nil },
	}
}

func TestReconcileRedisPromotesADataHoldingReplicaWhenSentinelCannotDecide(t *testing.T) {
	run := &emptyMasterRun{}
	r := emptyMasterReconciler(run, map[string]int64{"example-replication-0": 0, "example-replication-1": 1000, "example-replication-2": 1000}, false)

	_, err := r.reconcileRedis(context.Background(), newReplicationInstanceForTest())

	require.NoError(t, err)
	assert.Equal(t, "example-replication-2", run.promoted)
	assert.Equal(t, "example-replication-2", run.linkMaster)
	assert.ElementsMatch(t, []string{"example-replication-0", "example-replication-1", "example-replication-2"}, run.linkPods)
}

// Configured Sentinels fail over to a data-holding replica on their own after
// down-after-milliseconds: promoting one here would race them.
func TestReconcileRedisLeavesThePromotionToSentinelWhenItCanDecide(t *testing.T) {
	run := &emptyMasterRun{}
	r := emptyMasterReconciler(run, map[string]int64{"example-replication-0": 0, "example-replication-1": 1000, "example-replication-2": 1000}, true)

	_, err := r.reconcileRedis(context.Background(), newReplicationInstanceForTest())

	require.NoError(t, err)
	assert.Empty(t, run.promoted)
	assert.Empty(t, run.linkMaster)
}

func TestReconcileRedisDoesNotPromoteWhenAKeyCountIsUnknown(t *testing.T) {
	run := &emptyMasterRun{}
	r := emptyMasterReconciler(run, map[string]int64{"example-replication-0": 0, "example-replication-2": 1000}, false)

	_, err := r.reconcileRedis(context.Background(), newReplicationInstanceForTest())

	require.NoError(t, err)
	assert.Empty(t, run.promoted)
	assert.Empty(t, run.linkMaster)
}

func TestReconcileRedisDoesNotPromoteWhenSentinelCannotBeAsked(t *testing.T) {
	run := &emptyMasterRun{}
	r := emptyMasterReconciler(run, map[string]int64{"example-replication-0": 0, "example-replication-1": 1000, "example-replication-2": 1000}, false)
	r.SentinelCanDecide = func(context.Context, *rrvb2.RedisReplication) (bool, error) {
		return false, errors.New("dial tcp: i/o timeout")
	}

	_, err := r.reconcileRedis(context.Background(), newReplicationInstanceForTest())

	require.NoError(t, err)
	assert.Empty(t, run.promoted)
}

// A replica that follows a dead IP or another replica is invisible to
// Sentinel (#1779): re-point it while the master has other replicas attached.
func TestReconcileRedisRepointsReplicasThatDoNotFollowTheMaster(t *testing.T) {
	var linkMaster string
	var linkPods []string
	checker := &fakeChecker{
		keyCounts:   map[string]int64{"example-replication-0": 100, "example-replication-1": 100, "example-replication-2": 100},
		misattached: []string{"example-replication-2"},
	}
	r := &Reconciler{
		Client:                   emptyCtrlClient(),
		StatefulSet:              &fakeStatefulSetService{},
		K8sClient:                fake.NewSimpleClientset(),
		RedisReplicationTopology: topologyOf([]string{"example-replication-0"}, []string{"example-replication-1", "example-replication-2"}, nil),
		RedisReplicationRealMaster: func(context.Context, kubernetes.Interface, *rrvb2.RedisReplication, []string) string {
			return "example-replication-0"
		},
		Checker: checker,
		CreateRedisReplicationLink: func(_ context.Context, _ kubernetes.Interface, _ *rrvb2.RedisReplication, pods []string, realMaster string) error {
			linkMaster = realMaster
			linkPods = append([]string{}, pods...)
			return nil
		},
		SentinelMonitoredMaster:    sentinelMonitoring(""),
		SentinelFailoverInProgress: func(context.Context, *rrvb2.RedisReplication) (bool, error) { return false, nil },
	}

	_, err := r.reconcileRedis(context.Background(), newReplicationInstanceForTest())

	require.NoError(t, err)
	assert.Equal(t, "example-replication-0", linkMaster)
	assert.ElementsMatch(t, []string{"example-replication-0", "example-replication-2"}, linkPods)
}

// A Sentinel counts as configured when it monitors a real address, neither
// the 0.0.0.0 bootstrap placeholder nor its own IP (#1774).
func TestQuerySentinelCanDecideNeedsAQuorumOfConfiguredSentinels(t *testing.T) {
	rs := newStandaloneSentinelForTest("redis-sentinel", "example-replication")
	tests := map[string]struct {
		ipByHost map[string]string
		want     bool
	}{
		"restarted without state": {
			ipByHost: map[string]string{"10.0.1.10": "0.0.0.0", "10.0.1.11": "0.0.0.0", "10.0.1.12": "10.0.0.10"},
			want:     false,
		},
		"monitoring themselves": {
			ipByHost: map[string]string{"10.0.1.10": "10.0.1.10", "10.0.1.11": "0.0.0.0", "10.0.1.12": "10.0.0.10"},
			want:     false,
		},
		"configured, even on a dead master IP": {
			ipByHost: map[string]string{"10.0.1.10": "10.0.9.99", "10.0.1.11": "10.0.9.99", "10.0.1.12": "0.0.0.0"},
			want:     true,
		},
	}
	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			r := &Reconciler{
				Client:    ctrlClientWith(t, rs),
				K8sClient: fake.NewSimpleClientset(standaloneSentinelObjects(rs)...),
			}

			got, err := r.querySentinelCanDecide(context.Background(), &fakeSentinelInfoClient{ipByHost: tt.ipByHost}, newReplicationInstanceForTest())

			require.NoError(t, err)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestQuerySentinelCanDecideIsFalseWithoutLinkedSentinel(t *testing.T) {
	r := &Reconciler{Client: emptyCtrlClient(), K8sClient: fake.NewSimpleClientset()}

	got, err := r.querySentinelCanDecide(context.Background(), &fakeSentinelInfoClient{}, newReplicationInstanceForTest())

	require.NoError(t, err)
	assert.False(t, got)
}

// Configured Sentinels that cannot fail over (#1883: no replica left in their
// view) would leave the set without a master for good: once they had
// down-after-milliseconds + failover-timeout to do it, the operator takes over.
func TestReconcileRedisPromotesOnceSentinelHadTimeToFailOver(t *testing.T) {
	run := &emptyMasterRun{}
	r := emptyMasterReconciler(run, map[string]int64{"example-replication-0": 0, "example-replication-1": 1000, "example-replication-2": 1000}, true)
	now := time.Date(2026, 11, 24, 12, 0, 0, 0, time.UTC)
	r.Now = func() time.Time { return now }
	instance := newReplicationInstanceForTest()

	_, err := r.reconcileRedis(context.Background(), instance)
	require.NoError(t, err)
	require.Empty(t, run.promoted, "Sentinel gets its chance first")

	now = now.Add(sentinelFailoverGrace + time.Second)
	_, err = r.reconcileRedis(context.Background(), instance)

	require.NoError(t, err)
	assert.Equal(t, "example-replication-2", run.promoted)
}
