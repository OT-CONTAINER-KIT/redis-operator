package k8sutils

import (
	"context"
	"strconv"

	rrvb2 "github.com/OT-CONTAINER-KIT/redis-operator/api/redisreplication/v1beta2"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

const (
	redisRoleMaster = "master"
	redisRoleSlave  = "slave"
)

type RedisReplicationTopology struct {
	Masters []string
	Slaves  []string
	// Unobserved holds pods whose role could not be determined (missing, not ready, or probe failed).
	// Masters and Slaves may be incomplete when this is non-empty.
	Unobserved []string
}

func (t RedisReplicationTopology) Complete() bool {
	return len(t.Unobserved) == 0
}

func (t RedisReplicationTopology) Observed() int {
	return len(t.Masters) + len(t.Slaves)
}

// GetRedisReplicationTopology probes every replication pod for its role.
// Pods that cannot be probed are reported in Unobserved instead of failing
// the whole sweep, so a single crashed pod does not block reconciliation.
func GetRedisReplicationTopology(ctx context.Context, cl kubernetes.Interface, cr *rrvb2.RedisReplication) (RedisReplicationTopology, error) {
	return getRedisReplicationTopology(ctx, cl, cr, func(ctx context.Context, pod *corev1.Pod) (string, error) {
		redisClient := configureRedisReplicationClientForPod(ctx, cl, cr, pod)
		defer redisClient.Close()

		return checkRedisServerRole(ctx, redisClient, pod.Name)
	})
}

// GetRedisNodesByRole returns the pods with the given role ("master" or "slave").
// Unobserved pods are excluded; use GetRedisReplicationTopology if you need to
// know whether the view is complete.
func GetRedisNodesByRole(ctx context.Context, cl kubernetes.Interface, cr *rrvb2.RedisReplication, redisRole string) ([]string, error) {
	topology, err := GetRedisReplicationTopology(ctx, cl, cr)
	if err != nil {
		return nil, err
	}
	return topology.byRole(redisRole), nil
}

func (t RedisReplicationTopology) byRole(redisRole string) []string {
	switch redisRole {
	case redisRoleMaster:
		return t.Masters
	case redisRoleSlave:
		return t.Slaves
	default:
		return nil
	}
}

func getRedisReplicationTopology(ctx context.Context, cl kubernetes.Interface, cr *rrvb2.RedisReplication, probeRole func(context.Context, *corev1.Pod) (string, error)) (RedisReplicationTopology, error) {
	var topology RedisReplicationTopology

	statefulset, err := GetStatefulSet(ctx, cl, cr.GetNamespace(), cr.GetName())
	if err != nil {
		log.FromContext(ctx).Error(err, "Failed to Get the Statefulset of the", "custom resource", cr.Name, "in namespace", cr.Namespace)
		return RedisReplicationTopology{}, err
	}

	replicas := cr.Spec.GetReplicationCounts("replication")

	for i := 0; i < int(replicas); i++ {
		podName := statefulset.Name + "-" + strconv.Itoa(i)
		pod, err := cl.CoreV1().Pods(cr.Namespace).Get(ctx, podName, metav1.GetOptions{})
		if err != nil {
			if apierrors.IsNotFound(err) {
				topology.Unobserved = append(topology.Unobserved, podName)
				continue
			}
			return RedisReplicationTopology{}, err
		}

		if !IsRedisPodProbeable(pod) {
			log.FromContext(ctx).V(1).Info("Redis pod is not Running and Ready, skipping role probe", "pod", podName)
			topology.Unobserved = append(topology.Unobserved, podName)
			continue
		}

		podRole, err := probeRole(ctx, pod)
		if err != nil {
			// A cancelled context is not a pod-specific failure; bail out
			// instead of reporting a partial topology as success.
			if ctxErr := ctx.Err(); ctxErr != nil {
				return RedisReplicationTopology{}, ctxErr
			}
			log.FromContext(ctx).V(1).Info("Failed to probe Redis role, skipping pod", "pod", podName, "error", err)
			topology.Unobserved = append(topology.Unobserved, podName)
			continue
		}
		switch podRole {
		case redisRoleMaster:
			topology.Masters = append(topology.Masters, podName)
		case redisRoleSlave:
			topology.Slaves = append(topology.Slaves, podName)
		default:
			log.FromContext(ctx).Info("Redis pod reported an unknown role, skipping pod", "pod", podName, "role", podRole)
			topology.Unobserved = append(topology.Unobserved, podName)
		}
	}

	return topology, nil
}
