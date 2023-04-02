package k8sutils

import (
	"bytes"
	"context"
	"encoding/csv"
	"fmt"
	"net"
	"strconv"
	"strings"

	redisv1beta1 "github.com/OT-CONTAINER-KIT/redis-operator/api/v1beta1"

	"github.com/go-logr/logr"
	"github.com/go-redis/redis"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/tools/remotecommand"
)

// RedisDetails will hold the information for Redis Pod
type RedisDetails struct {
	PodName   string
	Namespace string
}

// getRedisServerIP will return the IP of redis service
func getRedisServerIP(redisInfo RedisDetails) string {
	logger := generateRedisManagerLogger(redisInfo.Namespace, redisInfo.PodName)
	redisPod, err := generateK8sClient().CoreV1().Pods(redisInfo.Namespace).Get(context.TODO(), redisInfo.PodName, metav1.GetOptions{})
	if err != nil {
		logger.Error(err, "Error in getting redis pod IP")
	}

	redisIP := redisPod.Status.PodIP
	// If we're NOT IPv4, assume were IPv6..
	if redisIP != "" {
		if net.ParseIP(redisIP).To4() == nil {
			logger.Info("Redis is IPv6", "ip", redisIP, "ipv6", net.ParseIP(redisIP).To16())
			redisIP = fmt.Sprintf("[%s]", redisIP)
		}
	}

	logger.Info("Successfully got the ip for redis", "ip", redisIP)
	return redisIP
}

// getRedisHostname will return the complete FQDN for redis
func getRedisHostname(redisInfo RedisDetails, cr *redisv1beta1.RedisCluster, role string) string {
	fqdn := fmt.Sprintf("%s.%s-%s-headless.%s.svc", redisInfo.PodName, cr.ObjectMeta.Name, role, cr.Namespace)
	return fqdn
}

// CreateSingleLeaderRedisCommand will create command for single leader cluster creation
func CreateSingleLeaderRedisCommand(cr *redisv1beta1.RedisCluster) []string {
	logger := generateRedisManagerLogger(cr.Namespace, cr.ObjectMeta.Name)
	cmd := []string{"redis-cli", "CLUSTER", "ADDSLOTS"}
	for i := 0; i < 16384; i++ {
		cmd = append(cmd, strconv.Itoa(i))
	}

	logger.Info("Redis Add Slots command for single node cluster is", "Command", cmd)
	return cmd
}

// CreateMultipleLeaderRedisCommand will create command for single leader cluster creation
func CreateMultipleLeaderRedisCommand(cr *redisv1beta1.RedisCluster) []string {
	logger := generateRedisManagerLogger(cr.Namespace, cr.ObjectMeta.Name)
	cmd := []string{"redis-cli", "--cluster", "create"}
	replicas := cr.Spec.GetReplicaCounts("leader")

	for podCount := 0; podCount <= int(replicas)-1; podCount++ {
		pod := RedisDetails{
			PodName:   cr.ObjectMeta.Name + "-leader-" + strconv.Itoa(podCount),
			Namespace: cr.Namespace,
		}
		if *cr.Spec.ClusterVersion == "v7" {
			cmd = append(cmd, getRedisHostname(pod, cr, "leader")+":6379")
		} else {
			cmd = append(cmd, getRedisServerIP(pod)+":6379")
		}
	}
	cmd = append(cmd, "--cluster-yes")

	logger.Info("Redis Add Slots command for single node cluster is", "Command", cmd)
	return cmd
}

// ExecuteRedisClusterCommand will execute redis cluster creation command
func ExecuteRedisClusterCommand(cr *redisv1beta1.RedisCluster) {
	logger := generateRedisManagerLogger(cr.Namespace, cr.ObjectMeta.Name)
	var cmd []string
	replicas := cr.Spec.GetReplicaCounts("leader")
	switch int(replicas) {
	case 1:
		err := executeFailoverCommand(cr, "leader")
		if err != nil {
			logger.Error(err, "error executing failover command")
		}
		cmd = CreateSingleLeaderRedisCommand(cr)
	default:
		cmd = CreateMultipleLeaderRedisCommand(cr)
	}

	if cr.Spec.KubernetesConfig.ExistingPasswordSecret != nil {
		pass, err := getRedisPassword(cr.Namespace, *cr.Spec.KubernetesConfig.ExistingPasswordSecret.Name, *cr.Spec.KubernetesConfig.ExistingPasswordSecret.Key)
		if err != nil {
			logger.Error(err, "Error in getting redis password")
		}
		cmd = append(cmd, "-a")
		cmd = append(cmd, pass)
	}
	cmd = append(cmd, getRedisTLSArgs(cr.Spec.TLS, cr.ObjectMeta.Name+"-leader-0")...)
	logger.Info("Redis cluster creation command is", "Command", cmd)
	executeCommand(cr, cmd, cr.ObjectMeta.Name+"-leader-0")
}

