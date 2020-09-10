package otmachinery

import (
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	redisv1alpha1 "redis-operator/pkg/apis/redis/v1alpha1"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
)

var log = logf.Log.WithName("controller_redis")

// GenerateSecret is a method that will generate a secret interface
func GenerateSecret(cr *redisv1alpha1.Redis) *corev1.Secret {
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
func CreateRedisSecret(cr *redisv1alpha1.Redis) {
	reqLogger := log.WithValues("Request.Namespace", cr.Namespace, "Request.Name", cr.ObjectMeta.Name)
	secretBody := GenerateSecret(cr)
	secretName, err := GenerateK8sClient().CoreV1().Secrets(cr.Namespace).Get(cr.ObjectMeta.Name, metav1.GetOptions{})
	if err != nil {
		reqLogger.Info("Creating secret for redis", "Secret.Name", cr.ObjectMeta.Name)
		GenerateK8sClient().CoreV1().Secrets(cr.Namespace).Create(secretBody)
	} else if secretBody != secretName {
		reqLogger.Info("Reconciling secret for redis", "Secret.Name", cr.ObjectMeta.Name)
		GenerateK8sClient().CoreV1().Secrets(cr.Namespace).Update(secretBody)
	} else {
		reqLogger.Info("Secret for redis are in sync", "Secret.Name", cr.ObjectMeta.Name)
	}
}
