package redisreplication

import (
	"context"
	"errors"
	"testing"
	"time"

	commonapi "github.com/OT-CONTAINER-KIT/redis-operator/api/common/v1beta2"
	rrvb2 "github.com/OT-CONTAINER-KIT/redis-operator/api/redisreplication/v1beta2"
	rsvb2 "github.com/OT-CONTAINER-KIT/redis-operator/api/redissentinel/v1beta2"
	redischecker "github.com/OT-CONTAINER-KIT/redis-operator/internal/controller/common/redis"
	"github.com/OT-CONTAINER-KIT/redis-operator/internal/k8sutils"
	"github.com/OT-CONTAINER-KIT/redis-operator/internal/service/redis"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/fake"
	"k8s.io/utils/ptr"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	clientfake "sigs.k8s.io/controller-runtime/pkg/client/fake"
)

const (
	pod0 = "example-replication-0"
	pod1 = "example-replication-1"
	pod2 = "example-replication-2"
)

// The master restarted empty while its Sentinels lost their state too: the
// replicas still hold the dataset but nobody will fail over to them.
func TestReconcileRedisPromotesTheBestDataHoldingSlaveWhenNoSentinelCanFailOver(t *testing.T) {
	ctrlClient, instance := newStatusClientForTest(t)
	links := &linkRecorder{}
	promotions := &promotionRecorder{}
	r := &Reconciler{
		Client:                     ctrlClient,
		K8sClient:                  fake.NewSimpleClientset(),
		RedisReplicationTopology:   topologyOf([]string{pod2}, []string{pod0, pod1}, nil),
		RedisReplicationRealMaster: noRealMaster,
		Checker:                    keyCountsOf(map[string]int64{pod0: 120, pod1: 120, pod2: 0}),
		CreateRedisReplicationLink: links.link,
		SentinelFailover:           noSentinelFailover,
		ReplicationStates: replicationStatesOf(map[string]k8sutils.RedisReplicationState{
			pod0: {Role: "slave", ReplID: "lost-master", Offset: 400},
			pod1: {Role: "slave", ReplID: "lost-master", Offset: 500},
		}),
		PromoteReplicationPod: promotions.promote,
	}

	result, err := r.reconcileRedis(context.Background(), instance)

	require.NoError(t, err)
	assert.Equal(t, promotedMasterRequeue, result.RequeueAfter)
	assert.Equal(t, []string{pod1}, promotions.pods)
	assert.Equal(t, []replicationLink{
		{pods: []string{pod1, pod0}, master: pod1},
		{pods: []string{pod1, pod2}, master: pod1},
	}, links.links)
	assert.Equal(t, pod1, recordedMaster(t, ctrlClient, instance))
}

// The empty pod already has a replica of its own, which then holds no data
// either; only the second master still holds the dataset.
func TestReconcileRedisPromotesTheDataHoldingMasterWhenNoSentinelCanFailOver(t *testing.T) {
	ctrlClient, instance := newStatusClientForTest(t)
	links := &linkRecorder{}
	promotions := &promotionRecorder{}
	r := &Reconciler{
		Client:                   ctrlClient,
		K8sClient:                fake.NewSimpleClientset(),
		RedisReplicationTopology: topologyOf([]string{pod0, pod2}, []string{pod1}, nil),
		RedisReplicationRealMaster: func(context.Context, kubernetes.Interface, *rrvb2.RedisReplication, []string) string {
			return pod0
		},
		Checker:                    keyCountsOf(map[string]int64{pod0: 0, pod1: 0, pod2: 120}),
		CreateRedisReplicationLink: links.link,
		SentinelFailover:           noSentinelFailover,
		ReplicationStates: replicationStatesOf(map[string]k8sutils.RedisReplicationState{
			pod2: {Role: "master", ReplID: "standalone", Offset: 900},
		}),
		PromoteReplicationPod: promotions.promote,
	}

	result, err := r.reconcileRedis(context.Background(), instance)

	require.NoError(t, err)
	assert.Equal(t, promotedMasterRequeue, result.RequeueAfter)
	assert.Equal(t, []string{pod2}, promotions.pods)
	assert.Equal(t, []replicationLink{
		{pods: []string{pod2, pod0}, master: pod2},
		{pods: []string{pod2, pod1}, master: pod2},
	}, links.links)
	assert.Equal(t, pod2, recordedMaster(t, ctrlClient, instance))
}

