package agent

import (
	"github.com/OT-CONTAINER-KIT/redis-operator/internal/cmd/agent/bootstrap"
	"github.com/spf13/cobra"
)

// CreateCommand creates a cobra command for the agent
func CreateCommand() *cobra.Command {
	agentCmd := &cobra.Command{
		Use:   "agent",
		Short: "Agent is a tool which run as a init/sidecar container along with redis/sentinel",
	}

	// Add bootstrap subcommand
	agentCmd.AddCommand(bootstrap.BootstrapCmd)

	return agentCmd
}
