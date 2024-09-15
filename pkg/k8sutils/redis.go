package k8sutils

import (
	"bytes"
	"context"
	"encoding/csv"
	"fmt"
	"net"
	"strconv"
	"strings"

	redisv1beta2 "github.com/OT-CONTAINER-KIT/redis-operator/api/v1beta2"
	"github.com/go-logr/logr"
	redis "github.com/redis/go-redis/v9"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/tools/remotecommand"
)

// RedisDetails will hold the information for Redis Pod
type RedisDetails struct {
	PodName   string
	Namespace string
}

// getRedisServerIP will return the IP of redis service
func getRedisServerIP(client kubernetes.Interface, logger logr.Logger, redisInfo RedisDetails) string {
	logger.V(1).Info("Fetching Redis pod", "namespace", redisInfo.Namespace, "podName", redisInfo.PodName)

	redisPod, err := client.CoreV1().Pods(redisInfo.Namespace).Get(context.TODO(), redisInfo.PodName, metav1.GetOptions{})
	if err != nil {
		logger.Error(err, "Error in getting Redis pod IP", "namespace", redisInfo.Namespace, "podName", redisInfo.PodName)
		return ""
	}

	redisIP := redisPod.Status.PodIP
	logger.V(1).Info("Fetched Redis pod IP", "ip", redisIP)

	// Check if IP is empty
	if redisIP == "" {
		logger.V(1).Info("Redis pod IP is empty", "namespace", redisInfo.Namespace, "podName", redisInfo.PodName)
		return ""
	}

	// If we're NOT IPv4, assume we're IPv6..
	if net.ParseIP(redisIP).To4() == nil {
		logger.V(1).Info("Redis is using IPv6", "ip", redisIP)
	}

	logger.V(1).Info("Successfully got the IP for Redis", "ip", redisIP)
	return redisIP
}

func getRedisServerAddress(client kubernetes.Interface, logger logr.Logger, rd RedisDetails, port int) string {
	ip := getRedisServerIP(client, logger, rd)
	format := "%s:%d"

	// if ip is IPv6, wrap it in brackets
	if net.ParseIP(ip).To4() == nil {
		format = "[%s]:%d"
	}

	return fmt.Sprintf(format, ip, port)
}

// getRedisHostname will return the complete FQDN for redis
func getRedisHostname(redisInfo RedisDetails, cr *redisv1beta2.RedisCluster, role string) string {
	fqdn := fmt.Sprintf("%s.%s-%s-headless.%s.svc", redisInfo.PodName, cr.ObjectMeta.Name, role, cr.Namespace)
	return fqdn
}

// CreateSingleLeaderRedisCommand will create command for single leader cluster creation
func CreateSingleLeaderRedisCommand(logger logr.Logger, cr *redisv1beta2.RedisCluster) []string {
	cmd := []string{"redis-cli", "CLUSTER", "ADDSLOTS"}
	for i := 0; i < 16384; i++ {
		cmd = append(cmd, strconv.Itoa(i))
	}
	logger.V(1).Info("Generating Redis Add Slots command for single node cluster",
		"BaseCommand", cmd[:3],
		"SlotsRange", "0-16383",
		"TotalSlots", 16384)

	return cmd
}

// CreateMultipleLeaderRedisCommand will create command for single leader cluster creation
func CreateMultipleLeaderRedisCommand(client kubernetes.Interface, logger logr.Logger, cr *redisv1beta2.RedisCluster) []string {
	cmd := []string{"redis-cli", "--cluster", "create"}
	replicas := cr.Spec.GetReplicaCounts("leader")

	for podCount := 0; podCount < int(replicas); podCount++ {
		podName := cr.ObjectMeta.Name + "-leader-" + strconv.Itoa(podCount)
		var address string
		if cr.Spec.ClusterVersion != nil && *cr.Spec.ClusterVersion == "v7" {
			address = getRedisHostname(RedisDetails{PodName: podName, Namespace: cr.Namespace}, cr, "leader") + fmt.Sprintf(":%d", *cr.Spec.Port)
		} else {
			address = getRedisServerAddress(client, logger, RedisDetails{PodName: podName, Namespace: cr.Namespace}, *cr.Spec.Port)
		}
		cmd = append(cmd, address)
	}
	cmd = append(cmd, "--cluster-yes")

	logger.V(1).Info("Redis cluster creation command", "CommandBase", cmd[:3], "Replicas", replicas)
	return cmd
}

