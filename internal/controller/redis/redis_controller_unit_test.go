package redis

import (
	"context"
	"testing"

	commonapi "github.com/OT-CONTAINER-KIT/redis-operator/api/common/v1beta2"
	rvb2 "github.com/OT-CONTAINER-KIT/redis-operator/api/redis/v1beta2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	clientfake "sigs.k8s.io/controller-runtime/pkg/client/fake"
)

func newRedisInstanceForTest() *rvb2.Redis {
	return &rvb2.Redis{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "example-redis",
			Namespace: "default",
		},
		Spec: rvb2.RedisSpec{
			KubernetesConfig: commonapi.KubernetesConfig{
				Image: "redis:7",
			},
		},
	}
}

func TestRedisUpdateStatusWritesAndSkipsWhenUnchanged(t *testing.T) {
	scheme := runtime.NewScheme()
	require.NoError(t, rvb2.AddToScheme(scheme))

	instance := newRedisInstanceForTest()
	ctrlClient := clientfake.NewClientBuilder().
		WithScheme(scheme).
		WithStatusSubresource(instance).
		WithObjects(instance.DeepCopy()).
		Build()

	r := &Reconciler{Client: ctrlClient}

	status := rvb2.RedisStatus{
		State:  rvb2.RedisReady,
		Reason: rvb2.ReadyRedisReason,
	}

	// First update writes the status and bumps the resourceVersion.
	fresh := &rvb2.Redis{}
	require.NoError(t, ctrlClient.Get(context.Background(), client.ObjectKeyFromObject(instance), fresh))
	require.NoError(t, r.updateStatus(context.Background(), fresh, status))

	written := &rvb2.Redis{}
	require.NoError(t, ctrlClient.Get(context.Background(), client.ObjectKeyFromObject(instance), written))
	assert.Equal(t, rvb2.RedisReady, written.Status.State)
	assert.Equal(t, rvb2.ReadyRedisReason, written.Status.Reason)
	rvAfterWrite := written.ResourceVersion

	// Second update with an identical status is a no-op (reflect.DeepEqual guard),
	// so the resourceVersion must not change.
	require.NoError(t, r.updateStatus(context.Background(), written, status))
	noop := &rvb2.Redis{}
	require.NoError(t, ctrlClient.Get(context.Background(), client.ObjectKeyFromObject(instance), noop))
	assert.Equal(t, rvAfterWrite, noop.ResourceVersion, "identical status update should be skipped")
}
