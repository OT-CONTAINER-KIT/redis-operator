package redis

import (
	"context"
	"testing"

	common "github.com/OT-CONTAINER-KIT/redis-operator/internal/controller/common"
	redisservice "github.com/OT-CONTAINER-KIT/redis-operator/internal/service/redis"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	k8sfake "k8s.io/client-go/kubernetes/fake"
)

func TestUpdateRedisRoleLabelSkipsUnprobeablePods(t *testing.T) {
	labels := map[string]string{"app": "redis"}
	clientset := k8sfake.NewSimpleClientset(
		newLabeledRedisPod("redis-0", labels, "10.0.0.10", corev1.PodRunning, true),
		newLabeledRedisPod("redis-1", labels, "", corev1.PodRunning, true),
		newLabeledRedisPod("redis-2", labels, "10.0.0.12", corev1.PodRunning, false),
		newLabeledRedisPod("redis-3", labels, "", corev1.PodPending, false),
	)
	redisClient := &fakeRedisClient{
		isMasterByHost: map[string]bool{
			"10.0.0.10": true,
		},
	}
	h := &healer{
		k8s:   clientset,
		redis: redisClient,
	}

	err := h.UpdateRedisRoleLabel(context.Background(), "default", labels, nil, nil)

	require.NoError(t, err)
	assert.Equal(t, []string{"10.0.0.10"}, redisClient.connectHosts)

	readyPod, err := clientset.CoreV1().Pods("default").Get(context.Background(), "redis-0", metav1.GetOptions{})
	require.NoError(t, err)
	assert.Equal(t, common.RedisRoleLabelMaster, readyPod.Labels[common.RedisRoleLabelKey])

	for _, podName := range []string{"redis-1", "redis-2", "redis-3"} {
		pod, getErr := clientset.CoreV1().Pods("default").Get(context.Background(), podName, metav1.GetOptions{})
		require.NoError(t, getErr)
		assert.Empty(t, pod.Labels[common.RedisRoleLabelKey])
	}
}

type fakeRedisClient struct {
	connectHosts   []string
	isMasterByHost map[string]bool
}

func (f *fakeRedisClient) Connect(info *redisservice.ConnectionInfo) redisservice.Service {
	f.connectHosts = append(f.connectHosts, info.Host)
	return &fakeRedisService{
		host:           info.Host,
		isMasterByHost: f.isMasterByHost,
	}
}

type fakeRedisService struct {
	host           string
	isMasterByHost map[string]bool
}

func (f *fakeRedisService) IsMaster(context.Context) (bool, error) {
	return f.isMasterByHost[f.host], nil
}

func (f *fakeRedisService) GetAttachedReplicaCount(context.Context) (int, error) {
	return 0, nil
}

func (f *fakeRedisService) SentinelMonitor(context.Context, *redisservice.ConnectionInfo, string, string) error {
	return nil
}

func (f *fakeRedisService) SentinelSet(context.Context, string, string, string) error {
	return nil
}

func (f *fakeRedisService) SentinelReset(context.Context, string) error {
	return nil
}

func (f *fakeRedisService) GetInfoSentinel(context.Context) (*redisservice.InfoSentinelResult, error) {
	return &redisservice.InfoSentinelResult{}, nil
}

func (f *fakeRedisService) GetClusterInfo(context.Context) (*redisservice.ClusterStatus, error) {
	return &redisservice.ClusterStatus{}, nil
}

func newLabeledRedisPod(name string, labels map[string]string, podIP string, phase corev1.PodPhase, ready bool) *corev1.Pod {
	podLabels := map[string]string{}
	for key, value := range labels {
		podLabels[key] = value
	}

	readyStatus := corev1.ConditionFalse
	if ready {
		readyStatus = corev1.ConditionTrue
	}

	return &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: "default",
			Labels:    podLabels,
		},
		Status: corev1.PodStatus{
			Phase: phase,
			PodIP: podIP,
			Conditions: []corev1.PodCondition{
				{
					Type:   corev1.PodReady,
					Status: readyStatus,
				},
			},
		},
	}
}
