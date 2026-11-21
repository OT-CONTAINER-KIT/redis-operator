package k8sutils

import (
	"context"
	"strconv"
	"testing"

	rcvb2 "github.com/OT-CONTAINER-KIT/redis-operator/api/rediscluster/v1beta2"
	"github.com/go-redis/redismock/v9"
	redis "github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/assert"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	k8sClientFake "k8s.io/client-go/kubernetes/fake"
	"k8s.io/utils/ptr"
)

const (
	// rejoinTestPreviousID is the ID the pod had before it was recreated; the
	// live cluster still lists it as a failed replica of rejoinTestMasterID.
	rejoinTestPreviousID = "5f1c2d3e4a5b6c7d8e9f0a1b2c3d4e5f60718a9b"

	rejoinTestNamespace = "redis-lab"
	rejoinTestCluster   = "redis-cluster"

	// rejoinTestOrphanID is the node ID of the pod that was recreated and came
	// back as a standalone master; rejoinTestMasterID is the master that was
	// promoted while it was gone.
	rejoinTestOrphanID = "0b6b30f6c8d7e2a2c0f7a20e50b4b9a80ee0d1f4"
	rejoinTestMasterID = "3c9c8a1d6b5f4e2a9d7c0b8a6f5e4d3c2b1a0f9e"

	rejoinTestFormedInfo = "cluster_state:ok\r\n" +
		"cluster_slots_assigned:16384\r\n" +
		"cluster_slots_pfail:0\r\n" +
		"cluster_slots_fail:0\r\n" +
		"cluster_known_nodes:6\r\n" +
		"cluster_size:3\r\n"

	rejoinTestIsolatedInfo = "cluster_state:ok\r\n" +
		"cluster_slots_assigned:1\r\n" +
		"cluster_slots_pfail:0\r\n" +
		"cluster_slots_fail:0\r\n" +
		"cluster_known_nodes:1\r\n" +
		"cluster_size:1\r\n"
)

func rejoinTestPod(name, ip string) *corev1.Pod {
	return &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: rejoinTestNamespace,
		},
		Status: corev1.PodStatus{
			Phase: corev1.PodRunning,
			PodIP: ip,
			Conditions: []corev1.PodCondition{
				{Type: corev1.PodReady, Status: corev1.ConditionTrue},
			},
		},
	}
}

func rejoinTestClusterSpec() *rcvb2.RedisCluster {
	return &rcvb2.RedisCluster{
		ObjectMeta: metav1.ObjectMeta{
			Name:      rejoinTestCluster,
			Namespace: rejoinTestNamespace,
		},
		Spec: rcvb2.RedisClusterSpec{
			ClusterSize: ptr.To(int32(3)),
			Port:        ptr.To(6379),
		},
	}
}

func TestClusterMembershipFromInfo(t *testing.T) {
	tests := []struct {
		name string
		info string
		want clusterMembership
	}{
		{
			name: "complete cluster",
			info: rejoinTestFormedInfo,
			want: clusterMembershipFormed,
		},
		{
			name: "standalone node",
			info: rejoinTestIsolatedInfo,
			want: clusterMembershipIsolated,
		},
		{
			name: "standalone node without slots",
			info: "cluster_state:fail\ncluster_slots_assigned:0\ncluster_known_nodes:1\n",
			want: clusterMembershipIsolated,
		},
		{
			name: "single node cluster that owns every slot",
			info: "cluster_state:ok\ncluster_slots_assigned:16384\ncluster_known_nodes:1\n",
			want: clusterMembershipFormed,
		},
		{
			name: "partially created cluster is left alone",
			info: "cluster_state:fail\ncluster_slots_assigned:5461\ncluster_known_nodes:3\n",
			want: clusterMembershipUnknown,
		},
		{
			name: "unparsable output",
			info: "cluster_state:ok\n",
			want: clusterMembershipUnknown,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, clusterMembershipFromInfo(tt.info))
		})
	}
}

