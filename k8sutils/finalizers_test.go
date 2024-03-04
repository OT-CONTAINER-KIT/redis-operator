package k8sutils

import (
	"context"
	"fmt"
	"testing"

	"github.com/OT-CONTAINER-KIT/redis-operator/api/v1beta2"
	mockClient "github.com/OT-CONTAINER-KIT/redis-operator/mocks/client"
	"github.com/go-logr/logr/testr"
	"github.com/stretchr/testify/assert"
	corev1 "k8s.io/api/core/v1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"

	// "k8s.io/apimachinery/pkg/types"
	// utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	k8sClientFake "k8s.io/client-go/kubernetes/fake"
	"k8s.io/utils/pointer"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// func TestHandleRedisFinalizer(t *testing.T) {
// 	cr := &v1beta2.Redis{
// 		TypeMeta: metav1.TypeMeta{
// 			Kind:       "Redis",
// 			APIVersion: "redis.opstreelabs.in/v1beta2",
// 		},
// 		ObjectMeta: metav1.ObjectMeta{
// 			Name:              "test-redis",
// 			Namespace:         "default",
// 			DeletionTimestamp: &metav1.Time{Time: time.Now()},
// 			Finalizers:        []string{RedisFinalizer},
// 		},
// 	}

// 	// Create a fake controller-runtime client
// 	scheme := runtime.NewScheme()
// 	mockAddToScheme := v1beta2.SchemeBuilder.Register(&v1beta2.Redis{}, &v1beta2.RedisList{}).AddToScheme(scheme)
// 	utilruntime.Must(mockAddToScheme)

// 	ctrlFakeclient := ctrlClientFake.NewClientBuilder().WithScheme(scheme).WithRuntimeObjects(cr.DeepCopyObject()).Build()
// 	k8sFakeClient := k8sClientFake.NewSimpleClientset(cr.DeepCopyObject())

// 	logger := testr.New(t)
// 	// Run the HandleRedisFinalizer function
// 	err := HandleRedisFinalizer(ctrlFakeclient, k8sFakeClient, logger, cr)
// 	assert.NoError(t, err)

// 	// Check if the PVC was deleted
// 	PVCName := fmt.Sprintf("%s-%s-0", cr.Name, cr.Name)
// 	_, err = k8sFakeClient.CoreV1().PersistentVolumeClaims(cr.Namespace).Get(context.TODO(), PVCName, metav1.GetOptions{})
// 	assert.True(t, k8serrors.IsNotFound(err))

// 	// Check if the finalizer was removed
// 	updatedCR := &v1beta2.Redis{}
// 	err = ctrlFakeclient.Get(context.TODO(), types.NamespacedName{Namespace: "default", Name: "test-redis"}, updatedCR)
// 	assert.NoError(t, err)
// 	assert.NotContains(t, updatedCR.GetFinalizers(), RedisFinalizer)

// 	// Ensure the logger's Error method was not called
// 	//logger.AssertNotCalled(t, "Error", mock.Anything, mock.Anything, mock.Anything)
// }

func TestFinalizeRedisPVC(t *testing.T) {
	tests := []struct {
		name          string
		existingPVC   *corev1.PersistentVolumeClaim
		expectError   bool
		errorExpected error
	}{
		{
			name: "PVC exists and is deleted successfully",
			existingPVC: &corev1.PersistentVolumeClaim{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-redis-test-redis-0",
					Namespace: "default",
				},
			},
			expectError:   false,
			errorExpected: nil,
		},
		{
			name: "PVC does not exist and no error should be returned",
			existingPVC: &corev1.PersistentVolumeClaim{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "nonexistent",
					Namespace: "default",
				},
			},
			expectError:   false,
			errorExpected: nil,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			logger := testr.New(t)
			cr := &v1beta2.Redis{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-redis",
					Namespace: "default",
				},
			}
			var k8sClient *k8sClientFake.Clientset
			if tc.existingPVC != nil {
				k8sClient = k8sClientFake.NewSimpleClientset(tc.existingPVC.DeepCopyObject())
			} else {
				k8sClient = k8sClientFake.NewSimpleClientset()
			}

			err := finalizeRedisPVC(k8sClient, logger, cr)
			if tc.expectError {
				assert.Error(t, err)
				assert.Equal(t, tc.errorExpected, err)
			} else {
				assert.NoError(t, err)
			}

			// Verify that the PVC is not found in case of success or non-existent PVC
			if !tc.expectError {
				pvcName := fmt.Sprintf("%s-%s-0", cr.Name, cr.Name)
				_, err = k8sClient.CoreV1().PersistentVolumeClaims(cr.Namespace).Get(context.TODO(), pvcName, metav1.GetOptions{})
				assert.True(t, k8serrors.IsNotFound(err))
			}
		})
	}
}

