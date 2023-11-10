package k8sutils

import (
	"context"
	"fmt"
	"testing"

	"github.com/OT-CONTAINER-KIT/redis-operator/api/v1beta2"
	"github.com/go-logr/logr/testr"
	"github.com/stretchr/testify/assert"
	corev1 "k8s.io/api/core/v1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	k8sClientFake "k8s.io/client-go/kubernetes/fake"
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
