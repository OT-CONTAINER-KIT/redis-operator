package probe

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/OT-CONTAINER-KIT/redis-operator/internal/agent/events"
	"github.com/redis/go-redis/v9"
)

// RoleDetector periodically checks the Redis role using INFO replication.
// It prints a log each time the role changes.
// All configuration is supplied through constructor parameters so that the caller
// can decide how to source configuration (for example, via environment variables).
type RoleDetector struct {
	client      *redis.Client
	interval    time.Duration
	currentRole string
	eventSender *events.K8sEventSender
}

// NewRoleDetector creates a detector that connects to the given Redis instance.
func NewRoleDetector(addr, password string, interval time.Duration) *RoleDetector {
	opts := &redis.Options{
		Addr:     addr,
		Password: password,
	}

	// Try to create event sender, but don't fail if it's not available
	eventSender, err := events.NewK8sEventSender()
	if err != nil {
		fmt.Printf("warning: failed to create event sender: %v\n", err)
	}

	return &RoleDetector{
		client:      redis.NewClient(opts),
		interval:    interval,
		eventSender: eventSender,
	}
}

// Run starts the detection loop and blocks until the context is cancelled.
func (d *RoleDetector) Run(ctx context.Context) error {
	// Perform an initial detection immediately.
	if err := d.detect(ctx); err != nil {
		fmt.Printf("role detection failed: %v\n", err)
	}

	ticker := time.NewTicker(d.interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return nil
		case <-ticker.C:
			if err := d.detect(ctx); err != nil {
				fmt.Printf("role detection failed: %v\n", err)
			}
		}
	}
}

// detect queries Redis and logs a message if the role has changed.
func (d *RoleDetector) detect(ctx context.Context) error {
	res, err := d.client.Info(ctx, "replication").Result()
	if err != nil {
		return err
	}
	role := parseRole(res)
	if role != d.currentRole {
		previousRole := d.currentRole
		if previousRole == "" {
			previousRole = "unknown"
		}
		fmt.Printf("detected role change: %s -> %s\n", previousRole, role)

		// Send Kubernetes event if event sender is available
		if d.eventSender != nil {
			if err := d.eventSender.SendRoleChangeEvent(ctx, previousRole, role); err != nil {
				fmt.Printf("warning: failed to send role change event: %v\n", err)
			}
		}

		d.currentRole = role
	}
	return nil
}

// parseRole extracts the role from the output of `INFO replication`.
func parseRole(info string) string {
	for _, line := range strings.Split(info, "\n") {
		if strings.HasPrefix(line, "role:") {
			return strings.TrimSpace(strings.TrimPrefix(line, "role:"))
		}
	}
	return "unknown"
}
