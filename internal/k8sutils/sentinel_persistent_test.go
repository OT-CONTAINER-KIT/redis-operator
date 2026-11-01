package k8sutils

import (
	"context"
	"fmt"
	"testing"

	commonapi "github.com/OT-CONTAINER-KIT/redis-operator/api/common/v1beta2"
	rsvb2 "github.com/OT-CONTAINER-KIT/redis-operator/api/redissentinel/v1beta2"
	"github.com/OT-CONTAINER-KIT/redis-operator/internal/controller/common"
	"github.com/OT-CONTAINER-KIT/redis-operator/internal/features"
	"github.com/stretchr/testify/require"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	kubernetesfake "k8s.io/client-go/kubernetes/fake"
	"k8s.io/utils/ptr"
)

func persistentSentinelFixture() *rsvb2.RedisSentinel {
	return &rsvb2.RedisSentinel{TypeMeta: metav1.TypeMeta{APIVersion: rsvb2.GroupVersion.String(), Kind: "RedisSentinel"}, ObjectMeta: metav1.ObjectMeta{Name: "sentinel", Namespace: "default", UID: "synthetic-resource"}, Spec: rsvb2.RedisSentinelSpec{Size: ptr.To(int32(3)), KubernetesConfig: commonapi.KubernetesConfig{Image: "synthetic-sentinel-image"}, PodSecurityContext: &corev1.PodSecurityContext{RunAsUser: ptr.To(int64(1000)), FSGroup: ptr.To(int64(1000))}, RedisSentinelConfig: &rsvb2.RedisSentinelConfig{RedisSentinelConfig: commonapi.RedisSentinelConfig{RedisReplicationName: "replication", MasterGroupName: "myMaster"}}, Storage: &rsvb2.RedisSentinelStorage{VolumeClaimTemplate: corev1.PersistentVolumeClaim{Spec: corev1.PersistentVolumeClaimSpec{Resources: corev1.VolumeResourceRequirements{Requests: corev1.ResourceList{corev1.ResourceStorage: resource.MustParse("1Gi")}}}}}}}
}

func TestStandaloneSentinelUsesRetainedConfigurationClaims(t *testing.T) {
	original := features.Enabled(features.GenerateConfigInInitContainer)
	t.Cleanup(func() {
		require.NoError(t, features.MutableFeatureGate.Set(fmt.Sprintf("GenerateConfigInInitContainer=%t", original)))
	})
	for _, global := range []bool{false, true} {
		require.NoError(t, features.MutableFeatureGate.Set(fmt.Sprintf("GenerateConfigInInitContainer=%t", global)))
		cr := persistentSentinelFixture()
		k8s := kubernetesfake.NewSimpleClientset()
		require.NoError(t, CreateRedisSentinel(context.Background(), k8s, cr, k8s, nil))
		sts, err := k8s.AppsV1().StatefulSets(cr.Namespace).Get(context.Background(), cr.GetStatefulSetName(), metav1.GetOptions{})
		require.NoError(t, err)
		require.Len(t, sts.Spec.VolumeClaimTemplates, 1)
		require.Equal(t, common.VolumeNameSentinelState, sts.Spec.VolumeClaimTemplates[0].Name)
		require.Equal(t, appsv1.RetainPersistentVolumeClaimRetentionPolicyType, sts.Spec.PersistentVolumeClaimRetentionPolicy.WhenDeleted)
		require.Equal(t, appsv1.RetainPersistentVolumeClaimRetentionPolicyType, sts.Spec.PersistentVolumeClaimRetentionPolicy.WhenScaled)
		for _, volume := range sts.Spec.Template.Spec.Volumes {
			require.Nil(t, volume.EmptyDir)
		}
		for _, container := range []corev1.Container{sts.Spec.Template.Spec.Containers[0], sts.Spec.Template.Spec.InitContainers[0]} {
			found := false
			for _, mount := range container.VolumeMounts {
				if mount.MountPath == "/etc/redis" {
					require.Equal(t, common.VolumeNameSentinelState, mount.Name)
					require.False(t, mount.ReadOnly)
					found = true
				}
			}
			require.True(t, found)
		}
		require.Contains(t, sts.Spec.Template.Spec.InitContainers[0].Args, "--persistent-sentinel-state")
	}
}

func TestPersistentSentinelRejectsUnsafeStorageChanges(t *testing.T) {
	cr := persistentSentinelFixture()
	cr.Spec.Storage = nil
	require.Error(t, validateStandaloneSentinelStorage(cr))
	cr = persistentSentinelFixture()
	cr.Spec.KubernetesConfig.PersistentVolumeClaimRetentionPolicy = &appsv1.StatefulSetPersistentVolumeClaimRetentionPolicy{WhenDeleted: appsv1.DeletePersistentVolumeClaimRetentionPolicyType}
	require.Error(t, validateStandaloneSentinelStorage(cr))
	cr = persistentSentinelFixture()
	cr.Spec.InitContainer = &commonapi.InitContainer{Enabled: ptr.To(true)}
	require.Error(t, validateStandaloneSentinelStorage(cr))
	cr = persistentSentinelFixture()
	cr.Spec.Storage.VolumeClaimTemplate.Spec.DataSource = &corev1.TypedLocalObjectReference{Kind: "PersistentVolumeClaim", Name: "shared-state"}
	require.Error(t, validateStandaloneSentinelStorage(cr))
	cr = persistentSentinelFixture()
	k8s := kubernetesfake.NewSimpleClientset(&appsv1.StatefulSet{ObjectMeta: metav1.ObjectMeta{Name: cr.GetStatefulSetName(), Namespace: cr.Namespace}})
	require.ErrorContains(t, CreateRedisSentinel(context.Background(), k8s, cr, k8s, nil), "explicit")
}
