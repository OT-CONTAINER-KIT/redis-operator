package k8sutils

import (
	"context"
	"fmt"

	redisv1beta2 "github.com/OT-CONTAINER-KIT/redis-operator/api/v1beta2"
	"github.com/go-logr/logr"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/utils/env"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

const (
	RedisFinalizer            string = "redisFinalizer"
	RedisClusterFinalizer     string = "redisClusterFinalizer"
	RedisReplicationFinalizer string = "redisReplicationFinalizer"
	RedisSentinelFinalizer    string = "redisSentinelFinalizer"
)

// HandleRedisFinalizer finalize resource if instance is marked to be deleted
func HandleRedisFinalizer(ctrlclient client.Client, k8sClient kubernetes.Interface, logger logr.Logger, cr *redisv1beta2.Redis) error {
	if cr.GetDeletionTimestamp() != nil {
		if controllerutil.ContainsFinalizer(cr, RedisFinalizer) {
			if cr.Spec.Storage != nil && !cr.Spec.Storage.KeepAfterDelete {
				if err := finalizeRedisPVC(k8sClient, logger, cr); err != nil {
					return err
				}
			}
			controllerutil.RemoveFinalizer(cr, RedisFinalizer)
			if err := ctrlclient.Update(context.TODO(), cr); err != nil {
				logger.Error(err, "Could not remove finalizer", "finalizer", RedisFinalizer)
				return err
			}
		}
	}
	return nil
}

// HandleRedisClusterFinalizer finalize resource if instance is marked to be deleted
func HandleRedisClusterFinalizer(ctrlclient client.Client, k8sClient kubernetes.Interface, logger logr.Logger, cr *redisv1beta2.RedisCluster) error {
	if cr.GetDeletionTimestamp() != nil {
		if controllerutil.ContainsFinalizer(cr, RedisClusterFinalizer) {
			if cr.Spec.Storage != nil && !cr.Spec.Storage.KeepAfterDelete {
				if err := finalizeRedisClusterPVC(k8sClient, logger, cr); err != nil {
					return err
				}
			}
			controllerutil.RemoveFinalizer(cr, RedisClusterFinalizer)
			if err := ctrlclient.Update(context.TODO(), cr); err != nil {
				logger.Error(err, "Could not remove finalizer "+RedisClusterFinalizer)
				return err
			}
		}
	}
	return nil
}

// Handle RedisReplicationFinalizer finalize resource if instance is marked to be deleted
func HandleRedisReplicationFinalizer(ctrlclient client.Client, k8sClient kubernetes.Interface, logger logr.Logger, cr *redisv1beta2.RedisReplication) error {
	if cr.GetDeletionTimestamp() != nil {
		if controllerutil.ContainsFinalizer(cr, RedisReplicationFinalizer) {
			if cr.Spec.Storage != nil && !cr.Spec.Storage.KeepAfterDelete {
				if err := finalizeRedisReplicationPVC(k8sClient, logger, cr); err != nil {
					return err
				}
			}
			controllerutil.RemoveFinalizer(cr, RedisReplicationFinalizer)
			if err := ctrlclient.Update(context.TODO(), cr); err != nil {
				logger.Error(err, "Could not remove finalizer "+RedisReplicationFinalizer)
				return err
			}
		}
	}
	return nil
}

// HandleRedisSentinelFinalizer finalize resource if instance is marked to be deleted
func HandleRedisSentinelFinalizer(ctrlclient client.Client, logger logr.Logger, cr *redisv1beta2.RedisSentinel) error {
	if cr.GetDeletionTimestamp() != nil {
		if controllerutil.ContainsFinalizer(cr, RedisSentinelFinalizer) {
			controllerutil.RemoveFinalizer(cr, RedisSentinelFinalizer)
			if err := ctrlclient.Update(context.TODO(), cr); err != nil {
				logger.Error(err, "Could not remove finalizer "+RedisSentinelFinalizer)
				return err
			}
		}
	}
	return nil
}

// AddFinalizer add finalizer for graceful deletion
func AddFinalizer(cr client.Object, finalizer string, cl client.Client) error {
	if !controllerutil.ContainsFinalizer(cr, finalizer) {
		controllerutil.AddFinalizer(cr, finalizer)
		return cl.Update(context.TODO(), cr)
	}
	return nil
}

// finalizeRedisPVC delete PVC
func finalizeRedisPVC(client kubernetes.Interface, logger logr.Logger, cr *redisv1beta2.Redis) error {
	pvcTemplateName := env.GetString(EnvOperatorSTSPVCTemplateName, cr.Name)
	PVCName := fmt.Sprintf("%s-%s-0", pvcTemplateName, cr.Name)
	err := client.CoreV1().PersistentVolumeClaims(cr.Namespace).Delete(context.TODO(), PVCName, metav1.DeleteOptions{})
	if err != nil && !errors.IsNotFound(err) {
		logger.Error(err, "Could not delete Persistent Volume Claim", "PVCName", PVCName)
		return err
	}
	return nil
}

// finalizeRedisClusterPVC delete PVCs
func finalizeRedisClusterPVC(client kubernetes.Interface, logger logr.Logger, cr *redisv1beta2.RedisCluster) error {
	for _, role := range []string{"leader", "follower"} {
		for i := 0; i < int(cr.Spec.GetReplicaCounts(role)); i++ {
			pvcTemplateName := env.GetString(EnvOperatorSTSPVCTemplateName, cr.Name+"-"+role)
			PVCName := fmt.Sprintf("%s-%s-%s-%d", pvcTemplateName, cr.Name, role, i)
			err := client.CoreV1().PersistentVolumeClaims(cr.Namespace).Delete(context.TODO(), PVCName, metav1.DeleteOptions{})
			if err != nil && !errors.IsNotFound(err) {
				logger.Error(err, "Could not delete Persistent Volume Claim "+PVCName)
				return err
			}
		}
		if cr.Spec.Storage.NodeConfVolume {
			for i := 0; i < int(cr.Spec.GetReplicaCounts(role)); i++ {
				PVCName := fmt.Sprintf("%s-%s-%s-%d", "node-conf", cr.Name, role, i)
				err := client.CoreV1().PersistentVolumeClaims(cr.Namespace).Delete(context.TODO(), PVCName, metav1.DeleteOptions{})
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
func finalizeRedisReplicationPVC(client kubernetes.Interface, logger logr.Logger, cr *redisv1beta2.RedisReplication) error {
	for i := 0; i < int(cr.Spec.GetReplicationCounts("replication")); i++ {
		pvcTemplateName := env.GetString(EnvOperatorSTSPVCTemplateName, cr.Name)
		PVCName := fmt.Sprintf("%s-%s-%d", pvcTemplateName, cr.Name, i)
		err := client.CoreV1().PersistentVolumeClaims(cr.Namespace).Delete(context.TODO(), PVCName, metav1.DeleteOptions{})
		if err != nil && !errors.IsNotFound(err) {
			logger.Error(err, "Could not delete Persistent Volume Claim "+PVCName)
			return err
		}
	}
	return nil
}
