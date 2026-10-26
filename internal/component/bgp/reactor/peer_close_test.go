// Design: docs/architecture/behavior/peer-lifecycle.md -- why a session left Established
// Overview: peer_close.go -- sessionCloseFor, the producer under test
// Related: rfc4760_peer_down_link_test.go -- startMPLinkNeighbor, the running Peer these drive

package reactor

import (
	"net/netip"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/bgp/fsm"
	"github.com/ze-software/ze/internal/component/bgp/message"
	"github.com/ze-software/ze/pkg/plugin/rpc"
)

// closeWithReason builds the close of a session that no FSM event ended, for the tests
// that hand a close straight to notifyPeerClosed or announceClosed rather than driving
// a running Peer to it. The production path derives every close with sessionCloseFor.
func closeWithReason(reason string) sessionClose {
	return sessionClose{reason: reason}
}

// awaitClose returns the close the running Peer reported to its lifecycle observers.
func awaitClose(t *testing.T, link *mpLinkNeighbor) sessionClose {
	t.Helper()
	select {
	case closed := <-link.events.causes:
		return closed
	case <-time.After(5 * time.Second):
		t.Fatal("the run loop never reported the session closed")
	}
	return sessionClose{}
}

// TestPeerCloseNamesTheNotificationTheNeighborSent drives a running Peer to Established
// and has the neighbor send a Cease NOTIFICATION.
//
// VALIDATES: the down the reactor hands its observers names the NOTIFICATION
// (rpc.ReasonNotification) and the FSM event that closed the session, Event 25 NotifMsg.
// PREVENTS: the run loop reporting a NOTIFICATION close under a reason no plugin reads as
// one, which is how Graceful Restart came to retain routes after a NOTIFICATION.
//
// RFC requirement: RFC4724-4-2 positive -- a session the neighbor ends with a NOTIFICATION
// is reported down by the running Peer with reason rpc.ReasonNotification, the reason on
// which Graceful Restart follows the normal BGP procedures instead of retaining routes.
//
// MUTATION: in sessionCloseFor, return rpc.ReasonTCPFailure for a notified close; this
// test goes red.
func TestPeerCloseNamesTheNotificationTheNeighborSent(t *testing.T) {
	link := startMPLinkNeighbor(t)

	cease := &message.Notification{ErrorCode: message.NotifyCease, ErrorSubcode: message.NotifyCeaseAdminShutdown}
	_, err := link.conn.Write(message.PackTo(cease, nil))
	require.NoError(t, err)

	closed := awaitClose(t, link)
	require.Equal(t, rpc.ReasonNotification, closed.reason)
	want, ok := fsm.EventNotifMsg.Number()
	require.True(t, ok)
	require.Equal(t, want, closed.fsmEvent, "the close carries Event 25, NotifMsg")
}

// TestPeerCloseNamesTheTransportLoss drives a running Peer to Established and has the
// neighbor drop the TCP connection with no NOTIFICATION.
//
// VALIDATES: the down names the transport loss (rpc.ReasonTCPFailure) and Event 18,
// TcpConnectionFails.
// PREVENTS: a transport loss reported as a NOTIFICATION, which would purge the routes
// RFC 4724 Section 4.2 retains, or as a local close, which BMP reports as reason 2.
//
// RFC requirement: RFC4724-4-2 negative -- a session the neighbor ends by dropping TCP with
// no NOTIFICATION is reported down with reason rpc.ReasonTCPFailure, never
// rpc.ReasonNotification, so Graceful Restart still retains its routes.
// RFC requirement: RFC7854-x-10 positive -- the close the running Peer reports for a dropped
// transport says why the session closed: reason rpc.ReasonTCPFailure with FSM Event 18.
//
// MUTATION: in sessionCloseFor, drop the Event 18 branch so the loss reads as a local
// close; this test goes red.
func TestPeerCloseNamesTheTransportLoss(t *testing.T) {
	link := startMPLinkNeighbor(t)

	require.NoError(t, link.conn.Close())

	closed := awaitClose(t, link)
	require.Equal(t, rpc.ReasonTCPFailure, closed.reason)
	want, ok := fsm.EventTCPConnectionFails.Number()
	require.True(t, ok)
	require.Equal(t, want, closed.fsmEvent, "the close carries Event 18, TcpConnectionFails")
}

