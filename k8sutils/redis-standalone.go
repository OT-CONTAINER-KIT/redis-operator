package k8sutils

import (
	"context"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	redisv1beta1 "redis-operator/api/v1beta1"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
)

// CreateRedisStandaloneSecret method will create a redis secret for standalone
func CreateRedisStandaloneSecret(cr *redisv1beta1.Redis) {
	reqLogger := log.WithValues("Request.Namespace", cr.Namespace, "Request.Name", cr.ObjectMeta.Name, "Setup.Type", "standalone")
	secretProp := SecretGenerator{
		Name:     cr.ObjectMeta.Name,
		Password: *cr.Spec.KubernetesConfig.Password,
		Labels: map[string]string{
			"app":       cr.ObjectMeta.Name,
			"setupType": "standalone",
		},
		Namespace: cr.Namespace,
	}
	secretBody := secretProp.GenerateSecretSpec()
	AddOwnerRefToObject(secretBody, redisAsOwner(cr))

	secretName, err := GenerateK8sClient().CoreV1().Secrets(cr.Namespace).Get(context.TODO(), cr.ObjectMeta.Name, metav1.GetOptions{})

	if err != nil {
		reqLogger.Info("Creating secret for redis", "Secret.Name", cr.ObjectMeta.Name)
		_, err := GenerateK8sClient().CoreV1().Secrets(cr.Namespace).Create(context.TODO(), secretProp.GenerateSecretSpec(), metav1.CreateOptions{})
		if err != nil {
			reqLogger.Error(err, "Failed in creating secret for redis")
		}
	} else if secretBody != secretName {
		reqLogger.Info("Reconciling secret for redis", "Secret.Name", cr.ObjectMeta.Name)
		_, err := GenerateK8sClient().CoreV1().Secrets(cr.Namespace).Update(context.TODO(), secretProp.GenerateSecretSpec(), metav1.UpdateOptions{})
		if err != nil {
			reqLogger.Error(err, "Failed in updating secret for redis")
		}
	} else {
		reqLogger.Info("Secret for redis are in sync", "Secret.Name", cr.ObjectMeta.Name)
	}
}
