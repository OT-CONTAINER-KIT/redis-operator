package redis

import (
	"context"
	"testing"

	"github.com/OT-CONTAINER-KIT/redis-operator/internal/controller/common"
	redisservice "github.com/OT-CONTAINER-KIT/redis-operator/internal/service/redis"
	"github.com/stretchr/testify/require"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	kubernetesfake "k8s.io/client-go/kubernetes/fake"
	k8stesting "k8s.io/client-go/testing"
)

func TestSentinelRoutingFailsClosed(t *testing.T) {
	for _, selected := range []string{"", "missing", "unready"} {
		t.Run(selected, func(t *testing.T) {
			var objects []runtime.Object
			for _, name := range []string{"old", "unready"} {
				objects = append(objects, &v1.Pod{ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: "redis", Labels: map[string]string{"app": "redis", common.RedisRoleLabelKey: common.RedisRoleLabelMaster}}})
			}
			k8s := kubernetesfake.NewSimpleClientset(objects...)
			err := NewHealer(k8s).SetRedisMasterLabel(context.Background(), "redis", map[string]string{"app": "redis"}, nil, nil, selected)
			require.Equal(t, selected == "", err == nil)
			pods, err := k8s.CoreV1().Pods("redis").List(context.Background(), metav1.ListOptions{})
			require.NoError(t, err)
			for _, pod := range pods.Items {
				require.NotEqual(t, common.RedisRoleLabelMaster, pod.Labels[common.RedisRoleLabelKey])
			}
		})
	}
}

func TestSentinelMonitorPreservesAnExistingElection(t *testing.T) {
	rs, _, objects := newSentinelFixture("192.0.2.10")
	k8s := kubernetesfake.NewSimpleClientset(objects...)
	redis := &fakeRedisClient{sentinelMasterByHost: map[string]*redisservice.SentinelMasterState{
		"192.0.2.10": {IP: "192.0.2.2", Port: "6379", Flags: "master", Epoch: "9"},
	}}
	h := &healer{k8s: k8s, redis: redis}
	require.NoError(t, h.SentinelMonitor(context.Background(), rs, "192.0.2.1"))
	require.Empty(t, redis.monitorHosts)
	require.Empty(t, redis.resetHosts)
}

func TestSentinelRoutingVerifiesRoleBeforeAdvertising(t *testing.T) {
	for _, master := range []bool{true, false} {
		t.Run(map[bool]string{true: "confirmed", false: "replica"}[master], func(t *testing.T) {
			old := &v1.Pod{ObjectMeta: metav1.ObjectMeta{Name: "old", Namespace: "redis", Labels: map[string]string{"app": "redis", common.RedisRoleLabelKey: "master"}}}
			chosen := &v1.Pod{ObjectMeta: metav1.ObjectMeta{Name: "chosen", Namespace: "redis", UID: "verified-incarnation", Labels: map[string]string{"app": "redis", common.RedisRoleLabelKey: "slave"}}, Status: v1.PodStatus{PodIP: "192.0.2.1", Conditions: []v1.PodCondition{{Type: v1.PodReady, Status: v1.ConditionTrue}}}}
			k8s := kubernetesfake.NewSimpleClientset(old, chosen)
			h := &healer{k8s: k8s, redis: &fakeRedisClient{isMasterByHost: map[string]bool{"192.0.2.1": master}}}
			err := h.SetRedisMasterLabel(context.Background(), "redis", map[string]string{"app": "redis"}, nil, nil, "chosen")
			require.Equal(t, master, err == nil)
			old, err = k8s.CoreV1().Pods("redis").Get(context.Background(), "old", metav1.GetOptions{})
			require.NoError(t, err)
			require.NotEqual(t, "master", old.Labels[common.RedisRoleLabelKey])
			chosen, err = k8s.CoreV1().Pods("redis").Get(context.Background(), "chosen", metav1.GetOptions{})
			require.NoError(t, err)
			require.Equal(t, master, chosen.Labels[common.RedisRoleLabelKey] == "master")
		})
	}
}

func TestSentinelRoutingRejectsSameNameReplacement(t *testing.T) {
	pod := &v1.Pod{ObjectMeta: metav1.ObjectMeta{Name: "chosen", Namespace: "redis", UID: "verified-incarnation", Labels: map[string]string{"app": "redis", common.RedisRoleLabelKey: "slave"}}, Status: v1.PodStatus{PodIP: "192.0.2.1", Conditions: []v1.PodCondition{{Type: v1.PodReady, Status: v1.ConditionTrue}}}}
	k8s := kubernetesfake.NewSimpleClientset(pod)
	k8s.PrependReactor("patch", "pods", func(action k8stesting.Action) (bool, runtime.Object, error) {
		replacement := pod.DeepCopy()
		replacement.UID = "replacement-incarnation"
		replacement.Status.PodIP = "192.0.2.2"
		require.NoError(t, k8s.Tracker().Delete(v1.SchemeGroupVersion.WithResource("pods"), "redis", "chosen"))
		require.NoError(t, k8s.Tracker().Create(v1.SchemeGroupVersion.WithResource("pods"), replacement, "redis"))
		return false, nil, nil
	})
	h := &healer{k8s: k8s, redis: &fakeRedisClient{isMasterByHost: map[string]bool{"192.0.2.1": true}}}
	require.Error(t, h.SetRedisMasterLabel(context.Background(), "redis", map[string]string{"app": "redis"}, nil, nil, "chosen"))
	current, err := k8s.CoreV1().Pods("redis").Get(context.Background(), "chosen", metav1.GetOptions{})
	require.NoError(t, err)
	require.Equal(t, "slave", current.Labels[common.RedisRoleLabelKey])
}
