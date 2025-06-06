package server

import (
	"time"

	"github.com/OT-CONTAINER-KIT/redis-operator/internal/agent/server"
	"github.com/spf13/cobra"
)

func CMD() *cobra.Command {
	var (
		redisAddr      string
		redisPassword  string
		detectInterval time.Duration
	)

	cmd := &cobra.Command{
		Use:   "server",
		Short: "Start the Redis role detector agent",
		RunE: func(cmd *cobra.Command, args []string) error {
			return server.Start(redisAddr, redisPassword, detectInterval)
		},
	}

	cmd.Flags().StringVar(&redisAddr, "redis-addr", "127.0.0.1:6379", "Redis address in host:port format")
	cmd.Flags().StringVar(&redisPassword, "redis-password", "", "Redis password for authentication (optional)")
	cmd.Flags().DurationVar(&detectInterval, "detect-interval", 10*time.Second, "Role detection interval")

	return cmd
}
