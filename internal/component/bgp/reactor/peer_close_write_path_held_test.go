// Design: docs/architecture/behavior/peer-lifecycle.md -- one close gives one event
// Related: peer_close.go -- sessionDroppedOwed, leaveCause, ErrWritePathHeld
// Related: session_connection.go -- reportWritePathHeld, which raises the event the session end leaves alone
// Related: session_write_budget_report_test.go -- TestSessionDroppedOwed, the same decision for a budget expiry

package reactor

import (
	"net/netip"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

// TestSessionDroppedNotOwedAfterHeldWritePath proves a session whose close met
// a held write path owes no session-dropped event: the close routine raised
// doctor-bgp-write-path-held for it. The close is the real routine, run behind
// a write path the test holds, and the cause is read the way the session end
// reads it (leaveCause), from a leave that recorded a hold timer expiry.
//
// VALIDATES: one close gives one event -- sessionDroppedOwed answers false for
// the cause of a session that reported a held write path, and still answers
// true for the same recorded reason on a session that reported none.
// PREVENTS: a hold expiry behind a stuck writer raising session-dropped beside
// doctor-bgp-write-path-held, two events for one close.
func TestSessionDroppedNotOwedAfterHeldWritePath(t *testing.T) {
	settings := NewPeerSettings(netip.MustParseAddr("192.0.2.1"), 65001, 65002, 0x01020301)
	session := NewSession(settings)
	client, _, ended := pipeSession(t, session)
	defer func() { _ = client.Close() }()

	require.True(t, sessionDroppedOwed(false, leaveCause(session, ErrHoldTimerExpired)),
		"precondition: a hold expiry with no NOTIFICATION owes session-dropped before any close reported a held path")

	release := holdWritePath(session)
	defer release()
	session.closeConnWithin(50 * time.Millisecond)
	requireConnClosed(t, ended)

	cause := leaveCause(session, ErrHoldTimerExpired)
	require.ErrorIs(t, cause, ErrWritePathHeld, "the session end must see that the close reported a held write path")
	require.ErrorIs(t, cause, ErrHoldTimerExpired, "the recorded close reason must stay readable")
	require.False(t, sessionDroppedOwed(false, cause),
		"a close that raised doctor-bgp-write-path-held must not raise session-dropped too")

	require.ErrorIs(t, leaveCause(session, nil), ErrWritePathHeld,
		"a leave that recorded no reason still names the held write path")
	require.False(t, sessionDroppedOwed(false, leaveCause(session, nil)))
}