func TestReconcileRedisPromotesAfterTheSentinelFailoverWindowExpires(t *testing.T) {
	ctrlClient, instance := newStatusClientForTest(t)
	now := time.Date(2026, time.October, 12, 9, 0, 0, 0, time.UTC)
	promotions := &promotionRecorder{}
	r := &Reconciler{
		Client:                     ctrlClient,
		K8sClient:                  fake.NewSimpleClientset(),
		RedisReplicationTopology:   topologyOf([]string{pod2}, []string{pod0, pod1}, nil),
		RedisReplicationRealMaster: noRealMaster,
		Checker:                    keyCountsOf(map[string]int64{pod0: 120, pod1: 120, pod2: 0}),
		CreateRedisReplicationLink: (&linkRecorder{}).link,
		SentinelFailover:           sentinelFailoverExpected(70 * time.Second),
		ReplicationStates: replicationStatesOf(map[string]k8sutils.RedisReplicationState{
			pod0: {Role: "slave", ReplID: "lost-master", Offset: 500},
			pod1: {Role: "slave", ReplID: "lost-master", Offset: 500},
		}),
		PromoteReplicationPod: promotions.promote,
		Now:                   func() time.Time { return now },
	}

	result, err := r.reconcileRedis(context.Background(), instance)
	require.NoError(t, err)
	assert.Equal(t, 60*time.Second, result.RequeueAfter)
	assert.Empty(t, promotions.pods)

	now = now.Add(60 * time.Second)
	result, err = r.reconcileRedis(context.Background(), instance)
	require.NoError(t, err)
	assert.Equal(t, 10*time.Second, result.RequeueAfter, "the requeue is cut to what is left of the window")
	assert.Empty(t, promotions.pods)

	now = now.Add(10 * time.Second)
	result, err = r.reconcileRedis(context.Background(), instance)
	require.NoError(t, err)
	assert.Equal(t, promotedMasterRequeue, result.RequeueAfter)
	assert.Equal(t, []string{pod0}, promotions.pods)
}

func TestReconcileRedisRestartsTheSentinelFailoverWindowWhenTheEmptyMasterIsRecreated(t *testing.T) {
	ctrlClient, instance := newStatusClientForTest(t)
	now := time.Date(2026, time.October, 12, 9, 0, 0, 0, time.UTC)
	k8sClient := fake.NewSimpleClientset(&corev1.Pod{ObjectMeta: metav1.ObjectMeta{Name: pod2, Namespace: "default", UID: "first"}})
	r := &Reconciler{
		Client:                     ctrlClient,
		K8sClient:                  k8sClient,
		RedisReplicationTopology:   topologyOf([]string{pod2}, []string{pod0, pod1}, nil),
		RedisReplicationRealMaster: noRealMaster,
		Checker:                    keyCountsOf(map[string]int64{pod0: 120, pod1: 120, pod2: 0}),
		CreateRedisReplicationLink: (&linkRecorder{}).link,
		SentinelFailover:           sentinelFailoverExpected(70 * time.Second),
		ReplicationStates:          replicationStatesOf(lostMasterReplicas),
		PromoteReplicationPod:      promotionForbidden(t),
		Now:                        func() time.Time { return now },
	}

	_, err := r.reconcileRedis(context.Background(), instance)
	require.NoError(t, err)

	now = now.Add(60 * time.Second)
	require.NoError(t, k8sClient.CoreV1().Pods("default").Delete(context.Background(), pod2, metav1.DeleteOptions{}))
	_, err = k8sClient.CoreV1().Pods("default").Create(context.Background(),
		&corev1.Pod{ObjectMeta: metav1.ObjectMeta{Name: pod2, Namespace: "default", UID: "second"}}, metav1.CreateOptions{})
	require.NoError(t, err)

	now = now.Add(10 * time.Second)
	result, err := r.reconcileRedis(context.Background(), instance)
	require.NoError(t, err)
	assert.Equal(t, 60*time.Second, result.RequeueAfter)
}

