package redisreplication

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strconv"
	"time"

	rrvb2 "github.com/OT-CONTAINER-KIT/redis-operator/api/redisreplication/v1beta2"
	rsvb2 "github.com/OT-CONTAINER-KIT/redis-operator/api/redissentinel/v1beta2"
	"github.com/OT-CONTAINER-KIT/redis-operator/internal/controller/common"
	redischecker "github.com/OT-CONTAINER-KIT/redis-operator/internal/controller/common/redis"
	"github.com/OT-CONTAINER-KIT/redis-operator/internal/service/redis"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
)

const replacementMinimumAge = 90 * time.Second

type replacementRecoveryProbe interface {
	Redis(context.Context, string, string) (redis.ReplicationObservation, error)
	Sentinel(context.Context, string, string, string) (redis.SentinelReplacementObservation, error)
}

type networkReplacementProbe struct{}

func (networkReplacementProbe) Redis(ctx context.Context, ip, password string) (redis.ReplicationObservation, error) {
	return redis.ObserveReplication(ctx, ip, password)
}

func (networkReplacementProbe) Sentinel(ctx context.Context, ip, password, group string) (redis.SentinelReplacementObservation, error) {
	return redis.ObserveSentinelReplacement(ctx, ip, password, group)
}

type replacementSnapshot struct {
	Pod       corev1.Pod
	ClaimUID  types.UID
	Binding   string
	PrimaryIP string
	Epoch     uint64
}

type replacementIdentity struct {
	Name, IP, Voter string
	UID, Claim      types.UID
	Restarts        int32
}

func (r *Reconciler) replacementTime() time.Time {
	if r.replacementNow != nil {
		return r.replacementNow().UTC()
	}
	return time.Now().UTC()
}

func controlledBy(meta metav1.Object, kind, name string, uid types.UID) bool {
	owner := metav1.GetControllerOf(meta)
	return uid != "" && owner != nil && owner.Kind == kind && owner.Name == name && owner.UID == uid
}

func replicaFollows(state redis.ReplicationObservation, ip string) bool {
	return state.Role == "slave" && state.MasterHost == ip && state.MasterPort == "6379" && state.MasterLink == "up" && state.Priority > 0
}

func onlineAttached(state redis.ReplicationObservation, ip string) bool {
	for _, replica := range state.Replicas {
		if replica.IP == ip && replica.Port == "6379" && replica.State == "online" && replica.Lag <= 5 {
			return true
		}
	}
	return false
}

func protectedReplacement(pod corev1.Pod, name string, now time.Time) bool {
	if pod.UID == "" || pod.Status.PodIP == "" || pod.Status.Phase != corev1.PodRunning || pod.DeletionTimestamp != nil || podReady(pod) {
		return false
	}
	if pod.Spec.TerminationGracePeriodSeconds != nil && *pod.Spec.TerminationGracePeriodSeconds < 1 {
		return false // Never turn this recovery into force deletion.
	}
	initialized, running := false, false
	for _, status := range pod.Status.InitContainerStatuses {
		if status.Name == "init-config" && status.State.Terminated != nil && status.State.Terminated.ExitCode == 0 && !status.State.Terminated.FinishedAt.IsZero() {
			initialized = now.Sub(status.State.Terminated.FinishedAt.Time) >= replacementMinimumAge
		}
	}
	for _, status := range pod.Status.ContainerStatuses {
		if status.Name == name && status.RestartCount == 0 && status.LastTerminationState.Terminated == nil && status.State.Running != nil && !status.State.Running.StartedAt.IsZero() {
			running = now.Sub(status.State.Running.StartedAt.Time) >= replacementMinimumAge
		}
	}
	capable, required := false, false
	for _, init := range pod.Spec.InitContainers {
		if init.Name != "init-config" {
			continue
		}
		for _, arg := range init.Args {
			capable = capable || arg == "--standalone-authority"
		}
		for _, env := range init.Env {
			required = required || (env.Name == "SENTINEL_BOOTSTRAP_REQUIRED" && env.Value == "true")
		}
	}
	return initialized && running && capable && required
}

