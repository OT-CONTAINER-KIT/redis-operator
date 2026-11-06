package redisreplication

import (
	"context"
	"errors"
	"fmt"
	"testing"

	rrvb2 "github.com/OT-CONTAINER-KIT/redis-operator/api/redisreplication/v1beta2"
	rsvb2 "github.com/OT-CONTAINER-KIT/redis-operator/api/redissentinel/v1beta2"
	"github.com/OT-CONTAINER-KIT/redis-operator/internal/service/redis"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type authSetProbe struct {
	redis.Service
	calls int
	err   error
}

func (f *authSetProbe) SentinelSet(context.Context, string, string, string) error {
	f.calls++
	return f.err
}

func authFixture() (*Reconciler, *rrvb2.RedisReplication, *rsvb2.RedisSentinel, []observedSentinelAuth, *authSetProbe) {
	f := &authSetProbe{}
	rr := &rrvb2.RedisReplication{ObjectMeta: metav1.ObjectMeta{Namespace: "test", Name: "redis", UID: "redis-owner"}}
	rs := &rsvb2.RedisSentinel{ObjectMeta: metav1.ObjectMeta{Namespace: "test", Name: "sentinel", UID: "sentinel-owner"}}
	pod := corev1.Pod{ObjectMeta: metav1.ObjectMeta{Name: "sentinel-0", UID: "sentinel-pod-0"}}
	peers := []observedSentinelAuth{{pod: pod, service: f, report: redis.SentinelMasterState{Flags: "master"}}}
	return &Reconciler{}, rr, rs, peers, f
}

func TestStandaloneAuthIsNotReappliedForEveryAuthorityRead(t *testing.T) {
	r, rr, rs, peers, f := authFixture()
	for range 3 {
		require.NoError(t, r.reconcileStandaloneAuth(context.Background(), rr, rs, "myMaster", "synthetic-password", peers))
	}
	require.Equal(t, 1, f.calls)
	require.NoError(t, r.reconcileStandaloneAuth(context.Background(), rr, rs, "myMaster", "synthetic-new-password", peers))
	require.Equal(t, 2, f.calls, "a different credential value remains retryable")
	for range 3 {
		require.NoError(t, r.reconcileStandaloneAuth(context.Background(), rr, rs, "myMaster", "synthetic-new-password", peers))
	}
	require.Equal(t, 2, f.calls)
}

func TestNoAuthProfileNeverDisconnectsSentinelMonitoringWithEmptyAuthSet(t *testing.T) {
	r, rr, rs, peers, f := authFixture()
	for range 3 {
		require.NoError(t, r.reconcileStandaloneAuth(context.Background(), rr, rs, "myMaster", "", peers))
	}
	require.Zero(t, f.calls)
}

func TestStandaloneAuthDefersDuringAnyActiveElection(t *testing.T) {
	r, rr, rs, peers, f := authFixture()
	for _, phase := range []string{"wait_promotion", "reconf_slaves", "update_config"} {
		peers[0].report.Flags = "master,failover_in_progress"
		peers[0].report.FailoverState = phase
		require.NoError(t, r.reconcileStandaloneAuth(context.Background(), rr, rs, "myMaster", "synthetic-password", peers))
	}
	require.Zero(t, f.calls)
	peers[0].report.Flags = "master"
	require.NoError(t, r.reconcileStandaloneAuth(context.Background(), rr, rs, "myMaster", "synthetic-password", peers))
	require.Equal(t, 1, f.calls)
}

func TestFailedAuthAndMissingMonitorRemainRetryable(t *testing.T) {
	r, rr, rs, peers, f := authFixture()
	f.err = errors.New("synthetic transport failure")
	require.Error(t, r.reconcileStandaloneAuth(context.Background(), rr, rs, "myMaster", "synthetic-password", peers))
	f.err = nil
	require.NoError(t, r.reconcileStandaloneAuth(context.Background(), rr, rs, "myMaster", "synthetic-password", peers))
	require.Equal(t, 2, f.calls)
	r.forgetSentinelAuth(authKey(rr, rs, peers[0].pod, "myMaster"))
	require.NoError(t, r.reconcileStandaloneAuth(context.Background(), rr, rs, "myMaster", "synthetic-password", peers))
	require.Equal(t, 3, f.calls)
}

func TestAuthenticationMemoIsBoundToPodAndResourceIdentityAndPruned(t *testing.T) {
	r, rr, rs, peers, f := authFixture()
	require.NoError(t, r.reconcileStandaloneAuth(context.Background(), rr, rs, "myMaster", "synthetic-password", peers))
	peers[0].pod.UID = "replacement-pod"
	r.pruneSentinelAuth(rr, rs, []corev1.Pod{peers[0].pod}, "myMaster")
	require.Empty(t, r.sentinelAuth)
	require.NoError(t, r.reconcileStandaloneAuth(context.Background(), rr, rs, "myMaster", "synthetic-password", peers))
	require.Equal(t, 2, f.calls)
	require.Len(t, r.sentinelAuth, 1)
	r.forgetSentinelAuthOwner(authKey(rr, rs, peers[0].pod, "myMaster").owner)
	require.Empty(t, r.sentinelAuth)
}

func TestAuthenticationMemoDiagnosticsAreRedacted(t *testing.T) {
	entry := cachedSentinelAuth{password: "synthetic-never-log"}
	require.NotContains(t, fmt.Sprintf("%v %#v", entry, entry), entry.password)
}
