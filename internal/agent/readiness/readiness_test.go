package readiness

import (
	"context"
	"errors"
	"io"
	"net"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func report(values ...string) interface{} {
	result := make([]interface{}, len(values))
	for i, value := range values {
		result[i] = value
	}
	return result
}

func TestReadinessPreservesReachableParticipantAndEpochRules(t *testing.T) {
	for _, mode := range []string{"all-ready", "unreachable", "unprepared", "contradictory", "epoch-split", "duplicate-peer", "committed-promotion", "uncommitted-election", "old-master-down", "malformed-peer"} {
		t.Run(mode, func(t *testing.T) {
			cfg := Config{MemberIP: "192.0.2.2", Group: "myMaster", Peers: []string{"a", "b", "c"}, RedisPassword: "synthetic-local", SentinelPassword: "synthetic-sentinel"}
			if mode == "duplicate-peer" {
				cfg.Peers[2] = "a"
			}
			ask := func(_ context.Context, address, password string, args ...string) (interface{}, error) {
				if address == "127.0.0.1:6379" {
					require.Equal(t, cfg.RedisPassword, password)
					if args[0] == "PING" {
						return "PONG", nil
					}
					return "role:slave\r\nmaster_link_status:up\r\nmaster_host:192.0.2.1\r\n", nil
				}
				require.Equal(t, cfg.SentinelPassword, password)
				require.Equal(t, "SENTINEL", args[0])
				switch args[1] {
				case "MASTER":
					if mode == "malformed-peer" && address == "c:26379" {
						return "not a master report", nil
					}
					if mode == "unreachable" && address == "c:26379" {
						return nil, errors.New("unreachable")
					}
					ip, epoch := "192.0.2.1", "1"
					if address == "c:26379" && (mode == "committed-promotion" || mode == "old-master-down" || mode == "uncommitted-election") {
						phase, flags := "reconf_slaves", "master,failover_in_progress"
						if mode == "uncommitted-election" {
							phase = "wait_promotion"
						}
						if mode == "old-master-down" {
							flags = "s_down,master,disconnected,failover_in_progress"
						}
						return report("ip", "192.0.2.3", "port", "6379", "flags", flags, "config-epoch", "1", "failover-state", phase), nil
					}
					if mode == "contradictory" && address == "c:26379" {
						ip = "192.0.2.3"
					}
					if mode == "epoch-split" {
						epoch = map[string]string{"a:26379": "1", "b:26379": "2", "c:26379": "3"}[address]
					}
					return report("ip", ip, "port", "6379", "flags", "master", "config-epoch", epoch), nil
				case "GET-MASTER-ADDR-BY-NAME":
					return report("192.0.2.1", "6379"), nil
				case "REPLICAS":
					if mode == "unprepared" && address == "c:26379" {
						return []interface{}{}, nil
					}
					return []interface{}{report("ip", "192.0.2.2", "flags", "slave", "role-reported", "slave", "master-link-status", "ok", "master-host", "192.0.2.1", "slave-priority", "100", "info-refresh", "10")}, nil
				default:
					t.Errorf("probe attempted a non-readiness command")
					return nil, errors.New("unexpected command")
				}
			}
			require.Equal(t, mode == "all-ready" || mode == "unreachable" || mode == "committed-promotion" || mode == "old-master-down", check(context.Background(), cfg, ask))
		})
	}
}

func TestReplicaEligibilityRemainsBoundToOneRecord(t *testing.T) {
	base := []string{"ip", "192.0.2.2", "flags", "slave", "role-reported", "slave", "master-link-status", "ok", "master-host", "192.0.2.1", "slave-priority", "100", "info-refresh", "10"}
	for _, tc := range []struct{ key, value string }{
		{"flags", "s_down,slave"},
		{"flags", "o_down,slave"},
		{"flags", "slave,disconnected"},
		{"role-reported", "master"},
		{"master-link-status", "err"},
		{"master-host", "192.0.2.3"},
		{"slave-priority", "0"},
		{"info-refresh", "0"},
		{"slave-priority", "not-a-number"},
	} {
		values := append([]string{}, base...)
		for i := 0; i < len(values); i += 2 {
			if values[i] == tc.key {
				values[i+1] = tc.value
			}
		}
		require.False(t, hasEligibleReplica([]interface{}{report(values...)}, "192.0.2.2", "192.0.2.1"))
	}
	require.True(t, hasEligibleReplica([]interface{}{report(base...)}, "192.0.2.2", "192.0.2.1"))
	bad := append([]string{}, base...)
	bad[3] = "slave,disconnected"
	other := append([]string{}, base...)
	other[1] = "192.0.2.4"
	require.False(t, hasEligibleReplica([]interface{}{report(bad...), report(other...)}, "192.0.2.2", "192.0.2.1"))
	require.False(t, hasEligibleReplica([]interface{}{report("ip", "192.0.2.2", "ip", "192.0.2.3")}, "192.0.2.2", "192.0.2.1"))
}

func TestEstablishedUnresponsiveConnectionRespectsDeadline(t *testing.T) {
	listener, err := (&net.ListenConfig{}).Listen(context.Background(), "tcp", "127.0.0.1:0")
	require.NoError(t, err)
	defer listener.Close()
	done := make(chan struct{})
	go func() {
		defer close(done)
		conn, err := listener.Accept()
		if err != nil {
			return
		}
		defer conn.Close()
		_, _ = io.Copy(io.Discard, conn) // Accept and consume commands; never reply.
	}()
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()
	start := time.Now()
	_, err = query(ctx, listener.Addr().String(), "", "PING")
	require.Error(t, err)
	require.Less(t, time.Since(start), 2*time.Second)
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("probe connection was not closed")
	}
}

func TestReadinessRejectsMissingIdentityAndMalformedReports(t *testing.T) {
	require.False(t, Check(context.Background(), Config{}))
	_, ok := fields(report("ip"))
	require.False(t, ok)
	_, ok = fields(report("ip", "x", "ip", "y"))
	require.False(t, ok)
	_, ok = fields(strings.Repeat("x", 10))
	require.False(t, ok)
}