func TestFinalizeRedisReplicationPVC(t *testing.T) {
	tests := []struct {
		name             string
		existingPVCs     []*corev1.PersistentVolumeClaim
		redisReplication *v1beta2.RedisReplication
		expectError      bool
	}{
		{
			name: "Successful deletion of Redis Replication PVCs",
			redisReplication: &v1beta2.RedisReplication{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "redis-replication",
					Namespace: "redis",
				},
				Spec: v1beta2.RedisReplicationSpec{
					Size: pointer.Int32(3),
				},
			},
			existingPVCs: []*corev1.PersistentVolumeClaim{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "redis-replication-redis-replication-0",
						Namespace: "redis",
					},
				},
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "redis-replication-redis-replication-1",
						Namespace: "redis",
					},
				},
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "redis-replication-redis-replication-2",
						Namespace: "redis",
					},
				},
			},
			expectError: false,
		},
		{
			name:         "PVC does not exist and no error should be returned",
			existingPVCs: nil,
			redisReplication: &v1beta2.RedisReplication{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "redis-replication",
					Namespace: "redis",
				},
				Spec: v1beta2.RedisReplicationSpec{
					Size: pointer.Int32(3),
				},
			},
			expectError: false,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			logger := testr.New(t)
			var k8sClient *k8sClientFake.Clientset
			if tc.existingPVCs != nil {
				k8sClient = k8sClientFake.NewSimpleClientset(helperToRuntimeObjects(tc.existingPVCs)...)
			} else {
				k8sClient = k8sClientFake.NewSimpleClientset()
			}

			err := finalizeRedisReplicationPVC(k8sClient, logger, tc.redisReplication)
			if tc.expectError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}

			// Verify PVCs are deleted
			if !tc.expectError {
				for _, pvc := range tc.existingPVCs {
					_, err := k8sClient.CoreV1().PersistentVolumeClaims(pvc.Namespace).Get(context.TODO(), pvc.Name, metav1.GetOptions{})
					assert.True(t, k8serrors.IsNotFound(err))
				}
			}
		})
	}
}

