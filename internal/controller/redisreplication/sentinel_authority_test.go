package redisreplication

import (
	"context"
	"testing"

	commonapi "github.com/OT-CONTAINER-KIT/redis-operator/api/common/v1beta2"
	rrvb2 "github.com/OT-CONTAINER-KIT/redis-operator/api/redisreplication/v1beta2"
	rsvb2 "github.com/OT-CONTAINER-KIT/redis-operator/api/redissentinel/v1beta2"
	"github.com/OT-CONTAINER-KIT/redis-operator/internal/controller/common"
	redishealer "github.com/OT-CONTAINER-KIT/redis-operator/internal/controller/common/redis"
	"github.com/OT-CONTAINER-KIT/redis-operator/internal/service/redis"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes"
	kubernetesfake "k8s.io/client-go/kubernetes/fake"
	"k8s.io/utils/ptr"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	clientfake "sigs.k8s.io/controller-runtime/pkg/client/fake"
)

func emptySentinelClient(t *testing.T) client.Client {
	t.Helper()
	scheme := runtime.NewScheme()
	require.NoError(t, rsvb2.AddToScheme(scheme))
	return clientfake.NewClientBuilder().WithScheme(scheme).Build()
}

func TestSentinelMajorityAndEpoch(t *testing.T) {
	a := redis.SentinelMasterState{IP: "192.0.2.1", Port: "6379", Flags: "master", Epoch: "5"}
	b := a
	b.IP = "192.0.2.2"
	old := b
	old.Epoch = "4"
	newer := b
	newer.Epoch = "6"
	down := a
	down.Flags = "master,s_down"
	electing := old
	electing.Flags = "master,failover_in_progress"
	malformed := a
	malformed.Epoch = "invalid"
	for _, tc := range []struct {
		name    string
		reports []redis.SentinelMasterState
		ok      bool
	}{
		{"unanimous", []redis.SentinelMasterState{a, a, a}, true},
		{"unavailable minority", []redis.SentinelMasterState{a, a}, true},
		{"below majority", []redis.SentinelMasterState{a}, false},
		{"same epoch conflict", []redis.SentinelMasterState{a, a, b}, false},
		{"older monitor recovery candidate", []redis.SentinelMasterState{a, a, old}, true},
		{"newer minority", []redis.SentinelMasterState{a, a, newer}, false},
		{"same epoch unhealthy", []redis.SentinelMasterState{a, a, down}, false},
		{"active election", []redis.SentinelMasterState{a, a, electing}, false},
		{"malformed report", []redis.SentinelMasterState{a, a, malformed}, false},
		{"more than configured", []redis.SentinelMasterState{a, a, a, a}, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			d, ok := agreedSentinelMaster(tc.reports, 3)
			require.Equal(t, tc.ok, ok)
			if ok {
				require.Equal(t, a.IP, d.IP)
				require.Equal(t, uint64(5), d.Epoch)
			}
		})
	}
}

func TestSurvivingMonitorTopologyAloneCannotGrantInitialization(t *testing.T) {
	placeholder := redis.SentinelMasterState{IP: "0.0.0.0", Port: "6379", Flags: "master,s_down,disconnected", Epoch: "0"}
	primary := placeholder
	primary.IP = "192.0.2.1"
	primary.Flags = "master"
	elected := primary
	elected.Epoch = "1"
	conflict := primary
	conflict.IP = "192.0.2.2"
	active := placeholder
	active.Flags = "master,failover_in_progress"
	for _, tc := range []struct {
		name    string
		reports []redis.SentinelMasterState
		ok      bool
	}{
		{"all missing", nil, true},
		{"existing placeholder is not absent", []redis.SentinelMasterState{placeholder}, false},
		{"healthy surviving seed", []redis.SentinelMasterState{primary, primary}, true},
		{"surviving healthy election", []redis.SentinelMasterState{elected}, true},
		{"conflicting address", []redis.SentinelMasterState{conflict}, false},
		{"conflicting epochs", []redis.SentinelMasterState{primary, elected}, false},
		{"active election", []redis.SentinelMasterState{active}, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			require.Equal(t, tc.ok, monitorRecoveryTopologyMatches(tc.reports, primary.IP))
			if len(tc.reports) > 0 {
				require.False(t, initialMonitorInitializationAllowed(&rrvb2.RedisReplication{}, len(tc.reports)))
			}
		})
	}
}

