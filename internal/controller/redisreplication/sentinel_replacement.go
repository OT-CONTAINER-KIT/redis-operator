package redisreplication

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"time"

	rrvb2 "github.com/OT-CONTAINER-KIT/redis-operator/api/redisreplication/v1beta2"
	rsvb2 "github.com/OT-CONTAINER-KIT/redis-operator/api/redissentinel/v1beta2"
	"github.com/OT-CONTAINER-KIT/redis-operator/internal/controller/common"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

const (
	replacementObservationPeriod = 30 * time.Second
	// A complete reconciliation includes rate-limited typed API reads; two
	// ordinary owners in the kind fixture were observed about 29 seconds apart.
	// Still discard stale observations after an extended controller gap.
	replacementObservationMaxGap = time.Minute
	replacementAttemptLimit      = 2
)

type replacementObservation struct {
	Binding      string    `json:"binding"`
	Since        time.Time `json:"since"`
	Seen         time.Time `json:"seen"`
	Controller   string    `json:"controller,omitempty"`
	Invalidation uint64    `json:"invalidation,omitempty"`
}

type replacementFenceKey struct {
	Owner types.NamespacedName
	UID   types.UID
}

// The public incarnation ID is not an authentication credential. A restarted
// controller cannot reuse another incarnation's stability interval. Within one
// incarnation, invalidate in memory BEFORE attempting a possibly failing API
// reset, so a later successful API read cannot resurrect contradicted evidence.
func (r *Reconciler) replacementFence(rr *rrvb2.RedisReplication, invalidate bool) (string, uint64, error) {
	r.replacementMu.Lock()
	defer r.replacementMu.Unlock()
	if r.replacementSession == "" {
		var identity [16]byte
		if _, err := rand.Read(identity[:]); err != nil {
			return "", 0, fmt.Errorf("cannot establish replacement observation incarnation")
		}
		r.replacementSession = hex.EncodeToString(identity[:])
	}
	if r.replacementInvalidations == nil {
		r.replacementInvalidations = map[replacementFenceKey]uint64{}
	}
	key := replacementFenceKey{Owner: client.ObjectKeyFromObject(rr), UID: rr.UID}
	if invalidate {
		r.replacementInvalidations[key]++
	}
	return r.replacementSession, r.replacementInvalidations[key], nil
}

func (r *Reconciler) forgetReplacementOwner(owner types.NamespacedName) {
	r.replacementMu.Lock()
	defer r.replacementMu.Unlock()
	for key := range r.replacementInvalidations {
		if key.Owner == owner {
			delete(r.replacementInvalidations, key)
		}
	}
}

type replacementAttempt struct {
	Name     string    `json:"name"`
	UID      types.UID `json:"uid"`
	ClaimUID types.UID `json:"claimUID"`
}

// The budget belongs to the RedisReplication UID and survives controller
// restarts. Success never silently replenishes it. An administrator must assess
// a repeated failure before explicitly clearing this annotation.
type replacementLedger struct {
	Version     int                     `json:"version"`
	Owner       types.UID               `json:"owner"`
	Attempts    int                     `json:"attempts"`
	Observation *replacementObservation `json:"observation,omitempty"`
	Pending     *replacementAttempt     `json:"pending,omitempty"`
}

func readReplacementLedger(rr *rrvb2.RedisReplication) (replacementLedger, error) {
	result := replacementLedger{Version: 1, Owner: rr.UID}
	encoded := rr.Annotations[common.SentinelReplacementRecoveryAnnotation]
	if encoded == "" {
		return result, nil
	}
	var fields map[string]json.RawMessage
	if err := json.Unmarshal([]byte(encoded), &fields); err != nil || fields["version"] == nil || fields["owner"] == nil || fields["attempts"] == nil {
		return replacementLedger{}, fmt.Errorf("incomplete replacement recovery history; explicit recovery is required")
	}
	result = replacementLedger{}
	if err := json.Unmarshal([]byte(encoded), &result); err != nil || result.Version != 1 || result.Owner != rr.UID || result.Attempts < 0 || result.Attempts > replacementAttemptLimit {
		return replacementLedger{}, fmt.Errorf("invalid replacement recovery history; explicit recovery is required")
	}
	if result.Pending != nil && (result.Attempts == 0 || result.Pending.Name == "" || result.Pending.UID == "" || result.Pending.ClaimUID == "") {
		return replacementLedger{}, fmt.Errorf("incomplete replacement recovery history; explicit recovery is required")
	}
	if result.Observation != nil && (len(result.Observation.Binding) != 64 || result.Observation.Since.IsZero() || result.Observation.Seen.IsZero() || result.Observation.Seen.Before(result.Observation.Since)) {
		return replacementLedger{}, fmt.Errorf("invalid replacement observation history; explicit recovery is required")
	}
	return result, nil
}

