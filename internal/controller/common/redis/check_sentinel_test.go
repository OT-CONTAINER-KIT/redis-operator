package redis

import (
	"context"
	"errors"
	"testing"

	redisservice "github.com/OT-CONTAINER-KIT/redis-operator/internal/service/redis"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	k8sfake "k8s.io/client-go/kubernetes/fake"
	"k8s.io/utils/ptr"
)

func sentinelMonitoringAddress(address string) *redisservice.InfoSentinelResult {
	return &redisservice.InfoSentinelResult{
		Masters: []redisservice.SentinelMasterInfo{{Name: "myMaster", Status: "ok", Address: address, Slaves: 1, Sentinels: 3}},
	}
}

func TestGetSentinelElectedMaster(t *testing.T) {
	tests := []struct {
		name         string
		sentinelInfo map[string]*redisservice.InfoSentinelResult
		want         string
	}{
		{
			// Right after a failover: the promoted pod has no replica yet and
			// the old master still has one; Sentinel already names the new one.
			name: "a quorum names the freshly promoted master",
			sentinelInfo: map[string]*redisservice.InfoSentinelResult{
				"10.0.1.10": sentinelMonitoringAddress("10.0.0.10:6379"),
				"10.0.1.11": sentinelMonitoringAddress("10.0.0.10:6379"),
				"10.0.1.12": sentinelMonitoringAddress("10.0.0.12:6379"),
			},
			want: "example-replication-0",
		},
		{
			name: "no quorum names a master",
			sentinelInfo: map[string]*redisservice.InfoSentinelResult{
				"10.0.1.10": sentinelMonitoringAddress("10.0.0.10:6379"),
				"10.0.1.11": sentinelMonitoringAddress("10.0.0.12:6379"),
			},
		},
		{
			name: "a quorum naming a replica is not trusted",
			sentinelInfo: map[string]*redisservice.InfoSentinelResult{
				"10.0.1.10": sentinelMonitoringAddress("10.0.0.11:6379"),
				"10.0.1.11": sentinelMonitoringAddress("10.0.0.11:6379"),
				"10.0.1.12": sentinelMonitoringAddress("10.0.0.11:6379"),
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rs, rr, objects := newSentinelFixture("10.0.1.10", "10.0.1.11", "10.0.1.12")
			_, replicationObjects := newReplicationFixture("10.0.0.10", "10.0.0.11", "10.0.0.12")
			objects = append(objects, replicationObjects...)
			redisClient := &fakeRedisClient{
				isMasterByHost:     map[string]bool{"10.0.0.10": true, "10.0.0.12": true},
				replicasByHost:     map[string]int{"10.0.0.12": 1},
				sentinelInfoByHost: tt.sentinelInfo,
			}
			c := &checker{redis: redisClient, k8s: k8sfake.NewSimpleClientset(objects...)}

			master, err := c.GetSentinelElectedMaster(context.Background(), rs, rr)

			require.NoError(t, err)
			assert.Equal(t, tt.want, master.Name)
		})
	}
}

// Only the Sentinel that leads a failover reports failover_in_progress.
func TestSentinelFailoverInProgress(t *testing.T) {
	tests := map[string]struct {
		flags map[string]string
		err   map[string]error
		want  bool
	}{
		"the leader reports the failover": {
			flags: map[string]string{"10.0.1.10": "master", "10.0.1.11": "master,failover_in_progress", "10.0.1.12": "master"},
			want:  true,
		},
		"no failover": {
			flags: map[string]string{"10.0.1.10": "master", "10.0.1.11": "master", "10.0.1.12": "master,s_down"},
		},
		"unreachable sentinels are skipped": {
			flags: map[string]string{"10.0.1.12": "master,failover_in_progress"},
			err:   map[string]error{"10.0.1.10": errors.New("dial tcp: i/o timeout"), "10.0.1.11": errors.New("dial tcp: i/o timeout")},
			want:  true,
		},
	}
	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			rs, _, objects := newSentinelFixture("10.0.1.10", "10.0.1.11", "10.0.1.12")
			c := &checker{
				k8s:   k8sfake.NewSimpleClientset(objects...),
				redis: &fakeRedisClient{sentinelFlagsByHost: tt.flags, errByHost: tt.err},
			}

			got, err := c.SentinelFailoverInProgress(context.Background(), rs)

			require.NoError(t, err)
			assert.Equal(t, tt.want, got)
		})
	}
}

// A Sentinel left on the bootstrap placeholder never fails over: the
// RedisSentinel controller must keep retrying until every Sentinel monitors
// the master.
func TestSentinelsMonitoring(t *testing.T) {
	rs, _, objects := newSentinelFixture("10.0.1.10", "10.0.1.11")
	c := &checker{
		k8s: k8sfake.NewSimpleClientset(objects...),
		redis: &fakeRedisClient{sentinelMasterIPByHost: map[string]string{
			"10.0.1.10": "10.0.0.10",
			"10.0.1.11": "0.0.0.0",
		}},
	}
	rs.Spec.Size = ptr.To(int32(3)) // the third Sentinel has no pod yet

	configured, expected, err := c.SentinelsMonitoring(context.Background(), rs, "10.0.0.10")

	require.NoError(t, err)
	assert.Equal(t, 1, configured)
	assert.Equal(t, 3, expected)
}

// A replica left on the IP of a recreated master, or chained to another
// replica, is invisible to Sentinel, which only learns replicas from the
// master.
func TestGetMisattachedReplicas(t *testing.T) {
	rr, objects := newReplicationFixture("10.0.0.10", "10.0.0.11", "10.0.0.12")
	c := &checker{
		k8s: k8sfake.NewSimpleClientset(objects...),
		redis: &fakeRedisClient{masterHostByHost: map[string]string{
			"10.0.0.10": "10.0.0.12",
			"10.0.0.11": "10.0.9.99",
		}},
	}

	got, err := c.GetMisattachedReplicas(context.Background(), rr, "example-replication-2",
		[]string{"example-replication-0", "example-replication-1"})

	require.NoError(t, err)
	assert.Equal(t, []string{"example-replication-1"}, got)
}

func TestGetMisattachedReplicasAcceptsTheMasterHostname(t *testing.T) {
	rr, objects := newReplicationFixture("10.0.0.10", "10.0.0.11", "10.0.0.12")
	c := &checker{
		k8s: k8sfake.NewSimpleClientset(objects...),
		redis: &fakeRedisClient{masterHostByHost: map[string]string{
			"10.0.0.10": "example-replication-2.example-replication-headless.default.svc.cluster.local",
		}},
	}

	got, err := c.GetMisattachedReplicas(context.Background(), rr, "example-replication-2", []string{"example-replication-0"})

	require.NoError(t, err)
	assert.Empty(t, got)
}
