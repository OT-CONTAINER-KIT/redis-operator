package redissentinel

import (
	"context"
	"testing"

	commonapi "github.com/OT-CONTAINER-KIT/redis-operator/api/common/v1beta2"
	rrvb2 "github.com/OT-CONTAINER-KIT/redis-operator/api/redisreplication/v1beta2"
	rsvb2 "github.com/OT-CONTAINER-KIT/redis-operator/api/redissentinel/v1beta2"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type recordingHealer struct{ monitor, set, reset []string }

func (h *recordingHealer) SentinelMonitor(_ context.Context, _ *rsvb2.RedisSentinel, master string) error {
	h.monitor = append(h.monitor, master)
	return nil
}

func (h *recordingHealer) SentinelSet(_ context.Context, _ *rsvb2.RedisSentinel, master string) error {
	h.set = append(h.set, master)
	return nil
}

func (h *recordingHealer) SentinelReset(context.Context, *rsvb2.RedisSentinel) error {
	h.reset = append(h.reset, "reset")
	return nil
}

func (h *recordingHealer) UpdateRedisRoleLabel(context.Context, string, map[string]string, *commonapi.ExistingPasswordSecret, *commonapi.TLSConfig, string) error {
	return nil
}

func sentinelAndReplication() (*rsvb2.RedisSentinel, *rrvb2.RedisReplication) {
	rs := &rsvb2.RedisSentinel{ObjectMeta: metav1.ObjectMeta{Namespace: "ns", Name: "redis-sentinel", Generation: 1}}
	rr := &rrvb2.RedisReplication{ObjectMeta: metav1.ObjectMeta{Namespace: "ns", Name: "redis", ResourceVersion: "100"}}
	return rs, rr
}

// The wedge: the replication controller demotes the replica Sentinel promoted, without changing
// the RedisReplication object, so no watch event re-runs this. Only a periodic pass repoints it.
func TestResyncSentinelRequeuesSoAStaleMasterIsRepointed(t *testing.T) {
	h := &recordingHealer{}
	r := &RedisSentinelReconciler{Healer: h}
	rs, rr := sentinelAndReplication()

	result, err := r.resyncSentinel(context.Background(), rs, rr, "10.0.0.2")
	if err != nil {
		t.Fatal(err)
	}
	if result.RequeueAfter != sentinelResyncInterval {
		t.Fatalf("RequeueAfter = %v, want %v", result.RequeueAfter, sentinelResyncInterval)
	}
	if len(h.monitor) != 1 || h.monitor[0] != "10.0.0.2" {
		t.Fatalf("monitor = %v, want [10.0.0.2]", h.monitor)
	}
}

// SENTINEL RESET drops the known replicas and sentinels; repeating it every pass could disrupt a
// failover. It runs only when something changed, as it did when this was event-driven only.
func TestResyncSentinelResetsOnlyWhenSomethingChanged(t *testing.T) {
	h := &recordingHealer{}
	r := &RedisSentinelReconciler{Healer: h}
	rs, rr := sentinelAndReplication()
	ctx := context.Background()

	mustResync := func() {
		t.Helper()
		if _, err := r.resyncSentinel(ctx, rs, rr, "10.0.0.2"); err != nil {
			t.Fatal(err)
		}
	}

	mustResync()
	mustResync()
	if len(h.reset) != 1 {
		t.Fatalf("resets after an unchanged second pass = %d, want 1", len(h.reset))
	}
	if len(h.monitor) != 2 {
		t.Fatalf("monitor calls = %d, want 2: every pass must verify the monitored master", len(h.monitor))
	}

	rr.ResourceVersion = "101"
	mustResync()
	if len(h.reset) != 2 {
		t.Fatalf("resets after the RedisReplication changed = %d, want 2", len(h.reset))
	}

	rs.Generation = 2
	mustResync()
	if len(h.reset) != 3 {
		t.Fatalf("resets after the RedisSentinel spec changed = %d, want 3", len(h.reset))
	}
}

// Mid-failover no pod may yet be a master with replicas attached; never point Sentinel at "".
func TestResyncSentinelLeavesSentinelAloneWithoutAMaster(t *testing.T) {
	h := &recordingHealer{}
	r := &RedisSentinelReconciler{Healer: h}
	rs, rr := sentinelAndReplication()

	result, err := r.resyncSentinel(context.Background(), rs, rr, "")
	if err != nil {
		t.Fatal(err)
	}
	if len(h.monitor)+len(h.set)+len(h.reset) != 0 {
		t.Fatalf("touched sentinel with no master: monitor=%v set=%v reset=%v", h.monitor, h.set, h.reset)
	}
	if result.RequeueAfter == 0 {
		t.Fatal("must requeue to retry once a master is known")
	}
}
