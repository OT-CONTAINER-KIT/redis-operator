package k8sutils

import (
	"context"
	"fmt"
	"testing"

	"github.com/go-logr/logr"
	"github.com/go-redis/redismock/v9"
	redis "github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/assert"
)

func Test_verifyLeaderPodInfo(t *testing.T) {
	logger := logr.Discard()

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

			result := verifyLeaderPodInfo(ctx, client, logger, "test-pod")

			assert.Equal(t, tt.expectedBool, result, "Test case: "+tt.name)

			if err := mock.ExpectationsWereMet(); err != nil {
				t.Errorf("there were unmet expectations: %s", err)
			}
		})
	}
}

func Test_getRedisClusterSlots(t *testing.T) {
	logger := logr.Discard()

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

			result := getRedisClusterSlots(ctx, client, logger, tt.nodeID)

			assert.Equal(t, tt.expectedResult, result, "Test case: "+tt.name)

			if err := mock.ExpectationsWereMet(); err != nil {
				t.Errorf("there were unmet expectations: %s", err)
			}
		})
	}
}

func Test_getAttachedFollowerNodeIDs(t *testing.T) {
	logger := logr.Discard()

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

			result := getAttachedFollowerNodeIDs(ctx, client, logger, tt.masterNodeID)

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
