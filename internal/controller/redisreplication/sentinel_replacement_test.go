package redisreplication

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"
	"time"

	commonapi "github.com/OT-CONTAINER-KIT/redis-operator/api/common/v1beta2"
	rrvb2 "github.com/OT-CONTAINER-KIT/redis-operator/api/redisreplication/v1beta2"
	rsvb2 "github.com/OT-CONTAINER-KIT/redis-operator/api/redissentinel/v1beta2"
	"github.com/OT-CONTAINER-KIT/redis-operator/internal/controller/common"
	"github.com/OT-CONTAINER-KIT/redis-operator/internal/service/redis"
	"github.com/stretchr/testify/require"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes"
	kubernetesfake "k8s.io/client-go/kubernetes/fake"
	k8stesting "k8s.io/client-go/testing"
	"k8s.io/utils/ptr"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	clientfake "sigs.k8s.io/controller-runtime/pkg/client/fake"
)

type fakeReplacementProbe struct {
	redis     map[string]redis.ReplicationObservation
	sentinels map[string]redis.SentinelReplacementObservation
	fail      map[string]bool
	onRead    func(string)
}

func (p *fakeReplacementProbe) Redis(_ context.Context, ip, _ string) (redis.ReplicationObservation, error) {
	if p.onRead != nil {
		p.onRead(ip)
	}
	if p.fail[ip] {
		return redis.ReplicationObservation{}, fmt.Errorf("synthetic unavailable member")
	}
	return p.redis[ip], nil
}

func (p *fakeReplacementProbe) Sentinel(_ context.Context, ip, _, _ string) (redis.SentinelReplacementObservation, error) {
	if p.onRead != nil {
		p.onRead(ip)
	}
	if p.fail[ip] {
		return redis.SentinelReplacementObservation{}, fmt.Errorf("synthetic unavailable voter")
	}
	return p.sentinels[ip], nil
}

type replacementFixture struct {
	t       *testing.T
	rr      *rrvb2.RedisReplication
	rs      *rsvb2.RedisSentinel
	api     client.Client
	k8s     *kubernetesfake.Clientset
	probe   *fakeReplacementProbe
	now     time.Time
	primary string
	target  string
	deleted []metav1.DeleteOptions
}

func replacementOwner(kind, name string, uid types.UID) []metav1.OwnerReference {
	return []metav1.OwnerReference{{Kind: kind, Name: name, UID: uid, Controller: ptr.To(true)}}
}

