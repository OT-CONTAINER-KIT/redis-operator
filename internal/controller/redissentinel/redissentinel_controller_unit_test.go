package redissentinel

import (
	"context"
	"errors"
	"testing"
	"time"

	commonapi "github.com/OT-CONTAINER-KIT/redis-operator/api/common/v1beta2"
	rcvb2 "github.com/OT-CONTAINER-KIT/redis-operator/api/rediscluster/v1beta2"
	rrvb2 "github.com/OT-CONTAINER-KIT/redis-operator/api/redisreplication/v1beta2"
	rsvb2 "github.com/OT-CONTAINER-KIT/redis-operator/api/redissentinel/v1beta2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes/fake"
	"k8s.io/utils/ptr"
	clientfake "sigs.k8s.io/controller-runtime/pkg/client/fake"
)

func TestReconcileSentinelRequeuesWithoutTouchingSentinelsWhenNoReachableMaster(t *testing.T) {
	scheme := runtime.NewScheme()
	require.NoError(t, rrvb2.AddToScheme(scheme))

	rr := &rrvb2.RedisReplication{
		ObjectMeta: metav1.ObjectMeta{Name: "example-replication", Namespace: "default"},
		Spec:       rrvb2.RedisReplicationSpec{Size: ptr.To(int32(3))},
	}
	instance := &rsvb2.RedisSentinel{
		ObjectMeta: metav1.ObjectMeta{Name: "example", Namespace: "default"},
		Spec: rsvb2.RedisSentinelSpec{
			Size: ptr.To(int32(3)),
			RedisSentinelConfig: &rsvb2.RedisSentinelConfig{
				RedisSentinelConfig: commonapi.RedisSentinelConfig{
					RedisReplicationName: rr.Name,
					MasterGroupName:      "myMaster",
				},
			},
		},
	}
	healer := &fakeHealer{}
	r := &RedisSentinelReconciler{
		Client:    clientfake.NewClientBuilder().WithScheme(scheme).WithObjects(rr).Build(),
		K8sClient: fake.NewSimpleClientset(),
		Checker:   &fakeChecker{},
		Healer:    healer,
	}

	result, err := r.reconcileSentinel(context.Background(), instance)

	require.NoError(t, err)
	assert.Equal(t, 10*time.Second, result.RequeueAfter)
	assert.Empty(t, healer.monitoredMasters)
}

func failoverInProgressSentinelReconciler(t *testing.T, checker *fakeChecker, healer *fakeHealer) (*RedisSentinelReconciler, *rsvb2.RedisSentinel) {
	t.Helper()
	scheme := runtime.NewScheme()
	require.NoError(t, rrvb2.AddToScheme(scheme))
	rr := &rrvb2.RedisReplication{
		ObjectMeta: metav1.ObjectMeta{Name: "example-replication", Namespace: "default"},
		Spec:       rrvb2.RedisReplicationSpec{Size: ptr.To(int32(3))},
	}
	instance := &rsvb2.RedisSentinel{
		ObjectMeta: metav1.ObjectMeta{Name: "example", Namespace: "default"},
		Spec: rsvb2.RedisSentinelSpec{
			Size: ptr.To(int32(3)),
			RedisSentinelConfig: &rsvb2.RedisSentinelConfig{
				RedisSentinelConfig: commonapi.RedisSentinelConfig{
					RedisReplicationName: rr.Name,
					MasterGroupName:      "myMaster",
				},
			},
		},
	}
	return &RedisSentinelReconciler{
		Client:    clientfake.NewClientBuilder().WithScheme(scheme).WithObjects(rr).Build(),
		K8sClient: fake.NewSimpleClientset(),
		Checker:   checker,
		Healer:    healer,
	}, instance
}

func podWithIP(name, ip string) corev1.Pod {
	return corev1.Pod{ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: "default"}, Status: corev1.PodStatus{PodIP: ip}}
}

