package bootstrap

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Test_GenerateConfig_SentinelAnnounceIP covers the fix where Sentinel announces
// its OWN pod FQDN via `sentinel announce-ip`, instead of the master's address
// that the `ip` (IP env) variable holds. The directive must only be emitted when
// both announce and resolve hostnames are enabled and the FQDN resolves.
func Test_GenerateConfig_SentinelAnnounceIP(t *testing.T) {
	const fakeFQDN = "redis-replication-sentinel-0.redis-replication-sentinel-headless.redis.svc.cluster.local"
	const masterIP = "10.0.0.5"

	tests := []struct {
		name              string
		announceHostnames string
		resolveHostnames  string
		fqdn              func() (string, error)
		expectAnnounce    bool
	}{
		{
			name:              "announce and resolve enabled emits announce-ip with the pod FQDN",
			announceHostnames: "yes",
			resolveHostnames:  "yes",
			fqdn:              func() (string, error) { return fakeFQDN, nil },
			expectAnnounce:    true,
		},
		{
			name:              "announce disabled omits announce-ip",
			announceHostnames: "no",
			resolveHostnames:  "yes",
			fqdn:              func() (string, error) { return fakeFQDN, nil },
			expectAnnounce:    false,
		},
		{
			name:              "resolve disabled omits announce-ip",
			announceHostnames: "yes",
			resolveHostnames:  "no",
			fqdn:              func() (string, error) { return fakeFQDN, nil },
			expectAnnounce:    false,
		},
		{
			name:              "fqdn resolution failure omits announce-ip",
			announceHostnames: "yes",
			resolveHostnames:  "yes",
			fqdn:              func() (string, error) { return "", errors.New("no fqdn") },
			expectAnnounce:    false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			orig := fqdnHostname
			fqdnHostname = tt.fqdn
			defer func() { fqdnHostname = orig }()

			confPath := filepath.Join(t.TempDir(), "sentinel.conf")
			t.Setenv("SENTINEL_CONFIG_FILE", confPath)
			t.Setenv("IP", masterIP) // master address consumed by `sentinel monitor`
			t.Setenv("ANNOUNCE_HOSTNAMES", tt.announceHostnames)
			t.Setenv("RESOLVE_HOSTNAMES", tt.resolveHostnames)

			require.NoError(t, GenerateConfig())

			raw, err := os.ReadFile(confPath)
			require.NoError(t, err)
			conf := string(raw)

			// The master address always feeds `sentinel monitor`.
			assert.Contains(t, conf, "sentinel monitor mymaster "+masterIP)

			if tt.expectAnnounce {
				assert.Contains(t, conf, "sentinel announce-ip "+fakeFQDN,
					"announce-ip must be the sentinel pod's own FQDN")
			} else {
				assert.NotContains(t, conf, "sentinel announce-ip")
			}
			// Regression guard: announce-ip must never be the master's IP.
			assert.NotContains(t, conf, "sentinel announce-ip "+masterIP)
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