func newReplacementFixture(t *testing.T) *replacementFixture {
	t.Helper()
	f := &replacementFixture{t: t, now: time.Date(2026, 10, 28, 12, 0, 0, 0, time.UTC), primary: "replication-0", target: "replication-2"}
	f.rr = &rrvb2.RedisReplication{
		ObjectMeta: metav1.ObjectMeta{
			Name: "replication", Namespace: "default", UID: "replication-owner", Generation: 1,
			Annotations: map[string]string{common.SentinelOwnershipAnnotation: "sentinel", common.SentinelStartupAnnotation: "enabled"},
		},
		Spec: rrvb2.RedisReplicationSpec{Size: ptr.To(int32(3)), Storage: &commonapi.Storage{}},
	}
	f.rs = &rsvb2.RedisSentinel{ObjectMeta: metav1.ObjectMeta{Name: "sentinel", Namespace: "default", UID: "sentinel-owner", Generation: 1}, Spec: rsvb2.RedisSentinelSpec{Size: ptr.To(int32(3)), Storage: &rsvb2.RedisSentinelStorage{}, RedisSentinelConfig: &rsvb2.RedisSentinelConfig{RedisSentinelConfig: commonapi.RedisSentinelConfig{RedisReplicationName: f.rr.Name, MasterGroupName: "myMaster"}}}}
	scheme := runtime.NewScheme()
	require.NoError(t, rrvb2.AddToScheme(scheme))
	require.NoError(t, rsvb2.AddToScheme(scheme))
	f.api = clientfake.NewClientBuilder().WithScheme(scheme).WithStatusSubresource(f.rr).WithObjects(f.rr, f.rs).Build()
	redisSTS := &appsv1.StatefulSet{ObjectMeta: metav1.ObjectMeta{Name: f.rr.Name, Namespace: f.rr.Namespace, UID: "redis-sts", Generation: 1, OwnerReferences: replacementOwner("RedisReplication", f.rr.Name, f.rr.UID)}, Spec: appsv1.StatefulSetSpec{Replicas: ptr.To(int32(3)), VolumeClaimTemplates: []corev1.PersistentVolumeClaim{{ObjectMeta: metav1.ObjectMeta{Name: "data"}}}}, Status: appsv1.StatefulSetStatus{ObservedGeneration: 1, CurrentRevision: "redis-revision", UpdateRevision: "redis-revision"}}
	sentinelSTS := &appsv1.StatefulSet{ObjectMeta: metav1.ObjectMeta{Name: f.rs.GetStatefulSetName(), Namespace: f.rs.Namespace, UID: "sentinel-sts", Generation: 1, OwnerReferences: replacementOwner("RedisSentinel", f.rs.Name, f.rs.UID)}, Spec: appsv1.StatefulSetSpec{Replicas: ptr.To(int32(3))}, Status: appsv1.StatefulSetStatus{ObservedGeneration: 1, CurrentRevision: "sentinel-revision", UpdateRevision: "sentinel-revision"}}
	redisSTS.Status.UpdatedReplicas, sentinelSTS.Status.UpdatedReplicas = 3, 3
	objects := []runtime.Object{redisSTS, sentinelSTS}
	f.probe = &fakeReplacementProbe{redis: map[string]redis.ReplicationObservation{}, sentinels: map[string]redis.SentinelReplacementObservation{}, fail: map[string]bool{}}
	for i := 0; i < 3; i++ {
		name, ip := fmt.Sprintf("replication-%d", i), fmt.Sprintf("192.0.2.%d", 10+i)
		pod := &corev1.Pod{ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: "default", UID: types.UID(name + "-uid"), ResourceVersion: "1", OwnerReferences: replacementOwner("StatefulSet", redisSTS.Name, redisSTS.UID), Labels: map[string]string{common.RedisRoleLabelKey: "slave"}}, Spec: corev1.PodSpec{TerminationGracePeriodSeconds: ptr.To(int64(30)), Containers: []corev1.Container{{Name: f.rr.Name, VolumeMounts: []corev1.VolumeMount{{Name: "data", MountPath: "/data"}}}}, InitContainers: []corev1.Container{{Name: "init-config", Args: []string{"bootstrap", "--standalone-authority"}, Env: []corev1.EnvVar{{Name: "SENTINEL_BOOTSTRAP_REQUIRED", Value: "true"}}}}, Volumes: []corev1.Volume{{Name: "data", VolumeSource: corev1.VolumeSource{PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{ClaimName: "data-" + name}}}}}, Status: corev1.PodStatus{Phase: corev1.PodRunning, PodIP: ip, Conditions: []corev1.PodCondition{{Type: corev1.PodReady, Status: corev1.ConditionTrue}}, ContainerStatuses: []corev1.ContainerStatus{{Name: f.rr.Name, State: corev1.ContainerState{Running: &corev1.ContainerStateRunning{StartedAt: metav1.NewTime(f.now.Add(-5 * time.Minute))}}}}, InitContainerStatuses: []corev1.ContainerStatus{{Name: "init-config", State: corev1.ContainerState{Terminated: &corev1.ContainerStateTerminated{ExitCode: 0, FinishedAt: metav1.NewTime(f.now.Add(-6 * time.Minute))}}}}}}
		state := redis.ReplicationObservation{Role: "slave", MasterHost: "192.0.2.10", MasterPort: "6379", MasterLink: "up", Priority: 100}
		switch i {
		case 0:
			pod.Labels[common.RedisRoleLabelKey] = "master"
			state = redis.ReplicationObservation{Role: "master", Replicas: []redis.AttachedReplica{{IP: "192.0.2.11", Port: "6379", State: "online"}}}
		case 1:
			state.Replicas = []redis.AttachedReplica{{IP: "192.0.2.12", Port: "6379", State: "online"}}
		case 2:
			pod.Status.Conditions[0].Status = corev1.ConditionFalse
			state.MasterHost = "192.0.2.11"
		}
		f.probe.redis[ip] = state
		pod.Spec.InitContainers[0].VolumeMounts = []corev1.VolumeMount{{Name: "data", MountPath: "/data", ReadOnly: true}}
		pod.Labels[appsv1.ControllerRevisionHashLabelKey] = "redis-revision"
		objects = append(objects, pod, &corev1.PersistentVolumeClaim{ObjectMeta: metav1.ObjectMeta{Name: "data-" + name, Namespace: "default", UID: types.UID("data-" + name + "-uid")}, Status: corev1.PersistentVolumeClaimStatus{Phase: corev1.ClaimBound}})
		voterName, voterIP := fmt.Sprintf("%s-%d", f.rs.GetStatefulSetName(), i), fmt.Sprintf("192.0.2.%d", 20+i)
		mount := corev1.VolumeMount{Name: common.VolumeNameSentinelState, MountPath: "/etc/redis"}
		voter := &corev1.Pod{ObjectMeta: metav1.ObjectMeta{Name: voterName, Namespace: "default", UID: types.UID(voterName + "-uid"), OwnerReferences: replacementOwner("StatefulSet", sentinelSTS.Name, sentinelSTS.UID)}, Spec: corev1.PodSpec{Volumes: []corev1.Volume{{Name: common.VolumeNameSentinelState, VolumeSource: corev1.VolumeSource{PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{ClaimName: common.VolumeNameSentinelState + "-" + voterName}}}}, Containers: []corev1.Container{{Name: f.rs.GetStatefulSetName(), VolumeMounts: []corev1.VolumeMount{mount}}}, InitContainers: []corev1.Container{{Name: "init-config", Args: []string{"bootstrap", "--persistent-sentinel-state"}, Env: []corev1.EnvVar{{Name: "PERSIST_SENTINEL_CONFIG", Value: "true"}}, VolumeMounts: []corev1.VolumeMount{mount}}}}, Status: corev1.PodStatus{Phase: corev1.PodRunning, PodIP: voterIP, Conditions: []corev1.PodCondition{{Type: corev1.PodReady, Status: corev1.ConditionTrue}}}}
		voter.Labels = map[string]string{appsv1.ControllerRevisionHashLabelKey: "sentinel-revision"}
		objects = append(objects, voter, &corev1.PersistentVolumeClaim{ObjectMeta: metav1.ObjectMeta{Name: common.VolumeNameSentinelState + "-" + voterName, Namespace: "default", UID: types.UID("claim-" + voterName)}, Status: corev1.PersistentVolumeClaimStatus{Phase: corev1.ClaimBound}})
		f.probe.sentinels[voterIP] = redis.SentinelReplacementObservation{Master: redis.SentinelMasterState{IP: "192.0.2.10", Port: "6379", Flags: "master", Epoch: "7"}, VoterID: fmt.Sprintf("%040x", i+1), Replicas: []redis.SentinelReplicaObservation{{IP: "192.0.2.11", Port: "6379", Flags: "slave", Role: "slave", MasterHost: "192.0.2.10", MasterPort: "6379", MasterLink: "ok", Priority: 100, InfoAge: 100}}}
	}
	f.k8s = kubernetesfake.NewSimpleClientset(objects...)
	f.k8s.PrependReactor("delete", "pods", func(action k8stesting.Action) (bool, runtime.Object, error) {
		deletion := action.(k8stesting.DeleteAction)
		options := deletion.GetDeleteOptions()
		require.Equal(t, f.target, deletion.GetName())
		require.Nil(t, options.GracePeriodSeconds, "normal pod grace must be preserved")
		require.NotNil(t, options.Preconditions)
		require.NotNil(t, options.Preconditions.UID)
		require.NotNil(t, options.Preconditions.ResourceVersion)
		pod, err := f.k8s.Tracker().Get(corev1.SchemeGroupVersion.WithResource("pods"), "default", deletion.GetName())
		require.NoError(t, err)
		require.Equal(t, pod.(*corev1.Pod).UID, *options.Preconditions.UID)
		require.Equal(t, pod.(*corev1.Pod).ResourceVersion, *options.Preconditions.ResourceVersion)
		f.deleted = append(f.deleted, options)
		return false, nil, nil
	})
	return f
}

