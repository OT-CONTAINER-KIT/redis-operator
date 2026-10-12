package redisreplication

import (
	"context"
	"errors"
	"fmt"
	"net"
	"strconv"
	"strings"
	"time"

	rrvb2 "github.com/OT-CONTAINER-KIT/redis-operator/api/redisreplication/v1beta2"
	rsvb2 "github.com/OT-CONTAINER-KIT/redis-operator/api/redissentinel/v1beta2"
	redishealer "github.com/OT-CONTAINER-KIT/redis-operator/internal/controller/common/redis"
	intctrlutil "github.com/OT-CONTAINER-KIT/redis-operator/internal/controllerutil"
	"github.com/OT-CONTAINER-KIT/redis-operator/internal/k8sutils"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

const (
	// Defaults of the RedisSentinel CRD, used when the spec leaves them empty.
	defaultSentinelDownAfter       = 5 * time.Second
	defaultSentinelFailoverTimeout = 10 * time.Second
	// sentinelFailoverMargin covers the Sentinel leader election, the time
	// until the operator observes the new master and clock skew between the
	// first refusal and the failover.
	sentinelFailoverMargin = 30 * time.Second

	emptyMasterRequeue    = 60 * time.Second
	promotedMasterRequeue = 10 * time.Second
)

// emptyMasterError is returned by ensureMasterHoldsData when the master to
// replicate from holds no keys while other pods hold data or could not be
// probed: replicating from it would wipe their data.
type emptyMasterError struct {
	master          string
	podsHoldingData []string
	podsUnknown     []string
}

func (e *emptyMasterError) Error() string {
	return fmt.Sprintf("master %s holds no keys while pods %v hold data and %v could not be probed, refusing to replicate from it; "+
		"to recover, run REPLICAOF NO ONE on a pod that holds data, then REPLICAOF <its address> 6379 on %s, and the operator will attach the rest",
		e.master, e.podsHoldingData, e.podsUnknown, e.master)
}

// emptyMasterObservation remembers since when a replication has been refused
// because of a given empty master pod.
type emptyMasterObservation struct {
	master string
	podUID types.UID
	since  time.Time
}

// recoverFromEmptyMaster handles a refusal to replicate data-holding pods from
// an empty master. The data-holding pods are usually replicas left behind by a
// master that restarted without persistence. Sentinel would normally promote
// one of them, but it cannot when it has lost its own state too, for example
// when its pods restarted together with the master, and the empty master is
// then left alone with no replicas. In that case the operator promotes the
// best data-holding pod itself and attaches every other pod to it.
//
// The promotion only happens when every pod's key count is known, and only
// once no Sentinel failover can be expected: either no Sentinel watches the
// replication, too few Sentinels still know the lost master and its replicas
// to elect a new one, or they have had their full failover window to do it.
// An inline spec.sentinel is left to its own failover.
//
// handled is false when the refusal stands and the caller should report it.
func (r *Reconciler) recoverFromEmptyMaster(ctx context.Context, instance *rrvb2.RedisReplication, topology k8sutils.RedisReplicationTopology, linkErr error) (result ctrl.Result, handled bool) {
	var refusal *emptyMasterError
	if !errors.As(linkErr, &refusal) || len(refusal.podsUnknown) > 0 || len(refusal.podsHoldingData) == 0 {
		return ctrl.Result{}, false
	}
	if instance.EnableSentinel() {
		return ctrl.Result{}, false
	}
	logger := log.FromContext(ctx).WithValues("emptyMaster", refusal.master)

	// The refusal may have looked at the masters only. Every pod has to be
	// accounted for before one is promoted over the others.
	pods := append(append([]string{}, topology.Masters...), topology.Slaves...)
	keyCounts := r.Checker.GetReplicationKeyCounts(ctx, instance, pods)
	var podsHoldingData []string
	for _, pod := range pods {
		keys, ok := keyCounts[pod]
		if !ok {
			logger.Info("Not promoting a data-holding pod: the key count of a pod is unknown", "pod", pod)
			return ctrl.Result{}, false
		}
		if keys > 0 && pod != refusal.master {
			podsHoldingData = append(podsHoldingData, pod)
		}
	}
	if keyCounts[refusal.master] > 0 || len(podsHoldingData) == 0 {
		return ctrl.Result{}, false
	}

	failoverExpected, window, err := r.sentinelFailover(ctx, instance, refusal.master)
	if err != nil {
		logger.Error(err, "Not promoting a data-holding pod: could not tell whether Sentinel will fail over")
		return ctrl.Result{}, false
	}
	if failoverExpected {
		if remaining := r.sentinelFailoverRemaining(ctx, instance, refusal.master, window); remaining > 0 {
			logger.Info("Master holds no keys while other pods hold data, waiting for Sentinel to fail over to one of them",
				"podsHoldingData", podsHoldingData, "remaining", remaining.String())
			result, _ := intctrlutil.RequeueAfter(ctx, min(remaining, emptyMasterRequeue), "")
			return result, true
		}
		logger.Info("Sentinel did not fail over within its failover window", "window", window.String())
	}

	candidate := bestDataHoldingPod(podsHoldingData, keyCounts, r.replicationStates(ctx, instance, podsHoldingData))
	if candidate == "" {
		logger.Info("Not promoting a data-holding pod: the replication state of a pod could not be read",
			"podsHoldingData", podsHoldingData)
		return ctrl.Result{}, false
	}

	logger.Info("Master holds no keys while other pods hold data and no Sentinel failover is expected, promoting a data-holding pod",
		"promoted", candidate, "podsHoldingData", podsHoldingData)
	if err := r.promoteReplicationPod(ctx, instance, candidate); err != nil {
		logger.Error(err, "Failed to promote the data-holding pod", "pod", candidate)
		return ctrl.Result{}, false
	}
	r.forgetEmptyMaster(instance)
	// Record the new master before attaching the rest, so the Sentinel
	// controller's readiness fallback stops pointing at the empty pod even if
	// attaching fails part way.
	if err := r.UpdateRedisReplicationMaster(ctx, instance, candidate, false); err != nil {
		logger.Error(err, "Failed to record the promoted pod as master", "pod", candidate)
	}
	// Attach the other data-holding pods first and one at a time: once any pod
	// replicates from the candidate, later reconciles identify the candidate as
	// the master by its attached replica, even if attaching the rest fails.
	var linkErrs []error
	for _, pod := range attachOrder(pods, podsHoldingData, candidate) {
		if err := r.linkReplicationPods(ctx, instance, []string{candidate, pod}, candidate); err != nil {
			linkErrs = append(linkErrs, fmt.Errorf("attach %s: %w", pod, err))
		}
	}
	if err := errors.Join(linkErrs...); err != nil {
		logger.Error(err, "Failed to attach some pods to the promoted master", "master", candidate)
	}
	result, _ = intctrlutil.RequeueAfter(ctx, promotedMasterRequeue, "")
	return result, true
}

// attachOrder lists every pod except master, the pods holding data first.
func attachOrder(pods, podsHoldingData []string, master string) []string {
	ordered := make([]string, 0, len(pods))
	holding := make(map[string]bool, len(podsHoldingData))
	for _, pod := range podsHoldingData {
		holding[pod] = true
		if pod != master {
			ordered = append(ordered, pod)
		}
	}
	for _, pod := range pods {
		if !holding[pod] && pod != master {
			ordered = append(ordered, pod)
		}
	}
	return ordered
}

// bestDataHoldingPod picks the pod to promote. Pods that replicated the same
// stream share its replication ID: the largest such group most likely carries
// on the lost master's dataset, and within it the highest offset has seen the
// most writes. More keys and then the lowest ordinal break the remaining ties,
// so repeated reconciles pick the same pod. It returns "" when the replication
// state of any pod is unknown.
func bestDataHoldingPod(pods []string, keyCounts map[string]int64, states map[string]k8sutils.RedisReplicationState) string {
	streamSize := make(map[string]int, len(pods))
	for _, pod := range pods {
		state, ok := states[pod]
		if !ok {
			return ""
		}
		streamSize[state.ReplID]++
	}
	better := func(a, b string) bool {
		sa, sb := states[a], states[b]
		if streamSize[sa.ReplID] != streamSize[sb.ReplID] {
			return streamSize[sa.ReplID] > streamSize[sb.ReplID]
		}
		if sa.Offset != sb.Offset {
			return sa.Offset > sb.Offset
		}
		if keyCounts[a] != keyCounts[b] {
			return keyCounts[a] > keyCounts[b]
		}
		return podOrdinal(a) < podOrdinal(b)
	}
	var best string
	for _, pod := range pods {
		if best == "" || better(pod, best) {
			best = pod
		}
	}
	return best
}

func podOrdinal(podName string) int {
	i := strings.LastIndex(podName, "-")
	if i < 0 {
		return -1
	}
	ordinal, err := strconv.Atoi(podName[i+1:])
	if err != nil {
		return -1
	}
	return ordinal
}

// sentinelFailoverRemaining tells how much longer to wait for a Sentinel
// failover away from emptyMaster. The wait starts at the first refusal seen
// for that pod; a recreated pod starts a new one.
func (r *Reconciler) sentinelFailoverRemaining(ctx context.Context, instance *rrvb2.RedisReplication, emptyMaster string, window time.Duration) time.Duration {
	var podUID types.UID
	if pod, err := r.K8sClient.CoreV1().Pods(instance.Namespace).Get(ctx, emptyMaster, metav1.GetOptions{}); err == nil {
		podUID = pod.UID
	}
	now := r.now()

	r.emptyMasterMu.Lock()
	defer r.emptyMasterMu.Unlock()
	if r.emptyMasterSince == nil {
		r.emptyMasterSince = make(map[types.UID]emptyMasterObservation)
	}
	observation, ok := r.emptyMasterSince[instance.UID]
	if !ok || observation.master != emptyMaster || observation.podUID != podUID {
		observation = emptyMasterObservation{master: emptyMaster, podUID: podUID, since: now}
		r.emptyMasterSince[instance.UID] = observation
	}
	return window - now.Sub(observation.since)
}

func (r *Reconciler) forgetEmptyMaster(instance *rrvb2.RedisReplication) {
	r.emptyMasterMu.Lock()
	defer r.emptyMasterMu.Unlock()
	delete(r.emptyMasterSince, instance.UID)
}

func (r *Reconciler) sentinelFailover(ctx context.Context, instance *rrvb2.RedisReplication, emptyMaster string) (bool, time.Duration, error) {
	if r.SentinelFailover != nil {
		return r.SentinelFailover(ctx, instance, emptyMaster)
	}
	return r.redisSentinelFailover(ctx, instance, emptyMaster)
}

// redisSentinelFailover asks every RedisSentinel that watches the replication
// whether its Sentinels can still fail over away from emptyMaster. A failover
// needs both the quorum to agree the master is down and a majority of the
// Sentinels to elect a leader, so it is expected only while that many
// Sentinels still monitor a master other than emptyMaster and know replicas
// to promote. A Sentinel that restarted without its state monitors no master,
// or the 0.0.0.0 placeholder written at startup, and knows no replicas.
func (r *Reconciler) redisSentinelFailover(ctx context.Context, instance *rrvb2.RedisReplication, emptyMaster string) (bool, time.Duration, error) {
	if r.Client == nil {
		return false, 0, errors.New("no client configured, cannot list RedisSentinels")
	}
	var sentinels rsvb2.RedisSentinelList
	if err := r.List(ctx, &sentinels, client.InNamespace(instance.Namespace)); err != nil {
		return false, 0, fmt.Errorf("list RedisSentinels: %w", err)
	}

	var expected bool
	var window time.Duration
	var emptyMasterAddresses map[string]bool
	for i := range sentinels.Items {
		rs := &sentinels.Items[i]
		if rs.Spec.RedisSentinelConfig == nil || rs.Spec.RedisSentinelConfig.RedisReplicationName != instance.Name {
			continue
		}
		if emptyMasterAddresses == nil {
			pod, err := r.K8sClient.CoreV1().Pods(instance.Namespace).Get(ctx, emptyMaster, metav1.GetOptions{})
			if err != nil {
				return false, 0, fmt.Errorf("get pod %s: %w", emptyMaster, err)
			}
			emptyMasterAddresses = map[string]bool{replicationPodHostname(instance, emptyMaster): true}
			if pod.Status.PodIP != "" {
				emptyMasterAddresses[pod.Status.PodIP] = true
			}
		}
		reports, err := r.Checker.GetSentinelMasters(ctx, rs)
		if err != nil {
			return false, 0, fmt.Errorf("ask the Sentinels of RedisSentinel %s: %w", rs.Name, err)
		}
		var able []string
		for _, report := range reports {
			if sentinelCanFailOver(report, emptyMasterAddresses) {
				able = append(able, report.Pod)
			}
		}
		needed := sentinelsNeededForFailover(rs)
		log.FromContext(ctx).V(1).Info("Counted the Sentinels that can fail over",
			"redisSentinel", rs.Name, "able", able, "needed", needed)
		if len(able) >= needed {
			expected = true
			window = max(window, sentinelFailoverWindow(rs))
		}
	}
	return expected, window, nil
}

func sentinelCanFailOver(report redishealer.SentinelMasterReport, emptyMasterAddresses map[string]bool) bool {
	if report.Err != nil {
		// It may still hold its state; assume the worst.
		return true
	}
	if report.Master == nil || report.Master.Slaves == 0 {
		return false
	}
	host := sentinelAddressHost(report.Master.Address)
	if host == "" || emptyMasterAddresses[host] {
		return false
	}
	if ip := net.ParseIP(host); ip != nil && ip.IsUnspecified() {
		return false
	}
	return true
}

func sentinelsNeededForFailover(rs *rsvb2.RedisSentinel) int {
	var size int
	if rs.Spec.Size != nil {
		size = int(*rs.Spec.Size)
	}
	majority := size/2 + 1
	quorum, err := strconv.Atoi(rs.Spec.RedisSentinelConfig.Quorum)
	if err != nil || quorum < 1 {
		quorum = majority
	}
	return max(quorum, majority)
}

func sentinelFailoverWindow(rs *rsvb2.RedisSentinel) time.Duration {
	cfg := rs.Spec.RedisSentinelConfig
	return parseMilliseconds(cfg.DownAfterMilliseconds, defaultSentinelDownAfter) +
		parseMilliseconds(cfg.FailoverTimeout, defaultSentinelFailoverTimeout) +
		sentinelFailoverMargin
}

func parseMilliseconds(value string, fallback time.Duration) time.Duration {
	ms, err := strconv.ParseInt(value, 10, 64)
	if err != nil || ms < 0 {
		return fallback
	}
	return time.Duration(ms) * time.Millisecond
}

func (r *Reconciler) replicationStates(ctx context.Context, instance *rrvb2.RedisReplication, pods []string) map[string]k8sutils.RedisReplicationState {
	if r.ReplicationStates != nil {
		return r.ReplicationStates(ctx, r.K8sClient, instance, pods)
	}
	return k8sutils.GetRedisReplicationStates(ctx, r.K8sClient, instance, pods)
}

func (r *Reconciler) promoteReplicationPod(ctx context.Context, instance *rrvb2.RedisReplication, pod string) error {
	if r.PromoteReplicationPod != nil {
		return r.PromoteReplicationPod(ctx, r.K8sClient, instance, pod)
	}
	return k8sutils.PromoteRedisReplicationPod(ctx, r.K8sClient, instance, pod)
}

func (r *Reconciler) now() time.Time {
	if r.Now != nil {
		return r.Now()
	}
	return time.Now()
}
