package k8sutils

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"errors"
	"strings"

	redisv1beta2 "github.com/OT-CONTAINER-KIT/redis-operator/api/v1beta2"
	"github.com/go-logr/logr"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
)

var log = logf.Log.WithName("controller_redis")

// getRedisPassword method will return the redis password from the secret
func getRedisPassword(client kubernetes.Interface, logger logr.Logger, namespace, name, secretKey string) (string, error) {
	secretName, err := client.CoreV1().Secrets(namespace).Get(context.TODO(), name, metav1.GetOptions{})
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

func getRedisTLSConfig(client kubernetes.Interface, logger logr.Logger, cr *redisv1beta2.RedisCluster, redisInfo RedisDetails) *tls.Config {
	if cr.Spec.TLS != nil {
		secret, err := client.CoreV1().Secrets(cr.Namespace).Get(context.TODO(), cr.Spec.TLS.Secret.SecretName, metav1.GetOptions{})
		if err != nil {
			logger.Error(err, "Failed in getting TLS secret for redis cluster")
			logger.V(1).Error(err, "Failed in getting TLS secret for redis cluster", "secretName", cr.Spec.TLS.Secret.SecretName, "namespace", cr.Namespace, "redisClusterName", cr.Name)
			return nil
		}

		tlsClientCert, certExists := secret.Data["tls.crt"]
		tlsClientKey, keyExists := secret.Data["tls.key"]
		tlsCaCertificate, caExists := secret.Data["ca.crt"]

		if !certExists || !keyExists || !caExists {
			logger.Error(errors.New("required TLS keys are missing in the secret"), "Missing TLS keys in the secret")
			return nil
		}

		cert, err := tls.X509KeyPair(tlsClientCert, tlsClientKey)
		if err != nil {
			logger.Error(err, "Couldn't load TLS client key pair")
			logger.V(1).Error(err, "Couldn't load TLS client key pair", "secretName", cr.Spec.TLS.Secret.SecretName, "namespace", cr.Namespace, "redisClusterName", cr.Name)
			return nil
		}

		tlsCaCertificates := x509.NewCertPool()
		ok := tlsCaCertificates.AppendCertsFromPEM(tlsCaCertificate)
		if !ok {
			logger.Error(errors.New("failed to load CA Certificates from secret"), "Invalid CA Certificates")
			logger.V(1).Error(err, "Invalid CA Certificates", "secretName", cr.Spec.TLS.Secret.SecretName, "namespace", cr.Namespace, "redisClusterName", cr.Name)
			return nil
		}

		return &tls.Config{
			Certificates: []tls.Certificate{cert},
			ServerName:   redisInfo.PodName,
			RootCAs:      tlsCaCertificates,
			MinVersion:   tls.VersionTLS12,
			ClientAuth:   tls.NoClientCert,
		}
	}
	return nil
}

func getRedisReplicationTLSConfig(client kubernetes.Interface, logger logr.Logger, cr *redisv1beta2.RedisReplication, redisInfo RedisDetails) *tls.Config {
	if cr.Spec.TLS != nil {
		secret, err := client.CoreV1().Secrets(cr.Namespace).Get(context.TODO(), cr.Spec.TLS.Secret.SecretName, metav1.GetOptions{})
		if err != nil {
			logger.Error(err, "Failed in getting TLS secret for redis replication")
			logger.V(1).Error(err, "Failed in getting TLS secret for redis replication", "secretName", cr.Spec.TLS.Secret.SecretName, "namespace", cr.Namespace, "redisReplicationName", cr.Name)
			return nil
		}

		tlsClientCert, certExists := secret.Data["tls.crt"]
		tlsClientKey, keyExists := secret.Data["tls.key"]
		tlsCaCertificate, caExists := secret.Data["ca.crt"]

		if !certExists || !keyExists || !caExists {
			logger.Error(errors.New("required TLS keys are missing in the secret"), "Missing TLS keys in the secret")
			return nil
		}

		cert, err := tls.X509KeyPair(tlsClientCert, tlsClientKey)
		if err != nil {
			logger.Error(err, "Couldn't load TLS client key pair")
			logger.V(1).Error(err, "Couldn't load TLS client key pair", "secretName", cr.Spec.TLS.Secret.SecretName, "namespace", cr.Namespace, "redisReplicationName", cr.Name)
			return nil
		}

		tlsCaCertificates := x509.NewCertPool()
		ok := tlsCaCertificates.AppendCertsFromPEM(tlsCaCertificate)
		if !ok {
			logger.Error(errors.New("failed to load CA Certificates from secret"), "Invalid CA Certificates")
			logger.V(1).Error(err, "Invalid CA Certificates", "secretName", cr.Spec.TLS.Secret.SecretName, "namespace", cr.Namespace, "redisReplicationName", cr.Name)
			return nil
		}

		return &tls.Config{
			Certificates: []tls.Certificate{cert},
			ServerName:   redisInfo.PodName,
			RootCAs:      tlsCaCertificates,
			MinVersion:   tls.VersionTLS12,
			ClientAuth:   tls.NoClientCert,
		}
	}
	return nil
}
