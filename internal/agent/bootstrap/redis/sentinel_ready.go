package bootstrap

import (
	"io"
	"os"
	"path/filepath"
)

// Copy the statically built agent into the shared configuration volume. The
// runtime Redis image need not include the operator or an extra timeout tool.
// exec replaces the shell so kubelet cancellation cannot orphan CLI children.
func writeStandaloneReadinessScript(directory string) error {
	if err := os.MkdirAll(directory, 0o755); err != nil {
		return err
	}
	executable, err := os.Executable()
	if err != nil {
		return err
	}
	source, err := os.Open(executable)
	if err != nil {
		return err
	}
	defer source.Close()
	target, err := os.CreateTemp(directory, ".readiness-*")
	if err != nil {
		return err
	}
	defer os.Remove(target.Name())
	defer target.Close()
	if _, err := io.Copy(target, source); err != nil {
		return err
	}
	if err := target.Chmod(0o755); err != nil {
		return err
	}
	if err := target.Close(); err != nil {
		return err
	}
	if err := os.Rename(target.Name(), filepath.Join(directory, "sentinel-ready")); err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(directory, "sentinel-ready.sh"), []byte("#!/bin/sh\nexec /etc/redis/sentinel-ready agent readiness\n"), 0o755)
}
