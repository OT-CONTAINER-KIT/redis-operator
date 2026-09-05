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
	"os"
	"strconv"
	"strings"
	"time"
)

// ManifestVersion is bumped when the on-disk backup layout changes.
const ManifestVersion = 1

// Layout is where a live Redis server keeps its data, read from the server
// itself rather than assumed to be /data.
type Layout struct {
	Dir            string `json:"dir"`
	DBFilename     string `json:"dbFilename"`
	AppendOnly     bool   `json:"appendOnly"`
	AppendDirname  string `json:"appendDirname,omitempty"`  // Redis 7+: multi-part AOF directory
	AppendFilename string `json:"appendFilename,omitempty"` // Redis 6: the single AOF file
	SavePolicy     string `json:"savePolicy,omitempty"`
	ClusterEnabled bool   `json:"clusterEnabled"`
	NodeConfigFile string `json:"nodeConfigFile,omitempty"`

	// What the shard held at snapshot time, so a restore has something to
	// verify against instead of merely logging DBSIZE.
	SourcePod string `json:"sourcePod,omitempty"`
	DBSize    int64  `json:"dbSize"`

	// Cluster identity. A restore does not ship nodes.conf — it rebuilds
	// slot ownership from these ranges, which survive pod IP changes.
	NodeID string   `json:"nodeId,omitempty"`
	Slots  []string `json:"slots,omitempty"` // "start-end" or "single"
}

// CaptureShardInfo fills in the dataset size and, for cluster nodes, the
// node's identity and owned slot ranges.
func (x *Executor) CaptureShardInfo(ctx context.Context, t Target, l *Layout) error {
	l.SourcePod = t.Pod

	out, err := x.RedisCLI(ctx, t, "DBSIZE")
	if err != nil {
		return fmt.Errorf("failed to read DBSIZE on %s: %w", t.Pod, err)
	}
	if l.DBSize, err = strconv.ParseInt(strings.TrimSpace(out), 10, 64); err != nil {
		return fmt.Errorf("unexpected DBSIZE %q on %s: %w", out, t.Pod, err)
	}

	if !l.ClusterEnabled {
		return nil
	}
	if l.NodeID, err = x.RedisCLI(ctx, t, "CLUSTER", "MYID"); err != nil {
		return fmt.Errorf("failed to read CLUSTER MYID on %s: %w", t.Pod, err)
	}
	nodes, err := x.RedisCLI(ctx, t, "CLUSTER", "NODES")
	if err != nil {
		return fmt.Errorf("failed to read CLUSTER NODES on %s: %w", t.Pod, err)
	}
	l.Slots = parseOwnedSlots(nodes)
	return nil
}

// parseOwnedSlots returns the slot ranges on the "myself" line of CLUSTER NODES.
//
// Line format: <id> <ip:port@cport> <flags> <master> <ping> <pong> <epoch>
// <link-state> <slot|start-end> ... — slots in transit are shown in brackets
// and are skipped.
func parseOwnedSlots(clusterNodes string) []string {
	var slots []string
	for line := range strings.SplitSeq(strings.ReplaceAll(clusterNodes, "\r\n", "\n"), "\n") {
		fields := strings.Fields(line)
		if len(fields) < 8 || !strings.Contains(fields[2], "myself") {
			continue
		}
		for _, f := range fields[8:] {
			if strings.HasPrefix(f, "[") {
				continue
			}
			slots = append(slots, f)
		}
		break
	}
	return slots
}

// Manifest is written next to the data files so a restore knows what it is
// looking at instead of guessing from filenames.
type Manifest struct {
	Version     int        `json:"version"`
	Kind        TargetKind `json:"kind"`
	OwnerName   string     `json:"ownerName"`
	Shards      int        `json:"shards"`
	CreatedAt   string     `json:"createdAt"`
	PerShard    []Layout   `json:"perShard"`
	RedisServer string     `json:"redisServer,omitempty"`
}

