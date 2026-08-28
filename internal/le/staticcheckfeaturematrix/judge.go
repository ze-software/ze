// Design: docs/architecture/testing/tracked-build-gate.md -- running Staticcheck over the matrix
//
// judge.go is the half of this gate that leaves the process: it hands the
// derived matrix to standalone Staticcheck on stdin and reads its verdict.
//
// Every way the run can end WITHOUT a verdict answers 2, apart from the tree
// failing to type-check, which answers 1. Staticcheck missing, Staticcheck
// exceeding the deadline, Staticcheck matching no package, and Staticcheck
// exiting a code that is not 1 are each "the matrix could not be judged", which
// a caller must be able to tell apart from "the matrix was judged and it
// failed".

package staticcheckfeaturematrix

import (
	"bytes"
	"context"
	"errors"
	"os/exec"
	"strings"
	"syscall"
	"time"

	"github.com/ze-software/ze/internal/core/env"
	"github.com/ze-software/ze/internal/core/textbuf"
)

// DeadlineKey is the dot-notation spelling of ZE_STATICCHECK_DEADLINE. It
// replaces the script's --deadline=D flag, which no caller passed: the tree is
// the checkout and the rendering is a pipe operator, so this command takes a
// keyword and no value of its own.
const DeadlineKey = "ze.staticcheck.deadline"

// defaultDeadline bounds one Staticcheck run over the whole matrix. A run past
// it is a hung process rather than a slow one.
const defaultDeadline = 25 * time.Minute

var deadlineEntry = env.MustRegister(env.EnvEntry{
	Key:         DeadlineKey,
	Type:        "duration",
	Default:     defaultDeadline.String(),
	Description: "how long one Staticcheck run over the feature matrix may take before the matrix is declared unjudgeable",
	// Private keeps the key out of `ze env list`. It bounds a build-host tool
	// and an operator has nothing to do with it.
	Private: true,
})

// Deadline answers the bound one Staticcheck run may take, and the reason a
// declared value could not be used.
func Deadline() (time.Duration, error) { return DeadlineFrom(env.Get(deadlineEntry.Key)) }

// DeadlineFrom is the parse, apart from where the value came from.
//
// It is exported for the same reason DeriveScoped is: env.Get caches the whole
// environment on its first call, so a test that sets the variable afterwards
// would be reading a value that is no longer there.
func DeadlineFrom(declared string) (time.Duration, error) {
	raw := strings.TrimSpace(declared)
	if raw == "" {
		return defaultDeadline, nil
	}
	parsed, err := time.ParseDuration(raw)
	if err != nil || parsed <= 0 {
		return 0, &badDeadlineError{value: raw}
	}
	return parsed, nil
}

// badDeadlineError is a declared deadline the run cannot use. It is a type
// rather than a formatted string so the caller can answer 2 for it without
// matching on message text.
type badDeadlineError struct{ value string }

// Error states the declared value and what was wanted.
func (e *badDeadlineError) Error() string {
	var tb textbuf.Buffer
	return tb.Str(DeadlineKey).Str(" needs a positive duration, got ").
		Quoted(e.value).Str("; matrix could not be judged").String()
}

// Judge runs Staticcheck over the rendered matrix in tree and answers what it
// said.
//
// The bool is whether the matrix was JUDGED at all. False means the answer is
// exit code 2 and the error says why; it is not a type-check failure.
func Judge(tree string, matrix Matrix, deadline time.Duration) (Verdict, bool, error) {
	rendered, err := matrix.Render()
	if err != nil {
		return Verdict{}, false, err
	}

	ctx, cancel := context.WithTimeout(context.Background(), deadline)
	defer cancel()

	cmd := exec.CommandContext(ctx, "staticcheck", "-checks=-all", "-matrix", "./...")
	cmd.Dir = tree
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	cmd.Cancel = func() error {
		if cmd.Process == nil {
			return nil
		}
		return syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL)
	}
	cmd.Stdin = bytes.NewReader(rendered)

	var toolStdout, toolStderr bytes.Buffer
	cmd.Stdout = &toolStdout
	cmd.Stderr = &toolStderr

	if err := cmd.Start(); err != nil {
		return Verdict{}, false, startFailure(ctx, err, deadline)
	}
	runErr := cmd.Wait()

	verdict := Verdict{
		Rows:        len(matrix),
		Diagnostics: splitLines(toolStdout.String()),
		Tool:        toolStderr.String(),
	}

	// Staticcheck reports an empty selection on its own streams and exits 0,
	// which reads as a clean tree and is the emptiest possible false green.
	if strings.Contains(toolStdout.String(), "matched no packages") ||
		strings.Contains(toolStderr.String(), "matched no packages") {
		return verdict, false, errors.New(
			"staticcheck matched no packages; matrix could not be judged; run from a Go module containing the selected packages")
	}

	if runErr == nil {
		verdict.Passed = true
		return verdict, true, nil
	}

	var exitErr *exec.ExitError
	if errors.As(runErr, &exitErr) && exitErr.ExitCode() == 1 {
		return verdict, true, nil
	}
	if errors.Is(ctx.Err(), context.DeadlineExceeded) {
		var tb textbuf.Buffer
		return verdict, false, errors.New(tb.Str("Staticcheck exceeded the ").Str(deadline.String()).
			Str(" deadline; matrix could not be judged, so retry after resolving the timeout").String())
	}
	if errors.As(runErr, &exitErr) {
		var tb textbuf.Buffer
		return verdict, false, errors.New(tb.Str("Staticcheck exited ").Int(int64(exitErr.ExitCode())).
			Str(" without a type-check verdict; verify that standalone Staticcheck supports -checks=-all and -matrix; matrix could not be judged").String())
	}
	var tb textbuf.Buffer
	return verdict, false, errors.New(tb.Str("Staticcheck did not complete: ").Err(runErr).
		Str("; matrix could not be judged").String())
}

// startFailure states why Staticcheck never ran, in the three forms a reader
// can act on: fix the clock, install the tool, or fix the executable.
func startFailure(ctx context.Context, err error, deadline time.Duration) error {
	switch {
	case errors.Is(ctx.Err(), context.DeadlineExceeded):
		var tb textbuf.Buffer
		return errors.New(tb.Str("Staticcheck could not start before the ").Str(deadline.String()).
			Str(" deadline; matrix could not be judged").String())
	case errors.Is(err, exec.ErrNotFound):
		return errors.New(
			"staticcheck was not found on PATH; install standalone Staticcheck, ensure it is on PATH, and retry; matrix could not be judged")
	default:
		var tb textbuf.Buffer
		return errors.New(tb.Str("could not start staticcheck: ").Err(err).
			Str("; verify the executable and PATH, then retry; matrix could not be judged").String())
	}
}

// splitLines answers one entry per non-empty line of a tool's output.
func splitLines(text string) []string {
	var out []string
	for line := range strings.SplitSeq(text, "\n") {
		if strings.TrimSpace(line) == "" {
			continue
		}
		out = append(out, line)
	}
	return out
}
