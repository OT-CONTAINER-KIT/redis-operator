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

func (c *Config) Append(config ...string) *Config {
	if len(config) == 0 {
		return c
	}
	c.content = fmt.Sprintf("%s\n%s %s", c.content, config[0], strings.Join(config[1:], " "))
	return c
}

func (c *Config) Commit() error {
	dir := filepath.Dir(c.path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("failed to create directory %s: %v", dir, err)
	}
	return os.WriteFile(c.path, []byte(c.content), 0o644)
}