func (f *replacementFixture) reconciler() *Reconciler {
	return &Reconciler{Client: f.api, APIReader: f.api, K8sClient: f.k8s, replacementProbe: f.probe, replacementNow: func() time.Time { return f.now }}
}

func (f *replacementFixture) run(r *Reconciler) {
	f.t.Helper()
	require.NoError(f.t, r.reconcileStandaloneReplacement(context.Background(), f.rr, f.rs, f.primary))
}

func (f *replacementFixture) advance(r *Reconciler, duration time.Duration) {
	f.t.Helper()
	for duration > 0 {
		step := min(duration, 5*time.Second)
		f.now = f.now.Add(step)
		f.run(r)
		duration -= step
	}
}

func (f *replacementFixture) ledger() replacementLedger {
	f.t.Helper()
	stored := &rrvb2.RedisReplication{}
	require.NoError(f.t, f.api.Get(context.Background(), client.ObjectKeyFromObject(f.rr), stored))
	state, err := readReplacementLedger(stored)
	require.NoError(f.t, err)
	return state
}

func (f *replacementFixture) updatePod(name string, update func(*corev1.Pod)) {
	f.t.Helper()
	pod, err := f.k8s.CoreV1().Pods("default").Get(context.Background(), name, metav1.GetOptions{})
	require.NoError(f.t, err)
	update(pod)
	_, err = f.k8s.CoreV1().Pods("default").Update(context.Background(), pod, metav1.UpdateOptions{})
	require.NoError(f.t, err)
}

func TestReplacementRecoveryRequiresStableEvidenceAndPreservesClaims(t *testing.T) {
	f := newReplacementFixture(t)
	r := f.reconciler()
	claims, err := f.k8s.CoreV1().PersistentVolumeClaims("default").List(context.Background(), metav1.ListOptions{})
	require.NoError(t, err)
	f.run(r)
	require.NotNil(t, f.ledger().Observation)
	require.Empty(t, f.deleted)
	f.advance(r, replacementObservationPeriod-time.Second)
	require.Empty(t, f.deleted)
	f.now = f.now.Add(2 * time.Second)
	f.run(r)
	require.Len(t, f.deleted, 1)
	require.Equal(t, 1, f.ledger().Attempts)
	require.Equal(t, types.UID("replication-2-uid"), f.ledger().Pending.UID)
	after, err := f.k8s.CoreV1().PersistentVolumeClaims("default").List(context.Background(), metav1.ListOptions{})
	require.NoError(t, err)
	require.Equal(t, claims.Items, after.Items)
	f.run(f.reconciler()) // A restarted controller must wait for the deleted UID.
	require.Len(t, f.deleted, 1)
}

func TestReplacementRecoveryDoesNotDependOnReadinessGatedCurrentRevision(t *testing.T) {
	f := newReplacementFixture(t)
	sts, err := f.k8s.AppsV1().StatefulSets("default").Get(context.Background(), f.rr.Name, metav1.GetOptions{})
	require.NoError(t, err)
	sts.Status.CurrentRevision = "older-ready-revision"
	_, err = f.k8s.AppsV1().StatefulSets("default").Update(context.Background(), sts, metav1.UpdateOptions{})
	require.NoError(t, err)
	r := f.reconciler()
	f.run(r)
	f.advance(r, replacementObservationPeriod)
	require.Len(t, f.deleted, 1, "all actual pod revisions are updated; currentRevision must not create a readiness deadlock")
}

