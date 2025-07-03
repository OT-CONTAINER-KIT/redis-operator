package k8sutils

import (
	"context"
	"fmt"

	rvb2 "github.com/OT-CONTAINER-KIT/redis-operator/api/redis/v1beta2"
	rcvb2 "github.com/OT-CONTAINER-KIT/redis-operator/api/rediscluster/v1beta2"
	rrvb2 "github.com/OT-CONTAINER-KIT/redis-operator/api/redisreplication/v1beta2"
	rsvb2 "github.com/OT-CONTAINER-KIT/redis-operator/api/redissentinel/v1beta2"
	"github.com/OT-CONTAINER-KIT/redis-operator/internal/controller/common"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/env"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

// HandleRedisFinalizer finalize resource if instance is marked to be deleted
func HandleRedisFinalizer(ctx context.Context, ctrlclient client.Client, cr *rvb2.Redis, finalizer string) error {
	if cr.GetDeletionTimestamp() != nil {
		if controllerutil.ContainsFinalizer(cr, finalizer) {
			if cr.Spec.Storage != nil && !cr.Spec.Storage.KeepAfterDelete {
				if err := finalizeRedisPVC(ctx, ctrlclient, cr); err != nil {
					return err
				}
			}
			controllerutil.RemoveFinalizer(cr, finalizer)
			if err := ctrlclient.Update(ctx, cr); err != nil {
				log.FromContext(ctx).Error(err, "Could not remove finalizer", "finalizer", finalizer)
				return err
			}
		}
	}
	return nil
}

// HandleRedisClusterFinalizer finalize resource if instance is marked to be deleted
func HandleRedisClusterFinalizer(ctx context.Context, ctrlclient client.Client, cr *rcvb2.RedisCluster, finalizer string) error {
	if cr.GetDeletionTimestamp() != nil {
		if controllerutil.ContainsFinalizer(cr, finalizer) {
			if cr.Spec.Storage != nil && !cr.Spec.Storage.KeepAfterDelete {
				if err := finalizeRedisClusterPVC(ctx, ctrlclient, cr); err != nil {
					return err
				}
			}
			controllerutil.RemoveFinalizer(cr, finalizer)
			if err := ctrlclient.Update(ctx, cr); err != nil {
				log.FromContext(ctx).Error(err, "Could not remove finalizer "+finalizer)
				return err
			}
		}
	}
	return nil
}

// Handle RedisReplicationFinalizer finalize resource if instance is marked to be deleted
func HandleRedisReplicationFinalizer(ctx context.Context, ctrlclient client.Client, cr *rrvb2.RedisReplication, finalizer string) error {
	if cr.GetDeletionTimestamp() != nil {
		if controllerutil.ContainsFinalizer(cr, finalizer) {
			if cr.Spec.Storage != nil && !cr.Spec.Storage.KeepAfterDelete {
				if err := finalizeRedisReplicationPVC(ctx, ctrlclient, cr); err != nil {
					return err
				}
			}
			controllerutil.RemoveFinalizer(cr, finalizer)
			if err := ctrlclient.Update(ctx, cr); err != nil {
				log.FromContext(ctx).Error(err, "Could not remove finalizer "+finalizer)
				return err
			}
		}
	}
	return nil
}

// HandleRedisSentinelFinalizer finalize resource if instance is marked to be deleted
func HandleRedisSentinelFinalizer(ctx context.Context, ctrlclient client.Client, cr *rsvb2.RedisSentinel, finalizer string) error {
	if cr.GetDeletionTimestamp() != nil {
		if controllerutil.ContainsFinalizer(cr, finalizer) {
			controllerutil.RemoveFinalizer(cr, finalizer)
			if err := ctrlclient.Update(ctx, cr); err != nil {
				log.FromContext(ctx).Error(err, "Could not remove finalizer "+finalizer)
				return err
			}
		}
	}
	return nil
}

// AddFinalizer add finalizer for graceful deletion
func AddFinalizer(ctx context.Context, cr client.Object, finalizer string, cl client.Client) error {
	return common.AddFinalizer(ctx, cr, finalizer, cl)
}

// finalizeRedisPVC delete PVC
func finalizeRedisPVC(ctx context.Context, client client.Client, cr *rvb2.Redis) error {
	pvcTemplateName := env.GetString(common.EnvOperatorSTSPVCTemplateName, cr.Name)
	PVCName := fmt.Sprintf("%s-%s-0", pvcTemplateName, cr.Name)
	pvc := &corev1.PersistentVolumeClaim{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: cr.Namespace,
			Name:      PVCName,
		},
	}
	err := client.Delete(ctx, pvc)
	if err != nil && !errors.IsNotFound(err) {
		log.FromContext(ctx).Error(err, "Could not delete Persistent Volume Claim", "PVCName", PVCName)
		return err
	}
	return nil
}

// finalizeRedisClusterPVC delete PVCs
func finalizeRedisClusterPVC(ctx context.Context, client client.Client, cr *rcvb2.RedisCluster) error {
	for _, role := range []string{"leader", "follower"} {
		for i := 0; i < int(cr.Spec.GetReplicaCounts(role)); i++ {
			pvcTemplateName := env.GetString(common.EnvOperatorSTSPVCTemplateName, cr.Name+"-"+role)
			PVCName := fmt.Sprintf("%s-%s-%s-%d", pvcTemplateName, cr.Name, role, i)
			pvc := &corev1.PersistentVolumeClaim{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: cr.Namespace,
					Name:      PVCName,
				},
			}
			err := client.Delete(ctx, pvc)
			if err != nil && !errors.IsNotFound(err) {
				log.FromContext(ctx).Error(err, "Could not delete Persistent Volume Claim "+PVCName)
				return err
			}
		}
		if cr.Spec.Storage.NodeConfVolume {
			for i := 0; i < int(cr.Spec.GetReplicaCounts(role)); i++ {
				PVCName := fmt.Sprintf("%s-%s-%s-%d", "node-conf", cr.Name, role, i)
				pvc := &corev1.PersistentVolumeClaim{
					ObjectMeta: metav1.ObjectMeta{
						Namespace: cr.Namespace,
						Name:      PVCName,
					},
				}
				err := client.Delete(ctx, pvc)
				if err != nil && !errors.IsNotFound(err) {
					log.FromContext(ctx).Error(err, "Could not delete Persistent Volume Claim "+PVCName)
					return err
				}
			}
		}
	}
	return nil
}

// finalizeRedisReplicationPVC delete PVCs
func finalizeRedisReplicationPVC(ctx context.Context, client client.Client, cr *rrvb2.RedisReplication) error {
	for i := 0; i < int(cr.Spec.GetReplicationCounts("replication")); i++ {
		pvcTemplateName := env.GetString(common.EnvOperatorSTSPVCTemplateName, cr.Name)
		PVCName := fmt.Sprintf("%s-%s-%d", pvcTemplateName, cr.Name, i)
		pvc := &corev1.PersistentVolumeClaim{
			ObjectMeta: metav1.ObjectMeta{
				Namespace: cr.Namespace,
				Name:      PVCName,
			},
		}
		err := client.Delete(ctx, pvc)
		if err != nil && !errors.IsNotFound(err) {
			log.FromContext(ctx).Error(err, "Could not delete Persistent Volume Claim "+PVCName)
			return err
		}
	}
	return nil
}
