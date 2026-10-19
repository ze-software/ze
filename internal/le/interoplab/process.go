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
	"os"
	"os/exec"
	"syscall"
	"time"

	"golang.org/x/term"

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
// for a stream the command attached) and the exit code. A command its bound
// ended answers the timeout exit code (124) with context.DeadlineExceeded; a
// command whose context was canceled answers no exit code, with
// context.Canceled, and the error is what the caller reads.
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
	// The child runs in a process group of its own, so a terminal Ctrl-C (sent
	// to the terminal's foreground group) reaches le alone, and le's signal
	// root decides what stops. Sharing le's group, the docker client died with
	// the same SIGINT that started the teardown, and a second one killed the
	// teardown's own `docker rm -f` while the daemon finished the removal. The
	// cancel kills that whole group, so the compilers of a `go build` go too.
	//
	// A child handed a terminal stays in le's group: a process in a background
	// group that reads its terminal is stopped by SIGTTIN, and one that writes
	// to a terminal set `tostop` by SIGTTOU, and both would then wait out their
	// bound. Only an *os.File stream is seen; a writer that wraps a terminal
	// (an io.MultiWriter over os.Stderr) is not, and writes from a background
	// group proceed under the default `-tostop`.
	if !sharesTerminal(command) {
		cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
		cmd.Cancel = func() error { return syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL) }
	}
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
	// A deadline and a cancel are told apart. The bound ending the command is a
	// hang and answers the timeout exit code, as timeout(1) does. A cancel is
	// the caller stopping (a Ctrl-C reaching le's signal root): le killed the
	// client, so it has no exit code of its own, and answering 124 would name a
	// timeout for an interrupt.
	if errors.Is(runCtx.Err(), context.DeadlineExceeded) {
		result.ExitCode = timeoutExitCode
		return result, runCtx.Err()
	}
	if runCtx.Err() != nil {
		return result, runCtx.Err()
	}
	var exit *exec.ExitError
	if !errors.As(err, &exit) {
		return result, err
	}
	result.ExitCode = processExitCode(exit)
	return result, nil
}

// sharesTerminal reports whether any stream attached to command is a
// terminal, the case in which the child must stay in le's process group.
func sharesTerminal(command ProcessCommand) bool {
	for _, stream := range []any{command.Stdin, command.Stdout, command.Stderr} {
		file, ok := stream.(*os.File)
		if !ok {
			continue
		}
		if term.IsTerminal(int(file.Fd())) {
			return true
		}
	}
	return false
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
