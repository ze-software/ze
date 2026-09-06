// Design: docs/architecture/testing/interop.md -- host process boundary for Docker labs
// Related: docker.go -- the closed Docker command grammar.
//
// Package interoplab provides the protocol-neutral Docker engine for Ze
// interoperability labs.
package interoplab

import (
	"context"
	"errors"
	"os/exec"
	"syscall"
	"time"

	"github.com/ze-software/ze/internal/core/textbuf"
)

const timeoutExitCode = 124

type processCommand struct {
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
}

type processResult struct {
	Stdout   string
	Stderr   string
	ExitCode int
}

type processRunner interface {
	Run(context.Context, processCommand) (processResult, error)
}

type systemProcessRunner struct{}

func (systemProcessRunner) Run(ctx context.Context, command processCommand) (processResult, error) {
	if len(command.Arguments) == 0 {
		return processResult{}, errors.New("process command has no arguments")
	}
	if command.Timeout <= 0 {
		return processResult{}, errors.New("process command timeout must be positive")
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
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err := cmd.Run()
	result := processResult{Stdout: stdout.String(), Stderr: stderr.String()}
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