func TestReplicaMasterID(t *testing.T) {
	nodes := []clusterNodesResponse{
		{rejoinTestMasterID, "10.0.0.10:6379@16379", "master", "-", "0", "0", "3", "connected", "0-5460"},
		{rejoinTestOrphanID, "10.0.0.93:6379@16379", "slave,fail", rejoinTestMasterID, "0", "0", "1", "disconnected"},
		{"dangling", "10.0.0.94:6379@16379", "slave", "-", "0", "0", "1", "connected"},
	}

	t.Run("failed replica keeps its master", func(t *testing.T) {
		masterID, ok := replicaMasterID(nodes, rejoinTestOrphanID)
		assert.True(t, ok)
		assert.Equal(t, rejoinTestMasterID, masterID)
	})

	t.Run("master is not a replica", func(t *testing.T) {
		_, ok := replicaMasterID(nodes, rejoinTestMasterID)
		assert.False(t, ok)
	})

	t.Run("unknown node", func(t *testing.T) {
		_, ok := replicaMasterID(nodes, "0000000000000000000000000000000000000000")
		assert.False(t, ok)
	})

	t.Run("replica without a master", func(t *testing.T) {
		_, ok := replicaMasterID(nodes, "dangling")
		assert.False(t, ok)
	})
}

func TestParseSlotTokens(t *testing.T) {
	tests := []struct {
		name   string
		tokens []string
		want   []int
	}{
		{name: "single slot", tokens: []string{"4655"}, want: []int{4655}},
		{name: "range", tokens: []string{"0-3"}, want: []int{0, 1, 2, 3}},
		{name: "mixed", tokens: []string{"0-1", "5460"}, want: []int{0, 1, 5460}},
		{name: "migration markers belong to another node", tokens: []string{"[4655->-a]", "[1-<-b]"}, want: nil},
		{name: "unparsable tokens", tokens: []string{"x", "1-y", "-1", "16384"}, want: nil},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, parseSlotTokens(tt.tokens))
		})
	}
}

func TestNodeSlots(t *testing.T) {
	node := clusterNodesResponse{"id", "10.0.0.9:6379@16379", "myself,master", "-", "0", "0", "1", "connected", "4655", "6000-6001"}
	assert.Equal(t, []int{4655, 6000, 6001}, nodeSlots(node))

	empty := clusterNodesResponse{"id", "10.0.0.9:6379@16379", "myself,master", "-", "0", "0", "1", "connected"}
	assert.Nil(t, nodeSlots(empty))
}

