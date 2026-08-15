// Design: docs/architecture/core-design.md — BGP reactor event loop
// Related: reactor_api_forward.go — forwardUpdateCore dispatch gate
// Related: forward_rs.go — reactorForwardRS dispatch gate
// Related: forward_pool.go — drainOverflow ordering hold
package reactor

import (
	"bufio"
	"bytes"
	"encoding/binary"
	"net/netip"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/bgp/fsm"
	"github.com/ze-software/ze/internal/component/bgp/message"
	"github.com/ze-software/ze/internal/component/bgp/wireu"
	"github.com/ze-software/ze/internal/component/plugin"
	bgpctx "github.com/ze-software/ze/internal/core/bgp/context"
	"github.com/ze-software/ze/internal/core/bgp/msgtype"
	"github.com/ze-software/ze/internal/core/family"
	"github.com/ze-software/ze/internal/core/selector"
)

// syncOrderPrefix is the one prefix these tests announce and withdraw, and
// syncOrderPrefixWire is its NLRI encoding (length byte then 3 significant
// octets). Both wire sections of an UPDATE are searched for that encoding, so
// the two operations on the same prefix are told apart by WHICH section carries
// it rather than by message order.
const syncOrderPrefix = "192.0.2.0/24"

var syncOrderPrefixWire = []byte{24, 192, 0, 2}

// syncOrderWithdrawBody is an UPDATE body that withdraws syncOrderPrefix and
// advertises nothing: withdrawn-routes length 4, the prefix, attribute length 0.
var syncOrderWithdrawBody = []byte{0, 4, 24, 192, 0, 2, 0, 0}

// syncOrderAnnounceBody is an UPDATE body that announces syncOrderPrefix:
// withdrawn-routes length 0, then ORIGIN, AS_PATH and NEXT_HOP, then the prefix.
var syncOrderAnnounceBody = []byte{
	0, 0, // withdrawn routes length
	0, 20, // total path attribute length
	0x40, 1, 1, 0, // ORIGIN = IGP
	0x40, 2, 6, 2, 1, 0, 0, 0xfd, 0xe9, // AS_PATH = AS_SEQUENCE [65001]
	0x40, 3, 4, 10, 0, 0, 1, // NEXT_HOP = 10.0.0.1
	24, 192, 0, 2, // NLRI = 192.0.2.0/24
}

// wireUpdate is one UPDATE read back off a destination's connection, reduced to
// the only two facts these tests assert on.
type wireUpdate struct {
	announces bool
	withdraws bool
}

// parseWireUpdates splits a destination's byte stream into BGP messages and
// reports, for each UPDATE, whether it announces or withdraws syncOrderPrefix.
// A truncated tail is ignored: the worker flushes whole messages, so a partial
// one only means the read raced the write.
func parseWireUpdates(t *testing.T, raw []byte) []wireUpdate {
	t.Helper()
	var out []wireUpdate
	for len(raw) >= message.HeaderLen {
		length := int(binary.BigEndian.Uint16(raw[16:18]))
		if length < message.HeaderLen || length > len(raw) {
			break
		}
		msgType := msgtype.MessageType(raw[18])
		body := raw[message.HeaderLen:length]
		raw = raw[length:]
		if msgType != msgtype.TypeUPDATE || len(body) < 4 {
			continue
		}
		withdrawnLen := int(binary.BigEndian.Uint16(body[0:2]))
		if 2+withdrawnLen+2 > len(body) {
			break
		}
		withdrawn := body[2 : 2+withdrawnLen]
		attrLen := int(binary.BigEndian.Uint16(body[2+withdrawnLen : 4+withdrawnLen]))
		if 4+withdrawnLen+attrLen > len(body) {
			break
		}
		nlri := body[4+withdrawnLen+attrLen:]
		out = append(out, wireUpdate{
			announces: bytes.Contains(nlri, syncOrderPrefixWire),
			withdraws: bytes.Contains(withdrawn, syncOrderPrefixWire),
		})
	}
	return out
}

