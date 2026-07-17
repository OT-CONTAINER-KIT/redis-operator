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
			confPath := filepath.Join(t.TempDir(), "redis.conf")

			t.Setenv("REDIS_CONFIG_FILE", confPath)
			t.Setenv("TLS_MODE", "true")
			t.Setenv("REDIS_TLS_CERT", "/tls/tls.crt")
			t.Setenv("REDIS_TLS_CERT_KEY", "/tls/tls.key")
			// Keep the run in standalone mode so GenerateConfig does not try to
			// reach the network / read nodes.conf for cluster bootstrapping.
			t.Setenv("SETUP_MODE", "standalone")
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

func Test_GenerateConfig_ClusterVersionGating(t *testing.T) {
	// cluster-preferred-endpoint-type is the cleanest directive to assert the
	// "v7 or newer" gating on: unlike cluster-announce-hostname it does not
	// depend on resolving the host FQDN, so it is emitted deterministically
	// whenever TLS + cluster mode + a v7-or-newer version + non-NodePort line
	// up. This is exactly the behaviour that silently regressed on v8 before
	// the version check stopped being an exact "v7" string comparison.
	tests := []struct {
		name                 string
		majorVersion         string
		expectPreferredHost  bool
		expectAnnounceAbsent bool
	}{
		{
			name:                 "v6 keeps the legacy IP-based behaviour",
			majorVersion:         "v6",
			expectPreferredHost:  false,
			expectAnnounceAbsent: true,
		},
		{
			name:                "v7 enables hostname-based discovery",
			majorVersion:        "v7",
			expectPreferredHost: true,
		},
		{
			name:                "v8 enables hostname-based discovery",
			majorVersion:        "v8",
			expectPreferredHost: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			confPath := filepath.Join(t.TempDir(), "redis.conf")

			t.Setenv("REDIS_CONFIG_FILE", confPath)
			t.Setenv("SETUP_MODE", "cluster")
			t.Setenv("NODE_CONF_DIR", t.TempDir())
			t.Setenv("REDIS_MAJOR_VERSION", tt.majorVersion)
			t.Setenv("NODEPORT", "false")
			t.Setenv("TLS_MODE", "true")
			t.Setenv("REDIS_TLS_CERT", "/tls/tls.crt")
			t.Setenv("REDIS_TLS_CERT_KEY", "/tls/tls.key")

			require.NoError(t, GenerateConfig())

			raw, err := os.ReadFile(confPath)
			require.NoError(t, err)
			conf := string(raw)

			if tt.expectPreferredHost {
				assert.Contains(t, conf, "cluster-preferred-endpoint-type hostname")
			} else {
				assert.NotContains(t, conf, "cluster-preferred-endpoint-type")
			}
			if tt.expectAnnounceAbsent {
				assert.NotContains(t, conf, "cluster-announce-hostname")
			}
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
