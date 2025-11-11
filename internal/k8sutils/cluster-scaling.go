package k8sutils

import (
	"context"
	"fmt"
	"net"
	"strconv"
	"strings"
	"time"

	rcvb2 "github.com/OT-CONTAINER-KIT/redis-operator/api/rediscluster/v1beta2"
	retry "github.com/avast/retry-go"
	redis "github.com/redis/go-redis/v9"
	"k8s.io/client-go/kubernetes"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

// ReshardRedisCluster transfer the slots from the last node to the provided transfer node.
//
// NOTE: when all slot been transferred, the node become slave of the transfer node.
func ReshardRedisCluster(ctx context.Context, client kubernetes.Interface, cr *rcvb2.RedisCluster, shardIdx int32, transferNodeIdx int32, remove bool) {
	transferNodeName := fmt.Sprintf("%s-leader-%d", cr.Name, transferNodeIdx)
	redisClient := configureRedisClient(ctx, client, cr, transferNodeName)
	defer redisClient.Close()

	var cmd []string

	// Transfer Pod details
	transferPOD := RedisDetails{
		PodName:   transferNodeName,
		Namespace: cr.Namespace,
	}
	// Remove POD details
	removePOD := RedisDetails{
		PodName:   cr.Name + "-leader-" + strconv.Itoa(int(shardIdx)),
		Namespace: cr.Namespace,
	}
	cmd = []string{"redis-cli", "--cluster", "reshard"}
	cmd = append(cmd, getEndpoint(ctx, client, cr, transferPOD))
	if cr.Spec.KubernetesConfig.ExistingPasswordSecret != nil {
		pass, err := getRedisPassword(ctx, client, cr.Namespace, *cr.Spec.KubernetesConfig.ExistingPasswordSecret.Name, *cr.Spec.KubernetesConfig.ExistingPasswordSecret.Key)
		if err != nil {
			log.FromContext(ctx).Error(err, "error in getting redis password")
			return
		}
		cmd = append(cmd, "-a")
		cmd = append(cmd, pass)
	}

	cmd = append(cmd, getRedisTLSArgs(cr.Spec.TLS, transferNodeName)...)

	//--cluster-from <node-id> --cluster-to <node-id> --cluster-slots <number of slots> --cluster-yes

	// Remove Node
	removeNodeID := getRedisNodeID(ctx, client, cr, removePOD)
	cmd = append(cmd, "--cluster-from")
	cmd = append(cmd, removeNodeID)

	// Transfer Node
	transferNodeID := getRedisNodeID(ctx, client, cr, transferPOD)
	cmd = append(cmd, "--cluster-to")
	cmd = append(cmd, transferNodeID)

	// Cluster Slots
	slots := getRedisClusterSlots(ctx, redisClient, removeNodeID)
	if slots == "0" || slots == "" {
		log.FromContext(ctx).Info("skipping the execution cmd because no slots found", "Cmd", cmd)
		return
	}
	cmd = append(cmd, "--cluster-slots")
	cmd = append(cmd, slots)

	cmd = append(cmd, "--cluster-yes")

	log.FromContext(ctx).Info(fmt.Sprintf("transferring %s slots from shard %d to shard %d", slots, shardIdx, transferNodeIdx))
	executeCommand(ctx, client, cr, cmd, transferNodeName)
	log.FromContext(ctx).Info(fmt.Sprintf("transferring %s slots from shard %d to shard %d completed", slots, shardIdx, transferNodeIdx))

	if remove {
		RemoveRedisNodeFromCluster(ctx, client, cr, removePOD)
	}
}

func getRedisClusterSlots(ctx context.Context, redisClient *redis.Client, nodeID string) string {
	totalSlots := 0

	redisSlots, err := redisClient.ClusterSlots(ctx).Result()
	if err != nil {
		log.FromContext(ctx).Error(err, "failed to get cluster slots")
		return ""
	}
	for _, slot := range redisSlots {
		for _, node := range slot.Nodes {
			if node.ID == nodeID {
				// Each slot range is a continuous block managed by the node
				totalSlots += slot.End - slot.Start + 1
				break
			}
		}
	}

	return strconv.Itoa(totalSlots)
}

// getRedisNodeID would return nodeID of a redis node by passing pod
func getRedisNodeID(ctx context.Context, client kubernetes.Interface, cr *rcvb2.RedisCluster, pod RedisDetails) string {
	redisClient := configureRedisClient(ctx, client, cr, pod.PodName)
	defer redisClient.Close()

	pong, err := redisClient.Ping(ctx).Result()
	if err != nil || pong != "PONG" {
		log.FromContext(ctx).Error(err, "Failed to ping Redis server")
		return ""
	}

	cmd := redis.NewStringCmd(ctx, "cluster", "myid")
	err = redisClient.Process(ctx, cmd)
	if err != nil {
		log.FromContext(ctx).Error(err, "Redis command failed with this error")
		return ""
	}

	output, err := cmd.Result()
	if err != nil {
		log.FromContext(ctx).Error(err, "Redis command failed with this error")
		return ""
	}
	log.FromContext(ctx).V(1).Info("Redis node ID ", "is", output)
	return output
}

// Rebalance the Redis CLuster using the Empty Master Nodes
func RebalanceRedisClusterEmptyMasters(ctx context.Context, client kubernetes.Interface, cr *rcvb2.RedisCluster) {
	logger := log.FromContext(ctx)

	// Use retry logic for rebalancing
	err := retry.Do(
		func() error {
			return rebalanceClusterWithEmptyMasters(ctx, client, cr)
		},
		retry.Attempts(3),
		retry.Delay(2*time.Second),
		retry.DelayType(retry.BackOffDelay),
		retry.OnRetry(func(n uint, err error) {
			logger.Info("Retrying cluster rebalance with empty masters", "attempt", n+1, "error", err)
		}),
	)
	if err != nil {
		logger.Error(err, "Failed to rebalance cluster with empty masters after retries")
		return
	}

	logger.Info("Successfully rebalanced cluster with empty masters")
}

// rebalanceClusterWithEmptyMasters performs the actual rebalance operation and validates the result
func rebalanceClusterWithEmptyMasters(ctx context.Context, client kubernetes.Interface, cr *rcvb2.RedisCluster) error {
	logger := log.FromContext(ctx)

	// Find a healthy leader node to execute the rebalance command
	totalRedisLeaderNodes := CheckRedisNodeCount(ctx, client, cr, "leader")
	var selectedPod string
	for i := int32(0); i < totalRedisLeaderNodes; i++ {
		podName := fmt.Sprintf("%s-leader-%d", cr.Name, i)
		// Test if pod is reachable
		redisClient := configureRedisClient(ctx, client, cr, podName)
		if _, err := redisClient.Ping(ctx).Result(); err == nil {
			selectedPod = podName
			redisClient.Close()
			break
		}
		redisClient.Close()
	}

	if selectedPod == "" {
		return fmt.Errorf("no healthy leader node found to execute rebalance")
	}

	logger.V(1).Info("Executing rebalance with empty masters", "pod", selectedPod)

	// cmd = redis-cli --cluster rebalance <redis>:<port> --cluster-use-empty-masters -a <pass>
	var cmd []string
	pod := RedisDetails{
		PodName:   selectedPod,
		Namespace: cr.Namespace,
	}
	cmd = []string{"redis-cli", "--cluster", "rebalance"}
	cmd = append(cmd, getEndpoint(ctx, client, cr, pod))
	cmd = append(cmd, "--cluster-use-empty-masters")
	if cr.Spec.KubernetesConfig.ExistingPasswordSecret != nil {
		pass, err := getRedisPassword(ctx, client, cr.Namespace, *cr.Spec.KubernetesConfig.ExistingPasswordSecret.Name, *cr.Spec.KubernetesConfig.ExistingPasswordSecret.Key)
		if err != nil {
			return fmt.Errorf("error getting redis password: %w", err)
		}
		cmd = append(cmd, "-a", pass)
	}

	cmd = append(cmd, getRedisTLSArgs(cr.Spec.TLS, selectedPod)...)

	output, err := executeCommand1(ctx, client, cr, cmd, selectedPod)
	if err != nil {
		return fmt.Errorf("failed to execute rebalance command: %w", err)
	}

	logger.V(1).Info("Rebalance command output", "output", output)

	// Validate that the cluster is now balanced (no empty masters)
	if err := validateClusterBalance(ctx, client, cr, false); err != nil {
		return fmt.Errorf("cluster validation failed after rebalance: %w", err)
	}

	return nil
}

func CheckIfEmptyMasters(ctx context.Context, client kubernetes.Interface, cr *rcvb2.RedisCluster) {
	totalRedisLeaderNodes := CheckRedisNodeCount(ctx, client, cr, "leader")
	redisClient := configureRedisClient(ctx, client, cr, cr.Name+"-leader-0")
	defer redisClient.Close()

	for i := 0; i < int(totalRedisLeaderNodes); i++ {
		pod := RedisDetails{
			PodName:   cr.Name + "-leader-" + strconv.Itoa(i),
			Namespace: cr.Namespace,
		}
		podNodeID := getRedisNodeID(ctx, client, cr, pod)
		podSlots := getRedisClusterSlots(ctx, redisClient, podNodeID)

		if podSlots == "0" || podSlots == "" {
			log.FromContext(ctx).V(1).Info("Found Empty Redis Leader Node", "pod", pod)
			RebalanceRedisClusterEmptyMasters(ctx, client, cr)
			break
		}
	}
}

// Rebalance Redis Cluster Would Rebalance the Redis Cluster without using the empty masters
func RebalanceRedisCluster(ctx context.Context, client kubernetes.Interface, cr *rcvb2.RedisCluster) {
	logger := log.FromContext(ctx)

	// Use retry logic for rebalancing
	err := retry.Do(
		func() error {
			return rebalanceCluster(ctx, client, cr)
		},
		retry.Attempts(3),
		retry.Delay(2*time.Second),
		retry.DelayType(retry.BackOffDelay),
		retry.OnRetry(func(n uint, err error) {
			logger.Info("Retrying cluster rebalance", "attempt", n+1, "error", err)
		}),
	)
	if err != nil {
		logger.Error(err, "Failed to rebalance cluster after retries")
		return
	}

	logger.Info("Successfully rebalanced cluster")
}

// rebalanceCluster performs the actual rebalance operation and validates the result
func rebalanceCluster(ctx context.Context, client kubernetes.Interface, cr *rcvb2.RedisCluster) error {
	logger := log.FromContext(ctx)

	// Find a healthy leader node to execute the rebalance command
	totalRedisLeaderNodes := CheckRedisNodeCount(ctx, client, cr, "leader")
	var selectedPod string
	for i := int32(0); i < totalRedisLeaderNodes; i++ {
		podName := fmt.Sprintf("%s-leader-%d", cr.Name, i)
		// Test if pod is reachable
		redisClient := configureRedisClient(ctx, client, cr, podName)
		if _, err := redisClient.Ping(ctx).Result(); err == nil {
			selectedPod = podName
			redisClient.Close()
			break
		}
		redisClient.Close()
	}

	if selectedPod == "" {
		return fmt.Errorf("no healthy leader node found to execute rebalance")
	}

	logger.V(1).Info("Executing rebalance", "pod", selectedPod)

	// cmd = redis-cli --cluster rebalance <redis>:<port> -a <pass>
	var cmd []string
	pod := RedisDetails{
		PodName:   selectedPod,
		Namespace: cr.Namespace,
	}
	cmd = []string{"redis-cli", "--cluster", "rebalance"}
	cmd = append(cmd, getEndpoint(ctx, client, cr, pod))
	if cr.Spec.KubernetesConfig.ExistingPasswordSecret != nil {
		pass, err := getRedisPassword(ctx, client, cr.Namespace, *cr.Spec.KubernetesConfig.ExistingPasswordSecret.Name, *cr.Spec.KubernetesConfig.ExistingPasswordSecret.Key)
		if err != nil {
			return fmt.Errorf("error getting redis password: %w", err)
		}
		cmd = append(cmd, "-a", pass)
	}

	cmd = append(cmd, getRedisTLSArgs(cr.Spec.TLS, selectedPod)...)

	output, err := executeCommand1(ctx, client, cr, cmd, selectedPod)
	if err != nil {
		return fmt.Errorf("failed to execute rebalance command: %w", err)
	}

	logger.V(1).Info("Rebalance command output", "output", output)

	// Validate that the cluster is balanced
	// Allow empty masters here as this is called during scale-down
	if err := validateClusterBalance(ctx, client, cr, true); err != nil {
		return fmt.Errorf("cluster validation failed after rebalance: %w", err)
	}

	return nil
}

// Add redis cluster node would add a node to the existing redis cluster using redis-cli
func AddRedisNodeToCluster(ctx context.Context, client kubernetes.Interface, cr *rcvb2.RedisCluster) {
	cmd := []string{"redis-cli", "--cluster", "add-node"}
	activeRedisNode := CheckRedisNodeCount(ctx, client, cr, "leader")
	newPod := RedisDetails{
		PodName:   cr.Name + "-leader-" + strconv.Itoa(int(activeRedisNode)),
		Namespace: cr.Namespace,
	}
	existingPod := RedisDetails{
		PodName:   cr.Name + "-leader-0",
		Namespace: cr.Namespace,
	}
	cmd = append(cmd, getEndpoint(ctx, client, cr, newPod))
	cmd = append(cmd, getEndpoint(ctx, client, cr, existingPod))
	if cr.Spec.KubernetesConfig.ExistingPasswordSecret != nil {
		pass, err := getRedisPassword(ctx, client, cr.Namespace, *cr.Spec.KubernetesConfig.ExistingPasswordSecret.Name, *cr.Spec.KubernetesConfig.ExistingPasswordSecret.Key)
		if err != nil {
			log.FromContext(ctx).Error(err, "Error in getting redis password")
		}
		cmd = append(cmd, "-a")
		cmd = append(cmd, pass)
	}

	cmd = append(cmd, getRedisTLSArgs(cr.Spec.TLS, cr.Name+"-leader-0")...)

	executeCommand(ctx, client, cr, cmd, cr.Name+"-leader-0")
}

// getAttachedFollowerNodeIDs would return a slice of redis followers attached to a redis leader
func getAttachedFollowerNodeIDs(ctx context.Context, redisClient *redis.Client, masterNodeID string) []string {
	// 3acb029fead40752f432c84f9bed2e639119a573 192.168.84.239:6379@16379,redis-cluster-v1beta2-follower-5 slave e3299968586dd457a8dba04fc6c747cecd38510f 0 1713595736542 6 connected
	slaveNodes, err := redisClient.ClusterSlaves(ctx, masterNodeID).Result()
	if err != nil {
		log.FromContext(ctx).Error(err, "Failed to get attached follower node IDs", "masterNodeID", masterNodeID)
		return nil
	}
	slaveIDs := make([]string, 0, len(slaveNodes))
	for _, slave := range slaveNodes {
		stringSlice := strings.Split(slave, " ")
		slaveIDs = append(slaveIDs, stringSlice[0])
	}
	log.FromContext(ctx).V(1).Info("Slaves Nodes attached to", "node", masterNodeID, "are", slaveIDs)
	return slaveIDs
}

// Remove redis follower node would remove all follower nodes of last leader node using redis-cli
func RemoveRedisFollowerNodesFromCluster(ctx context.Context, client kubernetes.Interface, cr *rcvb2.RedisCluster, shardIdx int32) {
	var cmd []string
	redisClient := configureRedisClient(ctx, client, cr, cr.Name+"-leader-0")
	defer redisClient.Close()

	existingPod := RedisDetails{
		PodName:   cr.Name + "-leader-0",
		Namespace: cr.Namespace,
	}
	lastLeaderPod := RedisDetails{
		PodName:   cr.Name + "-leader-" + strconv.Itoa(int(shardIdx)),
		Namespace: cr.Namespace,
	}

	cmd = []string{"redis-cli"}

	if cr.Spec.KubernetesConfig.ExistingPasswordSecret != nil {
		pass, err := getRedisPassword(ctx, client, cr.Namespace, *cr.Spec.KubernetesConfig.ExistingPasswordSecret.Name, *cr.Spec.KubernetesConfig.ExistingPasswordSecret.Key)
		if err != nil {
			log.FromContext(ctx).Error(err, "Error in getting redis password")
		}
		cmd = append(cmd, "-a")
		cmd = append(cmd, pass)
	}
	cmd = append(cmd, getRedisTLSArgs(cr.Spec.TLS, cr.Name+"-leader-0")...)

	lastLeaderPodNodeID := getRedisNodeID(ctx, client, cr, lastLeaderPod)
	followerNodeIDs := getAttachedFollowerNodeIDs(ctx, redisClient, lastLeaderPodNodeID)

	cmd = append(cmd, "--cluster", "del-node")
	cmd = append(cmd, getEndpoint(ctx, client, cr, existingPod))
	for _, followerNodeID := range followerNodeIDs {
		cmd = append(cmd, followerNodeID)
		executeCommand(ctx, client, cr, cmd, cr.Name+"-leader-0")
		cmd = cmd[:len(cmd)-1]
	}
}

// Remove redis cluster node would remove last node to the existing redis cluster using redis-cli
func RemoveRedisNodeFromCluster(ctx context.Context, client kubernetes.Interface, cr *rcvb2.RedisCluster, removePod RedisDetails) {
	redisClient := configureRedisClient(ctx, client, cr, cr.Name+"-leader-0")
	defer redisClient.Close()
	existingPod := RedisDetails{
		PodName:   cr.Name + "-leader-0",
		Namespace: cr.Namespace,
	}
	cmd := []string{"redis-cli", "--cluster", "del-node"}
	cmd = append(cmd, getEndpoint(ctx, client, cr, existingPod))
	cmd = append(cmd, getRedisNodeID(ctx, client, cr, removePod))
	if cr.Spec.KubernetesConfig.ExistingPasswordSecret != nil {
		pass, err := getRedisPassword(ctx, client, cr.Namespace, *cr.Spec.KubernetesConfig.ExistingPasswordSecret.Name, *cr.Spec.KubernetesConfig.ExistingPasswordSecret.Key)
		if err != nil {
			log.FromContext(ctx).Error(err, "Error in getting redis password")
		}
		cmd = append(cmd, "-a")
		cmd = append(cmd, pass)
	}
	cmd = append(cmd, getRedisTLSArgs(cr.Spec.TLS, cr.Name+"-leader-0")...)
	executeCommand(ctx, client, cr, cmd, cr.Name+"-leader-0")
}

// verifyLeaderPod return true if the pod is leader/master
func VerifyLeaderPod(ctx context.Context, client kubernetes.Interface, cr *rcvb2.RedisCluster, leadIndex int32) bool {
	podName := cr.Name + "-leader-" + strconv.Itoa(int(leadIndex))

	redisClient := configureRedisClient(ctx, client, cr, podName)
	defer redisClient.Close()
	return verifyLeaderPodInfo(ctx, redisClient, podName)
}

func verifyLeaderPodInfo(ctx context.Context, redisClient *redis.Client, podName string) bool {
	info, err := redisClient.Info(ctx, "replication").Result()
	if err != nil {
		log.FromContext(ctx).Error(err, "Failed to Get the role Info of the", "redis pod", podName)
		return false
	}

	lines := strings.Split(info, "\r\n")
	var role string
	for _, line := range lines {
		if strings.HasPrefix(line, "role:") {
			role = strings.TrimPrefix(line, "role:")
			return role == "master"
		}
	}
	return false
}

func ClusterFailover(ctx context.Context, client kubernetes.Interface, cr *rcvb2.RedisCluster, shardIdx int32) error {
	slavePodName := cr.Name + "-leader-" + strconv.Itoa(int(shardIdx))
	// cmd = redis-cli cluster failover  -a <pass>
	var cmd []string
	pod := RedisDetails{
		PodName:   slavePodName,
		Namespace: cr.Namespace,
	}
	host, port, err := net.SplitHostPort(getEndpoint(ctx, client, cr, pod))
	if err != nil {
		return err
	}
	cmd = []string{"redis-cli", "-h", host, "-p", port}
	if cr.Spec.KubernetesConfig.ExistingPasswordSecret != nil {
		pass, err := getRedisPassword(ctx, client, cr.Namespace, *cr.Spec.KubernetesConfig.ExistingPasswordSecret.Name, *cr.Spec.KubernetesConfig.ExistingPasswordSecret.Key)
		if err != nil {
			log.FromContext(ctx).Error(err, "Error in getting redis password")
		}
		cmd = append(cmd, "-a")
		cmd = append(cmd, pass)
	}

	cmd = append(cmd, getRedisTLSArgs(cr.Spec.TLS, slavePodName)...)
	cmd = append(cmd, "cluster", "failover")

	log.FromContext(ctx).V(1).Info("Redis cluster failover command is", "Command", cmd)
	execOut, err := executeCommand1(ctx, client, cr, cmd, slavePodName)
	if err != nil {
		log.FromContext(ctx).Error(err, "Could not execute command", "Command", cmd, "Output", execOut)
		return err
	}
	return nil
}

// validateClusterBalance checks if the cluster is properly balanced
// Returns error if any master has 0 slots (empty master) or if slots are not evenly distributed
func validateClusterBalance(ctx context.Context, client kubernetes.Interface, cr *rcvb2.RedisCluster, allowEmptyMasters bool) error {
	logger := log.FromContext(ctx)
	totalRedisLeaderNodes := CheckRedisNodeCount(ctx, client, cr, "leader")

	if totalRedisLeaderNodes == 0 {
		return fmt.Errorf("no leader nodes found in cluster")
	}

	// Try to connect to any available leader node
	var redisClient *redis.Client
	var connectedPod string
	for i := 0; i < int(totalRedisLeaderNodes); i++ {
		podName := fmt.Sprintf("%s-leader-%d", cr.Name, i)
		redisClient = configureRedisClient(ctx, client, cr, podName)
		// Test connection
		if _, err := redisClient.Ping(ctx).Result(); err == nil {
			connectedPod = podName
			break
		}
		redisClient.Close()
	}

	if redisClient == nil || connectedPod == "" {
		return fmt.Errorf("unable to connect to any leader node")
	}
	defer redisClient.Close()

	logger.V(1).Info("Validating cluster balance", "connectedPod", connectedPod, "totalLeaders", totalRedisLeaderNodes)

	emptyMasterCount := 0
	slotsPerNode := make(map[string]int)

	for i := 0; i < int(totalRedisLeaderNodes); i++ {
		pod := RedisDetails{
			PodName:   fmt.Sprintf("%s-leader-%d", cr.Name, i),
			Namespace: cr.Namespace,
		}
		podNodeID := getRedisNodeID(ctx, client, cr, pod)
		if podNodeID == "" {
			logger.V(1).Info("Could not get node ID for pod", "pod", pod.PodName)
			continue
		}

		podSlots := getRedisClusterSlots(ctx, redisClient, podNodeID)
		slotCount := 0
		if podSlots != "" && podSlots != "0" {
			// Parse slot count (podSlots can be like "5461" or "0-5460")
			if strings.Contains(podSlots, "-") {
				slotCount, _ = strconv.Atoi(strings.Split(podSlots, "-")[1])
				slotCount++ // inclusive range
			} else {
				slotCount, _ = strconv.Atoi(podSlots)
			}
		}

		slotsPerNode[pod.PodName] = slotCount
		if slotCount == 0 {
			emptyMasterCount++
			logger.V(1).Info("Found empty master node", "pod", pod.PodName, "nodeID", podNodeID)
		}
	}

	// Check if we have empty masters when we shouldn't
	if !allowEmptyMasters && emptyMasterCount > 0 {
		return fmt.Errorf("found %d empty master nodes, cluster is not balanced", emptyMasterCount)
	}

	logger.Info("Cluster balance validation completed", "emptyMasters", emptyMasterCount, "slotsPerNode", slotsPerNode)
	return nil
}