func TestReconcileRedisRestartsTheSentinelFailoverWindowAfterAPassWithoutRefusal(t *testing.T) {
	ctrlClient, instance := newStatusClientForTest(t)
	now := time.Date(2026, time.October, 12, 9, 0, 0, 0, time.UTC)
	topology := topologyOf([]string{pod2}, []string{pod0, pod1}, nil)
	r := &Reconciler{
		Client:    ctrlClient,
		K8sClient: fake.NewSimpleClientset(),
		RedisReplicationTopology: func(ctx context.Context, cl kubernetes.Interface, rr *rrvb2.RedisReplication) (k8sutils.RedisReplicationTopology, error) {
			return topology(ctx, cl, rr)
		},
		RedisReplicationRealMaster: noRealMaster,
		Checker:                    keyCountsOf(map[string]int64{pod0: 120, pod1: 120, pod2: 0}),
		CreateRedisReplicationLink: (&linkRecorder{}).link,
		SentinelFailover:           sentinelFailoverExpected(70 * time.Second),
		ReplicationStates:          replicationStatesOf(lostMasterReplicas),
		PromoteReplicationPod:      promotionForbidden(t),
		Now:                        func() time.Time { return now },
	}

	_, err := r.reconcileRedis(context.Background(), instance)
	require.NoError(t, err)

	now = now.Add(50 * time.Second)
	topology = topologyOf([]string{pod2}, []string{pod0}, []string{pod1})
	_, err = r.reconcileRedis(context.Background(), instance)
	require.NoError(t, err)

	now = now.Add(30 * time.Second)
	topology = topologyOf([]string{pod2}, []string{pod0, pod1}, nil)
	result, err := r.reconcileRedis(context.Background(), instance)
	require.NoError(t, err)
	assert.Equal(t, 60*time.Second, result.RequeueAfter)
}

func TestReconcileRedisKeepsRefusingWhenTheCandidateCannotBePromoted(t *testing.T) {
	ctrlClient, instance := newStatusClientForTest(t)
	links := &linkRecorder{}
	r := &Reconciler{
		Client:                     ctrlClient,
		K8sClient:                  fake.NewSimpleClientset(),
		RedisReplicationTopology:   topologyOf([]string{pod2}, []string{pod0, pod1}, nil),
		RedisReplicationRealMaster: noRealMaster,
		Checker:                    keyCountsOf(map[string]int64{pod0: 120, pod1: 120, pod2: 0}),
		CreateRedisReplicationLink: links.link,
		SentinelFailover:           noSentinelFailover,
		ReplicationStates: replicationStatesOf(map[string]k8sutils.RedisReplicationState{
			pod0: {Role: "slave", ReplID: "lost-master", Offset: 500},
			pod1: {Role: "slave", ReplID: "lost-master", Offset: 500},
		}),
		PromoteReplicationPod: func(context.Context, kubernetes.Interface, *rrvb2.RedisReplication, string) error {
			return errors.New("i/o timeout")
		},
	}

	result, err := r.reconcileRedis(context.Background(), instance)

	require.NoError(t, err)
	assert.Equal(t, 60*time.Second, result.RequeueAfter)
	assert.Empty(t, links.links)
	assert.Empty(t, recordedMaster(t, ctrlClient, instance))
}

func TestReconcileRedisRefusesToPromoteWhenAReplicationStateIsUnknown(t *testing.T) {
	links := &linkRecorder{}
	r := &Reconciler{
		K8sClient:                  fake.NewSimpleClientset(),
		RedisReplicationTopology:   topologyOf([]string{pod2}, []string{pod0, pod1}, nil),
		RedisReplicationRealMaster: noRealMaster,
		Checker:                    keyCountsOf(map[string]int64{pod0: 120, pod1: 120, pod2: 0}),
		CreateRedisReplicationLink: links.link,
		SentinelFailover:           noSentinelFailover,
		ReplicationStates: replicationStatesOf(map[string]k8sutils.RedisReplicationState{
			pod1: {Role: "slave", ReplID: "lost-master", Offset: 500},
		}),
		PromoteReplicationPod: promotionForbidden(t),
	}

	result, err := r.reconcileRedis(context.Background(), newReplicationInstanceForTest())

	require.NoError(t, err)
	assert.Equal(t, 60*time.Second, result.RequeueAfter)
	assert.Empty(t, links.links)
}