func (r *Reconciler) storeReplacementLedger(ctx context.Context, rr *rrvb2.RedisReplication, state replacementLedger) error {
	encoded, err := json.Marshal(state)
	if err != nil {
		return fmt.Errorf("cannot encode replacement recovery history")
	}
	before := rr.DeepCopy()
	if rr.Annotations == nil {
		rr.Annotations = map[string]string{}
	}
	rr.Annotations[common.SentinelReplacementRecoveryAnnotation] = string(encoded)
	// The budget reservation must win a resource-version compare-and-swap
	// before any Pod DELETE. A second controller cannot reserve another target.
	if err := r.Patch(ctx, rr, client.MergeFromWithOptions(before, client.MergeFromWithOptimisticLock{})); err != nil {
		return err
	}
	return nil
}

func activatedReplacementOwner(ctx context.Context, rr *rrvb2.RedisReplication) bool {
	return rr.UID != "" && rr.DeletionTimestamp == nil && !rr.EnableSentinel() && rr.Annotations[common.SentinelOwnershipAnnotation] == "sentinel" && rr.Annotations[common.SentinelStartupAnnotation] == "enabled" && !common.ShouldSkipReconcile(ctx, rr)
}

func (r *Reconciler) freshReplacementOwner(ctx context.Context, rr *rrvb2.RedisReplication, rs *rsvb2.RedisSentinel) (*rrvb2.RedisReplication, *rsvb2.RedisSentinel, error) {
	fresh := &rrvb2.RedisReplication{}
	if err := r.APIReader.Get(ctx, client.ObjectKeyFromObject(rr), fresh); err != nil {
		return nil, nil, err
	}
	linked := &rsvb2.RedisSentinel{}
	if err := r.APIReader.Get(ctx, client.ObjectKeyFromObject(rs), linked); err != nil {
		return nil, nil, err
	}
	if fresh.UID != rr.UID || fresh.Generation != rr.Generation || linked.UID != rs.UID || linked.Generation != rs.Generation || linked.DeletionTimestamp != nil || !activatedReplacementOwner(ctx, fresh) || linked.Spec.RedisSentinelConfig == nil || linked.Spec.RedisSentinelConfig.RedisReplicationName != fresh.Name {
		return nil, nil, fmt.Errorf("replacement ownership or configuration changed")
	}
	sole, err := r.findLinkedSentinel(ctx, fresh)
	if err != nil || sole == nil || sole.UID != linked.UID || sole.Generation != linked.Generation {
		return nil, nil, fmt.Errorf("replacement recovery requires one unchanged Sentinel authority")
	}
	return fresh, linked, nil
}

func (r *Reconciler) resetReplacementObservation(ctx context.Context, rr *rrvb2.RedisReplication) error {
	if rr.UID == "" {
		return nil
	}
	if _, _, err := r.replacementFence(rr, true); err != nil {
		return err
	}
	if r.APIReader == nil {
		return nil
	}
	fresh := &rrvb2.RedisReplication{}
	if err := r.APIReader.Get(ctx, client.ObjectKeyFromObject(rr), fresh); err != nil {
		if apierrors.IsNotFound(err) {
			return nil
		}
		return err
	}
	if fresh.UID != rr.UID || fresh.Generation != rr.Generation {
		return nil
	}
	state, err := readReplacementLedger(fresh)
	if err != nil || state.Observation == nil {
		return nil
	}
	state.Observation = nil
	if err := r.storeReplacementLedger(ctx, fresh, state); err != nil {
		return err
	}
	rr.Annotations, rr.ResourceVersion = fresh.Annotations, fresh.ResourceVersion
	return nil
}

