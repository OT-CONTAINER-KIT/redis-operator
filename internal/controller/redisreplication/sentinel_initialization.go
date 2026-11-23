package redisreplication

import (
	"context"
	"fmt"

	rrvb2 "github.com/OT-CONTAINER-KIT/redis-operator/api/redisreplication/v1beta2"
	"github.com/OT-CONTAINER-KIT/redis-operator/internal/controller/common"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// Absence after handoff is not evidence of a virgin voter. REMOVE also erases
// per-master leader/vote history, so recreating a monitor can enable a second
// vote in the same epoch under the retained Sentinel identity.
// A partial/unacknowledged first initialization is deliberately fail closed.
func initialMonitorInitializationAllowed(rr *rrvb2.RedisReplication, reports int) bool {
	return reports == 0 && rr.Annotations[common.SentinelOwnershipAnnotation] != "sentinel" && rr.Annotations[common.SentinelStartupAnnotation] != "enabled"
}

func (r *Reconciler) mayInitializeMissingMonitors(ctx context.Context, rr *rrvb2.RedisReplication, reports int) (bool, error) {
	if r.APIReader == nil {
		return false, nil
	}
	fresh := &rrvb2.RedisReplication{}
	if err := r.APIReader.Get(ctx, client.ObjectKeyFromObject(rr), fresh); err != nil {
		return false, err
	}
	if fresh.UID != rr.UID || fresh.Generation != rr.Generation {
		return false, fmt.Errorf("replication identity or specification changed before monitor initialization")
	}
	if !initialMonitorInitializationAllowed(fresh, reports) {
		return false, nil
	}
	rr.Annotations = fresh.Annotations
	rr.ResourceVersion = fresh.ResourceVersion
	return true, nil
}