func TestReplacementRecoveryHoldsOnIncompleteOrInapplicableEvidence(t *testing.T) {
	for _, name := range []string{"young process", "unfinished init", "unprotected init", "missing init data mount", "restarted process", "zero grace", "target primary", "direct replacement", "direct source down", "unexpected attached replica", "target has downstream replica", "unreachable target", "unreachable voter", "active election", "higher minority epoch", "missing monitor", "duplicate voter", "known target", "unhealthy direct replica", "lost data claim", "claim owned by pod", "additional claim owned by pod", "generic ephemeral claim", "foreign pod", "terminating Sentinel", "rolling update", "unupdated Redis pod", "unupdated Sentinel pod", "missing reader", "changed generation", "missing handoff"} {
		t.Run(name, func(t *testing.T) {
			f := newReplacementFixture(t)
			r := f.reconciler()
			switch name {
			case "young process":
				f.updatePod(f.target, func(p *corev1.Pod) {
					p.Status.ContainerStatuses[0].State.Running.StartedAt = metav1.NewTime(f.now.Add(-10 * time.Second))
				})
			case "unfinished init":
				f.updatePod(f.target, func(p *corev1.Pod) { p.Status.InitContainerStatuses = nil })
			case "unprotected init":
				f.updatePod(f.target, func(p *corev1.Pod) { p.Spec.InitContainers[0].Args = []string{"bootstrap"} })
			case "missing init data mount":
				f.updatePod(f.target, func(p *corev1.Pod) { p.Spec.InitContainers[0].VolumeMounts = nil })
			case "restarted process":
				f.updatePod(f.target, func(p *corev1.Pod) { p.Status.ContainerStatuses[0].RestartCount = 1 })
			case "zero grace":
				f.updatePod(f.target, func(p *corev1.Pod) { p.Spec.TerminationGracePeriodSeconds = ptr.To(int64(0)) })
			case "target primary":
				state := f.probe.redis["192.0.2.12"]
				state.Role = "master"
				f.probe.redis["192.0.2.12"] = state
			case "direct replacement":
				state := f.probe.redis["192.0.2.12"]
				state.MasterHost = "192.0.2.10"
				f.probe.redis["192.0.2.12"] = state
			case "direct source down":
				state := f.probe.redis["192.0.2.11"]
				state.MasterLink = "down"
				f.probe.redis["192.0.2.11"] = state
			case "unreachable target":
				f.probe.fail["192.0.2.12"] = true
			case "unexpected attached replica", "target has downstream replica":
				ip := "192.0.2.10"
				if name == "target has downstream replica" {
					ip = "192.0.2.12"
				}
				state := f.probe.redis[ip]
				state.Replicas = append(state.Replicas, redis.AttachedReplica{IP: "192.0.2.99", Port: "6379", State: "online"})
				f.probe.redis[ip] = state
			case "unreachable voter", "missing monitor":
				f.probe.fail["192.0.2.22"] = true
			case "active election":
				state := f.probe.sentinels["192.0.2.22"]
				state.Master.Flags = "master,failover_in_progress"
				f.probe.sentinels["192.0.2.22"] = state
			case "higher minority epoch":
				state := f.probe.sentinels["192.0.2.22"]
				state.Master.Epoch = "8"
				f.probe.sentinels["192.0.2.22"] = state
			case "duplicate voter":
				state := f.probe.sentinels["192.0.2.22"]
				state.VoterID = f.probe.sentinels["192.0.2.21"].VoterID
				f.probe.sentinels["192.0.2.22"] = state
			case "known target":
				state := f.probe.sentinels["192.0.2.22"]
				state.Replicas = append(state.Replicas, redis.SentinelReplicaObservation{IP: "192.0.2.12", Flags: "slave,s_down"})
				f.probe.sentinels["192.0.2.22"] = state
			case "unhealthy direct replica":
				state := f.probe.sentinels["192.0.2.22"]
				state.Replicas[0].InfoAge = 31000
				f.probe.sentinels["192.0.2.22"] = state
			case "lost data claim", "claim owned by pod":
				pvc, err := f.k8s.CoreV1().PersistentVolumeClaims("default").Get(context.Background(), "data-"+f.target, metav1.GetOptions{})
				require.NoError(t, err)
				if name == "lost data claim" {
					pvc.Status.Phase = corev1.ClaimLost
				} else {
					pvc.OwnerReferences = replacementOwner("Pod", f.target, "replication-2-uid")
				}
				_, err = f.k8s.CoreV1().PersistentVolumeClaims("default").Update(context.Background(), pvc, metav1.UpdateOptions{})
				require.NoError(t, err)
			case "additional claim owned by pod":
				f.updatePod(f.target, func(p *corev1.Pod) {
					p.Spec.Volumes = append(p.Spec.Volumes, corev1.Volume{Name: "extra", VolumeSource: corev1.VolumeSource{PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{ClaimName: "additional-state"}}})
				})
				_, err := f.k8s.CoreV1().PersistentVolumeClaims("default").Create(context.Background(), &corev1.PersistentVolumeClaim{ObjectMeta: metav1.ObjectMeta{Name: "additional-state", Namespace: "default", UID: "additional-claim", OwnerReferences: replacementOwner("Pod", f.target, "replication-2-uid")}, Status: corev1.PersistentVolumeClaimStatus{Phase: corev1.ClaimBound}}, metav1.CreateOptions{})
				require.NoError(t, err)
			case "foreign pod":
				f.updatePod(f.target, func(p *corev1.Pod) { p.OwnerReferences[0].UID = "other-workload" })
			case "generic ephemeral claim":
				f.updatePod(f.target, func(p *corev1.Pod) {
					p.Spec.Volumes = append(p.Spec.Volumes, corev1.Volume{Name: "ephemeral-state", VolumeSource: corev1.VolumeSource{Ephemeral: &corev1.EphemeralVolumeSource{VolumeClaimTemplate: &corev1.PersistentVolumeClaimTemplate{}}}})
					p.Spec.Containers[0].VolumeMounts = append(p.Spec.Containers[0].VolumeMounts, corev1.VolumeMount{Name: "ephemeral-state", MountPath: "/scratch"})
				})
				_, err := f.k8s.CoreV1().PersistentVolumeClaims("default").Create(context.Background(), &corev1.PersistentVolumeClaim{ObjectMeta: metav1.ObjectMeta{Name: f.target + "-ephemeral-state", Namespace: "default", UID: "generic-ephemeral-claim", OwnerReferences: replacementOwner("Pod", f.target, "replication-2-uid")}, Status: corev1.PersistentVolumeClaimStatus{Phase: corev1.ClaimBound}}, metav1.CreateOptions{})
				require.NoError(t, err)
			case "terminating Sentinel":
				f.updatePod(f.rs.GetStatefulSetName()+"-2", func(p *corev1.Pod) { p.DeletionTimestamp = ptr.To(metav1.NewTime(f.now)) })
			case "rolling update":
				sts, err := f.k8s.AppsV1().StatefulSets("default").Get(context.Background(), f.rr.Name, metav1.GetOptions{})
				require.NoError(t, err)
				sts.Status.UpdateRevision = "new-revision"
				_, err = f.k8s.AppsV1().StatefulSets("default").Update(context.Background(), sts, metav1.UpdateOptions{})
				require.NoError(t, err)
			case "unupdated Redis pod":
				f.updatePod(f.target, func(p *corev1.Pod) { p.Labels[appsv1.ControllerRevisionHashLabelKey] = "old-revision" })
			case "unupdated Sentinel pod":
				f.updatePod(f.rs.GetStatefulSetName()+"-2", func(p *corev1.Pod) { p.Labels[appsv1.ControllerRevisionHashLabelKey] = "old-revision" })
			case "missing reader":
				r.APIReader = nil
			case "changed generation":
				f.rr = f.rr.DeepCopy()
				f.rr.Generation++
			case "missing handoff":
				f.rr = f.rr.DeepCopy()
				delete(f.rr.Annotations, common.SentinelOwnershipAnnotation)
			}
			f.run(r)
			f.now = f.now.Add(2 * replacementObservationPeriod)
			f.run(r)
			require.Empty(t, f.deleted)
			require.Zero(t, f.ledger().Attempts)
		})
	}
}