func (r *Reconciler) reconcileStandaloneReplacement(ctx context.Context, rr *rrvb2.RedisReplication, rs *rsvb2.RedisSentinel, selected string) error {
	if r.APIReader == nil || !activatedReplacementOwner(ctx, rr) {
		return nil
	}
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	if selected == "" {
		return r.resetReplacementObservation(ctx, rr)
	}
	fresh, linked, err := r.freshReplacementOwner(ctx, rr, rs)
	if err != nil {
		return r.resetReplacementObservation(ctx, rr)
	}
	state, err := readReplacementLedger(fresh)
	if err != nil {
		log.FromContext(ctx).Info(err.Error())
		_, _, fenceErr := r.replacementFence(fresh, true)
		return fenceErr
	}
	persist := func() error {
		if err := r.storeReplacementLedger(ctx, fresh, state); err != nil {
			return err
		}
		rr.Annotations, rr.ResourceVersion = fresh.Annotations, fresh.ResourceVersion
		return nil
	}
	// Never overlap a previous gracefully terminating pod with a second
	// recovery. A replaced UID is considered only after its successor exists.
	if state.Pending != nil {
		pending, getErr := r.K8sClient.CoreV1().Pods(fresh.Namespace).Get(ctx, state.Pending.Name, metav1.GetOptions{})
		if apierrors.IsNotFound(getErr) || (getErr == nil && pending.DeletionTimestamp != nil) {
			return nil
		}
		if getErr != nil {
			return r.resetReplacementObservation(ctx, rr)
		}
		if podReady(*pending) {
			state.Pending, state.Observation = nil, nil
			return persist()
		}
	}
	view, observeErr := r.observeReplacement(ctx, fresh, linked, selected)
	if observeErr != nil || view == nil {
		if observeErr != nil {
			log.FromContext(ctx).V(1).Info("Deferring replacement recovery", "reason", observeErr.Error())
		}
		if state.Observation != nil {
			return r.resetReplacementObservation(ctx, rr)
		}
		return nil
	}
	if state.Pending != nil && (state.Pending.Name != view.Pod.Name || state.Pending.ClaimUID != view.ClaimUID) {
		return nil
	}
	if state.Pending != nil && state.Pending.UID != view.Pod.UID {
		// The first replacement started but is demonstrably stranded again.
		// Its new UID must earn a new observation period and a new budget slot.
		state.Pending, state.Observation = nil, nil
	}
	if state.Pending == nil && state.Attempts >= replacementAttemptLimit {
		log.FromContext(ctx).Info("Automatic replacement recovery budget exhausted; explicit recovery is required")
		return nil
	}
	now := r.replacementTime()
	incarnation, invalidation, err := r.replacementFence(fresh, false)
	if err != nil {
		return err
	}
	if state.Observation == nil || state.Observation.Binding != view.Binding || state.Observation.Controller != incarnation || state.Observation.Invalidation != invalidation || now.Before(state.Observation.Seen) || now.Sub(state.Observation.Seen) > replacementObservationMaxGap {
		state.Observation = &replacementObservation{Binding: view.Binding, Since: now, Seen: now, Controller: incarnation, Invalidation: invalidation}
		return persist()
	}
	if now.Sub(state.Observation.Since) < replacementObservationPeriod {
		if now.Sub(state.Observation.Seen) >= 5*time.Second {
			state.Observation.Seen = now
			return persist()
		}
		return nil
	}
	if state.Pending == nil {
		state.Attempts++
		state.Pending = &replacementAttempt{Name: view.Pod.Name, UID: view.Pod.UID, ClaimUID: view.ClaimUID}
		if err := persist(); err != nil {
			return err
		}
	}
	// Re-read both identities and all live topology after reserving the budget.
	// A controller restart may resume this exact UID, never an inferred successor.
	finalOwner, finalSentinel, err := r.freshReplacementOwner(ctx, fresh, linked)
	if err != nil {
		return r.resetReplacementObservation(ctx, rr)
	}
	finalState, err := readReplacementLedger(finalOwner)
	if err != nil || finalState.Pending == nil || *finalState.Pending != *state.Pending || finalState.Observation == nil || *finalState.Observation != *state.Observation || finalState.Attempts != state.Attempts {
		_, _, fenceErr := r.replacementFence(rr, true)
		return fenceErr
	}
	finalView, err := r.observeReplacement(ctx, finalOwner, finalSentinel, selected)
	if err != nil || finalView == nil || finalView.Binding != view.Binding || finalView.Pod.UID != state.Pending.UID || finalView.ClaimUID != state.Pending.ClaimUID {
		// Require a new stable interval after contradictory revalidation.
		return r.resetReplacementObservation(ctx, rr)
	}
	uid, version := finalView.Pod.UID, finalView.Pod.ResourceVersion
	if version == "" {
		return nil
	}
	// The network observations may have taken several seconds. Recheck the
	// ownership and reservation after them, immediately before Pod DELETE.
	actionOwner, _, err := r.freshReplacementOwner(ctx, finalOwner, finalSentinel)
	if err != nil {
		return r.resetReplacementObservation(ctx, rr)
	}
	actionState, err := readReplacementLedger(actionOwner)
	if err != nil || actionState.Pending == nil || *actionState.Pending != *finalState.Pending || actionState.Observation == nil || *actionState.Observation != *finalState.Observation || actionState.Attempts != finalState.Attempts {
		_, _, fenceErr := r.replacementFence(rr, true)
		return fenceErr
	}
	currentIncarnation, currentInvalidation, err := r.replacementFence(actionOwner, false)
	if err != nil {
		return err
	}
	if currentIncarnation != incarnation || currentInvalidation != invalidation {
		return nil
	}
	rr.Annotations, rr.ResourceVersion = actionOwner.Annotations, actionOwner.ResourceVersion
	// No zero-grace override, force deletion, role command or PVC mutation.
	err = r.K8sClient.CoreV1().Pods(finalOwner.Namespace).Delete(ctx, finalView.Pod.Name, metav1.DeleteOptions{Preconditions: &metav1.Preconditions{UID: &uid, ResourceVersion: &version}})
	if apierrors.IsNotFound(err) || apierrors.IsConflict(err) {
		return nil
	}
	if err != nil {
		return err
	}
	log.FromContext(ctx).Info("Recreating an undiscovered stranded replacement through protected startup", "pod", finalView.Pod.Name, "attempt", finalState.Attempts)
	return nil
}
