package runner

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestAwaitBackgroundReady proves the ready= barrier holds until its text
// appears and fails with a named reason when the budget ends first.
//
// VALIDATES: the wait returns true once a write carries the text, even when the
// text arrives after the wait began; on an expired context it records
// FailTypeBackgroundNeverReady and names the command, the text and the output.
// PREVENTS: a barrier that returns before the text, or a timeout that reads as a
// later protocol stall instead of naming the process that never became ready.
func TestAwaitBackgroundReady(t *testing.T) {
	cmd := &RunCommand{Mode: modeBackground, Seq: 1, Exec: "le test fixture x", Ready: "COLLECTOR: listening"}

	t.Run("text_arrives_late", func(t *testing.T) {
		sw := newSyncWriterPattern(cmd.Ready)
		_, err := sw.Write([]byte("starting\n"))
		require.NoError(t, err)
		written := make(chan struct{})
		go func() {
			defer close(written)
			<-time.After(50 * time.Millisecond)
			_, _ = sw.Write([]byte("COLLECTOR: listening on 1790\n"))
		}()
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		rec := &Record{}
		assert.True(t, awaitBackgroundReady(ctx, rec, cmd, sw))
		assert.NoError(t, rec.Error)
		<-written
	})

	t.Run("budget_ends_first", func(t *testing.T) {
		sw := newSyncWriterPattern(cmd.Ready)
		_, err := sw.Write([]byte("bind: address already in use\n"))
		require.NoError(t, err)
		ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
		defer cancel()
		rec := &Record{}
		assert.False(t, awaitBackgroundReady(ctx, rec, cmd, sw))
		assert.Equal(t, FailTypeBackgroundNeverReady, rec.FailureType)
		require.Error(t, rec.Error)
		assert.Contains(t, rec.Error.Error(), `ready="COLLECTOR: listening"`)
		assert.Contains(t, rec.Error.Error(), "address already in use")
		assert.Contains(t, rec.Error.Error(), "le test fixture x")
	})
}
