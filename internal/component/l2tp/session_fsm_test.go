package l2tp

import (
	"log/slog"
	"net/netip"
	"testing"
	"time"
)

// newEstablishedTunnel creates a tunnel in the established state with a
// wired reliable engine, suitable for session FSM tests.
func newEstablishedTunnel(t *testing.T, maxSessions uint16) *L2TPTunnel {
	t.Helper()
	logger := slog.Default()
	tun := newTunnel(100, 200, netip.MustParseAddrPort("10.0.0.1:1701"),
		ReliableConfig{MaxRetransmit: 5, RTimeout: time.Second, RTimeoutCap: 8 * time.Second, RecvWindow: 4},
		logger)
	tun.state = L2TPTunnelEstablished
	tun.maxSessions = maxSessions
	return tun
}

// buildICRQ builds a minimal valid ICRQ body with the given session ID.
func buildICRQ(assignedSID uint16, callSerial uint32) []byte {
	var buf [128]byte
	off := 0
	off += WriteAVPUint16(buf[:], off, true, AVPMessageType, uint16(MsgICRQ))
	off += WriteAVPUint16(buf[:], off, true, AVPAssignedSessionID, assignedSID)
	off += WriteAVPUint32(buf[:], off, true, AVPCallSerialNumber, callSerial)
	return buf[:off]
}

// buildICCN builds a minimal valid ICCN body.
func buildICCN(txSpeed, framingType uint32) []byte {
	var buf [128]byte
	off := 0
	off += WriteAVPUint16(buf[:], off, true, AVPMessageType, uint16(MsgICCN))
	off += WriteAVPUint32(buf[:], off, true, AVPTxConnectSpeed, txSpeed)
	off += WriteAVPUint32(buf[:], off, true, AVPFramingType, framingType)
	return buf[:off]
}

// buildICCNWithProxy builds an ICCN body with proxy LCP and auth AVPs.
func buildICCNWithProxy(txSpeed, framingType uint32) []byte {
	var buf [512]byte
	off := 0
	off += WriteAVPUint16(buf[:], off, true, AVPMessageType, uint16(MsgICCN))
	off += WriteAVPUint32(buf[:], off, true, AVPTxConnectSpeed, txSpeed)
	off += WriteAVPUint32(buf[:], off, true, AVPFramingType, framingType)
	// AC-19: Sequencing Required.
	off += WriteAVPEmpty(buf[:], off, true, 0, AVPSequencingRequired)
	// AC-17: Proxy LCP.
	off += WriteAVPBytes(buf[:], off, false, 0, AVPInitialReceivedLCPConfReq, []byte{0x01, 0x02})
	off += WriteAVPBytes(buf[:], off, false, 0, AVPLastSentLCPConfReq, []byte{0x03, 0x04})
	off += WriteAVPBytes(buf[:], off, false, 0, AVPLastReceivedLCPConfReq, []byte{0x05, 0x06})
	// AC-18: Proxy Auth.
	off += WriteAVPUint16(buf[:], off, false, AVPProxyAuthenType, 2) // CHAP
	off += WriteAVPString(buf[:], off, false, AVPProxyAuthenName, "user1")
	off += WriteAVPBytes(buf[:], off, false, 0, AVPProxyAuthenChallenge, []byte{0xAA, 0xBB})
	off += WriteAVPProxyAuthenID(buf[:], off, false, ProxyAuthenIDValue{ChapID: 42})
	off += WriteAVPBytes(buf[:], off, false, 0, AVPProxyAuthenResponse, []byte{0xCC, 0xDD})
	return buf[:off]
}

// buildOCRQ builds a minimal valid OCRQ body.
func buildOCRQ(assignedSID uint16, callSerial uint32) []byte {
	var buf [256]byte
	off := 0
	off += WriteAVPUint16(buf[:], off, true, AVPMessageType, uint16(MsgOCRQ))
	off += WriteAVPUint16(buf[:], off, true, AVPAssignedSessionID, assignedSID)
	off += WriteAVPUint32(buf[:], off, true, AVPCallSerialNumber, callSerial)
	off += WriteAVPUint32(buf[:], off, true, AVPMinimumBPS, 9600)
	off += WriteAVPUint32(buf[:], off, true, AVPMaximumBPS, 56000)
	off += WriteAVPUint32(buf[:], off, true, AVPBearerType, 1)
	off += WriteAVPUint32(buf[:], off, true, AVPFramingType, 1)
	off += WriteAVPString(buf[:], off, true, AVPCalledNumber, "5551234")
	return buf[:off]
}

// buildCDN builds a CDN body with Result Code and Assigned Session ID.
func buildCDN(resultCode, assignedSID uint16) []byte {
	var buf [128]byte
	off := 0
	off += WriteAVPUint16(buf[:], off, true, AVPMessageType, uint16(MsgCDN))
	off += WriteAVPResultCode(buf[:], off, true, ResultCodeValue{Result: resultCode})
	off += WriteAVPUint16(buf[:], off, true, AVPAssignedSessionID, assignedSID)
	return buf[:off]
}

