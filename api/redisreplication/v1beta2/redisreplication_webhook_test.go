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
		{
			Name:      "failed-create-v1beta2-redisreplication-external-master-with-sentinel",
			Operation: admissionv1beta1.Create,
			Object: func(t *testing.T, uid string) []byte {
				t.Helper()
				replication := mkRedisReplication(uid)
				replication.Spec.ExternalMaster = &v1beta2.ExternalMaster{Host: "redis-master.primary.example.com"}
				replication.Spec.Sentinel = &v1beta2.Sentinel{Size: 3}
				return marshal(t, replication)
			},
			Check: webhook.ValidationWebhookFailed("externalMaster cannot be combined with sentinel"),
		},
		{
			Name:      "failed-create-v1beta2-redisreplication-external-master-missing-host",
			Operation: admissionv1beta1.Create,
			Object: func(t *testing.T, uid string) []byte {
				t.Helper()
				replication := mkRedisReplication(uid)
				replication.Spec.ExternalMaster = &v1beta2.ExternalMaster{Host: ""}
				return marshal(t, replication)
			},
			Check: webhook.ValidationWebhookFailed("host must be set when externalMaster is configured"),
		},
		{
			Name:      "success-create-v1beta2-redisreplication-external-master",
			Operation: admissionv1beta1.Create,
			Object: func(t *testing.T, uid string) []byte {
				t.Helper()
				replication := mkRedisReplication(uid)
				replication.Spec.ExternalMaster = &v1beta2.ExternalMaster{Host: "redis-master.primary.example.com"}
				return marshal(t, replication)
			},
			Check: webhook.ValidationWebhookSucceeded,
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