func getRedisTLSArgs(tlsConfig *redisv1beta1.TLSConfig, clientHost string) []string {
	cmd := []string{}
	if tlsConfig != nil {
		cmd = append(cmd, "--tls")
		cmd = append(cmd, "--cacert")
		cmd = append(cmd, "/tls/ca.crt")
		cmd = append(cmd, "-h")
		cmd = append(cmd, clientHost)
	}
	return cmd
}

// createRedisReplicationCommand will create redis replication creation command
func createRedisReplicationCommand(cr *redisv1beta1.RedisCluster, leaderPod RedisDetails, followerPod RedisDetails) []string {
	logger := generateRedisManagerLogger(cr.Namespace, cr.ObjectMeta.Name)
	cmd := []string{"redis-cli", "--cluster", "add-node"}
	if *cr.Spec.ClusterVersion == "v7" {
		cmd = append(cmd, getRedisHostname(followerPod, cr, "follower")+":6379")
		cmd = append(cmd, getRedisHostname(leaderPod, cr, "leader")+":6379")
	} else {
		cmd = append(cmd, getRedisServerIP(followerPod)+":6379")
		cmd = append(cmd, getRedisServerIP(leaderPod)+":6379")
	}
	cmd = append(cmd, "--cluster-slave")

	if cr.Spec.KubernetesConfig.ExistingPasswordSecret != nil {
		pass, err := getRedisPassword(cr.Namespace, *cr.Spec.KubernetesConfig.ExistingPasswordSecret.Name, *cr.Spec.KubernetesConfig.ExistingPasswordSecret.Key)
		if err != nil {
			logger.Error(err, "Error in getting redis password")
		}
		cmd = append(cmd, "-a")
		cmd = append(cmd, pass)
	}
	cmd = append(cmd, getRedisTLSArgs(cr.Spec.TLS, leaderPod.PodName)...)
	logger.V(2).Info("Redis replication creation command is", "Command", cmd)
	return cmd
}

// ExecuteRedisReplicationCommand will execute the replication command
func ExecuteRedisReplicationCommand(cr *redisv1beta1.RedisCluster) {
	var podIP string
	logger := generateRedisManagerLogger(cr.Namespace, cr.ObjectMeta.Name)
	followerCounts := cr.Spec.GetReplicaCounts("follower")
	leaderCounts := cr.Spec.GetReplicaCounts("leader")
	nodes := checkRedisCluster(cr)
	for followerIdx := 0; followerIdx <= int(followerCounts)-1; followerIdx++ {
		followerPod := RedisDetails{
			PodName:   cr.ObjectMeta.Name + "-follower-" + strconv.Itoa(followerIdx),
			Namespace: cr.Namespace,
		}
		leaderPod := RedisDetails{
			PodName:   cr.ObjectMeta.Name + "-leader-" + strconv.Itoa(int(followerIdx)%int(leaderCounts)),
			Namespace: cr.Namespace,
		}
		podIP = getRedisServerIP(followerPod)
		if !checkRedisNodePresence(cr, nodes, podIP) {
			logger.Info("Adding node to cluster.", "Node.IP", podIP, "Follower.Pod", followerPod)
			cmd := createRedisReplicationCommand(cr, leaderPod, followerPod)
			executeCommand(cr, cmd, cr.ObjectMeta.Name+"-leader-0")
		} else {
			logger.Info("Skipping Adding node to cluster, already present.", "Follower.Pod", followerPod)
		}
	}
}