// buildWEN builds a WEN body with Call Errors AVP.
func buildWEN(ce CallErrorsValue) []byte {
	var buf [128]byte
	off := 0
	off += WriteAVPUint16(buf[:], off, true, AVPMessageType, uint16(MsgWEN))
	off += WriteAVPCallErrors(buf[:], off, true, ce)
	return buf[:off]
}

// buildSLI builds an SLI body with ACCM AVP.
func buildSLI(accm ACCMValue) []byte {
	var buf [128]byte
	off := 0
	off += WriteAVPUint16(buf[:], off, true, AVPMessageType, uint16(MsgSLI))
	off += WriteAVPACCM(buf[:], off, true, accm)
	return buf[:off]
}

// buildOCCN builds a minimal valid OCCN body.
func buildOCCN(txSpeed, framingType uint32) []byte {
	var buf [128]byte
	off := 0
	off += WriteAVPUint16(buf[:], off, true, AVPMessageType, uint16(MsgOCCN))
	off += WriteAVPUint32(buf[:], off, true, AVPTxConnectSpeed, txSpeed)
	off += WriteAVPUint32(buf[:], off, true, AVPFramingType, framingType)
	return buf[:off]
}

// --- AC-1: ICRQ -> ICRP full handshake start ---

func TestSession_IncomingLNS_ICRQ(t *testing.T) {
	// VALIDATES: AC-1 -- LAC sends ICRQ; ze sends ICRP with Assigned Session ID.
	tun := newEstablishedTunnel(t, 0)
	now := time.Now()
	logger := slog.Default()
	payload := buildICRQ(500, 1001)

	out := tun.handleICRQ(payload, now, logger)
	if len(out) != 1 {
		t.Fatalf("expected 1 send request (ICRP), got %d", len(out))
	}
	if tun.sessionCount() != 1 {
		t.Fatalf("expected 1 session, got %d", tun.sessionCount())
	}
	// Find the session and check state.
	var sess *L2TPSession
	for _, s := range tun.sessions {
		sess = s
	}
	if sess.state != L2TPSessionWaitConnect {
		t.Fatalf("expected session state wait-connect, got %s", sess.state)
	}
	if sess.remoteSID != 500 {
		t.Fatalf("expected remote SID 500, got %d", sess.remoteSID)
	}
}

// --- AC-2: ICRQ -> ICRP -> ICCN full handshake ---

func TestSession_IncomingLNS_FullHandshake(t *testing.T) {
	// VALIDATES: AC-1, AC-2 -- full ICRQ/ICRP/ICCN sequence.
	tun := newEstablishedTunnel(t, 0)
	now := time.Now()
	logger := slog.Default()

	// Step 1: ICRQ.
	icrqPayload := buildICRQ(500, 1001)
	out := tun.handleICRQ(icrqPayload, now, logger)
	if len(out) != 1 {
		t.Fatalf("ICRQ: expected 1 send (ICRP), got %d", len(out))
	}

	// Find session.
	var sess *L2TPSession
	for _, s := range tun.sessions {
		sess = s
	}
	if sess == nil {
		t.Fatal("no session created")
	}
	localSID := sess.localSID

	// Step 2: ICCN.
	iccnPayload := buildICCN(10000000, 0x00000002) // 10Mbps, sync framing
	out = tun.handleICCN(sess, iccnPayload, now, logger)
	if len(out) != 0 {
		t.Fatalf("ICCN: expected 0 sends, got %d", len(out))
	}
	if sess.state != L2TPSessionEstablished {
		t.Fatalf("expected established, got %s", sess.state)
	}
	if sess.txConnectSpeed != 10000000 {
		t.Fatalf("expected tx speed 10000000, got %d", sess.txConnectSpeed)
	}
	// Verify session still in map.
	if tun.lookupSession(localSID) == nil {
		t.Fatal("session missing from tunnel map after ICCN")
	}
}

// --- AC-3: ICCN with missing required AVP ---

func TestSession_IncomingLNS_ICCNMissingTxSpeed(t *testing.T) {
	// VALIDATES: AC-3 -- ICCN missing Tx Connect Speed -> CDN.
	tun := newEstablishedTunnel(t, 0)
	now := time.Now()
	logger := slog.Default()

	// Create session via ICRQ.
	icrqPayload := buildICRQ(500, 1001)
	tun.handleICRQ(icrqPayload, now, logger)
	var sess *L2TPSession
	for _, s := range tun.sessions {
		sess = s
	}

	// ICCN with only Message Type (missing Tx Connect Speed and Framing Type).
	var buf [32]byte
	n := WriteAVPUint16(buf[:], 0, true, AVPMessageType, uint16(MsgICCN))
	badICCN := buf[:n]

	out := tun.handleICCN(sess, badICCN, now, logger)
	if len(out) != 1 {
		t.Fatalf("expected 1 send (CDN), got %d", len(out))
	}
	if tun.sessionCount() != 0 {
		t.Fatalf("expected session removed, got %d sessions", tun.sessionCount())
	}
}

