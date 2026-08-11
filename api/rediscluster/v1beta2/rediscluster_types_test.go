package v1beta2_test

import (
	"testing"

	common "github.com/OT-CONTAINER-KIT/redis-operator/api/common/v1beta2"
	v1beta2 "github.com/OT-CONTAINER-KIT/redis-operator/api/rediscluster/v1beta2"
	"github.com/stretchr/testify/assert"
	"k8s.io/utils/ptr"
)

func TestGetReplicaCounts(t *testing.T) {
	tests := []struct {
		name     string
		spec     v1beta2.RedisClusterSpec
		role     string
		expected int32
	}{
		{
			name: "leader uses clusterSize when no override",
			spec: v1beta2.RedisClusterSpec{
				ClusterSize: ptr.To(int32(3)),
			},
			role:     "leader",
			expected: 3,
		},
		{
			name: "leader uses RedisLeader.Replicas override",
			spec: v1beta2.RedisClusterSpec{
				ClusterSize: ptr.To(int32(3)),
				RedisLeader: v1beta2.RedisLeader{
					RedisLeader: common.RedisLeader{Replicas: ptr.To(int32(5))},
				},
			},
			role:     "leader",
			expected: 5,
		},
		{
			name: "follower uses clusterSize when no override",
			spec: v1beta2.RedisClusterSpec{
				ClusterSize: ptr.To(int32(3)),
			},
			role:     "follower",
			expected: 3,
		},
		{
			name: "follower uses Replicas override (total pods)",
			spec: v1beta2.RedisClusterSpec{
				ClusterSize: ptr.To(int32(3)),
				RedisFollower: v1beta2.RedisFollower{
					RedisFollower: common.RedisFollower{Replicas: ptr.To(int32(9))},
				},
			},
			role:     "follower",
			expected: 9,
		},
		{
			name: "follower uses ReplicasPerShard * clusterSize",
			spec: v1beta2.RedisClusterSpec{
				ClusterSize: ptr.To(int32(3)),
				RedisFollower: v1beta2.RedisFollower{
					RedisFollower: common.RedisFollower{ReplicasPerShard: ptr.To(int32(3))},
				},
			},
			role:     "follower",
			expected: 9,
		},
		{
			name: "follower ReplicasPerShard respects leader Replicas override",
			spec: v1beta2.RedisClusterSpec{
				ClusterSize: ptr.To(int32(3)),
				RedisLeader: v1beta2.RedisLeader{
					RedisLeader: common.RedisLeader{Replicas: ptr.To(int32(4))},
				},
				RedisFollower: v1beta2.RedisFollower{
					RedisFollower: common.RedisFollower{ReplicasPerShard: ptr.To(int32(2))},
				},
			},
			role:     "follower",
			expected: 8,
		},
		{
			name: "follower ReplicasPerShard=1 gives 1x leaders",
			spec: v1beta2.RedisClusterSpec{
				ClusterSize: ptr.To(int32(3)),
				RedisFollower: v1beta2.RedisFollower{
					RedisFollower: common.RedisFollower{ReplicasPerShard: ptr.To(int32(1))},
				},
			},
			role:     "follower",
			expected: 3,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.spec.GetReplicaCounts(tt.role)
			assert.Equal(t, tt.expected, got)
		})
	}
}