// The refusal on several masters only counts their keys; the replica of the
// empty master must be counted too before anything is promoted.
func TestReconcileRedisRefusesToPromoteWhenTheKeyCountOfASlaveIsUnknown(t *testing.T) {
	r := &Reconciler{
		K8sClient:                fake.NewSimpleClientset(),
		RedisReplicationTopology: topologyOf([]string{pod0, pod2}, []string{pod1}, nil),
		RedisReplicationRealMaster: func(context.Context, kubernetes.Interface, *rrvb2.RedisReplication, []string) string {
			return pod0
		},
		Checker:                    keyCountsOf(map[string]int64{pod0: 0, pod2: 120}),
		CreateRedisReplicationLink: (&linkRecorder{}).link,
		SentinelFailover:           noSentinelFailover,
		ReplicationStates: replicationStatesOf(map[string]k8sutils.RedisReplicationState{
			pod2: {Role: "master", ReplID: "standalone", Offset: 900},
		}),
		PromoteReplicationPod: promotionForbidden(t),
	}

	result, err := r.reconcileRedis(context.Background(), newReplicationInstanceForTest())

	require.NoError(t, err)
	assert.Equal(t, 60*time.Second, result.RequeueAfter)
}

// An inline spec.sentinel elects masters through its own vote; the empty
// master guard stays a plain refusal there.
func TestReconcileRedisDoesNotPromoteWithAnInlineSentinel(t *testing.T) {
	r := &Reconciler{
		StatefulSet:              &fakeStatefulSetService{},
		K8sClient:                fake.NewSimpleClientset(),
		RedisReplicationTopology: topologyOf([]string{pod0, pod2}, []string{pod1}, nil),
		RedisReplicationRealMaster: func(context.Context, kubernetes.Interface, *rrvb2.RedisReplication, []string) string {
			return pod0
		},
		Checker:                    keyCountsOf(map[string]int64{pod0: 0, pod1: 0, pod2: 120}),
		CreateRedisReplicationLink: (&linkRecorder{}).link,
		SentinelFailover:           noSentinelFailover,
		ReplicationStates: replicationStatesOf(map[string]k8sutils.RedisReplicationState{
			pod2: {Role: "master", ReplID: "standalone", Offset: 900},
		}),
		PromoteReplicationPod: promotionForbidden(t),
		ConfigureSentinel:     func(context.Context, *rrvb2.RedisReplication, string) error { return nil },
	}

	result, err := r.reconcileRedis(context.Background(), newSentinelReplicationInstanceForTest())

	require.NoError(t, err)
	assert.Equal(t, 60*time.Second, result.RequeueAfter)
}

func TestReconcileRedisDoesNotPromoteAgainOnceThePromotedPodHasReplicas(t *testing.T) {
	r := &Reconciler{
		K8sClient:                fake.NewSimpleClientset(),
		RedisReplicationTopology: topologyOf([]string{pod1}, []string{pod0, pod2}, nil),
		RedisReplicationRealMaster: func(context.Context, kubernetes.Interface, *rrvb2.RedisReplication, []string) string {
			return pod1
		},
		Checker:                    keyCountsOf(map[string]int64{pod0: 120, pod1: 120, pod2: 0}),
		CreateRedisReplicationLink: (&linkRecorder{}).link,
		SentinelFailover:           noSentinelFailover,
		PromoteReplicationPod:      promotionForbidden(t),
	}

	result, err := r.reconcileRedis(context.Background(), newReplicationInstanceForTest())

	require.NoError(t, err)
	assert.Equal(t, ctrl.Result{}, result)
}