// newSyncOrderDest builds the destination peer of these tests: Established,
// IBGP so the forwarded body stays zero-copy, backed by a Session writing into
// the returned recordingConn, and holding the initial-sync gate setState closes
// in the same call that publishes Established (peer.go).
func newSyncOrderDest(t *testing.T, ctx *bgpctx.EncodingContext, ctxID bgpctx.ContextID) (*Peer, *recordingConn) {
	t.Helper()
	settings := &PeerSettings{
		Connection: ConnectionBoth,
		Address:    netip.MustParseAddr("10.0.0.2"),
		LocalAS:    65000,
		PeerAS:     65000,
		RouterID:   0x01020302,
	}
	peer := NewPeer(settings)
	peer.state.Store(int32(PeerStateEstablished))
	peer.negotiated.Store(&NegotiatedCapabilities{
		families:        map[family.Family]bool{family.IPv4Unicast: true},
		ExtendedMessage: false,
	})
	peer.sendCtx.Store(ctx)
	peer.sendCtxID = ctxID
	peer.refreshForwardFacts()

	session := NewSession(settings)
	require.NoError(t, session.fsm.Event(fsm.EventManualStart))
	require.NoError(t, session.fsm.Event(fsm.EventTCPConnectionConfirmed))
	require.NoError(t, session.fsm.Event(fsm.EventBGPOpen))
	require.NoError(t, session.fsm.Event(fsm.EventKeepaliveMsg))
	require.Equal(t, fsm.StateEstablished, session.fsm.State())

	conn := &recordingConn{}
	session.mu.Lock()
	session.conn = conn
	session.bufWriter = bufio.NewWriterSize(conn, 4096)
	session.mu.Unlock()

	peer.mu.Lock()
	peer.session = session
	peer.mu.Unlock()

	peer.sendingInitialRoutes.Store(1)
	return peer, conn
}

// newSyncOrderRail wires the source peer, the mid-sync destination and a forward
// pool running the production batch handler, so a dispatched item reaches the
// destination's connection exactly as it does in the daemon. The encoding
// context it returns is the one both peers send under, so a forwarded body
// stays zero-copy.
func newSyncOrderRail(t *testing.T) (*Reactor, *Peer, *Peer, *recordingConn, bgpctx.ContextID) {
	t.Helper()
	return newSyncOrderRailWith(t, fwdBatchHandler)
}

// newSyncOrderRailWith is newSyncOrderRail with the pool's batch handler chosen
// by the caller. One test needs to know WHEN the destination's worker runs, and
// the handler is the only point in a worker's cycle a test can observe: runWorker
// calls it, then calls drainOverflow.
func newSyncOrderRailWith(t *testing.T, handler func(fwdKey, []fwdItem)) (*Reactor, *Peer, *Peer, *recordingConn, bgpctx.ContextID) {
	t.Helper()

	ctx := bgpctx.EncodingContextForASN4(true)
	ctxID, _ := bgpctx.Registry.Register(ctx)

	cache := newRecentUpdateCache(100)
	t.Cleanup(cache.Stop)

	src := makeForwardSourcePeer(t, ctx, ctxID)
	dst, conn := newSyncOrderDest(t, ctx, ctxID)

	pool := newFwdPool(handler, fwdPoolConfig{chanSize: 8, idleTimeout: time.Second})
	t.Cleanup(pool.Stop)

	r := &Reactor{
		attrModHandlers: attrModHandlersWithDefaults(),
		recentUpdates:   cache,
		peers: map[netip.AddrPort]*Peer{
			src.Settings().PeerKey(): src,
			dst.Settings().PeerKey(): dst,
		},
		fwdPool: pool,
	}
	dst.SetReactor(r)

	return r, src, dst, conn, ctxID
}

// syncOrderPublish puts one UPDATE body in the reactor's recent-update cache
// under updateID, as a session read does, and returns it ready to forward.
func syncOrderPublish(t *testing.T, r *Reactor, ctxID bgpctx.ContextID, updateID uint64, body []byte) *ReceivedUpdate {
	t.Helper()

	wu := wireu.NewWireUpdate(body, ctxID)
	wu.SetMessageID(updateID)
	update := &ReceivedUpdate{
		WireUpdate:   wu,
		SourcePeerIP: netip.MustParseAddr(forwardSourceAddr),
		ReceivedAt:   time.Now(),
	}
	r.recentUpdates.Add(update)
	r.recentUpdates.Activate(updateID, 1)
	return update
}

