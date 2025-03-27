package bootstrap

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
)

// agent bootstrap --sentinel: use to init config for sentinel or redis, if --sentinel is not set, it will init config for redis
var (
	BootstrapCmd = &cobra.Command{
		Use:   "bootstrap",
		Short: "Bootstrap do some init work before run redis/sentinel",
		Run:   bootstrapRun,
	}
)

//nolint:gochecknoinits
func init() {
	BootstrapCmd.Flags().Bool("sentinel", false, "Generate sentinel config instead of redis config")
}

func bootstrapRun(cmd *cobra.Command, args []string) {
	generateConfig(cmd, args)
}

func generateConfig(cmd *cobra.Command, args []string) {
	sentinel, err := cmd.Flags().GetBool("sentinel")
	if err != nil {
		log.Fatalf("Failed to get sentinel flag: %v", err)
	}

	if sentinel {
		if err := generateSentinelConfig(); err != nil {
			log.Fatalf("Failed to generate sentinel config: %v", err)
		}
	} else {
		if err := generateRedisConfig(); err != nil {
			log.Fatalf("Failed to generate redis config: %v", err)
		}
	}
}

type config struct {
	content string
	path    string
}

func newConfig(path string, defaultConfig ...string) *config {
	return &config{
		path:    path,
		content: strings.Join(defaultConfig, "\n"),
	}
}

// append config[0] as key, config[1]... as value
func (c *config) append(config ...string) *config {
	if len(config) == 0 {
		return c
	}
	c.content = fmt.Sprintf("%s\n%s %s", c.content, config[0], strings.Join(config[1:], " "))
	return c
}

func (c *config) commit() error {
	dir := filepath.Dir(c.path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("failed to create directory %s: %v", dir, err)
	}
	return os.WriteFile(c.path, []byte(c.content), 0o644)
}
