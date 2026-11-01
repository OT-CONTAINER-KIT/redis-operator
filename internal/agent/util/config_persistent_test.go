package util

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestInitialConfigurationCannotOverwriteElectionState(t *testing.T) {
	path := filepath.Join(t.TempDir(), "sentinel.conf")
	require.NoError(t, NewConfig(path, "synthetic-election-history\n").CommitIfAbsent(0o600))
	err := NewConfig(path, "replacement-must-not-win\n").CommitIfAbsent(0o600)
	require.Error(t, err)
	data, err := os.ReadFile(path)
	require.NoError(t, err)
	require.Equal(t, "synthetic-election-history\n", string(data))
	entries, err := os.ReadDir(filepath.Dir(path))
	require.NoError(t, err)
	require.Len(t, entries, 1, "normal initialization must remove its temporary files")
}