// --- AC-4: ICRQ on non-established tunnel ---

func TestSession_IncomingLNS_NonEstablishedTunnel(t *testing.T) {
	// VALIDATES: AC-4 -- ICRQ on wait-ctl-conn tunnel dropped.
	tun := newEstablishedTunnel(t, 0)
	tun.state = L2TPTunnelWaitCtlConn // override
	now := time.Now()
	logger := slog.Default()

	out := tun.handleICRQ(buildICRQ(500, 1001), now, logger)
	if len(out) != 0 {
		t.Fatalf("expected 0 sends (dropped), got %d", len(out))
	}
	if tun.sessionCount() != 0 {
		t.Fatalf("expected 0 sessions, got %d", tun.sessionCount())
	}
}

// --- AC-5: max sessions reached ---

func TestSession_MaxSessionsEnforced(t *testing.T) {
	// VALIDATES: AC-5 -- max-sessions limit -> CDN RC=4.
	tun := newEstablishedTunnel(t, 1) // max 1 session
	now := time.Now()
	logger := slog.Default()

	// First ICRQ succeeds.
	tun.handleICRQ(buildICRQ(500, 1001), now, logger)
	if tun.sessionCount() != 1 {
		t.Fatalf("expected 1 session, got %d", tun.sessionCount())
	}

	// Second ICRQ rejected.
	out := tun.handleICRQ(buildICRQ(501, 1002), now, logger)
	if len(out) != 1 {
		t.Fatalf("expected 1 send (CDN), got %d", len(out))
	}
	if tun.sessionCount() != 1 {
		t.Fatalf("expected still 1 session, got %d", tun.sessionCount())
	}
}

// --- AC-6: Assigned Session ID = 0 ---

func TestSession_IncomingLNS_ICCNVariedSpeed(t *testing.T) {
	// VALIDATES: AC-2 -- ICCN with non-default tx speed captured.
	tun := newEstablishedTunnel(t, 0)
	now := time.Now()
	logger := slog.Default()

	tun.handleICRQ(buildICRQ(500, 1001), now, logger)
	var sess *L2TPSession
	for _, s := range tun.sessions {
		sess = s
	}
	tun.handleICCN(sess, buildICCN(56000, 1), now, logger) // 56kbps, async
	if sess.txConnectSpeed != 56000 {
		t.Fatalf("expected tx speed 56000, got %d", sess.txConnectSpeed)
	}
	if sess.framingType != 1 {
		t.Fatalf("expected framing 1 (async), got %d", sess.framingType)
	}
}

func TestSession_ICRQAssignedSIDZero(t *testing.T) {
	// VALIDATES: AC-6 -- Assigned Session ID = 0 -> CDN RC=2.
	tun := newEstablishedTunnel(t, 0)
	now := time.Now()
	logger := slog.Default()

	// ICRQ with SID=0 in the body (parseICRQ rejects it).
	payload := buildICRQ(0, 1001) // SID=0
	out := tun.handleICRQ(payload, now, logger)
	// parseICRQ returns error "missing Assigned Session ID AVP" for SID=0.
	if len(out) != 1 {
		t.Fatalf("expected 1 send (CDN), got %d", len(out))
	}
	if tun.sessionCount() != 0 {
		t.Fatalf("expected 0 sessions, got %d", tun.sessionCount())
	}
}

// --- AC-7, AC-8: CDN ---

func TestSession_CDN_EstablishedSession(t *testing.T) {
	// VALIDATES: AC-7 -- CDN destroys established session.
	tun := newEstablishedTunnel(t, 0)
	now := time.Now()
	logger := slog.Default()

	// Create + establish session.
	tun.handleICRQ(buildICRQ(500, 1001), now, logger)
	var sess *L2TPSession
	for _, s := range tun.sessions {
		sess = s
	}
	tun.handleICCN(sess, buildICCN(10000000, 2), now, logger)
	if sess.state != L2TPSessionEstablished {
		t.Fatal("session not established")
	}

	cdnPayload := buildCDN(1, 500)
	out := tun.handleCDN(sess.localSID, cdnPayload, logger)
	if len(out) != 0 {
		t.Fatalf("expected 0 sends (CDN is inbound), got %d", len(out))
	}
	if tun.sessionCount() != 0 {
		t.Fatalf("expected 0 sessions after CDN, got %d", tun.sessionCount())
	}
}