// DiscoverLayout asks the running server where it keeps its files.
//
// This replaces hardcoded /data paths. node.conf in particular is not in the
// data directory at all: the operator mounts it at /node-conf and Redis names
// it nodes.conf, so a hardcoded /data/node.conf never matches anything.
func (x *Executor) DiscoverLayout(ctx context.Context, t Target) (Layout, error) {
	l := Layout{}

	dir, err := x.ConfigGet(ctx, t, "dir")
	if err != nil {
		return l, fmt.Errorf("failed to read Redis 'dir': %w", err)
	}
	if dir == "" {
		return l, fmt.Errorf("redis reported an empty 'dir'; cannot locate the data directory")
	}
	l.Dir = strings.TrimRight(dir, "/")

	if l.DBFilename, err = x.ConfigGet(ctx, t, "dbfilename"); err != nil {
		return l, err
	}
	if l.DBFilename == "" {
		l.DBFilename = "dump.rdb"
	}

	ao, err := x.ConfigGet(ctx, t, "appendonly")
	if err != nil {
		return l, err
	}
	l.AppendOnly = strings.EqualFold(ao, "yes")

	if l.AppendOnly {
		// Redis 7 keeps a multi-part AOF in a directory; Redis 6 keeps a single
		// file. `appenddirname` does not exist on 6, so an empty answer is
		// how the two are told apart.
		if l.AppendDirname, err = x.ConfigGet(ctx, t, "appenddirname"); err != nil {
			return l, err
		}
		if l.AppendFilename, err = x.ConfigGet(ctx, t, "appendfilename"); err != nil {
			return l, err
		}
		if l.AppendDirname == "" && l.AppendFilename == "" {
			l.AppendFilename = "appendonly.aof"
		}
	}

	// Kept so a failed restore can put persistence back the way it found it.
	if l.SavePolicy, err = x.ConfigGet(ctx, t, "save"); err != nil {
		return l, err
	}

	ce, err := x.ConfigGet(ctx, t, "cluster-enabled")
	if err != nil {
		return l, err
	}
	l.ClusterEnabled = strings.EqualFold(ce, "yes")

	if l.ClusterEnabled {
		// Typically /node-conf/nodes.conf under this operator, which is a
		// different mount from the data directory.
		if l.NodeConfigFile, err = x.ConfigGet(ctx, t, "cluster-config-file"); err != nil {
			return l, err
		}
	}
	return l, nil
}

// Snapshot forces Redis to flush a consistent copy of its dataset to disk.
//
// Both persistence formats are handled, because which one matters depends on
// the server's own configuration: with appendonly yes Redis loads the AOF on
// start and never reads the RDB, so backing up only dump.rdb produces an
// archive that cannot be restored.
//
// Redis runs at most one persistence child at a time. A busy master may
// already be mid-BGSAVE from its save policy, or have an AOF rewrite queued;
// asking for another while a child is active is refused outright. So each
// step first waits for the server to be idle, then uses the monotonic
// rdb_saves / aof_rewrites counters (Redis 7+) to know its own request has
// finished — LASTSAVE alone has second resolution and cannot distinguish a
// save that completed in the same second as the baseline read.
func (x *Executor) Snapshot(ctx context.Context, t Target, l Layout, timeout time.Duration) error {
	if l.AppendOnly {
		if err := x.rewriteAOF(ctx, t, timeout); err != nil {
			return err
		}
	}
	return x.saveRDB(ctx, t, timeout)
}

// waitPersistenceIdle blocks until no RDB save or AOF rewrite is running or
// scheduled, so a request we issue next is accepted rather than refused.
func (x *Executor) waitPersistenceIdle(ctx context.Context, t Target, deadline time.Time) (map[string]string, error) {
	for {
		info, err := x.infoPersistence(ctx, t)
		if err != nil {
			return nil, fmt.Errorf("failed to poll persistence info: %w", err)
		}
		if info["rdb_bgsave_in_progress"] == "0" && info["aof_rewrite_in_progress"] == "0" && info["aof_rewrite_scheduled"] == "0" {
			return info, nil
		}
		if time.Now().After(deadline) {
			return nil, fmt.Errorf("timed out waiting for an in-flight save or AOF rewrite on %s to finish "+
				"(rdb_bgsave_in_progress=%s aof_rewrite_in_progress=%s aof_rewrite_scheduled=%s)",
				t.Pod, info["rdb_bgsave_in_progress"], info["aof_rewrite_in_progress"], info["aof_rewrite_scheduled"])
		}
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(500 * time.Millisecond):
		}
	}
}

// saveRDB triggers a background save and waits for it to complete.
func (x *Executor) saveRDB(ctx context.Context, t Target, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	before, err := x.waitPersistenceIdle(ctx, t, deadline)
	if err != nil {
		return err
	}
	baselineSaves, haveCounter := counter(before, "rdb_saves")
	baselineLast, err := x.lastSave(ctx, t)
	if err != nil {
		return fmt.Errorf("failed to read LASTSAVE baseline: %w", err)
	}

	// SCHEDULE never fails on a busy child: it runs as soon as one is free.
	if _, err := x.RedisCLI(ctx, t, "BGSAVE", "SCHEDULE"); err != nil {
		return fmt.Errorf("failed to trigger BGSAVE: %w", err)
	}

	for {
		info, err := x.infoPersistence(ctx, t)
		if err != nil {
			return fmt.Errorf("failed to poll persistence info: %w", err)
		}
		if info["rdb_bgsave_in_progress"] == "0" {
			done := false
			if haveCounter {
				if now, ok := counter(info, "rdb_saves"); ok && now > baselineSaves {
					done = true
				}
			} else if current, lerr := x.lastSave(ctx, t); lerr == nil && current > baselineLast {
				done = true
			}
			if done {
				if status := info["rdb_last_bgsave_status"]; status != "" && status != "ok" {
					return fmt.Errorf("BGSAVE reported failure on %s: rdb_last_bgsave_status=%s", t.Pod, status)
				}
				return nil
			}
		}
		if time.Now().After(deadline) {
			return fmt.Errorf("timed out after %s waiting for BGSAVE to complete on %s", timeout, t.Pod)
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(500 * time.Millisecond):
		}
	}
}