// TestRejoinStaleLeaderNodesRejoinsOrphan reproduces the state from a single
// deleted master pod: the recreated pod is a standalone master owning one slot,
// while the remaining nodes keep serving and still list it as a failed replica
// of the master that was promoted in its place. The pod must be reset, met into
// the live cluster and pointed back at that master instead of the operator
// trying `--cluster create` against it.
func TestRejoinStaleLeaderNodesRejoinsOrphan(t *testing.T) {
	ctx := context.Background()

	orphanClient, orphanMock := redismock.NewClientMock()
	orphanMock.ExpectClusterInfo().SetVal(rejoinTestIsolatedInfo)
	orphanMock.ExpectClusterNodes().SetVal(
		rejoinTestOrphanID + " 10.0.0.9:6379@16379 myself,master - 0 0 1 connected 4655\n",
	)
	// A pod recreated with its PVC keeps both the stale slot claim and the stale
	// replica dataset; all three commands are required before it can replicate.
	orphanMock.ExpectDBSize().SetVal(42)
	orphanMock.ExpectFlushAll().SetVal("OK")
	orphanMock.ExpectClusterNodes().SetVal(
		rejoinTestOrphanID + " 10.0.0.9:6379@16379 myself,master - 0 0 1 connected 4655\n",
	)
	orphanMock.ExpectClusterDelSlots(4655).SetVal("OK")
	orphanMock.ExpectClusterNodes().SetVal(
		rejoinTestOrphanID + " 10.0.0.9:6379@16379 myself,master - 0 0 1 connected\n",
	)
	orphanMock.ExpectClusterNodes().SetVal(
		rejoinTestOrphanID + " 10.0.0.9:6379@16379 myself,master - 0 0 1 connected\n" +
			rejoinTestMasterID + " 10.0.0.10:6379@16379 master - 0 0 3 connected 0-5460\n",
	)
	orphanMock.ExpectClusterReplicate(rejoinTestMasterID).SetVal("OK")
	// Every node that still lists the previous incarnation has to forget it,
	// including the reattached pod itself.
	orphanMock.ExpectClusterForget(rejoinTestPreviousID).SetVal("OK")

	formedNodes := rejoinTestMasterID + " 10.0.0.10:6379@16379," + rejoinTestCluster + "-leader-1 master - 0 0 3 connected 0-5460\n" +
		rejoinTestPreviousID + " 10.0.0.93:6379@16379," + rejoinTestCluster + "-leader-0 slave,fail " + rejoinTestMasterID + " 0 0 1 disconnected\n"

	seedClient, seedMock := redismock.NewClientMock()
	seedMock.ExpectClusterInfo().SetVal(rejoinTestFormedInfo)
	seedMock.ExpectClusterNodes().SetVal(formedNodes)
	seedMock.ExpectClusterMeet("10.0.0.9", "6379").SetVal("OK")
	seedMock.ExpectClusterForget(rejoinTestPreviousID).SetVal("OK")

	peerClient, peerMock := redismock.NewClientMock()
	peerMock.ExpectClusterInfo().SetVal(rejoinTestFormedInfo)

	k8sClient := k8sClientFake.NewSimpleClientset(
		rejoinTestPod(rejoinTestCluster+"-leader-0", "10.0.0.9"),
		rejoinTestPod(rejoinTestCluster+"-leader-1", "10.0.0.10"),
		rejoinTestPod(rejoinTestCluster+"-leader-2", "10.0.0.11"),
	)

	formed, repaired, err := rejoinStaleNodes(ctx, k8sClient, rejoinTestClusterSpec(), func(podName string) *redis.Client {
		switch podName {
		case rejoinTestCluster + "-leader-0":
			return orphanClient
		case rejoinTestCluster + "-leader-1":
			return seedClient
		case rejoinTestCluster + "-leader-2":
			return peerClient
		}
		t.Errorf("unexpected client requested for pod %s", podName)
		return nil
	})

	assert.NoError(t, err)
	assert.True(t, formed, "a complete cluster exists, so the caller must not run cluster create")
	assert.Equal(t, 1, repaired)
	assert.NoError(t, orphanMock.ExpectationsWereMet())
	assert.NoError(t, seedMock.ExpectationsWereMet())
	assert.NoError(t, peerMock.ExpectationsWereMet())
}

// TestRejoinStaleFollowerPod covers the same identity loss on a follower pod: a
// rollout replaces every pod, so a replica that comes back standalone has to be
// attached back to its shard as well.
func TestRejoinStaleFollowerPod(t *testing.T) {
	ctx := context.Background()

	replicaClient, replicaMock := redismock.NewClientMock()
	replicaMock.ExpectClusterInfo().SetVal(rejoinTestIsolatedInfo)
	replicaMock.ExpectClusterNodes().SetVal(
		rejoinTestOrphanID + " 10.0.0.30:6379@16379 myself,master - 0 0 0 connected\n",
	)
	replicaMock.ExpectDBSize().SetVal(3)
	replicaMock.ExpectFlushAll().SetVal("OK")
	replicaMock.ExpectClusterNodes().SetVal(
		rejoinTestOrphanID + " 10.0.0.30:6379@16379 myself,master - 0 0 0 connected\n",
	)
	replicaMock.ExpectClusterNodes().SetVal(
		rejoinTestOrphanID + " 10.0.0.30:6379@16379 myself,master - 0 0 0 connected\n" +
			rejoinTestMasterID + " 10.0.0.10:6379@16379 master - 0 0 3 connected 0-5460\n",
	)
	replicaMock.ExpectClusterReplicate(rejoinTestMasterID).SetVal("OK")
	replicaMock.ExpectClusterForget(rejoinTestPreviousID).SetVal("OK")

	peerClient, peerMock := redismock.NewClientMock()
	peerMock.ExpectClusterInfo().SetVal(rejoinTestFormedInfo)
	peerMock.ExpectClusterNodes().SetVal(
		rejoinTestMasterID + " 10.0.0.10:6379@16379," + rejoinTestCluster + "-leader-1 master - 0 0 3 connected 0-5460\n" +
			rejoinTestPreviousID + " 10.0.0.30:6379@16379," + rejoinTestCluster + "-follower-1 slave,fail " + rejoinTestMasterID + " 0 0 1 connected\n",
	)
	peerMock.ExpectClusterMeet("10.0.0.30", "6379").SetVal("OK")
	peerMock.ExpectClusterForget(rejoinTestPreviousID).SetVal("OK")

	k8sClient := k8sClientFake.NewSimpleClientset(
		rejoinTestPod(rejoinTestCluster+"-leader-1", "10.0.0.10"),
		rejoinTestPod(rejoinTestCluster+"-follower-1", "10.0.0.30"),
	)

	formed, repaired, err := rejoinStaleNodes(ctx, k8sClient, rejoinTestClusterSpec(), func(podName string) *redis.Client {
		switch podName {
		case rejoinTestCluster + "-leader-1":
			return peerClient
		case rejoinTestCluster + "-follower-1":
			return replicaClient
		}
		t.Errorf("unexpected client requested for pod %s", podName)
		return nil
	})

	assert.NoError(t, err)
	assert.True(t, formed)
	assert.Equal(t, 1, repaired)
	assert.NoError(t, replicaMock.ExpectationsWereMet())
	assert.NoError(t, peerMock.ExpectationsWereMet())
}

