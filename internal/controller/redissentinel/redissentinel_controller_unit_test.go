package redissentinel

import (
	"context"
	"testing"

	commonapi "github.com/OT-CONTAINER-KIT/redis-operator/api/common/v1beta2"
	rsvb2 "github.com/OT-CONTAINER-KIT/redis-operator/api/redissentinel/v1beta2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes/fake"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"
	clientfake "sigs.k8s.io/controller-runtime/pkg/client/fake"
)

// fakeSentinelStatefulSet implements the k8sutils.StatefulSet interface with
// configurable ready/desired replica counts so the status state machine can be
// exercised without a live cluster.
type fakeSentinelStatefulSet struct {
	ready   int32
	desired int32
}

func (f fakeSentinelStatefulSet) IsStatefulSetReady(context.Context, string, string) bool {
	return f.desired > 0 && f.ready == f.desired
}

func (f fakeSentinelStatefulSet) GetStatefulSetReplicas(context.Context, string, string) int32 {
	return f.desired
}

func (f fakeSentinelStatefulSet) GetStatefulSetReadyReplicas(context.Context, string, string) int32 {
	return f.ready
}

func newSentinelInstanceForTest(size *int32) *rsvb2.RedisSentinel {
	return &rsvb2.RedisSentinel{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "example-sentinel",
			Namespace: "default",
		},
		Spec: rsvb2.RedisSentinelSpec{
			Size: size,
			KubernetesConfig: commonapi.KubernetesConfig{
				Image: "redis-sentinel:7",
			},
		},
	}
}

func TestReconcileSentinelStatusStateClassification(t *testing.T) {
	scheme := runtime.NewScheme()
	require.NoError(t, rsvb2.AddToScheme(scheme))

	tests := []struct {
		name          string
		size          *int32
		ready         int32
		desired       int32
		wantState     rsvb2.RedisSentinelState
		wantReason    string
		wantReadyReps int32
	}{
		{
			name:          "no ready pods is initializing",
			size:          ptr.To(int32(3)),
			ready:         0,
			desired:       3,
			wantState:     rsvb2.RedisSentinelInitializing,
			wantReason:    rsvb2.InitializingSentinelReason,
			wantReadyReps: 0,
		},
		{
			name:          "partial ready pods is failed",
			size:          ptr.To(int32(3)),
			ready:         2,
			desired:       3,
			wantState:     rsvb2.RedisSentinelFailed,
			wantReason:    rsvb2.FailedSentinelReason,
			wantReadyReps: 2,
		},
		{
			name:          "all pods ready is ready",
			size:          ptr.To(int32(3)),
			ready:         3,
			desired:       3,
			wantState:     rsvb2.RedisSentinelReady,
			wantReason:    rsvb2.ReadySentinelReason,
			wantReadyReps: 3,
		},
		{
			name:          "nil size with no ready pods does not panic and is initializing",
			size:          nil,
			ready:         0,
			desired:       0,
			wantState:     rsvb2.RedisSentinelInitializing,
			wantReason:    rsvb2.InitializingSentinelReason,
			wantReadyReps: 0,
		},
		{
			name:          "nil size falls back to statefulset desired and is ready",
			size:          nil,
			ready:         3,
			desired:       3,
			wantState:     rsvb2.RedisSentinelReady,
			wantReason:    rsvb2.ReadySentinelReason,
			wantReadyReps: 3,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			instance := newSentinelInstanceForTest(tt.size)
			ctrlClient := clientfake.NewClientBuilder().
				WithScheme(scheme).
				WithStatusSubresource(instance).
				WithObjects(instance.DeepCopy()).
				Build()

			r := &RedisSentinelReconciler{
				Client:      ctrlClient,
				StatefulSet: fakeSentinelStatefulSet{ready: tt.ready, desired: tt.desired},
				K8sClient:   fake.NewSimpleClientset(),
			}

			fresh := &rsvb2.RedisSentinel{}
			require.NoError(t, ctrlClient.Get(context.Background(), client.ObjectKeyFromObject(instance), fresh))

			_, err := r.reconcileStatus(context.Background(), fresh)
			require.NoError(t, err)

			updated := &rsvb2.RedisSentinel{}
			require.NoError(t, ctrlClient.Get(context.Background(), client.ObjectKeyFromObject(instance), updated))
			assert.Equal(t, tt.wantState, updated.Status.State)
			assert.Equal(t, tt.wantReason, updated.Status.Reason)
			assert.Equal(t, tt.wantReadyReps, updated.Status.ReadyReplicas)
		})
	}
}

func TestSentinelUpdateStatusSkipsWriteWhenUnchanged(t *testing.T) {
	scheme := runtime.NewScheme()
	require.NoError(t, rsvb2.AddToScheme(scheme))

	instance := newSentinelInstanceForTest(ptr.To(int32(3)))
	ctrlClient := clientfake.NewClientBuilder().
		WithScheme(scheme).
		WithStatusSubresource(instance).
		WithObjects(instance.DeepCopy()).
		Build()

	r := &RedisSentinelReconciler{
		Client:      ctrlClient,
		StatefulSet: fakeSentinelStatefulSet{ready: 3, desired: 3},
		K8sClient:   fake.NewSimpleClientset(),
	}

	status := rsvb2.RedisSentinelStatus{
		State:         rsvb2.RedisSentinelReady,
		Reason:        rsvb2.ReadySentinelReason,
		ReadyReplicas: 3,
	}

	// First update writes the status and bumps the resourceVersion.
	fresh := &rsvb2.RedisSentinel{}
	require.NoError(t, ctrlClient.Get(context.Background(), client.ObjectKeyFromObject(instance), fresh))
	require.NoError(t, r.updateStatus(context.Background(), fresh, status))

	written := &rsvb2.RedisSentinel{}
	require.NoError(t, ctrlClient.Get(context.Background(), client.ObjectKeyFromObject(instance), written))
	assert.Equal(t, rsvb2.RedisSentinelReady, written.Status.State)
	rvAfterWrite := written.ResourceVersion

	// Second update with an identical status is a no-op (reflect.DeepEqual guard),
	// so the resourceVersion must not change.
	require.NoError(t, r.updateStatus(context.Background(), written, status))
	noop := &rsvb2.RedisSentinel{}
	require.NoError(t, ctrlClient.Get(context.Background(), client.ObjectKeyFromObject(instance), noop))
	assert.Equal(t, rvAfterWrite, noop.ResourceVersion, "identical status update should be skipped")
}
