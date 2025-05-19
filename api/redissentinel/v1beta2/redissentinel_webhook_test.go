package v1beta2_test

import (
	"encoding/json"
	"fmt"
	"testing"

	v1beta2 "github.com/OT-CONTAINER-KIT/redis-operator/api/redissentinel/v1beta2"
	"github.com/OT-CONTAINER-KIT/redis-operator/internal/testutil/webhook"
	"github.com/stretchr/testify/require"
	admissionv1beta1 "k8s.io/api/admission/v1beta1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/utils/ptr"
)

func TestRedisSentinelWebhook(t *testing.T) {
	cases := []webhook.ValidationWebhookTestCase{
		{
			Name:      "success-create-v1beta2-redissentinel-validate-clusterSize",
			Operation: admissionv1beta1.Create,
			Object: func(t *testing.T, uid string) []byte {
				t.Helper()
				sentinel := mkRedisSentinel(uid)
				sentinel.Spec.Size = ptr.To(int32(3))
				return marshal(t, sentinel)
			},
			Check: webhook.ValidationWebhookSucceeded,
		},

		{
			Name:      "failed-create-v1beta2-redissentinel-validate-clusterSize",
			Operation: admissionv1beta1.Create,
			Object: func(t *testing.T, uid string) []byte {
				t.Helper()
				sentinel := mkRedisSentinel(uid)
				sentinel.Spec.Size = ptr.To(int32(4))
				return marshal(t, sentinel)
			},
			Check: webhook.ValidationWebhookFailed("Redis Sentinel cluster size must be an odd number for proper leader election"),
		},
	}

	gvk := metav1.GroupVersionKind{
		Group:   "redis.redis.opstreelabs.in",
		Version: "v1beta2",
		Kind:    "RedisSentinel",
	}

	sentinel := &v1beta2.RedisSentinel{}
	webhook.RunValidationWebhookTests(t, gvk, sentinel, cases...)
}

func mkRedisSentinel(uid string) *v1beta2.RedisSentinel {
	return &v1beta2.RedisSentinel{
		ObjectMeta: metav1.ObjectMeta{
			Name: fmt.Sprintf("test-%s", uid),
			UID:  types.UID(fmt.Sprintf("test-%s", uid)),
		},
		Spec: v1beta2.RedisSentinelSpec{},
	}
}

func marshal(t *testing.T, obj interface{}) []byte {
	t.Helper()
	bytes, err := json.Marshal(obj)
	require.NoError(t, err)
	return bytes
}
