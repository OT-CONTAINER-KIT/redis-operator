package k8sutils

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"errors"
	"strings"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
)

// getRedisPassword method will return the redis password from the secret
func getRedisPassword(ctx context.Context, client kubernetes.Interface, namespace, name, secretKey string) (string, error) {
	secretName, err := client.CoreV1().Secrets(namespace).Get(context.TODO(), name, metav1.GetOptions{})
	if err != nil {
		logf.FromContext(ctx).Error(err, "Failed in getting existing secret for redis")
		return "", err
	}
	for key, value := range secretName.Data {
		if key == secretKey {
			logf.FromContext(ctx).Info("Secret key found in the secret", "secretKey", secretKey)
			return strings.TrimSpace(string(value)), nil
		}
	}
	logf.FromContext(ctx).Error(errors.New("secret key not found"), "Secret key not found in the secret")
	return "", nil
}

func getRedisTLSConfig(ctx context.Context, client kubernetes.Interface, namespace, tlsSecretName, podName string) *tls.Config {
	secret, err := client.CoreV1().Secrets(namespace).Get(context.TODO(), tlsSecretName, metav1.GetOptions{})
	if err != nil {
		logf.FromContext(ctx).Error(err, "Failed in getting TLS secret", "secretName", tlsSecretName, "namespace", namespace)
		return nil
	}

	tlsClientCert, certExists := secret.Data["tls.crt"]
	tlsClientKey, keyExists := secret.Data["tls.key"]
	tlsCaCertificate, caExists := secret.Data["ca.crt"]

	if !certExists || !keyExists || !caExists {
		logf.FromContext(ctx).Error(errors.New("required TLS keys are missing in the secret"), "Missing TLS keys in the secret")
		return nil
	}

	cert, err := tls.X509KeyPair(tlsClientCert, tlsClientKey)
	if err != nil {
		logf.FromContext(ctx).Error(err, "Couldn't load TLS client key pair", "secretName", tlsSecretName, "namespace", namespace)
		return nil
	}

	tlsCaCertificates := x509.NewCertPool()
	ok := tlsCaCertificates.AppendCertsFromPEM(tlsCaCertificate)
	if !ok {
		logf.FromContext(ctx).Error(err, "Invalid CA Certificates", "secretName", tlsSecretName, "namespace", namespace)
		return nil
	}

	return &tls.Config{
		Certificates: []tls.Certificate{cert},
		ServerName:   podName,
		RootCAs:      tlsCaCertificates,
		MinVersion:   tls.VersionTLS12,
		ClientAuth:   tls.NoClientCert,
	}
}
