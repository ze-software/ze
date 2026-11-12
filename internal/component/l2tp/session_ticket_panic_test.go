// Design: docs/architecture/l2tp/subscriber-session-model.md -- the order ticket on the exit of a contained panic

// VALIDATES: spec-single-owner-lifecycle-5-session-end-locals R-3 on the exit
// of a panic handle contains: the session-downs a drain handed to handle
// before the panic are delivered, and their order ticket is finished.
// PREVENTS: a contained control panic between a drain and its delivery
// leaving the tunnel's ticket open. The teardown clearTunnelAfterPanic runs
// next would then wait for ever behind it on the reactor goroutine, which
// stops every tunnel on the box.
//
// Method: the reactor's injected clock panics once, on its first call after a
// tie-breaker discarded tunnel orderTestTID and the winning tunnel entered the
// map. That is the call handle makes right after locateTunnelLocked returned
// the discarded tunnel's drained downs, inside the locked dispatch. The clock
// is the only dependency the reactor takes that a test can make panic inside
// a drain-to-delivery window. The window after the dispatched tunnel's own
// drain (collectKernelEventsLocked, then the unlock) has none: the listener,
// the kernel worker and the reliable engine are concrete types. Both windows
// end at the same deferred delivery in handle, which this test drives.

package l2tp

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	l2tpevents "github.com/ze-software/ze/internal/component/l2tp/events"
)

// tieBreakerDrainPanic is the value the armed clock panics with, so the test
// can tell from the guard's log that the window was reached.
const tieBreakerDrainPanic = "clock panicked after the tie-breaker drain"

// armClockPanicAfterTieBreakerDrain makes r's clock panic once, on its first
// call after loser took an order ticket and the winning tunnel replaced it as
// the only tunnel in the map. The clock runs on the goroutine that holds
// tunnelsMu at each of those calls, so it reads the map and the loser's order
// pointer without a lock of its own.
func armClockPanicAfterTieBreakerDrain(r *l2tpReactor, loser *L2TPTunnel) {
	fired := false
	r.params.Clock = func() time.Time {
		if !fired && loser.noticeOrder != nil && len(r.tunnelsByLocalID) == 1 {
			fired = true
			panic(tieBreakerDrainPanic)
		}
		return time.Unix(1000, 0).UTC()
	}
}

// driveTieBreakerLossThatPanics runs, on r, an SCCRQ whose lower tie breaker
// discards loser, with the clock armed to panic right after the discard's
// drain reached handle. It fails the test when the guard did not contain that
// panic, because the window under test was then not reached.
func driveTieBreakerLossThatPanics(t *testing.T, r *l2tpReactor, loser *L2TPTunnel) {
	t.Helper()
	_, guardLog := swapL2TPPanics(t)
	setTieBreaker(r, loser, []byte{9, 9, 9, 9, 9, 9, 9, 9})
	armClockPanicAfterTieBreakerDrain(r, loser)

	r.handle(rxPacket{from: orderTestPeer, bytes: buildSCCRQWithTieBreaker(t, 99, "peer", []byte{1, 1, 1, 1, 1, 1, 1, 1}), release: func() {}})

	require.Contains(t, guardLog.String(), tieBreakerDrainPanic,
		"the clock never panicked inside the dispatch, so the window between the drain and the delivery was not reached")
}

// TestContainedPanicAfterADrainFinishesItsTicket validates R-3 on a panic
// exit: after a contained panic between the tie-breaker drain and the
// delivery, the discarded tunnel's order has finished every ticket it issued.
func TestContainedPanicAfterADrainFinishesItsTicket(t *testing.T) {
	r, loser := newReactorForTicketPaths(t, newSessionOrderBus())

	driveTieBreakerLossThatPanics(t, r, loser)

	issued, finished := ticketCounts(r, loser)
	require.Positive(t, issued, "the tie-breaker drain took no order ticket for the session it removed")
	require.Equal(t, issued, finished,
		"the tunnel's order finished %d of the %d tickets it issued after the contained panic: the teardown that follows would wait for ever", finished, issued)
}

// TestContainedPanicAfterADrainDeliversItsDowns validates that the panic exit
// delivers what the drain returned: the route observer withdraws the
// discarded tunnel's established session, and the bus carries its one
// SessionDown.
func TestContainedPanicAfterADrainDeliversItsDowns(t *testing.T) {
	bus := newSessionOrderBus()
	notices := subscribeSessionNoticeLog(bus)
	r, loser := newReactorForTicketPaths(t, bus)
	observer := &recordingRouteObserver{}
	r.routeObserver = observer

	driveTieBreakerLossThatPanics(t, r, loser)

	require.Equal(t, [][2]uint16{{orderTestTID, orderTestSID}}, observer.downs(),
		"the route observer was not told to withdraw the discarded tunnel's established session")
	require.Equal(t, []string{l2tpevents.SessionDownEvent}, notices.of(orderTestTID, orderTestSID),
		"the bus did not carry exactly one SessionDown for the discarded tunnel's session")
}
