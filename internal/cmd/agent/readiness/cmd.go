package readiness

import (
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	probe "github.com/OT-CONTAINER-KIT/redis-operator/internal/agent/readiness"
	"github.com/spf13/cobra"
)

func CMD() *cobra.Command {
	return &cobra.Command{
		Use: "readiness", Short: "Check standalone Redis/Sentinel readiness without subprocesses",
		SilenceUsage: true, SilenceErrors: true,
		RunE: func(cmd *cobra.Command, _ []string) error {
			ctx, cancel := context.WithTimeout(cmd.Context(), 4*time.Second)
			defer cancel()
			group := os.Getenv("SENTINEL_BOOTSTRAP_GROUP")
			if group == "" {
				group = "myMaster"
			}
			if !probe.Check(ctx, probe.Config{
				MemberIP: os.Getenv("SENTINEL_MEMBER_IP"), Group: group,
				Peers:         strings.Split(os.Getenv("SENTINEL_BOOTSTRAP_ADDRS"), ","),
				RedisPassword: os.Getenv("REDIS_PASSWORD"), SentinelPassword: os.Getenv("SENTINEL_BOOTSTRAP_PASSWORD"),
			}) {
				return fmt.Errorf("redis and Sentinel readiness is not confirmed")
			}
			return nil
		},
	}
}
