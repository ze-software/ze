// Design: docs/architecture/forward-congestion-pool.md -- channel items and the ordering gates
// Related: peer.go -- forwardChannelPending, withdrawBehindForwards
// Related: forward_rs.go -- reactorForwardRS direct-write gate
// Related: forward_pool.go -- TryDispatch, safeBatchHandle

package reactor

import (
	"net/netip"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/plugin"
	"github.com/ze-software/ze/internal/core/selector"
)

// newChannelOrderRail is newSyncOrderRailWith with the destination out of its
// initial sync and a batch handler that stops the worker on entry to the first
// batch carrying a real item, BEFORE fwdBatchHandler takes session.writeMu.
// entered closes when the worker is stopped there, and release lets it go on.
func newChannelOrderRail(t *testing.T) (r *Reactor, src, dst *Peer, conn *recordingConn, publish func(uint64, []byte) *ReceivedUpdate, entered <-chan struct{}, release func()) {
	t.Helper()

	enteredCh := make(chan struct{})
	gate := make(chan struct{})
	var enterOnce, releaseOnce sync.Once
	r, src, dst, conn, ctxID := newSyncOrderRailWith(t, func(key fwdKey, items []fwdItem) {
		if fwdBatchHasRealItem(items) {
			enterOnce.Do(func() {
				close(enteredCh)
				<-gate
			})
		}
		fwdBatchHandler(key, items)
	})
	// Registered AFTER the rail, so it runs BEFORE the pool's own Stop: a failed
	// assertion would otherwise leave Stop waiting on a worker blocked in the
	// handler, and the test would hang instead of reporting.
	release = func() { releaseOnce.Do(func() { close(gate) }) }
	t.Cleanup(release)

	dst.sendingInitialRoutes.Store(0)
	require.False(t, dst.forwardOrderHold(false), "the destination must be out of its sync hold")

	publish = func(id uint64, body []byte) *ReceivedUpdate { return syncOrderPublish(t, r, ctxID, id, body) }
	return r, src, dst, conn, publish, enteredCh, release
}

// awaitEntered waits for the destination's worker to stop inside the batch
// that carries the first forwarded item.
func awaitEntered(t *testing.T, entered <-chan struct{}) {
	t.Helper()
	select {
	case <-entered:
	case <-time.After(5 * time.Second):
		t.Fatal("the forwarded item never reached the destination's batch handler")
	}
}

// TestForwardedUpdateWaitsForChannelItemRSRail proves that the route-server
// rail's direct write never overtakes a forwarded item that reached the
// destination's worker through its CHANNEL.
//
// VALIDATES: Peer.forwardChannelPending counts an item TryDispatch put on the
// channel until the worker has written it, and reactorForwardRS skips the
// direct write while the count is nonzero.
// PREVENTS: the forwarded-UPDATE half of the 2026-08-11 journal row. The first
// UPDATE falls back to TryDispatch because session.writeMu was taken; the next
// UPDATE for the same prefix read overflowPending as zero, took writeMu with
// TryLock before the worker did, and the peer ended holding a withdrawn prefix.
//
// Method: the test holds the destination's writeMu while the announce is
// forwarded, so the announce goes to the channel. The worker is stopped before
// it takes writeMu, the test releases writeMu, and the withdraw is forwarded in
// that window.
func TestForwardedUpdateWaitsForChannelItemRSRail(t *testing.T) {
	const announceID, withdrawID uint64 = 7600, 7601

	r, src, dst, conn, publish, entered, release := newChannelOrderRail(t)
	srcAddr := netip.MustParseAddr(forwardSourceAddr)

	dst.mu.RLock()
	session := dst.session
	dst.mu.RUnlock()
	require.NotNil(t, session, "the destination must hold a session")

	session.writeMu.Lock()
	_, dispatched := reactorForwardRS(r, publish(announceID, syncOrderAnnounceBody), announceID, srcAddr, src)
	session.writeMu.Unlock()
	require.Equal(t, 1, dispatched, "the announce must be dispatched, not dropped")
	require.False(t, dst.forwardOverflowPending(), "the announce must travel through the channel, not overflow")

	awaitEntered(t, entered)
	require.True(t, dst.forwardChannelPending(), "the announce is on its way through the channel and not written")

	_, dispatched = reactorForwardRS(r, publish(withdrawID, syncOrderWithdrawBody), withdrawID, srcAddr, src)
	require.Equal(t, 1, dispatched, "the withdraw must be dispatched, not dropped")

	release()

	var seen []wireUpdate
	require.Eventually(t, func() bool {
		seen = parseWireUpdates(t, conn.written())
		return len(seen) >= 2
	}, 5*time.Second, 5*time.Millisecond, "both forwarded UPDATEs must reach the peer")
	require.Equal(t, []string{"announce", "withdraw"}, wireKinds(seen), "wire order")
	require.Eventually(t, func() bool {
		return !dst.forwardChannelPending()
	}, 5*time.Second, 5*time.Millisecond, "the count must return to zero once the worker has written")
}

// TestAnnounceRailWithdrawFollowsChannelForwards proves that an announce-rail
// withdrawal waits for a forwarded announce of the same prefix that is still on
// the destination worker's channel.
//
// VALIDATES: Peer.withdrawBehindForwards answers true while a channel item is
// owed (forwardChannelPending), so the withdrawal joins the forward queue
// behind it rather than reaching the wire at once.
// PREVENTS: the withdrawal being written ahead of the queued announce, or being
// withheld as never advertised, which left the peer holding a route whose
// source withdrew it.
func TestAnnounceRailWithdrawFollowsChannelForwards(t *testing.T) {
	r, src, dst, conn, publish, entered, release := newChannelOrderRail(t)
	adapter := &reactorAPIAdapter{r: r}
	dst.resetAPISync(nil)

	require.NoError(t, adapter.forwardUpdateCore(publish(7700, syncOrderAnnounceBody), 7700, []*Peer{dst}, replayFenceSource(src, false)))
	require.False(t, dst.forwardOverflowPending(), "the announce must travel through the channel, not overflow")

	awaitEntered(t, entered)
	require.True(t, dst.withdrawBehindForwards(), "a channel item is owed, so the withdrawal must queue behind it")

	require.NoError(t, adapter.WithdrawNLRIBatch(t.Context(), selector.Addr(dst.Settings().Address),
		adjOutBatch("192.0.2.0/24", "10.0.0.1"), plugin.OperatorSender()))
	release()

	var seen []wireUpdate
	require.Eventually(t, func() bool {
		seen = parseWireUpdates(t, conn.written())
		return len(seen) >= 2
	}, 5*time.Second, 5*time.Millisecond, "the announce and the withdrawal must both reach the wire")
	require.Equal(t, []string{"announce", "withdraw"}, wireKinds(seen), "wire order")
}