// newSyncOrderFixture is newSyncOrderRail with the two things both AC-1 tests
// need: the forwarded withdraw of syncOrderPrefix, and the announce of the same
// prefix already queued for the destination.
func newSyncOrderFixture(t *testing.T, updateID uint64) (*Reactor, *Peer, *Peer, *recordingConn, *ReceivedUpdate) {
	t.Helper()

	r, src, dst, conn, ctxID := newSyncOrderRail(t)
	update := syncOrderPublish(t, r, ctxID, updateID, syncOrderWithdrawBody)

	// The announce this forwarded withdraw must never overtake. It is queued,
	// not sent: ShouldQueue() is true for a peer inside its initial sync, so the
	// injection rail parks it here and sendInitialRoutes drains it.
	dst.QueueAnnounce(testRoute(syncOrderPrefix))

	return r, src, dst, conn, update
}

// assertAnnounceThenWithdraw runs the destination's initial sync and pins the
// order the peer sees.
func assertAnnounceThenWithdraw(t *testing.T, dst *Peer, conn *recordingConn) {
	t.Helper()

	// Nothing the peer can act on reaches it while the sync is still to run: the
	// queued announce is in opQueue and the forwarded withdraw is parked behind
	// it. Never, not Empty: the dispatch gate alone leaves the item in a worker
	// that drains it microseconds later, so a single sample would pass against a
	// missing hold.
	require.Never(t, func() bool {
		return len(parseWireUpdates(t, conn.written())) > 0
	}, 200*time.Millisecond, 5*time.Millisecond,
		"no UPDATE may reach the peer before its initial sync runs")

	dst.sendInitialRoutes()

	var seen []wireUpdate
	require.Eventually(t, func() bool {
		seen = parseWireUpdates(t, conn.written())
		for _, u := range seen {
			if u.withdraws {
				return true
			}
		}
		return false
	}, 5*time.Second, 5*time.Millisecond,
		"the forwarded withdraw must reach the peer once its initial sync ends")

	announceAt, withdrawAt := -1, -1
	for i, u := range seen {
		if u.announces && announceAt < 0 {
			announceAt = i
		}
		if u.withdraws {
			withdrawAt = i
		}
	}
	require.GreaterOrEqual(t, announceAt, 0, "the queued announce must reach the peer")
	require.Less(t, announceAt, withdrawAt,
		"the queued announce must reach the peer BEFORE the forwarded withdraw of the same prefix")
	require.False(t, seen[len(seen)-1].announces,
		"the peer must not end holding a prefix that was withdrawn")
}

// TestForwardedWithdrawWaitsForQueuedAnnounceCoreRail pins AC-1 on the plugin
// forwarding rail: a withdraw forwarded while the destination is inside its
// initial route sync reaches that peer AFTER the announce already queued for it,
// so the peer does not end holding a prefix that was withdrawn.
//
// VALIDATES: AC-1 -- forwardUpdateCore consults Peer.ShouldQueue() and parks the
// item in overflow, and drainOverflow keeps it there until the sync ends.
// PREVENTS: the routing blackhole this spec exists to remove. Without the gate
// the withdraw is written and flushed immediately, the queued announce drains
// after it, and the peer believes the prefix is live.
func TestForwardedWithdrawWaitsForQueuedAnnounceCoreRail(t *testing.T) {
	const updateID uint64 = 7100

	r, _, dst, conn, _ := newSyncOrderFixture(t, updateID)
	adapter := &reactorAPIAdapter{r: r}

	sel, err := selector.Parse("*")
	require.NoError(t, err)
	require.NoError(t, adapter.ForwardUpdate(sel, updateID, "test-plugin", plugin.OperatorSender()))

	assertAnnounceThenWithdraw(t, dst, conn)
}

// TestForwardedWithdrawWaitsForQueuedAnnounceRSRail pins AC-1 on the route-server
// fast path, which reaches the destination's write buffer directly and so cannot
// be covered by the plugin rail's test.
//
// VALIDATES: AC-1 -- reactorForwardRS consults Peer.ShouldQueue() BEFORE
// tryDirectWriteNoFlush, so the item takes the overflow path instead of the
// destination's bufWriter.
// PREVENTS: the same blackhole reaching the peer by the rail that never touches
// the forward pool.
func TestForwardedWithdrawWaitsForQueuedAnnounceRSRail(t *testing.T) {
	const updateID uint64 = 7200

	r, src, dst, conn, update := newSyncOrderFixture(t, updateID)

	_, dispatched := reactorForwardRS(r, update, updateID, netip.MustParseAddr(forwardSourceAddr), src)
	require.Equal(t, 1, dispatched, "the destination must be dispatched to, not dropped")

	assertAnnounceThenWithdraw(t, dst, conn)
}