func retainedWorkload(sts *appsv1.StatefulSet, kind, name string, uid types.UID) bool {
	if sts == nil || sts.UID == "" || sts.DeletionTimestamp != nil || sts.Spec.Replicas == nil || *sts.Spec.Replicas != 3 || !controlledBy(sts, kind, name, uid) {
		return false
	}
	// currentRevision can remain old until every updated pod is Ready. Using
	// that readiness-dependent field here would prevent recovery of the very
	// member whose readiness cannot progress. Every actual pod is checked
	// against updateRevision below instead.
	if sts.Status.ObservedGeneration != sts.Generation || sts.Status.UpdateRevision == "" || sts.Status.UpdatedReplicas != 3 {
		return false
	}
	policy := sts.Spec.PersistentVolumeClaimRetentionPolicy
	return policy == nil || ((policy.WhenDeleted == "" || policy.WhenDeleted == appsv1.RetainPersistentVolumeClaimRetentionPolicyType) && (policy.WhenScaled == "" || policy.WhenScaled == appsv1.RetainPersistentVolumeClaimRetentionPolicyType))
}

func (r *Reconciler) replacementDataClaim(ctx context.Context, pod corev1.Pod, sts *appsv1.StatefulSet, containerName string) (types.UID, error) {
	volumeName := ""
	for _, container := range pod.Spec.Containers {
		if container.Name != containerName {
			continue
		}
		for _, mount := range container.VolumeMounts {
			if mount.MountPath == "/data" && !mount.ReadOnly && mount.SubPath == "" && mount.SubPathExpr == "" {
				volumeName = mount.Name
			}
		}
	}
	initMounted := false
	for _, init := range pod.Spec.InitContainers {
		if init.Name != "init-config" {
			continue
		}
		for _, mount := range init.VolumeMounts {
			if mount.Name == volumeName && volumeName != "" && mount.MountPath == "/data" && mount.ReadOnly && mount.SubPath == "" && mount.SubPathExpr == "" {
				initMounted = true
			}
		}
	}
	if !initMounted {
		return "", fmt.Errorf("replacement recovery requires protected initialization of the retained data claim")
	}
	claimName := ""
	for _, template := range sts.Spec.VolumeClaimTemplates {
		if template.Name == volumeName && template.Spec.DataSource == nil && template.Spec.DataSourceRef == nil {
			claimName = template.Name + "-" + pod.Name
		}
	}
	for _, volume := range pod.Spec.Volumes {
		if volume.Ephemeral != nil {
			return "", fmt.Errorf("replacement recovery cannot collect generic ephemeral claims")
		}
		if volume.Name != volumeName && volume.PersistentVolumeClaim != nil {
			if _, err := r.recoveryClaimUID(ctx, pod.Namespace, volume.PersistentVolumeClaim.ClaimName); err != nil {
				return "", err
			}
		}
	}
	for _, volume := range pod.Spec.Volumes {
		if volume.Name == volumeName && volume.PersistentVolumeClaim != nil && !volume.PersistentVolumeClaim.ReadOnly && claimName != "" && volume.PersistentVolumeClaim.ClaimName == claimName {
			return r.recoveryClaimUID(ctx, pod.Namespace, claimName)
		}
	}
	return "", fmt.Errorf("replacement recovery requires its retained data claim")
}

func (r *Reconciler) recoveryClaimUID(ctx context.Context, namespace, name string) (types.UID, error) {
	claim, err := r.K8sClient.CoreV1().PersistentVolumeClaims(namespace).Get(ctx, name, metav1.GetOptions{})
	if err != nil || claim == nil || claim.UID == "" || claim.DeletionTimestamp != nil || claim.Status.Phase != corev1.ClaimBound {
		return "", fmt.Errorf("replacement recovery requires an intact bound claim")
	}
	for _, owner := range claim.OwnerReferences {
		if owner.Kind == "Pod" {
			return "", fmt.Errorf("replacement recovery cannot delete a claim-owning pod")
		}
	}
	return claim.UID, nil
}

