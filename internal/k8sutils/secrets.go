package k8sutils

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"errors"
	"fmt"
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

// GetRedisTLSConfig builds the client TLS config the operator uses towards
// Redis pods from the TLS secret: the secret's certificate is presented as the
// client certificate and the peer chain is verified against its CA without
// checking the hostname. When the CA key is not set explicitly and the secret
// has no ca.crt, the chain is verified against the system trust store instead.
// It returns nil when TLS is not configured, and also when the secret cannot
// be used, after logging why; callers that require TLS must treat nil as an
// error.
func GetRedisTLSConfig(ctx context.Context, client kubernetes.Interface, namespace string, tlsConfig *commonapi.TLSConfig) *tls.Config {
	if tlsConfig == nil || tlsConfig.Secret.SecretName == "" {
		return nil
	}
	// An explicitly configured CA key that is missing is a misconfiguration;
	// an implicit one falls back to the system trust store.
	config, err := tlsConfigFromSecret(ctx, client, namespace, tlsConfig, tlsConfig.CaCertFile != "")
	if err != nil {
		logf.FromContext(ctx).Error(err, "Failed to build TLS config", "secretName", tlsConfig.Secret.SecretName, "namespace", namespace)
		return nil
	}
	return config
}

// GetSentinelTLSConfig builds the client TLS config for operator connections
// to the RedisReplication embedded Sentinel. Unlike GetRedisTLSConfig it never
// falls back to the system trust store: the Sentinel pods are configured with
// the secret's CA, and hostname verification is skipped, so trusting public
// roots would accept any publicly issued certificate. A secret without the CA,
// for example mid-rotation, is an error. It returns nil and no error when TLS
// is not configured.
func GetSentinelTLSConfig(ctx context.Context, client kubernetes.Interface, namespace string, tlsConfig *commonapi.TLSConfig) (*tls.Config, error) {
	if tlsConfig == nil {
		return nil, nil
	}
	if tlsConfig.Secret.SecretName == "" {
		return nil, errors.New("TLS secret name is not set")
	}
	return tlsConfigFromSecret(ctx, client, namespace, tlsConfig, true)
}

// tlsConfigFromSecret reads the TLS secret once, so the client certificate and
// the CA always come from the same version of it. Without requireCA a secret
// that lacks the CA key falls back to the system trust store.
func tlsConfigFromSecret(ctx context.Context, client kubernetes.Interface, namespace string, tlsConfig *commonapi.TLSConfig, requireCA bool) (*tls.Config, error) {
	tlsSecretName := tlsConfig.Secret.SecretName
	secret, err := client.CoreV1().Secrets(namespace).Get(ctx, tlsSecretName, metav1.GetOptions{})
	if err != nil {
		return nil, fmt.Errorf("get TLS secret %s/%s: %w", namespace, tlsSecretName, err)
	}

	caFile, certFile, keyFile := getTLSSecretKeys(tlsConfig)
	tlsClientCert, certExists := secret.Data[certFile]
	tlsClientKey, keyExists := secret.Data[keyFile]
	tlsCaCertificate, caExists := secret.Data[caFile]

	if !certExists || !keyExists {
		return nil, fmt.Errorf("TLS secret %s/%s is missing the %q or %q key", namespace, tlsSecretName, certFile, keyFile)
	}

	cert, err := tls.X509KeyPair(tlsClientCert, tlsClientKey)
	if err != nil {
		return nil, fmt.Errorf("load TLS client key pair from secret %s/%s: %w", namespace, tlsSecretName, err)
	}

	if !caExists {
		if requireCA {
			return nil, fmt.Errorf("TLS secret %s/%s is missing the CA key %q", namespace, tlsSecretName, caFile)
		}
		logf.FromContext(ctx).V(1).Info("CA certificate not found in TLS secret, using system trust store", "secretName", tlsSecretName)
		systemCertPool, err := x509.SystemCertPool()
		if err != nil {
			return nil, fmt.Errorf("load system certificate pool: %w", err)
		}
		return newTLSConfigVerifyingChainWithoutHostname(cert, systemCertPool), nil
	}

	tlsCaCertificates := x509.NewCertPool()
	if !tlsCaCertificates.AppendCertsFromPEM(tlsCaCertificate) {
		return nil, fmt.Errorf("TLS secret %s/%s has no valid PEM certificate under the CA key %q", namespace, tlsSecretName, caFile)
	}

	return newTLSConfigVerifyingChainWithoutHostname(cert, tlsCaCertificates), nil
}

// newTLSConfigVerifyingChainWithoutHostname builds a client *tls.Config that
// trusts the supplied root pool and verifies the peer certificate chain WITHOUT
// checking the server name. The operator dials Redis pods by IP / pod DNS that
// does not match the server certificate SNI, so Go's default hostname
// verification is disabled (InsecureSkipVerify) and the chain is instead
// validated by VerifyPeerCertificate against the provided roots.
func newTLSConfigVerifyingChainWithoutHostname(cert tls.Certificate, rootCAs *x509.CertPool) *tls.Config {
	return &tls.Config{
		Certificates:       []tls.Certificate{cert},
		RootCAs:            rootCAs,
		MinVersion:         tls.VersionTLS12,
		ClientAuth:         tls.NoClientCert,
		InsecureSkipVerify: true, // skips default verification; chain re-verified without hostname below
		VerifyPeerCertificate: func(rawCerts [][]byte, verifiedChains [][]*x509.Certificate) error {
			_, _, err := cryptutil.VerifyCertificateExceptServerName(rawCerts, &tls.Config{RootCAs: rootCAs})
			return err
		},
	}
}