// rewriteAOF compacts the append-only file so the captured AOF is a coherent
// point-in-time image rather than a partially written incr file.
func (x *Executor) rewriteAOF(ctx context.Context, t Target, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	before, err := x.waitPersistenceIdle(ctx, t, deadline)
	if err != nil {
		return err
	}
	baseline, haveCounter := counter(before, "aof_rewrites")

	// Redis may reply "scheduled" rather than "started" if a child appeared in
	// the meantime; either way the counter tells us when our rewrite has run.
	if _, err := x.RedisCLI(ctx, t, "BGREWRITEAOF"); err != nil {
		return fmt.Errorf("failed to trigger BGREWRITEAOF: %w", err)
	}

	for {
		info, err := x.infoPersistence(ctx, t)
		if err != nil {
			return fmt.Errorf("failed to poll persistence info: %w", err)
		}
		if info["aof_rewrite_in_progress"] == "0" && info["aof_rewrite_scheduled"] == "0" {
			done := !haveCounter
			if haveCounter {
				if now, ok := counter(info, "aof_rewrites"); ok && now > baseline {
					done = true
				}
			}
			if done {
				if status := info["aof_last_bgrewrite_status"]; status != "" && status != "ok" {
					return fmt.Errorf("BGREWRITEAOF reported failure on %s: aof_last_bgrewrite_status=%s", t.Pod, status)
				}
				return nil
			}
		}
		if time.Now().After(deadline) {
			return fmt.Errorf("timed out after %s waiting for BGREWRITEAOF to complete on %s", timeout, t.Pod)
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(500 * time.Millisecond):
		}
	}
}

// counter reads a monotonic INFO field, reporting whether the server has it.
func counter(info map[string]string, key string) (int64, bool) {
	v, ok := info[key]
	if !ok {
		return 0, false
	}
	n, err := strconv.ParseInt(strings.TrimSpace(v), 10, 64)
	return n, err == nil
}

func (x *Executor) lastSave(ctx context.Context, t Target) (int64, error) {
	out, err := x.RedisCLI(ctx, t, "LASTSAVE")
	if err != nil {
		return 0, err
	}
	return strconv.ParseInt(strings.TrimSpace(out), 10, 64)
}

func (x *Executor) infoPersistence(ctx context.Context, t Target) (map[string]string, error) {
	return x.InfoSection(ctx, t, "persistence")
}

// AlignNames renames the just-copied archive entries in the target's data
// directory to the names the target server is configured to load.
//
//   - dbfilename: renamed if it differs.
//   - AOF, both sides multi-part (Redis 7): the directory is renamed if the
//     appenddirname differs.
//   - AOF, both sides single-file (Redis 6): the file is renamed if the
//     appendfilename differs.
//   - AOF, source single-file into a multi-part target (6 -> 7): the file is
//     named after the target's appendfilename and left in the data directory,
//     which is exactly where Redis 7 looks for a legacy AOF to convert.
//
// Source and target names are operator/server-controlled, never user input,
// and are passed to mv as argv.
func (x *Executor) AlignNames(ctx context.Context, t Target, src, dst Layout) error {
	rename := func(from, to string) error {
		if from == "" || to == "" || from == to {
			return nil
		}
		if _, err := x.Exec(ctx, t, "mv", "-f", dst.Dir+"/"+from, dst.Dir+"/"+to); err != nil {
			return fmt.Errorf("failed to rename %s to %s in %s: %w", from, to, dst.Dir, err)
		}
		return nil
	}
	if err := rename(src.DBFilename, dst.DBFilename); err != nil {
		return err
	}
	if !dst.AppendOnly {
		return nil
	}
	srcAOF := src.AOFEntry()
	switch {
	case srcAOF == "":
		return nil
	case src.AppendDirname != "" && dst.AppendDirname != "":
		if err := rename(src.AppendDirname, dst.AppendDirname); err != nil {
			return err
		}
		// Every file in a Redis 7 AOF directory is named after appendfilename
		// (<prefix>.N.base.rdb, <prefix>.N.incr.aof, <prefix>.manifest), and
		// the manifest lists those names. If the prefix differs the server
		// finds no manifest, treats the directory as empty and starts empty.
		return x.alignAOFPrefix(ctx, t, dst.Dir+"/"+dst.AppendDirname, src.AppendFilename, dst.AppendFilename)
	case src.AppendDirname == "" && dst.AppendDirname == "":
		return rename(src.AppendFilename, dst.AppendFilename)
	case src.AppendDirname == "" && dst.AppendDirname != "":
		// Legacy single file into a Redis 7 server: keep it a file, under the
		// name the server will probe for before building its multi-part set.
		return rename(src.AppendFilename, dst.AppendFilename)
	default:
		return fmt.Errorf("cannot place a multi-part AOF (%s) into a server that only reads a single %s", src.AppendDirname, dst.AppendFilename)
	}
}

