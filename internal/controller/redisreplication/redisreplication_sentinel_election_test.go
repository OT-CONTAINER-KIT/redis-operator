package redisreplication

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	commonapi "github.com/OT-CONTAINER-KIT/redis-operator/api/common/v1beta2"
	rrvb2 "github.com/OT-CONTAINER-KIT/redis-operator/api/redisreplication/v1beta2"
	rsvb2 "github.com/OT-CONTAINER-KIT/redis-operator/api/redissentinel/v1beta2"
	"github.com/OT-CONTAINER-KIT/redis-operator/internal/service/redis"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	appsv1 "k8s.io/api/apps/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/fake"
	"k8s.io/utils/ptr"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	clientfake "sigs.k8s.io/controller-runtime/pkg/client/fake"
)

// Right after a Sentinel failover, the promoted pod is a master with no replica
// yet, while the old master can still report role:master with replicas
// attached (for example when it was paused with CLIENT PAUSE before a
// SENTINEL FAILOVER). The operator must follow Sentinel instead of the old
// master, otherwise it undoes the failover with SLAVEOF.
func failoverInProgressReconciler(sentinel func(context.Context, *rrvb2.RedisReplication, []string) (string, error), gotMaster *string) *Reconciler {
	return &Reconciler{
		Client:                   emptyCtrlClient(),
		StatefulSet:              &fakeStatefulSetService{},
		K8sClient:                fake.NewSimpleClientset(),
		RedisReplicationTopology: topologyOf([]string{"example-replication-0", "example-replication-2"}, []string{"example-replication-1"}, nil),
		// The old master example-replication-2 still has example-replication-1 attached.
		RedisReplicationRealMaster: func(context.Context, kubernetes.Interface, *rrvb2.RedisReplication, []string) string {
			return "example-replication-2"
		},
		Checker: keyCountsOf(map[string]int64{"example-replication-0": 100, "example-replication-1": 100, "example-replication-2": 100}),
		CreateRedisReplicationLink: func(_ context.Context, _ kubernetes.Interface, _ *rrvb2.RedisReplication, _ []string, realMaster string) error {
			*gotMaster = realMaster
			return nil
		},
		ConfigureSentinel: func(context.Context, *rrvb2.RedisReplication, string) error {
			return nil
		},
		SentinelMonitoredMaster: sentinel,
	}
}

func TestReconcileRedisFollowsTheMasterElectedByAStandaloneSentinel(t *testing.T) {
	var gotMaster string
	r := failoverInProgressReconciler(sentinelMonitoring("example-replication-0"), &gotMaster)

	result, err := r.reconcileRedis(context.Background(), newReplicationInstanceForTest())

	require.NoError(t, err)
	assert.Equal(t, ctrl.Result{}, result)
	assert.Equal(t, "example-replication-0", gotMaster)
}

func TestReconcileRedisFollowsTheMasterElectedByAnEmbeddedSentinel(t *testing.T) {
	var gotMaster string
	r := failoverInProgressReconciler(sentinelMonitoring("example-replication-0"), &gotMaster)

	result, err := r.reconcileRedis(context.Background(), newSentinelReplicationInstanceForTest())

	require.NoError(t, err)
	assert.Equal(t, ctrl.Result{}, result)
	assert.Equal(t, "example-replication-0", gotMaster)
}

func TestReconcileRedisKeepsTheMasterWithReplicasWhenSentinelHasNoMajority(t *testing.T) {
	var gotMaster string
	r := failoverInProgressReconciler(sentinelMonitoring(""), &gotMaster)

	_, err := r.reconcileRedis(context.Background(), newReplicationInstanceForTest())

	require.NoError(t, err)
	assert.Equal(t, "example-replication-2", gotMaster)
}

func TestReconcileRedisKeepsTheMasterWithReplicasWhenSentinelCannotBeAsked(t *testing.T) {
	var gotMaster string
	r := failoverInProgressReconciler(func(context.Context, *rrvb2.RedisReplication, []string) (string, error) {
		return "", errors.New("dial tcp: i/o timeout")
	}, &gotMaster)

	_, err := r.reconcileRedis(context.Background(), newReplicationInstanceForTest())

	require.NoError(t, err)
	assert.Equal(t, "example-replication-2", gotMaster)
}

func newStandaloneSentinelForTest(name, replication string) *rsvb2.RedisSentinel {
	return &rsvb2.RedisSentinel{
		ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: "default"},
		Spec: rsvb2.RedisSentinelSpec{
			Size: ptr.To(int32(3)),
			RedisSentinelConfig: &rsvb2.RedisSentinelConfig{
				RedisSentinelConfig: commonapi.RedisSentinelConfig{
					RedisReplicationName: replication,
					MasterGroupName:      "default-master",
				},
			},
		},
	}
}