// checkRedisCluster will check the redis cluster have sufficient nodes or not
func checkRedisCluster(cr *redisv1beta1.RedisCluster) [][]string {
	var client *redis.Client
	logger := generateRedisManagerLogger(cr.Namespace, cr.ObjectMeta.Name)
	client = configureRedisClient(cr, cr.ObjectMeta.Name+"-leader-0")
	defer client.Close()
	cmd := redis.NewStringCmd("cluster", "nodes")
	err := client.Process(cmd)
	if err != nil {
		logger.Error(err, "Redis command failed with this error")
	}

	output, err := cmd.Result()
	if err != nil {
		logger.Error(err, "Redis command failed with this error")
	}
	logger.Info("Redis cluster nodes are listed", "Output", output)

	csvOutput := csv.NewReader(strings.NewReader(output))
	csvOutput.Comma = ' '
	csvOutput.FieldsPerRecord = -1
	csvOutputRecords, err := csvOutput.ReadAll()
	if err != nil {
		logger.Error(err, "Error parsing Node Counts", "output", output)
	}
	return csvOutputRecords
}

// ExecuteFailoverOperation will execute redis failover operations
func ExecuteFailoverOperation(cr *redisv1beta1.RedisCluster) error {
	logger := generateRedisManagerLogger(cr.Namespace, cr.ObjectMeta.Name)
	err := executeFailoverCommand(cr, "leader")
	if err != nil {
		logger.Error(err, "Redis command failed for leader nodes")
		return err
	}
	err = executeFailoverCommand(cr, "follower")
	if err != nil {
		logger.Error(err, "Redis command failed for follower nodes")
		return err
	}
	return nil
}

// executeFailoverCommand will execute failover command
func executeFailoverCommand(cr *redisv1beta1.RedisCluster, role string) error {
	logger := generateRedisManagerLogger(cr.Namespace, cr.ObjectMeta.Name)
	replicas := cr.Spec.GetReplicaCounts(role)
	podName := fmt.Sprintf("%s-%s-", cr.ObjectMeta.Name, role)
	for podCount := 0; podCount <= int(replicas)-1; podCount++ {
		logger.Info("Executing redis failover operations", "Redis Node", podName+strconv.Itoa(podCount))
		client := configureRedisClient(cr, podName+strconv.Itoa(podCount))
		defer client.Close()
		cmd := redis.NewStringCmd("cluster", "reset")
		err := client.Process(cmd)
		if err != nil {
			logger.Error(err, "Redis command failed with this error")
			flushcommand := redis.NewStringCmd("flushall")
			err := client.Process(flushcommand)
			if err != nil {
				logger.Error(err, "Redis flush command failed with this error")
				return err
			}
		}
		err = client.Process(cmd)
		if err != nil {
			logger.Error(err, "Redis command failed with this error")
			return err
		}
		output, err := cmd.Result()
		if err != nil {
			logger.Error(err, "Redis command failed with this error")
			return err
		}
		logger.Info("Redis cluster failover executed", "Output", output)
	}
	return nil
}

// CheckRedisNodeCount will check the count of redis nodes
func CheckRedisNodeCount(cr *redisv1beta1.RedisCluster, nodeType string) int32 {
	var redisNodeType string
	logger := generateRedisManagerLogger(cr.Namespace, cr.ObjectMeta.Name)
	clusterNodes := checkRedisCluster(cr)
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
			if strings.Contains(node[2], redisNodeType) {
				count++
			}
		}
		logger.Info("Number of redis nodes are", "Nodes", strconv.Itoa(count), "Type", nodeType)
	} else {
		logger.Info("Total number of redis nodes are", "Nodes", strconv.Itoa(count))
	}
	return int32(count)
}

// CheckRedisClusterState will check the redis cluster state
func CheckRedisClusterState(cr *redisv1beta1.RedisCluster) int {
	logger := generateRedisManagerLogger(cr.Namespace, cr.ObjectMeta.Name)
	clusterNodes := checkRedisCluster(cr)
	count := 0

	for _, node := range clusterNodes {
		if strings.Contains(node[2], "fail") || strings.Contains(node[7], "disconnected") {
			count++
		}
	}
	logger.Info("Number of failed nodes in cluster", "Failed Node Count", count)
	return count
}

// configureRedisClient will configure the Redis Client
func configureRedisClient(cr *redisv1beta1.RedisCluster, podName string) *redis.Client {
	logger := generateRedisManagerLogger(cr.Namespace, cr.ObjectMeta.Name)
	redisInfo := RedisDetails{
		PodName:   podName,
		Namespace: cr.Namespace,
	}
	var client *redis.Client

	if cr.Spec.KubernetesConfig.ExistingPasswordSecret != nil {
		pass, err := getRedisPassword(cr.Namespace, *cr.Spec.KubernetesConfig.ExistingPasswordSecret.Name, *cr.Spec.KubernetesConfig.ExistingPasswordSecret.Key)
		if err != nil {
			logger.Error(err, "Error in getting redis password")
		}
		client = redis.NewClient(&redis.Options{
			Addr:      getRedisServerIP(redisInfo) + ":6379",
			Password:  pass,
			DB:        0,
			TLSConfig: getRedisTLSConfig(cr, redisInfo),
		})
	} else {
		client = redis.NewClient(&redis.Options{
			Addr:      getRedisServerIP(redisInfo) + ":6379",
			Password:  "",
			DB:        0,
			TLSConfig: getRedisTLSConfig(cr, redisInfo),
		})
	}
	return client
}

