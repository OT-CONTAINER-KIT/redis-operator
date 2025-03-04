package main

import (
	"os"

	"github.com/OT-CONTAINER-KIT/redis-operator/pkg/agent/bootstrap"
	"github.com/spf13/cobra"
)

// agent bootstrap --sentinel
func main() {
	rootCmd := &cobra.Command{
		Use:   "agent",
		Short: "Agent is a tool which run as a init/sidecar container along  with redis/sentinel",
	}

	rootCmd.AddCommand(bootstrap.BootstrapCmd)

	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}
