// Design: docs/architecture/core-design.md -- how le runs a gate's command
// Detail: ../gotoolchain/gotoolchain.go -- the environment that command runs under
//
// Package gaterun provides the environment and command execution that a ported
// gate area needs. It derives the environment from the checkout and gives the
// child direct terminal access.
//
// These functions form one concern. A gate is a named command that Make
// previously executed. At the start of each run, Make exported GOCACHE,
// CGO_ENABLED, and GOTOOLCHAIN. The reader observed the command during execution.
// Separate functions let an area run a command without this environment or
// reader-visible output. This occurred in three Python areas before
// gateapp.default_environment existed (internal/le/leaction/leaction.go).
//
// The package streams output instead of capturing it. A suite runs for minutes,
// and a reader needs immediate output. Therefore, the child inherits this
// process's stdout and stderr. A buffer would hide the output until the suite ends.
package gaterun

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"syscall"

	"github.com/ze-software/ze/internal/core/textbuf"
)

// CannotStart is the code a command that never ran answers. It is the shell's
// own number for it, and internal/le/setup/proc.go answers the same, so a caller
// that reads gate codes apart keeps reading them apart.
const CannotStart = 127

// Announce writes an action heading to stderr. The command's answer is a
// payload on stdout, so progress must not become part of a piped document.
func Announce(action string) {
	var tb textbuf.Buffer
	tb.Str("==> ").Str(action).Byte('\n').StdErr() //nolint:errcheck // CLI output
}

// Note writes one progress line for a person watching a run. Same stream and
// same reason as Announce.
func Note(line string) {
	fmt.Fprintln(os.Stderr, line) //nolint:errcheck // CLI output
}

// Stream runs one command and sends its output directly to the terminal. It
// returns the command's exit code.
//
// dir specifies the command's directory. environ specifies its complete
// environment. The child inherits only the variables in environ. This rule
// makes Toolchain.Environment the single definition of the environment for an
// action.
//
// If a command fails to start, Stream writes the reason to stderr and returns
// CannotStart. It does not report that the command ran and failed.
func Stream(argv []string, dir string, environ []string) int {
	if len(argv) == 0 {
		Note("error: a gate declared no command to run")
		return CannotStart
	}

	// The code uses context.Background without a deadline. A gate with a wall-clock
	// cap includes the cap in its OWN argv. There, `timeout` can signal the whole
	// process group. A deadline here would kill the child but leave its
	// grandchildren alive. A ze daemon or a tacacs mock would then hold the output
	// pipe open. internal/le/functional runs every suite under `timeout` to prevent
	// this failure.
	//nolint:gosec // argv comes from an area's own table. le is a build-host tool controlled through a developer's argv
	cmd := exec.CommandContext(context.Background(), argv[0], argv[1:]...)
	cmd.Dir = dir
	cmd.Env = environ
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		var exit *exec.ExitError
		if !errors.As(err, &exit) {
			var tb textbuf.Buffer
			Note(tb.Str("  cannot run ").Str(argv[0]).Str(": ").Err(err).String())
			return CannotStart
		}
		return ExitCode(exit)
	}
	return 0
}

// ExitCode returns the status that a shell would report for a finished child.
//
// exec.ExitError returns -1 when a signal kills a child. Callers cannot use -1
// as a status. Native commit preparation distinguishes exit codes, and a
// negative number matches none. ExitCode uses the shell convention of 128 plus
// the signal.
//
// ExitCode is exported because internal/le/job starts its own child.
// internal/le/job tees that child's output to a job log. Both child execution
// paths must map a finished wait to the same status.
//
// For an error that is not an exit, ExitCode returns 1. No child status exists
// in this case. The caller receives a failure status instead of a success status.
func ExitCode(err error) int {
	var exit *exec.ExitError
	if !errors.As(err, &exit) {
		return 1
	}
	if code := exit.ExitCode(); code >= 0 {
		return code
	}
	if status, ok := exit.Sys().(syscall.WaitStatus); ok && status.Signaled() {
		return 128 + int(status.Signal())
	}
	return 1
}

// ActionReport identifies the action, command, and exit code when another
// program performs the work.
type ActionReport struct {
	Action  string   `json:"action"`
	Command []string `json:"command"`
	Code    int      `json:"code"`
}

// Run announces one action, runs its command, and preserves the command's own
// exit code.
func Run(action string, argv []string, dir string, environ []string) (ActionReport, int) {
	Announce(action)
	code := Stream(argv, dir, environ)
	return ActionReport{Action: action, Command: argv, Code: code}, code
}
