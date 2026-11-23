package redis

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/require"
)

func postPromotionReport(epoch, phase, flags string) []interface{} {
	return []interface{}{"ip", "192.0.2.1", "port", "6379", "flags", flags, "config-epoch", epoch, "failover-state", phase}
}

func TestCommittedClientAddressRequiresBoundSnapshotAndHealthyMajority(t *testing.T) {
	raw := postPromotionReport("10", "reconf_slaves", "master,failover_in_progress,force_failover")
	queries := 0
	state, err := ReadSentinelMasterState(context.Background(), "myMaster", func(_ context.Context, args ...string) (interface{}, error) {
		queries++
		require.Equal(t, "SENTINEL", args[0])
		switch args[1] {
		case "MASTER":
			return raw, nil
		case "GET-MASTER-ADDR-BY-NAME":
			return []interface{}{"192.0.2.2", "6379"}, nil
		default:
			t.Fatal("authority read attempted a mutation")
			return nil, nil
		}
	})
	require.NoError(t, err)
	require.Equal(t, 3, queries)
	require.Equal(t, "192.0.2.1", state.IP, "raw diagnostic evidence must not be overwritten")
	require.Equal(t, "master,failover_in_progress,force_failover", state.Flags)
	address, committed := state.PostPromotionAddress()
	require.True(t, committed)
	require.Equal(t, "192.0.2.2", address)
	healthy := SentinelMasterState{IP: "192.0.2.2", Port: "6379", Flags: "master", Epoch: "10"}
	decision, ok := AgreedSentinelMaster([]SentinelMasterState{healthy, healthy, *state}, 3)
	require.True(t, ok)
	require.Equal(t, healthy.IP, decision.IP)
	_, ok = AgreedSentinelMaster([]SentinelMasterState{healthy, *state, *state}, 3)
	require.False(t, ok, "a reconfiguration report is not another healthy vote")
	older := healthy
	older.Epoch = "9"
	_, ok = AgreedSentinelMaster([]SentinelMasterState{older, older, *state}, 3)
	require.False(t, ok, "a committed higher epoch must still block the older majority")
	conflict := healthy
	conflict.IP = "192.0.2.3"
	_, ok = AgreedSentinelMaster([]SentinelMasterState{conflict, conflict, *state}, 3)
	require.False(t, ok, "client-address disagreement must not be suppressed")
}

func TestClientAddressIsNeverTrustedBeforePromotionAcknowledgement(t *testing.T) {
	for _, phase := range []string{"", "wait_start", "select_slave", "send_slaveof_noone", "wait_promotion", "unknown"} {
		t.Run(phase, func(t *testing.T) {
			calls := 0
			state, err := ReadSentinelMasterState(context.Background(), "myMaster", func(_ context.Context, args ...string) (interface{}, error) {
				calls++
				require.Equal(t, "MASTER", args[1])
				return postPromotionReport("10", phase, "master,failover_in_progress"), nil
			})
			require.NoError(t, err)
			require.Equal(t, 1, calls)
			_, bound := state.PostPromotionAddress()
			require.False(t, bound)
			healthy := SentinelMasterState{IP: "192.0.2.2", Port: "6379", Flags: "master", Epoch: "10"}
			_, ok := AgreedSentinelMaster([]SentinelMasterState{healthy, healthy, *state}, 3)
			require.False(t, ok)
		})
	}
}

func TestFailedOrChangingPromotionReadRetainsTheActiveElection(t *testing.T) {
	for _, mode := range []string{"address error", "second read error", "epoch changed", "phase changed", "invalid address", "invalid port", "same address", "short address"} {
		t.Run(mode, func(t *testing.T) {
			masters := 0
			state, err := ReadSentinelMasterState(context.Background(), "myMaster", func(_ context.Context, args ...string) (interface{}, error) {
				if args[1] == "MASTER" {
					masters++
					if masters == 2 {
						switch mode {
						case "second read error":
							return nil, errors.New("unavailable")
						case "epoch changed":
							return postPromotionReport("11", "reconf_slaves", "master,failover_in_progress"), nil
						case "phase changed":
							return postPromotionReport("10", "update_config", "master,failover_in_progress"), nil
						}
					}
					return postPromotionReport("10", "reconf_slaves", "master,failover_in_progress"), nil
				}
				switch mode {
				case "address error":
					return nil, errors.New("unavailable")
				case "invalid address":
					return []interface{}{"not-an-IP", "6379"}, nil
				case "invalid port":
					return []interface{}{"192.0.2.2", "6380"}, nil
				case "same address":
					return []interface{}{"192.0.2.1", "6379"}, nil
				case "short address":
					return []interface{}{"192.0.2.2"}, nil
				default:
					return []interface{}{"192.0.2.2", "6379"}, nil
				}
			})
			require.NoError(t, err, "known election evidence must not become an ignored/unavailable peer")
			_, bound := state.PostPromotionAddress()
			require.False(t, bound)
			healthy := SentinelMasterState{IP: "192.0.2.2", Port: "6379", Flags: "master", Epoch: "10"}
			_, agreed := AgreedSentinelMaster([]SentinelMasterState{healthy, healthy, *state}, 3)
			require.False(t, agreed)
		})
	}
}

func TestOldMasterHealthFlagsDoNotDescribeTheAcknowledgedNewPrimary(t *testing.T) {
	state := SentinelMasterState{IP: "192.0.2.1", Port: "6379", Flags: "s_down,o_down,master,disconnected,failover_in_progress", Epoch: "10", FailoverState: "reconf_slaves", clientIP: "192.0.2.2", clientPort: "6379"}
	healthy := SentinelMasterState{IP: "192.0.2.2", Port: "6379", Flags: "master", Epoch: "10"}
	_, ok := AgreedSentinelMaster([]SentinelMasterState{healthy, healthy, state}, 3)
	require.True(t, ok)
	state.Flags += ",unknown"
	_, ok = AgreedSentinelMaster([]SentinelMasterState{healthy, healthy, state}, 3)
	require.False(t, ok)
}