func TestFinalizeRedisClusterPVC(t *testing.T) {
	tests := []struct {
		name         string
		existingPVCs []*corev1.PersistentVolumeClaim
		redisCluster *v1beta2.RedisCluster
		expectError  bool
	}{
		{
			name: "Successful deletion of Redis Cluster PVCs",
			redisCluster: &v1beta2.RedisCluster{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "redis-cluster",
					Namespace: "redis",
				},
				Spec: v1beta2.RedisClusterSpec{
					Size: pointer.Int32(3),
					Storage: &v1beta2.ClusterStorage{
						NodeConfVolume: true,
					},
				},
			},
			existingPVCs: helperRedisClusterPVCs("redis-cluster", "redis"),
			expectError:  false,
		},
		{
			name:         "PVC does not exist and no error should be returned",
			existingPVCs: nil,
			redisCluster: &v1beta2.RedisCluster{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "redis-cluster",
					Namespace: "redis",
				},
				Spec: v1beta2.RedisClusterSpec{
					Size: pointer.Int32(3),
					Storage: &v1beta2.ClusterStorage{
						NodeConfVolume: false,
					},
				},
			},
			expectError: false,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			logger := testr.New(t)
			var k8sClient *k8sClientFake.Clientset
			if tc.existingPVCs != nil {
				k8sClient = k8sClientFake.NewSimpleClientset(helperToRuntimeObjects(tc.existingPVCs)...)
			} else {
				k8sClient = k8sClientFake.NewSimpleClientset()
			}

			err := finalizeRedisClusterPVC(k8sClient, logger, tc.redisCluster)
			if tc.expectError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}

			// Verify PVCs are deleted
			if !tc.expectError {
				for _, pvc := range tc.existingPVCs {
					_, err := k8sClient.CoreV1().PersistentVolumeClaims(pvc.Namespace).Get(context.TODO(), pvc.Name, metav1.GetOptions{})
					assert.True(t, k8serrors.IsNotFound(err))
				}
			}
		})
	}
}

func helperToRuntimeObjects(pvcs []*corev1.PersistentVolumeClaim) []runtime.Object {
	objs := make([]runtime.Object, len(pvcs))
	for i, pvc := range pvcs {
		objs[i] = pvc.DeepCopyObject()
	}
	return objs
}

func helperRedisClusterPVCs(clusterName string, namespace string) []*corev1.PersistentVolumeClaim {
	var pvcs []*corev1.PersistentVolumeClaim
	roles := []string{"leader", "follower"}
	for _, role := range roles {
		for i := 0; i < 3; i++ {
			clusterPVCName := fmt.Sprintf("%s-%s-%s-%s-%d", clusterName, role, clusterName, role, i)
			clusterPVC := &corev1.PersistentVolumeClaim{
				ObjectMeta: metav1.ObjectMeta{
					Name:      clusterPVCName,
					Namespace: namespace,
				},
			}
			pvcs = append(pvcs, clusterPVC)
			nodeConfPVCName := fmt.Sprintf("node-conf-%s-%s-%d", clusterName, role, i)
			nodeConfPVC := &corev1.PersistentVolumeClaim{
				ObjectMeta: metav1.ObjectMeta{
					Name:      nodeConfPVCName,
					Namespace: namespace,
				},
			}
			pvcs = append(pvcs, nodeConfPVC)
		}
	}
	return pvcs
}

func TestAddRedisFinalizer(t *testing.T) {
	fakeFinalizer := "FakeFinalizer"
	tests := []struct {
		name            string
		redisStandalone *v1beta2.Redis
		want            *v1beta2.Redis
		expectError     bool
	}{
		{
			name: "Redis CR without finalizer",
			redisStandalone: &v1beta2.Redis{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "redis-standalone",
					Namespace: "default",
				},
			},
			want: &v1beta2.Redis{
				ObjectMeta: metav1.ObjectMeta{
					Name:       "redis-standalone",
					Namespace:  "default",
					Finalizers: []string{RedisFinalizer},
				},
			},
			expectError: false,
		},
		{
			name: "Redis CR with finalizer",
			redisStandalone: &v1beta2.Redis{
				ObjectMeta: metav1.ObjectMeta{
					Name:       "redis-standalone",
					Namespace:  "default",
					Finalizers: []string{RedisFinalizer},
				},
			},
			want: &v1beta2.Redis{
				ObjectMeta: metav1.ObjectMeta{
					Name:       "redis-standalone",
					Namespace:  "default",
					Finalizers: []string{RedisFinalizer},
				},
			},
			expectError: false,
		},
		{
			name: "Redis CR with random finalizer",
			redisStandalone: &v1beta2.Redis{
				ObjectMeta: metav1.ObjectMeta{
					Name:       "redis-standalone",
					Namespace:  "default",
					Finalizers: []string{fakeFinalizer},
				},
			},
			want: &v1beta2.Redis{
				ObjectMeta: metav1.ObjectMeta{
					Name:       "redis-standalone",
					Namespace:  "default",
					Finalizers: []string{fakeFinalizer, RedisFinalizer},
				},
			},
			expectError: false,
		},
	}

	for _, tc := range tests {
		mockClient := &mockClient.MockClient{
			UpdateFn: func(ctx context.Context, obj client.Object, opts ...client.UpdateOption) error {
				return nil
			},
		}
		t.Run(tc.name, func(t *testing.T) {
			err := AddRedisFinalizer(tc.redisStandalone, mockClient)
			if tc.expectError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tc.want, tc.redisStandalone)
			}
		})
	}
}