// TestRejoinStaleLeaderNodesSkipsUnknownNode ensures a pod the cluster no longer
// knows is reported as orphaned but left untouched: attaching an unknown node
// would change the topology the user asked for.
func TestRejoinStaleLeaderNodesSkipsUnknownNode(t *testing.T) {
	ctx := context.Background()

	orphanClient, orphanMock := redismock.NewClientMock()
	orphanMock.ExpectClusterInfo().SetVal(rejoinTestIsolatedInfo)
	orphanMock.ExpectClusterNodes().SetVal(
		rejoinTestOrphanID + " 10.0.0.9:6379@16379 myself,master - 0 0 1 connected\n",
	)

	seedClient, seedMock := redismock.NewClientMock()
	seedMock.ExpectClusterInfo().SetVal(rejoinTestFormedInfo)
	seedMock.ExpectClusterNodes().SetVal(
		rejoinTestMasterID + " 10.0.0.10:6379@16379 master - 0 0 3 connected 0-5460\n",
	)

	k8sClient := k8sClientFake.NewSimpleClientset(
		rejoinTestPod(rejoinTestCluster+"-leader-0", "10.0.0.9"),
		rejoinTestPod(rejoinTestCluster+"-leader-1", "10.0.0.10"),
	)

	formed, repaired, err := rejoinStaleNodes(ctx, k8sClient, &rcvb2.RedisCluster{
		ObjectMeta: metav1.ObjectMeta{Name: rejoinTestCluster, Namespace: rejoinTestNamespace},
		Spec: rcvb2.RedisClusterSpec{
			// Only two leader pods exist in this case.
			ClusterSize: ptr.To(int32(2)),
			Port:        ptr.To(6379),
		},
	}, func(podName string) *redis.Client {
		switch podName {
		case rejoinTestCluster + "-leader-0":
			return orphanClient
		case rejoinTestCluster + "-leader-1":
			return seedClient
		}
		t.Errorf("unexpected client requested for pod %s", podName)
		return nil
	})

	// The pod cannot be attached anywhere, so the pass has to say so: the
	// caller raises an event and keeps the cluster create out of the picture.
	assert.Error(t, err)
	assert.True(t, formed)
	assert.Equal(t, 0, repaired)
	assert.NoError(t, orphanMock.ExpectationsWereMet(), "an unknown node must not be reset, met or replicated")
	assert.NoError(t, seedMock.ExpectationsWereMet())
}

// TestRejoinStaleLeaderNodesNoFormedCluster covers a fresh install: every pod is
// a standalone node, so nothing is repaired and the caller is free to create the
// cluster.
func TestRejoinStaleLeaderNodesNoFormedCluster(t *testing.T) {
	ctx := context.Background()

	clients := map[string]*redis.Client{}
	k8sObjects := []runtime.Object{}
	for i := 0; i < 3; i++ {
		podName := rejoinTestCluster + "-leader-" + strconv.Itoa(i)
		client, mock := redismock.NewClientMock()
		mock.ExpectClusterInfo().SetVal(rejoinTestIsolatedInfo)
		clients[podName] = client
		k8sObjects = append(k8sObjects, rejoinTestPod(podName, "10.0.0."+strconv.Itoa(i+9)))
	}

	formed, repaired, err := rejoinStaleNodes(ctx, k8sClientFake.NewSimpleClientset(k8sObjects...), rejoinTestClusterSpec(), func(podName string) *redis.Client {
		return clients[podName]
	})

	assert.NoError(t, err)
	assert.False(t, formed)
	assert.Equal(t, 0, repaired)
}

