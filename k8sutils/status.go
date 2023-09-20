package k8sutils

import (
	"context"

	status "github.com/OT-CONTAINER-KIT/redis-operator/api/status"
	redisv1beta2 "github.com/OT-CONTAINER-KIT/redis-operator/api/v1beta2"
	"github.com/go-logr/logr"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

// statusLogger will generate logging interface for status
func statusLogger(namespace string, name string) logr.Logger {
	reqLogger := log.WithValues("Request.Namespace", namespace, "Request.Name", name)
	return reqLogger
}

// UpdateRedisClusterStatus will update the status of the RedisCluster
func UpdateRedisClusterStatus(cr *redisv1beta2.RedisCluster, status status.RedisClusterState, resaon string, readyLeaderReplicas, readyFollowerReplicas int32) error {
	logger := statusLogger(cr.Namespace, cr.Name)
	cr.Status.State = status
	cr.Status.Reason = resaon
	cr.Status.ReadyLeaderReplicas = readyLeaderReplicas
	cr.Status.ReadyFollowerReplicas = readyFollowerReplicas

	client := generateK8sDynamicClient()
	gvr := schema.GroupVersionResource{
		Group:    "redis.redis.opstreelabs.in",
		Version:  "v1beta2",
		Resource: "redisclusters",
	}
	unstructuredObj, err := runtime.DefaultUnstructuredConverter.ToUnstructured(cr)
	if err != nil {
		logger.Error(err, "Failed to convert CR to unstructured object")
		return err
	}
	unstructuredRedisCluster := &unstructured.Unstructured{Object: unstructuredObj}

	_, err = client.Resource(gvr).Namespace(cr.Namespace).UpdateStatus(context.TODO(), unstructuredRedisCluster, metav1.UpdateOptions{})
	if err != nil {
		logger.Error(err, "Failed to update status")
		return err
	}
	return nil
}

func UpdateRedisStandaloneStatus(cr *redisv1beta2.Redis, status status.RedisStandaloneState, resaon string) error {
	logger := statusLogger(cr.Namespace, cr.Name)
	cr.Status.State = status
	cr.Status.Reason = resaon

	client := generateK8sDynamicClient()
	gvr := schema.GroupVersionResource{
		Group:    "redis.redis.opstreelabs.in",
		Version:  "v1beta2",
		Resource: "redis",
	}
	unstructuredObj, err := runtime.DefaultUnstructuredConverter.ToUnstructured(cr)
	if err != nil {
		logger.Error(err, "Failed to convert CR to unstructured object")
		return err
	}
	unstructuredRedisStandalone := &unstructured.Unstructured{Object: unstructuredObj}

	_, err = client.Resource(gvr).Namespace(cr.Namespace).UpdateStatus(context.TODO(), unstructuredRedisStandalone, metav1.UpdateOptions{})
	if err != nil {
		logger.Error(err, "Failed to update status")
		return err
	}
	return nil
}

func UpdateRedisSentinelStatus(cr *redisv1beta2.RedisSentinel, status status.RedisSentinelState, resaon string) error {
	logger := statusLogger(cr.Namespace, cr.Name)
	cr.Status.State = status
	cr.Status.Reason = resaon

	client := generateK8sDynamicClient()
	gvr := schema.GroupVersionResource{
		Group:    "redis.redis.opstreelabs.in",
		Version:  "v1beta2",
		Resource: "redissentinels",
	}
	unstructuredObj, err := runtime.DefaultUnstructuredConverter.ToUnstructured(cr)
	if err != nil {
		logger.Error(err, "Failed to convert CR to unstructured object")
		return err
	}
	unstructuredRedisSentinel := &unstructured.Unstructured{Object: unstructuredObj}

	_, err = client.Resource(gvr).Namespace(cr.Namespace).UpdateStatus(context.TODO(), unstructuredRedisSentinel, metav1.UpdateOptions{})
	if err != nil {
		logger.Error(err, "Failed to update status")
		return err
	}
	return nil
}

func UpdateRedisReplicationStatus(cr *redisv1beta2.RedisReplication, status status.RedisReplicationState, resaon string) error {
	logger := statusLogger(cr.Namespace, cr.Name)
	cr.Status.State = status
	cr.Status.Reason = resaon

	client := generateK8sDynamicClient()
	gvr := schema.GroupVersionResource{
		Group:    "redis.redis.opstreelabs.in",
		Version:  "v1beta2",
		Resource: "redisreplications",
	}
	unstructuredObj, err := runtime.DefaultUnstructuredConverter.ToUnstructured(cr)
	if err != nil {
		logger.Error(err, "Failed to convert CR to unstructured object")
		return err
	}
	unstructuredRedisReplication := &unstructured.Unstructured{Object: unstructuredObj}

	_, err = client.Resource(gvr).Namespace(cr.Namespace).UpdateStatus(context.TODO(), unstructuredRedisReplication, metav1.UpdateOptions{})
	if err != nil {
		logger.Error(err, "Failed to update status")
		return err
	}
	return nil
}
