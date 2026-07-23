// Design: docs/architecture/testing/ci-format.md -- await=stderr deterministic fence

package runner

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os/exec"
	"time"
)

// awaitStderrDefaultTimeout bounds an await=stderr fence when the test does not
// set its own :timeout=. Kept below the suite's per-test timeout (15s) so an
// await that never matches fails with a precise message rather than as an
// opaque test-level timeout.
const awaitStderrDefaultTimeout = 10 * time.Second

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

// awaitStderrTimeout resolves the effective fence timeout for a record. The
// value was validated at parse time; the error path here is defensive.
func (r *Record) awaitStderrTimeout() time.Duration {
	if r.AwaitStderrTimeout == "" {
		return awaitStderrDefaultTimeout
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
func awaitDaemonStderr(ctx context.Context, rec *Record, sw *syncWriter, bgProcs []*exec.Cmd, peerProcs map[*exec.Cmd]bool) bool {
	timeout := rec.awaitStderrTimeout()
	awaitCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	if sw.waitFor(awaitCtx) {
		return true
	}
	rec.Error = fmt.Errorf("await=stderr: daemon stderr never contained %q within %s", rec.AwaitStderr, timeout)
	rec.FailureType = "timeout"
	for _, p := range bgProcs {
		if !peerProcs[p] && p.Process != nil {
			terminateGracefully(p)
		}
	}
	return false
}
