package k8sutils

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	redisv1beta1 "redis-operator/api/v1beta1"
	"strings"

	"github.com/go-logr/logr"
	"github.com/google/uuid"
	corev1 "k8s.io/api/core/v1"
	kerror "k8s.io/apimachinery/pkg/api/errors"
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
			return strings.TrimSpace(string(value)), nil
		}
	}
	return "", nil
}

func secretLogger(namespace string, name string) logr.Logger {
	reqLogger := log.WithValues("Request.Secret.Namespace", namespace, "Request.Secret.Name", name)
	return reqLogger
}

func getRedisTLSConfig(cr *redisv1beta1.RedisCluster, redisInfo RedisDetails) *tls.Config {
	if cr.Spec.TLS != nil {
		reqLogger := log.WithValues("Request.Namespace", cr.Namespace, "Request.Name", cr.ObjectMeta.Name)
		secretName, err := generateK8sClient().CoreV1().Secrets(cr.Namespace).Get(context.TODO(), cr.Spec.TLS.Secret.SecretName, metav1.GetOptions{})
		if err != nil {
			reqLogger.Error(err, "Failed in getting TLS secret for redis")
		}

		var (
			tlsClientCert         []byte
			tlsClientKey          []byte
			tlsCaCertificate      []byte
			tlsCaCertificates     *x509.CertPool
			tlsClientCertificates []tls.Certificate
		)
		for key, value := range secretName.Data {
			if key == cr.Spec.TLS.CaKeyFile || key == "ca.crt" {
				tlsCaCertificate = value
			} else if key == cr.Spec.TLS.CertKeyFile || key == "tls.crt" {
				tlsClientCert = value
			} else if key == cr.Spec.TLS.KeyFile || key == "tls.key" {
				tlsClientKey = value
			}
		}

		cert, err := tls.X509KeyPair(tlsClientCert, tlsClientKey)
		if err != nil {
			reqLogger.Error(err, "Couldn't load TLS client key pair")
		}
		tlsClientCertificates = append(tlsClientCertificates, cert)

		tlsCaCertificates = x509.NewCertPool()
		ok := tlsCaCertificates.AppendCertsFromPEM(tlsCaCertificate)
		if !ok {
			reqLogger.Info("Failed to load CA Certificates from Secret")
		}

		return &tls.Config{
			Certificates: tlsClientCertificates,
			ServerName:   redisInfo.PodName,
			RootCAs:      tlsCaCertificates,
			MinVersion:   2,
			ClientAuth:   0,
		}
	}
	return nil
}

//func GenerateSecrets(name string, namespacelist []string, key *string, ownerRef metav1.OwnerReference) error {
func GenerateSecrets(instance *redisv1beta1.RedisCluster) error {
	var name = *instance.Spec.KubernetesConfig.ExistOrGenerateSecret.GeneratePasswordSecret.Name
	var namespacelist = instance.Spec.KubernetesConfig.ExistOrGenerateSecret.GeneratePasswordSecret.NameSpace
	var key = instance.Spec.KubernetesConfig.ExistOrGenerateSecret.GeneratePasswordSecret.Key

	genLogger := log.WithValues()

	rndID, err := uuid.NewRandom()
	if err != nil {
		genLogger.Error(err, "Unable to generate the UUID")
	}
	// If key is empty add the default value
	if key == nil {
		*key = "key"
	}

	// If no namespacelist is defined default would be added.
	if namespacelist == nil {
		namespacelist = append(namespacelist, "default")
	}

	// Key and Value for the secret
	value := rndID.NodeID()

	for _, namespace := range namespacelist {

		generatedSecretTemplate := generateSecretTemplate()
		generatedSecretTemplate.Name = name
		generatedSecretTemplate.Namespace = namespace
		generatedSecretTemplate.Data = map[string][]byte{
			*key: value,
		}

		AddOwnerRefToObject(generatedSecretTemplate, redisClusterAsOwner(instance))

		// Check whether the secret exist or not If not then create it
		_, err := generateK8sClient().CoreV1().Secrets(namespace).Get(context.Background(), name, metav1.GetOptions{})
		if kerror.IsNotFound(err) {
			_, err := generateK8sClient().CoreV1().Secrets(namespace).Create(context.Background(), generatedSecretTemplate, metav1.CreateOptions{})
			if err != nil {
				genLogger.Error(err, "Failed to create the Secrets by the operator")
				return err
			}
		} else {
			return err
		}

	}

	return nil

}

func generateSecretTemplate() *corev1.Secret {

	return &corev1.Secret{
		TypeMeta: metav1.TypeMeta{
			Kind:       "Secret",
			APIVersion: "v1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      "",
			Namespace: "",
		},

		Data: map[string][]byte{},

		Type: "Opaque",
	}

}