func TestBestDataHoldingPod(t *testing.T) {
	tests := []struct {
		name      string
		pods      []string
		keyCounts map[string]int64
		states    map[string]k8sutils.RedisReplicationState
		want      string
	}{
		{
			name:      "the replicas of one stream outnumber a pod with a higher offset",
			pods:      []string{pod0, pod1, pod2},
			keyCounts: map[string]int64{pod0: 10, pod1: 10, pod2: 10},
			states: map[string]k8sutils.RedisReplicationState{
				pod0: {Role: "master", ReplID: "stale", Offset: 9000},
				pod1: {Role: "slave", ReplID: "lost-master", Offset: 500},
				pod2: {Role: "slave", ReplID: "lost-master", Offset: 400},
			},
			want: pod1,
		},
		{
			name:      "the highest offset in the stream wins",
			pods:      []string{pod0, pod1},
			keyCounts: map[string]int64{pod0: 10, pod1: 10},
			states: map[string]k8sutils.RedisReplicationState{
				pod0: {Role: "slave", ReplID: "lost-master", Offset: 400},
				pod1: {Role: "slave", ReplID: "lost-master", Offset: 500},
			},
			want: pod1,
		},
		{
			name:      "more keys break an offset tie",
			pods:      []string{pod0, pod1},
			keyCounts: map[string]int64{pod0: 10, pod1: 20},
			states: map[string]k8sutils.RedisReplicationState{
				pod0: {Role: "slave", ReplID: "lost-master", Offset: 500},
				pod1: {Role: "slave", ReplID: "lost-master", Offset: 500},
			},
			want: pod1,
		},
		{
			name:      "the lowest ordinal breaks a full tie",
			pods:      []string{pod2, pod1},
			keyCounts: map[string]int64{pod1: 10, pod2: 10},
			states: map[string]k8sutils.RedisReplicationState{
				pod1: {Role: "slave", ReplID: "lost-master", Offset: 500},
				pod2: {Role: "slave", ReplID: "lost-master", Offset: 500},
			},
			want: pod1,
		},
		{
			name:      "an unknown state picks nothing",
			pods:      []string{pod0, pod1},
			keyCounts: map[string]int64{pod0: 10, pod1: 10},
			states: map[string]k8sutils.RedisReplicationState{
				pod0: {Role: "slave", ReplID: "lost-master", Offset: 500},
			},
			want: "",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, bestDataHoldingPod(tt.pods, tt.keyCounts, tt.states))
		})
	}
}