type replacementRoutingHealer struct {
	fakeHealer
	fail       bool
	selections []string
	onFailure  func()
}

func (h *replacementRoutingHealer) SetRedisMasterLabel(_ context.Context, _ string, _ map[string]string, _ *commonapi.ExistingPasswordSecret, _ *commonapi.TLSConfig, selected string) error {
	h.selections = append(h.selections, selected)
	if h.fail {
		if h.onFailure != nil {
			h.onFailure()
		}
		return fmt.Errorf("synthetic routing validation failed after Sentinel selection")
	}
	return nil
}

type failedReplacementReset struct {
	client.Client
	armed    bool
	failRead bool
}

func (c *failedReplacementReset) Get(ctx context.Context, key client.ObjectKey, object client.Object, options ...client.GetOption) error {
	if _, ok := object.(*rrvb2.RedisReplication); ok && c.armed && c.failRead {
		c.armed = false
		return fmt.Errorf("synthetic recovery reset read unavailable")
	}
	return c.Client.Get(ctx, key, object, options...)
}

func (c *failedReplacementReset) Patch(ctx context.Context, object client.Object, patch client.Patch, options ...client.PatchOption) error {
	if _, ok := object.(*rrvb2.RedisReplication); ok && c.armed && !c.failRead {
		c.armed = false
		return fmt.Errorf("synthetic recovery reset patch unavailable")
	}
	return c.Client.Patch(ctx, object, patch, options...)
}

func TestReplacementRecoveryFailedInvalidationNeverReusesInterval(t *testing.T) {
	for _, stage := range []string{"redis", "status"} {
		for _, mode := range []string{"read", "patch"} {
			for _, restart := range []bool{false, true} {
				t.Run(fmt.Sprintf("%s/%s/restart=%t", stage, mode, restart), func(t *testing.T) {
					f := newReplacementFixture(t)
					r := f.reconciler()
					f.run(r)
					unavailable := &failedReplacementReset{Client: f.api, failRead: mode == "read"}
					r.Client, r.APIReader = unavailable, unavailable
					healer := &replacementRoutingHealer{fail: true, onFailure: func() { unavailable.armed = true }}
					r.Healer = healer
					r.RedisReplicationTopology = topologyOf([]string{f.primary}, []string{"replication-1"}, []string{f.target})
					r.RedisReplicationRealMaster = func(context.Context, kubernetes.Interface, *rrvb2.RedisReplication, []string) string {
						return f.primary
					}
					r.StandaloneMaster = func(context.Context, *rrvb2.RedisReplication, *rsvb2.RedisSentinel) (string, error) {
						return f.primary, nil
					}
					f.now = f.now.Add(10 * time.Second)
					var err error
					if stage == "redis" {
						_, err = r.reconcileRedis(context.Background(), f.rr)
					} else {
						_, err = r.reconcileStatus(context.Background(), f.rr)
					}
					require.Error(t, err)
					require.False(t, unavailable.armed, "failure was not injected into observation reset")
					require.NotNil(t, f.ledger().Observation, "fault injection must leave the old persisted record in place")
					f.now = f.now.Add(21 * time.Second)
					if restart {
						r = f.reconciler()
					}
					f.run(r)
					require.Empty(t, f.deleted, "failed invalidation must not permit reuse of the old interval")
					require.Zero(t, f.ledger().Attempts)
					require.WithinDuration(t, f.now, f.ledger().Observation.Since, time.Millisecond)
					f.advance(r, replacementObservationPeriod)
					require.Len(t, f.deleted, 1)
				})
			}
		}
	}
}