func TestSession_CDN_AnyState(t *testing.T) {
	// VALIDATES: AC-8 -- CDN valid in wait-connect state too.
	tun := newEstablishedTunnel(t, 0)
	now := time.Now()
	logger := slog.Default()

	tun.handleICRQ(buildICRQ(500, 1001), now, logger)
	var sess *L2TPSession
	for _, s := range tun.sessions {
		sess = s
	}
	if sess.state != L2TPSessionWaitConnect {
		t.Fatal("expected wait-connect")
	}

	cdnPayload := buildCDN(3, 500) // admin close
	tun.handleCDN(sess.localSID, cdnPayload, logger)
	if tun.sessionCount() != 0 {
		t.Fatalf("expected 0 sessions after CDN in wait-connect, got %d", tun.sessionCount())
	}
}

// --- AC-9: StopCCN cascades to sessions ---

func TestSession_StopCCN_CascadeSessions(t *testing.T) {
	// VALIDATES: AC-9 -- StopCCN clears all sessions.
	tun := newEstablishedTunnel(t, 0)
	now := time.Now()
	logger := slog.Default()

	// Create two sessions.
	tun.handleICRQ(buildICRQ(500, 1001), now, logger)
	tun.handleICRQ(buildICRQ(501, 1002), now, logger)
	if tun.sessionCount() != 2 {
		t.Fatalf("expected 2 sessions, got %d", tun.sessionCount())
	}

	// Build StopCCN.
	var buf [64]byte
	off := 0
	off += WriteAVPUint16(buf[:], off, true, AVPMessageType, uint16(MsgStopCCN))
	off += WriteAVPUint16(buf[:], off, true, AVPAssignedTunnelID, 200)
	off += WriteAVPResultCode(buf[:], off, true, ResultCodeValue{Result: 1})
	stopCCN := buf[:off]

	tun.handleStopCCN(now, stopCCN)
	if tun.sessionCount() != 0 {
		t.Fatalf("expected 0 sessions after StopCCN, got %d", tun.sessionCount())
	}
	if tun.state != L2TPTunnelClosed {
		t.Fatalf("expected tunnel closed, got %s", tun.state)
	}
}

// --- AC-10: WEN ---

func TestSession_WEN_CallErrors(t *testing.T) {
	// VALIDATES: AC-10 -- WEN captured on session.
	tun := newEstablishedTunnel(t, 0)
	now := time.Now()
	logger := slog.Default()

	tun.handleICRQ(buildICRQ(500, 1001), now, logger)
	var sess *L2TPSession
	for _, s := range tun.sessions {
		sess = s
	}
	tun.handleICCN(sess, buildICCN(10000000, 2), now, logger)

	ce := CallErrorsValue{CRCErrors: 42, FramingErrors: 7}
	wenPayload := buildWEN(ce)
	tun.handleWEN(sess.localSID, wenPayload, logger)

	if sess.callErrors.CRCErrors != 42 {
		t.Fatalf("expected CRC errors 42, got %d", sess.callErrors.CRCErrors)
	}
	if sess.callErrors.FramingErrors != 7 {
		t.Fatalf("expected framing errors 7, got %d", sess.callErrors.FramingErrors)
	}
}

// --- AC-11: SLI ---

func TestSession_SLI_ACCM(t *testing.T) {
	// VALIDATES: AC-11 -- SLI captured on session.
	tun := newEstablishedTunnel(t, 0)
	now := time.Now()
	logger := slog.Default()

	tun.handleICRQ(buildICRQ(500, 1001), now, logger)
	var sess *L2TPSession
	for _, s := range tun.sessions {
		sess = s
	}
	tun.handleICCN(sess, buildICCN(10000000, 2), now, logger)

	accm := ACCMValue{SendACCM: 0x000A0000, RecvACCM: 0xFFFFFFFF}
	sliPayload := buildSLI(accm)
	tun.handleSLI(sess.localSID, sliPayload, logger)

	if sess.accm.SendACCM != 0x000A0000 {
		t.Fatalf("expected send ACCM 0x000A0000, got 0x%08x", sess.accm.SendACCM)
	}
	if sess.accm.RecvACCM != 0xFFFFFFFF {
		t.Fatalf("expected recv ACCM 0xFFFFFFFF, got 0x%08x", sess.accm.RecvACCM)
	}
}

// --- AC-12, AC-13: Outgoing Call LAC side ---

func TestSession_OutgoingLAC_OCRQ(t *testing.T) {
	// VALIDATES: AC-12 -- OCRQ -> OCRP with Assigned SID.
	tun := newEstablishedTunnel(t, 0)
	now := time.Now()
	logger := slog.Default()

	out := tun.handleOCRQ(buildOCRQ(600, 2001), now, logger)
	if len(out) != 1 {
		t.Fatalf("expected 1 send (OCRP), got %d", len(out))
	}
	if tun.sessionCount() != 1 {
		t.Fatalf("expected 1 session, got %d", tun.sessionCount())
	}
	var sess *L2TPSession
	for _, s := range tun.sessions {
		sess = s
	}
	if sess.state != L2TPSessionWaitCSAnswer {
		t.Fatalf("expected wait-cs-answer, got %s", sess.state)
	}
	if sess.remoteSID != 600 {
		t.Fatalf("expected remote SID 600, got %d", sess.remoteSID)
	}
}

