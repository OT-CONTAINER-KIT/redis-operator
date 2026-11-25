package k8sutils

import (
	"context"
	"fmt"
	"net"

	rcvb2 "github.com/OT-CONTAINER-KIT/redis-operator/api/rediscluster/v1beta2"
	"github.com/redis/go-redis/v9"
	"k8s.io/client-go/kubernetes"
)

// PrepareRedisFollowerScaleDown detaches the followers that the StatefulSet
// will remove. The controller must keep the current replica count on error.
func PrepareRedisFollowerScaleDown(ctx context.Context, client kubernetes.Interface, cr *rcvb2.RedisCluster, currentFollowers int32) error {
	return prepareRedisFollowerScaleDown(ctx, cr, currentFollowers, func(podName string) *redis.Client {
		return configureRedisClient(ctx, client, cr, podName)
	})
}

type followerScaleDownMember struct {
	podName string
	client  *redis.Client
	nodes   []clusterNodesResponse
	self    clusterNodesResponse
}

func (m *followerScaleDownMember) readTopology(ctx context.Context) error {
	host, _, err := net.SplitHostPort(m.client.Options().Addr)
	if err != nil || host == "" {
		return fmt.Errorf("no Redis address for pod %s", m.podName)
	}

	m.self = nil
	m.nodes, err = clusterNodes(ctx, m.client)
	if err != nil {
		return fmt.Errorf("read Redis topology from pod %s: %w", m.podName, err)
	}
	for _, node := range m.nodes {
		if len(node) < 8 || node[0] == "" {
			return fmt.Errorf("invalid Redis topology from pod %s", m.podName)
		}
		if hasFlag(node[2], "myself") {
			m.self = node
		}
	}
	if m.self == nil {
		return fmt.Errorf("topology from Redis pod %s does not identify its own node", m.podName)
	}
	return nil
}

func prepareRedisFollowerScaleDown(ctx context.Context, cr *rcvb2.RedisCluster, currentFollowers int32, makeClient func(string) *redis.Client) error {
	leaders := cr.Spec.GetReplicaCounts("leader")
	followers := cr.Spec.GetReplicaCounts("follower")
	retained := make([]followerScaleDownMember, 0, leaders+followers)
	var retiring []followerScaleDownMember
	for ordinal := range leaders + currentFollowers {
		podName := fmt.Sprintf("%s-leader-%d", cr.Name, ordinal)
		if ordinal >= leaders {
			podName = fmt.Sprintf("%s-follower-%d", cr.Name, ordinal-leaders)
		}
		member := followerScaleDownMember{podName: podName, client: makeClient(podName)}
		defer member.client.Close()
		if err := member.readTopology(ctx); err != nil {
			return err
		}
		if ordinal >= leaders+followers {
			retiring = append(retiring, member)
		} else {
			retained = append(retained, member)
		}
	}

	// Validate every target before attempting to detach any of them. StatefulSet
	// ordinals describe which pods will be deleted, not their current Redis roles.
	for _, member := range retiring {
		replica := hasFlag(member.self[2], "slave")
		detached := len(member.nodes) == 1 && hasFlag(member.self[2], "master")
		if len(member.self) != 8 || (!replica && !detached) {
			return fmt.Errorf("cannot scale down pod %s: Redis node %s is not a slotless replica or a detached node", member.podName, member.self[0])
		}
		primaryRetained := false
		for _, peer := range retained {
			if peer.self[0] == member.self[3] && hasFlag(peer.self[2], "master") {
				primaryRetained = true
			}
			for _, node := range peer.nodes {
				if node[0] == member.self[0] && (!hasFlag(node[2], "slave") || len(node) != 8) {
					return fmt.Errorf("cannot scale down pod %s: Redis pod %s does not see node %s as a slotless replica", member.podName, peer.podName, node[0])
				}
			}
		}
		if replica && !primaryRetained {
			return fmt.Errorf("cannot scale down pod %s: its Redis primary %s is not among the retained primaries", member.podName, member.self[3])
		}
	}

	for _, member := range retiring {
		nodeID := member.self[0]
		// A soft reset detaches the replica and preserves its ID. If forgetting
		// the ID fails partway through, the next reconcile can finish using the
		// same ID, even if the reset or some of the forget commands succeeded.
		if hasFlag(member.self[2], "slave") {
			// Recheck the role and identity immediately before resetting, since
			// this pod could have been promoted or replaced during preflight.
			if err := member.readTopology(ctx); err != nil {
				return err
			}
			if member.self[0] != nodeID || !hasFlag(member.self[2], "slave") || len(member.self) != 8 {
				return fmt.Errorf("cannot scale down pod %s: its Redis role or node ID changed", member.podName)
			}
			if err := member.client.ClusterResetSoft(ctx).Err(); err != nil {
				return fmt.Errorf("detach Redis replica %s: %w", member.podName, err)
			}
		}
		for _, peer := range retained {
			err := peer.client.ClusterForget(ctx, nodeID).Err()
			if err != nil && err.Error() != "ERR Unknown node "+nodeID {
				return fmt.Errorf("forget Redis node %s on pod %s: %w", nodeID, peer.podName, err)
			}
		}
	}
	return nil
}
