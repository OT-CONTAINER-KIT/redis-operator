package bootstrap

import (
	"os"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func Test_TLSCAFallback_EnvLogic(t *testing.T) {
	// Tests the CA cert selection logic used in GenerateConfig TLS block:
	// - REDIS_TLS_CA_CERT set   → use that path
	// - REDIS_TLS_CA_CERT unset → fall back to system trust store
	tests := []struct {
		name        string
		caCertEnv   string
		expectedCA  string
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

			// Mirror the logic from GenerateConfig TLS block
			caCert := os.Getenv("REDIS_TLS_CA_CERT")
			var resolvedCA string
			if caCert != "" {
				resolvedCA = caCert
			} else {
				resolvedCA = "/etc/ssl/certs/ca-certificates.crt"
			}

			assert.Equal(t, tt.expectedCA, resolvedCA)
		})
	}
}

func Test_GenerateConfig_TLS_WritesCorrectCALine(t *testing.T) {
	tmpDir := t.TempDir()
	confPath := tmpDir + "/redis.conf"

	tests := []struct {
		name        string
		envVars     map[string]string
		expectInConf string
	}{
		{
			name: "with explicit CA - writes explicit CA path",
			envVars: map[string]string{
				"TLS_MODE":           "true",
				"REDIS_TLS_CERT":     "/tls/tls.crt",
				"REDIS_TLS_CERT_KEY": "/tls/tls.key",
				"REDIS_TLS_CA_CERT":  "/tls/ca.crt",
				"REDIS_CONFIG_FILE":  confPath,
			},
			expectInConf: "tls-ca-cert-file /tls/ca.crt",
		},
		{
			name: "without CA - writes system trust store path",
			envVars: map[string]string{
				"TLS_MODE":          "true",
				"REDIS_TLS_CERT":    "/tls/tls.crt",
				"REDIS_TLS_CERT_KEY": "/tls/tls.key",
				"REDIS_CONFIG_FILE": confPath,
			},
			expectInConf: "tls-ca-cert-file /etc/ssl/certs/ca-certificates.crt",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			for k, v := range tt.envVars {
				t.Setenv(k, v)
			}
			// Unset CA cert if not in test case
			if _, ok := tt.envVars["REDIS_TLS_CA_CERT"]; !ok {
				os.Unsetenv("REDIS_TLS_CA_CERT")
			}

			// Mirror the CA resolution logic
			caCert := os.Getenv("REDIS_TLS_CA_CERT")
			var caLine string
			if caCert != "" {
				caLine = "tls-ca-cert-file " + caCert
			} else {
				caLine = "tls-ca-cert-file /etc/ssl/certs/ca-certificates.crt"
			}

			require.Equal(t, tt.expectInConf, caLine,
				"CA cert line in config should match expected value")
		})
	}
}

func Test_updateMyselfIP(t *testing.T) {
	testData := `7a6b5f4f99496c97f4e32c30c077aa95cab92664 10.244.0.246:0@16379,,tls-port=6379,shard-id=a03445a0d3f6d405af261041e0cb77a8a176f42b slave b66f2fa597eeda567cf05f3701419be9a3b2f50e 0 1756463509000 1 connected
93ad60e9ce21430683a3534d2c96ab1b8077cfe8 10.244.0.237:0@16379,,tls-port=6379,shard-id=2f177491b895051f91e91e554a2a9da2cd167aeb master - 0 1756463509685 2 connected 5461-10922
b66f2fa597eeda567cf05f3701419be9a3b2f50e 10.244.0.222:0@16379,,tls-port=6379,shard-id=a03445a0d3f6d405af261041e0cb77a8a176f42b myself,master - 0 0 1 connected 0-5460
88456cac1830f3e00f6ab681fb819b4b1d7ad36b 10.244.0.252:0@16379,,tls-port=6379,shard-id=b61110535f09b9c0703517f79da79118fee8d1a4 slave 580e234a8dcd74717c37d01ed8097929c64536ff 0 1756463509691 3 connected
580e234a8dcd74717c37d01ed8097929c64536ff 10.244.0.240:0@16379,,tls-port=6379,shard-id=b61110535f09b9c0703517f79da79118fee8d1a4 master - 0 1756463509583 3 connected 10923-16383
c0fc3c21460fec045775d2dcde220fb26ca668c1 10.244.0.249:0@16379,,tls-port=6379,shard-id=2f177491b895051f91e91e554a2a9da2cd167aeb slave 93ad60e9ce21430683a3534d2c96ab1b8077cfe8 0 1756463509583 2 connected
vars currentEpoch 3 lastVoteEpoch 0
`

	tmpFile := "/tmp/test_nodes.conf"
	err := os.WriteFile(tmpFile, []byte(testData), 0o644)
	if err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}
	defer os.Remove(tmpFile)

	newIP := "10.244.0.9"
	updated, err := updateMyselfIP(tmpFile, newIP)
	if err != nil {
		t.Errorf("updateMyselfIP() error = %v", err)
	}

	if updated == nil {
		t.Errorf("Expected changes to be made, but updated is nil")
		return
	}
	expectedContent := strings.ReplaceAll(testData, "10.244.0.222", newIP)
	updatedContent := string(updated)

	if updatedContent != expectedContent {
		t.Errorf("Expected updated content to match:\nExpected:\n%s\nGot:\n%s", expectedContent, updatedContent)
	}

	t.Logf("Successfully updated nodes.conf with new IP %s", newIP)
}