// standaloneSentinelObjects returns the two replication pods and the three
// sentinel pods of rs, selected by the labels of the sentinel StatefulSet.
func standaloneSentinelObjects(rs *rsvb2.RedisSentinel) []runtime.Object {
	selector := map[string]string{"app": rs.GetStatefulSetName(), "role": "sentinel"}
	objects := []runtime.Object{
		newRunningPodForTest("default", "example-replication-0", "10.0.0.10", nil, true),
		newRunningPodForTest("default", "example-replication-2", "10.0.0.12", nil, true),
		&appsv1.StatefulSet{
			ObjectMeta: metav1.ObjectMeta{Name: rs.GetStatefulSetName(), Namespace: "default"},
			Spec:       appsv1.StatefulSetSpec{Selector: &metav1.LabelSelector{MatchLabels: selector}},
		},
	}
	for i, ip := range []string{"10.0.1.10", "10.0.1.11", "10.0.1.12"} {
		objects = append(objects, newRunningPodForTest("default", fmt.Sprintf("%s-%d", rs.GetStatefulSetName(), i), ip, selector, true))
	}
	return objects
}

func ctrlClientWith(t *testing.T, objects ...client.Object) client.Client {
	t.Helper()
	scheme := runtime.NewScheme()
	require.NoError(t, rrvb2.AddToScheme(scheme))
	require.NoError(t, rsvb2.AddToScheme(scheme))
	return clientfake.NewClientBuilder().WithScheme(scheme).WithObjects(objects...).Build()
}

func standaloneSentinelInfo(masterName, address string) *redis.InfoSentinelResult {
	return &redis.InfoSentinelResult{Masters: []redis.SentinelMasterInfo{{Name: masterName, Status: "ok", Address: address, Slaves: 1, Sentinels: 3}}}
}

func TestQuerySentinelMonitoredMasterAsksTheLinkedStandaloneSentinel(t *testing.T) {
	rs := newStandaloneSentinelForTest("redis-sentinel", "example-replication")
	redisClient := &fakeSentinelInfoClient{infoByHost: map[string]*redis.InfoSentinelResult{
		"10.0.1.10": standaloneSentinelInfo("default-master", "10.0.0.10:6379"),
		"10.0.1.11": standaloneSentinelInfo("default-master", "10.0.0.10:6379"),
		"10.0.1.12": standaloneSentinelInfo("default-master", "10.0.0.12:6379"),
	}}
	r := &Reconciler{
		Client:    ctrlClientWith(t, rs),
		K8sClient: fake.NewSimpleClientset(standaloneSentinelObjects(rs)...),
	}

	got, err := r.querySentinelMonitoredMaster(context.Background(), redisClient, newReplicationInstanceForTest(), []string{"example-replication-0", "example-replication-2"})

	require.NoError(t, err)
	assert.Equal(t, "example-replication-0", got)
	assert.ElementsMatch(t, []string{"10.0.1.10", "10.0.1.11", "10.0.1.12"}, redisClient.connectedHosts)
}

func TestQuerySentinelMonitoredMasterUsesTheMasterGroupOfTheStandaloneSentinel(t *testing.T) {
	rs := newStandaloneSentinelForTest("redis-sentinel", "example-replication")
	redisClient := &fakeSentinelInfoClient{infoByHost: map[string]*redis.InfoSentinelResult{
		"10.0.1.10": standaloneSentinelInfo(masterGroupName, "10.0.0.10:6379"),
		"10.0.1.11": standaloneSentinelInfo(masterGroupName, "10.0.0.10:6379"),
		"10.0.1.12": standaloneSentinelInfo(masterGroupName, "10.0.0.10:6379"),
	}}
	r := &Reconciler{
		Client:    ctrlClientWith(t, rs),
		K8sClient: fake.NewSimpleClientset(standaloneSentinelObjects(rs)...),
	}

	got, err := r.querySentinelMonitoredMaster(context.Background(), redisClient, newReplicationInstanceForTest(), []string{"example-replication-0", "example-replication-2"})

	require.NoError(t, err)
	assert.Empty(t, got, "votes for another master group must not count")
}

func TestQuerySentinelMonitoredMasterIgnoresSentinelsOfAnotherReplication(t *testing.T) {
	rs := newStandaloneSentinelForTest("other-sentinel", "other-replication")
	redisClient := &fakeSentinelInfoClient{}
	r := &Reconciler{
		Client:    ctrlClientWith(t, rs),
		K8sClient: fake.NewSimpleClientset(standaloneSentinelObjects(rs)...),
	}

	got, err := r.querySentinelMonitoredMaster(context.Background(), redisClient, newReplicationInstanceForTest(), []string{"example-replication-0", "example-replication-2"})

	require.NoError(t, err)
	assert.Empty(t, got)
	assert.Empty(t, redisClient.connectedHosts, "no sentinel is linked, none must be asked")
}

