// Design: docs/architecture/testing/ci-format.md -- test execution utilities
// Overview: runner_exec.go -- test execution and process orchestration

package runner

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"syscall"
	"time"
)

var errEmptyExecCommand = errors.New("empty exec command")

const (
	modeForeground  = "foreground"
	fileCheckFailed = "file_check_failed"
	failParseError  = "parse_error"
)

// syncWriter is an io.Writer that captures output and supports waiting for patterns.
// Used to wait for ze-peer's "listening on" message before starting the client.
type syncWriter struct {
	mu      sync.Mutex
	buf     strings.Builder
	pattern string
	found   bool
}

// peerListeningPattern is the string ze-peer prints to stdout when ready.
const peerListeningPattern = "listening on"

// newSyncWriter creates a writer that waits for ze-peer's "listening on" output.
func newSyncWriter() *syncWriter {
	return &syncWriter{pattern: peerListeningPattern}
}

// maxOutputBytes caps captured output to prevent OOM from runaway processes.
const maxOutputBytes = 10 << 20 // 10 MB

// Write captures data and checks for the pattern.
// Output is capped at maxOutputBytes to prevent unbounded memory growth.
func (sw *syncWriter) Write(p []byte) (int, error) {
	sw.mu.Lock()
	defer sw.mu.Unlock()

	if sw.buf.Len() < maxOutputBytes {
		remaining := maxOutputBytes - sw.buf.Len()
		if len(p) > remaining {
			p = p[:remaining]
		}
		sw.buf.Write(p) //nolint:errcheck // strings.Builder.Write never fails
	}
	if !sw.found && strings.Contains(sw.buf.String(), sw.pattern) {
		sw.found = true
	}
	return len(p), nil
}

// WaitFor waits until the pattern is found or context is canceled.
// Returns true if pattern was found, false on timeout/cancel.
func (sw *syncWriter) WaitFor(ctx context.Context) bool {
	ticker := time.NewTicker(10 * time.Millisecond)
	defer ticker.Stop()

	for {
		sw.mu.Lock()
		found := sw.found
		sw.mu.Unlock()

		if found {
			return true
		}

		select {
		case <-ctx.Done():
			return false
		case <-ticker.C:
		}
	}
}

// String returns all captured output.
func (sw *syncWriter) String() string {
	sw.mu.Lock()
	defer sw.mu.Unlock()
	return sw.buf.String()
}

// peerOutput tracks stdout/stderr for a single ze-peer background process.
// Multi-peer tests create one per ze-peer so each has independent WaitFor
// synchronization and output capture.
type peerOutput struct {
	stdout *syncWriter
	stderr *strings.Builder
	proc   *exec.Cmd
}

// waitReady polls for a readiness file, returning when found or timeout expires.
func waitReady(ctx context.Context, path string, timeout time.Duration) {
	waitCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	ticker := time.NewTicker(20 * time.Millisecond)
	defer ticker.Stop()
	for {
		if _, err := os.Stat(path); err == nil {
			return
		}
		select {
		case <-waitCtx.Done():
			return
		case <-ticker.C:
		}
	}
}

func zeDaemonConfigArgIndex(args []string) int {
	for i := 0; i < len(args); i++ {
		arg := args[i]
		switch arg {
		case "-d", "--debug", "--insecure-web", "--color", "--no-color":
			continue
		case "-f", "--server", "--name", "--token", "--plugin", "--pprof", "--chaos-seed", "--chaos-rate", "--mcp", "--mcp-token", "--web":
			i++
			continue
		}

		if arg == "-" || strings.HasSuffix(arg, ".conf") || strings.HasSuffix(arg, ".cfg") || strings.HasSuffix(arg, ".yaml") || strings.HasSuffix(arg, ".yml") || strings.HasSuffix(arg, ".json") {
			return i
		}
		if strings.Contains(arg, string(filepath.Separator)) || strings.HasPrefix(arg, ".") {
			return i
		}
		return -1
	}
	return -1
}

func zeDaemonUsesWeb(args []string) bool {
	for _, arg := range args {
		if arg == "--web" || strings.HasPrefix(arg, "--web=") {
			return true
		}
	}
	return false
}

func zeDaemonShouldForceFileStorage(args []string) bool {
	return zeDaemonConfigArgIndex(args) >= 0 && !zeDaemonUsesWeb(args)
}

// terminateGracefully sends SIGTERM to a process and waits for it to exit.
// If it doesn't exit within timeout, it is forcefully killed.
func terminateGracefully(cmd *exec.Cmd, timeout time.Duration) {
	if cmd.Process == nil {
		_ = cmd.Wait()
		return
	}
	_ = cmd.Process.Signal(syscall.SIGTERM)
	timer := time.AfterFunc(timeout, func() {
		_ = cmd.Process.Kill()
	})
	_ = cmd.Wait()
	timer.Stop()
}
