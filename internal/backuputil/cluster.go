/*
Copyright 2020 Opstree Solutions.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package backuputil

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"
)

// TotalSlots is the size of the Redis Cluster keyspace.
const TotalSlots = 16384

// SlotSet is a bitmap over the cluster keyspace.
type SlotSet [TotalSlots]bool

// ParseSlotRanges turns the "start-end" / "single" tokens Redis prints into a
// SlotSet. Migration markers ("[123->-id]") are rejected — a backup taken
// mid-migration has no well-defined owner for that slot.
func ParseSlotRanges(tokens []string) (SlotSet, error) {
	var set SlotSet
	for _, tok := range tokens {
		if strings.HasPrefix(tok, "[") {
			return set, fmt.Errorf("slot %s is mid-migration; the topology is not stable", tok)
		}
		lo, hi, ok := strings.Cut(tok, "-")
		if !ok {
			hi = lo
		}
		a, err := strconv.Atoi(lo)
		if err != nil {
			return set, fmt.Errorf("bad slot token %q: %w", tok, err)
		}
		b, err := strconv.Atoi(hi)
		if err != nil {
			return set, fmt.Errorf("bad slot token %q: %w", tok, err)
		}
		if a < 0 || b >= TotalSlots || a > b {
			return set, fmt.Errorf("slot token %q is out of range", tok)
		}
		for i := a; i <= b; i++ {
			set[i] = true
		}
	}
	return set, nil
}

// Ranges compresses the set back into contiguous [start end] pairs.
func (s SlotSet) Ranges() [][2]int {
	var out [][2]int
	start := -1
	for i := range TotalSlots {
		switch {
		case s[i] && start < 0:
			start = i
		case !s[i] && start >= 0:
			out = append(out, [2]int{start, i - 1})
			start = -1
		}
	}
	if start >= 0 {
		out = append(out, [2]int{start, TotalSlots - 1})
	}
	return out
}

// Count returns how many slots are set.
func (s SlotSet) Count() int {
	n := 0
	for _, b := range s {
		if b {
			n++
		}
	}
	return n
}

// Minus returns the slots in s that are not in other.
func (s SlotSet) Minus(other SlotSet) SlotSet {
	var out SlotSet
	for i := range s {
		out[i] = s[i] && !other[i]
	}
	return out
}

// SubsetOf reports whether every slot in s is also in other.
func (s SlotSet) SubsetOf(other SlotSet) bool {
	for i := range s {
		if s[i] && !other[i] {
			return false
		}
	}
	return true
}

// DissolveClusterNode takes a cluster pod out of its cluster and empties it,
// so that after a restart it comes back as a fresh, isolated, empty node.
//
// Persistence is disabled first so terminating the pod writes nothing back.
// A replica has to be reset before it can be flushed (FLUSHALL is refused on
// a read-only node), and a master has to be flushed before it can be reset
// (CLUSTER RESET is refused on a node that still holds keys).
func (x *Executor) DissolveClusterNode(ctx context.Context, t Target, l Layout) error {
	if err := x.DisablePersistence(ctx, t); err != nil {
		return err
	}
	role, _, err := x.ReplicationRole(ctx, t)
	if err != nil {
		return fmt.Errorf("failed to read role of %s: %w", t.Pod, err)
	}
	if role == "master" {
		if _, err := x.RedisCLI(ctx, t, "FLUSHALL"); err != nil {
			return fmt.Errorf("failed to flush %s: %w", t.Pod, err)
		}
	}
	if _, err := x.RedisCLI(ctx, t, "CLUSTER", "RESET", "HARD"); err != nil {
		return fmt.Errorf("failed to reset cluster state on %s: %w", t.Pod, err)
	}
	if role != "master" {
		if _, err := x.RedisCLI(ctx, t, "FLUSHALL"); err != nil {
			return fmt.Errorf("failed to flush %s after reset: %w", t.Pod, err)
		}
	}

	paths := l.DataPaths()
	if l.NodeConfigFile != "" {
		paths = append(paths, l.NodeConfigFile)
	}
	if _, err := x.Exec(ctx, t, append([]string{"rm", "-rf"}, paths...)...); err != nil {
		return fmt.Errorf("failed to clear files on %s: %w", t.Pod, err)
	}
	return nil
}

// OwnedSlots reports the slots this node currently claims.
func (x *Executor) OwnedSlots(ctx context.Context, t Target) (SlotSet, error) {
	nodes, err := x.RedisCLI(ctx, t, "CLUSTER", "NODES")
	if err != nil {
		return SlotSet{}, fmt.Errorf("failed to read CLUSTER NODES on %s: %w", t.Pod, err)
	}
	return ParseSlotRanges(parseOwnedSlots(nodes))
}

// AddSlots assigns the given slots to this node. It prefers the single
// ADDSLOTSRANGE form and falls back to batched ADDSLOTS on servers that lack it.
func (x *Executor) AddSlots(ctx context.Context, t Target, slots SlotSet) error {
	ranges := slots.Ranges()
	if len(ranges) == 0 {
		return nil
	}
	args := []string{"CLUSTER", "ADDSLOTSRANGE"}
	for _, r := range ranges {
		args = append(args, strconv.Itoa(r[0]), strconv.Itoa(r[1]))
	}
	if _, err := x.RedisCLI(ctx, t, args...); err == nil {
		return nil
	}

	// Redis < 7: one slot per argument, in batches short enough for the exec
	// URL. ADDSLOTS is idempotent, so a partial batch is safe to retry.
	const batch = 512
	var pending []string
	flush := func() error {
		if len(pending) == 0 {
			return nil
		}
		if _, err := x.RedisCLI(ctx, t, append([]string{"CLUSTER", "ADDSLOTS"}, pending...)...); err != nil {
			return fmt.Errorf("failed to add slots on %s: %w", t.Pod, err)
		}
		pending = pending[:0]
		return nil
	}
	for i, own := range slots {
		if !own {
			continue
		}
		pending = append(pending, strconv.Itoa(i))
		if len(pending) >= batch {
			if err := flush(); err != nil {
				return err
			}
		}
	}
	return flush()
}

// SetConfigEpoch gives a fresh node a distinct epoch. Redis refuses this once
// the node knows a peer, so it must run before any MEET.
func (x *Executor) SetConfigEpoch(ctx context.Context, t Target, epoch int) error {
	if _, err := x.RedisCLI(ctx, t, "CLUSTER", "SET-CONFIG-EPOCH", strconv.Itoa(epoch)); err != nil {
		return fmt.Errorf("failed to set config epoch %d on %s: %w", epoch, t.Pod, err)
	}
	return nil
}

// ClusterAddress is what a node advertises to its peers.
type ClusterAddress struct {
	IP      string
	Port    int
	BusPort int
}

// AnnouncedAddress reads the address this node tells the cluster about. A
// cluster-announce-ip set by the operator (NodePort / hostNetwork modes)
// overrides the pod IP the caller has.
func (x *Executor) AnnouncedAddress(ctx context.Context, t Target, podIP string) (ClusterAddress, error) {
	addr := ClusterAddress{IP: podIP}
	if ip, err := x.ConfigGet(ctx, t, "cluster-announce-ip"); err == nil && ip != "" {
		addr.IP = ip
	}
	port, err := x.RedisPort(ctx, t)
	if err != nil {
		return addr, err
	}
	addr.Port = port
	if p, err := x.ConfigGet(ctx, t, "cluster-announce-port"); err == nil && p != "" && p != "0" {
		if n, convErr := strconv.Atoi(p); convErr == nil {
			addr.Port = n
		}
	}
	addr.BusPort = addr.Port + 10000
	if bp, err := x.ConfigGet(ctx, t, "cluster-announce-bus-port"); err == nil && bp != "" && bp != "0" {
		if n, convErr := strconv.Atoi(bp); convErr == nil {
			addr.BusPort = n
		}
	}
	return addr, nil
}

// Meet introduces a peer to this node.
func (x *Executor) Meet(ctx context.Context, t Target, peer ClusterAddress) error {
	if _, err := x.RedisCLI(ctx, t, "CLUSTER", "MEET", peer.IP, strconv.Itoa(peer.Port), strconv.Itoa(peer.BusPort)); err != nil {
		return fmt.Errorf("failed to MEET %s:%d from %s: %w", peer.IP, peer.Port, t.Pod, err)
	}
	return nil
}

// Replicate makes this node a replica of the given master.
func (x *Executor) Replicate(ctx context.Context, t Target, masterID string) error {
	if _, err := x.RedisCLI(ctx, t, "CLUSTER", "REPLICATE", masterID); err != nil {
		return fmt.Errorf("failed to make %s replicate %s: %w", t.Pod, masterID, err)
	}
	return nil
}

// NodeID reports this node's cluster identity.
func (x *Executor) NodeID(ctx context.Context, t Target) (string, error) {
	id, err := x.RedisCLI(ctx, t, "CLUSTER", "MYID")
	if err != nil {
		return "", fmt.Errorf("failed to read CLUSTER MYID on %s: %w", t.Pod, err)
	}
	return id, nil
}

// KnownMaster reports whether this node's view of the cluster includes the
// given node ID as a connected master (which CLUSTER REPLICATE requires).
func (x *Executor) KnownMaster(ctx context.Context, t Target, nodeID string) (bool, error) {
	nodes, err := x.RedisCLI(ctx, t, "CLUSTER", "NODES")
	if err != nil {
		return false, err
	}
	for line := range strings.SplitSeq(strings.ReplaceAll(nodes, "\r\n", "\n"), "\n") {
		f := strings.Fields(line)
		if len(f) < 8 || f[0] != nodeID {
			continue
		}
		flags := f[2]
		return strings.Contains(flags, "master") && !strings.Contains(flags, "handshake") && f[7] == "connected", nil
	}
	return false, nil
}

// ClusterHealth is the subset of CLUSTER INFO a restore gates on.
type ClusterHealth struct {
	State         string
	SlotsAssigned int
	SlotsOK       int
	KnownNodes    int
}

// Health reads CLUSTER INFO.
func (x *Executor) Health(ctx context.Context, t Target) (ClusterHealth, error) {
	out, err := x.RedisCLI(ctx, t, "CLUSTER", "INFO")
	if err != nil {
		return ClusterHealth{}, fmt.Errorf("failed to read CLUSTER INFO on %s: %w", t.Pod, err)
	}
	h := ClusterHealth{}
	for line := range strings.SplitSeq(strings.ReplaceAll(out, "\r\n", "\n"), "\n") {
		k, v, ok := strings.Cut(strings.TrimSpace(line), ":")
		if !ok {
			continue
		}
		switch k {
		case "cluster_state":
			h.State = v
		case "cluster_slots_assigned":
			h.SlotsAssigned, _ = strconv.Atoi(v)
		case "cluster_slots_ok":
			h.SlotsOK, _ = strconv.Atoi(v)
		case "cluster_known_nodes":
			h.KnownNodes, _ = strconv.Atoi(v)
		}
	}
	return h, nil
}

// WaitClusterOK polls until the node reports a fully assigned, healthy
// cluster of the expected size.
func (x *Executor) WaitClusterOK(ctx context.Context, t Target, expectNodes int, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	var last ClusterHealth
	for {
		h, err := x.Health(ctx, t)
		if err == nil {
			last = h
			if h.State == "ok" && h.SlotsAssigned == TotalSlots && h.SlotsOK == TotalSlots && h.KnownNodes >= expectNodes {
				return nil
			}
		}
		if time.Now().After(deadline) {
			return fmt.Errorf("timed out after %s waiting for the cluster to become healthy from %s "+
				"(state=%s slots_assigned=%d slots_ok=%d known_nodes=%d, expected %d)",
				timeout, t.Pod, last.State, last.SlotsAssigned, last.SlotsOK, last.KnownNodes, expectNodes)
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(2 * time.Second):
		}
	}
}

// WaitKnownMaster polls until this node sees the given master.
func (x *Executor) WaitKnownMaster(ctx context.Context, t Target, masterID string, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	for {
		ok, err := x.KnownMaster(ctx, t, masterID)
		if err == nil && ok {
			return nil
		}
		if time.Now().After(deadline) {
			return fmt.Errorf("timed out after %s waiting for %s to learn about master %s", timeout, t.Pod, masterID)
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(time.Second):
		}
	}
}
