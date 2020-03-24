package otmachinery

import (
	"k8s.io/client-go/rest"
	"k8s.io/client-go/kubernetes"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	redisv1alpha1 "redis-operator/redis-operator/pkg/apis/redis/v1alpha1"
)

var log = logf.Log.WithName("controller_redis")

// GenerateSecret is a method that will generate a secret interface
func GenerateSecret(cr *redisv1alpha1.Redis) *corev1.Secret {
	password := []byte(*cr.Spec.RedisPassword)
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
	return secret
}

// CreateRedisSecret method will create a redis secret
func CreateRedisSecret(cr *redisv1alpha1.Redis) {
	reqLogger := log.WithValues("Request.Namespace", cr.Namespace, "Request.Name", cr.ObjectMeta.Name)
	config, _ := rest.InClusterConfig()
	clientset, _ := kubernetes.NewForConfig(config)
	secretBody := GenerateSecret(cr)
	secretName, err := clientset.CoreV1().Secrets(cr.Namespace).Get(cr.ObjectMeta.Name, metav1.GetOptions{})
	if err != nil {
		reqLogger.Info("Creating secret for redis", "Secret.Name", cr.ObjectMeta.Name)
		clientset.CoreV1().Secrets(cr.Namespace).Create(secretBody)
	} else if secretBody != secretName {
		reqLogger.Info("Updating secret for redis", "Secret.Name", cr.ObjectMeta.Name)
		clientset.CoreV1().Secrets(cr.Namespace).Update(secretBody)
	} else {
		reqLogger.Info("Secret for redis are in sync", "Secret.Name", cr.ObjectMeta.Name)
	}
}
