package k8sutils

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	redisv1beta2 "github.com/OT-CONTAINER-KIT/redis-operator/api/v1beta2"
	"github.com/go-logr/logr"
	redis "github.com/redis/go-redis/v9"
	"k8s.io/client-go/kubernetes"
)

// ReshardRedisCluster transfer the slots from the last node to the first node.
//
// NOTE: when all slot been transferred, the node become slave of the first master node.
func ReshardRedisCluster(client kubernetes.Interface, logger logr.Logger, cr *redisv1beta2.RedisCluster, remove bool) {
	ctx := context.TODO()
	redisClient := configureRedisClient(client, logger, cr, cr.ObjectMeta.Name+"-leader-0")
	defer redisClient.Close()

	var cmd []string
	currentRedisCount := CheckRedisNodeCount(ctx, client, logger, cr, "leader")

	// Transfer Pod details
	transferPOD := RedisDetails{
		PodName:   cr.ObjectMeta.Name + "-leader-0",
		Namespace: cr.Namespace,
	}
	// Remove POD details
	removePOD := RedisDetails{
		PodName:   cr.Name + "-leader-" + strconv.Itoa(int(currentRedisCount)-1),
		Namespace: cr.Namespace,
	}
	cmd = []string{"redis-cli", "--cluster", "reshard"}

	if *cr.Spec.ClusterVersion == "v7" {
		cmd = append(cmd, getRedisHostname(transferPOD, cr, "leader")+fmt.Sprintf(":%d", *cr.Spec.Port))
	} else {
		cmd = append(cmd, getRedisServerAddress(client, logger, transferPOD, *cr.Spec.Port))
	}

	if cr.Spec.KubernetesConfig.ExistingPasswordSecret != nil {
		pass, err := getRedisPassword(client, logger, cr.Namespace, *cr.Spec.KubernetesConfig.ExistingPasswordSecret.Name, *cr.Spec.KubernetesConfig.ExistingPasswordSecret.Key)
		if err != nil {
			logger.Error(err, "Error in getting redis password")
		}
		cmd = append(cmd, "-a")
		cmd = append(cmd, pass)
	}

	cmd = append(cmd, getRedisTLSArgs(cr.Spec.TLS, cr.ObjectMeta.Name+"-leader-0")...)

	//--cluster-from <node-id> --cluster-to <node-id> --cluster-slots <number of slots> --cluster-yes

	// Remove Node
	removeNodeID := getRedisNodeID(ctx, client, logger, cr, removePOD)
	cmd = append(cmd, "--cluster-from")
	cmd = append(cmd, removeNodeID)

	// Transfer Node
	transferNodeID := getRedisNodeID(ctx, client, logger, cr, transferPOD)
	cmd = append(cmd, "--cluster-to")
	cmd = append(cmd, transferNodeID)

	// Cluster Slots
	slot := getRedisClusterSlots(ctx, redisClient, logger, removeNodeID)
	cmd = append(cmd, "--cluster-slots")
	cmd = append(cmd, slot)

	cmd = append(cmd, "--cluster-yes")

	logger.V(1).Info("Redis cluster reshard command is", "Command", cmd)

	if slot == "0" {
		logger.V(1).Info("Skipped the execution of", "Cmd", cmd)
		return
	}
	executeCommand(client, logger, cr, cmd, cr.ObjectMeta.Name+"-leader-0")

	if remove {
		RemoveRedisNodeFromCluster(ctx, client, logger, cr, removePOD)
	}
}

