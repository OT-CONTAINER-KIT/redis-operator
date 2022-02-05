package k8sutils

import (
	"context"
	redisv1beta1 "redis-operator/api/v1beta1"
	"strconv"

	"github.com/go-logr/logr"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

const (
	RedisFinalizer        string = "redisFinalizer"
	RedisClusterFinalizer string = "redisClusterFinalizer"
)

// finalizeLogger will generate logging interface
func finalizerLogger(namespace string, name string) logr.Logger {
	reqLogger := log.WithValues("Request.Service.Namespace", namespace, "Request.Finalizer.Name", name)
	return reqLogger
}

// HandleRedisFinalizer finalize resource if instance is marked to be deleted
func HandleRedisFinalizer(cr *redisv1beta1.Redis, cl client.Client) error {
	logger := finalizerLogger(cr.Namespace, RedisFinalizer)
	if cr.GetDeletionTimestamp() != nil {
		if controllerutil.ContainsFinalizer(cr, RedisFinalizer) {
			if err := finalizeRedisServices(cr); err != nil {
				return err
			}
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
func HandleRedisClusterFinalizer(cr *redisv1beta1.RedisCluster, cl client.Client) error {
	logger := finalizerLogger(cr.Namespace, RedisClusterFinalizer)
	if cr.GetDeletionTimestamp() != nil {
		if controllerutil.ContainsFinalizer(cr, RedisClusterFinalizer) {
			if err := finalizeRedisClusterServices(cr); err != nil {
				return err
			}
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

// AddRedisFinalizer add finalizer for graceful deletion
func AddRedisFinalizer(cr *redisv1beta1.Redis, cl client.Client) error {
	if !controllerutil.ContainsFinalizer(cr, RedisFinalizer) {
		controllerutil.AddFinalizer(cr, RedisFinalizer)
		return cl.Update(context.TODO(), cr)
	}
	return nil
}

// AddRedisClusterFinalizer add finalizer for graceful deletion
func AddRedisClusterFinalizer(cr *redisv1beta1.RedisCluster, cl client.Client) error {
	if !controllerutil.ContainsFinalizer(cr, RedisClusterFinalizer) {
		controllerutil.AddFinalizer(cr, RedisClusterFinalizer)
		return cl.Update(context.TODO(), cr)
	}
	return nil
}

// finalizeRedisServices delete Services
func finalizeRedisServices(cr *redisv1beta1.Redis) error {
	logger := finalizerLogger(cr.Namespace, RedisFinalizer)
	serviceName, headlessServiceName := cr.Name, cr.Name+"-headless"
	for _, svc := range []string{serviceName, headlessServiceName} {
		err := generateK8sClient().CoreV1().Services(cr.Namespace).Delete(context.TODO(), svc, metav1.DeleteOptions{})
		if err != nil && !errors.IsNotFound(err) {
			logger.Error(err, "Could not delete service "+svc)
			return err
		}
	}
	return nil
}

// finalizeRedisClusterServices delete Services
func finalizeRedisClusterServices(cr *redisv1beta1.RedisCluster) error {
	logger := finalizerLogger(cr.Namespace, RedisClusterFinalizer)
	serviceName, headlessServiceName := cr.Name, cr.Name+"-headless"
	for _, svc := range []string{serviceName, headlessServiceName} {
		err := generateK8sClient().CoreV1().Services(cr.Namespace).Delete(context.TODO(), svc, metav1.DeleteOptions{})
		if err != nil && !errors.IsNotFound(err) {
			logger.Error(err, "Could not delete service "+svc)
			return err
		}
	}
	return nil
}

// finalizeRedisPVC delete PVC
func finalizeRedisPVC(cr *redisv1beta1.Redis) error {
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
func finalizeRedisClusterPVC(cr *redisv1beta1.RedisCluster) error {
	logger := finalizerLogger(cr.Namespace, RedisClusterFinalizer)
	for _, role := range []string{"leader", "follower"} {
		for i := 0; i < int(cr.Spec.GetReplicaCounts(role)); i++ {
			PVCName := cr.Name + "-" + role + "-" + cr.Name + "-" + role + "-" + strconv.Itoa(i)
			err := generateK8sClient().CoreV1().PersistentVolumeClaims(cr.Namespace).Delete(context.TODO(), PVCName, metav1.DeleteOptions{})
			if err != nil && !errors.IsNotFound(err) {
				logger.Error(err, "Could not delete Persistent Volume Claim "+PVCName)
				return err
			}
		}
	}
	return nil
}
