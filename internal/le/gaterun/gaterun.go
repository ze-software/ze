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
// gateapp.default_environment existed (internal/le/le/action/leaction.go).
//
// The package streams output instead of capturing it. A suite runs for minutes,
// and a reader needs immediate output. Therefore, the child inherits this
// process's stdout and stderr. A buffer would hide the output until the suite ends.
package gaterun

import (
	"context"
	"errors"
	"fmt"
	"io"
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
	return StreamTee(argv, dir, environ, nil)
}

// StreamTee is Stream with a second reader of the child's stdout.
//
// The terminal still gets every byte as it arrives, so a person watching a
// long suite sees the same output. tally gets a copy, which is how a caller
// reads a count out of a child that answers only an exit code. A nil tally is
// Stream, byte for byte.
//
// The copy is a MultiWriter rather than a captured buffer on purpose: a suite
// prints megabytes over an hour, and holding that until the child exits would
// hide the output and grow without a bound.
func StreamTee(argv []string, dir string, environ []string, tally io.Writer) int {
	if len(argv) == 0 {
		Note("error: a gate declared no command to run")
		return CannotStart
	}

	// The code uses context.Background without a deadline. A gate with a wall-clock
	// cap includes the cap in its OWN argv. There, `timeout` can signal the whole
	// process group. A deadline here would kill the child but leave its
	// grandchildren alive. A ze daemon or a tacacs mock would then hold the output
	// pipe open. internal/le/test/functional runs every suite under `timeout` to prevent
	// this failure.
	//nolint:gosec // argv comes from an area's own table. le is a build-host tool controlled through a developer's argv
	cmd := exec.CommandContext(context.Background(), argv[0], argv[1:]...)
	cmd.Stdin = os.Stdin
	return finish(cmd, dir, environ, tally)
}

// StreamContext is Stream under ctx, for a verb whose root is cancelled by
// SIGINT or SIGTERM: a cancelled ctx kills the child's whole process group, so
// a `go test` child and every test binary it started stop at once and the
// verb answers the shell's code for the kill (137), never 0.
//
// The child heads a process group of its own (Setpgid), which is what lets the
// cancel reach the grandchildren that a plain CommandContext kill would leave
// holding the output pipe, the failure StreamTee's comment describes. The
// group is no longer the terminal's foreground group, so a Ctrl-C reaches le
// alone and le's signal root does the stop. The child gets no stdin for the
// same reason: a background group that reads the terminal is stopped by
// SIGTTIN and would wait for a cancel that never comes.
//
// It answers the group too: the cancel's SIGKILL reaches every member, but the
// return waits for the child alone, so a caller that needs the grandchildren
// gone MUST call ProcessGroup.AwaitExit after.
func StreamContext(ctx context.Context, argv []string, dir string, environ []string) (int, ProcessGroup) {
	if len(argv) == 0 {
		Note("error: a gate declared no command to run")
		return CannotStart, ProcessGroup{}
	}

	//nolint:gosec // argv comes from an area's own table. le is a build-host tool controlled through a developer's argv
	cmd := exec.CommandContext(ctx, argv[0], argv[1:]...)
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	cmd.Cancel = func() error { return syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL) }
	code := finish(cmd, dir, environ, nil)
	if cmd.Process == nil {
		return code, ProcessGroup{}
	}
	return code, ProcessGroup{leader: cmd.Process.Pid}
}

// finish gives cmd its directory, its whole environment and the terminal's
// output streams, runs it, and answers its exit code, or CannotStart for a
// child that never ran. The caller sets cmd.Stdin.
func finish(cmd *exec.Cmd, dir string, environ []string, tally io.Writer) int {
	cmd.Dir = dir
	cmd.Env = environ
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if tally != nil {
		cmd.Stdout = io.MultiWriter(os.Stdout, tally)
	}

	if err := cmd.Run(); err != nil {
		var exit *exec.ExitError
		if !errors.As(err, &exit) {
			var tb textbuf.Buffer
			Note(tb.Str("  cannot run ").Str(cmd.Args[0]).Str(": ").Err(err).String())
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
// A nil error is a child that exited cleanly, and ExitCode returns 0: callers
// pass the wait error straight in, and a success read as 1 fails every run.
// For an error that is not an exit, ExitCode returns 1. No child status exists
// in this case. The caller receives a failure status instead of a success status.
func ExitCode(err error) int {
	if err == nil {
		return 0
	}
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

// RunContext is Run with StreamContext: the command stops, process group and
// all, when ctx is canceled. It answers the group StreamContext does, for a
// caller that MUST see the killed members gone (ProcessGroup.AwaitExit).
func RunContext(ctx context.Context, action string, argv []string, dir string, environ []string) (ActionReport, int, ProcessGroup) {
	Announce(action)
	code, group := StreamContext(ctx, argv, dir, environ)
	return ActionReport{Action: action, Command: argv, Code: code}, code, group
}
