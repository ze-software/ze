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
// gateapp.default_environment existed (scripts/le/gateapp.py).
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
// own number for it, and scripts/le/process.py answers the same, so a caller
// that reads gate codes apart keeps reading them apart.
const CannotStart = 127

// Announce writes the heading that a gate prints before execution.
//
// Announce writes the heading to stderr, as scripts/le/devtools/gate.py did.
// The command's ANSWER is a payload on stdout. The operator can pipe this payload.
// A progress line on stdout would become part of the document. Each ported
// evidence tool also uses stderr for its own log
// (plan/spec-le-is-a-ze-binary.md, R-7).
func Announce(gate string) {
	var tb textbuf.Buffer
	fmt.Fprintln(os.Stderr, tb.Str("==> ").Str(gate).String()) //nolint:errcheck // CLI output
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
// makes Toolchain.Environment the single definition of the environment for a
// gate.
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
// as a status. `commit_helper.py` distinguishes 3 from 1, and a negative number
// matches neither status. A shell reports a signaled child as 128 plus the
// signal. ExitCode uses the same form.
//
// ExitCode is exported because internal/le/lejob starts its own child.
// internal/le/lejob tees that child's output to a job log. Both child execution
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

// GateReport contains the result of one forked gate. It identifies the gate,
// command, and exit code.
//
// GateReport is the payload when another program performs the gate's work. The
// reader already received the output directly from the terminal. The report
// records which gate ran, what it ran, and what it decided (ai/rules/cli.md).
type GateReport struct {
	Gate    string   `json:"gate"`
	Command []string `json:"command"`
	Code    int      `json:"code"`
}

// Run announces one gate, runs its command, and returns the report and command
// exit code.
//
// Run preserves the command's OWN code instead of converting it to 1. The
// discovery-index check uses 0 for fresh and 3 for stale. It uses 1 when the
// generator fails. scripts/dev/commit_helper.py blocks on 3 and keeps 1 as
// warn-only.
func Run(gate string, argv []string, dir string, environ []string) (GateReport, int) {
	Announce(gate)
	code := Stream(argv, dir, environ)
	return GateReport{Gate: gate, Command: argv, Code: code}, code
}
