package reactor

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/bgp/message"
	bgpctx "github.com/ze-software/ze/internal/core/bgp/context"
)

// TestFlushFwdDirtyContendedReachesWire guards the deferred flush of the
// route-server fast path against a contended destination.
//
// Goal: an UPDATE the fast path wrote into a destination's bufWriter reaches
// the wire even when the source's batch-boundary flush finds the destination's
// writeMu held, and the source then sends nothing more. Before the fix the
// destination was kept for "the next flush cycle", which runs only when the
// source's reader drains another message, so a holder that released writeMu
// without flushing (a refused export, an early return) left the route unsent
// until shutdown. That was the FlowSpec redirect-check hang: the receiver never
// got the unicast route, and the daemon logged "forward batch flush failed ...
// broken pipe" at stop.
//
// Method: write a frame into the destination's bufWriter the way
// tryDirectWriteNoFlush does, mark it dirty on the source, hold the
// destination's writeMu across flushFwdDirty, release it without flushing, and
// read the destination's conn. No further source read or flushFwdDirty call
// happens, so only a flush the contended path arranged can deliver the frame.
//
// VALIDATES: a contended deferred flush is handed to the destination's forward
// worker, which waits for writeMu and flushes.
// PREVENTS: a fast-path UPDATE parked unflushed in a bufWriter indefinitely.
func TestFlushFwdDirtyContendedReachesWire(t *testing.T) {
	ctx := bgpctx.EncodingContextForASN4(true)
	ctxID, _ := bgpctx.Registry.Register(ctx)

	_, source, _ := makeRSPeerWithSession(t, "10.0.0.1", 65001, ctx, ctxID)
	target, targetSession, targetReader := makeRSPeerWithSession(t, "10.0.0.2", 65002, ctx, ctxID)

	pool := newFwdPool(fwdBatchHandler, fwdPoolConfig{chanSize: 8, idleTimeout: time.Second})
	t.Cleanup(pool.Stop)
	target.reactor = &Reactor{fwdPool: pool}

	body := []byte{0, 0, 0, 0}
	targetSession.writeMu.Lock()
	require.NoError(t, targetSession.writeRawUpdateBody(body))
	targetSession.writeMu.Unlock()
	source.appendFwdDirty(target, targetSession)

	// Another writer holds the destination: the batch-boundary flush cannot
	// take writeMu, and the holder then leaves without flushing.
	targetSession.writeMu.Lock()
	source.flushFwdDirty()
	targetSession.writeMu.Unlock()

	read := make(chan int, 1)
	go func() {
		frame := make([]byte, 64)
		n, _ := targetReader.Read(frame) //nolint:errcheck // a short read fails the length check below
		read <- n
	}()

	select {
	case n := <-read:
		require.Equal(t, message.HeaderLen+len(body), n)
	case <-time.After(2 * time.Second):
		t.Fatal("the fast-path UPDATE never reached the destination: its flush was left to a cycle that never ran")
	}
}
