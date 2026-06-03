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

// configValueSanitizer strips the line-breaking characters (CR/LF) that would
// let a value escape its directive line. Redis only splits config into
// directives on '\n' and only treats '#' as a comment at the start of a line,
// and these values are always written after a fixed directive token, so
// stripping CR/LF is sufficient to prevent directive injection.
var configValueSanitizer = strings.NewReplacer(
	"\r", "",
	"\n", "",
)

// sanitizeConfigValue removes characters that would let a value escape its
// directive line. Cleaning rather than erroring keeps the bootstrap path
// resilient when upstream CRD validation is bypassed (e.g. existing objects,
// partial RBAC).
func sanitizeConfigValue(s string) string {
	return configValueSanitizer.Replace(s)
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