func TestLinkedSentinelRejectsMultipleAuthorities(t *testing.T) {
	scheme := runtime.NewScheme()
	require.NoError(t, rsvb2.AddToScheme(scheme))
	rs := &rsvb2.RedisSentinel{ObjectMeta: metav1.ObjectMeta{Name: "sentinel-a", Namespace: "redis"}, Spec: rsvb2.RedisSentinelSpec{RedisSentinelConfig: &rsvb2.RedisSentinelConfig{RedisSentinelConfig: commonapi.RedisSentinelConfig{RedisReplicationName: "replication"}}}}
	second := rs.DeepCopy()
	second.Name = "sentinel-b"
	r := &Reconciler{Client: clientfake.NewClientBuilder().WithScheme(scheme).WithObjects(rs, second).Build()}
	_, err := r.findLinkedSentinel(context.Background(), &rrvb2.RedisReplication{ObjectMeta: metav1.ObjectMeta{Name: "replication", Namespace: "redis"}})
	require.ErrorContains(t, err, "multiple Sentinel authorities")
}

func TestStandaloneAuthorityRejectsUnsupportedConnections(t *testing.T) {
	r := &Reconciler{}
	rs := &rsvb2.RedisSentinel{Spec: rsvb2.RedisSentinelSpec{Size: ptr.To(int32(3)), TLS: &commonapi.TLSConfig{}, RedisSentinelConfig: &rsvb2.RedisSentinelConfig{}}}
	_, _, err := r.sentinelSelectedMaster(context.Background(), &rrvb2.RedisReplication{}, rs)
	require.Error(t, err)
}

func TestStandaloneElectionNeverAuthorizesOperatorDemotion(t *testing.T) {
	scheme := runtime.NewScheme()
	require.NoError(t, rrvb2.AddToScheme(scheme))
	require.NoError(t, rsvb2.AddToScheme(scheme))
	rr := newReplicationInstanceForTest()
	rr.Status.MasterNode = rr.Name + "-0"
	rr.Status.ConnectionInfo = &rrvb2.ConnectionInfo{Host: "synthetic.invalid", Port: 6379}
	rs := &rsvb2.RedisSentinel{ObjectMeta: metav1.ObjectMeta{Name: "sentinel", Namespace: rr.Namespace}, Spec: rsvb2.RedisSentinelSpec{RedisSentinelConfig: &rsvb2.RedisSentinelConfig{RedisSentinelConfig: commonapi.RedisSentinelConfig{RedisReplicationName: rr.Name}}}}
	mutationCalls := 0
	r := &Reconciler{Client: clientfake.NewClientBuilder().WithScheme(scheme).WithStatusSubresource(rr).WithObjects(rr, rs).Build(), K8sClient: kubernetesfake.NewSimpleClientset(), Healer: &fakeHealer{}, RedisReplicationTopology: topologyOf([]string{rr.Name + "-0", rr.Name + "-1"}, []string{rr.Name + "-2"}, nil), RedisReplicationRealMaster: func(context.Context, kubernetes.Interface, *rrvb2.RedisReplication, []string) string {
		return rr.Name + "-0"
	}, StandaloneMaster: func(context.Context, *rrvb2.RedisReplication, *rsvb2.RedisSentinel) (string, error) {
		return rr.Name + "-0", nil
	}, CreateRedisReplicationLink: func(context.Context, kubernetes.Interface, *rrvb2.RedisReplication, []string, string) error {
		mutationCalls++
		return nil
	}}
	_, err := r.reconcileRedis(context.Background(), rr)
	require.NoError(t, err)
	require.Zero(t, mutationCalls, "even a sampled old selection must never enter running-member role repair")
}

func TestOnlyProtectedUnstartedMembersPermitMonitorRecovery(t *testing.T) {
	pod := &corev1.Pod{Spec: corev1.PodSpec{InitContainers: []corev1.Container{{Name: "init-config", Args: []string{"bootstrap", "--standalone-authority"}, Env: []corev1.EnvVar{{Name: "SENTINEL_BOOTSTRAP_REQUIRED", Value: "true"}}}}}, Status: corev1.PodStatus{Phase: corev1.PodPending}}
	require.True(t, protectedMemberInitializing(pod))
	legacy := pod.DeepCopy()
	legacy.Spec.InitContainers[0].Args = []string{"bootstrap"}
	require.False(t, protectedMemberInitializing(legacy))
	running := pod.DeepCopy()
	running.Status.ContainerStatuses = []corev1.ContainerStatus{{State: corev1.ContainerState{Running: &corev1.ContainerStateRunning{}}}}
	require.False(t, protectedMemberInitializing(running))
	previous := pod.DeepCopy()
	previous.Status.ContainerStatuses = []corev1.ContainerStatus{{LastTerminationState: corev1.ContainerState{Terminated: &corev1.ContainerStateTerminated{ExitCode: 0}}}}
	require.False(t, protectedMemberInitializing(previous))
	completed := pod.DeepCopy()
	completed.Status.InitContainerStatuses = []corev1.ContainerStatus{{Name: "init-config", State: corev1.ContainerState{Terminated: &corev1.ContainerStateTerminated{ExitCode: 0}}}}
	require.False(t, protectedMemberInitializing(completed))
	unknown := pod.DeepCopy()
	unknown.Status.Phase = corev1.PodUnknown
	require.False(t, protectedMemberInitializing(unknown))
	require.False(t, protectedMemberInitializing(nil))
}

