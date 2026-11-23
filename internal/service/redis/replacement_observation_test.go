package redis

import (
	"fmt"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestReplacementReplicationObservation(t *testing.T) {
	primary := "role:master\r\nconnected_slaves:1\r\nslave0:ip=192.0.2.2,port=6379,state=online,offset=123,lag=0\r\n"
	replica := "role:slave\r\nmaster_host:192.0.2.1\r\nmaster_port:6379\r\nmaster_link_status:up\r\nslave_priority:100\r\nconnected_slaves:0\r\n"
	state, err := parseReplicationObservation(primary)
	require.NoError(t, err)
	require.Equal(t, "master", state.Role)
	require.Equal(t, []AttachedReplica{{IP: "192.0.2.2", Port: "6379", State: "online", Lag: 0}}, state.Replicas)
	state, err = parseReplicationObservation(replica)
	require.NoError(t, err)
	require.Equal(t, "192.0.2.1", state.MasterHost)
	require.Equal(t, uint64(100), state.Priority)
	for name, info := range map[string]string{
		"missing role":          "connected_slaves:0\r\n",
		"duplicate role":        primary + "role:slave\r\n",
		"missing child":         strings.Replace(primary, "connected_slaves:1", "connected_slaves:2", 1),
		"undeclared child":      primary + "slave1:ip=192.0.2.3,port=6379,state=online,offset=123,lag=0\r\n",
		"invalid child address": strings.Replace(primary, "192.0.2.2", "not-an-IP", 1),
		"invalid child port":    strings.Replace(primary, "port=6379", "port=0", 1),
		"invalid child lag":     strings.Replace(primary, "lag=0", "lag=unknown", 1),
		"duplicate child field": strings.Replace(primary, "lag=0", "lag=0,lag=1", 1),
		"invalid source":        strings.Replace(replica, "master_host:192.0.2.1", "master_host:unknown", 1),
		"invalid priority":      strings.Replace(replica, "slave_priority:100", "slave_priority:-1", 1),
		"missing priority":      strings.Replace(replica, "slave_priority:100\r\n", "", 1),
		"invalid count":         strings.Replace(primary, "connected_slaves:1", "connected_slaves:999999999", 1),
	} {
		t.Run(name, func(t *testing.T) {
			_, err := parseReplicationObservation(info)
			require.Error(t, err)
		})
	}
}

func replacementSentinelFields(values map[string]string) []interface{} {
	result := []interface{}{}
	for key, value := range values {
		result = append(result, key, value)
	}
	return result
}

func TestReplacementSentinelEvidenceIsReadOnlyAndSnapshotBound(t *testing.T) {
	master := replacementSentinelFields(map[string]string{"ip": "192.0.2.1", "port": "6379", "flags": "master", "config-epoch": "7"})
	replica := replacementSentinelFields(map[string]string{
		"ip": "192.0.2.2", "port": "6379", "flags": "slave", "role-reported": "slave", "master-host": "192.0.2.1", "master-port": "6379", "master-link-status": "ok", "slave-priority": "100", "info-refresh": "10",
	})
	for _, mode := range []string{"stable", "changing master", "changing voter", "active election", "missing monitor", "malformed replicas", "duplicate fields", "duplicate addresses", "invalid voter", "failed replica read"} {
		t.Run(mode, func(t *testing.T) {
			masterCalls, idCalls := 0, 0
			var commands []string
			state, err := readSentinelReplacement(func(args ...interface{}) (interface{}, error) {
				command := fmt.Sprint(args[1])
				commands = append(commands, command)
				switch command {
				case "MASTER":
					masterCalls++
					if mode == "missing monitor" {
						return nil, fmt.Errorf("synthetic server reply that must not be returned")
					}
					if mode == "changing master" && masterCalls == 2 {
						return replacementSentinelFields(map[string]string{"ip": "192.0.2.3", "port": "6379", "flags": "master", "config-epoch": "8"}), nil
					}
					if mode == "active election" {
						return replacementSentinelFields(map[string]string{"ip": "192.0.2.1", "port": "6379", "flags": "master,failover_in_progress", "config-epoch": "7"}), nil
					}
					return master, nil
				case "MYID":
					idCalls++
					if mode == "invalid voter" {
						return "not-a-voter-identity", nil
					}
					if mode == "changing voter" && idCalls == 2 {
						return strings.Repeat("b", 40), nil
					}
					return strings.Repeat("a", 40), nil
				case "REPLICAS":
					if mode == "failed replica read" {
						return nil, fmt.Errorf("synthetic server reply that must not be returned")
					}
					if mode == "malformed replicas" {
						return []interface{}{[]interface{}{"ip"}}, nil
					}
					if mode == "duplicate fields" {
						return []interface{}{append(append([]interface{}{}, replica...), "ip", "192.0.2.2")}, nil
					}
					if mode == "duplicate addresses" {
						return []interface{}{replica, replica}, nil
					}
					return []interface{}{replica}, nil
				default:
					t.Fatalf("unexpected replacement observation command %v", args)
					return nil, nil
				}
			}, "myMaster")
			if mode == "stable" {
				require.NoError(t, err)
				require.Equal(t, "7", state.Master.Epoch)
				require.Equal(t, strings.Repeat("a", 40), state.VoterID)
				require.Equal(t, []string{"MASTER", "MYID", "REPLICAS", "MYID", "MASTER"}, commands)
				require.Len(t, state.Replicas, 1)
				require.Equal(t, "192.0.2.2", state.Replicas[0].IP)
			} else {
				require.Error(t, err)
				require.NotContains(t, err.Error(), "synthetic server reply")
			}
		})
	}
}