// TestForwardedUpdateWaitsForPendingOverflowRSRail pins the SECOND half of the
// route-server rail's ordering gate, the one the sync-hold tests cannot reach.
//
// Once a destination leaves its initial sync, the items parked behind that sync
// are still in its worker's overflow, on their way to the wire. A direct write
// would overtake them, so the rail asks the pool as well as the peer. The test
// holds that state open: the sync flag is cleared WITHOUT the wake
// Peer.wakeForwardOverflow sends, so the announce stays parked while
// forwardOrderHold() is already false. Nothing but the pending-overflow count
// can hold the withdraw back.
//
// VALIDATES: AC-1 -- reactorForwardRS consults Peer.forwardOverflowPending
// before tryDirectWriteNoFlush, so a forwarded UPDATE queues behind the items
// already waiting for the same destination.
// PREVENTS: the blackhole re-opening in the window between the end of a sync and
// the drain of what that sync parked. TryDispatch refuses its channel for the
// same reason; the direct write bypasses the pool and would not ask.
func TestForwardedUpdateWaitsForPendingOverflowRSRail(t *testing.T) {
	const announceID, withdrawID uint64 = 7400, 7401

	// gate blocks the destination's worker inside its FIRST batch, which is the
	// wake sentinel DispatchOverflow sends. runWorker calls the handler and only
	// then calls drainOverflow, so nothing can leave overflow until this test
	// opens the gate. Without it the worker could drain between the store and
	// the dispatch below, and the test would race its own setup.
	gate := make(chan struct{})
	var gateOnce, openOnce sync.Once
	r, src, dst, conn, ctxID := newSyncOrderRailWith(t, func(key fwdKey, items []fwdItem) {
		gateOnce.Do(func() { <-gate })
		fwdBatchHandler(key, items)
	})
	// Registered AFTER the rail, so it runs BEFORE the pool's own Stop: a failed
	// assertion below would otherwise leave Stop waiting on a worker blocked in
	// the handler, and the test would hang instead of reporting.
	openGate := func() { openOnce.Do(func() { close(gate) }) }
	t.Cleanup(openGate)

	srcAddr := netip.MustParseAddr(forwardSourceAddr)

	// Park the announce: the destination is inside its initial sync, so the RS
	// rail sends it to overflow and drainOverflow holds it there.
	announce := syncOrderPublish(t, r, ctxID, announceID, syncOrderAnnounceBody)
	_, dispatched := reactorForwardRS(r, announce, announceID, srcAddr, src)
	require.Equal(t, 1, dispatched, "the announce must be dispatched, not dropped")
	require.Eventually(t, func() bool {
		return dst.forwardOverflowPending()
	}, 5*time.Second, 5*time.Millisecond,
		"the forwarded announce must be parked in the destination's overflow")

	// The sync ends. The store is made directly, without the wake that follows
	// it in the daemon, so the announce is still parked here and the ONLY thing
	// that can hold the withdraw back is the pending-overflow count.
	dst.sendingInitialRoutes.Store(0)
	require.False(t, dst.forwardOrderHold(), "the destination must be out of its sync hold")
	require.True(t, dst.forwardOverflowPending(), "the announce must still be parked")

	withdraw := syncOrderPublish(t, r, ctxID, withdrawID, syncOrderWithdrawBody)
	_, dispatched = reactorForwardRS(r, withdraw, withdrawID, srcAddr, src)
	require.Equal(t, 1, dispatched, "the withdraw must be dispatched, not dropped")

	// Let the worker finish its sentinel batch. drainOverflow runs next, the
	// hold is already clear, and the two items leave in the order they were
	// queued. Both reached overflow, so both go out through the pool.
	openGate()

	var seen []wireUpdate
	require.Eventually(t, func() bool {
		seen = parseWireUpdates(t, conn.written())
		return len(seen) >= 2
	}, 5*time.Second, 5*time.Millisecond,
		"both forwarded UPDATEs must reach the peer")

	announceAt, withdrawAt := -1, -1
	for i, u := range seen {
		if u.announces && announceAt < 0 {
			announceAt = i
		}
		if u.withdraws {
			withdrawAt = i
		}
	}
	require.GreaterOrEqual(t, announceAt, 0, "the parked announce must reach the peer")
	require.GreaterOrEqual(t, withdrawAt, 0, "the forwarded withdraw must reach the peer")
	require.Less(t, announceAt, withdrawAt,
		"the parked announce must reach the peer BEFORE the withdraw forwarded after it")
	require.False(t, seen[len(seen)-1].announces,
		"the peer must not end holding a prefix that was withdrawn")
}