// alignAOFPrefix renames the multi-part AOF files inside dir from one
// appendfilename prefix to another and rewrites the manifest to match.
//
// The manifest is edited in Go and streamed back, rather than through sed in
// the pod: it is data, and the prefixes are server-configured names that are
// never spliced into a shell string.
func (x *Executor) alignAOFPrefix(ctx context.Context, t Target, dir, from, to string) error {
	if to == "" {
		return nil
	}
	listing, err := x.Exec(ctx, t, "ls", "-1", dir)
	if err != nil {
		return fmt.Errorf("failed to list %s: %w", dir, err)
	}
	if from == "" {
		// An archive written before the manifest recorded appendfilename.
		// The prefix is still knowable: the multi-part set's own manifest is
		// named <prefix>.manifest, and it is the only such file the archive
		// can carry (the target's own, if any, was removed before the copy).
		for name := range strings.SplitSeq(strings.TrimSpace(listing), "\n") {
			name = strings.TrimSpace(name)
			if strings.HasSuffix(name, ".manifest") && name != to+".manifest" {
				from = strings.TrimSuffix(name, ".manifest")
				break
			}
		}
	}
	if from == "" || from == to {
		return nil
	}
	renamed := 0
	for name := range strings.SplitSeq(strings.TrimSpace(listing), "\n") {
		name = strings.TrimSpace(name)
		if name == "" || !strings.HasPrefix(name, from+".") {
			continue
		}
		newName := to + strings.TrimPrefix(name, from)
		if _, err := x.Exec(ctx, t, "mv", "-f", dir+"/"+name, dir+"/"+newName); err != nil {
			return fmt.Errorf("failed to rename %s to %s: %w", name, newName, err)
		}
		renamed++
	}
	if renamed == 0 {
		return nil
	}

	manifestPath := dir + "/" + to + ".manifest"
	content, err := x.Exec(ctx, t, "cat", manifestPath)
	if err != nil {
		return fmt.Errorf("failed to read AOF manifest %s: %w", manifestPath, err)
	}
	// Manifest lines look like: file appendonly.aof.1.base.rdb seq 1 type b
	var out []string
	for line := range strings.SplitSeq(strings.ReplaceAll(content, "\r\n", "\n"), "\n") {
		fields := strings.Fields(line)
		if len(fields) >= 2 && fields[0] == "file" && strings.HasPrefix(fields[1], from+".") {
			fields[1] = to + strings.TrimPrefix(fields[1], from)
			line = strings.Join(fields, " ")
		}
		out = append(out, line)
	}
	rewritten := strings.Join(out, "\n")
	if !strings.HasSuffix(rewritten, "\n") {
		rewritten += "\n"
	}

	tmp, err := os.MkdirTemp("", "aof-manifest-*")
	if err != nil {
		return err
	}
	defer os.RemoveAll(tmp)
	if err := os.WriteFile(tmp+"/"+to+".manifest", []byte(rewritten), 0o600); err != nil {
		return err
	}
	if err := x.CopyTo(ctx, t, tmp, dir); err != nil {
		return fmt.Errorf("failed to write AOF manifest %s: %w", manifestPath, err)
	}
	return nil
}

// DataEntries lists the names inside the data directory that make up the
// backup, in the order they should be captured.
func (l Layout) DataEntries() []string {
	entries := []string{l.DBFilename}
	if name := l.AOFEntry(); name != "" {
		entries = append(entries, name)
	}
	return entries
}

// AOFEntry is the name of whatever holds the append-only log: the multi-part
// directory on Redis 7, the single file on Redis 6, or nothing when AOF is off.
func (l Layout) AOFEntry() string {
	if !l.AppendOnly {
		return ""
	}
	if l.AppendDirname != "" {
		return l.AppendDirname
	}
	return l.AppendFilename
}

// DataPaths is DataEntries resolved against the data directory.
func (l Layout) DataPaths() []string {
	entries := l.DataEntries()
	paths := make([]string, 0, len(entries))
	for _, e := range entries {
		paths = append(paths, l.Dir+"/"+e)
	}
	return paths
}
