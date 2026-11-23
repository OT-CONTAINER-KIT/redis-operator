package redis

import (
	"context"
	"encoding/hex"
	"fmt"
	"net"
	"strconv"
	"strings"
	"time"

	rediscli "github.com/redis/go-redis/v9"
)

// Replacement observations authorize only bounded workload recreation, never a
// Redis role command. Keep credentials and raw server replies out of errors.
type ReplicationObservation struct {
	Role, MasterHost, MasterPort, MasterLink string
	Priority                                 uint64
	Replicas                                 []AttachedReplica
}

type AttachedReplica struct {
	IP, Port, State string
	Lag             uint64
}

type SentinelReplicaObservation struct {
	IP, Port, Flags, Role, MasterHost, MasterPort, MasterLink string
	Priority, InfoAge                                         uint64
}

type SentinelReplacementObservation struct {
	Master   SentinelMasterState
	VoterID  string
	Replicas []SentinelReplicaObservation
}

func replacementClient(address, password string) *rediscli.Client {
	return rediscli.NewClient(&rediscli.Options{
		Addr: address, Password: password, Protocol: 2, DisableIdentity: true,
		DialTimeout: time.Second, ReadTimeout: time.Second, WriteTimeout: time.Second,
		ContextTimeoutEnabled: true, MaxRetries: -1,
	})
}

func ObserveReplication(ctx context.Context, ip, password string) (ReplicationObservation, error) {
	c := replacementClient(net.JoinHostPort(ip, "6379"), password)
	defer c.Close()
	info, err := c.Info(ctx, "replication").Result()
	if err != nil {
		return ReplicationObservation{}, fmt.Errorf("cannot observe Redis replication for replacement recovery")
	}
	return parseReplicationObservation(info)
}

func parseReplicationObservation(info string) (ReplicationObservation, error) {
	invalid := func() (ReplicationObservation, error) {
		return ReplicationObservation{}, fmt.Errorf("invalid replication observation for replacement recovery")
	}
	fields := map[string]string{}
	for _, line := range strings.Split(info, "\n") {
		key, value, ok := strings.Cut(strings.TrimSpace(line), ":")
		if !ok || strings.HasPrefix(key, "#") {
			continue
		}
		if _, exists := fields[key]; exists {
			return invalid()
		}
		fields[key] = value
	}
	result := ReplicationObservation{Role: fields["role"]}
	if result.Role != "master" && result.Role != "slave" {
		return invalid()
	}
	if result.Role == "slave" {
		result.MasterHost, result.MasterPort, result.MasterLink = fields["master_host"], fields["master_port"], fields["master_link_status"]
		priority, err := strconv.ParseUint(fields["slave_priority"], 10, 64)
		if err != nil || net.ParseIP(result.MasterHost) == nil || result.MasterPort != "6379" || (result.MasterLink != "up" && result.MasterLink != "down") {
			return invalid()
		}
		result.Priority = priority
	}
	count, err := strconv.ParseUint(fields["connected_slaves"], 10, 32)
	if err != nil || count > 1024 {
		return invalid()
	}
	for key := range fields {
		if !strings.HasPrefix(key, "slave") {
			continue
		}
		index, err := strconv.ParseUint(strings.TrimPrefix(key, "slave"), 10, 64)
		if err == nil && (index >= count || key != fmt.Sprintf("slave%d", index)) {
			return invalid()
		}
	}
	seen := map[string]bool{}
	for i := uint64(0); i < count; i++ {
		replica := map[string]string{}
		for _, field := range strings.Split(fields[fmt.Sprintf("slave%d", i)], ",") {
			key, value, ok := strings.Cut(field, "=")
			_, duplicate := replica[key]
			if !ok || duplicate {
				return invalid()
			}
			replica[key] = value
		}
		lag, err := strconv.ParseUint(replica["lag"], 10, 64)
		if err != nil || net.ParseIP(replica["ip"]) == nil || replica["port"] != "6379" || replica["state"] == "" || seen[replica["ip"]] {
			return invalid()
		}
		seen[replica["ip"]] = true
		result.Replicas = append(result.Replicas, AttachedReplica{IP: replica["ip"], Port: replica["port"], State: replica["state"], Lag: lag})
	}
	return result, nil
}

func ObserveSentinelReplacement(ctx context.Context, ip, password, group string) (SentinelReplacementObservation, error) {
	c := replacementClient(net.JoinHostPort(ip, "26379"), password)
	defer c.Close()
	return readSentinelReplacement(func(args ...interface{}) (interface{}, error) {
		return c.Do(ctx, args...).Result()
	}, group)
}

func readSentinelReplacement(ask func(...interface{}) (interface{}, error), group string) (SentinelReplacementObservation, error) {
	invalid := func() (SentinelReplacementObservation, error) {
		return SentinelReplacementObservation{}, fmt.Errorf("cannot verify stable Sentinel replacement evidence")
	}
	master := func() (*SentinelMasterState, error) {
		reply, err := ask("SENTINEL", "MASTER", group)
		fields, ok := reply.([]interface{})
		if err != nil || !ok {
			return nil, fmt.Errorf("invalid master observation")
		}
		return parseSentinelMaster(fields)
	}
	before, err := master()
	if err != nil || before.Flags != "master" || (before.FailoverState != "" && before.FailoverState != "none") {
		return invalid()
	}
	identity, err := ask("SENTINEL", "MYID")
	voter, ok := identity.(string)
	decoded, decodeErr := hex.DecodeString(voter)
	if err != nil || !ok || decodeErr != nil || len(decoded) != 20 || voter == strings.Repeat("0", 40) {
		return invalid()
	}
	reply, err := ask("SENTINEL", "REPLICAS", group)
	records, ok := reply.([]interface{})
	if err != nil || !ok {
		return invalid()
	}
	result := SentinelReplacementObservation{Master: *before, VoterID: voter}
	seen := map[string]bool{}
	for _, record := range records {
		values, ok := record.([]interface{})
		if !ok || len(values)%2 != 0 {
			return invalid()
		}
		fields := map[string]string{}
		for i := 0; i < len(values); i += 2 {
			key, keyOK := values[i].(string)
			value, valueOK := values[i+1].(string)
			_, duplicate := fields[key]
			if !keyOK || !valueOK || duplicate {
				return invalid()
			}
			fields[key] = value
		}
		if net.ParseIP(fields["ip"]) == nil || fields["port"] != "6379" || fields["flags"] == "" || seen[fields["ip"]] {
			return invalid()
		}
		seen[fields["ip"]] = true
		priority, priorityErr := strconv.ParseUint(fields["slave-priority"], 10, 64)
		age, ageErr := strconv.ParseUint(fields["info-refresh"], 10, 64)
		if priorityErr != nil || ageErr != nil {
			return invalid()
		}
		result.Replicas = append(result.Replicas, SentinelReplicaObservation{
			IP: fields["ip"], Port: fields["port"], Flags: fields["flags"], Role: fields["role-reported"],
			MasterHost: fields["master-host"], MasterPort: fields["master-port"], MasterLink: fields["master-link-status"], Priority: priority, InfoAge: age,
		})
	}
	identityAfter, err := ask("SENTINEL", "MYID")
	if err != nil || identityAfter != voter {
		return invalid()
	}
	after, err := master()
	if err != nil || *before != *after {
		return invalid()
	}
	return result, nil
}
