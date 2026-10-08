package redis

import (
	"context"
	"errors"
	"fmt"
	"testing"

	rrvb2 "github.com/OT-CONTAINER-KIT/redis-operator/api/redisreplication/v1beta2"
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
