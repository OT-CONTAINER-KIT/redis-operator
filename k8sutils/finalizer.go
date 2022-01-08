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

// HandleFinalizer finalize resource if instance is marked to be deleted
func HandleFinalizer(cr *redisv1beta1.Redis, cl client.Client) error {
	if cr.GetDeletionTimestamp() != nil {
		if controllerutil.ContainsFinalizer(cr, RedisFinalizer) {
			if err := finalizeServices(cr); err != nil {
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

// AddFinalizer add finalizer for graceful deletion
func AddFinalizer(cr *redisv1beta1.Redis, cl client.Client) error {
	if !controllerutil.ContainsFinalizer(cr, RedisFinalizer) {
		controllerutil.AddFinalizer(cr, RedisFinalizer)
		return cl.Update(context.TODO(), cr)
	}
	return nil
}

// finalizeServices delete Services
func finalizeServices(cr *redisv1beta1.Redis) error {
	serviceName, headlessServiceName := cr.ObjectMeta.Name, cr.ObjectMeta.Name+"-headless"
	for _, svc := range []string{serviceName, headlessServiceName} {
		err := generateK8sClient().CoreV1().Services(cr.Namespace).Delete(context.TODO(), svc, metav1.DeleteOptions{})
		if err != nil && !errors.IsNotFound(err) {
			return err
		}
	}
	return nil
}
