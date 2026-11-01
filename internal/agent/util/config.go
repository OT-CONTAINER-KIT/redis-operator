package util

import (
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"strings"

	"github.com/OT-CONTAINER-KIT/redis-operator/internal/util"
)

type Config struct {
	content string
	path    string
}

func NewConfig(path string, defaultConfig ...string) *Config {
	return &Config{
		path:    path,
		content: strings.Join(defaultConfig, "\n"),
	}
}

// configValueSanitizer strips the line-breaking characters (CR/LF) that would
// let a value escape its directive line. Redis only splits config into
// directives on '\n' and only treats '#' as a comment at the start of a line,
// and these values are always written after a fixed directive token, so
// stripping CR/LF is sufficient to prevent directive injection.
var configValueSanitizer = strings.NewReplacer(
	"\r", "",
	"\n", "",
)

// SanitizeConfigValue removes characters that would let a value escape its
// directive line. Cleaning rather than erroring keeps the bootstrap path
// resilient when upstream CRD validation is bypassed (e.g. existing objects,
// partial RBAC).
func SanitizeConfigValue(s string) string {
	return configValueSanitizer.Replace(s)
}

func (c *Config) Append(config ...string) *Config {
	if len(config) == 0 {
		return c
	}
	directive := SanitizeConfigValue(config[0])
	args := make([]string, 0, len(config)-1)
	for _, a := range config[1:] {
		args = append(args, SanitizeConfigValue(a))
	}
	c.content = fmt.Sprintf("%s\n%s %s", c.content, directive, strings.Join(args, " "))
	return c
}

func (c *Config) Commit() error {
	dir := filepath.Dir(c.path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("failed to create directory %s: %v", dir, err)
	}
	return os.WriteFile(c.path, []byte(c.content), 0o644)
}

// CommitIfAbsent publishes a fully written, synced initial configuration without
// replacing a runtime file that may already contain election history.
func (c *Config) CommitIfAbsent(mode os.FileMode) error {
	dir := filepath.Dir(c.path)
	if err := os.MkdirAll(dir, 0o770); err != nil {
		return err
	}
	file, err := os.CreateTemp(dir, ".sentinel-initial-*.conf")
	if err != nil {
		return err
	}
	defer file.Close()
	defer os.Remove(file.Name())
	if err := file.Chmod(mode); err != nil {
		return err
	}
	if _, err := io.WriteString(file, c.content); err != nil {
		return err
	}
	if err := file.Sync(); err != nil {
		return err
	}
	// Link is an atomic no-replace operation on the same filesystem.
	if err := os.Link(file.Name(), c.path); err != nil {
		return err
	}
	directory, err := os.Open(dir)
	if err != nil {
		return err
	}
	defer directory.Close()
	return directory.Sync()
}

func (c *Config) AppendExternalConfig(externalConfigFile string, expand bool) *Config {
	if _, err := os.Stat(externalConfigFile); err != nil {
		return c
	}
	includeFile := externalConfigFile
	if expand {
		expandedFile, err := ExpandExternalConfig(externalConfigFile, c.path)
		if err != nil {
			log.Printf("Warning: Failed to expand external config %s, including verbatim: %v", externalConfigFile, err)
		} else {
			includeFile = expandedFile
		}
	}
	return c.Append("include", includeFile)
}

func ExpandExternalConfig(externalConfigFile, confPath string) (string, error) {
	raw, err := os.ReadFile(externalConfigFile)
	if err != nil {
		return "", err
	}
	expanded := os.Expand(string(raw), func(key string) string {
		return util.CoalesceEnv1(key, "")
	})

	base := strings.TrimSuffix(filepath.Base(externalConfigFile), ".conf")
	expandedFile := filepath.Join(filepath.Dir(confPath), base+".expanded.conf")
	if err := os.WriteFile(expandedFile, []byte(expanded), 0o644); err != nil {
		return "", fmt.Errorf("write expanded config to %s: %v", expandedFile, err)
	}
	return expandedFile, nil
}