// ExecuteRedisClusterCommand will execute redis cluster creation command
func ExecuteRedisClusterCommand(ctx context.Context, client kubernetes.Interface, logger logr.Logger, cr *redisv1beta2.RedisCluster) {
	var cmd []string
	replicas := cr.Spec.GetReplicaCounts("leader")
	switch int(replicas) {
	case 1:
		err := executeFailoverCommand(ctx, client, logger, cr, "leader")
		if err != nil {
			logger.Error(err, "error executing failover command")
		}
		cmd = CreateSingleLeaderRedisCommand(logger, cr)
	default:
		cmd = CreateMultipleLeaderRedisCommand(client, logger, cr)
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
	logger.V(1).Info("Redis cluster creation command is", "Command", cmd)
	executeCommand(client, logger, cr, cmd, cr.ObjectMeta.Name+"-leader-0")
}

func getRedisTLSArgs(tlsConfig *redisv1beta2.TLSConfig, clientHost string) []string {
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
func createRedisReplicationCommand(client kubernetes.Interface, logger logr.Logger, cr *redisv1beta2.RedisCluster, leaderPod RedisDetails, followerPod RedisDetails) []string {
	cmd := []string{"redis-cli", "--cluster", "add-node"}
	var followerAddress, leaderAddress string

	if cr.Spec.ClusterVersion != nil && *cr.Spec.ClusterVersion == "v7" {
		followerAddress = getRedisHostname(followerPod, cr, "follower") + fmt.Sprintf(":%d", *cr.Spec.Port)
		leaderAddress = getRedisHostname(leaderPod, cr, "leader") + fmt.Sprintf(":%d", *cr.Spec.Port)
	} else {
		followerAddress = getRedisServerAddress(client, logger, followerPod, *cr.Spec.Port)
		leaderAddress = getRedisServerAddress(client, logger, leaderPod, *cr.Spec.Port)
	}

	cmd = append(cmd, followerAddress, leaderAddress, "--cluster-slave")

	if cr.Spec.KubernetesConfig.ExistingPasswordSecret != nil {
		pass, err := getRedisPassword(client, logger, cr.Namespace, *cr.Spec.KubernetesConfig.ExistingPasswordSecret.Name, *cr.Spec.KubernetesConfig.ExistingPasswordSecret.Key)
		if err != nil {
			logger.Error(err, "Failed to retrieve Redis password", "Secret", *cr.Spec.KubernetesConfig.ExistingPasswordSecret.Name)
		} else {
			cmd = append(cmd, "-a", pass)
		}
	}

	cmd = append(cmd, getRedisTLSArgs(cr.Spec.TLS, leaderPod.PodName)...)

	logger.V(1).Info("Generated Redis replication command",
		"FollowerAddress", followerAddress, "LeaderAddress", leaderAddress,
		"Command", cmd)

	return cmd
}

// ExecuteRedisReplicationCommand will execute the replication command
func ExecuteRedisReplicationCommand(ctx context.Context, client kubernetes.Interface, logger logr.Logger, cr *redisv1beta2.RedisCluster) {
	var podIP string
	followerCounts := cr.Spec.GetReplicaCounts("follower")
	leaderCounts := cr.Spec.GetReplicaCounts("leader")
	followerPerLeader := followerCounts / leaderCounts

	redisClient := configureRedisClient(client, logger, cr, cr.ObjectMeta.Name+"-leader-0")
	defer redisClient.Close()

	nodes := checkRedisCluster(ctx, redisClient, logger)
	for followerIdx := 0; followerIdx <= int(followerCounts)-1; {
		for i := 0; i < int(followerPerLeader) && followerIdx <= int(followerCounts)-1; i++ {
			followerPod := RedisDetails{
				PodName:   cr.ObjectMeta.Name + "-follower-" + strconv.Itoa(followerIdx),
				Namespace: cr.Namespace,
			}
			leaderPod := RedisDetails{
				PodName:   cr.ObjectMeta.Name + "-leader-" + strconv.Itoa((followerIdx)%int(leaderCounts)),
				Namespace: cr.Namespace,
			}
			podIP = getRedisServerIP(client, logger, followerPod)
			if !checkRedisNodePresence(cr, nodes, podIP) {
				logger.V(1).Info("Adding node to cluster.", "Node.IP", podIP, "Follower.Pod", followerPod)
				cmd := createRedisReplicationCommand(client, logger, cr, leaderPod, followerPod)
				redisClient := configureRedisClient(client, logger, cr, followerPod.PodName)
				pong, err := redisClient.Ping(ctx).Result()
				redisClient.Close()
				if err != nil {
					logger.Error(err, "Failed to ping Redis server", "Follower.Pod", followerPod)
					continue
				}
				if pong == "PONG" {
					executeCommand(client, logger, cr, cmd, cr.ObjectMeta.Name+"-leader-0")
				} else {
					logger.V(1).Info("Skipping execution of command due to failed Redis ping", "Follower.Pod", followerPod)
				}
			} else {
				logger.V(1).Info("Skipping Adding node to cluster, already present.", "Follower.Pod", followerPod)
			}

			followerIdx++
		}
	}
}

// checkRedisCluster will check the redis cluster have sufficient nodes or not
func checkRedisCluster(ctx context.Context, redisClient *redis.Client, logger logr.Logger) [][]string {
	output, err := redisClient.ClusterNodes(ctx).Result()
	if err != nil {
		logger.Error(err, "Error in getting Redis cluster nodes")
	}
	logger.V(1).Info("Redis cluster nodes are listed", "Output", output)

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
func ExecuteFailoverOperation(ctx context.Context, client kubernetes.Interface, logger logr.Logger, cr *redisv1beta2.RedisCluster) error {
	err := executeFailoverCommand(ctx, client, logger, cr, "leader")
	if err != nil {
		logger.Error(err, "Redis command failed for leader nodes")
		return err
	}
	err = executeFailoverCommand(ctx, client, logger, cr, "follower")
	if err != nil {
		logger.Error(err, "Redis command failed for follower nodes")
		return err
	}
	return nil
}

// executeFailoverCommand will execute failover command
func executeFailoverCommand(ctx context.Context, client kubernetes.Interface, logger logr.Logger, cr *redisv1beta2.RedisCluster, role string) error {
	replicas := cr.Spec.GetReplicaCounts(role)
	podName := fmt.Sprintf("%s-%s-", cr.ObjectMeta.Name, role)
	for podCount := 0; podCount <= int(replicas)-1; podCount++ {
		logger.V(1).Info("Executing redis failover operations", "Redis Node", podName+strconv.Itoa(podCount))
		client := configureRedisClient(client, logger, cr, podName+strconv.Itoa(podCount))
		defer client.Close()
		cmd := redis.NewStringCmd(ctx, "cluster", "reset")
		err := client.Process(ctx, cmd)
		if err != nil {
			logger.Error(err, "Redis command failed with this error")
			flushcommand := redis.NewStringCmd(ctx, "flushall")
			err = client.Process(ctx, flushcommand)
			if err != nil {
				logger.Error(err, "Redis flush command failed with this error")
				return err
			}
		}
		err = client.Process(ctx, cmd)
		if err != nil {
			logger.Error(err, "Redis command failed with this error")
			return err
		}
		output, err := cmd.Result()
		if err != nil {
			logger.Error(err, "Redis command failed with this error")
			return err
		}
		logger.V(1).Info("Redis cluster failover executed", "Output", output)
	}
	return nil
}

// CheckRedisNodeCount will check the count of redis nodes
func CheckRedisNodeCount(ctx context.Context, client kubernetes.Interface, logger logr.Logger, cr *redisv1beta2.RedisCluster, nodeType string) int32 {
	redisClient := configureRedisClient(client, logger, cr, cr.ObjectMeta.Name+"-leader-0")
	defer redisClient.Close()
	var redisNodeType string
	clusterNodes := checkRedisCluster(ctx, redisClient, logger)
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
		logger.V(1).Info("Number of redis nodes are", "Nodes", strconv.Itoa(count), "Type", nodeType)
	} else {
		logger.V(1).Info("Total number of redis nodes are", "Nodes", strconv.Itoa(count))
	}
	return int32(count)
}

// RedisClusterStatusHealth use `redis-cli --cluster check 127.0.0.1:6379`
func RedisClusterStatusHealth(ctx context.Context, client kubernetes.Interface, logger logr.Logger, cr *redisv1beta2.RedisCluster) bool {
	redisClient := configureRedisClient(client, logger, cr, cr.ObjectMeta.Name+"-leader-0")
	defer redisClient.Close()

	cmd := []string{"redis-cli", "--cluster", "check", "127.0.0.1:6379"}
	if cr.Spec.KubernetesConfig.ExistingPasswordSecret != nil {
		pass, err := getRedisPassword(client, logger, cr.Namespace, *cr.Spec.KubernetesConfig.ExistingPasswordSecret.Name, *cr.Spec.KubernetesConfig.ExistingPasswordSecret.Key)
		if err != nil {
			logger.Error(err, "Error in getting redis password")
		}
		cmd = append(cmd, "-a")
		cmd = append(cmd, pass)
	}
	cmd = append(cmd, getRedisTLSArgs(cr.Spec.TLS, cr.ObjectMeta.Name+"-leader-0")...)
	out, err := executeCommand1(client, logger, cr, cmd, cr.ObjectMeta.Name+"-leader-0")
	if err != nil {
		return false
	}
	// [OK] xxx keys in xxx masters.
	// [OK] All nodes agree about slots configuration.
	// [OK] All 16384 slots covered.
	if strings.Count(out, "[OK]") != 3 {
		return false
	}
	return true
}

// CheckRedisClusterState will check the redis cluster state
func CheckRedisClusterState(ctx context.Context, client kubernetes.Interface, logger logr.Logger, cr *redisv1beta2.RedisCluster) int {
	redisClient := configureRedisClient(client, logger, cr, cr.ObjectMeta.Name+"-leader-0")
	defer redisClient.Close()
	clusterNodes := checkRedisCluster(ctx, redisClient, logger)
	count := 0
	for _, node := range clusterNodes {
		if strings.Contains(node[2], "fail") || strings.Contains(node[7], "disconnected") {
			count++
		}
	}
	logger.V(1).Info("Number of failed nodes in cluster", "Failed Node Count", count)
	return count
}

// configureRedisClient will configure the Redis Client
func configureRedisClient(client kubernetes.Interface, logger logr.Logger, cr *redisv1beta2.RedisCluster, podName string) *redis.Client {
	redisInfo := RedisDetails{
		PodName:   podName,
		Namespace: cr.Namespace,
	}
	var err error
	var pass string
	if cr.Spec.KubernetesConfig.ExistingPasswordSecret != nil {
		pass, err = getRedisPassword(client, logger, cr.Namespace, *cr.Spec.KubernetesConfig.ExistingPasswordSecret.Name, *cr.Spec.KubernetesConfig.ExistingPasswordSecret.Key)
		if err != nil {
			logger.Error(err, "Error in getting redis password")
		}
	}
	opts := &redis.Options{
		Addr:     getRedisServerAddress(client, logger, redisInfo, *cr.Spec.Port),
		Password: pass,
		DB:       0,
	}
	if cr.Spec.TLS != nil {
		opts.TLSConfig = getRedisTLSConfig(client, logger, cr.Namespace, cr.Spec.TLS.Secret.SecretName, redisInfo.PodName)
	}
	return redis.NewClient(opts)
}

// executeCommand will execute the commands in pod
func executeCommand(client kubernetes.Interface, logger logr.Logger, cr *redisv1beta2.RedisCluster, cmd []string, podName string) {
	execOut, execErr := executeCommand1(client, logger, cr, cmd, podName)
	if execErr != nil {
		logger.Error(execErr, "Could not execute command", "Command", cmd, "Output", execOut)
		return
	}
	logger.V(1).Info("Successfully executed the command", "Command", cmd, "Output", execOut)
}

func executeCommand1(client kubernetes.Interface, logger logr.Logger, cr *redisv1beta2.RedisCluster, cmd []string, podName string) (stdout string, stderr error) {
	var (
		execOut bytes.Buffer
		execErr bytes.Buffer
	)
	config, err := GenerateK8sConfig()()
	if err != nil {
		logger.Error(err, "Could not find pod to execute")
		return "", err
	}
	targetContainer, pod := getContainerID(client, logger, cr, podName)
	if targetContainer < 0 {
		logger.Error(err, "Could not find pod to execute")
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
		logger.Error(err, "Failed to init executor")
		return "", err
	}

	err = exec.StreamWithContext(context.Background(), remotecommand.StreamOptions{
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
func getContainerID(client kubernetes.Interface, logger logr.Logger, cr *redisv1beta2.RedisCluster, podName string) (int, *corev1.Pod) {
	pod, err := client.CoreV1().Pods(cr.Namespace).Get(context.TODO(), podName, metav1.GetOptions{})
	if err != nil {
		logger.Error(err, "Could not get pod info", "Pod Name", podName, "Namespace", cr.Namespace)
		return -1, nil
	}

	logger.V(1).Info("Pod info retrieved successfully", "Pod Name", podName, "Namespace", cr.Namespace)

	targetContainer := -1
	for containerID, tr := range pod.Spec.Containers {
		logger.V(1).Info("Inspecting container", "Pod Name", podName, "Container ID", containerID, "Container Name", tr.Name)
		if tr.Name == cr.ObjectMeta.Name+"-leader" {
			targetContainer = containerID
			logger.V(1).Info("Leader container found", "Container ID", containerID, "Container Name", tr.Name)
			break
		}
	}

	if targetContainer == -1 {
		logger.V(1).Info("Leader container not found in pod", "Pod Name", podName)
		return -1, nil
	}

	return targetContainer, pod
}

// checkRedisNodePresence will check if the redis node exist in cluster or not
func checkRedisNodePresence(cr *redisv1beta2.RedisCluster, nodeList [][]string, nodeName string) bool {
	logger := generateRedisManagerLogger(cr.Namespace, cr.ObjectMeta.Name)
	logger.V(1).Info("Checking if Node is in cluster", "Node", nodeName)
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
func configureRedisReplicationClient(client kubernetes.Interface, logger logr.Logger, cr *redisv1beta2.RedisReplication, podName string) *redis.Client {
	redisInfo := RedisDetails{
		PodName:   podName,
		Namespace: cr.Namespace,
	}
	var err error
	var pass string
	if cr.Spec.KubernetesConfig.ExistingPasswordSecret != nil {
		pass, err = getRedisPassword(client, logger, cr.Namespace, *cr.Spec.KubernetesConfig.ExistingPasswordSecret.Name, *cr.Spec.KubernetesConfig.ExistingPasswordSecret.Key)
		if err != nil {
			logger.Error(err, "Error in getting redis password")
		}
	}
	opts := &redis.Options{
		Addr:     getRedisServerAddress(client, logger, redisInfo, 6379),
		Password: pass,
		DB:       0,
	}
	if cr.Spec.TLS != nil {
		opts.TLSConfig = getRedisTLSConfig(client, logger, cr.Namespace, cr.Spec.TLS.Secret.SecretName, podName)
	}
	return redis.NewClient(opts)
}

// Get Redis nodes by it's role i.e. master, slave and sentinel
func GetRedisNodesByRole(ctx context.Context, cl kubernetes.Interface, logger logr.Logger, cr *redisv1beta2.RedisReplication, redisRole string) []string {
	statefulset, err := GetStatefulSet(cl, logger, cr.GetNamespace(), cr.GetName())
	if err != nil {
		logger.Error(err, "Failed to Get the Statefulset of the", "custom resource", cr.Name, "in namespace", cr.Namespace)
	}

	var pods []string
	replicas := cr.Spec.GetReplicationCounts("replication")

	for i := 0; i < int(replicas); i++ {
		podName := statefulset.Name + "-" + strconv.Itoa(i)
		redisClient := configureRedisReplicationClient(cl, logger, cr, podName)
		defer redisClient.Close()
		podRole := checkRedisServerRole(ctx, redisClient, logger, podName)
		if podRole == redisRole {
			pods = append(pods, podName)
		}
	}

	return pods
}

// Check the Redis Server Role i.e. master, slave and sentinel
func checkRedisServerRole(ctx context.Context, redisClient *redis.Client, logger logr.Logger, podName string) string {
	info, err := redisClient.Info(ctx, "Replication").Result()
	if err != nil {
		logger.Error(err, "Failed to Get the role Info of the", "redis pod", podName)
		return ""
	}
	lines := strings.Split(info, "\r\n")
	for _, line := range lines {
		if strings.HasPrefix(line, "role:") {
			role := strings.TrimPrefix(line, "role:")
			logger.V(1).Info("Role of the Redis Pod", "pod", podName, "role", role)
			return role
		}
	}
	logger.Error(err, "Failed to find role from Info # Replication in", "redis pod", podName)
	return ""
}

// checkAttachedSlave would return redis pod name which has slave
func checkAttachedSlave(ctx context.Context, redisClient *redis.Client, logger logr.Logger, podName string) int {
	info, err := redisClient.Info(ctx, "Replication").Result()
	if err != nil {
		logger.Error(err, "Failed to get the connected slaves count of the", "redis pod", podName)
		return -1 // return -1 if failed to get the connected slaves count
	}

	lines := strings.Split(info, "\r\n")
	for _, line := range lines {
		if strings.HasPrefix(line, "connected_slaves:") {
			var connected_slaves int
			connected_slaves, err = strconv.Atoi(strings.TrimPrefix(line, "connected_slaves:"))
			if err != nil {
				logger.Error(err, "Failed to convert the connected slaves count of the", "redis pod", podName)
				return -1
			}
			logger.V(1).Info("Connected Slaves of the Redis Pod", "pod", podName, "connected_slaves", connected_slaves)
			return connected_slaves
		}
	}

	logger.Error(nil, "Failed to find connected_slaves from Info # Replication in", "redis pod", podName)
	return 0
}

func CreateMasterSlaveReplication(ctx context.Context, client kubernetes.Interface, logger logr.Logger, cr *redisv1beta2.RedisReplication, masterPods []string, realMasterPod string) error {
	logger.V(1).Info("Redis Master Node is set to", "pod", realMasterPod)
	realMasterInfo := RedisDetails{
		PodName:   realMasterPod,
		Namespace: cr.Namespace,
	}

	realMasterPodIP := getRedisServerIP(client, logger, realMasterInfo)

	for i := 0; i < len(masterPods); i++ {
		if masterPods[i] != realMasterPod {
			redisClient := configureRedisReplicationClient(client, logger, cr, masterPods[i])
			defer redisClient.Close()
			logger.V(1).Info("Setting the", "pod", masterPods[i], "to slave of", realMasterPod)
			err := redisClient.SlaveOf(ctx, realMasterPodIP, "6379").Err()
			if err != nil {
				logger.Error(err, "Failed to set", "pod", masterPods[i], "to slave of", realMasterPod)
				return err
			}
		}
	}

	return nil
}

func GetRedisReplicationRealMaster(ctx context.Context, client kubernetes.Interface, logger logr.Logger, cr *redisv1beta2.RedisReplication, masterPods []string) string {
	for _, podName := range masterPods {
		redisClient := configureRedisReplicationClient(client, logger, cr, podName)
		defer redisClient.Close()

		if checkAttachedSlave(ctx, redisClient, logger, podName) > 0 {
			return podName
		}
	}
	return ""
}
