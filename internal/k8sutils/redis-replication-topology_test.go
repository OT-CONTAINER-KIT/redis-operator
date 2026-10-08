package k8sutils

import (
	"context"
	"errors"
	"testing"

	rrvb2 "github.com/OT-CONTAINER-KIT/redis-operator/api/redisreplication/v1beta2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	k8sClientFake "k8s.io/client-go/kubernetes/fake"
	k8sTesting "k8s.io/client-go/testing"
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

// Every probe shares the reconcile context, so a cancellation fails all of them.
// Skipping them would report a partial topology as a complete one.
func TestGetRedisReplicationTopologyPropagatesContextCancellation(t *testing.T) {
	client := k8sClientFake.NewSimpleClientset(
		newRedisReplicationStatefulSet(),
		newReadyRedisPod("example-replication-0", "10.0.0.10"),
		newReadyRedisPod("example-replication-1", "10.0.0.11"),
		newReadyRedisPod("example-replication-2", "10.0.0.12"),
	)

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	topology, err := getRedisReplicationTopology(ctx, client, newRedisReplication(), func(ctx context.Context, _ *corev1.Pod) (string, error) {
		return "", ctx.Err()
	})

	require.ErrorIs(t, err, context.Canceled)
	assert.Equal(t, RedisReplicationTopology{}, topology)
}

func TestGetRedisReplicationTopologyReportsUnobservedPods(t *testing.T) {
	tests := []struct {
		name  string
		pod   runtime.Object
		probe func(pod *corev1.Pod) (string, error)
	}{
		{
			name: "pod not found",
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
		{
			name: "ready pod fails the probe",
			pod:  newReadyRedisPod("example-replication-2", "10.0.0.12"),
			probe: func(*corev1.Pod) (string, error) {
				return "", errors.New("probe failed")
			},
		},
		{
			name: "ready pod reports no role",
			pod:  newReadyRedisPod("example-replication-2", "10.0.0.12"),
			probe: func(*corev1.Pod) (string, error) {
				return "", nil
			},
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

			topology, err := getRedisReplicationTopology(context.Background(), client, newRedisReplication(), func(_ context.Context, pod *corev1.Pod) (string, error) {
				switch pod.Name {
				case "example-replication-0":
					return "master", nil
				case "example-replication-1":
					return "slave", nil
				default:
					require.NotNil(t, tt.probe, "pod %s should not have been probed", pod.Name)
					return tt.probe(pod)
				}
			})

			require.NoError(t, err)
			assert.Equal(t, []string{"example-replication-0"}, topology.Masters)
			assert.Equal(t, []string{"example-replication-1"}, topology.Slaves)
			assert.Equal(t, []string{"example-replication-2"}, topology.Unobserved)
			assert.False(t, topology.Complete())
			assert.Equal(t, 2, topology.Observed())
		})
	}
}

func TestGetRedisReplicationTopologyIsCompleteWhenEveryPodAnswers(t *testing.T) {
	client := k8sClientFake.NewSimpleClientset(
		newRedisReplicationStatefulSet(),
		newReadyRedisPod("example-replication-0", "10.0.0.10"),
		newReadyRedisPod("example-replication-1", "10.0.0.11"),
		newReadyRedisPod("example-replication-2", "10.0.0.12"),
	)

	topology, err := getRedisReplicationTopology(context.Background(), client, newRedisReplication(), func(_ context.Context, pod *corev1.Pod) (string, error) {
		if pod.Name == "example-replication-1" {
			return "master", nil
		}
		return "slave", nil
	})

	require.NoError(t, err)
	assert.Equal(t, []string{"example-replication-1"}, topology.Masters)
	assert.Equal(t, []string{"example-replication-0", "example-replication-2"}, topology.Slaves)
	assert.Empty(t, topology.Unobserved)
	assert.True(t, topology.Complete())
	assert.Equal(t, 3, topology.Observed())
}

// Only NotFound is a fact about the pod; any other API failure is a failure of
// the sweep and must not be reported as a partial topology.
func TestGetRedisReplicationTopologyFailsWhenPodCannotBeFetched(t *testing.T) {
	client := k8sClientFake.NewSimpleClientset(
		newRedisReplicationStatefulSet(),
		newReadyRedisPod("example-replication-0", "10.0.0.10"),
		newReadyRedisPod("example-replication-1", "10.0.0.11"),
		newReadyRedisPod("example-replication-2", "10.0.0.12"),
	)
	client.PrependReactor("get", "pods", func(action k8sTesting.Action) (bool, runtime.Object, error) {
		if action.(k8sTesting.GetAction).GetName() == "example-replication-1" {
			return true, nil, apierrors.NewInternalError(errors.New("etcd unavailable"))
		}
		return false, nil, nil
	})

	topology, err := getRedisReplicationTopology(context.Background(), client, newRedisReplication(), func(context.Context, *corev1.Pod) (string, error) {
		return "master", nil
	})

	require.Error(t, err)
	assert.True(t, apierrors.IsInternalError(err))
	assert.Equal(t, RedisReplicationTopology{}, topology)
}

func TestRedisReplicationTopologyByRole(t *testing.T) {
	topology := RedisReplicationTopology{
		Masters:    []string{"example-replication-1"},
		Slaves:     []string{"example-replication-0", "example-replication-2"},
		Unobserved: []string{"example-replication-3"},
	}

	assert.Equal(t, []string{"example-replication-1"}, topology.byRole("master"))
	assert.Equal(t, []string{"example-replication-0", "example-replication-2"}, topology.byRole("slave"))
	assert.Nil(t, topology.byRole("sentinel"))
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
