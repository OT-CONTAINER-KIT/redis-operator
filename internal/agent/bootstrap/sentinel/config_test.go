package bootstrap

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

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

func TestGenerateConfig_SentinelMonitorBootstrap(t *testing.T) {
	tests := []struct {
		name                 string
		ip                   string
		setIP                bool
		announceHostnames    string
		resolveHostnames     string
		wantMonitorLine      bool
		wantAnnounceIPLine   bool
		expectedMonitorValue string
	}{
		{
			name:               "omits monitor when IP is unset",
			setIP:              false,
			announceHostnames:  "no",
			resolveHostnames:   "no",
			wantMonitorLine:    false,
			wantAnnounceIPLine: false,
		},
		{
			name:               "omits monitor when IP is 0.0.0.0",
			ip:                 "0.0.0.0",
			setIP:              true,
			announceHostnames:  "yes",
			resolveHostnames:   "yes",
			wantMonitorLine:    false,
			wantAnnounceIPLine: false,
		},
		{
			name:                 "writes monitor and announce-ip when IP is valid",
			ip:                   "10.0.0.15",
			setIP:                true,
			announceHostnames:    "yes",
			resolveHostnames:     "yes",
			wantMonitorLine:      true,
			wantAnnounceIPLine:   true,
			expectedMonitorValue: "sentinel monitor mymaster 10.0.0.15 6379 2",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			confPath := filepath.Join(t.TempDir(), "sentinel.conf")
			t.Setenv("SENTINEL_CONFIG_FILE", confPath)
			t.Setenv("MASTER_GROUP_NAME", "mymaster")
			t.Setenv("PORT", "6379")
			t.Setenv("QUORUM", "2")
			t.Setenv("ANNOUNCE_HOSTNAMES", tt.announceHostnames)
			t.Setenv("RESOLVE_HOSTNAMES", tt.resolveHostnames)
			if tt.setIP {
				t.Setenv("IP", tt.ip)
			} else {
				os.Unsetenv("IP")
			}

			require.NoError(t, GenerateConfig())

			raw, err := os.ReadFile(confPath)
			require.NoError(t, err)
			conf := string(raw)

			hasMonitor := strings.Contains(conf, "\nsentinel monitor ")
			assert.Equal(t, tt.wantMonitorLine, hasMonitor, conf)
			if tt.wantMonitorLine {
				assert.Contains(t, conf, tt.expectedMonitorValue)
			}

			hasAnnounceIP := strings.Contains(conf, "sentinel announce-ip ")
			assert.Equal(t, tt.wantAnnounceIPLine, hasAnnounceIP, conf)
			if tt.wantAnnounceIPLine {
				assert.Contains(t, conf, "sentinel announce-ip "+tt.ip)
			}
		})
	}
}