func TestFullReconcileWithdrawsRoutesWhenDesiredTemplateIsRejected(t *testing.T) {
	scheme := runtime.NewScheme()
	require.NoError(t, rrvb2.AddToScheme(scheme))
	require.NoError(t, rsvb2.AddToScheme(scheme))
	rr := newReplicationInstanceForTest()
	rr.Finalizers = []string{RedisReplicationFinalizer}
	rr.Annotations = map[string]string{common.SentinelStartupAnnotation: "enabled"}
	rr.Status.MasterNode = rr.Name + "-0"
	rr.Status.ConnectionInfo = &rrvb2.ConnectionInfo{Host: "synthetic.invalid", Port: 6379}
	rs := &rsvb2.RedisSentinel{ObjectMeta: metav1.ObjectMeta{Name: "sentinel-a", Namespace: rr.Namespace}, Spec: rsvb2.RedisSentinelSpec{Size: ptr.To(int32(3)), RedisSentinelConfig: &rsvb2.RedisSentinelConfig{RedisSentinelConfig: commonapi.RedisSentinelConfig{RedisReplicationName: rr.Name}}}}
	other := rs.DeepCopy()
	other.Name = "sentinel-b"
	labels := common.GetRedisLabels(rr.Name, common.SetupTypeReplication, "replication", nil)
	labels[common.RedisRoleLabelKey] = "master"
	pod := &corev1.Pod{ObjectMeta: metav1.ObjectMeta{Name: rr.Name + "-0", Namespace: rr.Namespace, Labels: labels}}
	k8s := kubernetesfake.NewSimpleClientset(pod)
	api := clientfake.NewClientBuilder().WithScheme(scheme).WithStatusSubresource(rr).WithObjects(rr, rs, other).Build()
	r := &Reconciler{Client: api, K8sClient: k8s, Healer: redishealer.NewHealer(k8s)}
	_, err := r.Reconcile(context.Background(), ctrl.Request{NamespacedName: types.NamespacedName{Name: rr.Name, Namespace: rr.Namespace}})
	require.NoError(t, err)
	stored := &rrvb2.RedisReplication{}
	require.NoError(t, api.Get(context.Background(), client.ObjectKeyFromObject(rr), stored))
	require.Empty(t, stored.Status.MasterNode)
	current, err := k8s.CoreV1().Pods(rr.Namespace).Get(context.Background(), pod.Name, metav1.GetOptions{})
	require.NoError(t, err)
	require.NotEqual(t, "master", current.Labels[common.RedisRoleLabelKey])
	statefulsets, err := k8s.AppsV1().StatefulSets(rr.Namespace).List(context.Background(), metav1.ListOptions{})
	require.NoError(t, err)
	require.Empty(t, statefulsets.Items)
}

func TestSentinelOwnershipCannotReenterBootstrap(t *testing.T) {
	scheme := runtime.NewScheme()
	require.NoError(t, rrvb2.AddToScheme(scheme))
	rr := newReplicationInstanceForTest()
	r := &Reconciler{Client: clientfake.NewClientBuilder().WithScheme(scheme).WithObjects(rr).Build()}
	require.NoError(t, r.recordSentinelOwnership(context.Background(), rr, "bootstrap"))
	require.NoError(t, r.recordSentinelOwnership(context.Background(), rr, "sentinel"))
	require.Error(t, r.recordSentinelOwnership(context.Background(), rr, "bootstrap"))
	require.Equal(t, "sentinel", rr.Annotations[common.SentinelOwnershipAnnotation])
}

type staleOwnershipClient struct {
	client.Client
	snapshot *rrvb2.RedisReplication
}

func (c *staleOwnershipClient) Get(ctx context.Context, key client.ObjectKey, obj client.Object, options ...client.GetOption) error {
	if rr, ok := obj.(*rrvb2.RedisReplication); ok && key == client.ObjectKeyFromObject(c.snapshot) {
		*rr = *c.snapshot.DeepCopy()
		return nil
	}
	return c.Client.Get(ctx, key, obj, options...)
}

