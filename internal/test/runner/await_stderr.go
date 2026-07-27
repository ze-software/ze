// Design: docs/architecture/testing/ci-format.md -- await=stderr deterministic fence
// Related: plugin_stage_stall.go -- the same derive-from-test-budget shape for the plugin stall watchdog

package runner

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os/exec"
	"time"
)

// awaitStderrDefaultTimeout is the FLOOR for an await=stderr fence that sets no
// :timeout= of its own, used when the effective test budget is unknown or small.
const awaitStderrDefaultTimeout = 10 * time.Second

// awaitStderrBudgetShare is the fraction of the effective test budget an
// unqualified fence may consume. Below 1 on purpose: the fence exists so an
// await that never matches fails with a PRECISE message ("stderr never
// contained X") rather than as an opaque test-level timeout, which only works
// if it expires first.
const awaitStderrBudgetShare = 0.8

// defaultAwaitStderrTimeout derives the fence budget from the test budget the
// runner actually resolved, floored at awaitStderrDefaultTimeout.
//
// It used to be the bare 10s constant, whose comment read "kept below the
// suite's per-test timeout (15s)". That relationship was real but hardcoded, and
// the effective budget is computed at run time: a test may declare its own
// `option=timeout:` or a foreground `cmd=...:timeout=`, and the result is then
// multiplied by the parallel headroom. test/plugin/as112-external-refuses.ci
// declares a 15s command budget and waits on a refusal that costs a fork+exec of
// a second multi-megabyte binary plus a TLS connect-back; under load that
// exceeded the fixed 10s while the test's own 15s still had room, so the test
// failed inside a budget it had explicitly asked for. Deriving keeps the
// intended ordering at any budget instead of at one specific one.
//
// The floor is clamped BACK DOWN to the budget when the budget is smaller than
// it. Applying the floor unconditionally inverted the very ordering this
// function exists to preserve: a test declaring `timeout=5s` got a 10s fence, so
// the test-level timeout expired first and the fence's precise message was never
// the one reported -- the same failure as the old fixed constant, at the other
// end of the range.
func defaultAwaitStderrTimeout(testBudget time.Duration) time.Duration {
	if testBudget <= 0 {
		return awaitStderrDefaultTimeout
	}
	derived := max(time.Duration(float64(testBudget)*awaitStderrBudgetShare), awaitStderrDefaultTimeout)
	return min(derived, testBudget)
}

// awaitTypeStderr is the only supported await stream today.
const awaitTypeStderr = "stderr"

// parseAwait handles await=stderr:contains=TEXT[:timeout=DUR] lines. It makes
// the runner BLOCK until the daemon's relayed stderr contains TEXT before it
// tears the daemon down, so a test that observes an external plugin's
// refuse/warn message fences deterministically instead of sleeping. Only
// await=stderr is supported today. The TEXT needle follows the same rule as
// expect=stderr:contains= (kv-split on ':', so the needle must not contain a
// literal ':').
func (et *EncodingTests) parseAwait(r *Record, awaitType string, kv map[string]string) error {
	if awaitType != awaitTypeStderr {
		return fmt.Errorf("unknown await type %q (only await=stderr is supported)", awaitType)
	}
	contains := kv["contains"]
	if contains == "" {
		return errors.New("await=stderr:contains= must not be empty")
	}
	if r.AwaitStderr != "" {
		return errors.New("await=stderr may be specified only once per test")
	}
	if timeout := kv["timeout"]; timeout != "" {
		if _, err := time.ParseDuration(timeout); err != nil {
			return fmt.Errorf("await=stderr:timeout=%q: %w", timeout, err)
		}
		r.AwaitStderrTimeout = timeout
	}
	r.AwaitStderr = contains
	return nil
}

// awaitStderrTimeout resolves the effective fence timeout for a record, given
// the test budget the runner resolved for it. An explicit :timeout= on the
// await line always wins; otherwise the budget is derived. The value was
// validated at parse time; the error path here is defensive.
func (r *Record) awaitStderrTimeout(testBudget time.Duration) time.Duration {
	if r.AwaitStderrTimeout == "" {
		return defaultAwaitStderrTimeout(testBudget)
	}
	d, err := time.ParseDuration(r.AwaitStderrTimeout)
	if err != nil || d <= 0 {
		return awaitStderrDefaultTimeout
	}
	return d
}

// teeDaemonStderr returns the stderr writer for a started process. When an
// await=stderr fence is active (sw != nil) and the process is the ze daemon
// (isDaemon), it tees the relayed stderr through the fence's syncWriter as well
// as the accumulator, so the fence sees the daemon's output live. Every other
// case returns the accumulator unchanged, so tests without an await fence are
// byte-for-byte unaffected.
func teeDaemonStderr(acc io.Writer, sw *syncWriter, isDaemon bool) io.Writer {
	if sw != nil && isDaemon {
		return io.MultiWriter(acc, sw)
	}
	return acc
}

// awaitDaemonStderr blocks until the fence's syncWriter has seen the
// await=stderr needle, returning true. On timeout it records a precise failure
// on rec, gracefully stops the daemon processes (bgProcs that are not ze-peer),
// and returns false. Called only when rec.AwaitStderr != "".
func (r *Runner) awaitDaemonStderr(ctx context.Context, rec *Record, sw *syncWriter, bgProcs []*exec.Cmd, peerProcs map[*exec.Cmd]bool, testBudget time.Duration) bool {
	// Scale the fence by the parallel headroom (identity for serial runs): the
	// authored budget (default awaitStderrDefaultTimeout, or the test's :timeout=)
	// is measured unloaded, but a daemon slow to emit the awaited stderr line under
	// oversubscription must not trip this hard failure while the (also-widened)
	// outer test budget still has room.
	timeout := r.withParallelHeadroom(rec.awaitStderrTimeout(testBudget))
	awaitCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	if sw.waitFor(awaitCtx) {
		return true
	}
	rec.Error = fmt.Errorf("await=stderr: daemon stderr never contained %q within %s", rec.AwaitStderr, timeout)
	rec.FailureType = stateTimeout
	for _, p := range bgProcs {
		if !peerProcs[p] && p.Process != nil {
			terminateGracefully(p)
		}
	}
	return false
}
