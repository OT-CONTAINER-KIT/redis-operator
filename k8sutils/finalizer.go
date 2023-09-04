package k8sutils

import (
	"context"
	"strconv"

	redisv1beta2 "github.com/OT-CONTAINER-KIT/redis-operator/api/v1beta2"
	"github.com/go-logr/logr"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

const (
	RedisFinalizer            string = "redisFinalizer"
	RedisClusterFinalizer     string = "redisClusterFinalizer"
	RedisReplicationFinalizer string = "redisReplicationFinalizer"
	RedisSentinelFinalizer    string = "redisSentinelFinalizer"
)

// finalizeLogger will generate logging interface
func finalizerLogger(namespace string, name string) logr.Logger {
	reqLogger := log.WithValues("Request.Service.Namespace", namespace, "Request.Finalizer.Name", name)
	return reqLogger
}

// HandleRedisFinalizer finalize resource if instance is marked to be deleted
func HandleRedisFinalizer(cr *redisv1beta2.Redis, cl client.Client) error {
	logger := finalizerLogger(cr.Namespace, RedisFinalizer)
	if cr.GetDeletionTimestamp() != nil {
		if controllerutil.ContainsFinalizer(cr, RedisFinalizer) {
			if err := finalizeRedisPVC(cr); err != nil {
				return err
			}
			controllerutil.RemoveFinalizer(cr, RedisFinalizer)
			if err := cl.Update(context.TODO(), cr); err != nil {
				logger.Error(err, "Could not remove finalizer "+RedisFinalizer)
				return err
			}
		}
	}
	return nil
}

// HandleRedisClusterFinalizer finalize resource if instance is marked to be deleted
func HandleRedisClusterFinalizer(cr *redisv1beta2.RedisCluster, cl client.Client) error {
	logger := finalizerLogger(cr.Namespace, RedisClusterFinalizer)
	if cr.GetDeletionTimestamp() != nil {
		if controllerutil.ContainsFinalizer(cr, RedisClusterFinalizer) {
			if err := finalizeRedisClusterPVC(cr); err != nil {
				return err
			}
			controllerutil.RemoveFinalizer(cr, RedisClusterFinalizer)
			if err := cl.Update(context.TODO(), cr); err != nil {
				logger.Error(err, "Could not remove finalizer "+RedisClusterFinalizer)
				return err
			}
		}
	}
	return nil
}

// Handle RedisReplicationFinalizer finalize resource if instance is marked to be deleted
func HandleRedisReplicationFinalizer(cr *redisv1beta2.RedisReplication, cl client.Client) error {
	logger := finalizerLogger(cr.Namespace, RedisReplicationFinalizer)
	if cr.GetDeletionTimestamp() != nil {
		if controllerutil.ContainsFinalizer(cr, RedisReplicationFinalizer) {
			if err := finalizeRedisReplicationPVC(cr); err != nil {
				return err
			}
			controllerutil.RemoveFinalizer(cr, RedisReplicationFinalizer)
			if err := cl.Update(context.TODO(), cr); err != nil {
				logger.Error(err, "Could not remove finalizer "+RedisReplicationFinalizer)
				return err
			}
		}
	}
	return nil
}

// HandleRedisSentinelFinalizer finalize resource if instance is marked to be deleted
func HandleRedisSentinelFinalizer(cr *redisv1beta2.RedisSentinel, cl client.Client) error {
	logger := finalizerLogger(cr.Namespace, RedisSentinelFinalizer)
	if cr.GetDeletionTimestamp() != nil {
		if controllerutil.ContainsFinalizer(cr, RedisSentinelFinalizer) {
			if err := finalizeRedisSentinelPVC(cr); err != nil {
				return err
			}
			controllerutil.RemoveFinalizer(cr, RedisSentinelFinalizer)
			if err := cl.Update(context.TODO(), cr); err != nil {
				logger.Error(err, "Could not remove finalizer "+RedisSentinelFinalizer)
				return err
			}
		}
	}
	return nil
}

// AddRedisFinalizer add finalizer for graceful deletion
func AddRedisFinalizer(cr *redisv1beta2.Redis, cl client.Client) error {
	if !controllerutil.ContainsFinalizer(cr, RedisFinalizer) {
		controllerutil.AddFinalizer(cr, RedisFinalizer)
		return cl.Update(context.TODO(), cr)
	}
	return nil
}

// AddRedisClusterFinalizer add finalizer for graceful deletion
func AddRedisClusterFinalizer(cr *redisv1beta2.RedisCluster, cl client.Client) error {
	if !controllerutil.ContainsFinalizer(cr, RedisClusterFinalizer) {
		controllerutil.AddFinalizer(cr, RedisClusterFinalizer)
		return cl.Update(context.TODO(), cr)
	}
	return nil
}

// AddRedisReplicationFinalizer add finalizer for graceful deletion
func AddRedisReplicationFinalizer(cr *redisv1beta2.RedisReplication, cl client.Client) error {
	if !controllerutil.ContainsFinalizer(cr, RedisReplicationFinalizer) {
		controllerutil.AddFinalizer(cr, RedisReplicationFinalizer)
		return cl.Update(context.TODO(), cr)
	}
	return nil
}

// AddRedisSentinelFinalizer add finalizer for graceful deletion
func AddRedisSentinelFinalizer(cr *redisv1beta2.RedisSentinel, cl client.Client) error {
	if !controllerutil.ContainsFinalizer(cr, RedisSentinelFinalizer) {
		controllerutil.AddFinalizer(cr, RedisSentinelFinalizer)
		return cl.Update(context.TODO(), cr)
	}
	return nil
}

// finalizeRedisPVC delete PVC
func finalizeRedisPVC(cr *redisv1beta2.Redis) error {
	logger := finalizerLogger(cr.Namespace, RedisFinalizer)
	PVCName := cr.Name + "-" + cr.Name + "-0"
	err := generateK8sClient().CoreV1().PersistentVolumeClaims(cr.Namespace).Delete(context.TODO(), PVCName, metav1.DeleteOptions{})
	if err != nil && !errors.IsNotFound(err) {
		logger.Error(err, "Could not delete Persistent Volume Claim "+PVCName)
		return err
	}
	return nil
}

// finalizeRedisClusterPVC delete PVCs
func finalizeRedisClusterPVC(cr *redisv1beta2.RedisCluster) error {
	logger := finalizerLogger(cr.Namespace, RedisClusterFinalizer)
	for _, role := range []string{"leader", "follower"} {
		for i := 0; i < int(cr.Spec.GetReplicaCounts(role)); i++ {
			PVCName := cr.Name + "-" + cr.Name + "-" + role + "-" + strconv.Itoa(i)
			err := generateK8sClient().CoreV1().PersistentVolumeClaims(cr.Namespace).Delete(context.TODO(), PVCName, metav1.DeleteOptions{})
			if err != nil && !errors.IsNotFound(err) {
				logger.Error(err, "Could not delete Persistent Volume Claim "+PVCName)
				return err
			}
		}
		if cr.Spec.Storage.NodeConfVolume {
			for i := 0; i < int(cr.Spec.GetReplicaCounts(role)); i++ {
				PVCName := "node-conf" + cr.Name + "-" + role + "-" + strconv.Itoa(i)
				err := generateK8sClient().CoreV1().PersistentVolumeClaims(cr.Namespace).Delete(context.TODO(), PVCName, metav1.DeleteOptions{})
				if err != nil && !errors.IsNotFound(err) {
					logger.Error(err, "Could not delete Persistent Volume Claim "+PVCName)
					return err
				}
			}
		}

	}
	return nil
}

// finalizeRedisReplicationPVC delete PVCs
func finalizeRedisReplicationPVC(cr *redisv1beta2.RedisReplication) error {
	logger := finalizerLogger(cr.Namespace, RedisReplicationFinalizer)
	for i := 0; i < int(cr.Spec.GetReplicationCounts("replication")); i++ {
		PVCName := cr.Name + "-" + cr.Name + "-" + strconv.Itoa(i)
		err := generateK8sClient().CoreV1().PersistentVolumeClaims(cr.Namespace).Delete(context.TODO(), PVCName, metav1.DeleteOptions{})
		if err != nil && !errors.IsNotFound(err) {
			logger.Error(err, "Could not delete Persistent Volume Claim "+PVCName)
			return err
		}
	}

	return nil
}

func finalizeRedisSentinelPVC(cr *redisv1beta2.RedisSentinel) error {

	return nil
}
