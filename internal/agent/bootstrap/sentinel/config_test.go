package bootstrap

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func Test_GenerateConfig_SentinelAnnounceIP(t *testing.T) {
	tests := []struct {
		name              string
		resolveHostnames  string
		announceHostnames string
		hostnamesEnabled  bool
	}{
		{
			name:              "both enabled - uses FQDN, never 0.0.0.0",
			resolveHostnames:  "yes",
			announceHostnames: "yes",
			hostnamesEnabled:  true,
		},
		{
			name:              "resolve only - does not set sentinel announce-ip",
			resolveHostnames:  "yes",
			announceHostnames: "no",
			hostnamesEnabled:  false,
		},
		{
			name:              "announce only - does not set sentinel announce-ip",
			resolveHostnames:  "no",
			announceHostnames: "yes",
			hostnamesEnabled:  false,
		},
		{
			name:              "both disabled - does not set sentinel announce-ip",
			resolveHostnames:  "no",
			announceHostnames: "no",
			hostnamesEnabled:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			confPath := filepath.Join(t.TempDir(), "sentinel.conf")
			t.Setenv("SENTINEL_CONFIG_FILE", confPath)
			t.Setenv("RESOLVE_HOSTNAMES", tt.resolveHostnames)
			t.Setenv("ANNOUNCE_HOSTNAMES", tt.announceHostnames)

			require.NoError(t, GenerateConfig())

			raw, err := os.ReadFile(confPath)
			require.NoError(t, err)
			conf := string(raw)

			assert.NotContains(t, conf, "sentinel announce-ip 0.0.0.0",
				"sentinel announce-ip must never be set to 0.0.0.0")

			if !tt.hostnamesEnabled {
				assert.NotContains(t, conf, "sentinel announce-ip ")
			}
		})
	}
}

func Test_GenerateConfig_TLS_CACertFile(t *testing.T) {
	tests := []struct {
		name           string
		caCertEnv      string
		setCACertEnv   bool
		expectCALine   bool
		expectedCAPath string
	}{
		{
			name:           "explicit CA cert env set - writes provided path",
			caCertEnv:      "/tls/ca.crt",
			setCACertEnv:   true,
			expectCALine:   true,
			expectedCAPath: "/tls/ca.crt",
		},
		{
			name:         "CA cert env not set - omits tls-ca-cert-file",
			setCACertEnv: false,
			expectCALine: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			confPath := filepath.Join(t.TempDir(), "sentinel.conf")

			t.Setenv("SENTINEL_CONFIG_FILE", confPath)
			t.Setenv("TLS_MODE", "true")
			t.Setenv("REDIS_TLS_CERT", "/tls/tls.crt")
			t.Setenv("REDIS_TLS_CERT_KEY", "/tls/tls.key")
			if tt.setCACertEnv {
				t.Setenv("REDIS_TLS_CA_CERT", tt.caCertEnv)
			} else {
				os.Unsetenv("REDIS_TLS_CA_CERT")
			}

			require.NoError(t, GenerateConfig())

			raw, err := os.ReadFile(confPath)
			require.NoError(t, err)
			conf := string(raw)

			// TLS should always be configured when TLS_MODE is true.
			assert.Contains(t, conf, "tls-cert-file /tls/tls.crt")
			assert.Contains(t, conf, "tls-key-file /tls/tls.key")

			if tt.expectCALine {
				assert.Contains(t, conf, "tls-ca-cert-file "+tt.expectedCAPath)
			} else {
				assert.NotContains(t, conf, "tls-ca-cert-file")
			}
		})
	}
}