// Right after a Sentinel failover the old master may still have replicas
// attached while the promoted pod has none: the controller must keep the
// Sentinels on the master they elected instead of re-pointing them.
func TestReconcileSentinelKeepsMonitoringTheMasterElectedBySentinel(t *testing.T) {
	healer := &fakeHealer{}
	r, instance := failoverInProgressSentinelReconciler(t, &fakeChecker{
		master:        podWithIP("example-replication-2", "10.0.0.12"),
		electedMaster: podWithIP("example-replication-0", "10.0.0.10"),
	}, healer)

	_, err := r.reconcileSentinel(context.Background(), instance)

	require.NoError(t, err)
	assert.Equal(t, []string{"10.0.0.10"}, healer.monitoredMasters)
}

func TestReconcileSentinelFallsBackToTheMasterWithReplicasWithoutSentinelQuorum(t *testing.T) {
	healer := &fakeHealer{}
	r, instance := failoverInProgressSentinelReconciler(t, &fakeChecker{
		master: podWithIP("example-replication-2", "10.0.0.12"),
	}, healer)

	_, err := r.reconcileSentinel(context.Background(), instance)

	require.NoError(t, err)
	assert.Equal(t, []string{"10.0.0.12"}, healer.monitoredMasters)
}

func TestReconcileSentinelFallsBackToTheMasterWithReplicasWhenSentinelCannotBeAsked(t *testing.T) {
	healer := &fakeHealer{}
	r, instance := failoverInProgressSentinelReconciler(t, &fakeChecker{
		master:     podWithIP("example-replication-2", "10.0.0.12"),
		electedErr: errors.New("dial tcp: i/o timeout"),
	}, healer)

	_, err := r.reconcileSentinel(context.Background(), instance)

	require.NoError(t, err)
	assert.Equal(t, []string{"10.0.0.12"}, healer.monitoredMasters)
}

type fakeChecker struct {
	master        corev1.Pod
	electedMaster corev1.Pod
	electedErr    error
	failover      bool
}

func (f *fakeChecker) SentinelFailoverInProgress(context.Context, *rsvb2.RedisSentinel) (bool, error) {
	return f.failover, nil
}

func (f *fakeChecker) GetMasterFromReplication(context.Context, *rrvb2.RedisReplication) (corev1.Pod, error) {
	return f.master, nil
}

func (f *fakeChecker) GetSentinelElectedMaster(context.Context, *rsvb2.RedisSentinel, *rrvb2.RedisReplication) (corev1.Pod, error) {
	return f.electedMaster, f.electedErr
}

func (f *fakeChecker) GetPassword(context.Context, string, *commonapi.ExistingPasswordSecret) (string, error) {
	return "", nil
}

func (f *fakeChecker) CheckClusterSlotsAssigned(context.Context, *rcvb2.RedisCluster) (bool, error) {
	return true, nil
}

func (f *fakeChecker) GetReplicationKeyCounts(context.Context, *rrvb2.RedisReplication, []string) map[string]int64 {
	return nil
}

type fakeHealer struct {
	monitoredMasters []string
}

func (f *fakeHealer) SentinelMonitor(_ context.Context, _ *rsvb2.RedisSentinel, master string) error {
	f.monitoredMasters = append(f.monitoredMasters, master)
	return nil
}

func (f *fakeHealer) SentinelSet(context.Context, *rsvb2.RedisSentinel, string) error {
	return nil
}

func (f *fakeHealer) SentinelReset(context.Context, *rsvb2.RedisSentinel, *rrvb2.RedisReplication) error {
	return nil
}

func (f *fakeHealer) UpdateRedisRoleLabel(context.Context, string, map[string]string, *commonapi.ExistingPasswordSecret, *commonapi.TLSConfig, string) error {
	return nil
}

// Commands sent to the Sentinels while one of them runs a failover can abort
// it (SENTINEL RESET) or point them back at the old master.
func TestReconcileSentinelLeavesTheSentinelsAloneDuringAFailover(t *testing.T) {
	healer := &fakeHealer{}
	r, instance := failoverInProgressSentinelReconciler(t, &fakeChecker{
		master:   podWithIP("example-replication-0", "10.0.0.10"),
		failover: true,
	}, healer)

	result, err := r.reconcileSentinel(context.Background(), instance)

	require.NoError(t, err)
	assert.Equal(t, 5*time.Second, result.RequeueAfter)
	assert.Empty(t, healer.monitoredMasters)
}
