package k8sutils

import (
	"context"
	"testing"

	commonapi "github.com/OT-CONTAINER-KIT/redis-operator/api/common/v1beta2"
	rrvb2 "github.com/OT-CONTAINER-KIT/redis-operator/api/redisreplication/v1beta2"
	"github.com/OT-CONTAINER-KIT/redis-operator/internal/controller/common"
	"github.com/stretchr/testify/require"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	kubernetesfake "k8s.io/client-go/kubernetes/fake"
	"k8s.io/utils/ptr"
)

func TestActivatedStandaloneSentinelRejectsEmbeddedModeBeforeWorkloadChanges(t *testing.T) {
	for name, annotations := range map[string]map[string]string{
		"ownership": {common.SentinelOwnershipAnnotation: "sentinel"},
		"startup":   {common.SentinelStartupAnnotation: "enabled"},
		"both":      {common.SentinelOwnershipAnnotation: "sentinel", common.SentinelStartupAnnotation: "enabled"},
	} {
		t.Run(name, func(t *testing.T) {
			cr := &rrvb2.RedisReplication{
				ObjectMeta: metav1.ObjectMeta{Name: "replication", Namespace: "default", UID: "synthetic-owner", Annotations: annotations},
				Spec: rrvb2.RedisReplicationSpec{
					Size:             ptr.To(int32(3)),
					KubernetesConfig: commonapi.KubernetesConfig{Image: "synthetic-redis-image"},
					Sentinel:         &rrvb2.Sentinel{Size: 3},
				},
			}
			original := &appsv1.StatefulSet{
				ObjectMeta: metav1.ObjectMeta{Name: cr.Name, Namespace: cr.Namespace},
				Spec: appsv1.StatefulSetSpec{Template: corev1.PodTemplateSpec{
					ObjectMeta: metav1.ObjectMeta{Annotations: map[string]string{"synthetic-protected-template": "retained"}},
				}},
			}
			k8s := kubernetesfake.NewSimpleClientset(original.DeepCopy())
			err := CreateReplicationRedis(context.Background(), cr, k8s, nil)
			require.ErrorContains(t, err, "standalone Sentinel")
			for _, action := range k8s.Actions() {
				require.NotContains(t, []string{"create", "update", "patch", "delete", "delete-collection"}, action.GetVerb())
			}
			retained, err := k8s.AppsV1().StatefulSets(cr.Namespace).Get(context.Background(), cr.Name, metav1.GetOptions{})
			require.NoError(t, err)
			require.Equal(t, original.Spec.Template, retained.Spec.Template)
		})
	}
}

func TestSentinelRecoveryBookkeepingDoesNotRollWorkloads(t *testing.T) {
	cr := &rrvb2.RedisReplication{
		ObjectMeta: metav1.ObjectMeta{Name: "replication", Namespace: "default", UID: "synthetic-owner"},
		Spec: rrvb2.RedisReplicationSpec{
			Size:             ptr.To(int32(3)),
			KubernetesConfig: commonapi.KubernetesConfig{Image: "synthetic-redis-image"},
		},
	}
	k8s := kubernetesfake.NewSimpleClientset()
	require.NoError(t, CreateReplicationRedis(context.Background(), cr, k8s, nil))
	before, err := k8s.AppsV1().StatefulSets(cr.Namespace).Get(context.Background(), cr.Name, metav1.GetOptions{})
	require.NoError(t, err)
	cr.Annotations = map[string]string{common.SentinelReplacementRecoveryAnnotation: `{"version":1,"owner":"synthetic-owner","attempts":1}`}
	require.NoError(t, CreateReplicationRedis(context.Background(), cr, k8s, nil))
	after, err := k8s.AppsV1().StatefulSets(cr.Namespace).Get(context.Background(), cr.Name, metav1.GetOptions{})
	require.NoError(t, err)
	require.Equal(t, before.Spec, after.Spec)
	require.NotContains(t, after.Annotations, common.SentinelReplacementRecoveryAnnotation)
	require.NotContains(t, after.Spec.Template.Annotations, common.SentinelReplacementRecoveryAnnotation)
}
