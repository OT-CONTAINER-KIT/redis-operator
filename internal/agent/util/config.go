package util

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
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

// sanitizeConfigValue strips characters that would let a value escape its
// directive line (CR/LF) or comment-out trailing tokens (#). Returning the
// cleaned value rather than erroring keeps the bootstrap path resilient when
// upstream CRD validation is bypassed (e.g. existing objects, partial RBAC).
func sanitizeConfigValue(s string) string {
	r := strings.NewReplacer(
		"\r", "",
		"\n", "",
		"#", "",
	)
	return r.Replace(s)
}

func (c *Config) Append(config ...string) *Config {
	if len(config) == 0 {
		return c
	}
	directive := sanitizeConfigValue(config[0])
	args := make([]string, 0, len(config)-1)
	for _, a := range config[1:] {
		args = append(args, sanitizeConfigValue(a))
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
