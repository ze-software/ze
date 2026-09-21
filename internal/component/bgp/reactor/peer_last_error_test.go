package reactor

import (
	"bufio"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/core/report"

	"github.com/ze-software/ze/internal/component/bgp/fsm"
	"github.com/ze-software/ze/internal/component/bgp/message"
	"github.com/ze-software/ze/internal/component/plugin"
)

// newNotifyPeer builds an Established peer whose session writes through a
// connection that accepts writesLeft writes and refuses every write after
// them. The callbacks are attached with Peer.attachStatsCallbacks, which is the
// call runOnce makes for every connection attempt, so these tests read the
// production wiring rather than a copy of it.
func newNotifyPeer(t *testing.T, writesLeft int) (*Peer, *Session, *failAfterConn) {
	t.Helper()

	settings := NewPeerSettings(mustParseAddr("192.0.2.1"), 65000, 65001, 0x01010101)
	peer := NewPeer(settings)

	session := NewSession(settings)
	require.NoError(t, session.fsm.Event(fsm.EventManualStart))
	require.NoError(t, session.fsm.Event(fsm.EventTCPConnectionConfirmed))
	require.NoError(t, session.fsm.Event(fsm.EventBGPOpen))
	require.NoError(t, session.fsm.Event(fsm.EventKeepaliveMsg))
	require.Equal(t, fsm.StateEstablished, session.fsm.State())

	conn := &failAfterConn{writesLeft: writesLeft}
	session.mu.Lock()
	session.conn = conn
	session.bufWriter = bufio.NewWriterSize(conn, 4096)
	session.mu.Unlock()

	peer.mu.Lock()
	peer.session = session
	peer.mu.Unlock()

	peer.attachStatsCallbacks(session)
	return peer, session, conn
}

// TestNotificationRefusedBySocketStillRecordsTheReason drives a BFD Down Cease
// into a socket that refuses it, which is what a total loss of connectivity
// leaves behind, and reads the peer's operational state afterwards.
//
// VALIDATES: a NOTIFICATION ze could not write records its code, subcode and
// time under direction send-failed, so `show bgp peer` says WHY the session
// ended.
// PREVENTS: the blank last-error a torn-down peer used to report, which is the
// answer a healthy peer gives.
//
// RFC requirement: RFC9384-4-1 positive -- the Cease NOTIFICATION could not be
// sent, and the reason is still part of the peer's operational state: Stats
// answers code 6, subcode 10 and a non-zero time
// (internal/component/bgp/reactor/session_write.go, sendNotificationWithin,
// calls onNotifSent for a refused write too;
// internal/component/bgp/reactor/peer_stats.go, recordNotificationUnsent).
func TestNotificationRefusedBySocketStillRecordsTheReason(t *testing.T) {
	report.ResetForTest()
	defer report.ResetForTest()

	peer, session, conn := newNotifyPeer(t, 0)

	err := session.sendNotification(conn, message.NotifyCease, message.NotifyCeaseBFDDown, nil)
	require.Error(t, err, "the fixture connection refuses every write")

	stats := peer.Stats()
	assert.Equal(t, plugin.NotifSendFailed, stats.LastNotifDirection)
	assert.Equal(t, uint8(message.NotifyCease), stats.LastNotifCode)
	assert.Equal(t, message.NotifyCeaseBFDDown, stats.LastNotifSubcode)
	assert.False(t, stats.LastNotifTime.IsZero())

	// Nothing reached the peer, so nothing counts as sent, and the flag that
	// suppresses session-dropped stays false: the wire really did go away and
	// that event is the one an operator watches for it.
	assert.Equal(t, uint32(0), stats.NotificationsSent)
	assert.False(t, peer.notificationExchanged.Load(),
		"a NOTIFICATION nobody received is not an exchange")
}

// TestNotificationDeliveredIsNotRecordedAsUnsent is the other polarity: the
// send-failed state is reached by a failed write and by nothing else.
//
// VALIDATES: a peer with no teardown records no reason at all, and a
// NOTIFICATION the socket accepted records direction sent.
// PREVENTS: a last-error that reports every teardown as undelivered, which
// would tell an operator the peer was never told when it was.
//
// RFC requirement: RFC9384-4-1 negative -- operational state carries the
// "could not be sent" reason only where the write actually failed: a peer that
// has torn down nothing answers plugin.NotifNone, and a delivered Cease
// answers plugin.NotifSent with the sent counter raised
// (internal/component/bgp/reactor/peer_stats.go, recordNotificationSend).
func TestNotificationDeliveredIsNotRecordedAsUnsent(t *testing.T) {
	report.ResetForTest()
	defer report.ResetForTest()

	peer, session, conn := newNotifyPeer(t, 4)

	stats := peer.Stats()
	require.Equal(t, plugin.NotifNone, stats.LastNotifDirection,
		"a peer that never tore down has no reason to report")
	require.True(t, stats.LastNotifTime.IsZero())

	require.NoError(t, session.sendNotification(conn, message.NotifyCease, message.NotifyCeaseBFDDown, nil))

	stats = peer.Stats()
	assert.Equal(t, plugin.NotifSent, stats.LastNotifDirection)
	assert.Equal(t, uint8(message.NotifyCease), stats.LastNotifCode)
	assert.Equal(t, message.NotifyCeaseBFDDown, stats.LastNotifSubcode)
	assert.Equal(t, uint32(1), stats.NotificationsSent)
	assert.True(t, peer.notificationExchanged.Load())
}
