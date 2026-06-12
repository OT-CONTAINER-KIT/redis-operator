package v1beta2_test

import (
	"encoding/json"
	"fmt"
	"testing"

	common "github.com/OT-CONTAINER-KIT/redis-operator/api/common/v1beta2"
	v1beta2 "github.com/OT-CONTAINER-KIT/redis-operator/api/redisreplication/v1beta2"
	"github.com/OT-CONTAINER-KIT/redis-operator/internal/testutil/webhook"
	"github.com/stretchr/testify/require"
	admissionv1beta1 "k8s.io/api/admission/v1beta1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/utils/ptr"
)

func TestRedisReplicationWebhook(t *testing.T) {
	cases := []webhook.ValidationWebhookTestCase{
		{
			Name:      "failed-create-v1beta2-redisreplication-acl-both-sources",
			Operation: admissionv1beta1.Create,
			Object: func(t *testing.T, uid string) []byte {
				t.Helper()
				replication := mkRedisReplication(uid)
				replication.Spec.ACL = mkACLWithBothSources()
				return marshal(t, replication)
			},
			Check: webhook.ValidationWebhookFailed("only one of 'secret' or 'persistentVolumeClaim' can be specified"),
		},
	}

	gvk := metav1.GroupVersionKind{
		Group:   "redis.redis.opstreelabs.in",
		Version: "v1beta2",
		Kind:    "RedisReplication",
	}

	replication := &v1beta2.RedisReplication{}
	webhook.RunValidationWebhookTests(t, gvk, replication, cases...)
}

func mkRedisReplication(uid string) *v1beta2.RedisReplication {
	return &v1beta2.RedisReplication{
		ObjectMeta: metav1.ObjectMeta{
			Name: fmt.Sprintf("test-%s", uid),
			UID:  types.UID(fmt.Sprintf("test-%s", uid)),
		},
		Spec: v1beta2.RedisReplicationSpec{},
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