// fwdBatchHasRealItem reports whether a batch carries route data. Sentinels
// (overflow wake-ups, barrier done-callbacks) carry a nil peer, and a test that
// wants the batch a forwarded UPDATE travels in has to skip them: a worker sees
// one or more sentinel batches before the batch that matters.
func fwdBatchHasRealItem(items []fwdItem) bool {
	for i := range items {
		if items[i].peer != nil {
			return true
		}
	}
	return false
}

// TestForwardedUpdateWaitsForInFlightOverflowRSRail pins the LAST window of the
// route-server rail's ordering gate: the one between a released overflow item
// reaching the worker's channel and the worker taking session.writeMu.
//
// The item is out of w.overflow by then, so a gate that counted only the queued
// items reports "nothing pending" while those bytes are still nowhere near the
// wire. tryDirectWriteNoFlush takes the same writeMu with TryLock and writes
// straight into the destination's bufWriter, so a forward that lands in that
// window is written FIRST -- the same inversion this spec exists to remove, one
// window later.
//
// The window is held open deterministically: the pool's batch handler blocks on
// entry for the first batch carrying a real item, which is the released
// announce, and blocks BEFORE fwdBatchHandler so writeMu is still free. The
// withdraw is forwarded from the test goroutine while the worker sits there.
//
// VALIDATES: AC-1 -- overflowPending covers items in flight, not only items
// queued, so Peer.forwardOverflowPending refuses the direct write until the
// released announce has been written.
// PREVENTS: the blackhole re-opening in the enqueue-to-writeMu gap. With the
// count decremented as each item enters the channel, the withdraw goes to the
// destination's bufWriter first and the peer ends holding a prefix that was
// withdrawn.
func TestForwardedUpdateWaitsForInFlightOverflowRSRail(t *testing.T) {
	const announceID, withdrawID uint64 = 7500, 7501

	// entered fires when the worker is inside the batch that carries the
	// released announce, and release lets that batch proceed to the wire.
	entered := make(chan struct{})
	release := make(chan struct{})
	var enterOnce, releaseOnce sync.Once
	r, src, dst, conn, ctxID := newSyncOrderRailWith(t, func(key fwdKey, items []fwdItem) {
		if fwdBatchHasRealItem(items) {
			enterOnce.Do(func() {
				close(entered)
				<-release
			})
		}
		fwdBatchHandler(key, items)
	})
	// Registered AFTER the rail, so it runs BEFORE the pool's own Stop: a
	// failed assertion below would otherwise leave Stop waiting on a worker
	// blocked in the handler, and the test would hang instead of reporting.
	openGate := func() { releaseOnce.Do(func() { close(release) }) }
	t.Cleanup(openGate)

	srcAddr := netip.MustParseAddr(forwardSourceAddr)

	// Park the announce behind the destination's initial sync.
	announce := syncOrderPublish(t, r, ctxID, announceID, syncOrderAnnounceBody)
	_, dispatched := reactorForwardRS(r, announce, announceID, srcAddr, src)
	require.Equal(t, 1, dispatched, "the announce must be dispatched, not dropped")
	require.Eventually(t, func() bool {
		return dst.forwardOverflowPending()
	}, 5*time.Second, 5*time.Millisecond,
		"the forwarded announce must be parked in the destination's overflow")

	// End the sync and wake the worker, exactly as sendInitialRoutes does. The
	// worker releases the announce from overflow onto its channel and then
	// blocks in the handler, which is the window under test.
	dst.sendingInitialRoutes.Store(0)
	dst.wakeForwardOverflow()
	select {
	case <-entered:
	case <-time.After(5 * time.Second):
		t.Fatal("the released announce never reached the destination's batch handler")
	}
	require.False(t, dst.forwardOrderHold(), "the destination must be out of its sync hold")

	// The announce is in flight: out of w.overflow, not yet written. A withdraw
	// forwarded now must not reach the peer's write buffer ahead of it.
	withdraw := syncOrderPublish(t, r, ctxID, withdrawID, syncOrderWithdrawBody)
	_, dispatched = reactorForwardRS(r, withdraw, withdrawID, srcAddr, src)
	require.Equal(t, 1, dispatched, "the withdraw must be dispatched, not dropped")

	openGate()

	var seen []wireUpdate
	require.Eventually(t, func() bool {
		seen = parseWireUpdates(t, conn.written())
		return len(seen) >= 2
	}, 5*time.Second, 5*time.Millisecond,
		"both forwarded UPDATEs must reach the peer")

	announceAt, withdrawAt := -1, -1
	for i, u := range seen {
		if u.announces && announceAt < 0 {
			announceAt = i
		}
		if u.withdraws {
			withdrawAt = i
		}
	}
	require.GreaterOrEqual(t, announceAt, 0, "the released announce must reach the peer")
	require.GreaterOrEqual(t, withdrawAt, 0, "the forwarded withdraw must reach the peer")
	require.Less(t, announceAt, withdrawAt,
		"the announce released from overflow must reach the peer BEFORE the withdraw forwarded while it was in flight")
	require.False(t, seen[len(seen)-1].announces,
		"the peer must not end holding a prefix that was withdrawn")
}

