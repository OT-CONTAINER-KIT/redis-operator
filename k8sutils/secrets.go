package k8sutils

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"strings"

	redisv1beta2 "github.com/OT-CONTAINER-KIT/redis-operator/api/v1beta2"
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
			return strings.TrimSpace(string(value)), nil
		}
	}
	return "", nil
}

func secretLogger(namespace string, name string) logr.Logger {
	reqLogger := log.WithValues("Request.Secret.Namespace", namespace, "Request.Secret.Name", name)
	return reqLogger
}

func getRedisTLSConfig(cr *redisv1beta2.RedisCluster, redisInfo RedisDetails) *tls.Config {
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
			reqLogger.V(1).Info("Failed to load CA Certificates from Secret")
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

func getRedisReplicationTLSConfig(cr *redisv1beta2.RedisReplication, redisInfo RedisDetails) *tls.Config {
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
			reqLogger.V(1).Info("Failed to load CA Certificates from Secret")
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
