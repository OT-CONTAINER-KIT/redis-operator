// Package readiness performs bounded, read-only standalone Sentinel readiness.
package readiness

import (
	"context"
	"errors"
	"net"
	"strconv"
	"strings"
	"time"

	redisstate "github.com/OT-CONTAINER-KIT/redis-operator/internal/service/redis"
	redis "github.com/redis/go-redis/v9"
)

type Config struct {
	MemberIP, Group, RedisPassword, SentinelPassword string
	Peers                                            []string
}

type queryFunc func(context.Context, string, string, ...string) (interface{}, error)

// Check has no subprocesses and never returns server replies or credentials in
// an error. The caller also supplies an overall deadline for the entire probe.
func Check(ctx context.Context, cfg Config) bool {
	return check(ctx, cfg, query)
}

func query(ctx context.Context, address, password string, args ...string) (interface{}, error) {
	client := redis.NewClient(&redis.Options{
		Addr: address, Password: password, Protocol: 2, DisableIdentity: true,
		DialTimeout: time.Second, ReadTimeout: time.Second, WriteTimeout: time.Second,
		ContextTimeoutEnabled: true, MaxRetries: -1,
	})
	defer client.Close()
	command := make([]interface{}, len(args))
	for i, arg := range args {
		command[i] = arg
	}
	return client.Do(ctx, command...).Result()
}

func check(ctx context.Context, cfg Config, ask queryFunc) bool {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	if net.ParseIP(cfg.MemberIP) == nil || len(cfg.Peers) == 0 || cfg.Group == "" {
		return false
	}
	ping, err := ask(ctx, "127.0.0.1:6379", cfg.RedisPassword, "PING")
	if err != nil || ping != "PONG" {
		return false
	}
	reply, err := ask(ctx, "127.0.0.1:6379", cfg.RedisPassword, "INFO", "replication")
	info, ok := reply.(string)
	if err != nil || !ok {
		return false
	}
	local := map[string]string{}
	for _, line := range strings.Split(info, "\n") {
		if key, value, ok := strings.Cut(strings.TrimSpace(line), ":"); ok {
			local[key] = value
		}
	}
	wanted := cfg.MemberIP
	switch local["role"] {
	case "master":
	case "slave":
		wanted = local["master_host"]
		if local["master_link_status"] != "up" || net.ParseIP(wanted) == nil {
			return false
		}
	default:
		return false
	}
	type observation struct {
		report        *redisstate.SentinelMasterState
		contradiction bool
	}
	results := make(chan observation, len(cfg.Peers))
	seen := map[string]bool{}
	for _, peer := range cfg.Peers {
		if peer == "" || seen[peer] {
			return false
		}
		seen[peer] = true
	}
	for _, peer := range cfg.Peers {
		go func(peer string) {
			observed := observation{}
			defer func() { results <- observed }()
			master, err := redisstate.ReadSentinelMasterState(ctx, cfg.Group, func(ctx context.Context, args ...string) (interface{}, error) {
				return ask(ctx, net.JoinHostPort(peer, "26379"), cfg.SentinelPassword, args...)
			})
			if err != nil {
				if errors.Is(err, redisstate.ErrInvalidSentinelReport) {
					observed.contradiction = true
				}
				return // An unreachable participant supplies no vote.
			}
			address := master.IP
			advertised, promoted := master.PostPromotionAddress()
			if promoted {
				address = advertised
			}
			if address != wanted || master.Port != "6379" || (master.Flags != "master" && !promoted) {
				observed.contradiction = true
				return
			}
			if local["role"] == "slave" {
				reply, err := ask(ctx, net.JoinHostPort(peer, "26379"), cfg.SentinelPassword, "SENTINEL", "REPLICAS", cfg.Group)
				if err != nil || !hasEligibleReplica(reply, cfg.MemberIP, wanted) {
					observed.contradiction = true
					return
				}
			}
			observed.report = master
		}(peer)
	}
	var reports []redisstate.SentinelMasterState
	for range cfg.Peers {
		select {
		case <-ctx.Done():
			return false
		case observed := <-results:
			if observed.contradiction {
				return false
			}
			if observed.report != nil {
				reports = append(reports, *observed.report)
			}
		}
	}
	decision, agreed := redisstate.AgreedSentinelMaster(reports, len(cfg.Peers))
	return agreed && decision.IP == wanted
}

func fields(reply interface{}) (map[string]string, bool) {
	values, ok := reply.([]interface{})
	if !ok || len(values)%2 != 0 {
		return nil, false
	}
	result := map[string]string{}
	for i := 0; i < len(values); i += 2 {
		key, keyOK := values[i].(string)
		value, valueOK := values[i+1].(string)
		if !keyOK || !valueOK {
			return nil, false
		}
		if _, duplicate := result[key]; duplicate {
			return nil, false
		}
		result[key] = value
	}
	return result, true
}

func hasEligibleReplica(reply interface{}, own, wanted string) bool {
	records, ok := reply.([]interface{})
	if !ok {
		return false
	}
	for _, record := range records {
		replica, ok := fields(record)
		if !ok || replica["ip"] != own {
			continue
		}
		flags := "," + replica["flags"] + ","
		priority, priorityErr := strconv.ParseUint(replica["slave-priority"], 10, 64)
		refresh, refreshErr := strconv.ParseUint(replica["info-refresh"], 10, 64)
		if strings.Contains(flags, ",slave,") && !strings.Contains(flags, ",s_down,") && !strings.Contains(flags, ",o_down,") && !strings.Contains(flags, ",disconnected,") &&
			replica["role-reported"] == "slave" && replica["master-link-status"] == "ok" && replica["master-host"] == wanted &&
			priorityErr == nil && priority > 0 && refreshErr == nil && refresh > 0 {
			return true
		}
	}
	return false
}
