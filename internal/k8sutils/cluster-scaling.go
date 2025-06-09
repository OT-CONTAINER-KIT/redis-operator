package k8sutils

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	rcvb2 "github.com/OT-CONTAINER-KIT/redis-operator/api/rediscluster/v1beta2"
	redis "github.com/redis/go-redis/v9"
	"k8s.io/client-go/kubernetes"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

// ReshardRedisCluster transfer the slots from the last node to the first node.
//
// NOTE: when all slot been transferred, the node become slave of the first master node.
func ReshardRedisCluster(ctx context.Context, client kubernetes.Interface, cr *rcvb2.RedisCluster, shardIdx int32, remove bool) error {
	redisClient := configureRedisClient(ctx, client, cr, cr.ObjectMeta.Name+"-leader-0")
	defer redisClient.Close()

	var cmd []string

	// Transfer Pod details
	transferPOD := RedisDetails{
		PodName:   cr.ObjectMeta.Name + "-leader-0",
		Namespace: cr.Namespace,
	}
	// Remove POD details
	removePOD := RedisDetails{
		PodName:   cr.Name + "-leader-" + strconv.Itoa(int(shardIdx)),
		Namespace: cr.Namespace,
	}
	cmd = []string{"redis-cli", "--cluster", "reshard"}

	if *cr.Spec.ClusterVersion == "v7" {
		cmd = append(cmd, getRedisHostname(transferPOD, cr, "leader")+fmt.Sprintf(":%d", *cr.Spec.Port))
	} else {
		cmd = append(cmd, getRedisServerAddress(ctx, client, transferPOD, *cr.Spec.Port))
	}

	if cr.Spec.KubernetesConfig.ExistingPasswordSecret != nil {
		pass, err := getRedisPassword(ctx, client, cr.Namespace, *cr.Spec.KubernetesConfig.ExistingPasswordSecret.Name, *cr.Spec.KubernetesConfig.ExistingPasswordSecret.Key)
		if err != nil {
			log.FromContext(ctx).Error(err, "Error in getting redis password")
			return err
		}
		cmd = append(cmd, "-a")
		cmd = append(cmd, pass)
	}

	cmd = append(cmd, getRedisTLSArgs(cr.Spec.TLS, cr.ObjectMeta.Name+"-leader-0")...)

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
	slot, err := getRedisClusterSlots(ctx, redisClient, removeNodeID)
	if err != nil {
		return err
	}
	cmd = append(cmd, "--cluster-slots")
	cmd = append(cmd, strconv.Itoa(slot))

	cmd = append(cmd, "--cluster-yes")

	log.FromContext(ctx).V(1).Info("Redis cluster reshard command is", "Command", cmd)

	if slot == 0 {
		log.FromContext(ctx).V(1).Info("Skipped the execution of", "Cmd", cmd)
		return nil
	}
	cmdOut, err := executeCommand1(ctx, client, cr, cmd, cr.ObjectMeta.Name+"-leader-0")
	if err != nil {
		log.FromContext(ctx).Error(err, "Failed to run reshard command", "Command", cmd, "Output", cmdOut)
		return err
	}
	if remove {
		err = RemoveRedisNodeFromCluster(ctx, client, cr, removePOD)
		if err != nil {
			return err
		}
	}
	return nil
}

