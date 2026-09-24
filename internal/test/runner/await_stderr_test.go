package runner

import (
	"context"
	"io"
	"os/exec"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestParseAwaitStderr covers the await=stderr:contains= directive parse.
//
// VALIDATES: a well-formed await=stderr line records the needle (and optional
// timeout) on the Record; malformed lines are rejected at parse time rather
// than silently ignored (which would drop the fence and re-introduce a vacuous
// pass).
func TestParseAwaitStderr(t *testing.T) {
	et := NewEncodingTests(t.TempDir())

	t.Run("contains only", func(t *testing.T) {
		r := &Record{}
		require.NoError(t, et.parseLine(r, "x.ci", "await=stderr:contains=refusing to start"))
		assert.Equal(t, []string{"refusing to start"}, r.AwaitStderr)
		assert.False(t, r.AwaitThenStop, "a plain await keeps the peers in charge of the end")
		assert.Empty(t, r.AwaitStderrTimeout)
		assert.Equal(t, awaitStderrDefaultTimeout, r.awaitStderrTimeout(0))
	})

	t.Run("contains and timeout", func(t *testing.T) {
		r := &Record{}
		require.NoError(t, et.parseLine(r, "x.ci", "await=stderr:contains=warned:timeout=3s"))
		assert.Equal(t, []string{"warned"}, r.AwaitStderr)
		assert.Equal(t, 3*time.Second, r.awaitStderrTimeout(0))
		assert.Equal(t, 3*time.Second, r.awaitStderrTimeout(5*time.Minute),
			"an explicit :timeout= wins over any derived budget")
	})

	t.Run("empty contains rejected", func(t *testing.T) {
		r := &Record{}
		require.Error(t, et.parseLine(r, "x.ci", "await=stderr:contains="))
	})

	t.Run("unknown await type rejected", func(t *testing.T) {
		r := &Record{}
		require.Error(t, et.parseLine(r, "x.ci", "await=stdout:contains=x"))
	})

	t.Run("bad timeout rejected", func(t *testing.T) {
		r := &Record{}
		require.Error(t, et.parseLine(r, "x.ci", "await=stderr:contains=x:timeout=notaduration"))
	})

	t.Run("several awaits form one fence", func(t *testing.T) {
		r := &Record{}
		require.NoError(t, et.parseLine(r, "x.ci", "await=stderr:contains=first"))
		require.NoError(t, et.parseLine(r, "x.ci", "await=stderr:contains=OK: second: with colons"))
		assert.Equal(t, []string{"first", "OK: second: with colons"}, r.AwaitStderr)
	})

	t.Run("duplicate needle rejected", func(t *testing.T) {
		r := &Record{}
		require.NoError(t, et.parseLine(r, "x.ci", "await=stderr:contains=first"))
		require.Error(t, et.parseLine(r, "x.ci", "await=stderr:contains=first"))
	})

	t.Run("timeout on two lines rejected", func(t *testing.T) {
		r := &Record{}
		require.NoError(t, et.parseLine(r, "x.ci", "await=stderr:contains=first:timeout=3s"))
		require.Error(t, et.parseLine(r, "x.ci", "await=stderr:contains=second:timeout=4s"))
	})

	t.Run("then=stop recorded", func(t *testing.T) {
		r := &Record{}
		require.NoError(t, et.parseLine(r, "x.ci", "await=stderr:contains=sighup reload complete:then=stop"))
		assert.Equal(t, []string{"sighup reload complete"}, r.AwaitStderr)
		assert.True(t, r.AwaitThenStop)
	})

	t.Run("unknown then rejected", func(t *testing.T) {
		r := &Record{}
		require.Error(t, et.parseLine(r, "x.ci", "await=stderr:contains=x:then=wait"))
	})
}

// TestTeeDaemonStderr covers the stderr writer selection that guarantees the
// fence is additive.
//
// VALIDATES: with no fence (sw==nil) or a non-daemon process, the daemon's
// stderr writer is the plain accumulator unchanged (the "existing tests
// byte-for-byte unaffected" claim); with a fence on the ze daemon, output tees
// to BOTH the accumulator and the fence, and the fence fires even when the
// needle is split across writes (the streaming case the accumulating syncWriter
// exists for).
func TestTeeDaemonStderr(t *testing.T) {
	needle := "refusing to start"

	writeStr := func(t *testing.T, w io.Writer, s string) {
		t.Helper()
		_, err := w.Write([]byte(s))
		require.NoError(t, err)
	}

	t.Run("nil fence returns accumulator unchanged", func(t *testing.T) {
		var acc strings.Builder
		w := teeDaemonStderr(&acc, nil, true)
		writeStr(t, w, "hello "+needle+" world")
		assert.Equal(t, "hello "+needle+" world", acc.String())
	})

	t.Run("non-daemon process is not teed", func(t *testing.T) {
		var acc strings.Builder
		sw := newSyncWriterPattern(needle)
		w := teeDaemonStderr(&acc, sw, false)
		writeStr(t, w, "hello "+needle)
		assert.Equal(t, "hello "+needle, acc.String())
		assert.Empty(t, sw.String(), "a non-daemon process must not feed the fence")
	})

	t.Run("daemon with fence tees to both and fires across split writes", func(t *testing.T) {
		var acc strings.Builder
		sw := newSyncWriterPattern(needle)
		w := teeDaemonStderr(&acc, sw, true)
		// Split the needle across two writes to exercise the accumulating buffer.
		writeStr(t, w, "noise refusing to ")
		writeStr(t, w, "start here")
		assert.Equal(t, "noise refusing to start here", acc.String())
		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()
		assert.True(t, sw.waitFor(ctx), "the fence must fire once the needle appears, even split across writes")
	})
}

// TestSyncWriterWaitsForEveryNeedle covers the multi-needle fence.
//
// VALIDATES: a fence over several needles holds only once all of them have
// appeared, in any order, and names the missing ones until then.
// PREVENTS: a then=stop fence stopping the daemon on the reload outcome line
// while an observer plugin's relayed line, which the test also asserts, is
// still in flight.
func TestSyncWriterWaitsForEveryNeedle(t *testing.T) {
	sw := newSyncWriterPattern("alpha", "beta")
	_, err := sw.Write([]byte("beta arrives first\n"))
	require.NoError(t, err)
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()
	assert.False(t, sw.waitFor(ctx), "one needle of two must not hold the fence")
	assert.Equal(t, []string{"alpha"}, sw.missing())

	_, err = sw.Write([]byte("then alpha\n"))
	require.NoError(t, err)
	ctx2, cancel2 := context.WithTimeout(context.Background(), time.Second)
	defer cancel2()
	assert.True(t, sw.waitFor(ctx2))
	assert.Empty(t, sw.missing())
}

// TestAwaitSkipsReady covers which fences publish daemon.pid before
// daemon.ready.
//
// VALIDATES: only a plain await fence skips the readiness wait.
// PREVENTS: a then=stop test with no peer publishing daemon.pid at once, so its
// trigger sent SIGHUP before the daemon installed the handler and killed it
// (reload-plugin-only-no-change.ci failed its fence 3 runs out of 3).
func TestAwaitSkipsReady(t *testing.T) {
	assert.False(t, (&Record{}).awaitSkipsReady(), "no fence: the readiness wait applies")
	assert.True(t, (&Record{AwaitStderr: []string{"refusing"}}).awaitSkipsReady(), "reject fence: the daemon may never be ready")
	assert.False(t, (&Record{AwaitStderr: []string{"sighup reload complete"}, AwaitThenStop: true}).awaitSkipsReady(), "then=stop: the daemon must be ready before it is signaled")
}

// TestPeerLingers covers the linger detection the then=stop fence relies on.
//
// VALIDATES: the runner reads option=linger with the peer's own parser.
func TestPeerLingers(t *testing.T) {
	assert.True(t, peerLingers([]byte("option=linger:value=true\nexpect=bgp:conn=1:seq=1:hex=00\n")))
	assert.False(t, peerLingers([]byte("expect=bgp:conn=1:seq=1:hex=00\n")))
	assert.False(t, peerLingers([]byte("option=linger:value=false\n")))
	assert.False(t, peerLingers(nil))
}

// thenStopFixture starts a stand-in daemon, which runs until SIGTERM and then
// exits 0, and one lingering check peer running peerScript, in which DAEMON is
// replaced by the daemon's pid.
func thenStopFixture(t *testing.T, peerScript string) (*exec.Cmd, []peerOutput) {
	t.Helper()
	daemon := exec.CommandContext(t.Context(), "sh", "-c", "trap 'exit 0' TERM; echo ready; while :; do sleep 0.05; done")
	daemonOut := newSyncWriterPattern("ready")
	daemon.Stdout = daemonOut
	require.NoError(t, daemon.Start())
	// A SIGTERM sent before the trap is installed kills the shell instead of
	// ending it with 0, so the fixture waits for the line printed after it.
	require.True(t, daemonOut.waitFor(t.Context()))
	po := peerOutput{stdout: newSyncWriterPattern(peerSuccessToken), stderr: &lockedBuilder{}, checkMode: true, linger: true, label: "stdin=peer"}
	po.proc = exec.CommandContext(t.Context(), "sh", "-c", strings.ReplaceAll(peerScript, "DAEMON", strconv.Itoa(daemon.Process.Pid)))
	po.proc.Stdout = po.stdout
	require.NoError(t, po.proc.Start())
	return daemon, []peerOutput{po}
}

// peerEndsWithDaemon announces success, then holds until the daemon is gone,
// which is how a lingering ze-peer ends: the daemon closes its session.
const peerEndsWithDaemon = "echo successful; while kill -0 DAEMON 2>/dev/null; do sleep 0.05; done"

// TestAwaitThenStop drives the then=stop fence over real processes.
//
// VALIDATES: once the needle appears and the lingering check peer has printed
// its success token, the runner stops the daemon ITSELF and reaps the peer,
// which ends when the daemon does, and the daemon's exit reaches the caller. A
// lingering peer that never completes, and a needle that never appears, each
// fail the fence with a precise message instead of hanging.
// PREVENTS: the fixed-delay SIGTERM reload tests used to stop a daemon whose
// lingering peer never ends: under load it landed while the reload under test
// was still verifying, and shutdown canceled it.
func TestAwaitThenStop(t *testing.T) {
	r := &Runner{concurrency: 1}
	fence := func(t *testing.T, text string) *syncWriter {
		t.Helper()
		sw := newSyncWriterPattern("sighup reload complete")
		_, err := sw.Write([]byte(text))
		require.NoError(t, err)
		return sw
	}

	t.Run("stops the daemon once the needle and the lingering peer hold", func(t *testing.T) {
		daemon, peers := thenStopFixture(t, peerEndsWithDaemon)
		rec := &Record{AwaitStderr: []string{"sighup reload complete"}, AwaitThenStop: true}

		daemonErr, peerErr, ok := r.awaitThenStop(t.Context(), rec, fence(t, "sighup reload complete\n"), daemon, peers, 10*time.Second)
		require.True(t, ok, "fence failed: %v", rec.Error)
		require.NoError(t, daemonErr, "the daemon traps SIGTERM and exits 0")
		require.NoError(t, peerErr)
		require.NotNil(t, daemon.ProcessState, "the fence must reap the daemon it stopped")
		assert.True(t, peers[0].waited, "a reaped peer is marked so drainPeers does not Wait it again")
	})

	t.Run("a lingering peer that never completes fails the fence", func(t *testing.T) {
		daemon, peers := thenStopFixture(t, "while kill -0 DAEMON 2>/dev/null; do sleep 0.05; done")
		rec := &Record{AwaitStderr: []string{"sighup reload complete"}, AwaitStderrTimeout: "300ms", AwaitThenStop: true}

		_, _, ok := r.awaitThenStop(t.Context(), rec, fence(t, "sighup reload complete\n"), daemon, peers, 10*time.Second)
		require.False(t, ok)
		require.ErrorContains(t, rec.Error, "stdin=peer never completed")
		require.NotNil(t, daemon.ProcessState, "a failed fence still stops the daemon")
	})

	t.Run("a needle that never appears fails the fence", func(t *testing.T) {
		daemon, peers := thenStopFixture(t, peerEndsWithDaemon)
		rec := &Record{AwaitStderr: []string{"sighup reload complete"}, AwaitStderrTimeout: "200ms", AwaitThenStop: true}

		_, _, ok := r.awaitThenStop(t.Context(), rec, fence(t, "reloading config\n"), daemon, peers, 10*time.Second)
		require.False(t, ok)
		require.ErrorContains(t, rec.Error, `never contained ["sighup reload complete"]`)
		require.NotNil(t, daemon.ProcessState, "a failed fence still stops the daemon")
		_ = peers[0].proc.Wait() //nolint:errcheck // the daemon is gone, so the peer script ends
	})

	t.Run("a peer that outlives the daemon is torn down after its grace", func(t *testing.T) {
		daemon, peers := thenStopFixture(t, "trap 'exit 0' TERM; echo successful; while :; do sleep 0.05; done")
		require.True(t, peers[0].stdout.waitFor(t.Context()), "the peer installs its trap before it prints")
		require.NoError(t, stopAndReap(daemon))

		reaped := reapCheckPeers(peers)
		require.NoError(t, collectReapedPeers(peers, reaped, 100*time.Millisecond), "the peer traps SIGTERM and exits 0")
		assert.True(t, peers[0].waited)
	})
}

// TestAwaitStderrTimeoutDefaultsOnGarbage verifies the resolver falls back to
// the default for an empty or unparseable stored value (defensive: parse-time
// validation should prevent the latter, but the fence timeout must never be
// zero, which would make WaitFor return immediately and fence on nothing).
func TestAwaitStderrTimeoutDefaultsOnGarbage(t *testing.T) {
	assert.Equal(t, awaitStderrDefaultTimeout, (&Record{}).awaitStderrTimeout(0))
	assert.Equal(t, awaitStderrDefaultTimeout, (&Record{AwaitStderrTimeout: "garbage"}).awaitStderrTimeout(0))
	assert.Equal(t, awaitStderrDefaultTimeout, (&Record{AwaitStderrTimeout: "0s"}).awaitStderrTimeout(0))
	assert.Equal(t, 2*time.Second, (&Record{AwaitStderrTimeout: "2s"}).awaitStderrTimeout(0))
}

// VALIDATES: an unqualified fence takes its budget from the test budget the
// runner resolved and never EXCEEDS it, so the fence's precise message is always
// the one reported; above the floor it also stays strictly below the budget.
// PREVENTS: the failure that made test/plugin/as112-external-refuses.ci flaky --
// the fence was a fixed 10s while the test declared a 15s command budget, so
// under load the fence expired inside a budget the test had explicitly asked
// for, and reported it as "server likely failed to start or crashed".
func TestDefaultAwaitStderrTimeoutDerivesFromTestBudget(t *testing.T) {
	for name, tc := range map[string]struct {
		budget time.Duration
		want   time.Duration
	}{
		"unknown budget falls back to the floor": {0, awaitStderrDefaultTimeout},
		"negative budget falls back":             {-1 * time.Second, awaitStderrDefaultTimeout},
		"budget at the floor stays at the floor": {10 * time.Second, awaitStderrDefaultTimeout},
		// Clamped DOWN to the budget, not up to the floor: a fence longer than
		// the test budget can never report first, which defeats its purpose.
		"budget below the floor clamps to it": {2 * time.Second, 2 * time.Second},
		"budget just under the floor clamps":  {9 * time.Second, 9 * time.Second},
		"15s command budget yields 12s":       {15 * time.Second, 12 * time.Second},
		"60s budget yields 48s":               {60 * time.Second, 48 * time.Second},
	} {
		t.Run(name, func(t *testing.T) {
			got := defaultAwaitStderrTimeout(tc.budget)
			assert.Equal(t, tc.want, got)
			if tc.budget > 0 {
				assert.LessOrEqual(t, got, tc.budget,
					"the fence must never outlive the test budget, or its precise message is unreachable")
			}
			if tc.budget > awaitStderrDefaultTimeout {
				assert.Less(t, got, tc.budget,
					"above the floor the fence must expire strictly first")
			}
		})
	}
}