// TestForwardedAnnounceThenWithdrawKeepOrder pins AC-1 for the shape the two
// tests above do not reach: BOTH operations on the prefix are forwarded, so both
// are parked in the same worker's overflow and released into ONE batch when the
// destination's initial sync ends.
//
// VALIDATES: AC-1 -- the batch handler delivers a batch in the order it was
// queued, so a withdraw that arrived after an announce of the same prefix is
// written after it.
// PREVENTS: the blackhole returning through the batch handler. Sorting a batch
// by kind (withdrawals first) inverts exactly this pair: the peer applies the
// withdraw, then the announce, and ends holding a prefix that was withdrawn.
func TestForwardedAnnounceThenWithdrawKeepOrder(t *testing.T) {
	const announceID, withdrawID uint64 = 7300, 7301

	r, _, dst, conn, ctxID := newSyncOrderRail(t)
	adapter := &reactorAPIAdapter{r: r}

	sel, err := selector.Parse("*")
	require.NoError(t, err)

	syncOrderPublish(t, r, ctxID, announceID, syncOrderAnnounceBody)
	require.NoError(t, adapter.ForwardUpdate(sel, announceID, "test-plugin", plugin.OperatorSender()))
	syncOrderPublish(t, r, ctxID, withdrawID, syncOrderWithdrawBody)
	require.NoError(t, adapter.ForwardUpdate(sel, withdrawID, "test-plugin", plugin.OperatorSender()))

	// Both are parked behind the sync, so neither may reach the peer yet.
	require.Never(t, func() bool {
		return len(parseWireUpdates(t, conn.written())) > 0
	}, 200*time.Millisecond, 5*time.Millisecond,
		"no forwarded UPDATE may reach the peer before its initial sync runs")

	dst.sendInitialRoutes()

	var seen []wireUpdate
	require.Eventually(t, func() bool {
		seen = parseWireUpdates(t, conn.written())
		return len(seen) >= 2
	}, 5*time.Second, 5*time.Millisecond,
		"both forwarded UPDATEs must reach the peer once its initial sync ends")

	announceAt, withdrawAt := -1, -1
	for i, u := range seen {
		if u.announces && announceAt < 0 {
			announceAt = i
		}
		if u.withdraws {
			withdrawAt = i
		}
	}
	require.GreaterOrEqual(t, announceAt, 0, "the forwarded announce must reach the peer")
	require.GreaterOrEqual(t, withdrawAt, 0, "the forwarded withdraw must reach the peer")
	require.Less(t, announceAt, withdrawAt,
		"the forwarded announce must reach the peer BEFORE the forwarded withdraw of the same prefix")
	require.False(t, seen[len(seen)-1].announces,
		"the peer must not end holding a prefix that was withdrawn")
}
