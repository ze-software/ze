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

// deadlinePerRow bounds ONE row of the matrix. A run past its bound is a hung
// process rather than a slow one.
//
// The bound is per row because a run's SIZE is a choice: `check` on its own
// judges every row, a verify run judges the rows its change set can move, and a
// CI piece judges the rows dealt to it. One flat bound over all three either
// strangles the widest run or stops detecting a hang in the narrowest. The flat
// 25 minutes this replaces was set on 2026-08-17 against a matrix that reached
// 38 rows on 2026-07-24, and CI run 33450825487 measured 23m36s inside it.
//
// 90s is about two and a half times the widest cost one row has been measured
// at: 23m36s over 38 rows is 37.3s a row in CI, and a cold local run of
// 1142.63s over the same rows is 30.1s a row. That leaves a shared runner room
// to be slow and still calls a stuck Staticcheck stuck.
const deadlinePerRow = 90 * time.Second

var deadlineEntry = env.MustRegister(env.EnvEntry{
	Key:  DeadlineKey,
	Type: "duration",
	// No default value: an unset key derives the bound from the number of rows
	// the run judges, which a fixed string cannot state.
	Default:     "",
	Description: "an absolute bound on one Staticcheck run over the feature matrix; unset allows 90s per judged row before the matrix is declared unjudgeable",
	// Private keeps the key out of `ze env list`. It bounds a build-host tool
	// and an operator has nothing to do with it.
	Private: true,
})

// Deadline answers the bound one Staticcheck run over rows may take, and the
// reason a declared value could not be used.
func Deadline(rows int) (time.Duration, error) {
	return DeadlineFrom(env.Get(deadlineEntry.Key), rows)
}

// DeadlineFrom is the parse, apart from where the value came from.
//
// It is exported for the same reason DeriveScoped is: env.Get caches the whole
// environment on its first call, so a test that sets the variable afterwards
// would be reading a value that is no longer there.
//
// A declared value is ABSOLUTE and bounds the run whatever its size, because a
// person who names a duration is bounding the wall clock they are waiting on.
func DeadlineFrom(declared string, rows int) (time.Duration, error) {
	if rows < 1 {
		var tb textbuf.Buffer
		return 0, errors.New(tb.Str("a deadline needs at least one row to judge, got ").
			Int(int64(rows)).Str("; matrix could not be judged").String())
	}
	raw := strings.TrimSpace(declared)
	if raw == "" {
		return time.Duration(rows) * deadlinePerRow, nil
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
		Names:       matrix.Names(),
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
