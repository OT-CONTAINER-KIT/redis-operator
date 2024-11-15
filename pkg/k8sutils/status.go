package k8sutils

import (
	"context"
	"reflect"

	"github.com/OT-CONTAINER-KIT/redis-operator/api/status"
	redisv1beta2 "github.com/OT-CONTAINER-KIT/redis-operator/api/v1beta2"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

// UpdateRedisClusterStatus will update the status of the RedisCluster
func UpdateRedisClusterStatus(ctx context.Context, cr *redisv1beta2.RedisCluster, state status.RedisClusterState, reason string, readyLeaderReplicas, readyFollowerReplicas int32, dcl dynamic.Interface) error {
	newStatus := redisv1beta2.RedisClusterStatus{
		State:                 state,
		Reason:                reason,
		ReadyLeaderReplicas:   readyLeaderReplicas,
		ReadyFollowerReplicas: readyFollowerReplicas,
	}
	if reflect.DeepEqual(cr.Status, newStatus) {
		return nil
	}
	cr.Status = newStatus
	gvr := schema.GroupVersionResource{
		Group:    "redis.redis.opstreelabs.in",
		Version:  "v1beta2",
		Resource: "redisclusters",
	}
	unstructuredObj, err := runtime.DefaultUnstructuredConverter.ToUnstructured(cr)
	if err != nil {
		log.FromContext(ctx).Error(err, "Failed to convert CR to unstructured object")
		return err
	}
	unstructuredRedisCluster := &unstructured.Unstructured{Object: unstructuredObj}

	_, err = dcl.Resource(gvr).Namespace(cr.Namespace).UpdateStatus(context.TODO(), unstructuredRedisCluster, metav1.UpdateOptions{})
	if err != nil {
		log.FromContext(ctx).Error(err, "Failed to update status")
		return err
	}
	return nil
}
