package k8sutils

import (
	"context"
	"fmt"
	"testing"

	"github.com/go-redis/redismock/v9"
	redis "github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/assert"
)

func Test_clusterHasEmptyMasters_WhenConnectedMasterHasNoSlots_ReturnsTrue(t *testing.T) {
	nodes := []clusterNodesResponse{
		// master with slots assigned
		{
			"id-master-with-slots",
			"10.0.0.10:6379@16379",
			"myself,master",
			"-",
			"0",
			"0",
			"1",
			"connected",
			"0-5460",
		},
		// empty master (no slots)
		{
			"id-empty-master",
			"10.0.0.11:6379@16379",
			"master",
			"-",
			"0",
			"0",
			"2",
			"connected",
			// NOTE: no slot tokens => len(fields) == 8
		},
		// a replica (should be ignored)
		{
			"id-replica",
			"10.0.0.12:6379@16379",
			"slave",
			"id-master-with-slots",
			"0",
			"0",
			"3",
			"connected",
		},
	}

	// Act:
	got := clusterHasEmptyMasters(nodes)

	// Assert:
	if !got {
		t.Fatalf("expected true, got false")
	}
}

func Test_clusterHasEmptyMasters_skipMalformedNode(t *testing.T) {
	nodes := []clusterNodesResponse{
		// master with slot token
		{
			"id-empty-master",
			"10.0.0.11:6379@16379",
			"master",
			"-",
			"0",
			"0",
			"2",
			// NOTE: missing linkState => len(fields) < 8
		},
	}

	// Act:
	got := clusterHasEmptyMasters(nodes)

	// Assert:
	if got {
		t.Fatalf("expected false, got true")
	}
}

func Test_clusterHasEmptyMasters_skipFlags_ReturnsFalse(t *testing.T) {
	tests := []struct {
		name      string
		flags     string
		linkState string
	}{
		{
			name:      "with non master flag",
			flags:     "bla",
			linkState: "connected",
		},
		{
			name:      "with master,fail flag",
			flags:     "master,fail",
			linkState: "connected",
		},
		{
			name:      "with master,handshake flag",
			flags:     "master,handshake",
			linkState: "connected",
		},
		{
			name:      "with master,noaddr flag",
			flags:     "master,noaddr",
			linkState: "connected",
		},
		{
			name:      "with non connected linkState",
			flags:     "master",
			linkState: "disconnected",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			nodes := []clusterNodesResponse{
				// master with slot token
				{
					"id-empty-master",
					"10.0.0.11:6379@16379",
					tt.flags,
					"-",
					"0",
					"0",
					"2",
					tt.linkState,
				},
			}

			// Act:
			got := clusterHasEmptyMasters(nodes)

			// Assert:
			if got {
				t.Fatalf("expected false, got true")
			}
		})
	}
}

func Test_clusterHasEmptyMasters_ReturnsTrue(t *testing.T) {
	tests := []struct {
		name              string
		secondMasterSlots string
	}{
		{
			name:              "with empty string",
			secondMasterSlots: "",
		},
		{
			name:              "with non slot value",
			secondMasterSlots: "abc",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			nodes := []clusterNodesResponse{
				// master with slots assigned
				{
					"id-master-with-slots",
					"10.0.0.10:6379@16379",
					"myself,master",
					"-",
					"0",
					"0",
					"1",
					"connected",
					"0-5460",
				},
				// second master with slot token
				{
					"id-empty-master",
					"10.0.0.11:6379@16379",
					"master",
					"-",
					"0",
					"0",
					"2",
					"connected",
					tt.secondMasterSlots,
				},
				// a replica (should be ignored)
				{
					"id-replica",
					"10.0.0.12:6379@16379",
					"slave",
					"id-master-with-slots",
					"0",
					"0",
					"3",
					"connected",
				},
			}

			// Act:
			got := clusterHasEmptyMasters(nodes)

			// Assert:
			if !got {
				t.Fatalf("expected true, got false")
			}
		})
	}
}

