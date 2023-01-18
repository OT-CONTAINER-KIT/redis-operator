package k8sutils

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	redisv1beta1 "redis-operator/api/v1beta1"
	"strings"

	"github.com/go-logr/logr"
	corev1 "k8s.io/api/core/v1"
	kerror "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
)

type RedisSecretParams struct {
	name       string
	namespace  string
	key        string
	value      []byte
	ownerRef   metav1.OwnerReference
	ownerNS    string
	labels     map[string]string
	annotation map[string]string
}

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

func createSecretIfNotExist(secretParams RedisSecretParams) error {
	//Create a secret template and adding name, namespace, key and value
	secret := generateSecretTemplate(secretParams)

	genLogger := secretLogger(secretParams.namespace, secretParams.name)

	_, err := getSecret(secretParams.namespace, secretParams.name)
	if err != nil {
		if kerror.IsNotFound(err) {
			_, err := generateK8sClient().CoreV1().Secrets(secretParams.namespace).Create(context.TODO(), secret, metav1.CreateOptions{})
			if err != nil {
				genLogger.Error(err, "Failed to create the Secrets by the operator in ", "namespaces", secretParams.namespace)
				return err
			} else {
				genLogger.Info("Secret Created Successfully in ", "namespace", secretParams.namespace)
			}

		} else {
			genLogger.Error(err, "Miscellaneous error found in while getting the secret")
			return err
		}
	}
	return nil
}

func generateSecretTemplate(secretParams RedisSecretParams) *corev1.Secret {

	secret := &corev1.Secret{
		TypeMeta: metav1.TypeMeta{
			Kind:       "Secret",
			APIVersion: "v1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:        secretParams.name,
			Namespace:   secretParams.namespace,
			Labels:      secretParams.labels,
			Annotations: secretParams.annotation,
		},

		Data: map[string][]byte{
			secretParams.key: secretParams.value,
		},

		Type: "Opaque",
	}

	// Add owner reference to secret if exist in same namespace
	if secretParams.namespace == secretParams.ownerNS {
		AddOwnerRefToObject(secret, secretParams.ownerRef)
	}

	return secret
}

// GetStateFulSet is a method to get statefulset in Kubernetes
func getSecret(namespace string, name string) (*corev1.Secret, error) {
	logger := secretLogger(namespace, name)
	getOpts := metav1.GetOptions{
		TypeMeta: generateMetaInformation("Secret", "v1"),
	}
	secretInfo, err := generateK8sClient().CoreV1().Secrets(namespace).Get(context.TODO(), name, getOpts)

	if err != nil {
		logger.Info("Redis secret get action failed")
		return nil, err
	}
	logger.Info("Redis secret get action was successful")
	return secretInfo, nil
}