func TestSession_OutgoingLAC_OCCN(t *testing.T) {
	// VALIDATES: AC-13 -- OCCN received on session in wait-connect -> established.
	// PREVENTS: OCCN silently dropped or state not transitioned.
	//
	// handleOCCN requires WaitConnect state. Create a session via ICRQ (which
	// puts it in WaitConnect) and then deliver OCCN to exercise the handler.
	// The handler is state-based, not call-type-based.
	tun := newEstablishedTunnel(t, 0)
	now := time.Now()
	logger := slog.Default()

	// Step 1: Create session in WaitConnect via ICRQ.
	tun.handleICRQ(buildICRQ(500, 1001), now, logger)
	var sess *L2TPSession
	for _, s := range tun.sessions {
		sess = s
	}
	if sess == nil {
		t.Fatal("no session created")
	}
	if sess.state != L2TPSessionWaitConnect {
		t.Fatalf("expected wait-connect, got %s", sess.state)
	}

	// Step 2: Deliver OCCN.
	occnPayload := buildOCCN(56000, 1) // 56kbps, async framing
	out := tun.handleOCCN(sess, occnPayload, now, logger)
	if len(out) != 0 {
		t.Fatalf("OCCN: expected 0 sends, got %d", len(out))
	}
	if sess.state != L2TPSessionEstablished {
		t.Fatalf("expected established after OCCN, got %s", sess.state)
	}
	if sess.txConnectSpeed != 56000 {
		t.Fatalf("expected tx speed 56000, got %d", sess.txConnectSpeed)
	}
	if sess.framingType != 1 {
		t.Fatalf("expected framing 1, got %d", sess.framingType)
	}
}

// --- AC-14: Unknown mandatory AVP -> CDN ---

func TestSession_UnknownMandatoryAVP(t *testing.T) {
	// VALIDATES: AC-14 -- unknown M=1 vendor AVP -> CDN (not StopCCN).
	tun := newEstablishedTunnel(t, 0)
	now := time.Now()
	logger := slog.Default()

	// Build ICRQ with a mandatory vendor AVP (vendor=9999, type=1).
	var buf [128]byte
	off := 0
	off += WriteAVPUint16(buf[:], off, true, AVPMessageType, uint16(MsgICRQ))
	off += WriteAVPUint16(buf[:], off, true, AVPAssignedSessionID, 500)
	off += WriteAVPUint32(buf[:], off, true, AVPCallSerialNumber, 1001)
	// Mandatory vendor AVP: M=1, vendor=9999, type=1.
	off += WriteAVPBytes(buf[:], off, true, 9999, AVPType(1), []byte{0x01})
	payload := buf[:off]

	out := tun.handleICRQ(payload, now, logger)
	if len(out) != 1 {
		t.Fatalf("expected 1 send (CDN), got %d", len(out))
	}
	// Tunnel should still be established (not torn down).
	if tun.state != L2TPTunnelEstablished {
		t.Fatalf("expected tunnel still established, got %s", tun.state)
	}
}

// --- Duplicate remote SID ---

func TestSession_DuplicateRemoteSID_ICRQ(t *testing.T) {
	// VALIDATES: duplicate ICRQ with same Assigned Session ID is dropped.
	// PREVENTS: malicious peer creating two sessions with the same remote SID.
	tun := newEstablishedTunnel(t, 0)
	now := time.Now()
	logger := slog.Default()

	// First ICRQ succeeds.
	out := tun.handleICRQ(buildICRQ(500, 1001), now, logger)
	if len(out) != 1 {
		t.Fatalf("first ICRQ: expected 1 send (ICRP), got %d", len(out))
	}
	if tun.sessionCount() != 1 {
		t.Fatalf("expected 1 session, got %d", tun.sessionCount())
	}

	// Second ICRQ with same SID=500 but different call serial should be dropped.
	out = tun.handleICRQ(buildICRQ(500, 2002), now, logger)
	if len(out) != 0 {
		t.Fatalf("duplicate ICRQ: expected 0 sends (dropped), got %d", len(out))
	}
	if tun.sessionCount() != 1 {
		t.Fatalf("expected still 1 session, got %d", tun.sessionCount())
	}
}

// --- Missing Call Serial Number ---

func TestParseICRQ_MissingCallSerial(t *testing.T) {
	// VALIDATES: ICRQ missing Call Serial Number AVP -> error per RFC 2661 S7.6.
	var buf [64]byte
	off := 0
	off += WriteAVPUint16(buf[:], off, true, AVPMessageType, uint16(MsgICRQ))
	off += WriteAVPUint16(buf[:], off, true, AVPAssignedSessionID, 500)
	// No Call Serial Number AVP.
	_, err := parseICRQ(buf[:off])
	if err == nil {
		t.Fatal("expected error for missing Call Serial Number")
	}
}

// --- AC-16: Unknown header SessionID ---

