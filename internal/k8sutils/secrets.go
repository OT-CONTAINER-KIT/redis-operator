package k8sutils

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"errors"
	"strings"

	commonapi "github.com/OT-CONTAINER-KIT/redis-operator/api/common/v1beta2"
	"github.com/OT-CONTAINER-KIT/redis-operator/internal/util/cryptutil"
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
			logf.FromContext(ctx).V(1).Info("Secret key found in the secret", "secretKey", secretKey)
			return strings.TrimSpace(string(value)), nil
		}
	}
	logf.FromContext(ctx).Error(errors.New("secret key not found"), "Secret key not found in the secret")
	return "", nil
}

func getRedisTLSConfig(ctx context.Context, client kubernetes.Interface, namespace string, tlsConfig *commonapi.TLSConfig) *tls.Config {
	if tlsConfig == nil || tlsConfig.Secret.SecretName == "" {
		return nil
	}

	tlsSecretName := tlsConfig.Secret.SecretName
	secret, err := client.CoreV1().Secrets(namespace).Get(context.TODO(), tlsSecretName, metav1.GetOptions{})
	if err != nil {
		logf.FromContext(ctx).Error(err, "Failed in getting TLS secret", "secretName", tlsSecretName, "namespace", namespace)
		return nil
	}

	caFile, certFile, keyFile := getTLSSecretKeys(tlsConfig)
	tlsClientCert, certExists := secret.Data[certFile]
	tlsClientKey, keyExists := secret.Data[keyFile]
	tlsCaCertificate, caExists := secret.Data[caFile]

	if !certExists || !keyExists {
		logf.FromContext(ctx).Error(errors.New("required TLS cert or key is missing in the secret"), "Missing TLS cert/key in the secret")
		return nil
	}

	cert, err := tls.X509KeyPair(tlsClientCert, tlsClientKey)
	if err != nil {
		logf.FromContext(ctx).Error(err, "Couldn't load TLS client key pair", "secretName", tlsSecretName, "namespace", namespace)
		return nil
	}

	// If user explicitly set CA key and it is missing, treat this as misconfiguration.
	if !caExists && tlsConfig.CaCertFile != "" {
		logf.FromContext(ctx).Error(errors.New("configured TLS CA key file is missing in the secret"), "Missing configured TLS CA in the secret", "caKeyFile", tlsConfig.CaCertFile)
		return nil
	}

	if !caExists {
		logf.FromContext(ctx).V(1).Info("CA certificate not found in TLS secret, using system trust store", "secretName", tlsSecretName)
		return &tls.Config{
			Certificates: []tls.Certificate{cert},
			MinVersion:   tls.VersionTLS12,
			// RootCAs: nil instructs Go to use the system trust store
		}
	}

	tlsCaCertificates := x509.NewCertPool()
	ok := tlsCaCertificates.AppendCertsFromPEM(tlsCaCertificate)
	if !ok {
		logf.FromContext(ctx).Error(errors.New("invalid CA certificate"), "Invalid CA Certificates", "secretName", tlsSecretName, "namespace", namespace)
		return nil
	}

	return &tls.Config{
		Certificates:       []tls.Certificate{cert},
		RootCAs:            tlsCaCertificates,
		MinVersion:         tls.VersionTLS12,
		ClientAuth:         tls.NoClientCert,
		InsecureSkipVerify: true,
		VerifyPeerCertificate: func(rawCerts [][]byte, verifiedChains [][]*x509.Certificate) error {
			_, _, err := cryptutil.VerifyCertificateExceptServerName(rawCerts, &tls.Config{RootCAs: tlsCaCertificates})
			return err
		},
	}
}
