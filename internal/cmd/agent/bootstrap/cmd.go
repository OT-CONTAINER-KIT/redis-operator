package bootstrap

import (
	"github.com/OT-CONTAINER-KIT/redis-operator/internal/agent/bootstrap"
	"github.com/spf13/cobra"
)

func CMD() *cobra.Command {
	bootstrapCmd := &cobra.Command{
		Use:   "bootstrap",
		Short: "Bootstrap do some init work before run redis/sentinel",
		RunE: func(cmd *cobra.Command, args []string) error {
			sentinel, err := cmd.Flags().GetBool("sentinel")
			if err != nil {
				return err
			}
			return bootstrap.NewTask(sentinel).Run()
		},
	}
	bootstrapCmd.Flags().Bool("sentinel", false, "Generate sentinel config instead of redis config")
	// Capability flag: older init images must reject the new startup contract
	// instead of silently generating a guessed monitor or unprotected primary.
	bootstrapCmd.Flags().Bool("standalone-authority", false, "Support create-only standalone Sentinel authority initialization")
	bootstrapCmd.Flags().Bool("persistent-sentinel-state", false, "Preserve durable Sentinel configuration and election history")
	return bootstrapCmd
}
