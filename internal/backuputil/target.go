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

// Package backuputil holds the pieces the RedisBackup and RedisRestore
// controllers share: resolving which workload a CR actually points at,
// execing into it, and streaming files in and out of it.
package backuputil

import (
	"context"
	"fmt"

	rvb2 "github.com/OT-CONTAINER-KIT/redis-operator/api/redis/v1beta2"
	rcvb2 "github.com/OT-CONTAINER-KIT/redis-operator/api/rediscluster/v1beta2"
	rrvb2 "github.com/OT-CONTAINER-KIT/redis-operator/api/redisreplication/v1beta2"
	"github.com/OT-CONTAINER-KIT/redis-operator/internal/controller/common"
	appsv1 "k8s.io/api/apps/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// TargetKind is the kind of Redis workload a backup or restore points at.
type TargetKind string

const (
	KindRedis            TargetKind = "Redis"
	KindRedisReplication TargetKind = "RedisReplication"
	KindRedisCluster     TargetKind = "RedisCluster"

	// KindRedisSentinel is never a backup target — it holds no data — but a
	// restore of the RedisReplication it watches has to suspend it.
	KindRedisSentinel TargetKind = "RedisSentinel"
)

// SkipReconcileAnnotation returns the annotation that suspends the controller
// owning this kind, so a restore can scale the StatefulSet without the owning
// controller reconciling the replica count straight back.
//
// The annotation keys mirror internal/controller/common/skip_reconcile.go.
func SkipReconcileAnnotation(kind TargetKind) (string, bool) {
	switch kind {
	case KindRedis:
		return common.RedisSkipReconcileAnnotation, true
	case KindRedisReplication:
		return common.RedisReplicationSkipReconcileAnnotation, true
	case KindRedisCluster:
		return common.RedisClusterSkipReconcileAnnotation, true
	case KindRedisSentinel:
		return common.RedisSentinelSkipReconcileAnnotation, true
	default:
		return "", false
	}
}

// RestoreOwnerAnnotation names the RedisRestore that set a target's
// skip-reconcile annotation, so a later reconcile can tell its own interrupted
// attempt from a pause a user put there or a different restore's lock.
const RestoreOwnerAnnotation = "redisrestore.redis.redis.opstreelabs.in/restore-owner"

// Target is a single data-bearing Redis pod, resolved from a CR reference.
//
// The names here are derived the same way the operator derives them when it
// creates the workload, rather than assumed:
//
//	Redis "foo"            -> sts foo,        pod foo-0,        container foo
//	RedisReplication "foo" -> sts foo,        pod foo-<i>,      container foo
//	RedisCluster "foo"     -> sts foo-leader, pod foo-leader-i, container foo-leader
//
// See internal/k8sutils/redis-standalone.go, redis-replication.go and
// redis-cluster.go for the authoritative naming.
type Target struct {
	Kind        TargetKind
	Namespace   string
	StatefulSet string
	Pod         string
	Container   string
	Shard       int
	OwnerName   string

	// Primary marks a pod that holds a distinct copy of the data: the single
	// standalone pod, the replication master, or one cluster leader per shard.
	// A backup reads only from primaries.
	//
	// Non-primary pods (replication replicas, cluster followers) hold a copy
	// that is derived by replication. A restore must not write the archive
	// into them, but it must not leave their old data in place either — they
	// would come back holding pre-restore data and can win a master election.
	Primary bool

	// Ordinal is the pod's index within its StatefulSet.
	Ordinal int32
}

// Primaries returns only the targets a backup should read from.
func Primaries(targets []Target) []Target {
	out := make([]Target, 0, len(targets))
	for _, t := range targets {
		if t.Primary {
			out = append(out, t)
		}
	}
	return out
}

// Secondaries returns the pods that derive their data by replication.
func Secondaries(targets []Target) []Target {
	out := make([]Target, 0, len(targets))
	for _, t := range targets {
		if !t.Primary {
			out = append(out, t)
		}
	}
	return out
}

// StatefulSetNames lists every distinct StatefulSet the targets live in.
func StatefulSetNames(targets []Target) []string {
	seen := map[string]bool{}
	var out []string
	for _, t := range targets {
		if !seen[t.StatefulSet] {
			seen[t.StatefulSet] = true
			out = append(out, t.StatefulSet)
		}
	}
	return out
}

// ResolveKind reports the kind of the referenced Redis CR by looking it up.
// An explicit kind short-circuits the lookup; an empty one is discovered.
func ResolveKind(ctx context.Context, c client.Client, namespace, name string, explicit TargetKind) (TargetKind, error) {
	if explicit != "" {
		return explicit, nil
	}
	key := types.NamespacedName{Namespace: namespace, Name: name}

	redis := &rvb2.Redis{}
	if err := c.Get(ctx, key, redis); err == nil {
		return KindRedis, nil
	} else if !apierrors.IsNotFound(err) {
		return "", fmt.Errorf("failed to look up Redis %q: %w", name, err)
	}

	repl := &rrvb2.RedisReplication{}
	if err := c.Get(ctx, key, repl); err == nil {
		return KindRedisReplication, nil
	} else if !apierrors.IsNotFound(err) {
		return "", fmt.Errorf("failed to look up RedisReplication %q: %w", name, err)
	}

	cluster := &rcvb2.RedisCluster{}
	if err := c.Get(ctx, key, cluster); err == nil {
		return KindRedisCluster, nil
	} else if !apierrors.IsNotFound(err) {
		return "", fmt.Errorf("failed to look up RedisCluster %q: %w", name, err)
	}

	return "", fmt.Errorf("no Redis, RedisReplication or RedisCluster named %q found in namespace %q", name, namespace)
}

// Resolve returns every data-bearing target for the referenced CR.
//
// It reads the live StatefulSet rather than the CR's desired replica count, so
// a scaled workload is covered correctly.
func Resolve(ctx context.Context, c client.Client, namespace, name string, kind TargetKind) ([]Target, error) {
	switch kind {
	case KindRedis:
		if _, err := getSts(ctx, c, namespace, name); err != nil {
			return nil, err
		}
		return []Target{{
			Kind: kind, Namespace: namespace, OwnerName: name,
			StatefulSet: name, Pod: name + "-0", Container: name,
			Shard: 0, Ordinal: 0, Primary: true,
		}}, nil

	case KindRedisReplication:
		// Every pod in the replication StatefulSet holds the same dataset, one
		// as master and the rest as replicas. All of them are returned: the
		// backup reads from whichever is really the master, and the restore
		// has to account for every pod's PVC, not just pod-0's.
		sts, err := getSts(ctx, c, namespace, name)
		if err != nil {
			return nil, err
		}
		n := replicas(sts)
		if n == 0 {
			return nil, fmt.Errorf("statefulset %q has no replicas", name)
		}
		targets := make([]Target, 0, n)
		for i := range n {
			targets = append(targets, Target{
				Kind: kind, Namespace: namespace, OwnerName: name,
				StatefulSet: name,
				Pod:         fmt.Sprintf("%s-%d", name, i),
				Container:   name,
				Shard:       0, Ordinal: i,
				// Provisional: the caller resolves the real master and
				// re-marks it. Pod-0 is only the default.
				Primary: i == 0,
			})
		}
		return targets, nil

	case KindRedisCluster:
		// Each leader owns a distinct slice of the 16384 slots, so every
		// leader is a separate, non-redundant shard of the backup. Followers
		// mirror a leader; they carry no unique data but they do carry a PVC
		// that a restore must not leave holding pre-restore data.
		leaderSts := name + "-leader"
		leaders, err := getSts(ctx, c, namespace, leaderSts)
		if err != nil {
			return nil, err
		}
		n := replicas(leaders)
		if n == 0 {
			return nil, fmt.Errorf("statefulset %q has no replicas to back up", leaderSts)
		}
		targets := make([]Target, 0, n*2)
		for i := range n {
			targets = append(targets, Target{
				Kind: kind, Namespace: namespace, OwnerName: name,
				StatefulSet: leaderSts,
				Pod:         fmt.Sprintf("%s-%d", leaderSts, i),
				Container:   leaderSts,
				Shard:       int(i), Ordinal: i, Primary: true,
			})
		}

		followerSts := name + "-follower"
		if followers, ferr := getSts(ctx, c, namespace, followerSts); ferr == nil {
			fn := replicas(followers)
			for i := range fn {
				targets = append(targets, Target{
					Kind: kind, Namespace: namespace, OwnerName: name,
					StatefulSet: followerSts,
					Pod:         fmt.Sprintf("%s-%d", followerSts, i),
					Container:   followerSts,
					Shard:       int(i), Ordinal: i, Primary: false,
				})
			}
		}
		return targets, nil

	default:
		return nil, fmt.Errorf("unsupported target kind %q (supported: Redis, RedisReplication, RedisCluster)", kind)
	}
}

// ResolveReplicationPrimary re-marks the replication target that is actually
// the master, so a backup is never taken from a replica whose link to the
// master has been down (which would produce a valid but silently stale
// archive).
//
// If more than one pod claims to be master — a split brain after a failed
// failover — the one with replicas still attached is the live side and wins;
// a tie falls back to the lowest ordinal, which is the same tie-break the
// replication controller uses.
func ResolveReplicationPrimary(targets []Target, probe func(Target) (role string, connectedSlaves int, err error)) ([]Target, error) {
	best := -1
	bestSlaves := -1
	masters := 0
	var firstErr error
	for i, t := range targets {
		role, slaves, err := probe(t)
		if err != nil {
			if firstErr == nil {
				firstErr = err
			}
			continue
		}
		if role != "master" {
			continue
		}
		masters++
		if slaves > bestSlaves {
			best, bestSlaves = i, slaves
		}
	}
	if best < 0 {
		if firstErr != nil {
			return nil, fmt.Errorf("could not identify the replication master: %w", firstErr)
		}
		return nil, fmt.Errorf("no pod in the replication reports role:master; refusing to back up a replica")
	}
	if masters > 1 && bestSlaves == 0 {
		return nil, fmt.Errorf("%d pods report role:master and none has replicas attached; the replication is split-brained, refusing to guess which side is live", masters)
	}
	out := make([]Target, len(targets))
	copy(out, targets)
	for i := range out {
		out[i].Primary = i == best
	}
	return out, nil
}

func getSts(ctx context.Context, c client.Client, namespace, name string) (*appsv1.StatefulSet, error) {
	sts := &appsv1.StatefulSet{}
	if err := c.Get(ctx, types.NamespacedName{Namespace: namespace, Name: name}, sts); err != nil {
		if apierrors.IsNotFound(err) {
			return nil, fmt.Errorf("statefulset %q not found in namespace %q", name, namespace)
		}
		return nil, fmt.Errorf("failed to get statefulset %q: %w", name, err)
	}
	return sts, nil
}

func replicas(sts *appsv1.StatefulSet) int32 {
	if sts.Spec.Replicas == nil {
		return 1
	}
	return *sts.Spec.Replicas
}
