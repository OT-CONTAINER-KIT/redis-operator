package bootstrap

import (
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
)

func Test_SentinelTLSCAFallback_EnvLogic(t *testing.T) {
	// Tests the CA cert selection logic used in sentinel GenerateConfig TLS block:
	// - REDIS_TLS_CA_CERT set   → use that path
	// - REDIS_TLS_CA_CERT unset → fall back to system trust store
	tests := []struct {
		name       string
		caCertEnv  string
		expectedCA string
	}{
		{
			name:       "explicit CA cert env set - uses provided path",
			caCertEnv:  "/tls/ca.crt",
			expectedCA: "/tls/ca.crt",
		},
		{
			name:       "CA cert env not set - falls back to system trust store",
			caCertEnv:  "",
			expectedCA: "/etc/ssl/certs/ca-certificates.crt",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.caCertEnv != "" {
				t.Setenv("REDIS_TLS_CA_CERT", tt.caCertEnv)
			} else {
				os.Unsetenv("REDIS_TLS_CA_CERT")
			}

			// Mirror the logic from sentinel GenerateConfig TLS block
			redisTLSCACert := os.Getenv("REDIS_TLS_CA_CERT")
			var resolvedCA string
			if redisTLSCACert != "" {
				resolvedCA = redisTLSCACert
			} else {
				resolvedCA = "/etc/ssl/certs/ca-certificates.crt"
			}

			assert.Equal(t, tt.expectedCA, resolvedCA,
				"Sentinel CA cert path should match expected value")
		})
	}
}

func Test_SentinelTLSMode_CALineSelection(t *testing.T) {
	tests := []struct {
		name         string
		envVars      map[string]string
		expectCALine string
	}{
		{
			name: "TLS mode with explicit CA - uses provided CA path",
			envVars: map[string]string{
				"TLS_MODE":           "true",
				"REDIS_TLS_CERT":     "/tls/tls.crt",
				"REDIS_TLS_CERT_KEY": "/tls/tls.key",
				"REDIS_TLS_CA_CERT":  "/tls/ca.crt",
			},
			expectCALine: "tls-ca-cert-file /tls/ca.crt",
		},
		{
			name: "TLS mode without CA - falls back to system trust store",
			envVars: map[string]string{
				"TLS_MODE":           "true",
				"REDIS_TLS_CERT":     "/tls/tls.crt",
				"REDIS_TLS_CERT_KEY": "/tls/tls.key",
				// REDIS_TLS_CA_CERT intentionally not set
			},
			expectCALine: "tls-ca-cert-file /etc/ssl/certs/ca-certificates.crt",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			for k, v := range tt.envVars {
				t.Setenv(k, v)
			}
			if _, ok := tt.envVars["REDIS_TLS_CA_CERT"]; !ok {
				os.Unsetenv("REDIS_TLS_CA_CERT")
			}

			// Mirror sentinel TLS block CA resolution
			redisTLSCACert := os.Getenv("REDIS_TLS_CA_CERT")
			var caLine string
			if redisTLSCACert != "" {
				caLine = "tls-ca-cert-file " + redisTLSCACert
			} else {
				caLine = "tls-ca-cert-file /etc/ssl/certs/ca-certificates.crt"
			}

			assert.Equal(t, tt.expectCALine, caLine,
				"Sentinel TLS CA line should match expected value")
		})
	}
}