func TestShardPartnerPods(t *testing.T) {
	cr := rejoinTestClusterSpec()

	assert.Equal(t, []string{"redis-cluster-follower-0"}, shardPartnerPods(cr, "redis-cluster-leader-0"))
	assert.Equal(t, []string{"redis-cluster-leader-1"}, shardPartnerPods(cr, "redis-cluster-follower-4"))
	assert.Empty(t, shardPartnerPods(cr, "other-pod"))
}

func TestPlanStaleRejoin(t *testing.T) {
	cr := rejoinTestClusterSpec()
	master := clusterNodesResponse{
		rejoinTestMasterID, "10.0.0.10:6379@16379,redis-cluster-leader-1",
		"master", "-", "0", "0", "3", "connected", "0-5460",
	}
	promotedReplica := clusterNodesResponse{
		rejoinTestMasterID, "10.0.0.20:6379@16379,redis-cluster-follower-0",
		"master", "-", "0", "0", "4", "connected", "0-5460",
	}

	t.Run("previous incarnation of the same pod", func(t *testing.T) {
		nodes := []clusterNodesResponse{
			master,
			{rejoinTestPreviousID, "10.0.0.93:6379@16379,redis-cluster-leader-0", "slave,fail", rejoinTestMasterID, "0", "0", "1", "disconnected"},
		}
		plan, err := planStaleRejoin(nodes, cr, "redis-cluster-leader-0", rejoinTestOrphanID)
		assert.NoError(t, err)
		assert.Equal(t, rejoinTestMasterID, plan.masterID)
		assert.Equal(t, []string{rejoinTestPreviousID}, plan.forgetIDs)
	})

	t.Run("entry under the current node ID is not forgotten", func(t *testing.T) {
		nodes := []clusterNodesResponse{
			master,
			{rejoinTestOrphanID, "10.0.0.93:6379@16379,redis-cluster-leader-0", "slave,fail", rejoinTestMasterID, "0", "0", "1", "disconnected"},
		}
		plan, err := planStaleRejoin(nodes, cr, "redis-cluster-leader-0", rejoinTestOrphanID)
		assert.NoError(t, err)
		assert.Equal(t, rejoinTestMasterID, plan.masterID)
		assert.Empty(t, plan.forgetIDs)
	})

	t.Run("forgotten pod is found through its shard", func(t *testing.T) {
		plan, err := planStaleRejoin([]clusterNodesResponse{promotedReplica}, cr, "redis-cluster-leader-0", rejoinTestOrphanID)
		assert.NoError(t, err)
		assert.Equal(t, rejoinTestMasterID, plan.masterID)
		assert.Empty(t, plan.forgetIDs)
	})

	t.Run("entry without a hostname falls back to the shard", func(t *testing.T) {
		nodes := []clusterNodesResponse{
			promotedReplica,
			{rejoinTestPreviousID, "10.0.0.93:6379@16379", "slave,fail", rejoinTestMasterID, "0", "0", "1", "disconnected"},
		}
		plan, err := planStaleRejoin(nodes, cr, "redis-cluster-leader-0", rejoinTestOrphanID)
		assert.NoError(t, err)
		assert.Equal(t, rejoinTestMasterID, plan.masterID)
	})

	t.Run("failed master with slots is not overwritten", func(t *testing.T) {
		nodes := []clusterNodesResponse{
			master,
			{rejoinTestPreviousID, "10.0.0.93:6379@16379,redis-cluster-leader-0", "master,fail", "-", "0", "0", "1", "disconnected", "0-5460"},
		}
		_, err := planStaleRejoin(nodes, cr, "redis-cluster-leader-0", rejoinTestOrphanID)
		assert.ErrorContains(t, err, "refusing to flush")
	})

	t.Run("unknown pod without a shard master is reported", func(t *testing.T) {
		_, err := planStaleRejoin([]clusterNodesResponse{master}, cr, "redis-cluster-leader-0", rejoinTestOrphanID)
		assert.ErrorContains(t, err, "no record of pod")
	})
}