func TestRedisSentinelFailover(t *testing.T) {
	const (
		emptyMasterIP = "10.0.0.12"
		lostMasterIP  = "10.0.0.99"
	)
	instance := newReplicationInstanceForTest()
	monitoring := func(address string, slaves int) redischecker.SentinelMasterReport {
		return redischecker.SentinelMasterReport{Master: &redis.SentinelMasterInfo{Name: "myMaster", Address: address, Slaves: slaves}}
	}
	restarted := monitoring("0.0.0.0:6379", 0)
	lostMaster := monitoring(lostMasterIP+":6379", 2)
	unreachable := redischecker.SentinelMasterReport{Err: errors.New("i/o timeout")}
	window := 30*time.Second + 10*time.Second + sentinelFailoverMargin

	tests := []struct {
		name         string
		sentinels    []*rsvb2.RedisSentinel
		reports      map[string][]redischecker.SentinelMasterReport
		wantExpected bool
		wantWindow   time.Duration
	}{
		{
			name: "no RedisSentinel watches the replication",
			sentinels: []*rsvb2.RedisSentinel{
				newRedisSentinelForTest("other", "other-replication"),
			},
			reports: map[string][]redischecker.SentinelMasterReport{"other": {lostMaster, lostMaster, lostMaster}},
		},
		{
			name:      "every Sentinel restarted without its state",
			sentinels: []*rsvb2.RedisSentinel{newRedisSentinelForTest("rs", instance.Name)},
			reports:   map[string][]redischecker.SentinelMasterReport{"rs": {restarted, restarted, restarted}},
		},
		{
			name:      "no Sentinel monitors the master group",
			sentinels: []*rsvb2.RedisSentinel{newRedisSentinelForTest("rs", instance.Name)},
			reports:   map[string][]redischecker.SentinelMasterReport{"rs": {{}, {}, {}}},
		},
		{
			name:         "every Sentinel still monitors the lost master and its replicas",
			sentinels:    []*rsvb2.RedisSentinel{newRedisSentinelForTest("rs", instance.Name)},
			reports:      map[string][]redischecker.SentinelMasterReport{"rs": {lostMaster, lostMaster, lostMaster}},
			wantExpected: true,
			wantWindow:   window,
		},
		{
			name:         "a quorum still monitors the lost master",
			sentinels:    []*rsvb2.RedisSentinel{newRedisSentinelForTest("rs", instance.Name)},
			reports:      map[string][]redischecker.SentinelMasterReport{"rs": {lostMaster, lostMaster, restarted}},
			wantExpected: true,
			wantWindow:   window,
		},
		{
			name:      "a single Sentinel cannot fail over alone",
			sentinels: []*rsvb2.RedisSentinel{newRedisSentinelForTest("rs", instance.Name)},
			reports:   map[string][]redischecker.SentinelMasterReport{"rs": {lostMaster, restarted, restarted}},
		},
		{
			name:      "the Sentinels monitor the empty master, which they will not fail over while it answers",
			sentinels: []*rsvb2.RedisSentinel{newRedisSentinelForTest("rs", instance.Name)},
			reports: map[string][]redischecker.SentinelMasterReport{"rs": {
				monitoring(emptyMasterIP+":6379", 2),
				monitoring(replicationPodHostname(instance, pod2)+":6379", 2),
				monitoring(emptyMasterIP+":6379", 2),
			}},
		},
		{
			name:         "Sentinels that cannot be asked may still hold their state",
			sentinels:    []*rsvb2.RedisSentinel{newRedisSentinelForTest("rs", instance.Name)},
			reports:      map[string][]redischecker.SentinelMasterReport{"rs": {unreachable, unreachable, restarted}},
			wantExpected: true,
			wantWindow:   window,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			scheme := runtime.NewScheme()
			require.NoError(t, rsvb2.AddToScheme(scheme))
			builder := clientfake.NewClientBuilder().WithScheme(scheme)
			for _, rs := range tt.sentinels {
				builder = builder.WithObjects(rs)
			}
			r := &Reconciler{
				Client:    builder.Build(),
				K8sClient: fake.NewSimpleClientset(newRunningPodForTest("default", pod2, emptyMasterIP, nil, true)),
				Checker:   &fakeChecker{sentinelMasters: tt.reports},
			}

			expected, window, err := r.redisSentinelFailover(context.Background(), instance, pod2)

			require.NoError(t, err)
			assert.Equal(t, tt.wantExpected, expected)
			assert.Equal(t, tt.wantWindow, window)
		})
	}
}

func TestRedisSentinelFailoverFailsWhenTheSentinelsCannotBeListed(t *testing.T) {
	scheme := runtime.NewScheme()
	require.NoError(t, rsvb2.AddToScheme(scheme))
	r := &Reconciler{
		Client:    clientfake.NewClientBuilder().WithScheme(scheme).WithObjects(newRedisSentinelForTest("rs", "example-replication")).Build(),
		K8sClient: fake.NewSimpleClientset(newRunningPodForTest("default", pod2, "10.0.0.12", nil, true)),
		Checker:   &fakeChecker{sentinelErr: errors.New("connection refused")},
	}

	_, _, err := r.redisSentinelFailover(context.Background(), newReplicationInstanceForTest(), pod2)

	require.Error(t, err)
}

func TestSentinelsNeededForFailover(t *testing.T) {
	tests := []struct {
		size   int32
		quorum string
		want   int
	}{
		{size: 3, quorum: "2", want: 2},
		{size: 3, quorum: "1", want: 2},
		{size: 5, quorum: "4", want: 4},
		{size: 3, quorum: "", want: 2},
		{size: 1, quorum: "1", want: 1},
	}
	for _, tt := range tests {
		rs := newRedisSentinelForTest("rs", "example-replication")
		rs.Spec.Size = ptr.To(tt.size)
		rs.Spec.RedisSentinelConfig.Quorum = tt.quorum
		assert.Equal(t, tt.want, sentinelsNeededForFailover(rs), "size %d quorum %q", tt.size, tt.quorum)
	}
}

