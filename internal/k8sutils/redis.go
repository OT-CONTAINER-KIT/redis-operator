package k8sutils

import (
	"bytes"
	"context"
	"encoding/csv"
	"errors"
	"fmt"
	"net"
	"strconv"
	"strings"
	"time"

	commonapi "github.com/OT-CONTAINER-KIT/redis-operator/api/common/v1beta2"
	rvb2 "github.com/OT-CONTAINER-KIT/redis-operator/api/redis/v1beta2"
	rcvb2 "github.com/OT-CONTAINER-KIT/redis-operator/api/rediscluster/v1beta2"
	rrvb2 "github.com/OT-CONTAINER-KIT/redis-operator/api/redisreplication/v1beta2"
	common "github.com/OT-CONTAINER-KIT/redis-operator/internal/controller/common"
	"github.com/OT-CONTAINER-KIT/redis-operator/internal/envs"
	retry "github.com/avast/retry-go"
	redis "github.com/redis/go-redis/v9"
	"github.com/samber/lo"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/tools/remotecommand"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

// RedisDetails will hold the information for Redis Pod
type RedisDetails struct {
	PodName   string
	Namespace string
}

func (rd *RedisDetails) FQDN() string {
	return fmt.Sprintf("%s.%s.%s.svc.%s", rd.PodName, common.GetHeadlessServiceNameFromPodName(rd.PodName), rd.Namespace, envs.GetServiceDNSDomain())
}

func (rd *RedisDetails) String() string {
	return fmt.Sprintf("%s.%s", rd.PodName, rd.Namespace)
}

// getRedisServerIP will return the IP of redis service
func getRedisServerIP(ctx context.Context, client kubernetes.Interface, redisInfo RedisDetails) string {
	log.FromContext(ctx).V(1).Info("Fetching Redis pod", "namespace", redisInfo.Namespace, "podName", redisInfo.PodName)

	redisPod, err := client.CoreV1().Pods(redisInfo.Namespace).Get(context.TODO(), redisInfo.PodName, metav1.GetOptions{})
	if err != nil {
		log.FromContext(ctx).Error(err, "Error in getting Redis pod IP", "namespace", redisInfo.Namespace, "podName", redisInfo.PodName)
		return ""
	}

	redisIP := redisPod.Status.PodIP
	log.FromContext(ctx).V(1).Info("Fetched Redis pod IP", "ip", redisIP)

	// Check if IP is empty
	if redisIP == "" {
		log.FromContext(ctx).V(1).Info("Redis pod IP is empty", "namespace", redisInfo.Namespace, "podName", redisInfo.PodName)
		return ""
	}

	// If we're NOT IPv4, assume we're IPv6..
	if net.ParseIP(redisIP).To4() == nil {
		log.FromContext(ctx).V(1).Info("Redis is using IPv6", "ip", redisIP)
	}

	log.FromContext(ctx).V(1).Info("Successfully got the IP for Redis", "ip", redisIP)
	return redisIP
}

func getRedisServerAddress(ctx context.Context, client kubernetes.Interface, rd RedisDetails, port int) string {
	return formatRedisAddress(getRedisServerIP(ctx, client, rd), port)
}

func getEndpoint(ctx context.Context, client kubernetes.Interface, cr *rcvb2.RedisCluster, rd RedisDetails) string {
	var (
		host string
		port int
	)
	port = *cr.Spec.Port
	if cr.Spec.ClusterVersion != nil && *cr.Spec.ClusterVersion == "v7" {
		host = rd.FQDN()
	} else {
		host = getRedisServerIP(ctx, client, rd)
		if host == "" {
			return ""
		}
	}
	if cr.Spec.KubernetesConfig.GetServiceType() == "NodePort" {
		svc, err := getService(ctx, client, cr.Namespace, rd.PodName)
		if err != nil {
			log.FromContext(ctx).Error(err, "Failed to get service for redis pod", "Pod", rd.PodName)
			return ""
		}
		if svc.Spec.Type != corev1.ServiceTypeNodePort {
			log.FromContext(ctx).Error(errors.New("service type mismatch"), "Expected NodePort service type", "Pod", rd.PodName, "ActualType", svc.Spec.Type)
			return ""
		}
		svcPort, ok := lo.Find(svc.Spec.Ports, func(item corev1.ServicePort) bool {
			return item.Name == "redis-client"
		})
		if ok {
			port = int(svcPort.NodePort)
		}
		pod, err := client.CoreV1().Pods(rd.Namespace).Get(ctx, rd.PodName, metav1.GetOptions{})
		if err != nil {
			log.FromContext(ctx).Error(err, "")
			return ""
		}
		host = pod.Status.HostIP
	}
	return host + ":" + strconv.Itoa(port)
}

// podExecFunc matches executeCommand's signature; it is injected into
// executeSingleLeaderAddSlots so the command assembly and batching logic
// can be unit tested without a live pod exec.
type podExecFunc func(ctx context.Context, client kubernetes.Interface, cr *rcvb2.RedisCluster, cmd []string, podName string)

// executeSingleLeaderAddSlots assigns all 16384 hash slots to the single
// leader node. On Redis 7+ it uses CLUSTER ADDSLOTSRANGE 0 16383 (a single
// compact command). On older versions it falls back to batched CLUSTER
// ADDSLOTS calls to stay within the Kubernetes pod exec URL length limit.
func executeSingleLeaderAddSlots(ctx context.Context, client kubernetes.Interface, cr *rcvb2.RedisCluster, execute podExecFunc) {
	logger := log.FromContext(ctx)

	var flags []string
	if cr.Spec.KubernetesConfig.ExistingPasswordSecret != nil {
		pass, err := getRedisPassword(ctx, client, cr.Namespace, *cr.Spec.KubernetesConfig.ExistingPasswordSecret.Name, *cr.Spec.KubernetesConfig.ExistingPasswordSecret.Key)
		if err != nil {
			logger.Error(err, "Error in getting redis password")
		} else {
			flags = append(flags, "-a", pass)
		}
	}
	flags = append(flags, getRedisTLSArgs(cr.Spec.TLS, cr.Name+"-leader-0")...)

	podName := cr.Name + "-leader-0"

	// Redis 7+ supports ADDSLOTSRANGE which takes a start-end pair instead
	// of listing every slot number individually — avoids the URL length issue entirely.
	if cr.Spec.ClusterVersion != nil && *cr.Spec.ClusterVersion == "v7" {
		cmd := []string{"redis-cli"}
		cmd = append(cmd, flags...)
		cmd = append(cmd, "CLUSTER", "ADDSLOTSRANGE", "0", "16383")
		logger.V(1).Info("Executing CLUSTER ADDSLOTSRANGE 0 16383")
		execute(ctx, client, cr, cmd, podName)
		return
	}

	// Fallback for Redis <7: batch ADDSLOTS into chunks of 1000 to stay
	// within the pod exec URL length limit. CLUSTER ADDSLOTS is idempotent
	// for unassigned slots, so partial retries on the next reconcile are safe.
	const totalSlots = 16384
	const batchSize = 1000
	for start := 0; start < totalSlots; start += batchSize {
		end := min(start+batchSize, totalSlots)
		cmd := []string{"redis-cli"}
		cmd = append(cmd, flags...)
		cmd = append(cmd, "CLUSTER", "ADDSLOTS")
		for i := start; i < end; i++ {
			cmd = append(cmd, strconv.Itoa(i))
		}
		logger.V(1).Info("Executing CLUSTER ADDSLOTS batch",
			"SlotsRange", fmt.Sprintf("%d-%d", start, end-1))
		execute(ctx, client, cr, cmd, podName)
	}
}

// RepairDisconnectedNodes attempts to repair disconnected/failed nodes (both masters and slaves)
// by issuing CLUSTER MEET with the updated address, and for slaves, re-establishing replication
// via CLUSTER REPLICATE so the follower resolves its master's current IP from gossip.
func RepairDisconnectedNodes(ctx context.Context, client kubernetes.Interface, cr *rcvb2.RedisCluster) error {
	redisClient := configureRedisClient(ctx, client, cr, cr.Name+"-leader-0")
	defer redisClient.Close()
	return repairDisconnectedNodes(ctx, client, cr, redisClient, func(podName string) *redis.Client {
		return configureRedisClient(ctx, client, cr, podName)
	})
}

func repairDisconnectedNodes(ctx context.Context, client kubernetes.Interface, cr *rcvb2.RedisCluster, redisClient *redis.Client, makeClient func(podName string) *redis.Client) error {
	nodes, err := clusterNodes(ctx, redisClient)
	if err != nil {
		return err
	}
	var lastError error
	for _, node := range nodes {
		if !nodeFailedOrDisconnected(node) {
			continue
		}
		host, err := getHostFromClusterNode(node)
		if err != nil {
			lastError = err
			log.FromContext(ctx).V(1).Error(err, "Failed to get pod name from cluster node. Continuing with other nodes.", "Node", node)
			continue
		}
		podName := strings.Split(host, ".")[0]
		ip := getRedisServerIP(ctx, client, RedisDetails{
			PodName:   podName,
			Namespace: cr.Namespace,
		})
		if ip == "" {
			lastError = fmt.Errorf("failed to get IP for pod %s", podName)
			log.FromContext(ctx).V(1).Error(lastError, "Empty IP for pod, skipping.", "Pod", podName)
			continue
		}
		if err = redisClient.ClusterMeet(ctx, ip, strconv.Itoa(*cr.Spec.Port)).Err(); err != nil {
			lastError = err
			log.FromContext(ctx).V(1).Error(err, "Failed to execute CLUSTER MEET on node. Continuing with other nodes.", "Node", node)
			continue
		}
		if nodeIsOfType(node, "slave") {
			masterNodeID := node[3]
			followerClient := makeClient(podName)
			if err = followerClient.ClusterReplicate(ctx, masterNodeID).Err(); err != nil {
				lastError = err
				log.FromContext(ctx).V(1).Error(err, "Failed to execute CLUSTER REPLICATE on follower.", "Follower", podName, "MasterNodeID", masterNodeID)
			}
			followerClient.Close()
		}
	}
	return lastError
}

// RepairStaleReplication checks connected followers for broken replication
// (master_link_status != up) and re-issues CLUSTER REPLICATE to force
// the follower to re-resolve its master's current IP from gossip.
// This handles the scenario where a master pod restarts with a new IP:
// gossip propagates the update, but follower replication remains
// pointed at the stale address until explicitly refreshed.
//
// A broken replication link is invisible to gossip-based health checks
// (the follower still reports as "connected" in CLUSTER NODES), so this
// cannot be gated behind UnhealthyNodesInCluster. Detection requires
// asking each follower directly: one CLUSTER NODES call on leader-0 plus
// one INFO replication call per connected follower, per invocation.
// Returns the number of followers that were repaired and any error.
func RepairStaleReplication(ctx context.Context, client kubernetes.Interface, cr *rcvb2.RedisCluster) (int, error) {
	redisClient := configureRedisClient(ctx, client, cr, cr.Name+"-leader-0")
	defer redisClient.Close()
	return repairStaleReplication(ctx, redisClient, func(podName string) *redis.Client {
		return configureRedisClient(ctx, client, cr, podName)
	})
}

func repairStaleReplication(ctx context.Context, redisClient *redis.Client, makeClient func(podName string) *redis.Client) (int, error) {
	logger := log.FromContext(ctx)

	nodes, err := clusterNodes(ctx, redisClient)
	if err != nil {
		return 0, err
	}

	repaired := 0
	var lastError error
	for _, node := range nodes {
		if !nodeIsOfType(node, "slave") {
			continue
		}
		if nodeFailedOrDisconnected(node) {
			continue
		}
		host, err := getHostFromClusterNode(node)
		if err != nil {
			lastError = err
			continue
		}
		podName := strings.Split(host, ".")[0]
		masterNodeID := node[3]

		followerClient := makeClient(podName)
		info, err := followerClient.Info(ctx, "replication").Result()
		if err != nil {
			followerClient.Close()
			lastError = err
			logger.V(1).Error(err, "Failed to get replication info", "Follower", podName)
			continue
		}

		if replicationLinkUp(info) {
			followerClient.Close()
			continue
		}

		logger.Info("Follower replication link is down, re-issuing CLUSTER REPLICATE",
			"Follower", podName, "MasterNodeID", masterNodeID)
		if err = followerClient.ClusterReplicate(ctx, masterNodeID).Err(); err != nil {
			lastError = err
			logger.Error(err, "Failed to re-establish replication",
				"Follower", podName, "MasterNodeID", masterNodeID)
		} else {
			repaired++
		}
		followerClient.Close()
	}
	return repaired, lastError
}

// RejoinIsolatedNodes detects pods that have fallen out of the cluster and can
// see only themselves (CLUSTER INFO reports cluster_known_nodes <= 1) and
// rejoins them automatically. After a pod is deleted and recreated it can come
// back isolated — typically when it restarts before its peers are reachable or
// its nodes.conf was lost — and never rejoins on its own, which previously
// required an operator to run CLUSTER MEET by hand.
//
// This is complementary to RepairDisconnectedNodes: that repair works from
// leader-0's gossip view and so cannot see (or MEET) a live pod that has been
// dropped from, or never re-learned by, leader-0's node table — it MEETs a
// stale address that no longer reaches the live pod. RejoinIsolatedNodes
// instead probes each expected pod directly and, for any isolated one, issues
// CLUSTER MEET from that pod toward leader-0 so the isolated node initiates the
// handshake and re-learns the full topology through gossip. Follower pods are
// then reattached to their expected master with CLUSTER REPLICATE so they do
// not linger as empty masters. Returns the number of pods rejoined and any error.
func RejoinIsolatedNodes(ctx context.Context, client kubernetes.Interface, cr *rcvb2.RedisCluster) (int, error) {
	seedPod := cr.Name + "-leader-0"
	seedClient := configureRedisClient(ctx, client, cr, seedPod)
	defer seedClient.Close()
	seedIP := getRedisServerIP(ctx, client, RedisDetails{PodName: seedPod, Namespace: cr.Namespace})
	return rejoinIsolatedNodes(ctx, cr, seedClient, seedIP, func(podName string) *redis.Client {
		return configureRedisClient(ctx, client, cr, podName)
	})
}

func rejoinIsolatedNodes(ctx context.Context, cr *rcvb2.RedisCluster, seedClient *redis.Client, seedIP string, makeClient func(podName string) *redis.Client) (int, error) {
	logger := log.FromContext(ctx)
	if seedIP == "" {
		return 0, fmt.Errorf("failed to resolve IP for seed node %s-leader-0", cr.Name)
	}
	seedPod := cr.Name + "-leader-0"
	port := strconv.Itoa(*cr.Spec.Port)
	leaderCount := cr.Spec.GetReplicaCounts("leader")
	followerCount := cr.Spec.GetReplicaCounts("follower")

	// Map expected master pod name -> master node ID from the seed's gossip view
	// so rejoined followers can be reattached to the right master.
	nodes, err := clusterNodes(ctx, seedClient)
	if err != nil {
		return 0, err
	}
	masterIDByPod := masterNodeIDsByPod(nodes)

	rejoined := 0
	var lastErr error

	// rejoin probes a single pod and, if isolated, MEETs it back to the seed.
	// masterPod is the expected master pod name for a follower, or "" for a leader.
	rejoin := func(podName string, masterPod string) {
		// The seed is the reference point every other pod is rejoined against; it
		// cannot MEET itself. If the seed itself is isolated the whole cluster
		// query layer is broken and other reconcile steps that rely on leader-0
		// surface that separately.
		if podName == seedPod {
			return
		}
		podClient := makeClient(podName)
		defer podClient.Close()

		info, err := podClient.ClusterInfo(ctx).Result()
		if err != nil {
			lastErr = err
			logger.V(1).Error(err, "Failed to get CLUSTER INFO from pod", "Pod", podName)
			return
		}
		if known := clusterKnownNodes(info); known < 0 || known > 1 {
			return
		}

		logger.Info("Pod is isolated from the cluster, rejoining via CLUSTER MEET", "Pod", podName)
		if err := podClient.ClusterMeet(ctx, seedIP, port).Err(); err != nil {
			lastErr = err
			logger.Error(err, "Failed to CLUSTER MEET seed from isolated pod", "Pod", podName)
			return
		}
		rejoined++

		if masterPod == "" {
			return
		}
		masterID := masterIDByPod[masterPod]
		if masterID == "" {
			lastErr = fmt.Errorf("no known master node ID for %s, cannot reattach follower %s", masterPod, podName)
			logger.Error(lastErr, "Skipping CLUSTER REPLICATE for rejoined follower")
			return
		}
		// CLUSTER MEET only starts the gossip handshake; the follower may not yet
		// know the master's node ID, so retry REPLICATE until the handshake
		// completes. Without this the follower lingers as an empty master.
		if err := retry.Do(func() error {
			return podClient.ClusterReplicate(ctx, masterID).Err()
		}, retry.Attempts(5), retry.Delay(time.Second*2)); err != nil {
			lastErr = err
			logger.Error(err, "Failed to reattach rejoined follower to master", "Follower", podName, "MasterNodeID", masterID)
		}
	}

	for i := int32(0); i < leaderCount; i++ {
		rejoin(fmt.Sprintf("%s-leader-%d", cr.Name, i), "")
	}
	for i := int32(0); leaderCount > 0 && i < followerCount; i++ {
		masterPod := fmt.Sprintf("%s-leader-%d", cr.Name, i%leaderCount)
		rejoin(fmt.Sprintf("%s-follower-%d", cr.Name, i), masterPod)
	}
	return rejoined, lastErr
}

// masterNodeIDsByPod maps pod name -> node ID for healthy master nodes in the
// gossip view, used to reattach rejoined followers to their expected master.
func masterNodeIDsByPod(nodes []clusterNodesResponse) map[string]string {
	m := make(map[string]string)
	for _, node := range nodes {
		if !nodeIsOfType(node, "master") || nodeFailedOrDisconnected(node) {
			continue
		}
		host, err := getHostFromClusterNode(node)
		if err != nil {
			continue
		}
		m[strings.Split(host, ".")[0]] = node[0]
	}
	return m
}

// ReattachMisplacedReplicas demotes pods that are sitting as empty (slot-less)
// masters back into replicas of their shard's current slot-owning master. This
// covers two cases RejoinIsolatedNodes leaves behind:
//
//  1. A rejoined follower whose CLUSTER REPLICATE handshake did not complete in
//     the rejoin's single reconcile, so it lingers as an empty master.
//  2. An ex-leader that returned (e.g. after a delete + failover, possibly under
//     a new node ID) as an empty master while a promoted follower in the same
//     shard now owns the slots — leaving the shard split.
//
// Both are the same defect: a member of a shard is an empty master when it
// should be a replica of whichever member currently owns the slots. The repair
// is always data-safe because it only ever issues CLUSTER REPLICATE on a
// confirmed empty (0-slot) node, pointing it at the live slot owner — the empty
// node syncs from the data holder, never the reverse. A shard whose slots are
// owned only by a dead fail/noaddr orphan (no live owner) is skipped: that is
// the missed-promotion case which needs a forced failover, not a reattach.
// Returns the number of pods reattached and any error.
func ReattachMisplacedReplicas(ctx context.Context, client kubernetes.Interface, cr *rcvb2.RedisCluster) (int, error) {
	seedClient := configureRedisClient(ctx, client, cr, cr.Name+"-leader-0")
	defer seedClient.Close()
	return reattachMisplacedReplicas(ctx, cr, seedClient, func(podName string) *redis.Client {
		return configureRedisClient(ctx, client, cr, podName)
	})
}

// liveMasterEntry is a pod's live (non-fail/disconnected/noaddr) master entry in
// the seed's gossip view: its node ID and whether it currently owns slots.
type liveMasterEntry struct {
	id     string
	serves bool
}

func reattachMisplacedReplicas(ctx context.Context, cr *rcvb2.RedisCluster, seedClient *redis.Client, makeClient func(podName string) *redis.Client) (int, error) {
	logger := log.FromContext(ctx)
	leaderCount := cr.Spec.GetReplicaCounts("leader")
	followerCount := cr.Spec.GetReplicaCounts("follower")
	if leaderCount == 0 {
		return 0, nil
	}

	nodes, err := clusterNodes(ctx, seedClient)
	if err != nil {
		return 0, err
	}

	// Index each pod's live master entry from the seed's gossip view. Dead
	// fail/noaddr/disconnected entries (e.g. a replaced pod's old node ID) are
	// ignored here — they are handled by ForgetStaleNodes/RejoinIsolatedNodes —
	// so a pod that appears both as a dead orphan and a live empty master is
	// classified by its live entry only.
	liveMasterByPod := make(map[string]liveMasterEntry)
	for _, node := range nodes {
		if len(node) < 8 || !nodeIsOfType(node, "master") || nodeFailedDisconnectedOrNoAddr(node) {
			continue
		}
		host, err := getHostFromClusterNode(node)
		if err != nil {
			continue
		}
		liveMasterByPod[podNameFromHost(host)] = liveMasterEntry{id: node[0], serves: nodeServesSlots(node)}
	}

	reattached := 0
	var lastErr error
	for n := int32(0); n < leaderCount; n++ {
		members := shardMemberPods(cr.Name, n, leaderCount, followerCount)

		// The shard's current slot owner is the live member that serves slots.
		ownerID, ownerPod := "", ""
		for _, pod := range members {
			if e, ok := liveMasterByPod[pod]; ok && e.serves {
				ownerID, ownerPod = e.id, pod
				break
			}
		}
		// No live slot-owning member: either a genuinely new/empty shard (real
		// scale-up, left for the empty-master rebalance) or the missed-promotion
		// case (slots owned only by a dead orphan). Either way, do not reattach.
		if ownerID == "" {
			continue
		}

		// Any other member that is a live empty (0-slot) master is misplaced;
		// make it replicate the shard's slot owner.
		for _, pod := range members {
			if pod == ownerPod {
				continue
			}
			e, ok := liveMasterByPod[pod]
			if !ok || e.serves {
				continue
			}
			logger.Info("Reattaching misplaced empty master as replica of shard slot owner",
				"Pod", pod, "ShardOwnerPod", ownerPod, "ShardOwnerNodeID", ownerID)
			podClient := makeClient(pod)
			// CLUSTER REPLICATE needs the follower to have learned the owner's
			// node ID through gossip; retry to ride out an incomplete handshake.
			if err := retry.Do(func() error {
				return podClient.ClusterReplicate(ctx, ownerID).Err()
			}, retry.Attempts(5), retry.Delay(time.Second*2)); err != nil {
				lastErr = err
				logger.Error(err, "Failed to reattach misplaced empty master",
					"Pod", pod, "ShardOwnerNodeID", ownerID)
			} else {
				reattached++
			}
			podClient.Close()
		}
	}
	return reattached, lastErr
}

// shardMemberPods returns the pod names belonging to shard n under the operator's
// pod-name model: leader-n plus every follower-i with i % leaderCount == n.
func shardMemberPods(crName string, n, leaderCount, followerCount int32) []string {
	members := []string{fmt.Sprintf("%s-leader-%d", crName, n)}
	for i := int32(0); i < followerCount; i++ {
		if i%leaderCount == n {
			members = append(members, fmt.Sprintf("%s-follower-%d", crName, i))
		}
	}
	return members
}

// nodeServesSlots reports whether a CLUSTER NODES entry currently owns any hash
// slots (including migrating/importing markers).
func nodeServesSlots(node clusterNodesResponse) bool {
	if len(node) < 9 {
		return false
	}
	for _, tok := range node[8:] {
		if looksLikeSlotToken(tok) {
			return true
		}
	}
	return false
}

// nodeFailedDisconnectedOrNoAddr reports whether a node is in a state that makes
// it unusable as a live cluster member: flagged fail/fail?/noaddr or link-state
// disconnected.
func nodeFailedDisconnectedOrNoAddr(node clusterNodesResponse) bool {
	return nodeFailedOrDisconnected(node) || strings.Contains(node[2], "noaddr")
}

// podNameFromHost extracts the pod name from a cluster node's announced hostname
// (e.g. "redis-cluster-leader-4.redis-cluster-leader-headless...").
func podNameFromHost(host string) string {
	return strings.Split(host, ".")[0]
}

// clusterKnownNodes returns the cluster_known_nodes value from CLUSTER INFO
// output, or -1 if the field is absent or cannot be parsed.
func clusterKnownNodes(info string) int {
	for _, line := range strings.Split(info, "\r\n") {
		if strings.HasPrefix(line, "cluster_known_nodes:") {
			n, err := strconv.Atoi(strings.TrimSpace(strings.TrimPrefix(line, "cluster_known_nodes:")))
			if err != nil {
				return -1
			}
			return n
		}
	}
	return -1
}

// replicationLinkUp returns true when the INFO Replication output
// contains master_link_status:up, indicating healthy replication.
// Returns true for master nodes (no master_link_status field).
func replicationLinkUp(info string) bool {
	for _, line := range strings.Split(info, "\r\n") {
		if strings.HasPrefix(line, "master_link_status:") {
			return strings.TrimPrefix(line, "master_link_status:") == "up"
		}
	}
	return true
}

func getHostFromClusterNode(node clusterNodesResponse) (string, error) {
	addressAndHost := node[1]
	s := strings.Split(addressAndHost, ",")
	if len(s) != 2 {
		return "", fmt.Errorf("failed to extract host from host and address string, unexpected number of elements: %d", len(s))
	}
	return strings.Split(addressAndHost, ",")[1], nil
}

// CreateMultipleLeaderRedisCommand will create command for single leader cluster creation
func CreateMultipleLeaderRedisCommand(ctx context.Context, client kubernetes.Interface, cr *rcvb2.RedisCluster) RedisInvocation {
	cmd := RedisInvocation{
		Command: []string{"redis-cli", "--cluster", "create"},
	}
	replicas := cr.Spec.GetReplicaCounts("leader")
	for podCount := 0; podCount < int(replicas); podCount++ {
		rd := RedisDetails{
			PodName:   cr.Name + "-leader-" + strconv.Itoa(podCount),
			Namespace: cr.Namespace,
		}
		cmd.AddFlag(getEndpoint(ctx, client, cr, rd))
	}
	cmd.AddFlag("--cluster-yes")
	return cmd
}

// RedisInvocation models an invocation of redis-cli
type RedisInvocation struct {
	Command      []string // e.g. {"redis-cli", "--cluster", "create"}
	Flags        []string // e.g. {"-h", "localhost", "-p", "6379"}
	RedisCommand []string // e.g. {"CLUSTER", "ADDSLOTS", "1", "2", "3"}
}

// Builds the full argv for executeCommand
func (ri *RedisInvocation) Args() []string {
	args := append([]string{}, ri.Command...)
	args = append(args, ri.Flags...)
	args = append(args, ri.RedisCommand...)
	return args
}

func (ri *RedisInvocation) AddFlag(flag ...string) *RedisInvocation {
	ri.Flags = append(ri.Flags, flag...)
	return ri
}

// ExecuteRedisClusterCommand will execute redis cluster creation command
func ExecuteRedisClusterCommand(ctx context.Context, client kubernetes.Interface, cr *rcvb2.RedisCluster) {
	replicas := cr.Spec.GetReplicaCounts("leader")
	switch int(replicas) {
	case 1:
		err := executeFailoverCommand(ctx, client, cr, "leader")
		if err != nil {
			log.FromContext(ctx).Error(err, "error executing failover command")
		}
		executeSingleLeaderAddSlots(ctx, client, cr, executeCommand)
	default:
		cmd := CreateMultipleLeaderRedisCommand(ctx, client, cr)
		if cr.Spec.KubernetesConfig.ExistingPasswordSecret != nil {
			pass, err := getRedisPassword(ctx, client, cr.Namespace, *cr.Spec.KubernetesConfig.ExistingPasswordSecret.Name, *cr.Spec.KubernetesConfig.ExistingPasswordSecret.Key)
			if err != nil {
				log.FromContext(ctx).Error(err, "Error in getting redis password")
			}
			cmd.AddFlag("-a")
			cmd.AddFlag(pass)
		}
		cmd.AddFlag(getRedisTLSArgs(cr.Spec.TLS, cr.Name+"-leader-0")...)
		executeCommand(ctx, client, cr, cmd.Args(), cr.Name+"-leader-0")
	}
}

func getRedisTLSArgs(tlsConfig *commonapi.TLSConfig, clientHost string) []string {
	cmd := []string{}
	if tlsConfig != nil {
		cmd = append(cmd, "--tls")
		if tlsConfig.CaCertFile != "" {
			caFile, _, _ := getTLSSecretKeys(tlsConfig)
			cmd = append(cmd, "--cacert")
			cmd = append(cmd, "/tls/"+caFile)
		}
		cmd = append(cmd, "--insecure")
	}
	return cmd
}

// createRedisReplicationCommand will create redis replication creation command
func createRedisReplicationCommand(ctx context.Context, client kubernetes.Interface, cr *rcvb2.RedisCluster, leaderPod RedisDetails, followerPod RedisDetails) []string {
	cmd := []string{"redis-cli", "--cluster", "add-node"}
	cmd = append(cmd, getEndpoint(ctx, client, cr, followerPod))
	cmd = append(cmd, getEndpoint(ctx, client, cr, leaderPod))
	cmd = append(cmd, "--cluster-slave")
	if cr.Spec.KubernetesConfig.ExistingPasswordSecret != nil {
		pass, err := getRedisPassword(ctx, client, cr.Namespace, *cr.Spec.KubernetesConfig.ExistingPasswordSecret.Name, *cr.Spec.KubernetesConfig.ExistingPasswordSecret.Key)
		if err != nil {
			log.FromContext(ctx).Error(err, "Failed to retrieve Redis password", "Secret", *cr.Spec.KubernetesConfig.ExistingPasswordSecret.Name)
		} else {
			cmd = append(cmd, "-a", pass)
		}
	}
	cmd = append(cmd, getRedisTLSArgs(cr.Spec.TLS, leaderPod.PodName)...)
	return cmd
}

// ExecuteRedisReplicationCommand will execute the replication command
func ExecuteRedisReplicationCommand(ctx context.Context, client kubernetes.Interface, cr *rcvb2.RedisCluster) {
	var podIP string
	followerCounts := cr.Spec.GetReplicaCounts("follower")
	leaderCounts := cr.Spec.GetReplicaCounts("leader")
	followerPerLeader := followerCounts / leaderCounts

	redisClient := configureRedisClient(ctx, client, cr, cr.Name+"-leader-0")
	defer redisClient.Close()

	nodes, err := clusterNodes(ctx, redisClient)
	if err != nil {
		log.FromContext(ctx).Error(err, "failed to get cluster nodes")
	}
	for followerIdx := 0; followerIdx <= int(followerCounts)-1; {
		for i := 0; i < int(followerPerLeader) && followerIdx <= int(followerCounts)-1; i++ {
			followerPod := RedisDetails{
				PodName:   cr.Name + "-follower-" + strconv.Itoa(followerIdx),
				Namespace: cr.Namespace,
			}
			leaderPod := RedisDetails{
				PodName:   cr.Name + "-leader-" + strconv.Itoa((followerIdx)%int(leaderCounts)),
				Namespace: cr.Namespace,
			}
			podIP = getRedisServerIP(ctx, client, followerPod)
			if !checkRedisNodePresence(ctx, nodes, podIP) {
				log.FromContext(ctx).V(1).Info("Adding node to cluster.", "Node.IP", podIP, "Follower.Pod", followerPod)
				cmd := createRedisReplicationCommand(ctx, client, cr, leaderPod, followerPod)
				redisClient := configureRedisClient(ctx, client, cr, followerPod.PodName)
				pong, err := redisClient.Ping(ctx).Result()
				redisClient.Close()
				if err != nil {
					log.FromContext(ctx).Error(err, "Failed to ping Redis server", "Follower.Pod", followerPod)
					continue
				}
				if pong == "PONG" {
					executeCommand(ctx, client, cr, cmd, cr.Name+"-leader-0")
				} else {
					log.FromContext(ctx).V(1).Info("Skipping execution of command due to failed Redis ping", "Follower.Pod", followerPod)
				}
			} else {
				log.FromContext(ctx).V(1).Info("Skipping Adding node to cluster, already present.", "Follower.Pod", followerPod)
			}

			followerIdx++
		}
	}
}

type clusterNodesResponse []string

// clusterNodes will returns the response of CLUSTER NODES
func clusterNodes(ctx context.Context, redisClient *redis.Client) ([]clusterNodesResponse, error) {
	output, err := redisClient.ClusterNodes(ctx).Result()
	if err != nil {
		return nil, err
	}

	csvOutput := csv.NewReader(strings.NewReader(output))
	csvOutput.Comma = ' '
	csvOutput.FieldsPerRecord = -1
	csvOutputRecords, err := csvOutput.ReadAll()
	if err != nil {
		return nil, err
	}
	response := make([]clusterNodesResponse, 0, len(csvOutputRecords))
	for _, record := range csvOutputRecords {
		response = append(response, record)
	}
	return response, nil
}

// ExecuteFailoverOperation will execute redis failover operations
func ExecuteFailoverOperation(ctx context.Context, client kubernetes.Interface, cr *rcvb2.RedisCluster) error {
	err := executeFailoverCommand(ctx, client, cr, "leader")
	if err != nil {
		return err
	}
	err = executeFailoverCommand(ctx, client, cr, "follower")
	if err != nil {
		return err
	}
	return nil
}

// executeFailoverCommand will execute failover command
func executeFailoverCommand(ctx context.Context, client kubernetes.Interface, cr *rcvb2.RedisCluster, role string) error {
	replicas := cr.Spec.GetReplicaCounts(role)
	podName := fmt.Sprintf("%s-%s-", cr.Name, role)
	for podCount := 0; podCount <= int(replicas)-1; podCount++ {
		log.FromContext(ctx).V(1).Info("Executing redis failover operations", "Redis Node", podName+strconv.Itoa(podCount))
		client := configureRedisClient(ctx, client, cr, podName+strconv.Itoa(podCount))
		defer client.Close()
		cmd := redis.NewStringCmd(ctx, "cluster", "reset")
		err := client.Process(ctx, cmd)
		if err != nil {
			log.FromContext(ctx).Error(err, "Redis command failed with this error")
			flushcommand := redis.NewStringCmd(ctx, "flushall")
			err = client.Process(ctx, flushcommand)
			if err != nil {
				log.FromContext(ctx).Error(err, "Redis flush command failed with this error")
				return err
			}
		}
		err = client.Process(ctx, cmd)
		if err != nil {
			log.FromContext(ctx).Error(err, "Redis command failed with this error")
			return err
		}
		output, err := cmd.Result()
		if err != nil {
			log.FromContext(ctx).Error(err, "Redis command failed with this error")
			return err
		}
		log.FromContext(ctx).V(1).Info("Redis cluster failover executed", "Output", output)
	}
	return nil
}

// CheckRedisNodeCount will check the count of redis nodes known to the cluster
// (including failed/disconnected ones). This is used by the controller to
// decide whether the cluster topology exists at all. For detecting unhealthy
// nodes that need repair, use UnhealthyNodesInCluster instead.
func CheckRedisNodeCount(ctx context.Context, client kubernetes.Interface, cr *rcvb2.RedisCluster, nodeType string) int32 {
	redisClient := configureRedisClient(ctx, client, cr, cr.Name+"-leader-0")
	defer redisClient.Close()
	var redisNodeType string
	clusterNodes, err := clusterNodes(ctx, redisClient)
	if err != nil {
		log.FromContext(ctx).Error(err, "failed to get cluster nodes")
	}
	count := len(clusterNodes)

	switch nodeType {
	case "leader":
		redisNodeType = "master"
	case "follower":
		redisNodeType = "slave"
	default:
		redisNodeType = nodeType
	}
	if nodeType != "" {
		count = 0
		for _, node := range clusterNodes {
			if nodeIsOfType(node, redisNodeType) {
				count++
			}
		}
		log.FromContext(ctx).V(1).Info("Number of redis nodes are", "Nodes", strconv.Itoa(count), "Type", nodeType)
	} else {
		log.FromContext(ctx).V(1).Info("Total number of redis nodes are", "Nodes", strconv.Itoa(count))
	}
	return int32(count)
}

// RedisClusterStatusHealth use `redis-cli --cluster check 127.0.0.1:6379`
func RedisClusterStatusHealth(ctx context.Context, client kubernetes.Interface, cr *rcvb2.RedisCluster) bool {
	logger := log.FromContext(ctx)
	leaderReplicas := cr.Spec.GetReplicaCounts("leader")

	// Try to check cluster health from multiple leader nodes with retry logic
	var lastErr error
	for i := int32(0); i < leaderReplicas; i++ {
		podName := fmt.Sprintf("%s-leader-%d", cr.Name, i)

		// Retry logic with exponential backoff for each node
		err := retry.Do(
			func() error {
				return checkClusterHealth(ctx, client, cr, podName)
			},
			retry.Attempts(3),
			retry.Delay(500*time.Millisecond),
			retry.DelayType(retry.BackOffDelay),
			retry.OnRetry(func(n uint, err error) {
				logger.V(1).Info("Retrying cluster health check", "pod", podName, "attempt", n+1, "error", err)
			}),
		)

		if err == nil {
			// Successfully verified cluster health from this node
			logger.V(1).Info("Cluster health check passed", "pod", podName)
			return true
		}

		lastErr = err
		logger.V(1).Info("Cluster health check failed from node", "pod", podName, "error", err)
	}

	// All nodes failed the health check
	if lastErr != nil {
		logger.Error(lastErr, "Cluster health check failed from all leader nodes")
	}
	return false
}

// checkClusterHealth performs a single cluster health check against a specific pod
func checkClusterHealth(ctx context.Context, client kubernetes.Interface, cr *rcvb2.RedisCluster, podName string) error {
	logger := log.FromContext(ctx)

	cmd := []string{"redis-cli", "--cluster", "check", fmt.Sprintf("127.0.0.1:%d", *cr.Spec.Port)}
	if cr.Spec.KubernetesConfig.ExistingPasswordSecret != nil {
		pass, err := getRedisPassword(ctx, client, cr.Namespace, *cr.Spec.KubernetesConfig.ExistingPasswordSecret.Name, *cr.Spec.KubernetesConfig.ExistingPasswordSecret.Key)
		if err != nil {
			return fmt.Errorf("error getting redis password: %w", err)
		}
		cmd = append(cmd, "-a", pass)
	}
	cmd = append(cmd, getRedisTLSArgs(cr.Spec.TLS, podName)...)

	out, err := executeCommand1(ctx, client, cr, cmd, podName)
	if err != nil {
		return fmt.Errorf("failed to execute cluster check command: %w", err)
	}

	// Check for the expected success indicators
	// [OK] xxx keys in xxx masters.
	// [OK] All nodes agree about slots configuration.
	// [OK] All 16384 slots covered.
	okCount := strings.Count(out, "[OK]")
	if okCount != 3 {
		logger.V(1).Info("Cluster health check output", "pod", podName, "okCount", okCount, "output", out)
		return fmt.Errorf("cluster health check failed: expected 3 [OK] messages, got %d", okCount)
	}

	// Additional check: ensure no [ERR] or [WARNING] in critical lines
	if strings.Contains(out, "[ERR]") {
		return fmt.Errorf("cluster health check found errors in output")
	}

	return nil
}

// UnhealthyNodesInCluster returns the number of unhealthy nodes in the cluster cr
func UnhealthyNodesInCluster(ctx context.Context, client kubernetes.Interface, cr *rcvb2.RedisCluster) (int, error) {
	redisClient := configureRedisClient(ctx, client, cr, cr.Name+"-leader-0")
	defer redisClient.Close()
	clusterNodes, err := clusterNodes(ctx, redisClient)
	if err != nil {
		return 0, err
	}
	count := 0
	for _, node := range clusterNodes {
		if nodeFailedOrDisconnected(node) {
			count++
		}
	}
	log.FromContext(ctx).V(1).Info("Number of failed nodes in cluster", "Failed Node Count", count)
	return count, nil
}

func nodeIsOfType(node clusterNodesResponse, nodeType string) bool {
	return strings.Contains(node[2], nodeType)
}

func nodeFailedOrDisconnected(node clusterNodesResponse) bool {
	return strings.Contains(node[2], "fail") || strings.Contains(node[7], "disconnected")
}

// configureRedisClient will configure the Redis Client
func configureRedisClient(ctx context.Context, client kubernetes.Interface, cr *rcvb2.RedisCluster, podName string) *redis.Client {
	redisInfo := RedisDetails{
		PodName:   podName,
		Namespace: cr.Namespace,
	}
	var err error
	var pass string
	if cr.Spec.KubernetesConfig.ExistingPasswordSecret != nil {
		pass, err = getRedisPassword(ctx, client, cr.Namespace, *cr.Spec.KubernetesConfig.ExistingPasswordSecret.Name, *cr.Spec.KubernetesConfig.ExistingPasswordSecret.Key)
		if err != nil {
			log.FromContext(ctx).Error(err, "Error in getting redis password")
		}
	}
	opts := &redis.Options{
		Addr:         getRedisServerAddress(ctx, client, redisInfo, *cr.Spec.Port),
		Password:     pass,
		DB:           0,
		DialTimeout:  defaultRedisClientTimeout,
		ReadTimeout:  defaultRedisClientTimeout,
		WriteTimeout: defaultRedisClientTimeout,
	}
	if cr.Spec.TLS != nil {
		opts.TLSConfig = getRedisTLSConfig(ctx, client, cr.Namespace, cr.Spec.TLS)
	}
	return redis.NewClient(opts)
}

func configureRedisStandaloneClient(ctx context.Context, client kubernetes.Interface, cr *rvb2.Redis, podName string) *redis.Client {
	redisInfo := RedisDetails{
		PodName:   podName,
		Namespace: cr.Namespace,
	}
	var err error
	var pass string
	if cr.Spec.KubernetesConfig.ExistingPasswordSecret != nil {
		pass, err = getRedisPassword(ctx, client, cr.Namespace, *cr.Spec.KubernetesConfig.ExistingPasswordSecret.Name, *cr.Spec.KubernetesConfig.ExistingPasswordSecret.Key)
		if err != nil {
			log.FromContext(ctx).Error(err, "Error in getting redis password")
		}
	}
	opts := &redis.Options{
		Addr:     getRedisServerAddress(ctx, client, redisInfo, common.RedisPort),
		Password: pass,
		DB:       0,
	}
	if cr.Spec.TLS != nil {
		opts.TLSConfig = getRedisTLSConfig(ctx, client, cr.Namespace, cr.Spec.TLS)
	}
	return redis.NewClient(opts)
}

// executeCommand will execute the commands in pod
func executeCommand(ctx context.Context, client kubernetes.Interface, cr *rcvb2.RedisCluster, cmd []string, podName string) {
	execOut, execErr := executeCommand1(ctx, client, cr, cmd, podName)
	if execErr != nil {
		log.FromContext(ctx).Error(execErr, "Could not execute command", "Command", cmd, "Output", execOut)
		return
	}
	log.FromContext(ctx).V(1).Info("Successfully executed the command", "Command", cmd, "Output", execOut)
}

// defaultExecCommandTimeout bounds a single exec stream against a redis pod. It is generous
// enough for a legitimate slow `redis-cli --cluster create` while still guaranteeing the stream
// (and therefore the reconcile worker) cannot block forever. Override with EXEC_COMMAND_TIMEOUT.
const defaultExecCommandTimeout = 5 * time.Minute

// defaultRedisClientTimeout bounds dial/read/write operations of the go-redis clients the
// reconciler opens against redis pods, so an unreachable pod cannot stall a reconcile.
const defaultRedisClientTimeout = 5 * time.Second

func executeCommand1(ctx context.Context, client kubernetes.Interface, cr *rcvb2.RedisCluster, cmd []string, podName string) (stdout string, stderr error) {
	var (
		execOut bytes.Buffer
		execErr bytes.Buffer
	)
	config, err := GenerateK8sConfig()()
	if err != nil {
		log.FromContext(ctx).Error(err, "Could not find pod to execute")
		return "", err
	}
	targetContainer, pod := getContainerID(ctx, client, cr, podName)
	if targetContainer < 0 {
		log.FromContext(ctx).Error(err, "Could not find pod to execute")
		return "", err
	}

	req := client.CoreV1().RESTClient().Post().Resource("pods").Name(podName).Namespace(cr.Namespace).SubResource("exec")
	req.VersionedParams(&corev1.PodExecOptions{
		Container: pod.Spec.Containers[targetContainer].Name,
		Command:   cmd,
		Stdout:    true,
		Stderr:    true,
	}, scheme.ParameterCodec)
	exec, err := remotecommand.NewSPDYExecutor(config, "POST", req.URL())
	if err != nil {
		log.FromContext(ctx).Error(err, "Failed to init executor")
		return "", err
	}

	// Bound the exec stream with the reconcile ctx and a timeout so a blocked command (e.g. a
	// `redis-cli --cluster create` against pods that cannot yet form a cluster) returns an error
	// and requeues instead of pinning the reconcile worker forever, which would starve every
	// other Redis resource across all namespaces.
	execCtx, cancel := context.WithTimeout(ctx, envs.GetExecCommandTimeout(defaultExecCommandTimeout))
	defer cancel()
	err = exec.StreamWithContext(execCtx, remotecommand.StreamOptions{
		Stdout: &execOut,
		Stderr: &execErr,
		Tty:    false,
	})
	if err != nil {
		return execOut.String(), fmt.Errorf("execute command with error: %w, stderr: %s", err, execErr.String())
	}
	return execOut.String(), nil
}

// getContainerID will return the id of container from pod
func getContainerID(ctx context.Context, client kubernetes.Interface, cr *rcvb2.RedisCluster, podName string) (int, *corev1.Pod) {
	pod, err := client.CoreV1().Pods(cr.Namespace).Get(ctx, podName, metav1.GetOptions{})
	if err != nil {
		log.FromContext(ctx).Error(err, "Could not get pod info", "Pod Name", podName, "Namespace", cr.Namespace)
		return -1, nil
	}

	log.FromContext(ctx).V(1).Info("Pod info retrieved successfully", "Pod Name", podName, "Namespace", cr.Namespace)

	targetContainer := -1
	for containerID, tr := range pod.Spec.Containers {
		log.FromContext(ctx).V(1).Info("Inspecting container", "Pod Name", podName, "Container ID", containerID, "Container Name", tr.Name)
		if tr.Name == cr.Name+"-leader" {
			targetContainer = containerID
			log.FromContext(ctx).V(1).Info("Leader container found", "Container ID", containerID, "Container Name", tr.Name)
			break
		}
	}

	if targetContainer == -1 {
		log.FromContext(ctx).V(1).Info("Leader container not found in pod", "Pod Name", podName)
		return -1, nil
	}

	return targetContainer, pod
}

// checkRedisNodePresence will check if the redis node exist in cluster or not
func checkRedisNodePresence(ctx context.Context, nodeList []clusterNodesResponse, nodeName string) bool {
	log.FromContext(ctx).V(1).Info("Checking if Node is in cluster", "Node", nodeName)
	for _, node := range nodeList {
		s := strings.Split(node[1], ":")
		if s[0] == nodeName {
			return true
		}
	}
	return false
}

// configureRedisClient will configure the Redis Client
func configureRedisReplicationClient(ctx context.Context, client kubernetes.Interface, cr *rrvb2.RedisReplication, podName string) *redis.Client {
	pod, err := client.CoreV1().Pods(cr.Namespace).Get(ctx, podName, metav1.GetOptions{})
	if err == nil {
		return configureRedisReplicationClientForPod(ctx, client, cr, pod)
	}

	log.FromContext(ctx).V(1).Info("Falling back to redis replication pod lookup during client configuration", "pod", podName, "error", err)
	return configureRedisReplicationClientForAddress(ctx, client, cr, RedisDetails{
		PodName:   podName,
		Namespace: cr.Namespace,
	}, "")
}

func configureRedisReplicationClientForPod(ctx context.Context, client kubernetes.Interface, cr *rrvb2.RedisReplication, pod *corev1.Pod) *redis.Client {
	return configureRedisReplicationClientForAddress(ctx, client, cr, RedisDetails{
		PodName:   pod.Name,
		Namespace: cr.Namespace,
	}, pod.Status.PodIP)
}

func configureRedisReplicationClientForAddress(ctx context.Context, client kubernetes.Interface, cr *rrvb2.RedisReplication, redisInfo RedisDetails, podIP string) *redis.Client {
	var err error
	var pass string
	if cr.Spec.KubernetesConfig.ExistingPasswordSecret != nil {
		pass, err = getRedisPassword(ctx, client, cr.Namespace, *cr.Spec.KubernetesConfig.ExistingPasswordSecret.Name, *cr.Spec.KubernetesConfig.ExistingPasswordSecret.Key)
		if err != nil {
			log.FromContext(ctx).Error(err, "Error in getting redis password")
		}
	}
	var addr string
	if cr.Spec.TLS != nil {
		// Use DNS name for TLS connections
		addr = fmt.Sprintf("%s:%d", getRedisReplicationHostname(redisInfo, cr), 6379)
	} else {
		if podIP == "" {
			podIP = getRedisServerIP(ctx, client, redisInfo)
		}
		addr = formatRedisAddress(podIP, 6379)
	}
	opts := &redis.Options{
		Addr:         addr,
		Password:     pass,
		DB:           0,
		DialTimeout:  defaultRedisClientTimeout,
		ReadTimeout:  defaultRedisClientTimeout,
		WriteTimeout: defaultRedisClientTimeout,
	}
	if cr.Spec.TLS != nil {
		opts.TLSConfig = getRedisTLSConfig(ctx, client, cr.Namespace, cr.Spec.TLS)
	}
	return redis.NewClient(opts)
}

func formatRedisAddress(ip string, port int) string {
	if ip == "" {
		return fmt.Sprintf("%s:%d", ip, port)
	}
	format := "%s:%d"
	if net.ParseIP(ip).To4() == nil {
		format = "[%s]:%d"
	}
	return fmt.Sprintf(format, ip, port)
}

func getRedisReplicationHostname(redisInfo RedisDetails, cr *rrvb2.RedisReplication) string {
	return fmt.Sprintf("%s.%s-headless.%s.svc.%s", redisInfo.PodName, cr.Name, cr.Namespace, envs.GetServiceDNSDomain())
}

// Get Redis nodes by it's role i.e. master, slave and sentinel
func GetRedisNodesByRole(ctx context.Context, cl kubernetes.Interface, cr *rrvb2.RedisReplication, redisRole string) ([]string, error) {
	return getRedisNodesByRole(ctx, cl, cr, redisRole, func(ctx context.Context, pod *corev1.Pod) (string, error) {
		redisClient := configureRedisReplicationClientForPod(ctx, cl, cr, pod)
		defer redisClient.Close()

		return checkRedisServerRole(ctx, redisClient, pod.Name)
	})
}

func getRedisNodesByRole(ctx context.Context, cl kubernetes.Interface, cr *rrvb2.RedisReplication, redisRole string, probeRole func(context.Context, *corev1.Pod) (string, error)) ([]string, error) {
	statefulset, err := GetStatefulSet(ctx, cl, cr.GetNamespace(), cr.GetName())
	if err != nil {
		log.FromContext(ctx).Error(err, "Failed to Get the Statefulset of the", "custom resource", cr.Name, "in namespace", cr.Namespace)
		return nil, err
	}

	var pods []string
	replicas := cr.Spec.GetReplicationCounts("replication")

	for i := 0; i < int(replicas); i++ {
		podName := statefulset.Name + "-" + strconv.Itoa(i)
		pod, err := cl.CoreV1().Pods(cr.Namespace).Get(ctx, podName, metav1.GetOptions{})
		if err != nil {
			if apierrors.IsNotFound(err) {
				continue
			}
			return nil, err
		}

		if !IsRedisPodProbeable(pod) {
			continue
		}

		podRole, err := probeRole(ctx, pod)
		if err != nil {
			return nil, err
		}
		if podRole == redisRole {
			pods = append(pods, podName)
		}
	}

	return pods, nil
}

func IsRedisPodProbeable(pod *corev1.Pod) bool {
	if pod == nil || pod.Status.Phase != corev1.PodRunning || pod.Status.PodIP == "" {
		return false
	}
	for _, condition := range pod.Status.Conditions {
		if condition.Type == corev1.PodReady && condition.Status == corev1.ConditionTrue {
			return true
		}
	}
	return false
}

// Check the Redis Server Role i.e. master, slave and sentinel
func checkRedisServerRole(ctx context.Context, redisClient *redis.Client, podName string) (string, error) {
	info, err := redisClient.Info(ctx, "Replication").Result()
	if err != nil {
		log.FromContext(ctx).Error(err, "Failed to Get the role Info of the", "redis pod", podName)
		return "", err
	}
	lines := strings.Split(info, "\r\n")
	for _, line := range lines {
		if strings.HasPrefix(line, "role:") {
			role := strings.TrimPrefix(line, "role:")
			log.FromContext(ctx).V(1).Info("Role of the Redis Pod", "pod", podName, "role", role)
			return role, nil
		}
	}
	log.FromContext(ctx).Error(err, "Failed to find role from Info # Replication in", "redis pod", podName)
	return "", err
}

// checkAttachedSlave would return redis pod name which has slave
func checkAttachedSlave(ctx context.Context, redisClient *redis.Client, podName string) int {
	info, err := redisClient.Info(ctx, "Replication").Result()
	if err != nil {
		log.FromContext(ctx).Error(err, "Failed to get the connected slaves count of the", "redis pod", podName)
		return -1 // return -1 if failed to get the connected slaves count
	}

	lines := strings.Split(info, "\r\n")
	for _, line := range lines {
		if strings.HasPrefix(line, "connected_slaves:") {
			var connected_slaves int
			connected_slaves, err = strconv.Atoi(strings.TrimPrefix(line, "connected_slaves:"))
			if err != nil {
				log.FromContext(ctx).Error(err, "Failed to convert the connected slaves count of the", "redis pod", podName)
				return -1
			}
			log.FromContext(ctx).V(1).Info("Connected Slaves of the Redis Pod", "pod", podName, "connected_slaves", connected_slaves)
			return connected_slaves
		}
	}

	log.FromContext(ctx).Error(nil, "Failed to find connected_slaves from Info # Replication in", "redis pod", podName)
	return 0
}

func CreateMasterSlaveReplication(ctx context.Context, client kubernetes.Interface, cr *rrvb2.RedisReplication, masterPods []string, realMasterPod string) error {
	log.FromContext(ctx).V(1).Info("Redis Master Node is set to", "pod", realMasterPod)
	realMasterInfo := RedisDetails{
		PodName:   realMasterPod,
		Namespace: cr.Namespace,
	}

	var realMasterAddr string
	if cr.Spec.TLS != nil {
		// Use DNS name for TLS connections to match certificate validation
		realMasterAddr = getRedisReplicationHostname(realMasterInfo, cr)
		log.FromContext(ctx).V(1).Info("Using DNS address for TLS master replication", "masterAddr", realMasterAddr)
	} else {
		// Use IP address for non-TLS connections
		realMasterPodIP := getRedisServerIP(ctx, client, realMasterInfo)
		if realMasterPodIP == "" {
			return errors.New("CreateMasterSlaveReplication got empty master IP, refusing")
		}
		realMasterAddr = realMasterPodIP
		log.FromContext(ctx).V(1).Info("Using IP address for non-TLS master replication", "masterAddr", realMasterAddr)
	}

	for i := 0; i < len(masterPods); i++ {
		if masterPods[i] != realMasterPod {
			redisClient := configureRedisReplicationClient(ctx, client, cr, masterPods[i])
			defer redisClient.Close()
			log.FromContext(ctx).V(1).Info("Setting the", "pod", masterPods[i], "to slave of", realMasterPod, "masterAddr", realMasterAddr)
			err := redisClient.SlaveOf(ctx, realMasterAddr, "6379").Err()
			if err != nil {
				log.FromContext(ctx).Error(err, "Failed to set", "pod", masterPods[i], "to slave of", realMasterPod, "masterAddr", realMasterAddr)
				return err
			}
		}
	}

	return nil
}

func GetRedisReplicationRealMaster(ctx context.Context, client kubernetes.Interface, cr *rrvb2.RedisReplication, masterPods []string) string {
	for _, podName := range masterPods {
		redisClient := configureRedisReplicationClient(ctx, client, cr, podName)
		defer redisClient.Close()

		if checkAttachedSlave(ctx, redisClient, podName) > 0 {
			return podName
		}
	}
	return ""
}

func applyDynamicConfig(ctx context.Context, redisClient *redis.Client, podName string, dynamicConfig []string) (bool, error) {
	pong, err := redisClient.Ping(ctx).Result()
	if err != nil {
		log.FromContext(ctx).Error(err, "Failed to ping Redis instance", "pod", podName)
		return false, nil
	}
	if pong != "PONG" {
		log.FromContext(ctx).V(1).Info("Redis instance not ready", "pod", podName)
		return false, nil
	}

	for _, config := range dynamicConfig {
		parts := strings.SplitN(config, " ", 2)
		if len(parts) != 2 {
			log.FromContext(ctx).Error(nil, "Invalid config format", "config", config)
			continue
		}

		if err := redisClient.ConfigSet(ctx, parts[0], parts[1]).Err(); err != nil {
			log.FromContext(ctx).Error(err, "Failed to set config",
				"key", parts[0],
				"value", parts[1],
				"pod", podName)
			return true, err
		}

		log.FromContext(ctx).V(1).Info("Successfully set config",
			"key", parts[0],
			"value", parts[1],
			"pod", podName)
	}

	return true, nil
}

// SetRedisClusterDynamicConfig applies dynamic configuration to each Redis instance in the cluster
func SetRedisClusterDynamicConfig(ctx context.Context, client kubernetes.Interface, cr *rcvb2.RedisCluster) error {
	// Get dynamic configuration
	dynamicConfig := cr.Spec.GetRedisDynamicConfig()
	if len(dynamicConfig) == 0 {
		return nil
	}

	// Get the number of leader and follower pods
	leaderReplicas := cr.Spec.GetReplicaCounts("leader")
	followerReplicas := cr.Spec.GetReplicaCounts("follower")

	// Apply configuration to each Redis instance
	for i := 0; i < int(leaderReplicas+followerReplicas); i++ {
		var podName string
		if i < int(leaderReplicas) {
			podName = cr.Name + "-leader-" + strconv.Itoa(i)
		} else {
			podName = cr.Name + "-follower-" + strconv.Itoa(i-int(leaderReplicas))
		}

		redisClient := configureRedisClient(ctx, client, cr, podName)
		_, err := applyDynamicConfig(ctx, redisClient, podName, dynamicConfig)
		redisClient.Close()
		if err != nil {
			return err
		}
	}

	return nil
}

func SetRedisReplicationDynamicConfig(ctx context.Context, client kubernetes.Interface, cr *rrvb2.RedisReplication) error {
	return setRedisReplicationDynamicConfig(ctx, cr, func(podName string) *redis.Client {
		return configureRedisReplicationClient(ctx, client, cr, podName)
	})
}

func setRedisReplicationDynamicConfig(ctx context.Context, cr *rrvb2.RedisReplication, makeClient func(podName string) *redis.Client) error {
	dynamicConfig := cr.Spec.GetRedisDynamicConfig()
	if len(dynamicConfig) == 0 {
		return nil
	}

	replicas := cr.Spec.GetReplicationCounts("")
	for i := 0; i < int(replicas); i++ {
		podName := cr.Name + "-" + strconv.Itoa(i)

		redisClient := makeClient(podName)
		_, err := applyDynamicConfig(ctx, redisClient, podName, dynamicConfig)
		redisClient.Close()
		if err != nil {
			return err
		}
	}

	return nil
}

func SetRedisStandaloneDynamicConfig(ctx context.Context, client kubernetes.Interface, cr *rvb2.Redis) (bool, error) {
	return setRedisStandaloneDynamicConfig(ctx, cr, func(podName string) *redis.Client {
		return configureRedisStandaloneClient(ctx, client, cr, podName)
	})
}

func setRedisStandaloneDynamicConfig(ctx context.Context, cr *rvb2.Redis, makeClient func(podName string) *redis.Client) (bool, error) {
	dynamicConfig := cr.Spec.GetRedisDynamicConfig()
	if len(dynamicConfig) == 0 {
		return true, nil
	}

	podName := cr.Name + "-0"
	redisClient := makeClient(podName)
	defer redisClient.Close()

	return applyDynamicConfig(ctx, redisClient, podName, dynamicConfig)
}
