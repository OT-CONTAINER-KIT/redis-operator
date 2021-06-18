package k8sutils

import (
	"bytes"
	"context"
	"encoding/csv"
	"strconv"
	"strings"

	redisv1beta1 "redis-operator/api/v1beta1"

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
	redisIP, err := generateK8sClient().CoreV1().Pods(redisInfo.Namespace).Get(context.TODO(), redisInfo.PodName, metav1.GetOptions{})
	if err != nil {
		logger.Error(err, "Error in getting redis pod IP")
	}

	logger.Info("Successfully got the ip for redis", "ip", redisIP.Status.PodIP)
	return redisIP.Status.PodIP
}

// ExecuteRedisClusterCommand will execute redis cluster creation command
func ExecuteRedisClusterCommand(cr *redisv1beta1.RedisCluster) {
	logger := generateRedisManagerLogger(cr.Namespace, cr.ObjectMeta.Name)
	replicas := cr.Spec.Size
	cmd := []string{"redis-cli", "--cluster", "create"}
	for podCount := 0; podCount <= int(*replicas)-1; podCount++ {
		pod := RedisDetails{
			PodName:   cr.ObjectMeta.Name + "-leader-" + strconv.Itoa(podCount),
			Namespace: cr.Namespace,
		}
		cmd = append(cmd, getRedisServerIP(pod)+":6379")
	}
	cmd = append(cmd, "--cluster-yes")

	if cr.Spec.KubernetesConfig.ExistingPasswordSecret != nil {
		pass, err := getRedisPassword(cr.Namespace, *cr.Spec.KubernetesConfig.ExistingPasswordSecret.Name, *cr.Spec.KubernetesConfig.ExistingPasswordSecret.Key)
		if err != nil {
			logger.Error(err, "Error in getting redis password")
		}
		cmd = append(cmd, "-a")
		cmd = append(cmd, pass)
	}
	logger.Info("Redis cluster creation command is", "Command", cmd)
	executeCommand(cr, cmd, cr.ObjectMeta.Name+"-leader-0")
}

// createRedisReplicationCommand will create redis replication creation command
func createRedisReplicationCommand(cr *redisv1beta1.RedisCluster, leaderPod RedisDetails, followerPod RedisDetails) []string {
	logger := generateRedisManagerLogger(cr.Namespace, cr.ObjectMeta.Name)
	cmd := []string{"redis-cli", "--cluster", "add-node"}
	cmd = append(cmd, getRedisServerIP(followerPod)+":6379")
	cmd = append(cmd, getRedisServerIP(leaderPod)+":6379")
	cmd = append(cmd, "--cluster-slave")

	if cr.Spec.KubernetesConfig.ExistingPasswordSecret != nil {
		pass, err := getRedisPassword(cr.Namespace, *cr.Spec.KubernetesConfig.ExistingPasswordSecret.Name, *cr.Spec.KubernetesConfig.ExistingPasswordSecret.Key)
		if err != nil {
			logger.Error(err, "Error in getting redis password")
		}
		cmd = append(cmd, "-a")
		cmd = append(cmd, pass)
	}
	logger.Info("Redis replication creation command is", "Command", cmd)
	return cmd
}

// ExecuteRedisReplicationCommand will execute the replication command
func ExecuteRedisReplicationCommand(cr *redisv1beta1.RedisCluster) {
	logger := generateRedisManagerLogger(cr.Namespace, cr.ObjectMeta.Name)
	replicas := cr.Spec.Size
	nodes := checkRedisCluster(cr)
	for podCount := 0; podCount <= int(*replicas)-1; podCount++ {
		followerPod := RedisDetails{
			PodName:   cr.ObjectMeta.Name + "-follower-" + strconv.Itoa(podCount),
			Namespace: cr.Namespace,
		}
		leaderPod := RedisDetails{
			PodName:   cr.ObjectMeta.Name + "-leader-" + strconv.Itoa(podCount),
			Namespace: cr.Namespace,
		}
		podIP := getRedisServerIP(followerPod)
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
func ExecuteFailoverOperation(cr *redisv1beta1.RedisCluster) {
	executeFailoverCommand(cr, "leader")
	executeFailoverCommand(cr, "follower")
}

// executeFailoverCommand will execute failover command
func executeFailoverCommand(cr *redisv1beta1.RedisCluster, role string) {
	logger := generateRedisManagerLogger(cr.Namespace, cr.ObjectMeta.Name)
	replicas := cr.Spec.Size
	podName := cr.ObjectMeta.Name + "-" + role + "-"
	for podCount := 0; podCount <= int(*replicas)-1; podCount++ {
		logger.Info("Executing redis failover operations", "Redis Node", podName+strconv.Itoa(podCount))
		client := configureRedisClient(cr, podName+strconv.Itoa(podCount))
		cmd := redis.NewStringCmd("cluster", "reset")
		err := client.Process(cmd)
		if err != nil {
			logger.Error(err, "Redis command failed with this error")
			flushcommand := redis.NewStringCmd("flushall")
			err := client.Process(flushcommand)
			if err != nil {
				logger.Error(err, "Redis flush command failed with this error")
			}
		}

		output, err := cmd.Result()
		if err != nil {
			logger.Error(err, "Redis command failed with this error")
		}
		logger.Info("Redis cluster failover executed", "Output", output)
	}
}

// CheckRedisNodeCount will check the count of redis nodes
func CheckRedisNodeCount(cr *redisv1beta1.RedisCluster, nodeType string) int {
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
	return count
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
			Addr:     getRedisServerIP(redisInfo) + ":6379",
			Password: pass,
			DB:       0,
		})
	} else {
		client = redis.NewClient(&redis.Options{
			Addr:     getRedisServerIP(redisInfo) + ":6379",
			Password: "",
			DB:       0,
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
	}
	targetContainer, pod := getContainerID(cr, podName)
	if targetContainer < 0 {
		logger.Error(err, "Could not find pod to execute")
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
	}

	err = exec.Stream(remotecommand.StreamOptions{
		Stdout: &execOut,
		Stderr: &execErr,
		Tty:    false,
	})
	if err != nil {
		logger.Error(err, "Could not execute command", "Command", cmd, "Output", execOut.String(), "Error", execErr.String())
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
		if strings.Contains(node[1], nodeName) {
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