func getRedisClusterSlots(ctx context.Context, redisClient *redis.Client, nodeID string) (int, error) {
	totalSlots := 0

	redisSlots, err := redisClient.ClusterSlots(ctx).Result()
	if err != nil {
		log.FromContext(ctx).Error(err, "Failed to Get Cluster Slots")
		return 0, err
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

	log.FromContext(ctx).V(1).Info("Total cluster slots to be transferred from", "node", nodeID, "is", totalSlots)
	return totalSlots, nil
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
func RebalanceRedisClusterEmptyMasters(ctx context.Context, client kubernetes.Interface, cr *rcvb2.RedisCluster) error {
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
		cmd = append(cmd, getRedisServerAddress(ctx, client, pod, *cr.Spec.Port))
	}

	cmd = append(cmd, "--cluster-use-empty-masters")

	if cr.Spec.KubernetesConfig.ExistingPasswordSecret != nil {
		pass, err := getRedisPassword(ctx, client, cr.Namespace, *cr.Spec.KubernetesConfig.ExistingPasswordSecret.Name, *cr.Spec.KubernetesConfig.ExistingPasswordSecret.Key)
		if err != nil {
			log.FromContext(ctx).Error(err, "Error in getting redis password")
			return err
		}
		cmd = append(cmd, "-a")
		cmd = append(cmd, pass)
	}

	cmd = append(cmd, getRedisTLSArgs(cr.Spec.TLS, cr.ObjectMeta.Name+"-leader-0")...)

	log.FromContext(ctx).Info("Redis cluster rebalance command is", "Command", cmd)
	cmdOut, err := executeCommand1(ctx, client, cr, cmd, cr.ObjectMeta.Name+"-leader-1")
	if err != nil {
		log.FromContext(ctx).Error(err, "Failed to run rebalance command", "Command", cmd, "Output", cmdOut)
		return err
	}

	log.FromContext(ctx).Info("Successfully rebalanced redis cluster")
	return nil
}

func CheckIfEmptyMasters(ctx context.Context, client kubernetes.Interface, cr *rcvb2.RedisCluster) {
	totalRedisLeaderNodes, err := CheckRedisNodeCount(ctx, client, cr, "leader")
	if err != nil {
		log.FromContext(ctx).Error(err, "Failed to get total redis leader nodes")
		return
	}
	redisClient := configureRedisClient(ctx, client, cr, cr.ObjectMeta.Name+"-leader-0")
	defer redisClient.Close()

	for i := 0; i < int(totalRedisLeaderNodes); i++ {
		pod := RedisDetails{
			PodName:   cr.ObjectMeta.Name + "-leader-" + strconv.Itoa(i),
			Namespace: cr.Namespace,
		}
		podNodeID := getRedisNodeID(ctx, client, cr, pod)
		podSlots, err := getRedisClusterSlots(ctx, redisClient, podNodeID)
		if err != nil {
			log.FromContext(ctx).Error(err, "Failed to get redis cluster slots")
			return
		}

		if podSlots == 0 {
			log.FromContext(ctx).Info("Found Empty Redis Leader Node", "pod", pod)
			err = RebalanceRedisClusterEmptyMasters(ctx, client, cr)
			if err != nil {
				log.FromContext(ctx).Error(err, "Failed to rebalance empty master node", "pod", pod)
			}
			break
		}
	}
}

// Rebalance Redis Cluster Would Rebalance the Redis Cluster without using the empty masters
func RebalanceRedisCluster(ctx context.Context, client kubernetes.Interface, cr *rcvb2.RedisCluster) error {
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
		cmd = append(cmd, getRedisServerAddress(ctx, client, pod, *cr.Spec.Port))
	}

	if cr.Spec.KubernetesConfig.ExistingPasswordSecret != nil {
		pass, err := getRedisPassword(ctx, client, cr.Namespace, *cr.Spec.KubernetesConfig.ExistingPasswordSecret.Name, *cr.Spec.KubernetesConfig.ExistingPasswordSecret.Key)
		if err != nil {
			log.FromContext(ctx).Error(err, "Error in getting redis password")
			return err
		}
		cmd = append(cmd, "-a")
		cmd = append(cmd, pass)
	}

	cmd = append(cmd, getRedisTLSArgs(cr.Spec.TLS, cr.ObjectMeta.Name+"-leader-0")...)

	log.FromContext(ctx).Info("Redis cluster rebalance command is", "Command", cmd)
	cmdOut, err := executeCommand1(ctx, client, cr, cmd, cr.ObjectMeta.Name+"-leader-1")
	if err != nil {
		log.FromContext(ctx).Error(err, "Failed to run rebalance command", "Command", cmd, "Output", cmdOut)
		return err
	}
	return nil
}