func Test_clusterHasEmptyMasters_ReturnsFalse(t *testing.T) {
	tests := []struct {
		name              string
		expectedBool      bool
		secondMasterSlots string
	}{
		{
			name:              "with slot (without brackets)",
			secondMasterSlots: "5461",
		}, {
			name:              "with slot (with brackets)",
			secondMasterSlots: "[5461]",
		}, {
			name:              "with migration markers",
			secondMasterSlots: "[5461->-nodeid]",
		}, {
			name:              "with migration markers",
			secondMasterSlots: "[5461->-nodeid]",
		}, {
			name:              "with import markers",
			secondMasterSlots: "[5461-<-nodeid]",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			nodes := []clusterNodesResponse{
				// master with slots assigned
				{
					"id-master-with-slots",
					"10.0.0.10:6379@16379",
					"myself,master",
					"-",
					"0",
					"0",
					"1",
					"connected",
					"0-5460",
				},
				// second master with slot token
				{
					"id-empty-master",
					"10.0.0.11:6379@16379",
					"master",
					"-",
					"0",
					"0",
					"2",
					"connected",
					tt.secondMasterSlots,
				},
				// a replica (should be ignored)
				{
					"id-replica",
					"10.0.0.12:6379@16379",
					"slave",
					"id-master-with-slots",
					"0",
					"0",
					"3",
					"connected",
				},
			}

			// Act:
			got := clusterHasEmptyMasters(nodes)

			// Assert:
			if got {
				t.Fatalf("expected false, got true")
			}
		})
	}
}

func Test_clusterStableNoOpenSlots_ReturnsTrue(t *testing.T) {
	info := "cluster_state:ok\ncluster_slot_migration_active_tasks:0\n"
	nodes := []clusterNodesResponse{
		{
			"id-1",
			"10.0.0.10:6379@16379",
			"myself,master",
			"-",
			"0",
			"0",
			"1",
			"connected",
			"0-5460",
		},
		{
			"id-2",
			"10.0.0.11:6379@16379",
			"master",
			"-",
			"0",
			"0",
			"2",
			"connected",
			"5461-10922",
		},
		{
			"id-3",
			"10.0.0.12:6379@16379",
			"slave",
			"id-1",
			"0",
			"0",
			"3",
			"connected",
		},
	}

	if got := clusterStableNoOpenSlots(info, nodes); !got {
		t.Fatalf("expected true, got false")
	}
}

// For redis 6 compatibility
func Test_clusterStableNoOpenSlots_missingMigrationActiveTask_ReturnsTrue(t *testing.T) {
	info := "cluster_state:ok\n"
	nodes := []clusterNodesResponse{
		{
			"id-1",
			"10.0.0.10:6379@16379",
			"myself,master",
			"-",
			"0",
			"0",
			"1",
			"connected",
			"0-5460",
		},
		{
			"id-2",
			"10.0.0.11:6379@16379",
			"master",
			"-",
			"0",
			"0",
			"2",
			"connected",
			"5461-10922",
		},
		{
			"id-3",
			"10.0.0.12:6379@16379",
			"slave",
			"id-1",
			"0",
			"0",
			"3",
			"connected",
		},
	}

	if got := clusterStableNoOpenSlots(info, nodes); !got {
		t.Fatalf("expected true, got false")
	}
}

func Test_clusterStableNoOpenSlots_malformedNode_ReturnsFalse(t *testing.T) {
	info := "cluster_state:ok\ncluster_slot_migration_active_tasks:0\n"
	nodes := []clusterNodesResponse{
		{
			"id-1",
			"10.0.0.10:6379@16379",
			"myself,master",
			"-",
			"0",
			"0",
			"1",
		},
	}

	if got := clusterStableNoOpenSlots(info, nodes); got {
		t.Fatalf("expected false, got true")
	}
}