func TestSession_UnknownHeaderSID(t *testing.T) {
	// VALIDATES: AC-16 -- unknown SID in header -> drop.
	tun := newEstablishedTunnel(t, 0)
	logger := slog.Default()
	now := time.Now()

	// Dispatch ICCN for non-existent session ID 9999.
	out := tun.dispatchToSession(MsgICCN, 9999, nil, now, logger)
	if len(out) != 0 {
		t.Fatalf("expected 0 sends (dropped), got %d", len(out))
	}
}

// --- AC-17, AC-18, AC-19: Proxy LCP/Auth/Sequencing ---

func TestSession_ProxyLCPAndAuth(t *testing.T) {
	// VALIDATES: AC-17 (proxy LCP), AC-18 (proxy auth), AC-19 (sequencing required).
	tun := newEstablishedTunnel(t, 0)
	now := time.Now()
	logger := slog.Default()

	tun.handleICRQ(buildICRQ(500, 1001), now, logger)
	var sess *L2TPSession
	for _, s := range tun.sessions {
		sess = s
	}

	iccnPayload := buildICCNWithProxy(10000000, 2)
	tun.handleICCN(sess, iccnPayload, now, logger)

	if sess.state != L2TPSessionEstablished {
		t.Fatalf("expected established, got %s", sess.state)
	}
	// AC-19.
	if !sess.sequencingRequired {
		t.Fatal("expected sequencingRequired = true")
	}
	// AC-17.
	if len(sess.proxyInitialRecvLCPConfReq) != 2 || sess.proxyInitialRecvLCPConfReq[0] != 0x01 {
		t.Fatalf("proxy initial recv LCP unexpected: %x", sess.proxyInitialRecvLCPConfReq)
	}
	if len(sess.proxyLastSentLCPConfReq) != 2 || sess.proxyLastSentLCPConfReq[0] != 0x03 {
		t.Fatalf("proxy last sent LCP unexpected: %x", sess.proxyLastSentLCPConfReq)
	}
	if len(sess.proxyLastRecvLCPConfReq) != 2 || sess.proxyLastRecvLCPConfReq[0] != 0x05 {
		t.Fatalf("proxy last recv LCP unexpected: %x", sess.proxyLastRecvLCPConfReq)
	}
	// AC-18.
	if sess.proxyAuthenType != 2 {
		t.Fatalf("expected proxy auth type 2 (CHAP), got %d", sess.proxyAuthenType)
	}
	if sess.proxyAuthenName != "user1" {
		t.Fatalf("expected proxy auth name 'user1', got %q", sess.proxyAuthenName)
	}
	if sess.proxyAuthenID != 42 {
		t.Fatalf("expected proxy auth ID 42, got %d", sess.proxyAuthenID)
	}
}

// --- Phase 5 kernel integration flag tests ---

func TestSessionEstablishedSetsKernelFlag(t *testing.T) {
	// VALIDATES: AC-3 -- handleICCN sets kernelSetupNeeded and lnsMode.
	// PREVENTS: session established without kernel notification.
	tun := newEstablishedTunnel(t, 0)
	now := time.Now()
	logger := slog.Default()

	tun.handleICRQ(buildICRQ(500, 1001), now, logger)
	var sess *L2TPSession
	for _, s := range tun.sessions {
		sess = s
	}
	if sess == nil {
		t.Fatal("no session created")
	}

	tun.handleICCN(sess, buildICCN(10000000, 2), now, logger)
	if sess.state != L2TPSessionEstablished {
		t.Fatalf("expected established, got %s", sess.state)
	}
	if !sess.kernelSetupNeeded {
		t.Fatal("expected kernelSetupNeeded = true after ICCN")
	}
	if !sess.lnsMode {
		t.Fatal("expected lnsMode = true for LNS-side (ICCN)")
	}
}

func TestSessionEstablishedOutgoingSetsKernelFlag(t *testing.T) {
	// VALIDATES: AC-3 -- handleOCCN sets kernelSetupNeeded, lnsMode=false.
	// PREVENTS: outgoing session established without kernel notification.
	//
	// handleOCCN requires WaitConnect state. Create session via ICRQ
	// (which puts it in WaitConnect) then deliver OCCN.
	tun := newEstablishedTunnel(t, 0)
	now := time.Now()
	logger := slog.Default()

	tun.handleICRQ(buildICRQ(500, 1001), now, logger)
	var sess *L2TPSession
	for _, s := range tun.sessions {
		sess = s
	}
	if sess == nil {
		t.Fatal("no session created")
	}
	if sess.state != L2TPSessionWaitConnect {
		t.Fatalf("expected wait-connect, got %s", sess.state)
	}

	tun.handleOCCN(sess, buildOCCN(10000000, 2), now, logger)
	if sess.state != L2TPSessionEstablished {
		t.Fatalf("expected established, got %s", sess.state)
	}
	if !sess.kernelSetupNeeded {
		t.Fatal("expected kernelSetupNeeded = true after OCCN")
	}
	if sess.lnsMode {
		t.Fatal("expected lnsMode = false for LAC-side (OCCN)")
	}
}

