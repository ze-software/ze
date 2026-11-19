// Design: docs/architecture/l2tp/subscriber-session-model.md -- the route observer hears a withdrawal for each established session removed

// VALIDATES: retention reaping and tie-breaker loss withdraw the original
// established subscriber's route. A pre-PPP removal still publishes its
// SessionDown and leaves the other subscriber's route live.
// PREVENTS: a reaped or tie-breaker-discarded tunnel leaving its subscriber
// routes injected, and a withdrawal for a session that never ran PPP removing
// the record of another admitted session.
//
// Method: each test drives one removal path on the ticket paths' reactor,
// whose tunnel orderTestTID holds the established session orderTestSID, with
// the real subscriber bridge and route observer, plus a reader that records
// each session event the bus carries.

package l2tp

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	l2tpevents "github.com/ze-software/ze/internal/component/l2tp/events"
	"github.com/ze-software/ze/internal/core/redistevents"
)

// newReactorWithRecordedWithdrawals builds the ticket paths' reactor with a
// route observer that records each withdrawal, and a log of the session
// events the bus carries.
func newReactorWithRecordedWithdrawals(t *testing.T) (*l2tpReactor, *L2TPTunnel, *recordingRouteObserver, *sessionNoticeLog) {
	t.Helper()
	bus := newSessionOrderBus()
	notices := subscribeSessionNoticeLog(bus)
	r, tun := newReactorForTicketPaths(t, bus)
	observer := &recordingRouteObserver{}
	observer.start(t, bus, r.logger)
	return r, tun, observer, notices
}

// TestRetentionReapWithdrawsItsEstablishedSessionRoute validates that a tunnel
// reaped by retention (reapExpiredLocked) with an established session tells
// the route observer to withdraw that session's route.
func TestRetentionReapWithdrawsItsEstablishedSessionRoute(t *testing.T) {
	r, tun, observer, _ := newReactorWithRecordedWithdrawals(t)
	original := observer.seedHostRoute(t, tun.sessions[orderTestSID].Birth(), orderTestTID, orderTestSID)
	r.tunnelsMu.Lock()
	tun.state = L2TPTunnelClosed
	tun.engine.closed = true
	tun.engine.closedAt = time.Unix(0, 0).UTC()
	tun.engine.retention = time.Second
	r.tunnelsMu.Unlock()

	r.handleTick(tickReq{tunnelID: orderTestTID + 1})

	require.Equal(t, [][2]uint16{{orderTestTID, orderTestSID}}, observer.downs(),
		"the reaped tunnel's established session kept its subscriber route")
	observer.requireHostWithdrawal(t, original)
}

// TestTieBreakerLossWithdrawsItsEstablishedSessionRoute validates that a
// tunnel discarded by a tie breaker (resolveTieBreakerLocked) with an
// established session tells the route observer to withdraw that session's
// route. The SCCRQ's tie breaker equals the tunnel's, so both sides discard
// and handle dispatches no tunnel: the branch that delivers only the
// discarded tunnel's downs.
func TestTieBreakerLossWithdrawsItsEstablishedSessionRoute(t *testing.T) {
	r, tun, observer, _ := newReactorWithRecordedWithdrawals(t)
	original := observer.seedHostRoute(t, tun.sessions[orderTestSID].Birth(), orderTestTID, orderTestSID)
	equalTB := []byte{5, 5, 5, 5, 5, 5, 5, 5}
	setTieBreaker(r, tun, equalTB)

	r.handle(rxPacket{from: orderTestPeer, bytes: buildSCCRQWithTieBreaker(t, 99, "peer", equalTB), release: func() {}})

	require.Equal(t, [][2]uint16{{orderTestTID, orderTestSID}}, observer.downs(),
		"the discarded tunnel's established session kept its subscriber route")
	observer.requireHostWithdrawal(t, original)
}

// TestSessionRemovedBeforeEstablishedWithdrawsNoRoute validates that a session
// torn down before it reached established sends the route observer nothing,
// and that the bus still carries its one SessionDown, so the readers that
// keep their own state still learn it ended.
func TestSessionRemovedBeforeEstablishedWithdrawsNoRoute(t *testing.T) {
	const waitingSID uint16 = orderTestSID + 1
	r, tun, observer, notices := newReactorWithRecordedWithdrawals(t)
	original := observer.seedHostRoute(t, tun.sessions[orderTestSID].Birth(), orderTestTID, orderTestSID)
	waiting := &L2TPSession{localSID: waitingSID, remoteSID: waitingSID + 100, state: L2TPSessionWaitConnect}
	newFixtureL2TPSession(t, waiting)
	r.tunnelsMu.Lock()
	tun.sessions[waitingSID] = waiting
	r.tunnelsMu.Unlock()

	require.NoError(t, teardownCapturedByID(t, r, orderTestTID, waitingSID))

	require.Empty(t, actorRouteEntries(t, observer.routes.events(), redistevents.ActionRemove, 0),
		"a session that never ran PPP must not withdraw a route")
	require.Equal(t, []string{l2tpevents.SessionDownEvent}, notices.of(orderTestTID, waitingSID),
		"the bus did not carry exactly one SessionDown for the session removed before it was established")
	observer.replay(t, 7)
	require.Equal(t, []redistevents.RouteChangeEntry{original}, actorRouteEntries(t, observer.routes.events(), redistevents.ActionAdd, 7))
}
