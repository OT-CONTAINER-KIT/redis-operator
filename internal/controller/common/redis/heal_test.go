package redis

import (
	"context"
	"testing"

	common "github.com/OT-CONTAINER-KIT/redis-operator/internal/controller/common"
	sentinelv1beta2 "github.com/OT-CONTAINER-KIT/redis-operator/api/redissentinel/v1beta2"
	apicommon "github.com/OT-CONTAINER-KIT/redis-operator/api/common/v1beta2"
	redisservice "github.com/OT-CONTAINER-KIT/redis-operator/internal/service/redis"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
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
		host:              info.Host,
		isMasterByHost:    f.isMasterByHost,
		SentinelInfo:      &redisservice.InfoSentinelResult{},
		SentinelResetCalls: nil,
	}
}

type fakeRedisService struct {
	host               string
	isMasterByHost     map[string]bool
	SentinelInfo       *redisservice.InfoSentinelResult
	SentinelResetCalls []string // records masterGroupName each time SentinelReset was called
}

func (f *fakeRedisService) IsMaster(ctx context.Context) (bool, error) {
	return f.isMasterByHost[f.host], nil
}

func (f *fakeRedisService) GetAttachedReplicaCount(ctx context.Context) (int, error) {
	return 0, nil
}

func (f *fakeRedisService) SentinelMonitor(ctx context.Context, master *redisservice.ConnectionInfo, masterGroupName, quorum string) error {
	return nil
}

func (f *fakeRedisService) SentinelSet(ctx context.Context, masterGroupName, key, value string) error {
	return nil
}

func (f *fakeRedisService) SentinelReset(ctx context.Context, masterGroupName string) error {
	f.SentinelResetCalls = append(f.SentinelResetCalls, masterGroupName)
	return nil
}

func (f *fakeRedisService) GetInfoSentinel(ctx context.Context) (*redisservice.InfoSentinelResult, error) {
	return f.SentinelInfo, nil
}

func (f *fakeRedisService) GetClusterInfo(ctx context.Context) (*redisservice.ClusterStatus, error) {
	return &redisservice.ClusterStatus{}, nil
}

