package l2tp

import (
	"log/slog"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

// buildICRPWire builds an ICRP body assigning the given session ID.
func icrpBody(assignedSID uint16) []byte {
	buf := make([]byte, 64)
	return buf[:writeICRPBody(buf, assignedSID)]
}

func ocrpBody(assignedSID uint16) []byte {
	buf := make([]byte, 64)
	return buf[:writeOCRPBody(buf, assignedSID)]
}

// sessionMsgWire wraps a session-message body in a control datagram whose
// header carries the given destination Session ID, Ns 0 (the peer's first
// message) and Nr 1 (acknowledging the tunnel's first message), so a
// tunnel-level test can feed it through Process (which runs the reliable
// engine's Nr processing before dispatching -- the ACK that opens the send
// window).
func sessionMsgWire(sid uint16, body []byte) []byte {
	pkt := make([]byte, ControlHeaderLen+len(body))
	WriteControlHeader(pkt, 0, uint16(ControlHeaderLen+len(body)), 0, sid, 0, 1)
	copy(pkt[ControlHeaderLen:], body)
	return pkt
}

// --- LAC incoming call: ICRQ -> ICRP -> ICCN -> established (AC-3) ---

func TestPlaceIncomingCall_Handshake(t *testing.T) {
	// VALIDATES: AC-3 -- placing an incoming call sends ICRQ and enters
	// wait-reply; the peer's ICRP drives ICCN and established with lnsMode
	// false and a kernel-setup request.
	tun := newEstablishedTunnel(t, 0)
	now := time.Now()
	logger := slog.Default()

	localSID, out := tun.placeIncomingCall(now, callParams{
		callSerial: 42, bearerType: 1, framingType: 1, txConnectSpeed: 1_000_000,
		calledNumber: "5551000", callingNumber: "5552000",
	}, logger)
	require.NotZero(t, localSID)
	require.Len(t, out, 1, "ICRQ emitted")

	sess := tun.lookupSession(localSID)
	require.NotNil(t, sess)
	require.Equal(t, L2TPSessionWaitReply, sess.state)
	require.False(t, sess.lnsMode)

	// Validate the ICRQ body on the wire.
	hdr, err := ParseMessageHeader(out[0].bytes)
	require.NoError(t, err)
	icrq, err := parseICRQ(out[0].bytes[hdr.PayloadOff:hdr.Length])
	require.NoError(t, err)
	require.EqualValues(t, localSID, icrq.assignedSessionID)
	require.EqualValues(t, 42, icrq.callSerialNumber)

	// Peer answers ICRP (assigning SID 707) via the reliable engine: Ns=0
	// (peer's first delivered message), Nr=1 (acks our ICRQ). Feeding it
	// through Process exercises the ACK that opens the send window for ICCN.
	icrpWire := sessionMsgWire(localSID, icrpBody(707))
	rhdr, err := ParseMessageHeader(icrpWire)
	require.NoError(t, err)
	out2 := tun.Process(rhdr, icrpWire[rhdr.PayloadOff:rhdr.Length], now, TunnelDefaults{}, nil)
	require.Len(t, out2, 1, "ICCN emitted")
	require.Equal(t, L2TPSessionEstablished, sess.state)
	require.EqualValues(t, 707, sess.remoteSID)
	require.True(t, sess.kernelSetupNeeded)
	require.False(t, sess.lnsMode, "LAC side => lnsMode false")

	shdr, err := ParseMessageHeader(out2[0].bytes)
	require.NoError(t, err)
	require.EqualValues(t, 707, shdr.SessionID, "ICCN header addresses the peer SID")
	iccn, err := parseICCN(out2[0].bytes[shdr.PayloadOff:shdr.Length])
	require.NoError(t, err)
	require.EqualValues(t, 1_000_000, iccn.txConnectSpeed)
}

func TestICRP_WrongState(t *testing.T) {
	// VALIDATES: an ICRP for a session not in wait-reply is dropped.
	tun := newEstablishedTunnel(t, 0)
	now := time.Now()
	logger := slog.Default()
	sess := &L2TPSession{localSID: 5, state: L2TPSessionEstablished, fsmHistory: newFSMHistoryRing()}
	tun.addSession(sess)
	out := tun.handleICRP(sess, icrpBody(9), now, logger)
	require.Nil(t, out)
	require.Equal(t, L2TPSessionEstablished, sess.state)
}

// malformedICRPBody is an ICRP body with a Message Type but a zero Assigned
// Session ID, which parseICRP refuses.
func malformedICRPBody() []byte {
	bad := make([]byte, 32)
	off := WriteAVPUint16(bad, 0, true, AVPMessageType, uint16(MsgICRP))
	off += WriteAVPUint16(bad, off, true, AVPAssignedSessionID, 0)
	return bad[:off]
}

// requireCDN parses one outbound datagram as a CDN and checks its Result Code
// and Assigned Session ID.
func requireCDN(t *testing.T, wire []byte, resultCode, assignedSID uint16) {
	t.Helper()
	hdr, err := ParseMessageHeader(wire)
	require.NoError(t, err)
	cdn, err := parseCDN(wire[hdr.PayloadOff:hdr.Length])
	require.NoError(t, err)
	require.Equal(t, resultCode, cdn.result, "CDN Result Code")
	require.Equal(t, assignedSID, cdn.assignedSessionID, "CDN Assigned Session ID")
}

func TestICRP_Malformed_SendsCDN(t *testing.T) {
	// VALIDATES: a malformed ICRP tears the session down with a CDN RC=2 that
	// leaves at once. The ICRP goes through Process with Nr=1, as a peer sends
	// it, so the reliable engine acknowledges our ICRQ and frees the one-packet
	// slow-start window (RFC 2661 Appendix A) before the FSM builds the CDN.
	tun := newEstablishedTunnel(t, 0)
	now := time.Now()
	logger := slog.Default()
	localSID, out := tun.placeIncomingCall(now, callParams{callSerial: 1, framingType: 1}, logger)
	require.Len(t, out, 1, "ICRQ emitted")
	require.NotNil(t, tun.lookupSession(localSID))

	wire := sessionMsgWire(localSID, malformedICRPBody())
	hdr, err := ParseMessageHeader(wire)
	require.NoError(t, err)
	out = tun.Process(hdr, wire[hdr.PayloadOff:hdr.Length], now, TunnelDefaults{}, nil)
	require.Len(t, out, 1, "CDN emitted, carrying the acknowledgement of the ICRP")
	requireCDN(t, out[0].bytes, cdnResultGeneralError, localSID)
	require.Nil(t, tun.lookupSession(localSID), "session removed after CDN")
}

func TestICRP_Malformed_FullWindowQueuesCDN(t *testing.T) {
	// VALIDATES: when the send window is full, the CDN for a malformed ICRP is
	// queued in the reliable engine rather than sent as an empty datagram, and
	// the acknowledgement that opens the window releases that CDN. Our ICRQ is
	// unacknowledged and CWND starts at one packet (RFC 2661 Appendix A), so
	// the window is full when the handler runs.
	tun := newEstablishedTunnel(t, 0)
	now := time.Now()
	logger := slog.Default()
	localSID, out := tun.placeIncomingCall(now, callParams{callSerial: 1, framingType: 1}, logger)
	require.Len(t, out, 1, "ICRQ emitted")
	sess := tun.lookupSession(localSID)
	require.NotNil(t, sess)
	queued := len(tun.engine.sendQueue)

	out = tun.handleICRP(sess, malformedICRPBody(), now, logger)
	require.Empty(t, out, "window full: nothing leaves now, and never an empty datagram")
	require.Len(t, tun.engine.sendQueue, queued+1, "CDN queued in the reliable engine")
	require.Nil(t, tun.lookupSession(localSID), "session removed after CDN")

	// The peer's ZLB acknowledges the ICRQ (Nr=1) and opens the window.
	zlb := sessionMsgWire(0, nil)
	zhdr, err := ParseMessageHeader(zlb)
	require.NoError(t, err)
	out = tun.Process(zhdr, zlb[zhdr.PayloadOff:zhdr.Length], now, TunnelDefaults{}, nil)
	require.Len(t, out, 1, "the queued CDN leaves once the window opens")
	requireCDN(t, out[0].bytes, cdnResultGeneralError, localSID)
	require.Len(t, tun.engine.sendQueue, queued, "send queue drained")
}

// --- LNS outgoing call: OCRQ -> OCRP -> OCCN -> established (AC-4) ---

func TestPlaceOutgoingCall_Handshake(t *testing.T) {
	// VALIDATES: AC-4 -- placing an outgoing call sends OCRQ and enters
	// wait-reply; OCRP moves to wait-connect; OCCN establishes with
	// lnsMode true (ze is the LNS end of the outgoing call).
	tun := newEstablishedTunnel(t, 0)
	now := time.Now()
	logger := slog.Default()

	localSID, out := tun.placeOutgoingCall(now, callParams{
		callSerial: 7, minBPS: 9600, maxBPS: 128000, bearerType: 1, framingType: 1,
		calledNumber: "5559999",
	}, logger)
	require.NotZero(t, localSID)
	require.Len(t, out, 1, "OCRQ emitted")

	sess := tun.lookupSession(localSID)
	require.NotNil(t, sess)
	require.Equal(t, L2TPSessionWaitReply, sess.state)
	require.True(t, sess.lnsMode, "LNS side => lnsMode true at origination")

	hdr, err := ParseMessageHeader(out[0].bytes)
	require.NoError(t, err)
	ocrq, err := parseOCRQ(out[0].bytes[hdr.PayloadOff:hdr.Length])
	require.NoError(t, err)
	require.EqualValues(t, localSID, ocrq.assignedSessionID)
	require.Equal(t, "5559999", ocrq.calledNumber)

	// Peer answers OCRP assigning SID 808.
	out2 := tun.handleOCRP(sess, ocrpBody(808), now, logger)
	require.Nil(t, out2, "OCRP produces no immediate send")
	require.Equal(t, L2TPSessionWaitConnect, sess.state)
	require.EqualValues(t, 808, sess.remoteSID)

	// Peer connects the call: OCCN establishes.
	tun.handleOCCN(sess, buildOCCN(64000, 1), now, logger)
	require.Equal(t, L2TPSessionEstablished, sess.state)
	require.True(t, sess.kernelSetupNeeded)
	require.True(t, sess.lnsMode, "LNS-outgoing keeps lnsMode true through OCCN")
}

func TestOCRP_WrongState(t *testing.T) {
	// VALIDATES: an OCRP for a session not in wait-reply is dropped.
	tun := newEstablishedTunnel(t, 0)
	now := time.Now()
	logger := slog.Default()
	sess := &L2TPSession{localSID: 5, state: L2TPSessionEstablished, fsmHistory: newFSMHistoryRing()}
	tun.addSession(sess)
	out := tun.handleOCRP(sess, ocrpBody(9), now, logger)
	require.Nil(t, out)
	require.Equal(t, L2TPSessionEstablished, sess.state)
}
