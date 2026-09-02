package k8sutils

import commonapi "github.com/OT-CONTAINER-KIT/redis-operator/api/common/v1beta2"

const (
	defaultTLSCAKeyFile   = "ca.crt"
	defaultTLSCertKeyFile = "tls.crt"
	defaultTLSKeyFile     = "tls.key"
)

func tlsKeyOrDefault(override, fallback string) string {
	if override != "" {
		return override
	}
	return fallback
}

func getTLSSecretKeys(tlsConfig *commonapi.TLSConfig) (caFile, certFile, keyFile string) {
	if tlsConfig == nil {
		return defaultTLSCAKeyFile, defaultTLSCertKeyFile, defaultTLSKeyFile
	}

	caFile = tlsKeyOrDefault(tlsConfig.CaCertFile, defaultTLSCAKeyFile)
	certFile = tlsKeyOrDefault(tlsConfig.CertKeyFile, defaultTLSCertKeyFile)
	keyFile = tlsKeyOrDefault(tlsConfig.KeyFile, defaultTLSKeyFile)
	return caFile, certFile, keyFile
}
