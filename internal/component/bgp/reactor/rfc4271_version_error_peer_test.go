// Design: docs/architecture/behavior/fsm-open-sent.md — the OpenSent EventNotifMsgVerErr row
// Related: docs/architecture/behavior/fsm-open-confirm.md — the OpenConfirm EventNotifMsgVerErr row
// RFC: rfc/short/rfc4271.md — Section 8.2.2, OpenSent and OpenConfirm states, Event 24

package reactor

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/bgp/fsm"
	"github.com/ze-software/ze/internal/component/bgp/message"
)

// versionErrorStart is the ConnectRetryCounter each test below starts from, so a
// counter left alone (3), incremented (4) and reset (0) are three different values.
const versionErrorStart = 3

// openErrorNotification packs a NOTIFICATION OPEN Message Error with subcode.
func openErrorNotification(subcode uint8) []byte {
	return message.PackTo(&message.Notification{ErrorCode: message.NotifyOpenMessage, ErrorSubcode: subcode}, nil)
}

// startNeighborAt drives a running Peer to state (OpenSent or OpenConfirm) and sets
// its ConnectRetryCounter to versionErrorStart.
func startNeighborAt(t *testing.T, state fsm.State) *openSentNeighbor {
	t.Helper()
	start := startOpenSentNeighbor
	if state == fsm.StateOpenConfirm {
		start = startOpenConfirmNeighbor
	}
	n := start(t)
	for range versionErrorStart {
		n.peer.connectRetryCounter.Increment()
	}
	require.Equal(t, state, n.session.State())
	return n
}

// TestRFC4271VersionErrorNotificationReleasesQuietly drives a running Peer into
// OpenSent, and into OpenConfirm, with its ConnectRetryCounter at 3, then sends it
// a NOTIFICATION OPEN Message Error / Unsupported Version Number (2/1): the
// "version error" NOTIFICATION RFC 4271 Section 8.1.5 names Event 24.
//
// VALIDATES: in both states the Peer writes no NOTIFICATION, drops the TCP
// connection (EOF at the far end), stops its HoldTimer, KeepaliveTimer and
// ConnectRetryTimer, releases the session, reaches Idle, and leaves the counter
// at 3.
// PREVENTS: a received 2/1 filed as Event 25 (until 2026-10-01 handleNotification
// fired EventNotifMsg for every NOTIFICATION), which in OpenSent answers the peer
// with a Finite State Machine Error and in both states counts a failed attempt.
//
// RFC requirement: RFC4271-8.2.2-26 positive -- a running Peer in OpenSent at ConnectRetryCounter 3 that receives NOTIFICATION 2/1 writes no NOTIFICATION, closes the TCP connection, stops its three session timers, drops the session, reaches Idle, and leaves the counter at 3.
// RFC requirement: RFC4271-8.2.2-27 positive -- a running Peer in OpenConfirm at ConnectRetryCounter 3 that receives NOTIFICATION 2/1 writes no NOTIFICATION, closes the TCP connection, stops its three session timers, drops the session, reaches Idle, and leaves the counter at 3.
// RFC requirement: RFC4271-8.2.2-24 negative -- a running Peer in OpenSent that receives NOTIFICATION 2/1 (Event 24, not in the "any other event" list) writes no NOTIFICATION 5/0, and its ConnectRetryCounter stays at 3.
func TestRFC4271VersionErrorNotificationReleasesQuietly(t *testing.T) {
	for _, state := range []fsm.State{fsm.StateOpenSent, fsm.StateOpenConfirm} {
		t.Run(state.String(), func(t *testing.T) {
			n := startNeighborAt(t, state)

			_, err := n.conn.Write(openErrorNotification(message.NotifyOpenUnsupportedVersion))
			require.NoError(t, err)

			requireReleased(t, n, [][2]byte(nil), versionErrorStart)
			assert.Equal(t, fsm.StateIdle, n.session.State(), "the FSM is Idle")
		})
	}
}

// TestRFC4271OtherOpenErrorNotificationIsNotAVersionError is the non-triggering
// input for the test above: the same running Peers receive NOTIFICATION OPEN
// Message Error / Bad Peer AS (2/2). Same error code, another subcode, so it is
// Event 25 and takes each state's Event 25 path.
//
// VALIDATES: in OpenSent the Peer answers with exactly one NOTIFICATION 5/0 and
// the counter moves from 3 to 4 (OpenSent's "any other event" list); in
// OpenConfirm the Peer writes no NOTIFICATION and the counter moves from 3 to 4
// (the Event 18 / Event 25 list). Both drop the connection and the session.
// PREVENTS: an Event 24 classifier that reads only the error code, so every OPEN
// Message Error would skip the counter.
//
// RFC requirement: RFC4271-8.2.2-26 negative -- a running Peer in OpenSent at ConnectRetryCounter 3 that receives NOTIFICATION 2/2 (not a version error) writes one NOTIFICATION 5/0, closes the connection, drops the session, and moves the counter to 4.
// RFC requirement: RFC4271-8.2.2-27 negative -- a running Peer in OpenConfirm at ConnectRetryCounter 3 that receives NOTIFICATION 2/2 (not a version error) writes no NOTIFICATION, closes the connection, drops the session, and moves the counter to 4.
func TestRFC4271OtherOpenErrorNotificationIsNotAVersionError(t *testing.T) {
	for _, tc := range []struct {
		state fsm.State
		want  [][2]byte
	}{
		{state: fsm.StateOpenSent, want: [][2]byte{fsmErrorNotification}},
		{state: fsm.StateOpenConfirm, want: [][2]byte(nil)},
	} {
		t.Run(tc.state.String(), func(t *testing.T) {
			n := startNeighborAt(t, tc.state)

			_, err := n.conn.Write(openErrorNotification(message.NotifyOpenBadPeerAS))
			require.NoError(t, err)

			requireReleased(t, n, tc.want, versionErrorStart+1)
		})
	}
}
