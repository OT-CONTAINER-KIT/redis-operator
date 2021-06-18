package k8sutils

import (
	"context"
	"github.com/go-logr/logr"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
)

var log = logf.Log.WithName("controller_redis")

// getRedisPassword method will return the redis password
func getRedisPassword(namespace, name, secretKey string) (string, error) {
	logger := secretLogger(namespace, name)
	secretName, err := generateK8sClient().CoreV1().Secrets(namespace).Get(context.TODO(), name, metav1.GetOptions{})
	if err != nil {
		logger.Error(err, "Failed in getting existing secret for redis")
		return "", err
	}
	for key, value := range secretName.Data {
		if key == secretKey {
			return string(value), nil
		}
	}
	return "", nil
}

func secretLogger(namespace string, name string) logr.Logger {
	reqLogger := log.WithValues("Request.Secret.Namespace", namespace, "Request.Secret.Name", name)
	return reqLogger
}