func TestStaleCachedBootstrapOwnershipCannotAuthorizeDemotion(t *testing.T) {
	scheme := runtime.NewScheme()
	require.NoError(t, rrvb2.AddToScheme(scheme))
	require.NoError(t, rsvb2.AddToScheme(scheme))
	live := newReplicationInstanceForTest()
	live.UID = "same-replication"
	live.Generation = 1
	live.Finalizers = []string{RedisReplicationFinalizer}
	live.Annotations = map[string]string{common.SentinelOwnershipAnnotation: "sentinel"}
	live.Status.MasterNode = live.Name + "-0"
	live.Status.ConnectionInfo = &rrvb2.ConnectionInfo{Host: "synthetic.invalid", Port: 6379}
	stale := live.DeepCopy()
	stale.Annotations[common.SentinelOwnershipAnnotation] = "bootstrap"
	rs := &rsvb2.RedisSentinel{ObjectMeta: metav1.ObjectMeta{Name: "sentinel", Namespace: live.Namespace}, Spec: rsvb2.RedisSentinelSpec{Size: ptr.To(int32(3)), RedisSentinelConfig: &rsvb2.RedisSentinelConfig{RedisSentinelConfig: commonapi.RedisSentinelConfig{RedisReplicationName: live.Name}}}}
	api := clientfake.NewClientBuilder().WithScheme(scheme).WithStatusSubresource(live).WithObjects(live, rs).Build()
	cached := &staleOwnershipClient{Client: api, snapshot: stale}
	mutations := 0
	r := &Reconciler{Client: cached, APIReader: api, K8sClient: kubernetesfake.NewSimpleClientset(), Healer: &fakeHealer{}, RedisReplicationTopology: topologyOf([]string{live.Name + "-0", live.Name + "-1"}, []string{live.Name + "-2"}, nil), RedisReplicationRealMaster: func(context.Context, kubernetes.Interface, *rrvb2.RedisReplication, []string) string {
		return live.Name + "-0"
	}, StandaloneMaster: func(_ context.Context, rr *rrvb2.RedisReplication, _ *rsvb2.RedisSentinel) (string, error) {
		require.Equal(t, "sentinel", rr.Annotations[common.SentinelOwnershipAnnotation])
		return live.Name + "-0", nil
	}, CreateRedisReplicationLink: func(context.Context, kubernetes.Interface, *rrvb2.RedisReplication, []string, string) error {
		mutations++
		return nil
	}}
	_, err := r.Reconcile(context.Background(), ctrl.Request{NamespacedName: client.ObjectKeyFromObject(live)})
	require.NoError(t, err)
	require.Zero(t, mutations, "persisted handoff must win over a stale bootstrap cache entry")
	current := &rrvb2.RedisReplication{}
	require.NoError(t, api.Get(context.Background(), client.ObjectKeyFromObject(live), current))
	require.Equal(t, "sentinel", current.Annotations[common.SentinelOwnershipAnnotation])
}

func TestSentinelVotesRequireActualDurableConfigMounts(t *testing.T) {
	mount := corev1.VolumeMount{Name: common.VolumeNameSentinelState, MountPath: "/etc/redis"}
	pod := corev1.Pod{ObjectMeta: metav1.ObjectMeta{Name: "sentinel-0"}, Spec: corev1.PodSpec{Volumes: []corev1.Volume{{Name: common.VolumeNameSentinelState, VolumeSource: corev1.VolumeSource{PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{ClaimName: "sentinel-state-sentinel-0"}}}}, Containers: []corev1.Container{{Name: "sentinel", VolumeMounts: []corev1.VolumeMount{mount}}}, InitContainers: []corev1.Container{{Name: "init-config", Args: []string{"bootstrap", "--persistent-sentinel-state"}, Env: []corev1.EnvVar{{Name: "PERSIST_SENTINEL_CONFIG", Value: "true"}}, VolumeMounts: []corev1.VolumeMount{mount}}}}}
	require.True(t, persistentSentinelPod(pod, "sentinel"))
	legacy := pod.DeepCopy()
	legacy.Spec.Volumes[0].VolumeSource = corev1.VolumeSource{EmptyDir: &corev1.EmptyDirVolumeSource{}}
	require.False(t, persistentSentinelPod(*legacy, "sentinel"))
	oldAgent := pod.DeepCopy()
	oldAgent.Spec.InitContainers[0].Args = []string{"bootstrap", "--standalone-authority"}
	require.False(t, persistentSentinelPod(*oldAgent, "sentinel"))
	wrongClaim := pod.DeepCopy()
	wrongClaim.Spec.Volumes[0].PersistentVolumeClaim.ClaimName = "shared-state"
	require.False(t, persistentSentinelPod(*wrongClaim, "sentinel"))
}
