package redis

import (
	"context"
	"errors"
	"fmt"
	"testing"

	rrvb2 "github.com/OT-CONTAINER-KIT/redis-operator/api/redisreplication/v1beta2"
	redisservice "github.com/OT-CONTAINER-KIT/redis-operator/internal/service/redis"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	k8sfake "k8s.io/client-go/kubernetes/fake"
	"k8s.io/utils/ptr"
)

func TestGetMasterFromReplicationSkipsUnreachableAndUnreadyPods(t *testing.T) {
	rr, objects := newReplicationFixture("10.0.0.10", "10.0.0.11")
	labels := map[string]string{"app": rr.GetStatefulSetName()}
	objects = append(objects, newLabeledRedisPod(rr.GetStatefulSetName()+"-2", labels, "10.0.0.12", corev1.PodRunning, false))
	redisClient := &fakeRedisClient{
		errByHost:      map[string]error{"10.0.0.10": errors.New("dial tcp 10.0.0.10:6379: i/o timeout")},
		isMasterByHost: map[string]bool{"10.0.0.11": true, "10.0.0.12": true},
		replicasByHost: map[string]int{"10.0.0.11": 1, "10.0.0.12": 1},
	}
	c := &checker{redis: redisClient, k8s: k8sfake.NewSimpleClientset(objects...)}

	master, err := c.GetMasterFromReplication(context.Background(), rr)
	require.NoError(t, err)

	assert.Equal(t, rr.GetStatefulSetName()+"-1", master.Name)
	assert.NotContains(t, redisClient.connectHosts, "10.0.0.12")
}

func TestGetReplicationKeyCountsSkipsUnprobeableAndFailedPods(t *testing.T) {
	rr, objects := newReplicationFixture("10.0.0.10", "10.0.0.11")
	labels := map[string]string{"app": rr.GetStatefulSetName()}
	objects = append(objects, newLabeledRedisPod(rr.GetStatefulSetName()+"-2", labels, "10.0.0.12", corev1.PodRunning, false))
	redisClient := &fakeRedisClient{
		keyCountByHost:    map[string]int64{"10.0.0.11": 120, "10.0.0.12": 120},
		keyCountErrByHost: map[string]error{"10.0.0.10": errors.New("dial tcp 10.0.0.10:6379: i/o timeout")},
	}
	c := &checker{redis: redisClient, k8s: k8sfake.NewSimpleClientset(objects...)}

	keyCounts := c.GetReplicationKeyCounts(context.Background(), rr, []string{
		rr.GetStatefulSetName() + "-0",
		rr.GetStatefulSetName() + "-1",
		rr.GetStatefulSetName() + "-2",
		rr.GetStatefulSetName() + "-3",
	})

	assert.Equal(t, map[string]int64{rr.GetStatefulSetName() + "-1": 120}, keyCounts)
	assert.NotContains(t, redisClient.connectHosts, "10.0.0.12")
}

func TestGetSentinelMastersReportsEverySentinel(t *testing.T) {
	rs, _, objects := newSentinelFixture("10.0.1.10", "10.0.1.11", "10.0.1.12")
	labels := map[string]string{"app": rs.GetStatefulSetName()}
	objects = append(objects,
		newLabeledRedisPod(rs.GetStatefulSetName()+"-3", labels, "10.0.1.13", corev1.PodRunning, false),
		newLabeledRedisPod(rs.GetStatefulSetName()+"-4", labels, "10.0.1.14", corev1.PodFailed, false),
	)
	monitored := redisservice.SentinelMasterInfo{Name: "myMaster", Status: "ok", Address: "10.0.0.10:6379", Slaves: 2, Sentinels: 3}
	redisClient := &fakeRedisClient{
		errByHost: map[string]error{"10.0.1.12": errors.New("dial tcp 10.0.1.12:26379: i/o timeout")},
		sentinelInfoByHost: map[string]*redisservice.InfoSentinelResult{
			"10.0.1.10": {Masters: []redisservice.SentinelMasterInfo{monitored}},
			"10.0.1.11": {Masters: []redisservice.SentinelMasterInfo{{Name: "otherGroup", Address: "10.0.0.20:6379"}}},
		},
	}
	c := &checker{redis: redisClient, k8s: k8sfake.NewSimpleClientset(objects...)}

	reports, err := c.GetSentinelMasters(context.Background(), rs)

	require.NoError(t, err)
	require.Len(t, reports, 4)
	byPod := make(map[string]SentinelMasterReport, len(reports))
	for _, report := range reports {
		byPod[report.Pod] = report
	}
	assert.Equal(t, &monitored, byPod[rs.GetStatefulSetName()+"-0"].Master)
	assert.NoError(t, byPod[rs.GetStatefulSetName()+"-0"].Err)
	assert.Nil(t, byPod[rs.GetStatefulSetName()+"-1"].Master, "a Sentinel that does not monitor the group reports no master")
	assert.NoError(t, byPod[rs.GetStatefulSetName()+"-1"].Err)
	assert.Error(t, byPod[rs.GetStatefulSetName()+"-2"].Err)
	assert.Error(t, byPod[rs.GetStatefulSetName()+"-3"].Err, "a Sentinel that is not ready may still hold its state")
	assert.NotContains(t, byPod, rs.GetStatefulSetName()+"-4", "a terminated pod runs no Sentinel")
	assert.NotContains(t, redisClient.connectHosts, "10.0.1.13")
}

func TestGetSentinelMastersReportsNothingWithoutASentinelStatefulSet(t *testing.T) {
	rs, _, _ := newSentinelFixture()
	c := &checker{redis: &fakeRedisClient{}, k8s: k8sfake.NewSimpleClientset()}

	reports, err := c.GetSentinelMasters(context.Background(), rs)

	require.NoError(t, err)
	assert.Empty(t, reports)
}

func newReplicationFixture(podIPs ...string) (*rrvb2.RedisReplication, []runtime.Object) {
	rr := &rrvb2.RedisReplication{
		ObjectMeta: metav1.ObjectMeta{Name: "example-replication", Namespace: "default"},
		Spec:       rrvb2.RedisReplicationSpec{Size: ptr.To(int32(len(podIPs)))},
	}
	labels := map[string]string{"app": rr.GetStatefulSetName()}
	objects := []runtime.Object{
		&appsv1.StatefulSet{
			ObjectMeta: metav1.ObjectMeta{Name: rr.GetStatefulSetName(), Namespace: "default"},
			Spec: appsv1.StatefulSetSpec{
				Replicas: ptr.To(int32(len(podIPs))),
				Selector: &metav1.LabelSelector{MatchLabels: labels},
			},
		},
	}
	for i, podIP := range podIPs {
		objects = append(objects, newLabeledRedisPod(fmt.Sprintf("%s-%d", rr.GetStatefulSetName(), i), labels, podIP, corev1.PodRunning, true))
	}
	return rr, objects
}
