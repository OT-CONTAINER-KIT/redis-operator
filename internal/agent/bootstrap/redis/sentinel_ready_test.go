package bootstrap

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestReadinessExecutesCopiedAgentWithoutCLIOrWatchdogChildren(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, writeStandaloneReadinessScript(dir))
	data, err := os.ReadFile(filepath.Join(dir, "sentinel-ready.sh"))
	require.NoError(t, err)
	require.Equal(t, "#!/bin/sh\nexec /etc/redis/sentinel-ready agent readiness\n", string(data))
	source, err := os.Executable()
	require.NoError(t, err)
	sourceInfo, err := os.Stat(source)
	require.NoError(t, err)
	copyInfo, err := os.Stat(filepath.Join(dir, "sentinel-ready"))
	require.NoError(t, err)
	require.Equal(t, sourceInfo.Size(), copyInfo.Size())
	require.Equal(t, os.FileMode(0o755), copyInfo.Mode().Perm())
	entries, err := os.ReadDir(dir)
	require.NoError(t, err)
	require.Len(t, entries, 2)
}
