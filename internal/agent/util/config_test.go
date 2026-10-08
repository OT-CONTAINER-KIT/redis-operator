package util

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestAppend_RejectsNewlineInjection guards against CRD-driven Sentinel
// config injection (issue #1763): a malicious masterGroupName that contains
// newlines must not be allowed to introduce extra config directives into
// sentinel.conf.
func TestAppend_RejectsNewlineInjection(t *testing.T) {
	path := filepath.Join(t.TempDir(), "sentinel.conf")
	cfg := NewConfig(path)

	// Mimics the agent's call site at internal/agent/bootstrap/sentinel/config.go
	// with the PoC payload from issue #1763.
	maliciousGroupName := "mymaster 127.0.0.1 6379 2\nsentinel deny-scripts-reconfig no\nsentinel set-auth-pass mymaster injected-password"
	cfg.Append("sentinel monitor", maliciousGroupName, "127.0.0.1", "6379", "2")

	if err := cfg.Commit(); err != nil {
		t.Fatalf("Commit failed: %v", err)
	}
	contents, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile failed: %v", err)
	}
	got := string(contents)

	// The injection vector is newlines inside an appended value: each new
	// line in sentinel.conf is parsed as an independent directive. Reject
	// any directive line attacker-controlled tokens could spawn.
	for _, line := range strings.Split(got, "\n") {
		if strings.HasPrefix(line, "sentinel deny-scripts-reconfig") ||
			strings.HasPrefix(line, "sentinel set-auth-pass") {
			t.Errorf("config contains injected directive line %q; rendered config:\n%s", line, got)
		}
	}
	if strings.Count(got, "\nsentinel monitor") != 1 {
		t.Errorf("expected exactly one 'sentinel monitor' directive; rendered config:\n%s", got)
	}
}

// TestAppend_PreservesValidValues ensures sanitization does not mangle
// well-formed inputs.
func TestAppend_PreservesValidValues(t *testing.T) {
	path := filepath.Join(t.TempDir(), "sentinel.conf")
	cfg := NewConfig(path)

	cfg.Append("sentinel monitor", "mymaster", "127.0.0.1", "6379", "2")
	cfg.Append("sentinel down-after-milliseconds", "mymaster", "30000")

	if err := cfg.Commit(); err != nil {
		t.Fatalf("Commit failed: %v", err)
	}
	contents, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile failed: %v", err)
	}
	got := string(contents)

	for _, want := range []string{
		"sentinel monitor mymaster 127.0.0.1 6379 2",
		"sentinel down-after-milliseconds mymaster 30000",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("expected %q in rendered config, got:\n%s", want, got)
		}
	}
}

func TestExpandExternalConfig_MissingVarBecomesEmpty(t *testing.T) {
	dir := t.TempDir()
	externalPath := filepath.Join(dir, "redis-additional.conf")
	confPath := filepath.Join(dir, "redis.conf")
	if err := os.WriteFile(externalPath, []byte("maxmemory-policy ${NOT_SET_VAR}\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	os.Unsetenv("NOT_SET_VAR")
	out, err := ExpandExternalConfig(externalPath, confPath)
	if err != nil {
		t.Fatal(err)
	}
	if want := filepath.Join(dir, "redis-additional.expanded.conf"); out != want {
		t.Fatalf("expanded path = %q, want %q", out, want)
	}

	raw, err := os.ReadFile(out)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(raw), "${NOT_SET_VAR}") {
		t.Fatalf("placeholder not expanded: %q", raw)
	}
}

func TestExpandExternalConfig_WritesToConfDirNotSourceDir(t *testing.T) {
	// Source lives in a read-only dir; conf dir is separate and writable —
	// mirrors the pod layout (ConfigMap mount vs config emptyDir).
	roDir := t.TempDir()
	externalPath := filepath.Join(roDir, "redis-sentinel-additional.conf")
	if err := os.WriteFile(externalPath, []byte("loglevel ${SENTINEL_LOGLEVEL}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(roDir, 0o555); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(roDir, 0o755) })

	confDir := t.TempDir()
	confPath := filepath.Join(confDir, "sentinel.conf")

	t.Setenv("SENTINEL_LOGLEVEL", "verbose")
	out, err := ExpandExternalConfig(externalPath, confPath)
	if err != nil {
		t.Fatal(err)
	}
	if want := filepath.Join(confDir, "redis-sentinel-additional.expanded.conf"); out != want {
		t.Fatalf("expanded path = %q, want %q", out, want)
	}

	raw, err := os.ReadFile(out)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(raw), "loglevel verbose") {
		t.Fatalf("expected expanded directive, got %q", raw)
	}
	if strings.Contains(string(raw), "${SENTINEL_LOGLEVEL}") {
		t.Fatalf("placeholder not expanded: %q", raw)
	}
}

func TestAppendExternalConfig(t *testing.T) {
	t.Run("gate on includes expanded copy", func(t *testing.T) {
		dir := t.TempDir()
		externalPath := filepath.Join(dir, "redis-additional.conf")
		if err := os.WriteFile(externalPath, []byte("maxmemory-policy ${MAXMEMORY_POLICY}\n"), 0o644); err != nil {
			t.Fatal(err)
		}
		t.Setenv("MAXMEMORY_POLICY", "allkeys-lru")

		cfg := NewConfig(filepath.Join(dir, "redis.conf"))
		cfg.AppendExternalConfig(externalPath, true)

		want := "include " + filepath.Join(dir, "redis-additional.expanded.conf")
		if !strings.Contains(cfg.content, want) {
			t.Fatalf("expected %q in %q", want, cfg.content)
		}
	})

	t.Run("gate off includes raw file and writes nothing", func(t *testing.T) {
		dir := t.TempDir()
		externalPath := filepath.Join(dir, "redis-additional.conf")
		if err := os.WriteFile(externalPath, []byte("maxmemory-policy ${MAXMEMORY_POLICY}\n"), 0o644); err != nil {
			t.Fatal(err)
		}

		cfg := NewConfig(filepath.Join(dir, "redis.conf"))
		cfg.AppendExternalConfig(externalPath, false)

		if !strings.Contains(cfg.content, "include "+externalPath) {
			t.Fatalf("expected raw include in %q", cfg.content)
		}
		if _, err := os.Stat(filepath.Join(dir, "redis-additional.expanded.conf")); !os.IsNotExist(err) {
			t.Fatal("no expanded copy should be written when gate is off")
		}
	})

	t.Run("missing file appends nothing", func(t *testing.T) {
		dir := t.TempDir()
		cfg := NewConfig(filepath.Join(dir, "redis.conf"))
		cfg.AppendExternalConfig(filepath.Join(dir, "does-not-exist.conf"), true)
		if strings.Contains(cfg.content, "include") {
			t.Fatalf("unexpected include in %q", cfg.content)
		}
	})
}
