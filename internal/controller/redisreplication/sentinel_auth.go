package redisreplication

import (
	"context"
	"fmt"

	rrvb2 "github.com/OT-CONTAINER-KIT/redis-operator/api/redisreplication/v1beta2"
	rsvb2 "github.com/OT-CONTAINER-KIT/redis-operator/api/redissentinel/v1beta2"
	"github.com/OT-CONTAINER-KIT/redis-operator/internal/service/redis"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
)

type sentinelAuthKey struct {
	owner                               types.NamespacedName
	replicationUID, sentinelUID, podUID types.UID
	group                               string
}

type cachedSentinelAuth struct{ password string }

func (cachedSentinelAuth) String() string   { return "<redacted>" }
func (cachedSentinelAuth) GoString() string { return "<redacted>" }

type observedSentinelAuth struct {
	pod     corev1.Pod
	service redis.Service
	report  redis.SentinelMasterState
}

func authKey(rr *rrvb2.RedisReplication, rs *rsvb2.RedisSentinel, pod corev1.Pod, group string) sentinelAuthKey {
	return sentinelAuthKey{owner: types.NamespacedName{Namespace: rr.Namespace, Name: rr.Name}, replicationUID: rr.UID, sentinelUID: rs.UID, podUID: pod.UID, group: group}
}

func (r *Reconciler) forgetSentinelAuthOwner(owner types.NamespacedName) {
	r.sentinelAuthMu.Lock()
	defer r.sentinelAuthMu.Unlock()
	for key := range r.sentinelAuth {
		if key.owner == owner {
			delete(r.sentinelAuth, key)
		}
	}
}

func (r *Reconciler) forgetSentinelAuth(key sentinelAuthKey) {
	r.sentinelAuthMu.Lock()
	defer r.sentinelAuthMu.Unlock()
	delete(r.sentinelAuth, key)
}

func (r *Reconciler) pruneSentinelAuth(rr *rrvb2.RedisReplication, rs *rsvb2.RedisSentinel, pods []corev1.Pod, group string) {
	current := map[types.UID]bool{}
	for _, pod := range pods {
		current[pod.UID] = true
	}
	owner := types.NamespacedName{Namespace: rr.Namespace, Name: rr.Name}
	r.sentinelAuthMu.Lock()
	defer r.sentinelAuthMu.Unlock()
	for key := range r.sentinelAuth {
		if key.owner == owner && (key.replicationUID != rr.UID || key.sentinelUID != rs.UID || key.group != group || !current[key.podUID]) {
			delete(r.sentinelAuth, key)
		}
	}
}

func (r *Reconciler) reconcileStandaloneAuth(ctx context.Context, rr *rrvb2.RedisReplication, rs *rsvb2.RedisSentinel, group, password string, peers []observedSentinelAuth) error {
	// A fresh no-auth monitor has no auth-pass to set. In Redis even setting
	// auth-pass to the same value drops the master's monitoring connections.
	// Authentication removal/rotation is outside the qualified static profile.
	if password == "" {
		r.forgetSentinelAuthOwner(types.NamespacedName{Namespace: rr.Namespace, Name: rr.Name})
		return nil
	}
	for _, peer := range peers {
		if hasFlag(peer.report, "failover_in_progress") {
			return nil
		}
	}
	for _, peer := range peers {
		if peer.pod.UID == "" {
			return fmt.Errorf("cannot configure Sentinel authentication without pod identity")
		}
		key := authKey(rr, rs, peer.pod, group)
		r.sentinelAuthMu.Lock()
		cached, known := r.sentinelAuth[key]
		r.sentinelAuthMu.Unlock()
		if known && cached.password == password {
			continue
		}
		// Apply at most once per credential value and process/pod identity.
		// Failed or partially initialized authentication remains retryable.
		if err := peer.service.SentinelSet(ctx, group, "auth-pass", password); err != nil {
			return fmt.Errorf("cannot configure Sentinel authentication")
		}
		r.sentinelAuthMu.Lock()
		if r.sentinelAuth == nil {
			r.sentinelAuth = map[sentinelAuthKey]cachedSentinelAuth{}
		}
		r.sentinelAuth[key] = cachedSentinelAuth{password: password}
		r.sentinelAuthMu.Unlock()
	}
	return nil
}
