package server

import (
	"context"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/OT-CONTAINER-KIT/redis-operator/internal/agent/server/probe"
)

// Start launches a role detector that periodically checks the Redis role.
// Configuration is provided via function parameters.
// The call blocks until the context is canceled (SIGINT/SIGTERM) or the
// detector exits. Any error returned by detector.Run is propagated to caller.
func Start(addr, password string, interval time.Duration) error {
	detector := probe.NewRoleDetector(addr, password, interval)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go handleSignals(cancel)

	return detector.Run(ctx)
}

func handleSignals(cancel context.CancelFunc) {
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	<-sigCh
	cancel()
}
