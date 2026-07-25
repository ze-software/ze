package l2tp

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

// buildICRQDatagram wraps an ICRQ body in a control header. Header
// SessionID is 0 -- ICRQ creates the session, so the peer has no local
// SID to address yet (RFC 2661 S10.2).
func buildICRQDatagram(destTID, ns, nr, assignedSID uint16, callSerial uint32) []byte {
	body := buildICRQ(assignedSID, callSerial)
	pkt := make([]byte, ControlHeaderLen+len(body))
	WriteControlHeader(pkt, 0, uint16(ControlHeaderLen+len(body)), destTID, 0, ns, nr) //nolint:gosec // fixed small body
	copy(pkt[ControlHeaderLen:], body)
	return pkt
}

// buildICCNDatagram wraps an ICCN body in a control header addressed to
// sessionID -- the Assigned Session ID ze returned in its ICRP.
func buildICCNDatagram(destTID, sessionID, ns, nr uint16, txSpeed, framingType uint32) []byte {
	body := buildICCN(txSpeed, framingType)
	pkt := make([]byte, ControlHeaderLen+len(body))
	WriteControlHeader(pkt, 0, uint16(ControlHeaderLen+len(body)), destTID, sessionID, ns, nr) //nolint:gosec // fixed small body
	copy(pkt[ControlHeaderLen:], body)
	return pkt
}

// readCtrlOfType reads datagrams until one carries the wanted message
// type, mirroring the .ci peer's recv_ctrl: ZLB ACKs (12-byte, empty
// body) are skipped because they interleave with every FSM reply.
func readCtrlOfType(t *testing.T, c *testClient, want MessageType) []byte {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if err := c.conn.SetReadDeadline(deadline); err != nil {
			t.Fatalf("set read deadline: %v", err)
		}
		rbuf := make([]byte, 1500)
		n, _, err := c.conn.ReadFromUDP(rbuf)
		if err != nil {
			t.Fatalf("waiting for message type %d: %v", uint16(want), err)
		}
		if n <= ControlHeaderLen {
			continue // ZLB ACK
		}
		if extractMsgType(rbuf[ControlHeaderLen:n]) == want {
			return rbuf[:n]
		}
	}
	t.Fatalf("timed out waiting for message type %d", uint16(want))
	return nil
}

// TestSession_StopCCNCascadeThroughEngine -- AC-9 through the full
// reliable-delivery path.
//
// VALIDATES: AC-9 -- a StopCCN that arrives after two sessions have been
// established over the reliable engine clears BOTH sessions, logging
// "StopCCN clearing sessions" with count=2. The existing
// TestSession_StopCCN_CascadeSessions calls handleICRQ/handleStopCCN
// directly and so never exercises reliable-engine sequencing, reorder
// buffering, or the reactor's SessionID plumbing.
//
// PREVENTS: regression of the deterministic
// test/l2tp/session-stopccn-cascade.ci failure, where the peer's ICCNs
// never reached their sessions and the tunnel therefore held zero
// established sessions when the StopCCN arrived.
func TestSession_StopCCNCascadeThroughEngine(t *testing.T) {
	ln, r, logs, stop := buildLogReactor(t)
	defer stop()

	client, localTID := driveToEstablished(t, ln, "")
	defer client.Close()
	waitForLog(t, logs, "tunnel now established")

	// Session 1: ICRQ (Ns=2) -> ICRP -> ICCN (Ns=3).
	client.Send(t, buildICRQDatagram(localTID, 2, 1, 500, 1001))
	icrp1 := readCtrlOfType(t, client, MsgICRP)
	sid1Bytes := extractAVP(t, icrp1, AVPAssignedSessionID)
	require.Len(t, sid1Bytes, 2, "ICRP must carry an Assigned Session ID")
	zeSID1 := uint16(sid1Bytes[0])<<8 | uint16(sid1Bytes[1])
	client.Send(t, buildICCNDatagram(localTID, zeSID1, 3, 2, 10000000, 2))

	// Session 2: ICRQ (Ns=4) -> ICRP -> ICCN (Ns=5).
	client.Send(t, buildICRQDatagram(localTID, 4, 2, 501, 1002))
	icrp2 := readCtrlOfType(t, client, MsgICRP)
	sid2Bytes := extractAVP(t, icrp2, AVPAssignedSessionID)
	require.Len(t, sid2Bytes, 2, "ICRP must carry an Assigned Session ID")
	zeSID2 := uint16(sid2Bytes[0])<<8 | uint16(sid2Bytes[1])
	client.Send(t, buildICCNDatagram(localTID, zeSID2, 5, 3, 56000, 1))

	waitForLog(t, logs, "session established (incoming LNS)")

	// Both ICCNs must have reached their sessions: the tunnel holds two
	// ESTABLISHED sessions before the StopCCN arrives.
	require.Eventually(t, func() bool {
		r.tunnelsMu.Lock()
		defer r.tunnelsMu.Unlock()
		tunnel := r.tunnelsByLocalID[localTID]
		if tunnel == nil {
			return false
		}
		established := 0
		for _, s := range tunnel.sessions {
			if s.state == L2TPSessionEstablished {
				established++
			}
		}
		return established == 2
	}, 2*time.Second, 2*time.Millisecond, "both sessions must be established before StopCCN")

	// StopCCN (Ns=6) tears down the tunnel and MUST cascade to both.
	client.Send(t, buildStopCCN(t, localTID, 6, 3, 42, 1))
	waitForLog(t, logs, "peer StopCCN received; tunnel closed")

	require.Contains(t, logs.String(), "StopCCN clearing sessions",
		"StopCCN must cascade to the tunnel's active sessions")
	require.Contains(t, logs.String(), "count=2",
		"StopCCN must clear BOTH sessions")

	r.tunnelsMu.Lock()
	tunnel := r.tunnelsByLocalID[localTID]
	require.NotNil(t, tunnel)
	remaining := len(tunnel.sessions)
	state := tunnel.state
	r.tunnelsMu.Unlock()
	require.Equal(t, 0, remaining, "no session may survive StopCCN")
	require.Equal(t, L2TPTunnelClosed, state)
}