func Test_clusterStableNoOpenSlots_ReturnsFalse(t *testing.T) {
	tests := []struct {
		name                            string
		clusterState                    string
		clusterSlotMigrationActiveTasks string
		nodeFields                      string
		linkState                       string
		slots                           string
	}{
		{
			name:                            "cluster state no ok",
			clusterState:                    "bad",
			clusterSlotMigrationActiveTasks: "0",
			nodeFields:                      "master",
			linkState:                       "connected",
			slots:                           "5461",
		},
		{
			name:                            "active migration tasks",
			clusterState:                    "ok",
			clusterSlotMigrationActiveTasks: "2",
			nodeFields:                      "master",
			linkState:                       "connected",
			slots:                           "5461",
		},
		{
			name:                            "handshake flag",
			clusterState:                    "ok",
			clusterSlotMigrationActiveTasks: "0",
			nodeFields:                      "master,handshake",
			linkState:                       "connected",
			slots:                           "5461",
		},
		{
			name:                            "fail flag",
			clusterState:                    "ok",
			clusterSlotMigrationActiveTasks: "0",
			nodeFields:                      "master,fail",
			linkState:                       "connected",
			slots:                           "5461",
		},
		{
			name:                            "noaddr flag",
			clusterState:                    "ok",
			clusterSlotMigrationActiveTasks: "0",
			nodeFields:                      "master,noaddr",
			linkState:                       "connected",
			slots:                           "5461",
		},
		{
			name:                            "linkState not connected",
			clusterState:                    "ok",
			clusterSlotMigrationActiveTasks: "0",
			nodeFields:                      "master",
			linkState:                       "disconnected",
			slots:                           "5461",
		},
		{
			name:                            "migrating slots",
			clusterState:                    "ok",
			clusterSlotMigrationActiveTasks: "0",
			nodeFields:                      "master",
			linkState:                       "connected",
			slots:                           "[5491->-6000]",
		},
		{
			name:                            "importing slots",
			clusterState:                    "ok",
			clusterSlotMigrationActiveTasks: "0",
			nodeFields:                      "master",
			linkState:                       "connected",
			slots:                           "[5491-<-6000]",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			info := "cluster_state:" + tt.clusterState + "\ncluster_slot_migration_active_tasks:" + tt.clusterSlotMigrationActiveTasks + "\n"

			nodes := []clusterNodesResponse{
				{
					"id-1",
					"10.0.0.10:6379@16379",
					"master",
					"-",
					"0",
					"0",
					"1",
					"connected",
					"0-5460",
				},
				{
					"id-2",
					"10.0.0.11:6379@16379",
					tt.nodeFields,
					"-",
					"0",
					"0",
					"2",
					tt.linkState,
					tt.slots,
				},
			}
			if got := clusterStableNoOpenSlots(info, nodes); got {
				t.Fatalf("expected false, got true")
			}
		})
	}
}

func Test_verifyLeaderPodInfo(t *testing.T) {
	tests := []struct {
		name         string
		section      string
		response     string
		err          error
		expectedBool bool
	}{
		{
			name:         "is master",
			section:      "replication",
			response:     "role:master\r\n",
			expectedBool: true,
		},
		{
			name:         "is replica",
			section:      "replication",
			response:     "role:slave\r\n",
			expectedBool: false,
		},
		{
			name:         "redis info error",
			section:      "replication",
			err:          redis.ErrClosed,
			expectedBool: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()
			client, mock := redismock.NewClientMock()

			if tt.err != nil {
				mock.ExpectInfo(tt.section).SetErr(tt.err)
			} else {
				mock.ExpectInfo(tt.section).SetVal(tt.response)
			}

			result := verifyLeaderPodInfo(ctx, client, "test-pod")

			assert.Equal(t, tt.expectedBool, result, "Test case: "+tt.name)

			if err := mock.ExpectationsWereMet(); err != nil {
				t.Errorf("there were unmet expectations: %s", err)
			}
		})
	}
}