// TestPeerCloseNamesTheCongestionTeardownLocal drives a running Peer to Established and
// tears it down through the GR-aware congestion teardown, which closes the transport with
// no NOTIFICATION and fires Event 18.
//
// VALIDATES: the down names a local close (rpc.ReasonLocalClose) carrying Event 18,
// TcpConnectionFails, because ze closed that transport itself.
// PREVENTS: the teardown reported as the transport loss it imitates, which BMP writes as
// Peer Down reason 4, "The remote system closed the session without a notification
// message", when the local system closed it.
//
// RFC requirement: RFC7854-x-10 positive -- the close the running Peer reports for a
// session ze tore down for congestion says why the session closed: reason
// rpc.ReasonLocalClose with FSM Event 18, never rpc.ReasonTCPFailure.
//
// MUTATION: in sessionCloseFor, drop the ErrCongestionTeardown branch so the teardown
// reads as a transport loss; this test goes red.
func TestPeerCloseNamesTheCongestionTeardownLocal(t *testing.T) {
	link := startMPLinkNeighbor(t)

	teardown := congestionTeardownPeer(func(netip.AddrPort) *Peer { return link.peer })
	teardown(netip.AddrPort{}, true)

	closed := awaitClose(t, link)
	require.Equal(t, rpc.ReasonLocalClose, closed.reason)
	want, ok := fsm.EventTCPConnectionFails.Number()
	require.True(t, ok)
	require.Equal(t, want, closed.fsmEvent, "the close carries Event 18, TcpConnectionFails")
}

// TestSessionCloseForCoversEveryCause holds the derivation to its four outcomes, for the
// closes a scripted neighbor cannot produce: ze's own NOTIFICATION, a session whose
// received UPDATEs could not be published, and a close ze made with no NOTIFICATION on
// the wire.
//
// VALIDATES: a NOTIFICATION decides first whatever the event; without one a failed
// receive publication is rpc.ReasonAdjRIBInInvalid whatever the event; Event 18 without
// either is a transport loss unless the session recorded ErrCongestionTeardown, which
// makes it a local close; any other event without either is a local close carrying that
// event's RFC 4271 Section 8.1 number.
// PREVENTS: an automatic stop whose Cease could not be written being reported as a
// NOTIFICATION, or as an event number ze never observed, and a session whose Adj-RIB-In
// is known invalid being reported as a close Graceful Restart retains routes after.
func TestSessionCloseForCoversEveryCause(t *testing.T) {
	tests := []struct {
		name     string
		notified bool
		failed   bool
		event    fsm.Event
		cause    error
		reason   string
		number   uint16
	}{
		{"ze sent a Hold Timer Expired NOTIFICATION", true, false, fsm.EventHoldTimerExpires, nil, rpc.ReasonNotification, 10},
		{"a notification wins over a transport loss", true, false, fsm.EventTCPConnectionFails, nil, rpc.ReasonNotification, 18},
		{"a delivered Cease wins over a failed publication", true, true, fsm.EventAutomaticStop, nil, rpc.ReasonNotification, 8},
		{"failed publication whose Cease was not written", false, true, fsm.EventAutomaticStop, ErrTeardown, rpc.ReasonAdjRIBInInvalid, 8},
		{"failed publication after a transport loss", false, true, fsm.EventTCPConnectionFails, nil, rpc.ReasonAdjRIBInInvalid, 18},
		{"failed publication after a congestion teardown", false, true, fsm.EventTCPConnectionFails, ErrCongestionTeardown, rpc.ReasonAdjRIBInInvalid, 18},
		{"transport loss", false, false, fsm.EventTCPConnectionFails, nil, rpc.ReasonTCPFailure, 18},
		{"transport loss with an unrelated close reason", false, false, fsm.EventTCPConnectionFails, ErrHoldTimerExpired, rpc.ReasonTCPFailure, 18},
		{"congestion teardown", false, false, fsm.EventTCPConnectionFails, ErrCongestionTeardown, rpc.ReasonLocalClose, 18},
		{"automatic stop whose Cease was not written", false, false, fsm.EventAutomaticStop, nil, rpc.ReasonLocalClose, 8},
		{"manual stop whose Cease was not written", false, false, fsm.EventManualStop, nil, rpc.ReasonLocalClose, 2},
		{"BFD down", false, false, fsm.EventBfdDown, nil, rpc.ReasonLocalClose, 31},
		{"an event with no registered number", false, false, fsm.Event(-1), nil, rpc.ReasonLocalClose, 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := sessionCloseFor(tt.notified, tt.failed, tt.event, tt.cause)
			require.Equal(t, tt.reason, got.reason)
			require.Equal(t, tt.number, got.fsmEvent)
		})
	}
}