func TestReplacementRecoveryObservationDoesNotCrossControllerRestart(t *testing.T) {
	f := newReplacementFixture(t)
	r := f.reconciler()
	f.run(r)
	f.advance(r, replacementObservationPeriod-5*time.Second)
	f.now = f.now.Add(6 * time.Second)
	restarted := f.reconciler()
	f.run(restarted)
	require.Empty(t, f.deleted)
	require.Zero(t, f.ledger().Attempts)
	require.WithinDuration(t, f.now, f.ledger().Observation.Since, time.Millisecond)
	f.advance(restarted, replacementObservationPeriod)
	require.Len(t, f.deleted, 1)
}

func TestReplacementInvalidationIsOwnerBoundAndIndependentOfAuthCache(t *testing.T) {
	f := newReplacementFixture(t)
	r := f.reconciler()
	incarnation, initial, err := r.replacementFence(f.rr, false)
	require.NoError(t, err)
	require.NotEmpty(t, incarnation)
	require.Zero(t, initial)
	other := f.rr.DeepCopy()
	other.Namespace, other.UID = "other-namespace", "other-owner"
	_, _, err = r.replacementFence(f.rr, true)
	require.NoError(t, err)
	r.forgetSentinelAuthOwner(client.ObjectKeyFromObject(f.rr))
	sameIncarnation, invalidated, err := r.replacementFence(f.rr, false)
	require.NoError(t, err)
	require.Equal(t, incarnation, sameIncarnation)
	require.Equal(t, uint64(1), invalidated)
	_, otherInvalidation, err := r.replacementFence(other, false)
	require.NoError(t, err)
	require.Zero(t, otherInvalidation)
	r.forgetReplacementOwner(client.ObjectKeyFromObject(f.rr))
	require.NotContains(t, r.replacementInvalidations, replacementFenceKey{Owner: client.ObjectKeyFromObject(f.rr), UID: f.rr.UID})
}

func TestReplacementRecoveryRoutingFailureResetsBothReconcilePaths(t *testing.T) {
	for _, stage := range []string{"redis", "status"} {
		t.Run(stage, func(t *testing.T) {
			f := newReplacementFixture(t)
			r := f.reconciler()
			f.run(r)
			require.NotNil(t, f.ledger().Observation)
			healer := &replacementRoutingHealer{fail: true}
			r.Healer = healer
			r.RedisReplicationTopology = topologyOf([]string{f.primary}, []string{"replication-1"}, []string{f.target})
			r.RedisReplicationRealMaster = func(context.Context, kubernetes.Interface, *rrvb2.RedisReplication, []string) string {
				return f.primary
			}
			r.StandaloneMaster = func(context.Context, *rrvb2.RedisReplication, *rsvb2.RedisSentinel) (string, error) {
				return f.primary, nil
			}
			var reconcile func(context.Context, *rrvb2.RedisReplication) (ctrl.Result, error)
			if stage == "redis" {
				reconcile = r.reconcileRedis
			} else {
				reconcile = r.reconcileStatus
			}
			f.now = f.now.Add(10 * time.Second)
			_, err := reconcile(context.Background(), f.rr)
			require.ErrorContains(t, err, "synthetic routing validation failed")
			require.Equal(t, []string{f.primary}, healer.selections)
			require.Nil(t, f.ledger().Observation, "a known serving-primary contradiction must discard the earlier positive interval")
			require.Zero(t, f.ledger().Attempts)
			f.now = f.now.Add(21 * time.Second)
			f.run(r)
			require.Empty(t, f.deleted)
			require.WithinDuration(t, f.now, f.ledger().Observation.Since, time.Millisecond)
			f.advance(r, replacementObservationPeriod)
			require.Len(t, f.deleted, 1)
		})
	}
}

func TestReplacementRecoveryRestartsObservationAfterAnElectionOrUIDChange(t *testing.T) {
	for _, mode := range []string{"quorum lost", "new pod UID"} {
		t.Run(mode, func(t *testing.T) {
			f := newReplacementFixture(t)
			r := f.reconciler()
			f.run(r)
			if mode == "quorum lost" {
				f.now = f.now.Add(time.Minute)
				f.probe.fail["192.0.2.22"] = true
				f.run(r)
				require.Nil(t, f.ledger().Observation)
				delete(f.probe.fail, "192.0.2.22")
			} else {
				f.now = f.now.Add(5 * time.Second)
				f.updatePod(f.target, func(p *corev1.Pod) { p.UID = "new-pod-uid"; p.ResourceVersion = "2" })
			}
			f.run(r)
			require.Empty(t, f.deleted)
			require.WithinDuration(t, f.now, f.ledger().Observation.Since, time.Millisecond)
			f.advance(r, replacementObservationPeriod+time.Second)
			require.Len(t, f.deleted, 1)
		})
	}
}

func TestReplacementRecoveryBudgetSurvivesRestart(t *testing.T) {
	f := newReplacementFixture(t)
	state := replacementLedger{Version: 1, Owner: f.rr.UID, Attempts: replacementAttemptLimit}
	require.NoError(t, f.reconciler().storeReplacementLedger(context.Background(), f.rr, state))
	f.run(f.reconciler())
	f.now = f.now.Add(time.Hour)
	f.run(f.reconciler())
	require.Empty(t, f.deleted)
	require.Equal(t, replacementAttemptLimit, f.ledger().Attempts)
}