func TestAddRedisClusterFinalizer(t *testing.T) {
	fakeFinalizer := "FakeFinalizer"
	tests := []struct {
		name         string
		redisCluster *v1beta2.RedisCluster
		want         *v1beta2.RedisCluster
		expectError  bool
	}{
		{
			name: "Redis Cluster CR without finalizer",
			redisCluster: &v1beta2.RedisCluster{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "redis-cluster",
					Namespace: "default",
				},
			},
			want: &v1beta2.RedisCluster{
				ObjectMeta: metav1.ObjectMeta{
					Name:       "redis-cluster",
					Namespace:  "default",
					Finalizers: []string{RedisClusterFinalizer},
				},
			},
			expectError: false,
		},
		{
			name: "Redis Cluster CR with finalizer",
			redisCluster: &v1beta2.RedisCluster{
				ObjectMeta: metav1.ObjectMeta{
					Name:       "redis-cluster",
					Namespace:  "default",
					Finalizers: []string{RedisClusterFinalizer},
				},
			},
			want: &v1beta2.RedisCluster{
				ObjectMeta: metav1.ObjectMeta{
					Name:       "redis-cluster",
					Namespace:  "default",
					Finalizers: []string{RedisClusterFinalizer},
				},
			},
			expectError: false,
		},
		{
			name: "Redis Cluster CR with random finalizer",
			redisCluster: &v1beta2.RedisCluster{
				ObjectMeta: metav1.ObjectMeta{
					Name:       "redis-cluster",
					Namespace:  "default",
					Finalizers: []string{fakeFinalizer},
				},
			},
			want: &v1beta2.RedisCluster{
				ObjectMeta: metav1.ObjectMeta{
					Name:       "redis-cluster",
					Namespace:  "default",
					Finalizers: []string{fakeFinalizer, RedisClusterFinalizer},
				},
			},
			expectError: false,
		},
	}

	for _, tc := range tests {
		mockClient := &mockClient.MockClient{
			UpdateFn: func(ctx context.Context, obj client.Object, opts ...client.UpdateOption) error {
				return nil
			},
		}
		t.Run(tc.name, func(t *testing.T) {
			err := AddRedisClusterFinalizer(tc.redisCluster, mockClient)
			if tc.expectError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tc.want, tc.redisCluster)
			}
		})
	}
}