func TestSessionCDNQueuesTeardown(t *testing.T) {
	// VALIDATES: AC-10 -- CDN on established session queues kernel teardown.
	// PREVENTS: kernel resources leaked after CDN.
	tun := newEstablishedTunnel(t, 0)
	now := time.Now()
	logger := slog.Default()

	tun.handleICRQ(buildICRQ(500, 1001), now, logger)
	var sess *L2TPSession
	for _, s := range tun.sessions {
		sess = s
	}
	tun.handleICCN(sess, buildICCN(10000000, 2), now, logger)
	localSID := sess.localSID

	if len(tun.pendingKernelTeardowns) != 0 {
		t.Fatalf("expected 0 pending teardowns before CDN, got %d", len(tun.pendingKernelTeardowns))
	}

	// CDN removes the established session. buildCDN(resultCode, assignedSID).
	cdnPayload := buildCDN(2, localSID)
	tun.handleCDN(localSID, cdnPayload, logger)

	if len(tun.pendingKernelTeardowns) != 1 {
		t.Fatalf("expected 1 pending teardown after CDN, got %d", len(tun.pendingKernelTeardowns))
	}
	td := tun.pendingKernelTeardowns[0]
	if td.localTID != tun.localTID || td.localSID != localSID {
		t.Fatalf("teardown event IDs wrong: tid=%d sid=%d", td.localTID, td.localSID)
	}
}

func TestStopCCNQueuesAllTeardowns(t *testing.T) {
	// VALIDATES: AC-12 -- StopCCN queues kernel teardowns for all sessions.
	// PREVENTS: kernel resources leaked after StopCCN.
	tun := newEstablishedTunnel(t, 0)
	now := time.Now()
	logger := slog.Default()

	// Create 3 established sessions.
	for _, peerSID := range []uint16{500, 600, 700} {
		tun.handleICRQ(buildICRQ(peerSID, uint32(peerSID)+500), now, logger)
	}
	sessions := make([]*L2TPSession, 0, len(tun.sessions))
	for _, s := range tun.sessions {
		sessions = append(sessions, s)
	}
	for _, s := range sessions {
		tun.handleICCN(s, buildICCN(10000000, 2), now, logger)
	}

	if len(tun.pendingKernelTeardowns) != 0 {
		t.Fatalf("expected 0 pending teardowns before StopCCN, got %d", len(tun.pendingKernelTeardowns))
	}

	// Build StopCCN payload directly (same pattern as existing tests).
	var buf [64]byte
	off := 0
	off += WriteAVPUint16(buf[:], off, true, AVPMessageType, uint16(MsgStopCCN))
	off += WriteAVPUint16(buf[:], off, true, AVPAssignedTunnelID, 200)
	off += WriteAVPResultCode(buf[:], off, true, ResultCodeValue{Result: 1})
	tun.handleStopCCN(now, buf[:off])

	if len(tun.pendingKernelTeardowns) != 3 {
		t.Fatalf("expected 3 pending teardowns after StopCCN, got %d", len(tun.pendingKernelTeardowns))
	}
}

// --- Parser unit tests ---

func TestParseICRQ_Valid(t *testing.T) {
	payload := buildICRQ(500, 1001)
	info, err := parseICRQ(payload)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if info.assignedSessionID != 500 {
		t.Fatalf("expected SID 500, got %d", info.assignedSessionID)
	}
	if info.callSerialNumber != 1001 {
		t.Fatalf("expected call serial 1001, got %d", info.callSerialNumber)
	}
}

func TestParseICRQ_MissingSID(t *testing.T) {
	// ICRQ with only Message Type, no Assigned Session ID.
	var buf [32]byte
	n := WriteAVPUint16(buf[:], 0, true, AVPMessageType, uint16(MsgICRQ))
	_, err := parseICRQ(buf[:n])
	if err == nil {
		t.Fatal("expected error for missing SID")
	}
}

func TestParseICCN_Valid(t *testing.T) {
	payload := buildICCN(10000000, 2)
	info, err := parseICCN(payload)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if info.txConnectSpeed != 10000000 {
		t.Fatalf("expected tx speed 10000000, got %d", info.txConnectSpeed)
	}
	if info.framingType != 2 {
		t.Fatalf("expected framing 2, got %d", info.framingType)
	}
}

func TestParseICCN_MissingTxSpeed(t *testing.T) {
	// ICCN with only Message Type + Framing (missing Tx Connect Speed).
	var buf [64]byte
	off := WriteAVPUint16(buf[:], 0, true, AVPMessageType, uint16(MsgICCN))
	off += WriteAVPUint32(buf[:], off, true, AVPFramingType, 2)
	_, err := parseICCN(buf[:off])
	if err == nil {
		t.Fatal("expected error for missing tx speed")
	}
}

