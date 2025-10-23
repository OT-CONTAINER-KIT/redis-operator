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

	commonapi "github.com/OT-CONTAINER-KIT/redis-operator/api/common/v1beta2"
	rcvb2 "github.com/OT-CONTAINER-KIT/redis-operator/api/rediscluster/v1beta2"
	rrvb2 "github.com/OT-CONTAINER-KIT/redis-operator/api/redisreplication/v1beta2"
	common "github.com/OT-CONTAINER-KIT/redis-operator/internal/controller/common"
	redis "github.com/redis/go-redis/v9"
	"github.com/samber/lo"
	corev1 "k8s.io/api/core/v1"
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
	return fmt.Sprintf("%s.%s.%s.svc", rd.PodName, common.GetHeadlessServiceNameFromPodName(rd.PodName), rd.Namespace)
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
	ip := getRedisServerIP(ctx, client, rd)
	format := "%s:%d"

	// if ip is IPv6, wrap it in brackets
	if net.ParseIP(ip).To4() == nil {
		format = "[%s]:%d"
	}

	return fmt.Sprintf(format, ip, port)
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
		// if ip is IPv6, wrap it in brackets
		if net.ParseIP(host).To4() == nil {
			host = "[" + host + "]"
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

// CreateSingleLeaderRedisCommand will create command for single leader cluster creation
func CreateSingleLeaderRedisCommand(ctx context.Context, cr *rcvb2.RedisCluster) RedisInvocation {
	cmd := RedisInvocation{
		Command:      []string{"redis-cli"},
		RedisCommand: []string{"CLUSTER", "ADDSLOTS"},
	}
	for i := 0; i < 16384; i++ {
		cmd.RedisCommand = append(cmd.RedisCommand, strconv.Itoa(i))
	}
	log.FromContext(ctx).V(1).Info("Generating Redis Add Slots command for single node cluster",
		"BaseCommand", []string{"redis-cli", "CLUSTER", "ADDSLOTS"},
		"SlotsRange", "0-16383",
		"TotalSlots", 16384)

	return cmd
}

// RepairDisconnectedMasters attempts to repair disconnected/failed masters by issuing
// a CLUSTER MEET with the updated address of the host
func RepairDisconnectedMasters(ctx context.Context, client kubernetes.Interface, cr *rcvb2.RedisCluster) error {
	redisClient := configureRedisClient(ctx, client, cr, cr.Name+"-leader-0")
	defer redisClient.Close()
	return repairDisconnectedMasters(ctx, client, cr, redisClient)
}

func repairDisconnectedMasters(ctx context.Context, client kubernetes.Interface, cr *rcvb2.RedisCluster, redisClient *redis.Client) error {
	nodes, err := clusterNodes(ctx, redisClient)
	if err != nil {
		return err
	}
	masterNodeType := "master"
	var lastError error
	for _, node := range nodes {
		if !nodeIsOfType(node, masterNodeType) {
			continue
		}
		if !nodeFailedOrDisconnected(node) {
			continue
		}
		host, err := getMasterHostFromClusterNode(node)
		if err != nil {
			lastError = err
			log.FromContext(ctx).V(1).Error(err, "Failed to get pod name from cluster node. Continuing with other nodes.", "Node", node)
			continue
		}
		ip := getRedisServerIP(ctx, client, RedisDetails{
			// host may be FQDN like redis-cluster-leader-0.redis-cluster-leader-headless.default.svc.cluster.local
			// or it may be like redis-cluster-leader-0
			// we need to adapt
			PodName:   strings.Split(host, ".")[0],
			Namespace: cr.Namespace,
		})
		err = redisClient.ClusterMeet(ctx, ip, strconv.Itoa(*cr.Spec.Port)).Err()
		if err != nil {
			lastError = err
			log.FromContext(ctx).V(1).Error(err, "Failed to execute CLUSTER MEET on node. Continuing with other nodes.", "Node", node)
			continue
		}
	}
	return lastError
}

func getMasterHostFromClusterNode(node clusterNodesResponse) (string, error) {
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
	var cmd RedisInvocation
	replicas := cr.Spec.GetReplicaCounts("leader")
	switch int(replicas) {
	case 1:
		err := executeFailoverCommand(ctx, client, cr, "leader")
		if err != nil {
			log.FromContext(ctx).Error(err, "error executing failover command")
		}
		cmd = CreateSingleLeaderRedisCommand(ctx, cr)
	default:
		cmd = CreateMultipleLeaderRedisCommand(ctx, client, cr)
	}

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

func getRedisTLSArgs(tlsConfig *commonapi.TLSConfig, clientHost string) []string {
	cmd := []string{}
	if tlsConfig != nil {
		cmd = append(cmd, "--tls")
		cmd = append(cmd, "--cacert")
		cmd = append(cmd, "/tls/ca.crt")
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

// CheckRedisNodeCount will check the count of redis nodes
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
	redisClient := configureRedisClient(ctx, client, cr, cr.Name+"-leader-0")
	defer redisClient.Close()

	cmd := []string{"redis-cli", "--cluster", "check", fmt.Sprintf("127.0.0.1:%d", *cr.Spec.Port)}
	if cr.Spec.KubernetesConfig.ExistingPasswordSecret != nil {
		pass, err := getRedisPassword(ctx, client, cr.Namespace, *cr.Spec.KubernetesConfig.ExistingPasswordSecret.Name, *cr.Spec.KubernetesConfig.ExistingPasswordSecret.Key)
		if err != nil {
			log.FromContext(ctx).Error(err, "Error in getting redis password")
		}
		cmd = append(cmd, "-a")
		cmd = append(cmd, pass)
	}
	cmd = append(cmd, getRedisTLSArgs(cr.Spec.TLS, cr.Name+"-leader-0")...)
	out, err := executeCommand1(ctx, client, cr, cmd, cr.Name+"-leader-0")
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
		Addr:     getRedisServerAddress(ctx, client, redisInfo, *cr.Spec.Port),
		Password: pass,
		DB:       0,
	}
	if cr.Spec.TLS != nil {
		opts.TLSConfig = getRedisTLSConfig(ctx, client, cr.Namespace, cr.Spec.TLS.Secret.SecretName, redisInfo.PodName)
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
func getContainerID(ctx context.Context, client kubernetes.Interface, cr *rcvb2.RedisCluster, podName string) (int, *corev1.Pod) {
	pod, err := client.CoreV1().Pods(cr.Namespace).Get(context.TODO(), podName, metav1.GetOptions{})
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
	var addr string
	if cr.Spec.TLS != nil {
		// Use DNS name for TLS connections
		addr = fmt.Sprintf("%s:%d", getRedisReplicationHostname(redisInfo, cr), 6379)
	} else {
		// Use IP address for non-TLS connections
		addr = getRedisServerAddress(ctx, client, redisInfo, 6379)
	}
	opts := &redis.Options{
		Addr:     addr,
		Password: pass,
		DB:       0,
	}
	if cr.Spec.TLS != nil {
		opts.TLSConfig = getRedisTLSConfig(ctx, client, cr.Namespace, cr.Spec.TLS.Secret.SecretName, podName)
	}
	return redis.NewClient(opts)
}

func getRedisReplicationHostname(redisInfo RedisDetails, cr *rrvb2.RedisReplication) string {
	return fmt.Sprintf("%s.%s-headless.%s.svc.cluster.local", redisInfo.PodName, cr.Name, cr.Namespace)
}

// Get Redis nodes by it's role i.e. master, slave and sentinel
func GetRedisNodesByRole(ctx context.Context, cl kubernetes.Interface, cr *rrvb2.RedisReplication, redisRole string) ([]string, error) {
	statefulset, err := GetStatefulSet(ctx, cl, cr.GetNamespace(), cr.GetName())
	if err != nil {
		log.FromContext(ctx).Error(err, "Failed to Get the Statefulset of the", "custom resource", cr.Name, "in namespace", cr.Namespace)
		return nil, err
	}

	var pods []string
	replicas := cr.Spec.GetReplicationCounts("replication")

	for i := 0; i < int(replicas); i++ {
		podName := statefulset.Name + "-" + strconv.Itoa(i)
		redisClient := configureRedisReplicationClient(ctx, cl, cr, podName)
		defer redisClient.Close()
		podRole, err := checkRedisServerRole(ctx, redisClient, podName)
		if err != nil {
			return nil, err
		}
		if podRole == redisRole {
			pods = append(pods, podName)
		}
	}

	return pods, nil
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
		defer redisClient.Close()

		// Check if Redis instance is accessible
		pong, err := redisClient.Ping(ctx).Result()
		if err != nil {
			log.FromContext(ctx).Error(err, "Failed to ping Redis instance", "pod", podName)
			continue
		}
		if pong != "PONG" {
			log.FromContext(ctx).V(1).Info("Redis instance not ready", "pod", podName)
			continue
		}

		// Apply dynamic configuration parameters
		for _, config := range dynamicConfig {
			parts := strings.SplitN(config, " ", 2)
			if len(parts) != 2 {
				log.FromContext(ctx).Error(nil, "Invalid config format", "config", config)
				continue
			}

			err := redisClient.ConfigSet(ctx, parts[0], parts[1]).Err()
			if err != nil {
				log.FromContext(ctx).Error(err, "Failed to set config",
					"key", parts[0],
					"value", parts[1],
					"pod", podName)
				return err
			}

			log.FromContext(ctx).V(1).Info("Successfully set config",
				"key", parts[0],
				"value", parts[1],
				"pod", podName)
		}
	}

	return nil
}