func TestReplacementRecoverySuccessDoesNotReplenishBudget(t *testing.T) {
	f := newReplacementFixture(t)
	before, err := f.k8s.CoreV1().Pods("default").Get(context.Background(), f.target, metav1.GetOptions{})
	require.NoError(t, err)
	r := f.reconciler()
	f.run(r)
	f.advance(r, replacementObservationPeriod)
	require.Len(t, f.deleted, 1)
	successor := before.DeepCopy()
	successor.UID, successor.ResourceVersion = "recovered-successor", "2"
	successor.Status.Conditions[0].Status = corev1.ConditionTrue
	_, err = f.k8s.CoreV1().Pods("default").Create(context.Background(), successor, metav1.CreateOptions{})
	require.NoError(t, err)
	f.run(f.reconciler())
	require.Nil(t, f.ledger().Pending)
	require.Nil(t, f.ledger().Observation)
	require.Equal(t, 1, f.ledger().Attempts)
}

func TestReplacementRecoveryStopsAfterSecondStrandedUID(t *testing.T) {
	f := newReplacementFixture(t)
	before, err := f.k8s.CoreV1().Pods("default").Get(context.Background(), f.target, metav1.GetOptions{})
	require.NoError(t, err)
	r := f.reconciler()
	require.NoError(t, r.storeReplacementLedger(context.Background(), f.rr, replacementLedger{Version: 1, Owner: f.rr.UID, Attempts: 1}))
	f.run(r)
	f.advance(r, replacementObservationPeriod)
	require.Len(t, f.deleted, 1)
	require.Equal(t, replacementAttemptLimit, f.ledger().Attempts)
	successor := before.DeepCopy()
	successor.UID, successor.ResourceVersion = "still-stranded-successor", "2"
	_, err = f.k8s.CoreV1().Pods("default").Create(context.Background(), successor, metav1.CreateOptions{})
	require.NoError(t, err)
	f.run(f.reconciler())
	f.now = f.now.Add(time.Hour)
	f.run(f.reconciler())
	require.Len(t, f.deleted, 1, "the new stranded UID must not be deleted after the persisted budget is exhausted")
	require.Equal(t, replacementAttemptLimit, f.ledger().Attempts)
}

func TestReplacementRecoveryWaitsForGracefulTermination(t *testing.T) {
	f := newReplacementFixture(t)
	r := f.reconciler()
	f.run(r)
	f.advance(r, replacementObservationPeriod-5*time.Second)
	f.now = f.now.Add(6 * time.Second)
	deletions := 0
	f.k8s.PrependReactor("delete", "pods", func(action k8stesting.Action) (bool, runtime.Object, error) {
		deletion := action.(k8stesting.DeleteAction)
		require.Nil(t, deletion.GetDeleteOptions().GracePeriodSeconds)
		object, err := f.k8s.Tracker().Get(corev1.SchemeGroupVersion.WithResource("pods"), "default", f.target)
		require.NoError(t, err)
		pod := object.(*corev1.Pod).DeepCopy()
		pod.DeletionTimestamp = ptr.To(metav1.NewTime(f.now))
		require.NoError(t, f.k8s.Tracker().Update(corev1.SchemeGroupVersion.WithResource("pods"), pod, "default"))
		deletions++
		return true, nil, nil
	})
	f.run(r)
	f.now = f.now.Add(time.Hour)
	f.run(f.reconciler())
	require.Equal(t, 1, deletions)
	require.Equal(t, 1, f.ledger().Attempts)
	require.NotNil(t, f.ledger().Pending)
}

type failReplacementReservation struct{ client.Client }

func (c failReplacementReservation) Patch(ctx context.Context, object client.Object, patch client.Patch, options ...client.PatchOption) error {
	if rr, ok := object.(*rrvb2.RedisReplication); ok {
		state, err := readReplacementLedger(rr)
		if err == nil && state.Attempts > 0 {
			return apierrors.NewConflict(schema.GroupResource{Group: rrvb2.GroupVersion.Group, Resource: "redisreplications"}, rr.Name, fmt.Errorf("synthetic concurrent budget reservation"))
		}
	}
	return c.Client.Patch(ctx, object, patch, options...)
}

func TestReplacementRecoveryMustPersistReservationBeforeDelete(t *testing.T) {
	f := newReplacementFixture(t)
	r := f.reconciler()
	f.run(r)
	f.advance(r, replacementObservationPeriod-5*time.Second)
	f.now = f.now.Add(6 * time.Second)
	r.Client = failReplacementReservation{Client: f.api}
	require.Error(t, r.reconcileStandaloneReplacement(context.Background(), f.rr, f.rs, f.primary))
	require.Empty(t, f.deleted)
	require.Zero(t, f.ledger().Attempts)
}

func TestReplacementRecoveryRevalidatesAfterReserving(t *testing.T) {
	f := newReplacementFixture(t)
	r := f.reconciler()
	f.run(r)
	f.advance(r, replacementObservationPeriod-5*time.Second)
	f.now = f.now.Add(6 * time.Second)
	reads := 0
	f.probe.onRead = func(ip string) {
		if ip == "192.0.2.10" {
			reads++
			if reads == 2 {
				for voter, state := range f.probe.sentinels {
					state.Master.Epoch = "8"
					f.probe.sentinels[voter] = state
				}
			}
		}
	}
	f.run(r)
	require.Empty(t, f.deleted)
	require.Equal(t, 1, f.ledger().Attempts)
	require.NotNil(t, f.ledger().Pending)
	require.Nil(t, f.ledger().Observation)
	f.probe.onRead = nil
	restarted := f.reconciler()
	f.run(restarted)
	f.advance(restarted, replacementObservationPeriod+time.Second)
	require.Len(t, f.deleted, 1)
	require.Equal(t, 1, f.ledger().Attempts, "resuming the reserved UID must not spend another attempt")
}

