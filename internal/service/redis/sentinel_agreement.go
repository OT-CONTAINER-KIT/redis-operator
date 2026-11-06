package redis

import (
	"strconv"
	"strings"
)

type SentinelDecision struct {
	IP    string
	Epoch uint64
}

// AgreedSentinelMaster is a read-only publication/startup decision, never a
// permit to demote a running Redis member or replace an existing monitor.
func AgreedSentinelMaster(reports []SentinelMasterState, configured int) (SentinelDecision, bool) {
	if configured < 1 || len(reports) > configured {
		return SentinelDecision{}, false
	}
	votes := map[SentinelDecision]int{}
	healthy := func(r SentinelMasterState) bool { return r.Flags == "master" }
	for _, r := range reports {
		epoch, err := strconv.ParseUint(r.Epoch, 10, 64)
		if err != nil || r.IP == "" || r.Port != "6379" || !strings.Contains(","+r.Flags+",", ",master,") {
			return SentinelDecision{}, false
		}
		if healthy(r) {
			votes[SentinelDecision{IP: r.IP, Epoch: epoch}]++
		}
	}
	for candidate, count := range votes {
		if count < configured/2+1 {
			continue
		}
		for _, r := range reports {
			epoch, _ := strconv.ParseUint(r.Epoch, 10, 64)
			if epoch > candidate.Epoch {
				return SentinelDecision{}, false
			}
			if strings.Contains(","+r.Flags+",", ",failover_in_progress,") {
				advertised, bound := r.PostPromotionAddress()
				if bound && epoch == candidate.Epoch && advertised == candidate.IP {
					continue // The promotion is acknowledged; only replica reconfiguration remains.
				}
				return SentinelDecision{}, false
			}
			if epoch == candidate.Epoch && (r.IP != candidate.IP || !healthy(r)) {
				return SentinelDecision{}, false
			}
		}
		return candidate, true
	}
	return SentinelDecision{}, false
}
