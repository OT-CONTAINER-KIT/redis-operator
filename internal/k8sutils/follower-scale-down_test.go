package k8sutils

import (
	"context"
	"errors"
	"strings"
	"testing"

	commonapi "github.com/OT-CONTAINER-KIT/redis-operator/api/common/v1beta2"
	rcvb2 "github.com/OT-CONTAINER-KIT/redis-operator/api/rediscluster/v1beta2"
	"github.com/go-redis/redismock/v9"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"
)

func TestPrepareRedisFollowerScaleDown(t *testing.T) {
	const leader = "primary 10.0.0.1:6379@16379 master - 0 0 1 connected 0-16383\n"
	const kept = "kept 10.0.0.2:6379@16379 slave primary 0 0 1 connected\n"
	const removed = "removed 10.0.0.3:6379@16379 slave primary 0 0 1 connected\n"
	const detached = "removed 10.0.0.3:6379@16379 myself,master - 0 0 1 connected\n"

	cr := &rcvb2.RedisCluster{
		ObjectMeta: metav1.ObjectMeta{Name: "example"},
		Spec: rcvb2.RedisClusterSpec{
			ClusterSize:   ptr.To(int32(1)),
			RedisFollower: rcvb2.RedisFollower{RedisFollower: commonapi.RedisFollower{Replicas: ptr.To(int32(1))}},
		},
	}

	tests := []struct {
		name        string
		leader      string
		kept        string
		removed     string
		beforeReset string
		commands    func(leaderMock, keptMock, removedMock redismock.ClientMock)
		wantErr     string
	}{
		{
			name:        "forget only the surplus follower",
			leader:      leader + kept + removed,
			kept:        leader + kept + removed,
			removed:     leader + kept + removed,
			beforeReset: leader + kept + removed,
			commands: func(leaderMock, keptMock, removedMock redismock.ClientMock) {
				removedMock.ExpectClusterResetSoft().SetVal("OK")
				leaderMock.ExpectClusterForget("removed").SetVal("OK")
				keptMock.ExpectClusterForget("removed").SetVal("OK")
			},
		},
		{
			name:        "abort when a peer cannot forget the node",
			leader:      leader + kept + removed,
			kept:        leader + kept + removed,
			removed:     leader + kept + removed,
			beforeReset: leader + kept + removed,
			commands: func(leaderMock, keptMock, removedMock redismock.ClientMock) {
				removedMock.ExpectClusterResetSoft().SetVal("OK")
				leaderMock.ExpectClusterForget("removed").SetVal("OK")
				keptMock.ExpectClusterForget("removed").SetErr(errors.New("temporary Redis failure"))
			},
			wantErr: "temporary Redis failure",
		},
		{
			name:    "retry after the replica was reset and only one peer forgot it",
			leader:  leader + kept,
			kept:    leader + kept + removed,
			removed: detached,
			commands: func(leaderMock, keptMock, _ redismock.ClientMock) {
				leaderMock.ExpectClusterForget("removed").SetErr(errors.New("ERR Unknown node removed"))
				keptMock.ExpectClusterForget("removed").SetVal("OK")
			},
		},
		{
			name:    "refuse a follower pod that has become a primary",
			leader:  leader + kept + removed,
			kept:    leader + kept + removed,
			removed: leader + kept + "removed 10.0.0.3:6379@16379 myself,master - 0 0 2 connected 2\n",
			wantErr: "not a slotless replica or a detached node",
		},
		{
			name:        "refuse a promotion after preflight",
			leader:      leader + kept + removed,
			kept:        leader + kept + removed,
			removed:     leader + kept + removed,
			beforeReset: leader + kept + "removed 10.0.0.3:6379@16379 myself,master - 0 0 2 connected 2\n",
			wantErr:     "its Redis role or node ID changed",
		},
		{
			name:    "refuse when the target and peer disagree about ownership",
			leader:  leader + kept + "removed 10.0.0.3:6379@16379 master - 0 0 2 connected 2\n",
			kept:    leader + kept + removed,
			removed: leader + kept + removed,
			wantErr: "does not see node removed as a slotless replica",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			leaderClient, leaderMock := redismock.NewClientMock()
			keptClient, keptMock := redismock.NewClientMock()
			removedClient, removedMock := redismock.NewClientMock()
			leaderMock.ExpectClusterNodes().SetVal(strings.Replace(tt.leader, "16379 master", "16379 myself,master", 1))
			keptMock.ExpectClusterNodes().SetVal(strings.Replace(tt.kept, "10.0.0.2:6379@16379 slave", "10.0.0.2:6379@16379 myself,slave", 1))
			removedMock.ExpectClusterNodes().SetVal(strings.Replace(tt.removed, "10.0.0.3:6379@16379 slave", "10.0.0.3:6379@16379 myself,slave", 1))
			if tt.beforeReset != "" {
				removedMock.ExpectClusterNodes().SetVal(strings.Replace(tt.beforeReset, "10.0.0.3:6379@16379 slave", "10.0.0.3:6379@16379 myself,slave", 1))
			}
			if tt.commands != nil {
				tt.commands(leaderMock, keptMock, removedMock)
			}

			clients := map[string]*redis.Client{
				"example-leader-0":   leaderClient,
				"example-follower-0": keptClient,
				"example-follower-1": removedClient,
			}
			err := prepareRedisFollowerScaleDown(context.Background(), cr, 2, func(podName string) *redis.Client {
				client, found := clients[podName]
				require.True(t, found, podName)
				return client
			})

			if tt.wantErr == "" {
				require.NoError(t, err)
			} else {
				require.ErrorContains(t, err, tt.wantErr)
			}
			require.NoError(t, leaderMock.ExpectationsWereMet())
			require.NoError(t, keptMock.ExpectationsWereMet())
			require.NoError(t, removedMock.ExpectationsWereMet())
		})
	}
}
