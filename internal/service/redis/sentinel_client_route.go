package redis

import (
	"context"
	"errors"
	"fmt"
	"net"
	"strings"
)

// ErrInvalidSentinelReport distinguishes a malformed reachable response from
// an unavailable peer. It must not be silently discarded as a missing vote.
var ErrInvalidSentinelReport = errors.New("invalid Sentinel authority report")

// ReadSentinelMasterState retains raw election evidence and, after acknowledged
// promotion only, binds the client address to two identical raw state reads.
// Redis advertises its promoted member before MASTER's diagnostic address is
// replaced: redis/redis 8.4.0 src/sentinel.c, sentinelGetCurrentMasterAddress.
func ReadSentinelMasterState(ctx context.Context, group string, query func(context.Context, ...string) (interface{}, error)) (*SentinelMasterState, error) {
	read := func() (*SentinelMasterState, error) {
		reply, err := query(ctx, "SENTINEL", "MASTER", group)
		if err != nil {
			return nil, err
		}
		values, ok := reply.([]interface{})
		if !ok {
			return nil, fmt.Errorf("%w: malformed master response", ErrInvalidSentinelReport)
		}
		return parseSentinelMaster(values)
	}
	before, err := read()
	if err != nil || !before.postPromotionPhase() {
		return before, err
	}
	reply, err := query(ctx, "SENTINEL", "GET-MASTER-ADDR-BY-NAME", group)
	if err != nil {
		// Never turn a known active election into an unavailable/no-vote peer.
		return before, nil
	}
	address, ok := reply.([]interface{})
	if !ok || len(address) != 2 {
		return before, nil
	}
	ip, ipOK := address[0].(string)
	port, portOK := address[1].(string)
	after, err := read()
	if err != nil || !ipOK || !portOK || *before != *after {
		return before, nil
	}
	before.clientIP, before.clientPort = ip, port
	return before, nil
}

func (r SentinelMasterState) postPromotionPhase() bool {
	if r.FailoverState != "reconf_slaves" && r.FailoverState != "update_config" {
		return false
	}
	flags := map[string]bool{}
	for _, flag := range strings.Split(r.Flags, ",") {
		switch flag {
		case "master", "failover_in_progress", "force_failover", "s_down", "o_down", "disconnected", "master_down":
			flags[flag] = true
		default:
			return false
		}
	}
	return flags["master"] && flags["failover_in_progress"]
}

// PostPromotionAddress is evidence of compatibility, not an additional healthy
// vote. A healthy majority at this exact address and epoch is still required.
func (r SentinelMasterState) PostPromotionAddress() (string, bool) {
	valid := r.postPromotionPhase() && r.clientIP != r.IP && net.ParseIP(r.clientIP) != nil && r.Port == "6379" && r.clientPort == "6379"
	return r.clientIP, valid
}
