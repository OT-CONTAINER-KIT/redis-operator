package k8sutils

import (
	"bufio"
	"bytes"
	"context"
	"github.com/go-redis/redis"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/remotecommand"
	redisv1beta1 "redis-operator/api/v1beta1"
	"regexp"
	"strconv"
	"strings"
)

var (
	execOut bytes.Buffer
	execErr bytes.Buffer
)

// RedisDetails will hold the information for Redis Pod
type RedisDetails struct {
	PodName   string
	Namespace string
}

// getRedisServerIP will return the IP of redis service
func getRedisServerIP(redisInfo RedisDetails) string {
	reqLogger := log.WithValues("Request.Namespace", redisInfo.Namespace, "Request.PodName", redisInfo.PodName)
	redisIP, _ := GenerateK8sClient().CoreV1().Pods(redisInfo.Namespace).
		Get(context.TODO(), redisInfo.PodName, metav1.GetOptions{})

	reqLogger.Info("Successfully got the ip for redis", "ip", redisIP.Status.PodIP)
	return redisIP.Status.PodIP
}

// ExecuteRedisClusterCommand will execute redis cluster creation command
func ExecuteRedisClusterCommand(cr *redisv1beta1.Redis) {
	reqLogger := log.WithValues("Request.Namespace", cr.Namespace, "Request.Name", cr.ObjectMeta.Name)
	replicas := cr.Spec.Size
	cmd := []string{"redis-cli", "--cluster", "create"}
	for podCount := 0; podCount <= int(*replicas)-1; podCount++ {
		pod := RedisDetails{
			PodName:   cr.ObjectMeta.Name + "-master-" + strconv.Itoa(podCount),
			Namespace: cr.Namespace,
		}
		cmd = append(cmd, getRedisServerIP(pod)+":6379")
	}
	cmd = append(cmd, "--cluster-yes")
	if cr.Spec.GlobalConfig.Password != nil && cr.Spec.GlobalConfig.ExistingPasswordSecret == nil {
		cmd = append(cmd, "-a")
		cmd = append(cmd, *cr.Spec.GlobalConfig.Password)
	}

	if cr.Spec.GlobalConfig.ExistingPasswordSecret != nil {
		pass := getRedisPassword(cr)
		cmd = append(cmd, "-a")
		cmd = append(cmd, pass)
	}
	reqLogger.Info("Redis cluster creation command is", "Command", cmd)
	executeCommand(cr, cmd, cr.ObjectMeta.Name+"-master-0")
}

// createRedisReplicationCommand will create redis replication creation command
func createRedisReplicationCommand(cr *redisv1beta1.Redis, nodeNumber string) []string {
	reqLogger := log.WithValues("Request.Namespace", cr.Namespace, "Request.Name", cr.ObjectMeta.Name)
	cmd := []string{"redis-cli", "--cluster", "add-node"}
	masterPod := RedisDetails{
		PodName:   cr.ObjectMeta.Name + "-master-" + nodeNumber,
		Namespace: cr.Namespace,
	}
	slavePod := RedisDetails{
		PodName:   cr.ObjectMeta.Name + "-slave-" + nodeNumber,
		Namespace: cr.Namespace,
	}
	cmd = append(cmd, getRedisServerIP(slavePod)+":6379")
	cmd = append(cmd, getRedisServerIP(masterPod)+":6379")
	cmd = append(cmd, "--cluster-slave")

	if cr.Spec.GlobalConfig.Password != nil && cr.Spec.GlobalConfig.ExistingPasswordSecret == nil {
		cmd = append(cmd, "-a")
		cmd = append(cmd, *cr.Spec.GlobalConfig.Password)
	}
	if cr.Spec.GlobalConfig.ExistingPasswordSecret != nil {
		pass := getRedisPassword(cr)
		cmd = append(cmd, "-a")
		cmd = append(cmd, pass)
	}
	reqLogger.Info("Redis replication creation command is", "Command", cmd)
	return cmd
}

// ExecuteRedisReplicationCommand will execute the replication command
func ExecuteRedisReplicationCommand(cr *redisv1beta1.Redis) {
	replicas := cr.Spec.Size
	for podCount := 0; podCount <= int(*replicas)-1; podCount++ {
		cmd := createRedisReplicationCommand(cr, strconv.Itoa(podCount))
		executeCommand(cr, cmd, cr.ObjectMeta.Name+"-master-0")
	}
}

// checkRedisCluster will check the redis cluster have sufficient nodes or not
func checkRedisCluster(cr *redisv1beta1.Redis) string {
	var client *redis.Client
	reqLogger := log.WithValues("Request.Namespace", cr.Namespace, "Request.Name", cr.ObjectMeta.Name)

	client = configureRedisClient(cr, cr.ObjectMeta.Name+"-master-0")
	cmd := redis.NewStringCmd("cluster", "nodes")
	err := client.Process(cmd)
	if err != nil {
		reqLogger.Error(err, "Redis command failed with this error")
	}

	output, err := cmd.Result()
	if err != nil {
		reqLogger.Error(err, "Redis command failed with this error")
	}
	reqLogger.Info("Redis cluster nodes are listed", "Output", output)
	return output
}

// ExecuteFaioverOperation will execute redis failover operations
func ExecuteFaioverOperation(cr *redisv1beta1.Redis) {
	executeFailoverCommand(cr, "master")
	executeFailoverCommand(cr, "slave")
}