// executeCommand will execute the commands in pod
func executeCommand(cr *redisv1beta1.RedisCluster, cmd []string, podName string) {
	var (
		execOut bytes.Buffer
		execErr bytes.Buffer
	)
	logger := generateRedisManagerLogger(cr.Namespace, cr.ObjectMeta.Name)
	config, err := generateK8sConfig()
	if err != nil {
		logger.Error(err, "Could not find pod to execute")
		return
	}
	targetContainer, pod := getContainerID(cr, podName)
	if targetContainer < 0 {
		logger.Error(err, "Could not find pod to execute")
		return
	}

	req := generateK8sClient().CoreV1().RESTClient().Post().Resource("pods").Name(podName).Namespace(cr.Namespace).SubResource("exec")
	req.VersionedParams(&corev1.PodExecOptions{
		Container: pod.Spec.Containers[targetContainer].Name,
		Command:   cmd,
		Stdout:    true,
		Stderr:    true,
	}, scheme.ParameterCodec)
	exec, err := remotecommand.NewSPDYExecutor(config, "POST", req.URL())
	if err != nil {
		logger.Error(err, "Failed to init executor")
		return
	}

	err = exec.Stream(remotecommand.StreamOptions{
		Stdout: &execOut,
		Stderr: &execErr,
		Tty:    false,
	})
	if err != nil {
		logger.Error(err, "Could not execute command", "Command", cmd, "Output", execOut.String(), "Error", execErr.String())
		return
	}
	logger.Info("Successfully executed the command", "Command", cmd, "Output", execOut.String())
}

// getContainerID will return the id of container from pod
func getContainerID(cr *redisv1beta1.RedisCluster, podName string) (int, *corev1.Pod) {
	logger := generateRedisManagerLogger(cr.Namespace, cr.ObjectMeta.Name)
	pod, err := generateK8sClient().CoreV1().Pods(cr.Namespace).Get(context.TODO(), podName, metav1.GetOptions{})
	if err != nil {
		logger.Error(err, "Could not get pod info")
	}

	targetContainer := -1
	for containerID, tr := range pod.Spec.Containers {
		logger.Info("Pod Counted successfully", "Count", containerID, "Container Name", tr.Name)
		if tr.Name == cr.ObjectMeta.Name+"-leader" {
			targetContainer = containerID
			break
		}
	}
	return targetContainer, pod
}

// checkRedisNodePresence will check if the redis node exist in cluster or not
func checkRedisNodePresence(cr *redisv1beta1.RedisCluster, nodeList [][]string, nodeName string) bool {
	logger := generateRedisManagerLogger(cr.Namespace, cr.ObjectMeta.Name)
	logger.Info("Checking if Node is in cluster", "Node", nodeName)
	for _, node := range nodeList {
		s := strings.Split(node[1], ":")
		if s[0] == nodeName {
			return true
		}
	}
	return false
}

// generateRedisManagerLogger will generate logging interface for Redis operations
func generateRedisManagerLogger(namespace, name string) logr.Logger {
	reqLogger := log.WithValues("Request.RedisManager.Namespace", namespace, "Request.RedisManager.Name", name)
	return reqLogger
}

// configureRedisClient will configure the Redis Client
func configureRedisReplicationClient(cr *redisv1beta1.RedisReplication, podName string) *redis.Client {
	logger := generateRedisManagerLogger(cr.Namespace, cr.ObjectMeta.Name)
	redisInfo := RedisDetails{
		PodName:   podName,
		Namespace: cr.Namespace,
	}
	var client *redis.Client

	if cr.Spec.KubernetesConfig.ExistingPasswordSecret != nil {
		pass, err := getRedisPassword(cr.Namespace, *cr.Spec.KubernetesConfig.ExistingPasswordSecret.Name, *cr.Spec.KubernetesConfig.ExistingPasswordSecret.Key)
		if err != nil {
			logger.Error(err, "Error in getting redis password")
		}
		client = redis.NewClient(&redis.Options{
			Addr:      getRedisServerIP(redisInfo) + ":6379",
			Password:  pass,
			DB:        0,
			TLSConfig: getRedisReplicationTLSConfig(cr, redisInfo),
		})
	} else {
		client = redis.NewClient(&redis.Options{
			Addr:      getRedisServerIP(redisInfo) + ":6379",
			Password:  "",
			DB:        0,
			TLSConfig: getRedisReplicationTLSConfig(cr, redisInfo),
		})
	}
	return client
}

