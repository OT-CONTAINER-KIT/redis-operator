package v1beta2_test

import (
	"encoding/json"
	"fmt"
	"testing"

	common "github.com/OT-CONTAINER-KIT/redis-operator/api/common/v1beta2"
	v1beta2 "github.com/OT-CONTAINER-KIT/redis-operator/api/redis/v1beta2"
	"github.com/OT-CONTAINER-KIT/redis-operator/internal/testutil/webhook"
	"github.com/stretchr/testify/require"
	admissionv1beta1 "k8s.io/api/admission/v1beta1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/utils/ptr"
)

func TestRedisWebhook(t *testing.T) {
	cases := []webhook.ValidationWebhookTestCase{
		{
			Name:      "failed-create-v1beta2-redis-acl-both-sources",
			Operation: admissionv1beta1.Create,
			Object: func(t *testing.T, uid string) []byte {
				t.Helper()
				redis := mkRedis(uid)
				redis.Spec.ACL = mkACLWithBothSources()
				return marshal(t, redis)
			},
			Check: webhook.ValidationWebhookFailed("only one of 'secret' or 'persistentVolumeClaim' can be specified"),
		},
	}

	gvk := metav1.GroupVersionKind{
		Group:   "redis.redis.opstreelabs.in",
		Version: "v1beta2",
		Kind:    "Redis",
	}

	redis := &v1beta2.Redis{}
	webhook.RunValidationWebhookTests(t, gvk, redis, cases...)
}

func mkRedis(uid string) *v1beta2.Redis {
	return &v1beta2.Redis{
		ObjectMeta: metav1.ObjectMeta{
			Name: fmt.Sprintf("test-%s", uid),
			UID:  types.UID(fmt.Sprintf("test-%s", uid)),
		},
		Spec: v1beta2.RedisSpec{},
	}
}

func marshal(t *testing.T, obj interface{}) []byte {
	t.Helper()
	bytes, err := json.Marshal(obj)
	require.NoError(t, err)
	return bytes
}

func mkACLWithBothSources() *common.ACLConfig {
	return &common.ACLConfig{
		Secret: &corev1.SecretVolumeSource{
			SecretName: "test-secret",
		},
		PersistentVolumeClaim: ptr.To("test-pvc"),
	}
}