func TestSentinelReset_Conditional(t *testing.T) {
	masterGroupName := "mymaster"

	// RedisSentinel with nested MasterGroupName via RedisSentinelConfig
	makeSentinelRS := func() *sentinelv1beta2.RedisSentinel {
		size := int32(3)
		return &sentinelv1beta2.RedisSentinel{
			ObjectMeta: metav1.ObjectMeta{Name: "rs", Namespace: "default"},
			Spec: sentinelv1beta2.RedisSentinelSpec{
				Size: &size,
				RedisSentinelConfig: &sentinelv1beta2.RedisSentinelConfig{
					RedisSentinelConfig: apicommon.RedisSentinelConfig{
						MasterGroupName: masterGroupName,
					},
				},
			},
		}
	}

	// sentinel labels must match StatefulSet selector
	sentinelLabels := map[string]string{"app": "redis-sentinel", "sentinel-name": "rs"}
	makePod := func(name string) *corev1.Pod {
		return &corev1.Pod{
			ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: "default", Labels: sentinelLabels},
		}
	}
	makeSTS := func() *appsv1.StatefulSet {
		return &appsv1.StatefulSet{
			ObjectMeta: metav1.ObjectMeta{Name: "rs-sentinel", Namespace: "default"},
			Spec: appsv1.StatefulSetSpec{
				Selector: &metav1.LabelSelector{MatchLabels: sentinelLabels},
			},
		}
	}

	// expectedSlaves = 2 (size=3 minus 1 master)
	// expectedSentinels = 3 (INFO counts the reporting sentinel itself)

	t.Run("no reset when topology matches", func(t *testing.T) {
		rs := makeSentinelRS()
		fakeSvc := &fakeRedisSvc{
			info: &redisservice.InfoSentinelResult{
				Masters: []redisservice.SentinelMasterInfo{
					{Name: masterGroupName, Slaves: 2, Sentinels: 3},
				},
			},
			resetCalls: nil,
		}
		k8s := k8sfake.NewSimpleClientset([]runtime.Object{makeSTS(), makePod("sentinel-0")}...)
		h := &healer{k8s: k8s, redis: &fakeRedisSvcClient{svc: fakeSvc}}

		err := h.SentinelReset(context.Background(), rs, 2)
		require.NoError(t, err)
		assert.Empty(t, fakeSvc.resetCalls, "SentinelReset should not be called when topology matches")
	})

	t.Run("reset when slave count mismatches", func(t *testing.T) {
		rs := makeSentinelRS()
		fakeSvc := &fakeRedisSvc{
			info: &redisservice.InfoSentinelResult{
				Masters: []redisservice.SentinelMasterInfo{
					{Name: masterGroupName, Slaves: 1, Sentinels: 3}, // expected 2, got 1
				},
			},
			resetCalls: nil,
		}
		k8s := k8sfake.NewSimpleClientset([]runtime.Object{makeSTS(), makePod("sentinel-0")}...)
		h := &healer{k8s: k8s, redis: &fakeRedisSvcClient{svc: fakeSvc}}

		err := h.SentinelReset(context.Background(), rs, 2)
		require.NoError(t, err)
		assert.Equal(t, []string{masterGroupName}, fakeSvc.resetCalls, "SentinelReset should be called when slave count mismatches")
	})

	t.Run("reset when sentinel count mismatches", func(t *testing.T) {
		rs := makeSentinelRS()
		fakeSvc := &fakeRedisSvc{
			info: &redisservice.InfoSentinelResult{
				Masters: []redisservice.SentinelMasterInfo{
					{Name: masterGroupName, Slaves: 2, Sentinels: 2}, // expected 3, got 2
				},
			},
			resetCalls: nil,
		}
		k8s := k8sfake.NewSimpleClientset([]runtime.Object{makeSTS(), makePod("sentinel-0")}...)
		h := &healer{k8s: k8s, redis: &fakeRedisSvcClient{svc: fakeSvc}}

		err := h.SentinelReset(context.Background(), rs, 2)
		require.NoError(t, err)
		assert.Equal(t, []string{masterGroupName}, fakeSvc.resetCalls, "SentinelReset should be called when sentinel count mismatches")
	})

	t.Run("skip when master group not found", func(t *testing.T) {
		rs := makeSentinelRS()
		fakeSvc := &fakeRedisSvc{
			info:       &redisservice.InfoSentinelResult{Masters: []redisservice.SentinelMasterInfo{}},
			resetCalls: nil,
		}
		k8s := k8sfake.NewSimpleClientset([]runtime.Object{makeSTS(), makePod("sentinel-0")}...)
		h := &healer{k8s: k8s, redis: &fakeRedisSvcClient{svc: fakeSvc}}

		err := h.SentinelReset(context.Background(), rs, 2)
		require.NoError(t, err)
		assert.Empty(t, fakeSvc.resetCalls, "SentinelReset should not be called when master group not found")
	})
}

// fakeRedisSvcClient returns a single controlled fake service from Connect
type fakeRedisSvcClient struct{ svc *fakeRedisSvc }

func (c *fakeRedisSvcClient) Connect(*redisservice.ConnectionInfo) redisservice.Service { return c.svc }

type fakeRedisSvc struct {
	info       *redisservice.InfoSentinelResult
	resetCalls []string
}

func (f *fakeRedisSvc) IsMaster(ctx context.Context) (bool, error)                    { return false, nil }
func (f *fakeRedisSvc) GetAttachedReplicaCount(ctx context.Context) (int, error)       { return 0, nil }
func (f *fakeRedisSvc) SentinelMonitor(ctx context.Context, master *redisservice.ConnectionInfo, masterGroupName, quorum string) error {
	return nil
}
func (f *fakeRedisSvc) SentinelSet(ctx context.Context, masterGroupName, key, value string) error { return nil }
func (f *fakeRedisSvc) GetClusterInfo(ctx context.Context) (*redisservice.ClusterStatus, error) { return &redisservice.ClusterStatus{}, nil }

func (f *fakeRedisSvc) GetInfoSentinel(ctx context.Context) (*redisservice.InfoSentinelResult, error) {
	return f.info, nil
}

func (f *fakeRedisSvc) SentinelReset(ctx context.Context, masterGroupName string) error {
	f.resetCalls = append(f.resetCalls, masterGroupName)
	return nil
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
