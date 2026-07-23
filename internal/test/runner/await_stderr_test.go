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
		assert.Equal(t, awaitStderrDefaultTimeout, r.awaitStderrTimeout())
	})

	t.Run("contains and timeout", func(t *testing.T) {
		r := &Record{}
		require.NoError(t, et.parseLine(r, "x.ci", "await=stderr:contains=warned:timeout=3s"))
		assert.Equal(t, "warned", r.AwaitStderr)
		assert.Equal(t, 3*time.Second, r.awaitStderrTimeout())
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
	assert.Equal(t, awaitStderrDefaultTimeout, (&Record{}).awaitStderrTimeout())
	assert.Equal(t, awaitStderrDefaultTimeout, (&Record{AwaitStderrTimeout: "garbage"}).awaitStderrTimeout())
	assert.Equal(t, awaitStderrDefaultTimeout, (&Record{AwaitStderrTimeout: "0s"}).awaitStderrTimeout())
	assert.Equal(t, 2*time.Second, (&Record{AwaitStderrTimeout: "2s"}).awaitStderrTimeout())
}
