package k8sutils

import (
	"context"
	redisv1beta1 "redis-operator/api/v1beta1"

	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

const RedisFinalizer string = "redisFinalizer"

// HandleRedisFinalizer finalize resource if instance is marked to be deleted
func HandleRedisFinalizer(cr *redisv1beta1.Redis, cl client.Client) error {
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

// finalizeRedisServices delete Services
func finalizeRedisServices(cr *redisv1beta1.Redis) error {
	serviceName, headlessServiceName := cr.Name, cr.Name+"-headless"
	for _, svc := range []string{serviceName, headlessServiceName} {
		err := generateK8sClient().CoreV1().Services(cr.Namespace).Delete(context.TODO(), svc, metav1.DeleteOptions{})
		if err != nil && !errors.IsNotFound(err) {
			return err
		}
	}
	return nil
}

// finalizeRedisPVC delete PVC
func finalizeRedisPVC(cr *redisv1beta1.Redis) error {
	PVCName := cr.Name + "-" + cr.Name + "-0"
	return generateK8sClient().CoreV1().PersistentVolumeClaims(cr.Namespace).Delete(context.TODO(), PVCName, metav1.DeleteOptions{})
}
