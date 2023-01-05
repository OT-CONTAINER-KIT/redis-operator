package k8sutils

import (
	"bytes"
	"context"
	"strconv"

	redisv1beta1 "redis-operator/api/v1beta1"

	v1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/tools/remotecommand"
)

// SentinelDetails will hold the information for Sentinel Pod

type SentinelDetails struct {
	PodName   string
	Namespace string
}

// Sentinel Mode addition

func ExecuteSenitnelCommand(sentinelCR *redisv1beta1.RedisSentinel, clusterCR *v1.StatefulSet) {

	logger := generateRedisManagerLogger(sentinelCR.Namespace, sentinelCR.ObjectMeta.Name)

	replicas := sentinelCR.Spec.GetSentinelCounts("sentinel")

	sentinelConfig := sentinelCR.Spec.RedisSnt.RedisSentinelConfig

	var listofIP = listofIP(clusterCR)
	var port = sentinelConfig.RedisPort
	var quorum = sentinelConfig.Quorum
	var groupName = sentinelConfig.MasterGroupName

	for _, ip := range listofIP {

		cmd := []string{"redis-cli", "sentinel", "monitor", groupName, ip, strconv.Itoa(int(port)), strconv.Itoa(int(quorum))}
		logger.Info("Sentinel creation command is", "Command", cmd)

		// Check the Password in the Sentinel mode if any later ++

		for i := 0; i < int(replicas); i++ {
			executeSentinelBaseCommand(sentinelCR, cmd, sentinelCR.ObjectMeta.Name+"-sentinel-"+strconv.Itoa(i))
		}

	}

}

// This below code is draft

// func createSentinelCommand(instance *redisv1beta1.RedisCluster) {

// 	// Loading The Masters in the Redis Cluster
// 	//redisLeaderInfo, err := GetStatefulSet(instance.Namespace, instance.ObjectMeta.Name+"-leader")

// 	// Target is to get the  list of IP's out of the master

// 	var listofIP = listofIP(instance)

// 	// Need to change the Port and quorum later
// 	var port = ":6379"
// 	var quorum = "2"

// 	for _, ip := range listofIP {

// 	}

// }

// Pass the Redis Cluster and get the List of IP's of leaders
func listofIP(cr *v1.StatefulSet) []string {

	var iplist []string

	replicas := cr.Spec.Replicas

	for podCount := 0; podCount <= int(*replicas)-1; podCount++ {

		pod := RedisDetails{
			PodName:   cr.ObjectMeta.Name + "-leader-" + strconv.Itoa(podCount),
			Namespace: cr.Namespace,
		}
		iplist = append(iplist, getRedisServerIP(pod))
	}

	return iplist
}

// Execute the base command of sentinel
func executeSentinelBaseCommand(cr *redisv1beta1.RedisSentinel, cmd []string, podName string) {
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
	targetContainer, pod := getSentinelContainerID(cr, podName)
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
func getSentinelContainerID(cr *redisv1beta1.RedisSentinel, podName string) (int, *corev1.Pod) {
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