func TestAddRedisReplicationFinalizer(t *testing.T) {
	fakeFinalizer := "FakeFinalizer"
	tests := []struct {
		name             string
		redisReplication *v1beta2.RedisReplication
		want             *v1beta2.RedisReplication
		expectError      bool
	}{
		{
			name: "Redis Replication CR without finalizer",
			redisReplication: &v1beta2.RedisReplication{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "redis-replication",
					Namespace: "default",
				},
			},
			want: &v1beta2.RedisReplication{
				ObjectMeta: metav1.ObjectMeta{
					Name:       "redis-replication",
					Namespace:  "default",
					Finalizers: []string{RedisReplicationFinalizer},
				},
			},
			expectError: false,
		},
		{
			name: "Redis Replication CR with finalizer",
			redisReplication: &v1beta2.RedisReplication{
				ObjectMeta: metav1.ObjectMeta{
					Name:       "redis-replication",
					Namespace:  "default",
					Finalizers: []string{RedisReplicationFinalizer},
				},
			},
			want: &v1beta2.RedisReplication{
				ObjectMeta: metav1.ObjectMeta{
					Name:       "redis-replication",
					Namespace:  "default",
					Finalizers: []string{RedisReplicationFinalizer},
				},
			},
			expectError: false,
		},
		{
			name: "Redis Replication CR with random finalizer",
			redisReplication: &v1beta2.RedisReplication{
				ObjectMeta: metav1.ObjectMeta{
					Name:       "redis-replication",
					Namespace:  "default",
					Finalizers: []string{fakeFinalizer},
				},
			},
			want: &v1beta2.RedisReplication{
				ObjectMeta: metav1.ObjectMeta{
					Name:       "redis-replication",
					Namespace:  "default",
					Finalizers: []string{fakeFinalizer, RedisReplicationFinalizer},
				},
			},
			expectError: false,
		},
	}

	for _, tc := range tests {
		mockClient := &mockClient.MockClient{
			UpdateFn: func(ctx context.Context, obj client.Object, opts ...client.UpdateOption) error {
				return nil
			},
		}
		t.Run(tc.name, func(t *testing.T) {
			err := AddRedisReplicationFinalizer(tc.redisReplication, mockClient)
			if tc.expectError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tc.want, tc.redisReplication)
			}
		})
	}
}

func TestAddRedisSentinelFinalizer(t *testing.T) {
	fakeFinalizer := "FakeFinalizer"
	tests := []struct {
		name          string
		redisSentinel *v1beta2.RedisSentinel
		want          *v1beta2.RedisSentinel
		expectError   bool
	}{
		{
			name: "Redis Sentinel CR without finalizer",
			redisSentinel: &v1beta2.RedisSentinel{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "redis-sentinel",
					Namespace: "default",
				},
			},
			want: &v1beta2.RedisSentinel{
				ObjectMeta: metav1.ObjectMeta{
					Name:       "redis-sentinel",
					Namespace:  "default",
					Finalizers: []string{RedisSentinelFinalizer},
				},
			},
			expectError: false,
		},
		{
			name: "Redis Sentinel CR with finalizer",
			redisSentinel: &v1beta2.RedisSentinel{
				ObjectMeta: metav1.ObjectMeta{
					Name:       "redis-sentinel",
					Namespace:  "default",
					Finalizers: []string{RedisSentinelFinalizer},
				},
			},
			want: &v1beta2.RedisSentinel{
				ObjectMeta: metav1.ObjectMeta{
					Name:       "redis-sentinel",
					Namespace:  "default",
					Finalizers: []string{RedisSentinelFinalizer},
				},
			},
			expectError: false,
		},
		{
			name: "Redis Sentinel CR with random finalizer",
			redisSentinel: &v1beta2.RedisSentinel{
				ObjectMeta: metav1.ObjectMeta{
					Name:       "redis-sentinel",
					Namespace:  "default",
					Finalizers: []string{fakeFinalizer},
				},
			},
			want: &v1beta2.RedisSentinel{
				ObjectMeta: metav1.ObjectMeta{
					Name:       "redis-sentinel",
					Namespace:  "default",
					Finalizers: []string{fakeFinalizer, RedisSentinelFinalizer},
				},
			},
			expectError: false,
		},
	}

	for _, tc := range tests {
		mockClient := &mockClient.MockClient{
			UpdateFn: func(ctx context.Context, obj client.Object, opts ...client.UpdateOption) error {
				return nil
			},
		}
		t.Run(tc.name, func(t *testing.T) {
			err := AddRedisSentinelFinalizer(tc.redisSentinel, mockClient)
			if tc.expectError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tc.want, tc.redisSentinel)
			}
		})
	}
}
