package k8sutils

import (
	"context"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	redisv1beta1 "redis-operator/api/v1beta1"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
)

var log = logf.Log.WithName("controller_redis")

// GenerateSecret is a method that will generate a secret interface
func GenerateSecret(cr *redisv1beta1.Redis) *corev1.Secret {
	password := []byte(*cr.Spec.GlobalConfig.Password)
	labels := map[string]string{
		"app": cr.ObjectMeta.Name,
	}
	secret := &corev1.Secret{
		TypeMeta:   GenerateMetaInformation("Secret", "v1"),
		ObjectMeta: GenerateObjectMetaInformation(cr.ObjectMeta.Name, cr.Namespace, labels, GenerateSecretAnots()),
		Data: map[string][]byte{
			"password": password,
		},
	}
	AddOwnerRefToObject(secret, AsOwner(cr))
	return secret
}

// CreateRedisSecret method will create a redis secret
func CreateRedisSecret(cr *redisv1beta1.Redis) {
	reqLogger := log.WithValues("Request.Namespace", cr.Namespace, "Request.Name", cr.ObjectMeta.Name)
	secretBody := GenerateSecret(cr)
	secretName, err := GenerateK8sClient().CoreV1().Secrets(cr.Namespace).Get(context.TODO(), cr.ObjectMeta.Name, metav1.GetOptions{})
	if err != nil {
		reqLogger.Info("Creating secret for redis", "Secret.Name", cr.ObjectMeta.Name)
		_, err := GenerateK8sClient().CoreV1().Secrets(cr.Namespace).Create(context.TODO(), secretBody, metav1.CreateOptions{})
		if err != nil {
			reqLogger.Error(err, "Failed in creating secret for redis")
		}
	} else if secretBody != secretName {
		reqLogger.Info("Reconciling secret for redis", "Secret.Name", cr.ObjectMeta.Name)
		_, err := GenerateK8sClient().CoreV1().Secrets(cr.Namespace).Update(context.TODO(), secretBody, metav1.UpdateOptions{})
		if err != nil {
			reqLogger.Error(err, "Failed in updating secret for redis")
		}
	} else {
		reqLogger.Info("Secret for redis are in sync", "Secret.Name", cr.ObjectMeta.Name)
	}
}

// getRedisPassword method will return the redis password
func getRedisPassword(cr *redisv1beta1.Redis) string {
	reqLogger := log.WithValues("Request.Namespace", cr.Namespace, "Request.Name", cr.ObjectMeta.Name)
	secretName, err := GenerateK8sClient().CoreV1().Secrets(cr.Namespace).Get(context.TODO(), *cr.Spec.GlobalConfig.ExistingPasswordSecret.Name, metav1.GetOptions{})
	if err != nil {
		reqLogger.Error(err, "Failed in getting existing secret for redis")
	}
	for key, value := range secretName.Data {
		if key == *cr.Spec.GlobalConfig.ExistingPasswordSecret.Key {
			return string(value)
		}
	}
	return ""
}