// emptyCtrlClient is the API client of a namespace without RedisSentinel:
// several observed masters make the controller look for a linked Sentinel.
func emptyCtrlClient() client.Client {
	scheme := runtime.NewScheme()
	_ = rrvb2.AddToScheme(scheme)
	_ = rsvb2.AddToScheme(scheme)
	return clientfake.NewClientBuilder().WithScheme(scheme).Build()
}

// Between SLAVEOF NO ONE and +switch-master the Sentinels still name the old
// master while the promoted pod already reports role:master: any SLAVEOF or
// SENTINEL command sent by the operator then undoes or aborts the failover.
func TestReconcileRedisLeavesTheReplicationAloneDuringASentinelFailover(t *testing.T) {
	var gotMaster string
	sentinelConfigured := false
	r := failoverInProgressReconciler(sentinelMonitoring("example-replication-2"), &gotMaster)
	r.ConfigureSentinel = func(context.Context, *rrvb2.RedisReplication, string) error {
		sentinelConfigured = true
		return nil
	}
	r.SentinelFailoverInProgress = func(context.Context, *rrvb2.RedisReplication) (bool, error) { return true, nil }

	result, err := r.reconcileRedis(context.Background(), newSentinelReplicationInstanceForTest())

	require.NoError(t, err)
	assert.Equal(t, 5*time.Second, result.RequeueAfter)
	assert.Empty(t, gotMaster, "no replication link must be created during a failover")
	assert.False(t, sentinelConfigured, "sentinel must not be reconfigured during a failover")
}

func TestReconcileRedisCarriesOnWhenSentinelCannotTellAboutAFailover(t *testing.T) {
	var gotMaster string
	r := failoverInProgressReconciler(sentinelMonitoring("example-replication-0"), &gotMaster)
	r.SentinelFailoverInProgress = func(context.Context, *rrvb2.RedisReplication) (bool, error) {
		return false, errors.New("dial tcp: i/o timeout")
	}

	_, err := r.reconcileRedis(context.Background(), newReplicationInstanceForTest())

	require.NoError(t, err)
	assert.Equal(t, "example-replication-0", gotMaster)
}

// A standalone Sentinel reconfigures the replicas itself, like an embedded one:
// re-pointing "disconnected" replicas while it fails over chains them to the
// old master.
func TestReconcileRedisLeavesDisconnectedReplicasToALinkedStandaloneSentinel(t *testing.T) {
	createCalled := false
	rs := newStandaloneSentinelForTest("redis-sentinel", "example-replication")
	r := &Reconciler{
		Client:                   ctrlClientWith(t, rs),
		StatefulSet:              &fakeStatefulSetService{},
		K8sClient:                fake.NewSimpleClientset(),
		RedisReplicationTopology: topologyOf([]string{"example-replication-0"}, []string{"example-replication-1", "example-replication-2"}, nil),
		RedisReplicationRealMaster: func(context.Context, kubernetes.Interface, *rrvb2.RedisReplication, []string) string {
			return ""
		},
		Checker: keyCountsOf(map[string]int64{"example-replication-0": 100, "example-replication-1": 100, "example-replication-2": 100}),
		CreateRedisReplicationLink: func(context.Context, kubernetes.Interface, *rrvb2.RedisReplication, []string, string) error {
			createCalled = true
			return nil
		},
		SentinelMonitoredMaster:    sentinelMonitoring(""),
		SentinelFailoverInProgress: func(context.Context, *rrvb2.RedisReplication) (bool, error) { return false, nil },
	}

	_, err := r.reconcileRedis(context.Background(), newReplicationInstanceForTest())

	require.NoError(t, err)
	assert.False(t, createCalled)
}

func TestQuerySentinelFailoverInProgressAsksTheLinkedStandaloneSentinel(t *testing.T) {
	rs := newStandaloneSentinelForTest("redis-sentinel", "example-replication")
	redisClient := &fakeSentinelInfoClient{flagsByHost: map[string]string{
		"10.0.1.10": "master",
		"10.0.1.11": "master,failover_in_progress",
		"10.0.1.12": "master",
	}}
	r := &Reconciler{
		Client:    ctrlClientWith(t, rs),
		K8sClient: fake.NewSimpleClientset(standaloneSentinelObjects(rs)...),
	}

	got, err := r.querySentinelFailoverInProgress(context.Background(), redisClient, newReplicationInstanceForTest())

	require.NoError(t, err)
	assert.True(t, got)
}

func TestQuerySentinelFailoverInProgressIsFalseWithoutLinkedSentinel(t *testing.T) {
	redisClient := &fakeSentinelInfoClient{}
	r := &Reconciler{Client: emptyCtrlClient(), K8sClient: fake.NewSimpleClientset()}

	got, err := r.querySentinelFailoverInProgress(context.Background(), redisClient, newReplicationInstanceForTest())

	require.NoError(t, err)
	assert.False(t, got)
	assert.Empty(t, redisClient.connectedHosts)
}
