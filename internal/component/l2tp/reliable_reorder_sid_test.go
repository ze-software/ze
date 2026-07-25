package l2tp

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

// TestReliable_ReorderQueuePreservesSessionID -- RFC 2661 S5.8 in-order
// delivery must not corrupt the message it delivers.
//
// VALIDATES: a control message buffered by the reorder queue is delivered
// with the Session ID from ITS OWN header. The FSM keys session dispatch
// on RecvEntry.SessionID (tunnel_fsm.go handleMessage ->
// dispatchToSession), so a wrong value silently misroutes the message.
//
// PREVENTS: regression of the reorder-queue Session ID loss --
// reorderEntry carried only (ns, payload), so the gap-fill delivery
// hard-coded SessionID 0. Session ID 0 is reserved and never allocated
// (session.go allocateSessionID), so lookupSession(0) always missed and
// every gap-filled ICCN/ICRP/OCCN/OCRP/CDN/WEN/SLI was dropped as
// "session-scoped message for unknown SID".
func TestReliable_ReorderQueuePreservesSessionID(t *testing.T) {
	e := NewReliableEngine(ReliableConfig{LocalTunnelID: 100, PeerTunnelID: 200, RecvWindow: 8})
	now := time.Now()

	// Ns=1 arrives first (gap at Ns=0) and is buffered. Its header
	// addresses session 0x1234.
	outOfOrder := MessageHeader{IsControl: true, TunnelID: 100, SessionID: 0x1234, Ns: 1, Nr: 0}
	res := e.OnReceive(outOfOrder, buildICCN(10000000, 2), now)
	require.Equal(t, ClassReorderQueued, res.Class, "Ns=1 with nextRecvSeq=0 must be buffered")

	// Ns=0 fills the gap; both messages are delivered in order.
	inOrder := MessageHeader{IsControl: true, TunnelID: 100, SessionID: 0, Ns: 0, Nr: 0}
	res = e.OnReceive(inOrder, buildICRQ(500, 1001), now)
	require.Equal(t, ClassDelivered, res.Class)
	require.Len(t, res.Delivered, 2, "gap fill must deliver both messages")

	require.Equal(t, uint16(0), res.Delivered[0].SessionID, "in-order ICRQ addresses session 0")
	require.Equal(t, uint16(0x1234), res.Delivered[1].SessionID,
		"gap-filled message must keep the Session ID from its own header")
}

// TestReliable_ReorderQueuePreservesSessionIDMultiGap covers a two-message
// gap: each buffered entry must keep its own Session ID, not the first
// one's and not the gap-filler's.
//
// VALIDATES: per-entry Session ID fidelity across a multi-message flush.
// PREVENTS: a fix that threads one Session ID through the whole flush.
func TestReliable_ReorderQueuePreservesSessionIDMultiGap(t *testing.T) {
	e := NewReliableEngine(ReliableConfig{LocalTunnelID: 100, PeerTunnelID: 200, RecvWindow: 8})
	now := time.Now()

	res := e.OnReceive(MessageHeader{IsControl: true, TunnelID: 100, SessionID: 0xAAAA, Ns: 2}, buildICCN(1, 2), now)
	require.Equal(t, ClassReorderQueued, res.Class)
	res = e.OnReceive(MessageHeader{IsControl: true, TunnelID: 100, SessionID: 0xBBBB, Ns: 1}, buildICCN(2, 2), now)
	require.Equal(t, ClassReorderQueued, res.Class)

	res = e.OnReceive(MessageHeader{IsControl: true, TunnelID: 100, SessionID: 0xCCCC, Ns: 0}, buildICRQ(500, 1001), now)
	require.Equal(t, ClassDelivered, res.Class)
	require.Len(t, res.Delivered, 3)
	require.Equal(t, uint16(0xCCCC), res.Delivered[0].SessionID)
	require.Equal(t, uint16(0xBBBB), res.Delivered[1].SessionID, "Ns=1 must keep its own Session ID")
	require.Equal(t, uint16(0xAAAA), res.Delivered[2].SessionID, "Ns=2 must keep its own Session ID")
}

// TestSession_GapFilledSLIReachesItsSession -- the user-visible
// consequence of the reorder-queue Session ID.
//
// VALIDATES: when a peer's SLI overtakes its ICCN on the wire, the
// reliable engine buffers the SLI, the ICCN establishes the session, and
// the gap-filled SLI still reaches THAT session and applies its ACCM.
//
// PREVENTS: a peer whose session-scoped message is reordered by the
// network having it silently dropped as "unknown SID" -- ze would report
// a healthy session while ignoring the link parameters the peer set.
func TestSession_GapFilledSLIReachesItsSession(t *testing.T) {
	tun := newEstablishedTunnel(t, 0)
	now := time.Now()
	defaults := TunnelDefaults{HostName: "ze-test", FramingCapabilities: 0x3, RecvWindow: 16}

	// Ns=0: ICRQ in order -> ze allocates a local Session ID and sends ICRP.
	out := tun.Process(MessageHeader{IsControl: true, TunnelID: 100, SessionID: 0, Ns: 0},
		buildICRQ(500, 1001), now, defaults, nil)
	require.NotEmpty(t, out, "ICRQ must produce an ICRP")
	require.Equal(t, 1, tun.sessionCount())
	var localSID uint16
	for _, s := range tun.sessions {
		localSID = s.localSID
	}
	require.NotZero(t, localSID, "Session ID 0 is reserved and must never be allocated")

	// Ns=2: SLI overtakes the ICCN and is buffered by the reorder queue.
	accm := ACCMValue{SendACCM: 0x000A0000, RecvACCM: 0x000B0000}
	tun.Process(MessageHeader{IsControl: true, TunnelID: 100, SessionID: localSID, Ns: 2},
		buildSLI(accm), now, defaults, nil)

	// Ns=1: the ICCN fills the gap. The engine delivers ICCN (establishing
	// the session) and then flushes the buffered SLI.
	tun.Process(MessageHeader{IsControl: true, TunnelID: 100, SessionID: localSID, Ns: 1},
		buildICCN(10000000, 2), now, defaults, nil)

	sess := tun.lookupSession(localSID)
	require.NotNil(t, sess)
	require.Equal(t, L2TPSessionEstablished, sess.state, "in-order ICCN must establish the session")
	require.Equal(t, accm, sess.accm,
		"the gap-filled SLI must reach its own session, not session 0")
}
