// Design: plan/spec-healthcheck-0-umbrella.md -- probe shell command execution
// Overview: healthcheck.go -- plugin lifecycle and probe management
package healthcheck

import (
	"context"
	"io"
	"os/exec"
	"syscall"
	"time"
)

const maxOutputBytes = 64 * 1024 // 64KB cap on captured probe output (#9)

// runProbeCommand executes a shell command and returns true if it exits 0.
// Uses process group isolation and kills the entire group on timeout.
func runProbeCommand(ctx context.Context, command string, timeoutSec uint32) bool {
	timeout := time.Duration(timeoutSec) * time.Second
	cmdCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	cmd := exec.CommandContext(cmdCtx, "/bin/sh", "-c", command) //nolint:gosec // admin-controlled config value
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	cmd.Cancel = func() error {
		if cmd.Process == nil {
			return nil
		}
		return syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL)
	}

	// Capture output with bounded buffer to prevent unbounded allocation (#9).
	output := &limitedBuffer{max: maxOutputBytes}
	cmd.Stdout = output
	cmd.Stderr = output

	if err := cmd.Run(); err != nil {
		if cmdCtx.Err() != nil {
			logger().Warn("probe timed out", "command", command, "timeout", timeout)
			return false
		}
		if output.Len() > 0 {
			logger().Warn("probe failed", "command", command, "output", output.String())
		}
		return false
	}

	if output.Len() > 0 {
		logger().Debug("probe succeeded", "command", command, "output", output.String())
	}
	return true
}

// limitedBuffer is a write buffer that silently discards data beyond max bytes.
type limitedBuffer struct {
	buf []byte
	max int
}

func (b *limitedBuffer) Write(p []byte) (int, error) {
	remaining := b.max - len(b.buf)
	if remaining <= 0 {
		return io.Discard.Write(p)
	}
	if len(p) > remaining {
		b.buf = append(b.buf, p[:remaining]...)
		return len(p), nil // report full write to avoid cmd error
	}
	b.buf = append(b.buf, p...)
	return len(p), nil
}

func (b *limitedBuffer) Len() int       { return len(b.buf) }
func (b *limitedBuffer) String() string { return string(b.buf) }