// Get Redis nodes by it's role i.e. master, slave and sentinel
func GetRedisNodesByRole(cr *redisv1beta1.RedisReplication, redisRole string) []string {
	logger := generateRedisManagerLogger(cr.Namespace, cr.ObjectMeta.Name)
	statefulset, err := GetStatefulSet(cr.Namespace, cr.Name)
	if err != nil {
		logger.Error(err, "Failed to Get the Statefulset of the", "custom resource", cr.Name, "in namespace", cr.Namespace)
	}

	var pods []string
	replicas := cr.Spec.GetReplicationCounts("replication")

	for i := 0; i < int(replicas); i++ {

		podName := statefulset.Name + "-" + strconv.Itoa(i)
		podRole := checkRedisServerRole(cr, podName)
		if podRole == redisRole {
			pods = append(pods, podName)
		}
	}

	return pods
}

// Check the Redis Server Role i.e. master, slave and sentinel
func checkRedisServerRole(cr *redisv1beta1.RedisReplication, podName string) string {
	logger := generateRedisManagerLogger(cr.Namespace, cr.ObjectMeta.Name)

	redisClient := configureRedisReplicationClient(cr, podName)
	defer redisClient.Close()
	info, err := redisClient.Info("replication").Result()
	if err != nil {
		logger.Error(err, "Failed to Get the role Info of the", "redis pod", podName)
	}

	lines := strings.Split(info, "\r\n")
	role := ""
	for _, line := range lines {
		if strings.HasPrefix(line, "role:") {
			role = strings.TrimPrefix(line, "role:")
			break
		}
	}

	return role

}

// checkAttachedSlave would return redis pod name which has slave
func checkAttachedSlave(cr *redisv1beta1.RedisReplication, masterPods []string) string {
	logger := generateRedisManagerLogger(cr.Namespace, cr.ObjectMeta.Name)

	for _, podName := range masterPods {

		connected_slaves := ""
		redisClient := configureRedisReplicationClient(cr, podName)
		defer redisClient.Close()
		info, err := redisClient.Info("replication").Result()
		if err != nil {
			logger.Error(err, "Failed to Get the connected slaves Info of the", "redis pod", podName)
		}

		lines := strings.Split(info, "\r\n")

		for _, line := range lines {
			if strings.HasPrefix(line, "connected_slaves:") {
				connected_slaves = strings.TrimPrefix(line, "connected_slaves:")
				break
			}
		}

		nums, _ := strconv.Atoi(connected_slaves)
		if nums > 0 {
			return podName
		}

	}

	return ""

}

func CreateMasterSlaveReplication(cr *redisv1beta1.RedisReplication, masterPods []string, slavePods []string) error {
	logger := generateRedisManagerLogger(cr.Namespace, cr.ObjectMeta.Name)

	var realMasterPod string
	realMasterPod = checkAttachedSlave(cr, masterPods)

	if len(slavePods) < 1 {
		realMasterPod = masterPods[0]
		logger.Info("No Master Node Found with attached slave promoting the following pod to master", "pod", masterPods[0])
	}

	logger.Info("Redis Master Node is set to", "pod", realMasterPod)
	realMasterInfo := RedisDetails{
		PodName:   realMasterPod,
		Namespace: cr.Namespace,
	}

	realMasterPodIP := getRedisServerIP(realMasterInfo)

	for i := 0; i < len(masterPods); i++ {
		if masterPods[i] != realMasterPod {

			redisClient := configureRedisReplicationClient(cr, masterPods[i])
			defer redisClient.Close()
			log.Info("Setting the", "pod", masterPods[i], "to slave of", realMasterPod)
			err := redisClient.SlaveOf(realMasterPodIP, "6379").Err()
			if err != nil {
				logger.Error(err, "Failed to set", "pod", masterPods[i], "to slave of", realMasterPod)
				return err
			}
		}
	}

	return nil
}