func TestParseOCRQ_Valid(t *testing.T) {
	payload := buildOCRQ(600, 2001)
	info, err := parseOCRQ(payload)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if info.assignedSessionID != 600 {
		t.Fatalf("expected SID 600, got %d", info.assignedSessionID)
	}
}

func TestParseOCCN_Valid(t *testing.T) {
	payload := buildOCCN(56000, 1)
	info, err := parseOCCN(payload)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if info.txConnectSpeed != 56000 {
		t.Fatalf("expected tx speed 56000, got %d", info.txConnectSpeed)
	}
	if info.framingType != 1 {
		t.Fatalf("expected framing 1, got %d", info.framingType)
	}
}

func TestParseCDN_Valid(t *testing.T) {
	payload := buildCDN(1, 500)
	info, err := parseCDN(payload)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if info.result != 1 {
		t.Fatalf("expected result 1, got %d", info.result)
	}
	if info.assignedSessionID != 500 {
		t.Fatalf("expected SID 500, got %d", info.assignedSessionID)
	}
}

// --- Wire builder round-trip tests ---

func TestWriteICRPBody(t *testing.T) {
	var buf [64]byte
	n := writeICRPBody(buf[:], 42)
	if n == 0 {
		t.Fatal("writeICRPBody returned 0 bytes")
	}
	// Parse it back to verify.
	iter := NewAVPIterator(buf[:n])
	_, attrType, _, value, ok := iter.Next()
	if !ok || attrType != AVPMessageType {
		t.Fatal("first AVP should be Message Type")
	}
	mt, _ := ReadAVPUint16(value)
	if MessageType(mt) != MsgICRP {
		t.Fatalf("expected ICRP (11), got %d", mt)
	}
	_, attrType, _, value, ok = iter.Next()
	if !ok || attrType != AVPAssignedSessionID {
		t.Fatal("second AVP should be Assigned Session ID")
	}
	sid, _ := ReadAVPUint16(value)
	if sid != 42 {
		t.Fatalf("expected SID 42, got %d", sid)
	}
}

func TestWriteCDNBody(t *testing.T) {
	var buf [64]byte
	n := writeCDNBody(buf[:], 55, 2)
	if n == 0 {
		t.Fatal("writeCDNBody returned 0 bytes")
	}
	info, err := parseCDN(buf[:n])
	if err != nil {
		t.Fatalf("CDN round-trip failed: %v", err)
	}
	if info.result != 2 {
		t.Fatalf("expected result 2, got %d", info.result)
	}
	if info.assignedSessionID != 55 {
		t.Fatalf("expected SID 55, got %d", info.assignedSessionID)
	}
}

func TestWriteOCRPBody(t *testing.T) {
	var buf [64]byte
	n := writeOCRPBody(buf[:], 77)
	if n == 0 {
		t.Fatal("writeOCRPBody returned 0 bytes")
	}
	iter := NewAVPIterator(buf[:n])
	_, attrType, _, value, ok := iter.Next()
	if !ok || attrType != AVPMessageType {
		t.Fatal("first AVP should be Message Type")
	}
	mt, _ := ReadAVPUint16(value)
	if MessageType(mt) != MsgOCRP {
		t.Fatalf("expected OCRP (8), got %d", mt)
	}
	_, attrType, _, value, ok = iter.Next()
	if !ok || attrType != AVPAssignedSessionID {
		t.Fatal("second AVP should be Assigned Session ID")
	}
	sid, _ := ReadAVPUint16(value)
	if sid != 77 {
		t.Fatalf("expected SID 77, got %d", sid)
	}
}

// --- Session state string ---

func TestSessionState_String(t *testing.T) {
	tests := []struct {
		state L2TPSessionState
		want  string
	}{
		{L2TPSessionIdle, "idle"},
		{L2TPSessionWaitTunnel, "wait-tunnel"},
		{L2TPSessionWaitReply, "wait-reply"},
		{L2TPSessionWaitConnect, "wait-connect"},
		{L2TPSessionWaitCSAnswer, "wait-cs-answer"},
		{L2TPSessionEstablished, "established"},
		{L2TPSessionState(99), "unknown"},
	}
	for _, tt := range tests {
		if got := tt.state.String(); got != tt.want {
			t.Errorf("state %d: got %q, want %q", tt.state, got, tt.want)
		}
	}
}

// --- Boundary tests ---

func TestSession_SIDBoundary_MaxUint16(t *testing.T) {
	// VALIDATES: Assigned Session ID 65535 is valid.
	payload := buildICRQ(65535, 1)
	info, err := parseICRQ(payload)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if info.assignedSessionID != 65535 {
		t.Fatalf("expected SID 65535, got %d", info.assignedSessionID)
	}
}

func TestSession_SIDBoundary_Zero(t *testing.T) {
	// VALIDATES: Assigned Session ID 0 is rejected by parser.
	payload := buildICRQ(0, 1)
	_, err := parseICRQ(payload)
	if err == nil {
		t.Fatal("expected error for SID=0")
	}
}