// observeReplacement requires all three healthy retained voters, a serving
// primary, a healthy direct replica, and an undiscovered replica behind that
// replica. Ordinary discovery delay, active elections and partial evidence do
// not grant permission to recreate a pod.
func (r *Reconciler) observeReplacement(ctx context.Context, rr *rrvb2.RedisReplication, rs *rsvb2.RedisSentinel, selected string) (*replacementSnapshot, error) {
	if rr.UID == "" || rs.UID == "" || rr.Spec.Size == nil || *rr.Spec.Size != 3 || rs.Spec.Size == nil || *rs.Spec.Size != 3 || rr.Spec.Storage == nil || rs.Spec.Storage == nil || rr.EnableSentinel() || rr.Spec.TLS != nil || rs.Spec.TLS != nil || rs.Spec.RedisSentinelConfig == nil || rs.Spec.RedisSentinelConfig.ResolveHostnames == "yes" {
		return nil, nil
	}
	redisSTS, err := r.K8sClient.AppsV1().StatefulSets(rr.Namespace).Get(ctx, rr.Name, metav1.GetOptions{})
	if err != nil || !retainedWorkload(redisSTS, "RedisReplication", rr.Name, rr.UID) {
		return nil, fmt.Errorf("replacement recovery is unavailable during workload changes")
	}
	sentinelSTS, err := r.K8sClient.AppsV1().StatefulSets(rs.Namespace).Get(ctx, rs.GetStatefulSetName(), metav1.GetOptions{})
	if err != nil || !retainedWorkload(sentinelSTS, "RedisSentinel", rs.Name, rs.UID) {
		return nil, fmt.Errorf("replacement recovery requires stable retained Sentinel resources")
	}
	probe := r.replacementProbe
	if probe == nil {
		probe = networkReplacementProbe{}
	}
	checker := redischecker.NewChecker(r.K8sClient)
	password, err := checker.GetPassword(ctx, rr.Namespace, rr.Spec.KubernetesConfig.ExistingPasswordSecret)
	if err != nil {
		return nil, fmt.Errorf("cannot resolve replacement observation credentials")
	}
	sentinelPassword, err := checker.GetPassword(ctx, rs.Namespace, rs.Spec.KubernetesConfig.ExistingPasswordSecret)
	if err != nil {
		return nil, fmt.Errorf("cannot resolve Sentinel observation credentials")
	}
	pods := map[string]corev1.Pod{}
	states := map[string]redis.ReplicationObservation{}
	identities := []replacementIdentity{}
	var candidate, direct, primary string
	for i := 0; i < 3; i++ {
		name := fmt.Sprintf("%s-%d", rr.Name, i)
		pod, err := r.K8sClient.CoreV1().Pods(rr.Namespace).Get(ctx, name, metav1.GetOptions{})
		if err != nil || pod.UID == "" || pod.Status.Phase != corev1.PodRunning || pod.DeletionTimestamp != nil || pod.Status.PodIP == "" || pod.Labels[appsv1.ControllerRevisionHashLabelKey] != redisSTS.Status.UpdateRevision || !controlledBy(pod, "StatefulSet", redisSTS.Name, redisSTS.UID) {
			return nil, fmt.Errorf("replacement recovery requires three current running members")
		}
		state, err := probe.Redis(ctx, pod.Status.PodIP, password)
		if err != nil {
			return nil, fmt.Errorf("cannot verify replacement replication chain")
		}
		pods[name], states[name] = *pod, state
		if name == selected {
			if !podReady(*pod) || state.Role != "master" {
				return nil, fmt.Errorf("replacement recovery requires a verified serving primary")
			}
			primary = name
		} else if podReady(*pod) && state.Role == "slave" {
			if direct != "" {
				return nil, nil
			}
			direct = name
		} else if protectedReplacement(*pod, rr.Name, r.replacementTime()) && state.Role == "slave" && pod.Labels[common.RedisRoleLabelKey] != common.RedisRoleLabelMaster {
			if candidate != "" {
				return nil, nil
			}
			candidate = name
		} else {
			return nil, nil
		}
		identity := replacementIdentity{Name: name, UID: pod.UID, IP: pod.Status.PodIP}
		for _, status := range pod.Status.ContainerStatuses {
			if status.Name == rr.Name {
				identity.Restarts = status.RestartCount
			}
		}
		identities = append(identities, identity)
	}
	if primary == "" || direct == "" || candidate == "" {
		return nil, nil
	}
	p, a, target := pods[primary], pods[direct], pods[candidate]
	if len(states[primary].Replicas) != 1 || len(states[direct].Replicas) != 1 || len(states[candidate].Replicas) != 0 || !replicaFollows(states[direct], p.Status.PodIP) || !replicaFollows(states[candidate], a.Status.PodIP) || !onlineAttached(states[primary], a.Status.PodIP) || onlineAttached(states[primary], target.Status.PodIP) || !onlineAttached(states[direct], target.Status.PodIP) {
		return nil, nil
	}
	claim, err := r.replacementDataClaim(ctx, target, redisSTS, rr.Name)
	if err != nil {
		return nil, err
	}
	group := rs.Spec.RedisSentinelConfig.MasterGroupName
	if group == "" {
		group = "myMaster"
	}
	var epoch uint64
	voters := map[string]bool{}
	for i := 0; i < 3; i++ {
		name := fmt.Sprintf("%s-%d", rs.GetStatefulSetName(), i)
		pod, err := r.K8sClient.CoreV1().Pods(rs.Namespace).Get(ctx, name, metav1.GetOptions{})
		if err != nil || pod.UID == "" || pod.DeletionTimestamp != nil || pod.Status.Phase != corev1.PodRunning || !podReady(*pod) || pod.Labels[appsv1.ControllerRevisionHashLabelKey] != sentinelSTS.Status.UpdateRevision || !controlledBy(pod, "StatefulSet", sentinelSTS.Name, sentinelSTS.UID) || !persistentSentinelPod(*pod, rs.GetStatefulSetName()) {
			return nil, fmt.Errorf("replacement recovery requires every retained Sentinel")
		}
		sentinelClaim, err := r.recoveryClaimUID(ctx, rs.Namespace, common.VolumeNameSentinelState+"-"+pod.Name)
		if err != nil {
			return nil, err
		}
		report, err := probe.Sentinel(ctx, pod.Status.PodIP, sentinelPassword, group)
		observedEpoch, epochErr := strconv.ParseUint(report.Master.Epoch, 10, 64)
		if err != nil || epochErr != nil || observedEpoch == 0 || report.Master.Flags != "master" || (report.Master.FailoverState != "" && report.Master.FailoverState != "none") || report.Master.IP != p.Status.PodIP || report.Master.Port != "6379" || report.VoterID == "" || voters[report.VoterID] || (i > 0 && observedEpoch != epoch) {
			return nil, fmt.Errorf("replacement recovery requires unanimous stable authority")
		}
		epoch = observedEpoch
		voters[report.VoterID] = true
		eligible := false
		for _, replica := range report.Replicas {
			if replica.IP == target.Status.PodIP {
				return nil, nil // Sentinel already owns this member's repair.
			}
			if replica.IP == a.Status.PodIP && replica.Port == "6379" && replica.Flags == "slave" && replica.Role == "slave" && replica.MasterHost == p.Status.PodIP && replica.MasterPort == "6379" && replica.MasterLink == "ok" && replica.Priority > 0 && replica.InfoAge <= 30000 {
				eligible = true
			}
		}
		if !eligible {
			return nil, fmt.Errorf("replacement recovery requires a healthy discovered direct replica")
		}
		identity := replacementIdentity{Name: pod.Name, UID: pod.UID, IP: pod.Status.PodIP, Claim: sentinelClaim, Voter: report.VoterID}
		for _, status := range pod.Status.ContainerStatuses {
			if status.Name == rs.GetStatefulSetName() {
				identity.Restarts = status.RestartCount
			}
		}
		identities = append(identities, identity)
	}
	// This digest contains public workload/voter identities, never credentials.
	binding, err := json.Marshal(struct {
		Replication, Sentinel, RedisSTS, SentinelSTS, Claim                    types.UID
		Generation, SentinelGeneration, RedisGeneration, SentinelSTSGeneration int64
		Group, Primary, RedisRevision, SentinelRevision                        string
		Epoch                                                                  uint64
		Members                                                                []replacementIdentity
	}{
		Replication: rr.UID, Sentinel: rs.UID, RedisSTS: redisSTS.UID, SentinelSTS: sentinelSTS.UID, Claim: claim,
		Generation: rr.Generation, SentinelGeneration: rs.Generation, RedisGeneration: redisSTS.Generation, SentinelSTSGeneration: sentinelSTS.Generation,
		Group: group, Primary: selected, RedisRevision: redisSTS.Status.UpdateRevision, SentinelRevision: sentinelSTS.Status.UpdateRevision, Epoch: epoch, Members: identities,
	})
	if err != nil {
		return nil, fmt.Errorf("cannot bind replacement observations")
	}
	digest := sha256.Sum256(binding)
	return &replacementSnapshot{Pod: target, ClaimUID: claim, Binding: hex.EncodeToString(digest[:]), PrimaryIP: p.Status.PodIP, Epoch: epoch}, nil
}