func getRedisClusterSlots(ctx context.Context, redisClient *redis.Client, logger logr.Logger, nodeID string) string {
	totalSlots := 0

	redisSlots, err := redisClient.ClusterSlots(ctx).Result()
	if err != nil {
		logger.Error(err, "Failed to Get Cluster Slots")
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

	logger.V(1).Info("Total cluster slots to be transferred from", "node", nodeID, "is", totalSlots)
	return strconv.Itoa(totalSlots)
}

// getRedisNodeID would return nodeID of a redis node by passing pod
func getRedisNodeID(ctx context.Context, client kubernetes.Interface, logger logr.Logger, cr *redisv1beta2.RedisCluster, pod RedisDetails) string {
	redisClient := configureRedisClient(client, logger, cr, pod.PodName)
	defer redisClient.Close()

	pong, err := redisClient.Ping(ctx).Result()
	if err != nil || pong != "PONG" {
		logger.Error(err, "Failed to ping Redis server")
		return ""
	}

	cmd := redis.NewStringCmd(ctx, "cluster", "myid")
	err = redisClient.Process(ctx, cmd)
	if err != nil {
		logger.Error(err, "Redis command failed with this error")
		return ""
	}

	output, err := cmd.Result()
	if err != nil {
		logger.Error(err, "Redis command failed with this error")
		return ""
	}
	logger.V(1).Info("Redis node ID ", "is", output)
	return output
}

// Rebalance the Redis CLuster using the Empty Master Nodes
func RebalanceRedisClusterEmptyMasters(client kubernetes.Interface, logger logr.Logger, cr *redisv1beta2.RedisCluster) {
	// cmd = redis-cli --cluster rebalance <redis>:<port> --cluster-use-empty-masters -a <pass>
	var cmd []string
	pod := RedisDetails{
		PodName:   cr.ObjectMeta.Name + "-leader-1",
		Namespace: cr.Namespace,
	}
	cmd = []string{"redis-cli", "--cluster", "rebalance"}

	if *cr.Spec.ClusterVersion == "v7" {
		cmd = append(cmd, getRedisHostname(pod, cr, "leader")+fmt.Sprintf(":%d", *cr.Spec.Port))
	} else {
		cmd = append(cmd, getRedisServerAddress(client, logger, pod, *cr.Spec.Port))
	}

	cmd = append(cmd, "--cluster-use-empty-masters")

	if cr.Spec.KubernetesConfig.ExistingPasswordSecret != nil {
		pass, err := getRedisPassword(client, logger, cr.Namespace, *cr.Spec.KubernetesConfig.ExistingPasswordSecret.Name, *cr.Spec.KubernetesConfig.ExistingPasswordSecret.Key)
		if err != nil {
			logger.Error(err, "Error in getting redis password")
		}
		cmd = append(cmd, "-a")
		cmd = append(cmd, pass)
	}

	cmd = append(cmd, getRedisTLSArgs(cr.Spec.TLS, cr.ObjectMeta.Name+"-leader-0")...)

	logger.V(1).Info("Redis cluster rebalance command is", "Command", cmd)
	executeCommand(client, logger, cr, cmd, cr.ObjectMeta.Name+"-leader-1")
}

func CheckIfEmptyMasters(ctx context.Context, client kubernetes.Interface, logger logr.Logger, cr *redisv1beta2.RedisCluster) {
	totalRedisLeaderNodes := CheckRedisNodeCount(ctx, client, logger, cr, "leader")
	redisClient := configureRedisClient(client, logger, cr, cr.ObjectMeta.Name+"-leader-0")
	defer redisClient.Close()

	for i := 0; i < int(totalRedisLeaderNodes); i++ {
		pod := RedisDetails{
			PodName:   cr.ObjectMeta.Name + "-leader-" + strconv.Itoa(i),
			Namespace: cr.Namespace,
		}
		podNodeID := getRedisNodeID(ctx, client, logger, cr, pod)
		podSlots := getRedisClusterSlots(ctx, redisClient, logger, podNodeID)

		if podSlots == "0" || podSlots == "" {
			logger.V(1).Info("Found Empty Redis Leader Node", "pod", pod)
			RebalanceRedisClusterEmptyMasters(client, logger, cr)
			break
		}
	}
}

// Rebalance Redis Cluster Would Rebalance the Redis Cluster without using the empty masters
func RebalanceRedisCluster(client kubernetes.Interface, logger logr.Logger, cr *redisv1beta2.RedisCluster) {
	// cmd = redis-cli --cluster rebalance <redis>:<port> -a <pass>
	var cmd []string
	pod := RedisDetails{
		PodName:   cr.ObjectMeta.Name + "-leader-1",
		Namespace: cr.Namespace,
	}
	cmd = []string{"redis-cli", "--cluster", "rebalance"}

	if *cr.Spec.ClusterVersion == "v7" {
		cmd = append(cmd, getRedisHostname(pod, cr, "leader")+fmt.Sprintf(":%d", *cr.Spec.Port))
	} else {
		cmd = append(cmd, getRedisServerAddress(client, logger, pod, *cr.Spec.Port))
	}

	if cr.Spec.KubernetesConfig.ExistingPasswordSecret != nil {
		pass, err := getRedisPassword(client, logger, cr.Namespace, *cr.Spec.KubernetesConfig.ExistingPasswordSecret.Name, *cr.Spec.KubernetesConfig.ExistingPasswordSecret.Key)
		if err != nil {
			logger.Error(err, "Error in getting redis password")
		}
		cmd = append(cmd, "-a")
		cmd = append(cmd, pass)
	}

	cmd = append(cmd, getRedisTLSArgs(cr.Spec.TLS, cr.ObjectMeta.Name+"-leader-0")...)

	logger.V(1).Info("Redis cluster rebalance command is", "Command", cmd)
	executeCommand(client, logger, cr, cmd, cr.ObjectMeta.Name+"-leader-1")
}

// Add redis cluster node would add a node to the existing redis cluster using redis-cli
func AddRedisNodeToCluster(ctx context.Context, client kubernetes.Interface, logger logr.Logger, cr *redisv1beta2.RedisCluster) {
	var cmd []string
	activeRedisNode := CheckRedisNodeCount(ctx, client, logger, cr, "leader")

	newPod := RedisDetails{
		PodName:   cr.ObjectMeta.Name + "-leader-" + strconv.Itoa(int(activeRedisNode)),
		Namespace: cr.Namespace,
	}
	existingPod := RedisDetails{
		PodName:   cr.ObjectMeta.Name + "-leader-0",
		Namespace: cr.Namespace,
	}

	cmd = []string{"redis-cli", "--cluster", "add-node"}

	if *cr.Spec.ClusterVersion == "v7" {
		cmd = append(cmd, getRedisHostname(newPod, cr, "leader")+fmt.Sprintf(":%d", *cr.Spec.Port))
		cmd = append(cmd, getRedisHostname(existingPod, cr, "leader")+fmt.Sprintf(":%d", *cr.Spec.Port))
	} else {
		cmd = append(cmd, getRedisServerAddress(client, logger, newPod, *cr.Spec.Port))
		cmd = append(cmd, getRedisServerAddress(client, logger, existingPod, *cr.Spec.Port))
	}

	if cr.Spec.KubernetesConfig.ExistingPasswordSecret != nil {
		pass, err := getRedisPassword(client, logger, cr.Namespace, *cr.Spec.KubernetesConfig.ExistingPasswordSecret.Name, *cr.Spec.KubernetesConfig.ExistingPasswordSecret.Key)
		if err != nil {
			logger.Error(err, "Error in getting redis password")
		}
		cmd = append(cmd, "-a")
		cmd = append(cmd, pass)
	}

	cmd = append(cmd, getRedisTLSArgs(cr.Spec.TLS, cr.ObjectMeta.Name+"-leader-0")...)

	logger.V(1).Info("Redis cluster add-node command is", "Command", cmd)
	executeCommand(client, logger, cr, cmd, cr.ObjectMeta.Name+"-leader-0")
}

// getAttachedFollowerNodeIDs would return a slice of redis followers attached to a redis leader
func getAttachedFollowerNodeIDs(ctx context.Context, redisClient *redis.Client, logger logr.Logger, masterNodeID string) []string {
	// 3acb029fead40752f432c84f9bed2e639119a573 192.168.84.239:6379@16379,redis-cluster-v1beta2-follower-5 slave e3299968586dd457a8dba04fc6c747cecd38510f 0 1713595736542 6 connected
	slaveNodes, err := redisClient.ClusterSlaves(ctx, masterNodeID).Result()
	if err != nil {
		logger.Error(err, "Failed to get attached follower node IDs", "masterNodeID", masterNodeID)
		return nil
	}
	slaveIDs := make([]string, 0, len(slaveNodes))
	for _, slave := range slaveNodes {
		stringSlice := strings.Split(slave, " ")
		slaveIDs = append(slaveIDs, stringSlice[0])
	}
	logger.V(1).Info("Slaves Nodes attached to", "node", masterNodeID, "are", slaveIDs)
	return slaveIDs
}

// Remove redis follower node would remove all follower nodes of last leader node using redis-cli
func RemoveRedisFollowerNodesFromCluster(ctx context.Context, client kubernetes.Interface, logger logr.Logger, cr *redisv1beta2.RedisCluster) {
	var cmd []string
	redisClient := configureRedisClient(client, logger, cr, cr.ObjectMeta.Name+"-leader-0")
	defer redisClient.Close()
	currentRedisCount := CheckRedisNodeCount(ctx, client, logger, cr, "leader")

	existingPod := RedisDetails{
		PodName:   cr.ObjectMeta.Name + "-leader-0",
		Namespace: cr.Namespace,
	}
	lastLeaderPod := RedisDetails{
		PodName:   cr.ObjectMeta.Name + "-leader-" + strconv.Itoa(int(currentRedisCount)-1),
		Namespace: cr.Namespace,
	}

	cmd = []string{"redis-cli"}

	if cr.Spec.KubernetesConfig.ExistingPasswordSecret != nil {
		pass, err := getRedisPassword(client, logger, cr.Namespace, *cr.Spec.KubernetesConfig.ExistingPasswordSecret.Name, *cr.Spec.KubernetesConfig.ExistingPasswordSecret.Key)
		if err != nil {
			logger.Error(err, "Error in getting redis password")
		}
		cmd = append(cmd, "-a")
		cmd = append(cmd, pass)
	}
	cmd = append(cmd, getRedisTLSArgs(cr.Spec.TLS, cr.ObjectMeta.Name+"-leader-0")...)

	lastLeaderPodNodeID := getRedisNodeID(ctx, client, logger, cr, lastLeaderPod)
	followerNodeIDs := getAttachedFollowerNodeIDs(ctx, redisClient, logger, lastLeaderPodNodeID)

	cmd = append(cmd, "--cluster", "del-node")
	if *cr.Spec.ClusterVersion == "v7" {
		cmd = append(cmd, getRedisHostname(existingPod, cr, "leader")+fmt.Sprintf(":%d", *cr.Spec.Port))
	} else {
		cmd = append(cmd, getRedisServerAddress(client, logger, existingPod, *cr.Spec.Port))
	}

	for _, followerNodeID := range followerNodeIDs {
		cmd = append(cmd, followerNodeID)
		logger.V(1).Info("Redis cluster follower remove command is", "Command", cmd)
		executeCommand(client, logger, cr, cmd, cr.ObjectMeta.Name+"-leader-0")
		cmd = cmd[:len(cmd)-1]
	}
}

// Remove redis cluster node would remove last node to the existing redis cluster using redis-cli
func RemoveRedisNodeFromCluster(ctx context.Context, client kubernetes.Interface, logger logr.Logger, cr *redisv1beta2.RedisCluster, removePod RedisDetails) {
	var cmd []string
	redisClient := configureRedisClient(client, logger, cr, cr.ObjectMeta.Name+"-leader-0")
	defer redisClient.Close()
	// currentRedisCount := CheckRedisNodeCount(ctx, client, logger, cr, "leader")

	existingPod := RedisDetails{
		PodName:   cr.ObjectMeta.Name + "-leader-0",
		Namespace: cr.Namespace,
	}
	//removePod := RedisDetails{
	//	PodName:   cr.ObjectMeta.Name + "-leader-" + strconv.Itoa(int(currentRedisCount)-1),
	//	Namespace: cr.Namespace,
	//}

	cmd = []string{"redis-cli", "--cluster", "del-node"}

	if *cr.Spec.ClusterVersion == "v7" {
		cmd = append(cmd, getRedisHostname(existingPod, cr, "leader")+fmt.Sprintf(":%d", *cr.Spec.Port))
	} else {
		cmd = append(cmd, getRedisServerAddress(client, logger, existingPod, *cr.Spec.Port))
	}

	removePodNodeID := getRedisNodeID(ctx, client, logger, cr, removePod)
	cmd = append(cmd, removePodNodeID)

	if cr.Spec.KubernetesConfig.ExistingPasswordSecret != nil {
		pass, err := getRedisPassword(client, logger, cr.Namespace, *cr.Spec.KubernetesConfig.ExistingPasswordSecret.Name, *cr.Spec.KubernetesConfig.ExistingPasswordSecret.Key)
		if err != nil {
			logger.Error(err, "Error in getting redis password")
		}
		cmd = append(cmd, "-a")
		cmd = append(cmd, pass)
	}

	cmd = append(cmd, getRedisTLSArgs(cr.Spec.TLS, cr.ObjectMeta.Name+"-leader-0")...)

	logger.V(1).Info("Redis cluster leader remove command is", "Command", cmd)
	if getRedisClusterSlots(ctx, redisClient, logger, removePodNodeID) != "0" {
		logger.V(1).Info("Skipping execution remove leader not empty", "cmd", cmd)
	}
	executeCommand(client, logger, cr, cmd, cr.ObjectMeta.Name+"-leader-0")
}

// verifyLeaderPod return true if the pod is leader/master
func VerifyLeaderPod(ctx context.Context, client kubernetes.Interface, logger logr.Logger, cr *redisv1beta2.RedisCluster) bool {
	podName := cr.Name + "-leader-" + strconv.Itoa(int(CheckRedisNodeCount(ctx, client, logger, cr, "leader"))-1)

	redisClient := configureRedisClient(client, logger, cr, podName)
	defer redisClient.Close()
	return verifyLeaderPodInfo(ctx, redisClient, logger, podName)
}

func verifyLeaderPodInfo(ctx context.Context, redisClient *redis.Client, logger logr.Logger, podName string) bool {
	info, err := redisClient.Info(ctx, "replication").Result()
	if err != nil {
		logger.Error(err, "Failed to Get the role Info of the", "redis pod", podName)
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

func ClusterFailover(ctx context.Context, client kubernetes.Interface, logger logr.Logger, cr *redisv1beta2.RedisCluster) {
	slavePodName := cr.Name + "-leader-" + strconv.Itoa(int(CheckRedisNodeCount(ctx, client, logger, cr, "leader"))-1)
	// cmd = redis-cli cluster failover  -a <pass>
	var cmd []string
	pod := RedisDetails{
		PodName:   slavePodName,
		Namespace: cr.Namespace,
	}

	cmd = []string{"redis-cli", "cluster", "failover"}

	if *cr.Spec.ClusterVersion == "v7" {
		cmd = append(cmd, getRedisHostname(pod, cr, "leader")+fmt.Sprintf(":%d", *cr.Spec.Port))
	} else {
		cmd = append(cmd, getRedisServerAddress(client, logger, pod, *cr.Spec.Port))
	}

	if cr.Spec.KubernetesConfig.ExistingPasswordSecret != nil {
		pass, err := getRedisPassword(client, logger, cr.Namespace, *cr.Spec.KubernetesConfig.ExistingPasswordSecret.Name, *cr.Spec.KubernetesConfig.ExistingPasswordSecret.Key)
		if err != nil {
			logger.Error(err, "Error in getting redis password")
		}
		cmd = append(cmd, "-a")
		cmd = append(cmd, pass)
	}

	cmd = append(cmd, getRedisTLSArgs(cr.Spec.TLS, slavePodName)...)

	logger.V(1).Info("Redis cluster failover command is", "Command", cmd)
	executeCommand(client, logger, cr, cmd, slavePodName)
}
