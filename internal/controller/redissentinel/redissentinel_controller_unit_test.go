package redissentinel

import (
	"context"
	"testing"
	"time"

	commonapi "github.com/OT-CONTAINER-KIT/redis-operator/api/common/v1beta2"
	rcvb2 "github.com/OT-CONTAINER-KIT/redis-operator/api/rediscluster/v1beta2"
	rrvb2 "github.com/OT-CONTAINER-KIT/redis-operator/api/redisreplication/v1beta2"
	rsvb2 "github.com/OT-CONTAINER-KIT/redis-operator/api/redissentinel/v1beta2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes/fake"
	"k8s.io/utils/ptr"
	clientfake "sigs.k8s.io/controller-runtime/pkg/client/fake"
)

func TestReconcileSentinelRequeuesWithoutTouchingSentinelsWhenNoReachableMaster(t *testing.T) {
	scheme := runtime.NewScheme()
	require.NoError(t, rrvb2.AddToScheme(scheme))

	rr := &rrvb2.RedisReplication{
		ObjectMeta: metav1.ObjectMeta{Name: "example-replication", Namespace: "default"},
		Spec:       rrvb2.RedisReplicationSpec{Size: ptr.To(int32(3))},
	}
	instance := &rsvb2.RedisSentinel{
		ObjectMeta: metav1.ObjectMeta{Name: "example", Namespace: "default"},
		Spec: rsvb2.RedisSentinelSpec{
			Size: ptr.To(int32(3)),
			RedisSentinelConfig: &rsvb2.RedisSentinelConfig{
				RedisSentinelConfig: commonapi.RedisSentinelConfig{
					RedisReplicationName: rr.Name,
					MasterGroupName:      "myMaster",
				},
			},
		},
	}
	healer := &fakeHealer{}
	r := &RedisSentinelReconciler{
		Client:    clientfake.NewClientBuilder().WithScheme(scheme).WithObjects(rr).Build(),
		K8sClient: fake.NewSimpleClientset(),
		Checker:   &fakeChecker{},
		Healer:    healer,
	}

	result, err := r.reconcileSentinel(context.Background(), instance)

	require.NoError(t, err)
	assert.Equal(t, 5*time.Second, result.RequeueAfter)
	assert.Empty(t, healer.monitoredMasters)
}

func TestRecreateSentinelResourcesAfterRoutingWasCleared(t *testing.T) {
	scheme := runtime.NewScheme()
	require.NoError(t, rrvb2.AddToScheme(scheme))
	rr := &rrvb2.RedisReplication{ObjectMeta: metav1.ObjectMeta{Name: "replication", Namespace: "default"}, Status: rrvb2.RedisReplicationStatus{ConnectionInfo: &rrvb2.ConnectionInfo{Host: "synthetic.invalid", Port: 6379}}}
	rs := &rsvb2.RedisSentinel{ObjectMeta: metav1.ObjectMeta{Name: "sentinel", Namespace: "default"}, Spec: rsvb2.RedisSentinelSpec{Size: ptr.To(int32(3)), RedisSentinelConfig: &rsvb2.RedisSentinelConfig{RedisSentinelConfig: commonapi.RedisSentinelConfig{RedisReplicationName: rr.Name, MasterGroupName: "myMaster"}}}}
	rs.Spec.Storage = &rsvb2.RedisSentinelStorage{VolumeClaimTemplate: corev1.PersistentVolumeClaim{Spec: corev1.PersistentVolumeClaimSpec{Resources: corev1.VolumeResourceRequirements{Requests: corev1.ResourceList{corev1.ResourceStorage: resource.MustParse("1Gi")}}}}}
	rs.Spec.PodSecurityContext = &corev1.PodSecurityContext{RunAsUser: ptr.To(int64(1000)), FSGroup: ptr.To(int64(1000))}
	k8s := fake.NewSimpleClientset()
	healer := &fakeHealer{}
	r := &RedisSentinelReconciler{Client: clientfake.NewClientBuilder().WithScheme(scheme).WithObjects(rr).Build(), K8sClient: k8s, Healer: healer}
	_, err := r.reconcileSentinel(context.Background(), rs)
	require.NoError(t, err)
	sts, err := k8s.AppsV1().StatefulSets("default").Get(context.Background(), rs.GetStatefulSetName(), metav1.GetOptions{})
	require.NoError(t, err)
	require.Equal(t, "init-config", sts.Spec.Template.Spec.InitContainers[0].Name)
	require.Empty(t, healer.monitoredMasters, "resource reconciliation must not initialize from cached routing status")
}

type fakeChecker struct {
	master corev1.Pod
}

func (f *fakeChecker) GetMasterFromReplication(context.Context, *rrvb2.RedisReplication) (corev1.Pod, error) {
	return f.master, nil
}

func (f *fakeChecker) GetPassword(context.Context, string, *commonapi.ExistingPasswordSecret) (string, error) {
	return "", nil
}

func (f *fakeChecker) CheckClusterSlotsAssigned(context.Context, *rcvb2.RedisCluster) (bool, error) {
	return true, nil
}

func (f *fakeChecker) GetReplicationKeyCounts(context.Context, *rrvb2.RedisReplication, []string) map[string]int64 {
	return nil
}

type fakeHealer struct {
	monitoredMasters []string
}

func (f *fakeHealer) SetRedisMasterLabel(context.Context, string, map[string]string, *commonapi.ExistingPasswordSecret, *commonapi.TLSConfig, string) error {
	return nil
}

func (f *fakeHealer) SentinelMonitor(_ context.Context, _ *rsvb2.RedisSentinel, master string) error {
	f.monitoredMasters = append(f.monitoredMasters, master)
	return nil
}

func (f *fakeHealer) SentinelSet(context.Context, *rsvb2.RedisSentinel, string) error {
	return nil
}

func (f *fakeHealer) SentinelReset(context.Context, *rsvb2.RedisSentinel, *rrvb2.RedisReplication) error {
	return nil
}

func (f *fakeHealer) UpdateRedisRoleLabel(context.Context, string, map[string]string, *commonapi.ExistingPasswordSecret, *commonapi.TLSConfig, string) error {
	return nil
}