func TestReplacementRecoveryDoesNotCountUnobservedTimeAsStability(t *testing.T) {
	for _, mode := range []string{"observation gap", "authority unavailable"} {
		t.Run(mode, func(t *testing.T) {
			f := newReplacementFixture(t)
			r := f.reconciler()
			f.run(r)
			if mode == "observation gap" {
				f.now = f.now.Add(replacementObservationMaxGap + time.Second)
			} else {
				require.NoError(t, r.reconcileStandaloneReplacement(context.Background(), f.rr, f.rs, ""))
				require.Nil(t, f.ledger().Observation)
				f.now = f.now.Add(time.Minute)
			}
			f.run(r)
			require.Empty(t, f.deleted)
			require.WithinDuration(t, f.now, f.ledger().Observation.Since, time.Millisecond)
			f.advance(r, replacementObservationPeriod)
			require.Len(t, f.deleted, 1)
		})
	}
}

func TestReplacementRecoveryAllowsMeasuredRateLimitedReconcileCadence(t *testing.T) {
	f := newReplacementFixture(t)
	r := f.reconciler()
	f.run(r)
	f.now = f.now.Add(29 * time.Second)
	f.run(r)
	require.Empty(t, f.deleted)
	f.now = f.now.Add(29 * time.Second)
	f.run(r)
	require.Len(t, f.deleted, 1)
}

func TestReplacementRecoveryFinalOwnerAndRoleChangesBlockDeletion(t *testing.T) {
	for _, mode := range []string{"specification changed", "target promoted"} {
		t.Run(mode, func(t *testing.T) {
			f := newReplacementFixture(t)
			r := f.reconciler()
			f.run(r)
			f.advance(r, replacementObservationPeriod-5*time.Second)
			f.now = f.now.Add(6 * time.Second)
			primaryReads := 0
			f.probe.onRead = func(ip string) {
				if ip == "192.0.2.10" {
					primaryReads++
					if primaryReads == 2 && mode == "target promoted" {
						state := f.probe.redis["192.0.2.12"]
						state.Role = "master"
						f.probe.redis["192.0.2.12"] = state
					}
				}
				if primaryReads == 2 && ip == "192.0.2.22" && mode == "specification changed" {
					stored := &rrvb2.RedisReplication{}
					require.NoError(t, f.api.Get(context.Background(), client.ObjectKeyFromObject(f.rr), stored))
					stored.Generation++
					require.NoError(t, f.api.Update(context.Background(), stored))
				}
			}
			f.run(r)
			require.Empty(t, f.deleted)
			require.Equal(t, 1, f.ledger().Attempts)
		})
	}
}

func TestReplacementRecoveryUIDPreconditionDoesNotDeleteSuccessor(t *testing.T) {
	f := newReplacementFixture(t)
	r := f.reconciler()
	f.run(r)
	f.advance(r, replacementObservationPeriod-5*time.Second)
	f.now = f.now.Add(6 * time.Second)
	f.k8s.PrependReactor("delete", "pods", func(action k8stesting.Action) (bool, runtime.Object, error) {
		deletion := action.(k8stesting.DeleteAction)
		options := deletion.GetDeleteOptions()
		require.Equal(t, types.UID("replication-2-uid"), *options.Preconditions.UID)
		pod, err := f.k8s.Tracker().Get(corev1.SchemeGroupVersion.WithResource("pods"), "default", f.target)
		require.NoError(t, err)
		successor := pod.(*corev1.Pod).DeepCopy()
		successor.UID, successor.ResourceVersion = "successor-uid", "99"
		require.NoError(t, f.k8s.Tracker().Update(corev1.SchemeGroupVersion.WithResource("pods"), successor, "default"))
		return true, nil, apierrors.NewConflict(schema.GroupResource{Resource: "pods"}, f.target, fmt.Errorf("synthetic UID precondition failure"))
	})
	f.run(r)
	require.Empty(t, f.deleted)
	retained, err := f.k8s.CoreV1().Pods("default").Get(context.Background(), f.target, metav1.GetOptions{})
	require.NoError(t, err)
	require.Equal(t, types.UID("successor-uid"), retained.UID)
}

func TestReplacementRecoveryRejectsMalformedHistory(t *testing.T) {
	f := newReplacementFixture(t)
	for _, encoded := range []string{"not-json", "{}", `{"version":2,"owner":"replication-owner","attempts":0}`, `{"version":1,"owner":"old-owner","attempts":0}`, `{"version":1,"owner":"replication-owner","attempts":-1}`, `{"version":1,"owner":"replication-owner","attempts":3}`, `{"version":1,"owner":"replication-owner","attempts":1,"pending":{"name":"replication-2"}}`} {
		copy := f.rr.DeepCopy()
		copy.Annotations[common.SentinelReplacementRecoveryAnnotation] = encoded
		_, err := readReplacementLedger(copy)
		require.Error(t, err, encoded)
	}
	encoded, err := json.Marshal(replacementLedger{Version: 1, Owner: f.rr.UID, Attempts: 0})
	require.NoError(t, err)
	copy := f.rr.DeepCopy()
	copy.Annotations[common.SentinelReplacementRecoveryAnnotation] = string(encoded)
	_, err = readReplacementLedger(copy)
	require.NoError(t, err)
}
