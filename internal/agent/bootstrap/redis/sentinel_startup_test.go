package bootstrap

import (
	"context"
	"net"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestStartupSourceMustBelongToReplication(t *testing.T) {
	lookup := func(context.Context, string) ([]net.IPAddr, error) {
		return []net.IPAddr{{IP: net.ParseIP("192.0.2.1")}}, nil
	}
	require.True(t, startupSourceMember(context.Background(), "192.0.2.1", []string{"member.synthetic.invalid"}, lookup))
	require.False(t, startupSourceMember(context.Background(), "192.0.2.2", []string{"member.synthetic.invalid"}, lookup))
	require.False(t, startupSourceMember(context.Background(), "192.0.2.1", nil, lookup))
	require.False(t, startupSourceMember(context.Background(), "bad-address", []string{"member.synthetic.invalid"}, lookup))
}

func TestStartupRejectsQuotedOrEscapedRoleOverrides(t *testing.T) {
	for _, line := range []string{`"replicaof" 192.0.2.200 6379`, `'dir' /other`, `RePlIcAoF 192.0.2.200 6379`, `replica\of 192.0.2.200 6379`} {
		path := filepath.Join(t.TempDir(), "external.conf")
		require.NoError(t, os.WriteFile(path, []byte(line), 0o600))
		require.Error(t, validateStartupOverrides(path))
	}
}
