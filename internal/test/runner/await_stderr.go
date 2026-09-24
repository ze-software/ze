// Design: docs/architecture/testing/ci-format.md -- await=stderr deterministic fence
// Related: plugin_stage_stall.go -- the same derive-from-test-budget shape for the plugin stall watchdog

package runner

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os/exec"
	"slices"
	"strings"
	"syscall"
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

// parseAwait handles await=stderr:contains=TEXT[:timeout=DUR][:then=stop]
// lines. It makes the runner BLOCK until the daemon's relayed stderr contains
// TEXT before it tears the daemon down, so a test that observes a daemon line
// fences deterministically instead of sleeping. Only await=stderr is supported
// today. The TEXT needle follows the same rule as expect=stderr:contains=.
//
// Several await lines form ONE fence that holds once every needle has appeared,
// in any order. timeout= may be declared on one line only, because two budgets
// for one fence leave the reader to guess which applies. then=stop on any line
// makes the runner stop the daemon itself once the fence holds (see
// awaitThenStop); its only accepted value is "stop".
func (et *EncodingTests) parseAwait(r *Record, awaitType string, kv map[string]string) error {
	if awaitType != directiveTypeStderr {
		return fmt.Errorf("unknown await type %q (only await=stderr is supported)", awaitType)
	}
	contains := kv["contains"]
	if contains == "" {
		return errors.New("await=stderr:contains= must not be empty")
	}
	if slices.Contains(r.AwaitStderr, contains) {
		return fmt.Errorf("await=stderr:contains=%q is declared twice", contains)
	}
	if timeout := kv["timeout"]; timeout != "" {
		if r.AwaitStderrTimeout != "" {
			return errors.New("await=stderr:timeout= may be declared on one await line only")
		}
		if _, err := time.ParseDuration(timeout); err != nil {
			return fmt.Errorf("await=stderr:timeout=%q: %w", timeout, err)
		}
		r.AwaitStderrTimeout = timeout
	}
	if then, ok := kv["then"]; ok {
		if then != "stop" {
			return fmt.Errorf("await=stderr:then=%q (only then=stop is supported)", then)
		}
		r.AwaitThenStop = true
	}
	r.AwaitStderr = append(r.AwaitStderr, contains)
	return nil
}

// awaitSkipsReady reports whether the runner publishes daemon.pid without
// first waiting for daemon.ready. A plain await fence skips that wait: it serves
// the reject-fence bucket, where a plugin aborts startup and daemon.ready may
// never be written. A then=stop fence does NOT skip it: its daemon is expected
// to run, and a trigger that reads daemon.pid to send SIGHUP before the daemon
// has installed its handler kills the daemon instead of reloading it.
func (r *Record) awaitSkipsReady() bool {
	if len(r.AwaitStderr) == 0 {
		return false
	}
	return !r.AwaitThenStop
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

// awaitDaemonStderr blocks until the fence's syncWriter has seen every
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
	rec.Error = fmt.Errorf("await=stderr: daemon stderr never contained %q within %s", sw.missing(), timeout)
	rec.FailureType = stateTimeout
	for _, p := range bgProcs {
		if !peerProcs[p] && p.Process != nil {
			terminateGracefully(p)
		}
	}
	return false
}

// awaitThenStop is the then=stop form of the await fence. It returns the
// daemon's exit error and the check peers' joined exit errors, and false when
// the fence failed (rec.Error then says why).
//
// The sequence is fixed, and each step waits on an event, never on a clock:
//
//  1. every await needle appears on the daemon's stderr;
//  2. every LINGERING check peer has printed peerSuccessToken, or has exited;
//  3. the runner stops the daemon (SIGTERM, then a kill after
//     teardownGraceTimeout) and reaps it;
//  4. every check peer is reaped. A lingering peer ends when the daemon closes
//     its session, so it normally exits by itself here. One still running after
//     peerDrainGrace holds a socket nobody serves, so it is sent SIGTERM.
//
// Step 2 exists because a needle says nothing about the bytes a peer is still
// owed: a reload rollback prints its outcome line before the re-added session
// has carried its routes. A NON-lingering check peer is not waited there on
// purpose, because it ends itself, and its expectations may need the stop
// itself (the Cease a shutdown sends, reload-dynamic-peer-survives.ci).
//
// Steps 1 and 2 share one deadline, the fence budget awaitDaemonStderr uses.
// Without this form a lingering peer made a test stop its daemon with a
// fixed-delay SIGTERM, which under load landed while the reload under test was
// still verifying, so shutdown canceled it.
func (r *Runner) awaitThenStop(ctx context.Context, rec *Record, sw *syncWriter, fgProc *exec.Cmd, peers []peerOutput, testBudget time.Duration) (daemonErr, peerErr error, ok bool) {
	if fgProc == nil || fgProc.Process == nil {
		rec.Error = errors.New("await=stderr:then=stop: the test starts no daemon for the runner to stop")
		return nil, nil, false
	}
	timeout := r.withParallelHeadroom(rec.awaitStderrTimeout(testBudget))
	fenceCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	if !sw.waitFor(fenceCtx) {
		rec.Error = fmt.Errorf("await=stderr: daemon stderr never contained %q within %s", sw.missing(), timeout)
		rec.FailureType = stateTimeout
		terminateGracefully(fgProc)
		return nil, nil, false
	}

	reaped := reapCheckPeers(peers)
	if pending := waitLingerPeers(fenceCtx, peers, reaped); len(pending) != 0 {
		rec.Error = fmt.Errorf("await=stderr:then=stop: lingering check peer(s) %s never completed their expectations within %s", strings.Join(pending, ", "), timeout)
		rec.FailureType = stateTimeout
		terminateGracefully(fgProc)
		return nil, nil, false
	}

	daemonErr = stopAndReap(fgProc)
	return daemonErr, collectReapedPeers(peers, reaped, peerDrainGrace), true
}

