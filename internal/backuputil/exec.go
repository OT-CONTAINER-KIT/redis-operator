/*
Copyright 2020 Opstree Solutions.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package backuputil

import (
	"archive/tar"
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/client-go/kubernetes"
	k8sscheme "k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/remotecommand"
	utilexec "k8s.io/utils/exec"
)

// Executor runs commands and moves files in and out of a Target's container.
//
// Every transfer is streamed. Nothing here buffers a whole RDB or AOF in
// memory, so a backup larger than the operator's memory limit does not OOM it.
type Executor struct {
	K8sClient  kubernetes.Interface
	RESTConfig *rest.Config
}

func (x *Executor) stream(ctx context.Context, t Target, cmd []string, opts remotecommand.StreamOptions, stdin bool) error {
	req := x.K8sClient.CoreV1().RESTClient().Post().
		Resource("pods").
		Name(t.Pod).
		Namespace(t.Namespace).
		SubResource("exec")

	req.VersionedParams(&corev1.PodExecOptions{
		Container: t.Container,
		Command:   cmd,
		Stdin:     stdin,
		Stdout:    opts.Stdout != nil,
		Stderr:    opts.Stderr != nil,
	}, k8sscheme.ParameterCodec)

	exec, err := remotecommand.NewSPDYExecutor(x.RESTConfig, "POST", req.URL())
	if err != nil {
		return fmt.Errorf("failed to create SPDY executor for %s/%s: %w", t.Namespace, t.Pod, err)
	}
	return exec.StreamWithContext(ctx, opts)
}

// Exec runs a command and returns its stdout.
//
// The command is always passed as an argv slice and never assembled into a
// shell string, so no caller can smuggle shell metacharacters through it.
func (x *Executor) Exec(ctx context.Context, t Target, cmd ...string) (string, error) {
	var stdout, stderr bytes.Buffer
	err := x.stream(ctx, t, cmd, remotecommand.StreamOptions{Stdout: &stdout, Stderr: &stderr}, false)
	if err != nil {
		return "", fmt.Errorf("exec %v in %s failed (stderr: %s): %w",
			cmd, t.Pod, strings.TrimSpace(stderr.String()), err)
	}
	return stdout.String(), nil
}

// RedisCLI runs redis-cli inside the container and returns trimmed stdout.
//
// Authentication is inherited from the container's own REDISCLI_AUTH
// environment variable, which the operator sets when a password is configured,
// so no credential is passed on the command line or logged.
//
// The command runs through a fixed shell shim that mirrors what the operator's
// own probes and execs do (internal/k8sutils/statefulset.go getProbeInfo and
// redisCLIAuthSanitizer, internal/k8sutils/redis.go getRedisTLSArgs):
//
//   - REDISCLI_AUTH is stripped of CR/LF first. A password created with
//     `echo pw | base64` carries a trailing newline; the server trims it when
//     loading requirepass, the raw env var does not, and every call would be
//     WRONGPASS.
//   - When the container was started with TLS_MODE=true, --tls and the cert
//     paths the operator injected are added. Under TLS the plaintext port is 0
//     and only tls-port listens, so a bare redis-cli is refused forever.
//   - -h $(hostname) and -p ${REDIS_PORT} match the probe, so whatever the
//     probe can reach, this can too.
//
// The shim is a constant string. Caller arguments are forwarded via "$@" and
// never spliced into it, so no argument can reach the shell as code.
//
// -e makes redis-cli exit non-zero on an error reply. Without it FLUSHALL on a
// read-only replica, or any command against a server still LOADING, prints
// the -ERR line and exits 0, and every err != nil check here would pass.
func (x *Executor) RedisCLI(ctx context.Context, t Target, args ...string) (string, error) {
	argv := append([]string{"sh", "-c", redisCLIShim, "sh"}, args...)
	out, err := x.Exec(ctx, t, argv...)
	return strings.TrimSpace(out), err
}

// redisCLIShim is the constant prelude RedisCLI runs redis-cli through.
// --insecure matches getRedisTLSArgs: the operator's exec path does not pin
// the server certificate over the in-pod loopback either.
const redisCLIShim = `if [ -n "${REDISCLI_AUTH:-}" ]; then REDISCLI_AUTH="$(printf %s "$REDISCLI_AUTH" | tr -d '\r\n')"; export REDISCLI_AUTH; fi
TLS=""
if [ "${TLS_MODE:-}" = "true" ]; then
  TLS="--tls --insecure --cert ${REDIS_TLS_CERT} --key ${REDIS_TLS_CERT_KEY}"
  if [ -n "${REDIS_TLS_CA_CERT:-}" ]; then TLS="$TLS --cacert ${REDIS_TLS_CA_CERT}"; fi
fi
exec redis-cli -e -h "$(hostname)" -p "${REDIS_PORT:-6379}" $TLS "$@"`

// ConfigGet reads one Redis configuration parameter from the live server.
// It returns an empty string when the parameter is unknown to this server.
func (x *Executor) ConfigGet(ctx context.Context, t Target, param string) (string, error) {
	out, err := x.RedisCLI(ctx, t, "CONFIG", "GET", param)
	if err != nil {
		return "", err
	}
	lines := strings.Split(strings.ReplaceAll(out, "\r\n", "\n"), "\n")
	if len(lines) < 2 {
		return "", nil
	}
	return strings.TrimSpace(lines[1]), nil
}

// FileExists reports whether a path exists inside the container.
func (x *Executor) FileExists(ctx context.Context, t Target, path string) bool {
	_, err := x.Exec(ctx, t, "test", "-e", path)
	return err == nil
}

// copyFromAttempts is how many times a transfer is retried.
//
// A long-running exec stream can be truncated mid-transfer without the command
// itself failing, which surfaces as an unexpected EOF partway through the tar.
// That is transient, so the transfer is retried rather than failing the backup.
const copyFromAttempts = 3

// CopyFrom streams entries out of remoteDir in the container into localDir.
//
// The tar stream is consumed incrementally through an io.Pipe rather than
// collected into a bytes.Buffer first, so the transfer size is independent of
// the operator's memory limit.
func (x *Executor) CopyFrom(ctx context.Context, t Target, remoteDir string, entries []string, localDir string) error {
	if len(entries) == 0 {
		return nil
	}

	var lastErr error
	for attempt := 1; attempt <= copyFromAttempts; attempt++ {
		if attempt > 1 {
			// Discard whatever the failed attempt wrote; a partial file must
			// never end up in the backup.
			if err := os.RemoveAll(localDir); err != nil {
				return fmt.Errorf("failed to clear %s before retry: %w", localDir, err)
			}
			if err := os.MkdirAll(localDir, 0o750); err != nil {
				return fmt.Errorf("failed to recreate %s before retry: %w", localDir, err)
			}
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(time.Duration(attempt) * time.Second):
			}
		}

		err := x.copyFromOnce(ctx, t, remoteDir, entries, localDir)
		if err == nil {
			return nil
		}
		lastErr = err
	}
	return fmt.Errorf("copying %v from %s failed after %d attempts: %w",
		entries, t.Pod, copyFromAttempts, lastErr)
}

func (x *Executor) copyFromOnce(ctx context.Context, t Target, remoteDir string, entries []string, localDir string) error {
	cmd := append([]string{"tar", "cf", "-", "-C", remoteDir}, entries...)

	pr, pw := io.Pipe()
	var stderr bytes.Buffer
	var streamErr error
	done := make(chan struct{})

	go func() {
		defer close(done)
		streamErr = x.stream(ctx, t, cmd, remotecommand.StreamOptions{Stdout: pw, Stderr: &stderr}, false)
		if streamErr != nil && tarFileChangedOnly(streamErr, stderr.String()) {
			// GNU tar exits 1 when a file grew while it was being read — the
			// live incr AOF does exactly that under write traffic. The archive
			// it produced is complete and loadable; only the exit status is
			// noise. (busybox tar, used by the operator's own images, does not
			// report this at all.)
			streamErr = nil
		}
		if streamErr != nil {
			streamErr = fmt.Errorf("tar create in %s failed (stderr: %s): %w",
				t.Pod, strings.TrimSpace(stderr.String()), streamErr)
		}
		_ = pw.CloseWithError(streamErr)
	}()

	extractErr := extractTar(pr, localDir)
	if extractErr != nil {
		_ = pr.CloseWithError(extractErr)
	}
	<-done

	// The command's own failure explains the truncation better than the
	// resulting short read, so report it in preference.
	if streamErr != nil {
		return streamErr
	}
	return extractErr
}

// CopyTo streams localDir's contents into remoteDir inside the container.
//
// The archive is generated on the fly into the exec stdin, so the source file
// is never held in memory (the previous implementation held it twice).
func (x *Executor) CopyTo(ctx context.Context, t Target, localDir, remoteDir string) error {
	pr, pw := io.Pipe()

	go func() {
		tw := tar.NewWriter(pw)
		err := filepath.Walk(localDir, func(path string, info os.FileInfo, err error) error {
			if err != nil {
				return err
			}
			rel, err := filepath.Rel(localDir, path)
			if err != nil {
				return err
			}
			if rel == "." {
				return nil
			}
			hdr, err := tar.FileInfoHeader(info, "")
			if err != nil {
				return err
			}
			hdr.Name = filepath.ToSlash(rel)
			if info.IsDir() {
				hdr.Name += "/"
			}
			if err := tw.WriteHeader(hdr); err != nil {
				return err
			}
			if info.IsDir() || !info.Mode().IsRegular() {
				return nil
			}
			// #nosec G304,G122 -- path is produced by walking localDir, a
			// directory this process created under os.MkdirTemp.
			f, err := os.Open(path)
			if err != nil {
				return err
			}
			defer f.Close()
			_, err = io.Copy(tw, f)
			return err
		})
		if err == nil {
			err = tw.Close()
		} else {
			_ = tw.Close()
		}
		_ = pw.CloseWithError(err)
	}()

	var stderr bytes.Buffer
	err := x.stream(ctx, t, []string{"tar", "xf", "-", "-C", remoteDir},
		remotecommand.StreamOptions{Stdin: pr, Stdout: io.Discard, Stderr: &stderr}, true)
	if err != nil {
		return fmt.Errorf("tar extract in %s failed (stderr: %s): %w",
			t.Pod, strings.TrimSpace(stderr.String()), err)
	}
	return nil
}

// tarFileChangedOnly reports whether a tar failure is GNU tar's exit status 1
// with nothing worse than "file changed as we read it" on stderr.
func tarFileChangedOnly(err error, stderr string) bool {
	var codeErr utilexec.CodeExitError
	if !errors.As(err, &codeErr) || codeErr.ExitStatus() != 1 {
		return false
	}
	for line := range strings.SplitSeq(strings.TrimSpace(stderr), "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.Contains(line, "file changed as we read it") ||
			strings.Contains(line, "Exiting with failure status due to previous errors") {
			continue
		}
		return false
	}
	return true
}

// extractTar writes a tar stream into destDir.
//
// Entry names come from inside the target container, so they are untrusted.
// Every write goes through an *os.Root scoped to destDir: the kernel-level
// traversal check refuses any path that resolves outside the root, including
// via "..", an absolute name, or a symlink planted earlier in the same archive
// (which a purely lexical check cannot catch).
func extractTar(r io.Reader, destDir string) error {
	root, err := os.OpenRoot(destDir)
	if err != nil {
		return fmt.Errorf("failed to open %s as a root: %w", destDir, err)
	}
	defer root.Close()

	tr := tar.NewReader(r)
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			return nil
		}
		if err != nil {
			return fmt.Errorf("failed to read tar stream: %w", err)
		}

		name, err := sanitizeEntryName(hdr.Name)
		if err != nil {
			return err
		}
		if name == "" {
			continue
		}

		switch hdr.Typeflag {
		case tar.TypeDir:
			if err := root.MkdirAll(name, 0o750); err != nil {
				return fmt.Errorf("failed to create dir %s: %w", name, err)
			}
		case tar.TypeReg:
			if parent := filepath.Dir(name); parent != "." {
				if err := root.MkdirAll(parent, 0o750); err != nil {
					return fmt.Errorf("failed to create parent of %s: %w", name, err)
				}
			}
			out, err := root.OpenFile(name, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0o600)
			if err != nil {
				return fmt.Errorf("failed to create file %s: %w", name, err)
			}
			// Copy exactly the number of bytes the header declares rather than
			// draining the stream, so a malformed archive cannot write
			// unbounded data into the operator's filesystem.
			if _, err := io.CopyN(out, tr, hdr.Size); err != nil && err != io.EOF {
				_ = out.Close()
				return fmt.Errorf("failed to write file %s: %w", name, err)
			}
			if err := out.Close(); err != nil {
				return fmt.Errorf("failed to close file %s: %w", name, err)
			}
		default:
			// Symlinks and devices are never part of a Redis data directory,
			// and allowing them is how archives escape their destination.
			continue
		}
	}
}

// sanitizeEntryName turns an archive entry name into a relative path that
// cannot escape its destination, rejecting anything that tries.
func sanitizeEntryName(name string) (string, error) {
	cleaned := filepath.Clean(filepath.FromSlash(name))
	if cleaned == "." || cleaned == string(os.PathSeparator) {
		return "", nil
	}
	if filepath.IsAbs(cleaned) || strings.HasPrefix(cleaned, ".."+string(os.PathSeparator)) || cleaned == ".." {
		return "", fmt.Errorf("archive entry %q escapes the destination directory", name)
	}
	return cleaned, nil
}