// Add redis cluster node would add a node to the existing redis cluster using redis-cli
func AddRedisNodeToCluster(ctx context.Context, client kubernetes.Interface, cr *rcvb2.RedisCluster) error {
	var cmd []string
	activeRedisNode, err := CheckRedisNodeCount(ctx, client, cr, "leader")
	if err != nil {
		log.FromContext(ctx).Error(err, "Failed to get active redis node")
		return err
	}

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
		cmd = append(cmd, getRedisServerAddress(ctx, client, newPod, *cr.Spec.Port))
		cmd = append(cmd, getRedisServerAddress(ctx, client, existingPod, *cr.Spec.Port))
	}

	if cr.Spec.KubernetesConfig.ExistingPasswordSecret != nil {
		pass, err := getRedisPassword(ctx, client, cr.Namespace, *cr.Spec.KubernetesConfig.ExistingPasswordSecret.Name, *cr.Spec.KubernetesConfig.ExistingPasswordSecret.Key)
		if err != nil {
			log.FromContext(ctx).Error(err, "Error in getting redis password")
			return err
		}
		cmd = append(cmd, "-a")
		cmd = append(cmd, pass)
	}

	cmd = append(cmd, getRedisTLSArgs(cr.Spec.TLS, cr.ObjectMeta.Name+"-leader-0")...)

	log.FromContext(ctx).Info("Redis cluster add-node command is", "Command", cmd)
	cmdOut, err := executeCommand1(ctx, client, cr, cmd, cr.ObjectMeta.Name+"-leader-0")
	if err != nil {
		log.FromContext(ctx).Error(err, "Failed to run add-node command", "Command", cmd, "Output", cmdOut)
		return err
	}

	log.FromContext(ctx).Info("Successfully added redis node to cluster", "NodeName", newPod.PodName)
	return nil
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
func RemoveRedisFollowerNodesFromCluster(ctx context.Context, client kubernetes.Interface, cr *rcvb2.RedisCluster, shardIdx int32) error {
	var cmd []string
	redisClient := configureRedisClient(ctx, client, cr, cr.ObjectMeta.Name+"-leader-0")
	defer redisClient.Close()

	existingPod := RedisDetails{
		PodName:   cr.ObjectMeta.Name + "-leader-0",
		Namespace: cr.Namespace,
	}
	lastLeaderPod := RedisDetails{
		PodName:   cr.ObjectMeta.Name + "-leader-" + strconv.Itoa(int(shardIdx)),
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
	cmd = append(cmd, getRedisTLSArgs(cr.Spec.TLS, cr.ObjectMeta.Name+"-leader-0")...)

	lastLeaderPodNodeID := getRedisNodeID(ctx, client, cr, lastLeaderPod)
	followerNodeIDs := getAttachedFollowerNodeIDs(ctx, redisClient, lastLeaderPodNodeID)

	cmd = append(cmd, "--cluster", "del-node")
	if *cr.Spec.ClusterVersion == "v7" {
		cmd = append(cmd, getRedisHostname(existingPod, cr, "leader")+fmt.Sprintf(":%d", *cr.Spec.Port))
	} else {
		cmd = append(cmd, getRedisServerAddress(ctx, client, existingPod, *cr.Spec.Port))
	}

	for _, followerNodeID := range followerNodeIDs {
		cmd = append(cmd, followerNodeID)
		log.FromContext(ctx).Info("Redis cluster follower remove command is", "Command", cmd)
		cmdOut, err := executeCommand1(ctx, client, cr, cmd, cr.ObjectMeta.Name+"-leader-0")
		if err != nil {
			log.FromContext(ctx).Error(err, "Failed to run remove follower node command", "Command", cmd, "Output", cmdOut)
			return err
		}
		cmd = cmd[:len(cmd)-1]
	}

	log.FromContext(ctx).Info("Successfully removed redis follower nodes from cluster", "followerNodeIDs", followerNodeIDs)
	return nil
}

// Remove redis cluster node would remove last node to the existing redis cluster using redis-cli
func RemoveRedisNodeFromCluster(ctx context.Context, client kubernetes.Interface, cr *rcvb2.RedisCluster, removePod RedisDetails) error {
	var cmd []string
	redisClient := configureRedisClient(ctx, client, cr, cr.ObjectMeta.Name+"-leader-0")
	defer redisClient.Close()
	// currentRedisCount := CheckRedisNodeCount(ctx, client, cr, "leader")

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
		cmd = append(cmd, getRedisServerAddress(ctx, client, existingPod, *cr.Spec.Port))
	}

	removePodNodeID := getRedisNodeID(ctx, client, cr, removePod)
	cmd = append(cmd, removePodNodeID)

	if cr.Spec.KubernetesConfig.ExistingPasswordSecret != nil {
		pass, err := getRedisPassword(ctx, client, cr.Namespace, *cr.Spec.KubernetesConfig.ExistingPasswordSecret.Name, *cr.Spec.KubernetesConfig.ExistingPasswordSecret.Key)
		if err != nil {
			log.FromContext(ctx).Error(err, "Error in getting redis password")
			return err
		}
		cmd = append(cmd, "-a")
		cmd = append(cmd, pass)
	}

	cmd = append(cmd, getRedisTLSArgs(cr.Spec.TLS, cr.ObjectMeta.Name+"-leader-0")...)

	slot, err := getRedisClusterSlots(ctx, redisClient, removePodNodeID)
	if err != nil {
		log.FromContext(ctx).Error(err, "Failed to get redis cluster slots", "nodeName", removePod.PodName)
		return err
	}
	if slot != 0 {
		log.FromContext(ctx).V(1).Info("Skipping execution remove leader not empty", "nodeName", removePod.PodName)
		return nil
	}

	log.FromContext(ctx).Info("Redis cluster leader remove command is", "Command", cmd)
	cmdOut, err := executeCommand1(ctx, client, cr, cmd, cr.ObjectMeta.Name+"-leader-0")
	if err != nil {
		log.FromContext(ctx).Error(err, "Failed to run remove leader node command", "Command", cmd, "Output", cmdOut)
		return err
	}

	log.FromContext(ctx).Info("Successfully removed redis node from cluster", "nodeName", removePod.PodName)
	return nil
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

	cmd = []string{"redis-cli", "-h"}

	if *cr.Spec.ClusterVersion == "v7" {
		cmd = append(cmd, getRedisHostname(pod, cr, "leader"))
	} else {
		cmd = append(cmd, getRedisServerIP(ctx, client, pod))
	}
	cmd = append(cmd, "-p")
	cmd = append(cmd, strconv.Itoa(*cr.Spec.Port))

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

	log.FromContext(ctx).Info("Redis cluster failover command is", "Command", cmd)
	execOut, err := executeCommand1(ctx, client, cr, cmd, slavePodName)
	if err != nil {
		log.FromContext(ctx).Error(err, "Could not execute command", "Command", cmd, "Output", execOut)
		return err
	}
	return nil
}
