package bootstrap

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func Test_GenerateConfig_SentinelAnnounceIP(t *testing.T) {
	const fakeFQDN = "redis-replication-s-0.redis-replication-s-hl.ns.svc.cluster.local"

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
			orig := fqdnHostname
			fqdnHostname = func() (string, error) { return fakeFQDN, nil }
			t.Cleanup(func() { fqdnHostname = orig })

			confPath := filepath.Join(t.TempDir(), "sentinel.conf")
			t.Setenv("SENTINEL_CONFIG_FILE", confPath)
			t.Setenv("RESOLVE_HOSTNAMES", tt.resolveHostnames)
			t.Setenv("ANNOUNCE_HOSTNAMES", tt.announceHostnames)
			// Pin the guard below to the real pre-fix value rather than the default.
			t.Setenv("IP", "0.0.0.0")

			require.NoError(t, GenerateConfig())

			raw, err := os.ReadFile(confPath)
			require.NoError(t, err)
			conf := string(raw)

			assert.NotContains(t, conf, "sentinel announce-ip 0.0.0.0",
				"sentinel announce-ip must never be set to 0.0.0.0")

			if tt.hostnamesEnabled {
				// Positive assertion: without this an implementation that stopped
				// emitting announce-ip altogether would still satisfy the test.
				assert.Contains(t, conf, "sentinel announce-ip "+fakeFQDN)
			} else {
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

func Test_GenerateConfig_ExternalConfig_ExpandsEnvPlaceholders(t *testing.T) {
	dir := t.TempDir()
	confPath := filepath.Join(dir, "sentinel.conf")
	externalPath := filepath.Join(dir, "redis-sentinel-additional.conf")

	// SENTINEL_LOGLEVEL is not something GenerateConfig writes itself, so the
	// value can only reach the config through expansion of the external file.
	external := "loglevel ${SENTINEL_LOGLEVEL}\nsentinel deny-scripts-reconfig no\n"
	require.NoError(t, os.WriteFile(externalPath, []byte(external), 0o644))

	t.Setenv("SENTINEL_CONFIG_FILE", confPath)
	t.Setenv("EXTERNAL_CONFIG_FILE", externalPath)
	t.Setenv("TLS_MODE", "false")
	t.Setenv("SENTINEL_LOGLEVEL", "verbose")
	t.Setenv("EXPAND_EXTERNAL_CONFIG", "true")

	require.NoError(t, GenerateConfig())

	raw, err := os.ReadFile(confPath)
	require.NoError(t, err)
	conf := string(raw)

	// The include must point at an expanded copy, not the raw placeholder file.
	expandedPath := filepath.Join(dir, "redis-sentinel-additional.expanded.conf")
	assert.Contains(t, conf, "include "+expandedPath)
	assert.NotContains(t, conf, "include "+externalPath)

	includedRaw, err := os.ReadFile(expandedPath)
	require.NoError(t, err)
	included := string(includedRaw)

	assert.Contains(t, included, "loglevel verbose")
	assert.NotContains(t, included, "${SENTINEL_LOGLEVEL}")
	// Non-placeholder directives are preserved unchanged.
	assert.Contains(t, included, "sentinel deny-scripts-reconfig no")
}

func Test_GenerateConfig_ExternalConfig_GateOff_IncludesVerbatim(t *testing.T) {
	dir := t.TempDir()
	confPath := filepath.Join(dir, "sentinel.conf")
	externalPath := filepath.Join(dir, "redis-sentinel-additional.conf")

	require.NoError(t, os.WriteFile(externalPath, []byte("loglevel ${SENTINEL_LOGLEVEL}\n"), 0o644))

	t.Setenv("SENTINEL_CONFIG_FILE", confPath)
	t.Setenv("EXTERNAL_CONFIG_FILE", externalPath)
	t.Setenv("TLS_MODE", "false")
	t.Setenv("SENTINEL_LOGLEVEL", "verbose")
	// Gate off (default) — must include the raw file, no expanded copy written.
	os.Unsetenv("EXPAND_EXTERNAL_CONFIG")

	require.NoError(t, GenerateConfig())

	raw, err := os.ReadFile(confPath)
	require.NoError(t, err)
	conf := string(raw)

	assert.Contains(t, conf, "include "+externalPath)
	_, statErr := os.Stat(filepath.Join(dir, "redis-sentinel-additional.expanded.conf"))
	assert.True(t, os.IsNotExist(statErr), "no expanded copy should be written when gate is off")
}

func Test_GenerateConfig_ExternalConfig_MissingVarBecomesEmpty(t *testing.T) {
	dir := t.TempDir()
	confPath := filepath.Join(dir, "sentinel.conf")
	externalPath := filepath.Join(dir, "redis-sentinel-additional.conf")

	require.NoError(t, os.WriteFile(externalPath, []byte("loglevel ${NOT_SET_VAR}\n"), 0o644))

	t.Setenv("SENTINEL_CONFIG_FILE", confPath)
	t.Setenv("EXTERNAL_CONFIG_FILE", externalPath)
	t.Setenv("TLS_MODE", "false")
	t.Setenv("EXPAND_EXTERNAL_CONFIG", "true")
	os.Unsetenv("NOT_SET_VAR")

	require.NoError(t, GenerateConfig())

	includedRaw, err := os.ReadFile(filepath.Join(dir, "redis-sentinel-additional.expanded.conf"))
	require.NoError(t, err)
	assert.NotContains(t, string(includedRaw), "${NOT_SET_VAR}")
	assert.Contains(t, string(includedRaw), "loglevel \n")
}
