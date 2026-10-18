package redis

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCountKeyspaceKeys(t *testing.T) {
	tests := []struct {
		name string
		info string
		want int64
	}{
		{name: "empty instance", info: "# Keyspace\r\n", want: 0},
		{name: "single db", info: "# Keyspace\r\ndb0:keys=120,expires=3,avg_ttl=0\r\n", want: 120},
		{name: "multiple dbs", info: "# Keyspace\r\ndb0:keys=120,expires=3,avg_ttl=0\r\ndb3:keys=7,expires=0,avg_ttl=0,subexpiry=0\r\n", want: 127},
		{name: "zero keys", info: "# Keyspace\r\ndb0:keys=0,expires=0,avg_ttl=0\r\n", want: 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := countKeyspaceKeys(tt.info)
			require.NoError(t, err)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestCountKeyspaceKeysRejectsMalformedCount(t *testing.T) {
	_, err := countKeyspaceKeys("# Keyspace\r\ndb0:keys=many,expires=0,avg_ttl=0\r\n")
	assert.Error(t, err)
}