func Test_getRedisClusterSlots(t *testing.T) {
	tests := []struct {
		name            string
		nodeID          string
		clusterSlots    []redis.ClusterSlot
		clusterSlotsErr error
		expectedResult  string
	}{
		{
			name:   "successful slot count",
			nodeID: "node123",
			clusterSlots: []redis.ClusterSlot{
				{Start: 0, End: 4999, Nodes: []redis.ClusterNode{{ID: "node123"}}},
				{Start: 5000, End: 9999, Nodes: []redis.ClusterNode{{ID: "node123"}}},
			},
			expectedResult: "10000",
		},
		{
			name:            "error fetching cluster slots",
			nodeID:          "node123",
			clusterSlotsErr: redis.ErrClosed,
			expectedResult:  "",
		},
		{
			name:   "no slots for node",
			nodeID: "node999",
			clusterSlots: []redis.ClusterSlot{
				{Start: 0, End: 4999, Nodes: []redis.ClusterNode{{ID: "node123"}}},
			},
			expectedResult: "0",
		},
		{
			name:   "slots for multiple nodes",
			nodeID: "node123",
			clusterSlots: []redis.ClusterSlot{
				{Start: 0, End: 1999, Nodes: []redis.ClusterNode{{ID: "node123"}}},
				{Start: 2000, End: 3999, Nodes: []redis.ClusterNode{{ID: "node456"}}},
				{Start: 4000, End: 5999, Nodes: []redis.ClusterNode{{ID: "node123"}, {ID: "node789"}}},
			},
			expectedResult: "4000",
		},
		{
			name:   "single slot range",
			nodeID: "node123",
			clusterSlots: []redis.ClusterSlot{
				{Start: 100, End: 100, Nodes: []redis.ClusterNode{{ID: "node123"}}},
			},
			expectedResult: "1",
		},
		{
			name:   "mixed slot ranges",
			nodeID: "node123",
			clusterSlots: []redis.ClusterSlot{
				{Start: 0, End: 499, Nodes: []redis.ClusterNode{{ID: "node123"}}},
				{Start: 500, End: 999, Nodes: []redis.ClusterNode{{ID: "node123"}, {ID: "node999"}}},
				{Start: 1000, End: 1499, Nodes: []redis.ClusterNode{{ID: "node999"}}},
				{Start: 1500, End: 1999, Nodes: []redis.ClusterNode{{ID: "node123"}}},
			},
			expectedResult: "1500",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.TODO()
			client, mock := redismock.NewClientMock()

			if tt.clusterSlotsErr != nil {
				mock.ExpectClusterSlots().SetErr(tt.clusterSlotsErr)
			} else {
				mock.ExpectClusterSlots().SetVal(tt.clusterSlots)
			}

			result := getRedisClusterSlots(ctx, client, tt.nodeID)

			assert.Equal(t, tt.expectedResult, result, "Test case: "+tt.name)

			if err := mock.ExpectationsWereMet(); err != nil {
				t.Errorf("there were unmet expectations: %s", err)
			}
		})
	}
}

func Test_getAttachedFollowerNodeIDs(t *testing.T) {
	tests := []struct {
		name                 string
		masterNodeID         string
		slaveNodeIDs         []string
		clusterSlavesErr     error
		expectedslaveNodeIDs []string
	}{
		{
			name:                 "successful retrieval of slave nodes",
			masterNodeID:         "master123",
			slaveNodeIDs:         []string{"slave1", "slave2"},
			expectedslaveNodeIDs: []string{"slave1", "slave2"},
		},
		{
			name:                 "error fetching slave nodes",
			masterNodeID:         "master123",
			clusterSlavesErr:     redis.ErrClosed,
			expectedslaveNodeIDs: nil,
		},
		{
			name:                 "no attached slave nodes",
			masterNodeID:         "master456",
			slaveNodeIDs:         []string{},
			expectedslaveNodeIDs: []string{},
		},
		{
			name:                 "nil response for slave nodes",
			masterNodeID:         "masterNode123",
			slaveNodeIDs:         nil,
			expectedslaveNodeIDs: nil,
			clusterSlavesErr:     nil,
		},
		{
			name:                 "large number of attached slave nodes",
			masterNodeID:         "master123",
			slaveNodeIDs:         generateLargeListOfSlaves(1000), // Helper function needed
			expectedslaveNodeIDs: generateLargeListOfSlaves(1000),
		},
		{
			name:                 "invalid master node ID",
			masterNodeID:         "invalidMasterID",
			slaveNodeIDs:         nil,
			expectedslaveNodeIDs: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()
			client, mock := redismock.NewClientMock()

			if tt.clusterSlavesErr != nil {
				mock.ExpectClusterSlaves(tt.masterNodeID).SetErr(tt.clusterSlavesErr)
			} else {
				mock.ExpectClusterSlaves(tt.masterNodeID).SetVal(tt.slaveNodeIDs)
			}

			result := getAttachedFollowerNodeIDs(ctx, client, tt.masterNodeID)

			assert.ElementsMatch(t, tt.expectedslaveNodeIDs, result, "Test case: "+tt.name)

			if err := mock.ExpectationsWereMet(); err != nil {
				t.Errorf("there were unmet expectations: %s", err)
			}
		})
	}
}

func generateLargeListOfSlaves(n int) []string {
	var slaves []string
	for i := 0; i < n; i++ {
		slaves = append(slaves, fmt.Sprintf("slaveNode%d", i))
	}
	return slaves
}
