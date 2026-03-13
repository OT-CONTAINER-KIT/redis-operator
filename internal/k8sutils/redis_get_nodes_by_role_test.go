package k8sutils

import (
	"context"
	"errors"
	"testing"

	rrvb2 "github.com/OT-CONTAINER-KIT/redis-operator/api/redisreplication/v1beta2"
	"github.com/stretchr/testify/assert"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	k8sClientFake "k8s.io/client-go/kubernetes/fake"
	"k8s.io/utils/ptr"
)

func TestIsRedisPodProbeable(t *testing.T) {
	tests := []struct {
		name string
		pod  *corev1.Pod
		want bool
	}{
		{
			name: "nil pod",
			pod:  nil,
			want: false,
		},
		{
			name: "pod pending",
			pod:  newPendingRedisPod("example-replication-0"),
			want: false,
		},
		{
			name: "pod running but not ready",
			pod:  newRunningNotReadyRedisPod("example-replication-0", "10.0.0.10"),
			want: false,
		},
		{
			name: "pod running and ready without ip",
			pod:  newReadyRedisPod("example-replication-0", ""),
			want: false,
		},
		{
			name: "pod running and ready with ip",
			pod:  newReadyRedisPod("example-replication-0", "10.0.0.10"),
			want: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, IsRedisPodProbeable(tt.pod))
		})
	}
}

func TestGetRedisNodesByRoleSkipsUnprobeablePods(t *testing.T) {
	tests := []struct {
		name string
		pod  runtime.Object
	}{
		{
			name: "pod not found",
			pod:  nil,
		},
		{
			name: "pod pending",
			pod:  newPendingRedisPod("example-replication-2"),
		},
		{
			name: "pod running but not ready",
			pod:  newRunningNotReadyRedisPod("example-replication-2", "10.0.0.12"),
		},
		{
			name: "pod running and ready without ip",
			pod:  newReadyRedisPod("example-replication-2", ""),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			objects := []runtime.Object{
				newRedisReplicationStatefulSet(),
				newReadyRedisPod("example-replication-0", "10.0.0.10"),
				newReadyRedisPod("example-replication-1", "10.0.0.11"),
			}
			if tt.pod != nil {
				objects = append(objects, tt.pod)
			}
			client := k8sClientFake.NewSimpleClientset(objects...)

			var probedPods []string
			nodes, err := getRedisNodesByRole(context.Background(), client, newRedisReplication(), "master", func(_ context.Context, pod *corev1.Pod) (string, error) {
				probedPods = append(probedPods, pod.Name)
				if pod.Name == "example-replication-0" {
					return "master", nil
				}
				return "slave", nil
			})

			assert.NoError(t, err)
			assert.Equal(t, []string{"example-replication-0"}, nodes)
			assert.ElementsMatch(t, []string{"example-replication-0", "example-replication-1"}, probedPods)
		})
	}
}

func TestGetRedisNodesByRoleFailsWhenReadyPodProbeFails(t *testing.T) {
	client := k8sClientFake.NewSimpleClientset(
		newRedisReplicationStatefulSet(),
		newReadyRedisPod("example-replication-0", "10.0.0.10"),
		newReadyRedisPod("example-replication-1", "10.0.0.11"),
		newReadyRedisPod("example-replication-2", "10.0.0.12"),
	)

	_, err := getRedisNodesByRole(context.Background(), client, newRedisReplication(), "master", func(_ context.Context, pod *corev1.Pod) (string, error) {
		if pod.Name == "example-replication-1" {
			return "", errors.New("probe failed")
		}
		return "slave", nil
	})

	assert.ErrorContains(t, err, "probe failed")
}

func TestGetRedisNodesByRoleCompleteTopology(t *testing.T) {
	client := k8sClientFake.NewSimpleClientset(
		newRedisReplicationStatefulSet(),
		newReadyRedisPod("example-replication-0", "10.0.0.10"),
		newReadyRedisPod("example-replication-1", "10.0.0.11"),
		newReadyRedisPod("example-replication-2", "10.0.0.12"),
	)

	nodes, err := getRedisNodesByRole(context.Background(), client, newRedisReplication(), "slave", func(_ context.Context, pod *corev1.Pod) (string, error) {
		if pod.Name == "example-replication-0" {
			return "master", nil
		}
		return "slave", nil
	})

	assert.NoError(t, err)
	assert.ElementsMatch(t, []string{"example-replication-1", "example-replication-2"}, nodes)
}

func newRedisReplication() *rrvb2.RedisReplication {
	size := int32(3)
	return &rrvb2.RedisReplication{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "example-replication",
			Namespace: "default",
		},
		Spec: rrvb2.RedisReplicationSpec{
			Size: ptr.To(size),
		},
	}
}

func newRedisReplicationStatefulSet() *appsv1.StatefulSet {
	return &appsv1.StatefulSet{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "example-replication",
			Namespace: "default",
		},
	}
}

func newReadyRedisPod(name, podIP string) *corev1.Pod {
	return &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: "default",
		},
		Status: corev1.PodStatus{
			Phase: corev1.PodRunning,
			PodIP: podIP,
			Conditions: []corev1.PodCondition{
				{
					Type:   corev1.PodReady,
					Status: corev1.ConditionTrue,
				},
			},
		},
	}
}

func newRunningNotReadyRedisPod(name, podIP string) *corev1.Pod {
	return &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: "default",
		},
		Status: corev1.PodStatus{
			Phase: corev1.PodRunning,
			PodIP: podIP,
			Conditions: []corev1.PodCondition{
				{
					Type:   corev1.PodReady,
					Status: corev1.ConditionFalse,
				},
			},
		},
	}
}

func newPendingRedisPod(name string) *corev1.Pod {
	return &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: "default",
		},
		Status: corev1.PodStatus{
			Phase: corev1.PodPending,
		},
	}
}
