// Design: docs/architecture/testing/interop.md -- host process boundary for Docker labs
// Related: docker.go -- the closed Docker command grammar.
//
// Package interoplab provides the protocol-neutral Docker engine for Ze
// interoperability labs.
package interoplab

import (
	"context"
	"errors"
	"io"
	"os/exec"
	"syscall"
	"time"

	"github.com/ze-software/ze/internal/core/textbuf"
)

const timeoutExitCode = 124

// ProcessCommand is one bounded host process the wrapper runs: a docker
// command, or the lab's own Go cross-compile.
type ProcessCommand struct {
	Arguments []string
	Timeout   time.Duration
	// Directory is the working directory the command runs in. Empty inherits
	// this process's, which is what every Docker command wants.
	Directory string
	// Environment is the COMPLETE environment the command runs under, not an
	// addition to this process's. Empty inherits this process's. A lab
	// cross-compile names one (zebuild.go), because the toolchain it must run
	// under is derived from the checkout rather than from the machine.
	Environment []string
	// Stdin, Stdout and Stderr attach the process to the caller's streams. A
	// nil Stdout or Stderr is captured into the ProcessResult instead; a set
	// one is not, so a one-shot that streams an hour of build output to the
	// operator's terminal holds none of it in memory. A nil Stdin reads
	// nothing.
	Stdin  io.Reader
	Stdout io.Writer
	Stderr io.Writer
}

// ProcessResult is what a ProcessCommand left: the captured streams (empty
// for a stream the command attached) and the exit code.
type ProcessResult struct {
	Stdout   string
	Stderr   string
	ExitCode int
}

// ProcessRunner is the wrapper's host process boundary. NewDocker runs the
// system's; a test hands NewFixtureDocker a double that records the argv.
type ProcessRunner interface {
	Run(context.Context, ProcessCommand) (ProcessResult, error)
}

type systemProcessRunner struct{}

func (systemProcessRunner) Run(ctx context.Context, command ProcessCommand) (ProcessResult, error) {
	if len(command.Arguments) == 0 {
		return ProcessResult{}, errors.New("process command has no arguments")
	}
	if command.Timeout <= 0 {
		return ProcessResult{}, errors.New("process command timeout must be positive")
	}

	runCtx, cancel := context.WithTimeout(ctx, command.Timeout)
	defer cancel()

	//nolint:gosec // Every argv comes from the closed Docker grammar in docker.go or the fixed Go build in zebuild.go.
	cmd := exec.CommandContext(runCtx, command.Arguments[0], command.Arguments[1:]...)
	cmd.Dir = command.Directory
	// A nil Env inherits this process's, which is what os/exec already does, so
	// an empty declaration needs no branch here.
	cmd.Env = command.Environment
	var stdout textbuf.Buffer
	var stderr textbuf.Buffer
	cmd.Stdin = command.Stdin
	cmd.Stdout = &stdout
	if command.Stdout != nil {
		cmd.Stdout = command.Stdout
	}
	cmd.Stderr = &stderr
	if command.Stderr != nil {
		cmd.Stderr = command.Stderr
	}
	err := cmd.Run()
	result := ProcessResult{Stdout: stdout.String(), Stderr: stderr.String()}
	if err == nil {
		return result, nil
	}
	if runCtx.Err() != nil {
		result.ExitCode = timeoutExitCode
		return result, runCtx.Err()
	}
	var exit *exec.ExitError
	if !errors.As(err, &exit) {
		return result, err
	}
	result.ExitCode = processExitCode(exit)
	return result, nil
}

func processExitCode(exit *exec.ExitError) int {
	if code := exit.ExitCode(); code >= 0 {
		return code
	}
	status, ok := exit.Sys().(syscall.WaitStatus)
	if !ok {
		return 1
	}
	if !status.Signaled() {
		return 1
	}
	return 128 + int(status.Signal())
}
