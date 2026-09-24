// Design: docs/architecture/testing/ci-format.md -- cmd=background ready= barrier
// Related: runner_exec.go -- runOrchestrated starts the process and tees its output here
// Related: record_parse_cmd.go -- parseCmdExec reads ready=

package runner

import (
	"context"
	"fmt"
)

// awaitBackgroundReady blocks until a cmd=background process has printed its
// ready= text, and reports whether it did.
//
// The wait is bounded by ctx, the test's own context, so the barrier can never
// outlive the budget the runner kills the test at. It has no deadline of its
// own on purpose: a fixed one is either shorter than a slow start on a loaded
// host, which turns a harness delay into a failure, or longer than the test,
// which is a branch that cannot run.
//
// On failure it records FailTypeBackgroundNeverReady on rec and names the
// command, the text it waited for, and everything the process printed, so the
// operator reads why the step never started rather than a later protocol stall.
func awaitBackgroundReady(ctx context.Context, rec *Record, cmd *RunCommand, sw *syncWriter) bool {
	if sw.waitFor(ctx) {
		return true
	}
	rec.Error = fmt.Errorf("cmd seq=%d (%s): background process never printed ready=%q before the test budget ended (output=%q)",
		cmd.Seq, cmd.Exec, cmd.Ready, sw.String())
	rec.FailureType = FailTypeBackgroundNeverReady
	return false
}