// lostMasterReplicas are two replicas that followed the same master up to the
// same offset.
var lostMasterReplicas = map[string]k8sutils.RedisReplicationState{
	pod0: {Role: "slave", ReplID: "lost-master", Offset: 500},
	pod1: {Role: "slave", ReplID: "lost-master", Offset: 500},
}

func newRedisSentinelForTest(name, replicationName string) *rsvb2.RedisSentinel {
	return &rsvb2.RedisSentinel{
		ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: "default"},
		Spec: rsvb2.RedisSentinelSpec{
			Size: ptr.To(int32(3)),
			RedisSentinelConfig: &rsvb2.RedisSentinelConfig{
				RedisSentinelConfig: commonapi.RedisSentinelConfig{
					RedisReplicationName: replicationName,
					MasterGroupName:      "myMaster",
					SentinelConfig: commonapi.SentinelConfig{
						Quorum:                "2",
						DownAfterMilliseconds: "30000",
						FailoverTimeout:       "10000",
					},
				},
			},
		},
	}
}

func newStatusClientForTest(t *testing.T) (client.Client, *rrvb2.RedisReplication) {
	t.Helper()
	scheme := runtime.NewScheme()
	require.NoError(t, rrvb2.AddToScheme(scheme))
	seed := newReplicationInstanceForTest()
	seed.UID = types.UID("example-replication-uid")
	ctrlClient := clientfake.NewClientBuilder().
		WithScheme(scheme).
		WithStatusSubresource(seed).
		WithObjects(seed.DeepCopy()).
		Build()
	instance := &rrvb2.RedisReplication{}
	require.NoError(t, ctrlClient.Get(context.Background(), client.ObjectKeyFromObject(seed), instance))
	return ctrlClient, instance
}

func recordedMaster(t *testing.T, ctrlClient client.Client, instance *rrvb2.RedisReplication) string {
	t.Helper()
	updated := &rrvb2.RedisReplication{}
	require.NoError(t, ctrlClient.Get(context.Background(), client.ObjectKeyFromObject(instance), updated))
	return updated.Status.MasterNode
}

func noRealMaster(context.Context, kubernetes.Interface, *rrvb2.RedisReplication, []string) string {
	return ""
}

func noSentinelFailover(context.Context, *rrvb2.RedisReplication, string) (bool, time.Duration, error) {
	return false, 0, nil
}

func sentinelFailoverExpected(window time.Duration) func(context.Context, *rrvb2.RedisReplication, string) (bool, time.Duration, error) {
	return func(context.Context, *rrvb2.RedisReplication, string) (bool, time.Duration, error) {
		return true, window, nil
	}
}

func replicationStatesOf(states map[string]k8sutils.RedisReplicationState) func(context.Context, kubernetes.Interface, *rrvb2.RedisReplication, []string) map[string]k8sutils.RedisReplicationState {
	return func(_ context.Context, _ kubernetes.Interface, _ *rrvb2.RedisReplication, pods []string) map[string]k8sutils.RedisReplicationState {
		read := make(map[string]k8sutils.RedisReplicationState, len(pods))
		for _, pod := range pods {
			if state, ok := states[pod]; ok {
				read[pod] = state
			}
		}
		return read
	}
}

func promotionForbidden(t *testing.T) func(context.Context, kubernetes.Interface, *rrvb2.RedisReplication, string) error {
	t.Helper()
	return func(_ context.Context, _ kubernetes.Interface, _ *rrvb2.RedisReplication, pod string) error {
		t.Errorf("unexpected promotion of %s", pod)
		return nil
	}
}

type promotionRecorder struct {
	pods []string
}

func (p *promotionRecorder) promote(_ context.Context, _ kubernetes.Interface, _ *rrvb2.RedisReplication, pod string) error {
	p.pods = append(p.pods, pod)
	return nil
}

type replicationLink struct {
	pods   []string
	master string
}

type linkRecorder struct {
	links []replicationLink
}

func (l *linkRecorder) link(_ context.Context, _ kubernetes.Interface, _ *rrvb2.RedisReplication, pods []string, master string) error {
	l.links = append(l.links, replicationLink{pods: append([]string{}, pods...), master: master})
	return nil
}
