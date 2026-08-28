// Design: docs/architecture/core-design.md -- child process exit and signal propagation
// Related: lejob.go -- admitted jobs use the same signal forwarding primitive
package lejob

import (
	"context"
	"io"
	"os"
	"os/exec"

	"github.com/ze-software/ze/internal/le/gaterun"
)

// ProcessIO specifies the streams and environment for one child process.
type ProcessIO struct {
	Dir     string
	Environ []string
	Stdin   io.Reader
	Stdout  io.Writer
	Stderr  io.Writer
}

// RunProcess starts one child, forwards SIGINT and SIGTERM, waits, and returns
// the shell-compatible child status. A start error returns CannotStart and the
// error. A child failure is a verdict, not a start error.
func RunProcess(argv []string, processIO ProcessIO) (int, error) {
	if len(argv) == 0 {
		return gaterun.CannotStart, ErrNoCommand
	}
	if processIO.Environ == nil {
		processIO.Environ = os.Environ()
	}
	//nolint:gosec // argv is the caller's command; le is a build-host tool
	cmd := exec.CommandContext(context.Background(), argv[0], argv[1:]...)
	cmd.Dir = processIO.Dir
	cmd.Env = processIO.Environ
	cmd.Stdin = processIO.Stdin
	cmd.Stdout = processIO.Stdout
	cmd.Stderr = processIO.Stderr
	if err := cmd.Start(); err != nil {
		return gaterun.CannotStart, err
	}
	stop := forwardSignals(cmd)
	err := cmd.Wait()
	stop()
	if err != nil {
		return gaterun.ExitCode(err), nil
	}
	return 0, nil
}
