package redis

import (
	"context"
	"errors"
	"testing"

	"github.com/go-redis/redismock/v9"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSentinelMasterParsesARESP2Reply(t *testing.T) {
	client, mock := redismock.NewClientMock()
	mock.ExpectDo("SENTINEL", "MASTER", "mymaster").SetVal([]interface{}{
		"name", "mymaster", "ip", "10.0.0.10", "port", "6379", "flags", "master,failover_in_progress", "num-slaves", int64(2),
	})

	fields, err := sentinelMaster(context.Background(), client, "mymaster")

	require.NoError(t, err)
	assert.Equal(t, map[string]string{
		"name": "mymaster", "ip": "10.0.0.10", "port": "6379", "flags": "master,failover_in_progress", "num-slaves": "2",
	}, fields)
	require.NoError(t, mock.ExpectationsWereMet())
}

// go-redis v9 negotiates RESP3 by default, where SENTINEL MASTER is a map.
func TestSentinelMasterParsesARESP3Reply(t *testing.T) {
	client, mock := redismock.NewClientMock()
	mock.ExpectDo("SENTINEL", "MASTER", "mymaster").SetVal(map[interface{}]interface{}{
		"ip": "10.0.0.10", "flags": "master", "num-other-sentinels": int64(2),
	})

	fields, err := sentinelMaster(context.Background(), client, "mymaster")

	require.NoError(t, err)
	assert.Equal(t, map[string]string{"ip": "10.0.0.10", "flags": "master", "num-other-sentinels": "2"}, fields)
}

func TestSentinelMasterIgnoresAnOddTrailingField(t *testing.T) {
	client, mock := redismock.NewClientMock()
	mock.ExpectDo("SENTINEL", "MASTER", "mymaster").SetVal([]interface{}{"ip", "10.0.0.10", "flags"})

	fields, err := sentinelMaster(context.Background(), client, "mymaster")

	require.NoError(t, err)
	assert.Equal(t, map[string]string{"ip": "10.0.0.10"}, fields)
}

// A Sentinel that does not monitor the group yet (fresh pod) is not an error:
// it simply has nothing to report.
func TestSentinelMasterReturnsNothingForAnUnknownGroup(t *testing.T) {
	client, mock := redismock.NewClientMock()
	mock.ExpectDo("SENTINEL", "MASTER", "mymaster").SetErr(errors.New("ERR No such master with that name"))

	fields, err := sentinelMaster(context.Background(), client, "mymaster")

	require.NoError(t, err)
	assert.Nil(t, fields)
}

func TestSentinelMasterReturnsOtherErrors(t *testing.T) {
	client, mock := redismock.NewClientMock()
	mock.ExpectDo("SENTINEL", "MASTER", "mymaster").SetErr(errors.New("NOAUTH Authentication required."))

	fields, err := sentinelMaster(context.Background(), client, "mymaster")

	require.EqualError(t, err, "NOAUTH Authentication required.")
	assert.Nil(t, fields)
}

func TestSentinelMasterWithoutConnectionReturnsNothing(t *testing.T) {
	fields, err := NewService().SentinelMaster(context.Background(), "mymaster")

	require.NoError(t, err)
	assert.Nil(t, fields)
}

func TestSentinelFailoverInProgress(t *testing.T) {
	tests := []struct {
		flags string
		want  bool
	}{
		{flags: "master,failover_in_progress", want: true},
		{flags: "failover_in_progress", want: true},
		{flags: "master", want: false},
		{flags: "master,s_down,o_down", want: false},
		{flags: "", want: false},
	}
	for _, tt := range tests {
		t.Run(tt.flags, func(t *testing.T) {
			assert.Equal(t, tt.want, SentinelFailoverInProgress(map[string]string{"flags": tt.flags}))
		})
	}
	assert.False(t, SentinelFailoverInProgress(nil))
}