// executeFailoverCommand will execute failover command
func executeFailoverCommand(cr *redisv1beta1.Redis, role string) {
	reqLogger := log.WithValues("Request.Namespace", cr.Namespace, "Request.Name", cr.ObjectMeta.Name)
	replicas := cr.Spec.Size
	podName := cr.ObjectMeta.Name + "-" + role + "-"
	for podCount := 0; podCount <= int(*replicas)-1; podCount++ {
		reqLogger.Info("Executing redis failover operations", "Redis Node", podName+strconv.Itoa(podCount))
		client := configureRedisClient(cr, podName+strconv.Itoa(podCount))
		cmd := redis.NewStringCmd("cluster", "reset")
		err := client.Process(cmd)
		if err != nil {
			reqLogger.Error(err, "Redis command failed with this error")
			flushcommand := redis.NewStringCmd("flushall")
			err := client.Process(flushcommand)
			if err != nil {
				reqLogger.Error(err, "Redis flush command failed with this error")
			}
		}

		output, err := cmd.Result()
		if err != nil {
			reqLogger.Error(err, "Redis command failed with this error")
		}
		reqLogger.Info("Redis cluster failover executed", "Output", output)
	}
}

// CheckRedisNodeCount will check the count of redis nodes
func CheckRedisNodeCount(cr *redisv1beta1.Redis) int {
	reqLogger := log.WithValues("Request.Namespace", cr.Namespace, "Request.Name", cr.ObjectMeta.Name)
	output := checkRedisCluster(cr)
	scanner := bufio.NewScanner(strings.NewReader(output))

	count := 0
	for scanner.Scan() {
		count++
	}
	reqLogger.Info("Total number of redis nodes are", "Nodes", strconv.Itoa(count))
	return count
}

// CheckRedisClusterState will check the redis cluster state
func CheckRedisClusterState(cr *redisv1beta1.Redis) int {
	reqLogger := log.WithValues("Request.Namespace", cr.Namespace, "Request.Name", cr.ObjectMeta.Name)
	output := checkRedisCluster(cr)
	pattern := regexp.MustCompile("fail")
	match := pattern.FindAllStringIndex(output, -1)
	reqLogger.Info("Number of failed nodes in cluster", "Failed Node Count", len(match))
	return len(match)
}

// configureRedisClient will configure the Redis Client
func configureRedisClient(cr *redisv1beta1.Redis, podName string) *redis.Client {
	redisInfo := RedisDetails{
		PodName:   podName,
		Namespace: cr.Namespace,
	}
	var client *redis.Client

	if cr.Spec.GlobalConfig.Password != nil && cr.Spec.GlobalConfig.ExistingPasswordSecret == nil {
		client = redis.NewClient(&redis.Options{
			Addr:     getRedisServerIP(redisInfo) + ":6379",
			Password: *cr.Spec.GlobalConfig.Password,
			DB:       0,
		})
	} else if cr.Spec.GlobalConfig.ExistingPasswordSecret != nil {
		pass := getRedisPassword(cr)
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
func executeCommand(cr *redisv1beta1.Redis, cmd []string, podName string) {
	reqLogger := log.WithValues("Request.Namespace", cr.Namespace, "Request.Name", cr.ObjectMeta.Name)
	config, err := rest.InClusterConfig()
	if err != nil {
		reqLogger.Error(err, "Error while reading Incluster config")
	}
	targetContainer, pod := getContainerID(cr, podName)
	if targetContainer < 0 {
		reqLogger.Error(err, "Could not find pod to execute")
	}

	req := GenerateK8sClient().CoreV1().RESTClient().Post().Resource("pods").Name(podName).Namespace(cr.Namespace).SubResource("exec")
	req.VersionedParams(&corev1.PodExecOptions{
		Container: pod.Spec.Containers[targetContainer].Name,
		Command:   cmd,
		Stdout:    true,
		Stderr:    true,
	}, scheme.ParameterCodec)
	exec, err := remotecommand.NewSPDYExecutor(config, "POST", req.URL())
	if err != nil {
		reqLogger.Error(err, "Failed to init executor")
	}

	err = exec.Stream(remotecommand.StreamOptions{
		Stdout: &execOut,
		Stderr: &execErr,
		Tty:    false,
	})
	if err != nil {
		reqLogger.Error(err, "Could not execute command")
	}
	reqLogger.Info("Successfully executed the command", "Command", cmd, "Output", execOut.String())
}

// getContainerID will return the id of container from pod
func getContainerID(cr *redisv1beta1.Redis, podName string) (int, *corev1.Pod) {
	reqLogger := log.WithValues("Request.Namespace", cr.Namespace, "Request.Name", cr.ObjectMeta.Name)
	pod, err := GenerateK8sClient().CoreV1().Pods(cr.Namespace).Get(context.TODO(), podName, metav1.GetOptions{})
	if err != nil {
		reqLogger.Error(err, "Could not get pod info")
	}

	targetContainer := -1
	for containerID, tr := range pod.Spec.Containers {
		reqLogger.Info("Pod Counted successfully", "Count", containerID, "Container Name", tr.Name)
		if tr.Name == cr.ObjectMeta.Name+"-master" {
			targetContainer = containerID
			break
		}
	}
	return targetContainer, pod
}
