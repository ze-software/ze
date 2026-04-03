// Design: plan/spec-healthcheck-0-umbrella.md -- probe shell command execution
package healthcheck

import (
	"bytes"
	"context"
	"os/exec"
	"syscall"
	"time"
)

// runProbeCommand executes a shell command and returns true if it exits 0.
// Uses process group isolation and kills the entire group on timeout.
func runProbeCommand(ctx context.Context, command string, timeoutSec uint32) bool {
	timeout := time.Duration(timeoutSec) * time.Second
	cmdCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	cmd := exec.CommandContext(cmdCtx, "/bin/sh", "-c", command) //nolint:gosec // admin-controlled config value
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	// Kill entire process group on context cancellation (not just the shell).
	cmd.Cancel = func() error {
		return syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL)
	}

	var output bytes.Buffer
	cmd.Stdout = &output
	cmd.Stderr = &output

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
