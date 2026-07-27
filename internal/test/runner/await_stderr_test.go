package runner

import (
	"context"
	"io"
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
		assert.Equal(t, "refusing to start", r.AwaitStderr)
		assert.Empty(t, r.AwaitStderrTimeout)
		assert.Equal(t, awaitStderrDefaultTimeout, r.awaitStderrTimeout(0))
	})

	t.Run("contains and timeout", func(t *testing.T) {
		r := &Record{}
		require.NoError(t, et.parseLine(r, "x.ci", "await=stderr:contains=warned:timeout=3s"))
		assert.Equal(t, "warned", r.AwaitStderr)
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

	t.Run("duplicate await rejected", func(t *testing.T) {
		r := &Record{}
		require.NoError(t, et.parseLine(r, "x.ci", "await=stderr:contains=first"))
		require.Error(t, et.parseLine(r, "x.ci", "await=stderr:contains=second"))
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
