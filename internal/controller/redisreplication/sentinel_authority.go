package redisreplication

import (
	"context"
	"errors"
	"fmt"
	"net"
	"strconv"
	"strings"
	"syscall"
	"time"

	rrvb2 "github.com/OT-CONTAINER-KIT/redis-operator/api/redisreplication/v1beta2"
	rsvb2 "github.com/OT-CONTAINER-KIT/redis-operator/api/redissentinel/v1beta2"
	"github.com/OT-CONTAINER-KIT/redis-operator/internal/controller/common"
	redischecker "github.com/OT-CONTAINER-KIT/redis-operator/internal/controller/common/redis"
	"github.com/OT-CONTAINER-KIT/redis-operator/internal/k8sutils"
	"github.com/OT-CONTAINER-KIT/redis-operator/internal/service/redis"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func (r *Reconciler) findLinkedSentinel(ctx context.Context, rr *rrvb2.RedisReplication) (*rsvb2.RedisSentinel, error) {
	list := &rsvb2.RedisSentinelList{}
	var reader client.Reader = r.Client
	if r.APIReader != nil {
		reader = r.APIReader
	}
	if err := reader.List(ctx, list, client.InNamespace(rr.Namespace)); err != nil {
		return nil, err
	}
	var linked *rsvb2.RedisSentinel
	for i := range list.Items {
		item := &list.Items[i]
		if item.Spec.RedisSentinelConfig == nil || item.Spec.RedisSentinelConfig.RedisReplicationName != rr.Name {
			continue
		}
		if linked != nil || rr.EnableSentinel() {
			return nil, fmt.Errorf("multiple Sentinel authorities monitor RedisReplication %s", rr.Name)
		}
		linked = item
	}
	return linked, nil
}

// Initial replication must be established before standalone Sentinel can start.
// Once status or the Sentinel StatefulSet exists, never fall back to bootstrap.
func (r *Reconciler) linkedSentinel(ctx context.Context, rr *rrvb2.RedisReplication) (*rsvb2.RedisSentinel, error) {
	if r.APIReader != nil {
		fresh := &rrvb2.RedisReplication{}
		if err := r.APIReader.Get(ctx, client.ObjectKeyFromObject(rr), fresh); err != nil {
			return nil, err
		}
		if fresh.UID != rr.UID || fresh.Generation != rr.Generation {
			return nil, fmt.Errorf("replication identity or specification changed before authority selection")
		}
		rr.Annotations = fresh.Annotations
		rr.Status = fresh.Status
		rr.ResourceVersion = fresh.ResourceVersion
	}
	linked, err := r.findLinkedSentinel(ctx, rr)
	if err != nil {
		return nil, err
	}
	if linked == nil && (rr.Annotations[common.SentinelOwnershipAnnotation] == "sentinel" || rr.Annotations[common.SentinelStartupAnnotation] == "enabled") {
		return nil, fmt.Errorf("activated standalone Sentinel authority is absent")
	}
	if linked == nil {
		return linked, err
	}
	// Without a direct API read, this method may observe authority but may
	// never grant bootstrap mutation from potentially stale cached metadata.
	if r.APIReader == nil {
		return linked, nil
	}
	if rr.Status.MasterNode == "" && rr.Status.ConnectionInfo == nil && rr.Annotations[common.SentinelOwnershipAnnotation] != "sentinel" && rr.Annotations[common.SentinelStartupAnnotation] != "enabled" {
		_, err := r.K8sClient.AppsV1().StatefulSets(rr.Namespace).Get(ctx, linked.GetStatefulSetName(), metav1.GetOptions{})
		if apierrors.IsNotFound(err) {
			if rr.Annotations[common.SentinelOwnershipAnnotation] == "" {
				if err := r.recordSentinelOwnership(ctx, rr, "bootstrap"); err != nil {
					return nil, err
				}
			}
			return nil, nil
		}
		if err != nil {
			return nil, err
		}
	}
	if rr.Annotations[common.SentinelOwnershipAnnotation] == "bootstrap" && rr.Annotations[common.SentinelStartupAnnotation] != "enabled" && r.bootstrapMonitorsAbsent(ctx, linked) {
		topology, err := r.redisReplicationTopology(ctx, rr)
		if err != nil {
			return nil, err
		}
		// This reconciler is the sole creator of standalone monitors. Before
		// any exist, Sentinel cannot run an election for this group. Retain
		// bootstrap role ownership until the complete initial topology is ready.
		if topology.Complete() && (len(topology.Masters) != 1 || (rr.Spec.Size != nil && *rr.Spec.Size > 1 && r.redisReplicationRealMaster(ctx, rr, topology.Masters) == "")) {
			return nil, nil
		}
	}
	return linked, nil
}

func (r *Reconciler) bootstrapMonitorsAbsent(ctx context.Context, rs *rsvb2.RedisSentinel) bool {
	if rs.Spec.Size == nil || *rs.Spec.Size < 1 || rs.Spec.RedisSentinelConfig == nil || rs.Spec.TLS != nil {
		return false
	}
	sts, err := r.K8sClient.AppsV1().StatefulSets(rs.Namespace).Get(ctx, rs.GetStatefulSetName(), metav1.GetOptions{})
	if err != nil {
		return false
	}
	pods, err := r.K8sClient.CoreV1().Pods(rs.Namespace).List(ctx, metav1.ListOptions{LabelSelector: labels.SelectorFromSet(sts.Spec.Selector.MatchLabels).String()})
	if err != nil || len(pods.Items) != int(*rs.Spec.Size) {
		return false
	}
	password, err := redischecker.NewChecker(r.K8sClient).GetPassword(ctx, rs.Namespace, rs.Spec.KubernetesConfig.ExistingPasswordSecret)
	if err != nil {
		return false
	}
	group := rs.Spec.RedisSentinelConfig.MasterGroupName
	if group == "" {
		group = "myMaster"
	}
	for _, pod := range pods.Items {
		if !podReady(pod) {
			return false
		}
		_, err := redis.NewClient().Connect(&redis.ConnectionInfo{Host: pod.Status.PodIP, Port: "26379", Password: password}).GetSentinelMaster(ctx, group)
		if err == nil || !strings.Contains(err.Error(), "No such master") {
			return false
		}
	}
	return true
}

type sentinelDecision struct {
	IP    string
	Epoch uint64
}

func sentinelEpoch(report redis.SentinelMasterState) (uint64, bool) {
	epoch, err := strconv.ParseUint(report.Epoch, 10, 64)
	return epoch, err == nil && report.IP != "" && report.Port == "6379" && hasFlag(report, "master")
}

func hasFlag(report redis.SentinelMasterState, flag string) bool {
	for _, value := range strings.Split(report.Flags, ",") {
		if value == flag {
			return true
		}
	}
	return false
}

func healthySentinel(report redis.SentinelMasterState) bool {
	return !hasFlag(report, "failover_in_progress") && !hasFlag(report, "s_down") && !hasFlag(report, "o_down") && !hasFlag(report, "disconnected")
}

// A majority at one epoch is required. A newer or same-epoch conflicting report,
// including an unhealthy one, blocks publication. This never permits role repair.
func agreedSentinelMaster(reports []redis.SentinelMasterState, configured int) (sentinelDecision, bool) {
	decision, ok := redis.AgreedSentinelMaster(reports, configured)
	return sentinelDecision{IP: decision.IP, Epoch: decision.Epoch}, ok
}

func (r *Reconciler) sentinelSelectedMaster(ctx context.Context, rr *rrvb2.RedisReplication, rs *rsvb2.RedisSentinel) (string, bool, error) {
	if r.StandaloneMaster != nil {
		selected, err := r.StandaloneMaster(ctx, rr, rs)
		return selected, false, err
	}
	if rs.Spec.Size == nil || *rs.Spec.Size < 1 || rs.Spec.RedisSentinelConfig == nil || rs.Spec.TLS != nil || rr.Spec.TLS != nil || rs.Spec.RedisSentinelConfig.ResolveHostnames == "yes" {
		return "", false, fmt.Errorf("standalone authority requires IP-addressed non-TLS Sentinel and Redis")
	}
	if rs.Spec.Storage == nil {
		return "", false, fmt.Errorf("standalone Sentinel authority requires persistent election state")
	}
	if port := rs.Spec.RedisSentinelConfig.RedisPort; port != "" && port != "6379" {
		return "", false, fmt.Errorf("unsupported Redis port")
	}
	sts, err := r.K8sClient.AppsV1().StatefulSets(rs.Namespace).Get(ctx, rs.GetStatefulSetName(), metav1.GetOptions{})
	if err != nil {
		return "", false, err
	}
	pods, err := r.K8sClient.CoreV1().Pods(rs.Namespace).List(ctx, metav1.ListOptions{LabelSelector: labels.SelectorFromSet(sts.Spec.Selector.MatchLabels).String()})
	if err != nil {
		return "", false, err
	}
	checker := redischecker.NewChecker(r.K8sClient)
	password, err := checker.GetPassword(ctx, rs.Namespace, rs.Spec.KubernetesConfig.ExistingPasswordSecret)
	if err != nil {
		return "", false, err
	}
	masterPassword, err := checker.GetPassword(ctx, rr.Namespace, rr.Spec.KubernetesConfig.ExistingPasswordSecret)
	if err != nil {
		return "", false, err
	}
	group := rs.Spec.RedisSentinelConfig.MasterGroupName
	if group == "" {
		group = "myMaster"
	}
	var reports []redis.SentinelMasterState
	var missing []redis.Service
	var authentication []observedSentinelAuth
	r.pruneSentinelAuth(rr, rs, pods.Items, group)
	for _, pod := range pods.Items {
		if !podReady(pod) || !persistentSentinelPod(pod, rs.GetStatefulSetName()) {
			continue
		}
		svc := redis.NewClient().Connect(&redis.ConnectionInfo{Host: pod.Status.PodIP, Port: "26379", Password: password})
		queryCtx, cancel := context.WithTimeout(ctx, 2*time.Second)
		report, err := svc.GetSentinelMaster(queryCtx, group)
		cancel()
		if err == nil {
			reports = append(reports, *report)
			authentication = append(authentication, observedSentinelAuth{pod: pod, service: svc, report: *report})
		} else if errors.Is(err, redis.ErrInvalidSentinelReport) {
			return "", false, err
		} else if strings.Contains(err.Error(), "No such master") {
			r.forgetSentinelAuth(authKey(rr, rs, pod, group))
			info, infoErr := svc.GetInfoSentinel(ctx)
			if infoErr != nil || info == nil {
				return "", false, fmt.Errorf("cannot verify absent Sentinel monitor")
			}
			if len(info.Masters) != 0 {
				return "", false, fmt.Errorf("sentinel already monitors another group; explicit migration is required")
			}
			missing = append(missing, svc)
		}
	}
	if len(missing) > 0 {
		allowed, err := r.mayInitializeMissingMonitors(ctx, rr, len(reports))
		if err != nil {
			return "", false, err
		}
		if !allowed {
			return "", false, fmt.Errorf("sentinel monitor history is uncertain; explicit voter-generation recovery is required")
		}
	}
	if err := r.reconcileStandaloneAuth(ctx, rr, rs, group, masterPassword, authentication); err != nil {
		return "", false, err
	}
	decision, ok := agreedSentinelMaster(reports, int(*rs.Spec.Size))
	if len(reports) > 0 && rr.Annotations[common.SentinelOwnershipAnnotation] != "sentinel" {
		if err := r.recordSentinelOwnership(ctx, rr, "sentinel"); err != nil {
			return "", false, err
		}
	}
	if !ok {
		// With lost monitors there is no election to overwrite. Seeding is
		// allowed only from a complete Redis topology when every configured
		// Sentinel answered and all surviving reports agree with that topology.
		// Unknown/unreachable peers are not evidence of an absent monitor.
		topology, err := k8sutils.GetRunningRedisReplicationTopology(ctx, r.K8sClient, rr)
		if err != nil || !r.bootstrapTopologyAvailable(ctx, rr, topology) || len(topology.Masters) != 1 || len(missing) == 0 || len(reports)+len(missing) != int(*rs.Spec.Size) {
			return "", false, fmt.Errorf("sentinel quorum has no uncontested healthy primary")
		}
		primary, err := r.K8sClient.CoreV1().Pods(rr.Namespace).Get(ctx, topology.Masters[0], metav1.GetOptions{})
		if err != nil || primary.Status.Phase != corev1.PodRunning || primary.DeletionTimestamp != nil || primary.Status.PodIP == "" {
			return "", false, fmt.Errorf("missing-monitor recovery requires a ready primary")
		}
		if !monitorRecoveryTopologyMatches(reports, primary.Status.PodIP) {
			return "", false, fmt.Errorf("sentinel quorum has no uncontested healthy primary")
		}
		attached, err := redis.NewClient().Connect(&redis.ConnectionInfo{Host: primary.Status.PodIP, Port: "6379", Password: masterPassword}).GetAttachedReplicaCount(ctx)
		if err != nil || attached != len(topology.Slaves) {
			return "", false, fmt.Errorf("missing-monitor recovery requires direct replicas attached to the sole primary")
		}
		decision = sentinelDecision{IP: primary.Status.PodIP, Epoch: 0}
	}
	var selected string
	for i := int32(0); rr.Spec.Size != nil && i < *rr.Spec.Size; i++ {
		pod, err := r.K8sClient.CoreV1().Pods(rr.Namespace).Get(ctx, fmt.Sprintf("%s-%d", rr.Name, i), metav1.GetOptions{})
		if err == nil && pod.Status.PodIP != "" {
			if pod.Status.PodIP == decision.IP && (podReady(*pod) || ((len(missing) > 0 || !ok) && pod.Status.Phase == corev1.PodRunning && pod.DeletionTimestamp == nil)) {
				selected = pod.Name
			}
		} else if err != nil && !apierrors.IsNotFound(err) {
			return "", false, err
		}
	}
	if selected == "" {
		return "", false, fmt.Errorf("sentinel primary is not a ready Redis member")
	}
	if primary, err := redis.NewClient().Connect(&redis.ConnectionInfo{Host: decision.IP, Port: "6379", Password: masterPassword}).IsMaster(ctx); err != nil || !primary {
		return "", false, fmt.Errorf("sentinel primary does not confirm role:master")
	}
	if len(missing) > 0 {
		// Persist the handoff before the first creation attempt. Losing every
		// monitor afterward must never return role ownership to bootstrap.
		if err := r.recordSentinelOwnership(ctx, rr, "sentinel"); err != nil {
			return "", false, err
		}
	}
	for _, service := range missing {
		if err := service.SentinelMonitorIfAbsent(ctx, &redis.ConnectionInfo{Host: decision.IP, Port: "6379", Password: masterPassword}, group, rs.Spec.RedisSentinelConfig.Quorum); err != nil {
			return "", false, err
		}
	}
	if len(missing) > 0 || !ok {
		return "", false, fmt.Errorf("initialized absent Sentinel monitors; waiting for agreement")
	}
	return selected, false, nil
}

func persistentSentinelPod(pod corev1.Pod, containerName string) bool {
	claim := false
	for _, volume := range pod.Spec.Volumes {
		if volume.Name == common.VolumeNameSentinelState && volume.PersistentVolumeClaim != nil && !volume.PersistentVolumeClaim.ReadOnly && volume.PersistentVolumeClaim.ClaimName == common.VolumeNameSentinelState+"-"+pod.Name {
			claim = true
		}
	}
	if !claim {
		return false
	}
	mounted := func(container corev1.Container) bool {
		for _, mount := range container.VolumeMounts {
			if mount.Name == common.VolumeNameSentinelState && mount.MountPath == "/etc/redis" && !mount.ReadOnly && mount.SubPath == "" && mount.SubPathExpr == "" {
				return true
			}
		}
		return false
	}
	main := false
	for _, container := range pod.Spec.Containers {
		if container.Name == containerName && mounted(container) {
			main = true
		}
	}
	if !main {
		return false
	}
	for _, init := range pod.Spec.InitContainers {
		if init.Name != "init-config" || !mounted(init) {
			continue
		}
		capable, persistent := false, false
		for _, arg := range init.Args {
			if arg == "--persistent-sentinel-state" {
				capable = true
			}
		}
		for _, env := range init.Env {
			if env.Name == "PERSIST_SENTINEL_CONFIG" && env.Value == "true" {
				persistent = true
			}
		}
		return capable && persistent
	}
	return false
}

func (r *Reconciler) recordSentinelOwnership(ctx context.Context, rr *rrvb2.RedisReplication, owner string) error {
	if rr.Annotations[common.SentinelOwnershipAnnotation] == owner {
		return nil
	}
	if rr.Annotations[common.SentinelOwnershipAnnotation] == "sentinel" {
		return fmt.Errorf("sentinel role ownership cannot return to bootstrap")
	}
	before := rr.DeepCopy()
	if rr.Annotations == nil {
		rr.Annotations = map[string]string{}
	}
	rr.Annotations[common.SentinelOwnershipAnnotation] = owner
	return r.Patch(ctx, rr, client.MergeFrom(before))
}

func (r *Reconciler) bootstrapTopologyAvailable(ctx context.Context, rr *rrvb2.RedisReplication, topology k8sutils.RedisReplicationTopology) bool {
	if topology.Complete() {
		return true
	}
	if rr.Spec.Size == nil || topology.Observed() < int(*rr.Spec.Size)/2+1 {
		return false
	}
	for _, name := range topology.Unobserved {
		pod, err := r.K8sClient.CoreV1().Pods(rr.Namespace).Get(ctx, name, metav1.GetOptions{})
		if err != nil || !protectedMemberInitializing(pod) {
			return false
		}
		if pod.Status.PodIP == "" {
			return false
		}
		conn, err := (&net.Dialer{Timeout: 500 * time.Millisecond}).DialContext(ctx, "tcp", net.JoinHostPort(pod.Status.PodIP, "6379"))
		if err == nil {
			_ = conn.Close()
			return false
		}
		// Stale Kubernetes readiness or an operator-side network partition
		// cannot establish that a competing Redis process is absent.
		if !errors.Is(err, syscall.ECONNREFUSED) {
			return false
		}
	}
	return true
}

// A protected init member cannot yet serve Redis or own a competing election.
// Missing, unreachable, running or legacy members never satisfy this exception.
func protectedMemberInitializing(pod *corev1.Pod) bool {
	if pod == nil || pod.DeletionTimestamp != nil || pod.Status.Phase != corev1.PodPending {
		return false
	}
	for _, status := range pod.Status.ContainerStatuses {
		if status.RestartCount > 0 || status.State.Running != nil || status.State.Terminated != nil || status.LastTerminationState.Running != nil || status.LastTerminationState.Terminated != nil || status.LastTerminationState.Waiting != nil {
			return false
		}
	}
	for _, status := range pod.Status.InitContainerStatuses {
		if status.Name == "init-config" && status.State.Terminated != nil && status.State.Terminated.ExitCode == 0 {
			return false
		}
	}
	for _, init := range pod.Spec.InitContainers {
		if init.Name != "init-config" {
			continue
		}
		capable, required := false, false
		for _, arg := range init.Args {
			if arg == "--standalone-authority" {
				capable = true
			}
		}
		for _, env := range init.Env {
			if env.Name == "SENTINEL_BOOTSTRAP_REQUIRED" && env.Value == "true" {
				required = true
			}
		}
		return capable && required
	}
	return false
}

func podReady(pod corev1.Pod) bool {
	if pod.Status.PodIP == "" || pod.DeletionTimestamp != nil {
		return false
	}
	for _, condition := range pod.Status.Conditions {
		if condition.Type == corev1.PodReady && condition.Status == corev1.ConditionTrue {
			return true
		}
	}
	return false
}

func monitorRecoveryTopologyMatches(reports []redis.SentinelMasterState, primaryIP string) bool {
	var knownEpoch uint64
	var known bool
	for _, report := range reports {
		epoch, valid := sentinelEpoch(report)
		if !valid || !healthySentinel(report) || report.IP != primaryIP {
			return false
		}
		if known && epoch != knownEpoch {
			return false
		}
		known, knownEpoch = true, epoch
	}
	return true
}

func (r *Reconciler) routeRedisMaster(ctx context.Context, rr *rrvb2.RedisReplication, masterPod string) error {
	discardRecovery := func(cause error) error {
		// Routing rechecks the serving-primary prerequisite after authority
		// selection. Failure invalidates the earlier recovery interval too.
		if err := r.resetReplacementObservation(ctx, rr); err != nil {
			return err
		}
		return cause
	}
	selector := common.GetRedisLabels(rr.Name, common.SetupTypeReplication, "replication", rr.Labels)
	if err := r.Healer.SetRedisMasterLabel(ctx, rr.Namespace, selector, rr.Spec.KubernetesConfig.ExistingPasswordSecret, rr.Spec.TLS, masterPod); err != nil {
		routeErr := discardRecovery(err)
		_ = r.UpdateRedisReplicationMaster(ctx, rr, "", true)
		return routeErr
	}
	if err := r.UpdateRedisReplicationMaster(ctx, rr, masterPod, true); err != nil {
		return discardRecovery(err)
	}
	if masterPod != "" && rr.Annotations[common.SentinelStartupAnnotation] != "enabled" {
		before := rr.DeepCopy()
		if rr.Annotations == nil {
			rr.Annotations = map[string]string{}
		}
		rr.Annotations[common.SentinelStartupAnnotation] = "enabled"
		if err := r.Patch(ctx, rr, client.MergeFrom(before)); err != nil {
			return discardRecovery(err)
		}
	}
	return nil
}
