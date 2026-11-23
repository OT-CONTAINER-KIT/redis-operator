package redis

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestParseSentinelMasterState(t *testing.T) {
	valid := []interface{}{"ip", "192.0.2.1", "port", int64(6379), "flags", "master", "config-epoch", "8"}
	state, err := parseSentinelMaster(valid)
	require.NoError(t, err)
	require.Equal(t, "6379", state.Port)
	require.Equal(t, "8", state.Epoch)
	for _, invalid := range [][]interface{}{
		nil,
		{"ip"},
		{int64(1), "value"},
		{"ip", "192.0.2.1"},
		{"ip", "192.0.2.1", "port", "6379", "flags", "master", "config-epoch", "8", "config-epoch", "9"},
		{"ip", "192.0.2.1", "port", "6379", "flags", "master", "config-epoch", true},
	} {
		_, err := parseSentinelMaster(invalid)
		require.ErrorIs(t, err, ErrInvalidSentinelReport)
	}
}
