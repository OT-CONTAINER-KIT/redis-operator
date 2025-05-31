package agent

import (
	"github.com/OT-CONTAINER-KIT/redis-operator/internal/cmd/agent/bootstrap"
	"github.com/spf13/cobra"
)

// CMD creates a cobra command for the agent
func CMD() *cobra.Command {
	agentCmd := &cobra.Command{
		Use:   "agent",
		Short: "Agent is a tool which run as a init/sidecar container along with redis/sentinel",
	}
	agentCmd.AddCommand(bootstrap.CMD())
	return agentCmd
}
