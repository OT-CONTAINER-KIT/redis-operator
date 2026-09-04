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

// InfoSection parses one INFO section into key/value pairs.
func (x *Executor) InfoSection(ctx context.Context, t Target, section string) (map[string]string, error) {
	out, err := x.RedisCLI(ctx, t, "INFO", section)
	if err != nil {
		return nil, err
	}
	fields := make(map[string]string)
	for line := range strings.SplitSeq(strings.ReplaceAll(out, "\r\n", "\n"), "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		if k, v, ok := strings.Cut(line, ":"); ok {
			fields[strings.TrimSpace(k)] = strings.TrimSpace(v)
		}
	}
	return fields, nil
}

// IsMaster reports whether this pod currently serves as a replication master.
func (x *Executor) IsMaster(ctx context.Context, t Target) (bool, error) {
	role, _, err := x.ReplicationRole(ctx, t)
	return role == "master", err
}

// ReplicationRole reports the pod's role and, for a master, how many replicas
// are attached. The attached count is what disambiguates a split brain: the
// side the replicas still follow is the live one.
func (x *Executor) ReplicationRole(ctx context.Context, t Target) (role string, connectedSlaves int, err error) {
	info, err := x.InfoSection(ctx, t, "replication")
	if err != nil {
		return "", 0, err
	}
	n, _ := strconv.Atoi(info["connected_slaves"])
	return info["role"], n, nil
}

// DisablePersistence stops Redis writing its dataset to disk.
//
// It is called before the on-disk files are replaced so that terminating the
// pod afterwards cannot flush the in-memory dataset back over them. Without
// it, shutting a pod down rewrites exactly the files the restore just placed.
func (x *Executor) DisablePersistence(ctx context.Context, t Target) error {
	if _, err := x.RedisCLI(ctx, t, "CONFIG", "SET", "appendonly", "no"); err != nil {
		return fmt.Errorf("failed to disable AOF on %s: %w", t.Pod, err)
	}
	if _, err := x.RedisCLI(ctx, t, "CONFIG", "SET", "save", ""); err != nil {
		return fmt.Errorf("failed to disable RDB snapshots on %s: %w", t.Pod, err)
	}
	return nil
}

// EnablePersistence reverses DisablePersistence using the settings captured
// in the layout, so a restore that fails midway does not leave a serving
// Redis with persistence silently off until its next container start.
func (x *Executor) EnablePersistence(ctx context.Context, t Target, l Layout) error {
	if l.AppendOnly {
		if _, err := x.RedisCLI(ctx, t, "CONFIG", "SET", "appendonly", "yes"); err != nil {
			return fmt.Errorf("failed to re-enable AOF on %s: %w", t.Pod, err)
		}
	}
	if _, err := x.RedisCLI(ctx, t, "CONFIG", "SET", "save", l.SavePolicy); err != nil {
		return fmt.Errorf("failed to restore save policy on %s: %w", t.Pod, err)
	}
	return nil
}

// ClearData removes the on-disk dataset so the pod restarts empty.
//
// Replicas and cluster followers must be emptied rather than restored into:
// leaving their pre-restore data in place lets them serve stale reads, and
// lets them win a master election and replicate the old dataset back over the
// restored one.
func (x *Executor) ClearData(ctx context.Context, t Target, l Layout) error {
	if err := x.DisablePersistence(ctx, t); err != nil {
		return err
	}
	// A replica is read-only and refuses FLUSHALL; detach it first. Nothing
	// depends on the link any more — every pod is restarted and re-attached
	// to the restored master afterwards.
	if err := x.PromoteMaster(ctx, t); err != nil {
		return err
	}
	if _, err := x.Exec(ctx, t, append([]string{"rm", "-rf"}, l.DataPaths()...)...); err != nil {
		return fmt.Errorf("failed to clear data on %s: %w", t.Pod, err)
	}
	// Drop the in-memory copy too, so nothing can be replicated out of this
	// pod between now and its restart.
	if _, err := x.RedisCLI(ctx, t, "FLUSHALL"); err != nil {
		return fmt.Errorf("failed to flush %s: %w", t.Pod, err)
	}
	return nil
}

// PromoteMaster detaches a pod from any master it is following.
func (x *Executor) PromoteMaster(ctx context.Context, t Target) error {
	if _, err := x.RedisCLI(ctx, t, "REPLICAOF", "NO", "ONE"); err != nil {
		return fmt.Errorf("failed to promote %s to master: %w", t.Pod, err)
	}
	return nil
}

// FollowMaster points a pod at the given master and waits for the initial sync
// to complete, so the caller knows the replica actually holds the data.
func (x *Executor) FollowMaster(ctx context.Context, t Target, masterHost string, masterPort int, timeout time.Duration) error {
	port := fmt.Sprintf("%d", masterPort)
	if _, err := x.RedisCLI(ctx, t, "REPLICAOF", masterHost, port); err != nil {
		return fmt.Errorf("failed to point %s at %s:%s: %w", t.Pod, masterHost, port, err)
	}
	return x.WaitReplicaSynced(ctx, t, masterHost, masterPort, timeout)
}

// WaitReplicaSynced waits until the pod reports a healthy, fully synced link
// to the given master. It issues no command of its own, so it serves cluster
// replicas too, where REPLICAOF is refused and CLUSTER REPLICATE has already
// been sent.
func (x *Executor) WaitReplicaSynced(ctx context.Context, t Target, masterHost string, masterPort int, timeout time.Duration) error {
	port := fmt.Sprintf("%d", masterPort)
	deadline := time.Now().Add(timeout)
	for {
		info, err := x.InfoSection(ctx, t, "replication")
		// Up, to the master we asked for, and not still receiving the initial
		// dataset — a link can report up while the bulk transfer is in flight.
		if err == nil && info["master_link_status"] == "up" &&
			info["master_host"] == masterHost && info["master_port"] == port &&
			info["master_sync_in_progress"] == "0" {
			return nil
		}
		if time.Now().After(deadline) {
			status := "unknown"
			if info != nil {
				status = info["master_link_status"]
			}
			return fmt.Errorf("timed out after %s waiting for %s to sync from %s (master_link_status=%s)",
				timeout, t.Pod, masterHost, status)
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(2 * time.Second):
		}
	}
}

// RedisPort reports the port the server accepts clients on.
//
// Under TLS the operator sets `port 0` and `tls-port 6379`; peers (REPLICAOF,
// CLUSTER MEET) must be given the TLS port, not the disabled plaintext one.
func (x *Executor) RedisPort(ctx context.Context, t Target) (int, error) {
	for _, key := range []string{"port", "tls-port"} {
		out, err := x.ConfigGet(ctx, t, key)
		if err != nil {
			return 0, err
		}
		if n, convErr := strconv.Atoi(strings.TrimSpace(out)); convErr == nil && n > 0 {
			return n, nil
		}
	}
	return 0, fmt.Errorf("%s reports neither a plaintext port nor a tls-port", t.Pod)
}
