package bootstrap

import (
	"context"
	"errors"
	"fmt"
	"net"
	"os"
	"strings"
	"time"

	agentutil "github.com/OT-CONTAINER-KIT/redis-operator/internal/agent/util"
	"github.com/OT-CONTAINER-KIT/redis-operator/internal/service/redis"
	"github.com/OT-CONTAINER-KIT/redis-operator/internal/util"
)

// A replacement joins as a replica before serving traffic. This initializer
// never sends REPLICAOF to a running member. If authority is unavailable it
// fails closed; Kubernetes retries the init container rather than inventing a
// primary or discarding the member's PVC data.
// A simultaneous cold start requires an explicit recovery workflow: this
// initializer never elects a primary while all Redis processes are stopped.
func sentinelStartup(cfg *agentutil.Config) error {
	addresses := util.CoalesceEnv1("SENTINEL_BOOTSTRAP_ADDRS", "")
	if addresses == "" {
		if util.CoalesceEnv1("SENTINEL_BOOTSTRAP_REQUIRED", "") == "true" {
			return fmt.Errorf("missing Sentinel configuration for activated replication")
		}
		return nil
	}
	if util.CoalesceEnv1("DATA_DIR", "/data") != "/data" {
		return fmt.Errorf("protected Sentinel startup requires the mounted /data directory")
	}
	if err := validateStartupOverrides(util.CoalesceEnv1("EXTERNAL_CONFIG_FILE", "/etc/redis/external.conf.d/redis-additional.conf")); err != nil {
		return err
	}
	group := util.CoalesceEnv1("SENTINEL_BOOTSTRAP_GROUP", "myMaster")
	password := util.CoalesceEnv1("SENTINEL_BOOTSTRAP_PASSWORD", "")
	redisPassword := util.CoalesceEnv1("REDIS_PASSWORD", "")
	peers := strings.Split(addresses, ",")
	var reports []redis.SentinelMasterState
	ctx, cancel := context.WithTimeout(context.Background(), 12*time.Second)
	defer cancel()
	for _, address := range peers {
		report, err := redis.NewClient().Connect(&redis.ConnectionInfo{Host: address, Port: "26379", Password: password}).GetSentinelMaster(ctx, group)
		if err == nil {
			reports = append(reports, *report)
		} else if errors.Is(err, redis.ErrInvalidSentinelReport) {
			return fmt.Errorf("cannot start from a malformed Sentinel authority report")
		}
	}
	decision, ok := redis.AgreedSentinelMaster(reports, len(peers))
	if !ok {
		return fmt.Errorf("waiting for Sentinel majority before starting a replacement Redis member")
	}
	if !startupSourceMember(ctx, decision.IP, strings.Split(util.CoalesceEnv1("SENTINEL_BOOTSTRAP_MEMBERS", ""), ","), net.DefaultResolver.LookupIPAddr) {
		return fmt.Errorf("sentinel startup source is not a current RedisReplication member")
	}
	primary, err := redis.NewClient().Connect(&redis.ConnectionInfo{Host: decision.IP, Port: "6379", Password: redisPassword}).IsMaster(ctx)
	if err != nil || !primary {
		return fmt.Errorf("waiting for the Sentinel primary to confirm its role")
	}
	keys, err := redis.NewClient().Connect(&redis.ConnectionInfo{Host: decision.IP, Port: "6379", Password: redisPassword}).GetKeyCount(ctx)
	if err != nil {
		return fmt.Errorf("cannot check startup primary data")
	}
	if keys == 0 && util.CoalesceEnv1("PERSISTENCE_ENABLED", "false") == "true" {
		empty, err := localDatasetEmpty(util.CoalesceEnv1("DATA_DIR", "/data"))
		if err != nil || !empty {
			return fmt.Errorf("refusing to attach persisted or unrecognized local data to an empty primary")
		}
	}
	if net.ParseIP(decision.IP) == nil {
		return fmt.Errorf("sentinel startup requires an IP address")
	}
	cfg.Append("replicaof", decision.IP, "6379")
	return nil
}

func validateStartupOverrides(path string) error {
	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("cannot validate external startup configuration")
	}
	if util.CoalesceEnv1("EXPAND_EXTERNAL_CONFIG", "false") == "true" {
		return fmt.Errorf("expanded external configuration is not supported by protected Sentinel startup")
	}
	for _, line := range strings.Split(string(data), "\n") {
		fields := strings.Fields(line)
		if len(fields) == 0 || strings.HasPrefix(fields[0], "#") {
			continue
		}
		key := strings.ToLower(fields[0])
		if strings.ContainsAny(key, "\"'\\") {
			return fmt.Errorf("quoted or escaped external directives are not supported by protected Sentinel startup")
		}
		if strings.Contains(key, "$") {
			return fmt.Errorf("dynamic external directive is not supported by protected Sentinel startup")
		}
		switch key {
		case "replicaof", "slaveof", "include", "dir", "dbfilename", "appenddirname", "appendfilename", "appendonly", "cluster-enabled", "requirepass", "masterauth":
			return fmt.Errorf("external role, persistence or authentication overrides are not supported by protected Sentinel startup")
		}
	}
	return nil
}

func startupSourceMember(ctx context.Context, ip string, members []string, lookup func(context.Context, string) ([]net.IPAddr, error)) bool {
	if net.ParseIP(ip) == nil {
		return false
	}
	for _, member := range members {
		if member == "" {
			continue
		}
		addresses, err := lookup(ctx, member)
		if err != nil {
			continue
		}
		for _, address := range addresses {
			if address.IP.Equal(net.ParseIP(ip)) {
				return true
			}
		}
	}
	return false
}
