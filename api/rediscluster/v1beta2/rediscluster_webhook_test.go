package v1beta2_test

import (
	"encoding/json"
	"fmt"
	"testing"

	common "github.com/OT-CONTAINER-KIT/redis-operator/api/common/v1beta2"
	v1beta2 "github.com/OT-CONTAINER-KIT/redis-operator/api/rediscluster/v1beta2"
	"github.com/OT-CONTAINER-KIT/redis-operator/internal/testutil/webhook"
	"github.com/stretchr/testify/require"
	admissionv1beta1 "k8s.io/api/admission/v1beta1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/utils/ptr"
)

func TestRedisClusterWebhook(t *testing.T) {
	cases := []webhook.ValidationWebhookTestCase{
		{
			Name:      "success-create-v1beta2-rediscluster-validate-clusterSize-3",
			Operation: admissionv1beta1.Create,
			Object: func(t *testing.T, uid string) []byte {
				t.Helper()
				cluster := mkRedisCluster(uid)
				cluster.Spec.ClusterSize = ptr.To(int32(3))
				return marshal(t, cluster)
			},
			Check: webhook.ValidationWebhookSucceeded,
		},
		{
			Name:      "failed-create-v1beta2-rediscluster-validate-clusterSize-2",
			Operation: admissionv1beta1.Create,
			Object: func(t *testing.T, uid string) []byte {
				t.Helper()
				cluster := mkRedisCluster(uid)
				cluster.Spec.ClusterSize = ptr.To(int32(2))
				return marshal(t, cluster)
			},
			Check: webhook.ValidationWebhookFailed("Redis cluster must have at least 3 shards"),
		},
		{
			Name:      "failed-create-v1beta2-rediscluster-acl-both-sources",
			Operation: admissionv1beta1.Create,
			Object: func(t *testing.T, uid string) []byte {
				t.Helper()
				cluster := mkRedisCluster(uid)
				cluster.Spec.ClusterSize = ptr.To(int32(3))
				cluster.Spec.ACL = mkACLWithBothSources()
				return marshal(t, cluster)
			},
			Check: webhook.ValidationWebhookFailed("only one of 'secret' or 'persistentVolumeClaim' can be specified"),
		},
	}

	gvk := metav1.GroupVersionKind{
		Group:   "redis.redis.opstreelabs.in",
		Version: "v1beta2",
		Kind:    "RedisCluster",
	}

	cluster := &v1beta2.RedisCluster{}
	webhook.RunValidationWebhookTests(t, gvk, cluster, cases...)
}

func mkRedisCluster(uid string) *v1beta2.RedisCluster {
	return &v1beta2.RedisCluster{
		ObjectMeta: metav1.ObjectMeta{
			Name: fmt.Sprintf("test-%s", uid),
			UID:  types.UID(fmt.Sprintf("test-%s", uid)),
		},
		Spec: v1beta2.RedisClusterSpec{},
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
			SecretName: "redis-acl-secret",
		},
		PersistentVolumeClaim: ptr.To("redis-acl-pvc"),
	}
}