// peerReap is one check peer's exit, published by the goroutine that waits it.
// err is written before done is closed, so a reader that saw done closed reads
// the final err.
type peerReap struct {
	done chan struct{}
	err  error
}

// reapCheckPeers starts one waiter per running check peer and returns them by
// peer index (nil for a peer it does not wait). One goroutine per process
// lifecycle: it ends when the process does, and collectReapedPeers bounds that
// by signaling. Scaffolding peers are left to terminateScaffoldPeers.
func reapCheckPeers(peers []peerOutput) []*peerReap {
	reaped := make([]*peerReap, len(peers))
	for i := range peers {
		if !peers[i].checkMode || peers[i].proc == nil || peers[i].waited {
			continue
		}
		pr := &peerReap{done: make(chan struct{})}
		reaped[i] = pr
		go func(proc *exec.Cmd) {
			pr.err = proc.Wait()
			close(pr.done)
		}(peers[i].proc)
	}
	return reaped
}

// waitLingerPeers blocks until every lingering check peer has announced its
// completion or exited, and returns the labels of those that did neither
// before ctx ended. A lingering peer prints peerSuccessToken the moment its
// last expectation is met (peer.completed, internal/test/peer/reject.go).
func waitLingerPeers(ctx context.Context, peers []peerOutput, reaped []*peerReap) []string {
	ticker := time.NewTicker(10 * time.Millisecond)
	defer ticker.Stop()
	for {
		var pending []string
		for i := range peers {
			if !peers[i].linger || reaped[i] == nil {
				continue
			}
			if lingerPeerSettled(&peers[i], reaped[i]) {
				continue
			}
			pending = append(pending, peers[i].label)
		}
		if len(pending) == 0 {
			return nil
		}
		select {
		case <-ctx.Done():
			return pending
		case <-ticker.C:
		}
	}
}

// lingerPeerSettled reports whether a lingering check peer has exited or has
// printed its success token.
func lingerPeerSettled(po *peerOutput, pr *peerReap) bool {
	select {
	case <-pr.done:
		return true
	default:
	}
	return strings.Contains(po.stdout.String(), peerSuccessToken)
}

// stopAndReap sends the daemon SIGTERM, kills it after teardownGraceTimeout,
// and returns its exit error, which an expect=exit:code= assertion reads.
func stopAndReap(cmd *exec.Cmd) error {
	_ = cmd.Process.Signal(syscall.SIGTERM) //nolint:errcheck // a process that already exited is reaped below
	timer := time.AfterFunc(teardownGraceTimeout, func() {
		_ = cmd.Process.Kill() //nolint:errcheck // the SIGTERM may already have ended it
	})
	defer timer.Stop()
	return cmd.Wait()
}

// collectReapedPeers waits every reaped check peer, bounded: a peer still
// running after grace is sent SIGTERM, and killed after teardownGraceTimeout.
// Each peer is marked waited so drainPeers does not Wait it a second time.
// Returns the peers' exit errors joined.
func collectReapedPeers(peers []peerOutput, reaped []*peerReap, grace time.Duration) error {
	deadline := time.NewTimer(grace)
	defer deadline.Stop()
	var errs []error
	expired := false
	for i, pr := range reaped {
		if pr == nil {
			continue
		}
		if !expired {
			select {
			case <-pr.done:
			case <-deadline.C:
				expired = true
			}
		}
		if expired {
			// The daemon is gone, so a peer still running holds a session
			// nobody serves. The grace is shared, so every later peer is past it too.
			endPeer(peers[i].proc, pr)
		}
		peers[i].waited = true
		if pr.err != nil {
			errs = append(errs, pr.err)
		}
	}
	return errors.Join(errs...)
}

// endPeer sends a still-running peer SIGTERM, kills it after
// teardownGraceTimeout, and returns once its waiter has reaped it.
func endPeer(proc *exec.Cmd, pr *peerReap) {
	select {
	case <-pr.done:
		return
	default:
	}
	_ = proc.Process.Signal(syscall.SIGTERM) //nolint:errcheck // it may have exited meanwhile; the waiter reaps it either way
	kill := time.AfterFunc(teardownGraceTimeout, func() {
		_ = proc.Process.Kill() //nolint:errcheck // SIGTERM may already have ended it
	})
	<-pr.done
	kill.Stop()
}
