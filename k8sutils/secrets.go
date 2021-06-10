package k8sutils

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	redisv1beta1 "redis-operator/api/v1beta1"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
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

func getRedisTLSConfig(cr *redisv1beta1.Redis, redisInfo RedisDetails) *tls.Config {
	if cr.Spec.GlobalConfig.TLS != nil {
		reqLogger := log.WithValues("Request.Namespace", cr.Namespace, "Request.Name", cr.ObjectMeta.Name)
		secretName, err := GenerateK8sClient().CoreV1().Secrets(cr.Namespace).Get(context.TODO(), cr.Spec.GlobalConfig.TLS.Secret.SecretName, metav1.GetOptions{})
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
			if key == cr.Spec.GlobalConfig.TLS.CaKeyFile || key == "ca.crt" {
				tlsCaCertificate = value
			} else if key == cr.Spec.GlobalConfig.TLS.CertKeyFile || key == "tls.key" {
				tlsClientKey = value
			} else if key == cr.Spec.GlobalConfig.TLS.KeyFile || key == "tls.crt" {
				tlsClientCert = value
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
