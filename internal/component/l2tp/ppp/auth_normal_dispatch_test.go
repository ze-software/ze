package ppp

import (
	"encoding/binary"
	"net"
	"testing"
	"time"
)

// scriptedPeerLCPOnly performs the same LCP CA + CR + CA exchange as
// scriptedPeer (helpers_test.go) but RETURNS as soon as ze's own CA is
// consumed, rather than idling on peerEnd. After it exits, subsequent
// driver-side writes (e.g. a CHAP Challenge emitted by
// runCHAPAuthPhase) appear on peerEnd where the test can read them
// directly via readPeerFrame -- no concurrent idle reader racing the
// test for bytes.
//
// Shape matches scriptedPeer:
//  1. read ze's Configure-Request
//  2. send Configure-Ack echoing ze's options (so any Auth-Protocol ze
//     advertised is accepted verbatim)
//  3. send peer's own Configure-Request carrying only MRU=1500
//  4. read ze's Configure-Ack
//  5. return
func scriptedPeerLCPOnly(t *testing.T, conn net.Conn, done chan<- struct{}) {
	t.Helper()
	defer close(done)

	buf := make([]byte, MaxFrameLen)

	n, err := conn.Read(buf)
	if err != nil {
		t.Errorf("peer: read CR: %v", err)
		return
	}
	driverProto, driverPayload, _, err := ParseFrame(buf[:n])
	if err != nil || driverProto != ProtoLCP {
		t.Errorf("peer: bad first frame: proto=0x%04x err=%v", driverProto, err)
		return
	}
	driverCR, err := ParseLCPPacket(driverPayload)
	if err != nil || driverCR.Code != LCPConfigureRequest {
		t.Errorf("peer: expected CR, got code=%d err=%v", driverCR.Code, err)
		return
	}

	out := make([]byte, MaxFrameLen)
	off := WriteFrame(out, 0, ProtoLCP, nil)
	off += WriteLCPPacket(out, off, LCPConfigureAck, driverCR.Identifier, driverCR.Data)
	if _, err := conn.Write(out[:off]); err != nil {
		t.Errorf("peer: write CA: %v", err)
		return
	}

	peerMRU := []byte{LCPOptMRU, 0x04, 0x05, 0xDC}
	off = WriteFrame(out, 0, ProtoLCP, nil)
	off += WriteLCPPacket(out, off, LCPConfigureRequest, 1, peerMRU)
	if _, err := conn.Write(out[:off]); err != nil {
		t.Errorf("peer: write CR: %v", err)
		return
	}

	n, err = conn.Read(buf)
	if err != nil {
		t.Errorf("peer: read driver CA: %v", err)
		return
	}
	_, driverPayload, _, _ = ParseFrame(buf[:n])
	driverCA, err := ParseLCPPacket(driverPayload)
	if err != nil || driverCA.Code != LCPConfigureAck {
		t.Errorf("peer: expected driver CA, got code=%d err=%v", driverCA.Code, err)
		return
	}
}

// VALIDATES: Decision E, normal-path CHAP-MD5 dispatch. When the peer
//
//	ACKs ze's CONFREQ with Auth-Protocol = 0xC223 +
//	Algorithm = 0x05, the session transitions to Opened and
//	runAuthPhase dispatches to runCHAPAuthPhase. The first
//	wire act of runCHAPAuthPhase is a CHAP Challenge frame
//	(code 1) with Value-Size = 16.
//
// PREVENTS: regression where the normal (non-proxy) path fails to
//
//	thread configuredAuthMethod = CHAP-MD5 into
//	runAuthPhase, leaving the handler on runNoAuthPhase or
//	the wrong method.
func TestNormalPathDispatchesCHAPMD5(t *testing.T) {
	reg := newPipeRegistry()
	installPipeRegistry(t, reg)
	pair := newPipePair(reg, 17001)
	defer closeConn(pair.peerEnd)

	ops, _, _ := newFakeOps()
	d := newTestDriverNoResponder(&fakeBackend{}, ops)
	if err := d.Start(); err != nil {
		t.Fatalf("Start: %v", err)
	}
	defer d.Stop()

	peerDone := make(chan struct{})
	go scriptedPeerLCPOnly(t, pair.peerEnd, peerDone)
	t.Cleanup(func() { <-peerDone })

	d.SessionsIn() <- StartSession{
		TunnelID:   301,
		SessionID:  401,
		ChanFD:     17001,
		UnitFD:     917,
		UnitNum:    17,
		LNSMode:    true,
		MaxMRU:     1500,
		AuthMethod: AuthMethodCHAPMD5,
	}

	// peerDone fires once scriptedPeerLCPOnly consumes ze's CA; after
	// that the pipe is quiet until runCHAPAuthPhase writes the
	// Challenge. Read it directly.
	<-peerDone
	proto, payload := readPeerFrame(t, pair.peerEnd)
	if proto != ProtoCHAP {
		t.Fatalf("post-Opened frame proto = 0x%04x, want ProtoCHAP 0x%04x",
			proto, ProtoCHAP)
	}
	if len(payload) < chapHeaderLen+1+chapChallengeValueLen {
		t.Fatalf("CHAP frame too short for Challenge: %x", payload)
	}
	if payload[0] != CHAPCodeChallenge {
		t.Errorf("CHAP code = %d, want %d (Challenge)",
			payload[0], CHAPCodeChallenge)
	}
	if payload[4] != chapChallengeValueLen {
		t.Errorf("CHAP Value-Size = %d, want %d", payload[4], chapChallengeValueLen)
	}

	if err := d.StopSession(301, 401); err != nil {
		t.Logf("StopSession: %v", err)
	}
	drainEventsBest(t, d.EventsOut(), 2, 500*time.Millisecond)
}

// VALIDATES: Decision E, normal-path MS-CHAPv2 dispatch. When the peer
//
//	ACKs ze's CONFREQ with Auth-Protocol = 0xC223 +
//	Algorithm = 0x81, runAuthPhase dispatches to
//	runMSCHAPv2AuthPhase; the first wire act is a CHAP
//	Challenge whose Value-Size is the 16-byte Authenticator
//	Challenge required by RFC 2759 Section 4.
//
// PREVENTS: regression where MS-CHAPv2 silently collapses to CHAP-MD5
//
//	on the normal path (the CHAP Algorithm byte never
//	reaches authMethodToLCPOptions or the dispatch switch).
func TestNormalPathDispatchesMSCHAPv2(t *testing.T) {
	reg := newPipeRegistry()
	installPipeRegistry(t, reg)
	pair := newPipePair(reg, 17101)
	defer closeConn(pair.peerEnd)

	ops, _, _ := newFakeOps()
	d := newTestDriverNoResponder(&fakeBackend{}, ops)
	if err := d.Start(); err != nil {
		t.Fatalf("Start: %v", err)
	}
	defer d.Stop()

	peerDone := make(chan struct{})
	go scriptedPeerLCPOnly(t, pair.peerEnd, peerDone)
	t.Cleanup(func() { <-peerDone })

	d.SessionsIn() <- StartSession{
		TunnelID:   302,
		SessionID:  402,
		ChanFD:     17101,
		UnitFD:     918,
		UnitNum:    18,
		LNSMode:    true,
		MaxMRU:     1500,
		AuthMethod: AuthMethodMSCHAPv2,
	}

	<-peerDone
	proto, payload := readPeerFrame(t, pair.peerEnd)
	if proto != ProtoCHAP {
		t.Fatalf("post-Opened frame proto = 0x%04x, want ProtoCHAP 0x%04x",
			proto, ProtoCHAP)
	}
	if len(payload) < chapHeaderLen+1+16 {
		t.Fatalf("CHAP frame too short for MS-CHAPv2 Challenge: %x", payload)
	}
	if payload[0] != CHAPCodeChallenge {
		t.Errorf("CHAP code = %d, want %d (Challenge)",
			payload[0], CHAPCodeChallenge)
	}
	// RFC 2759 Section 4: the Authenticator Challenge is 16 octets.
	if payload[4] != 16 {
		t.Errorf("Value-Size = %d, want 16 (MS-CHAPv2 Authenticator Challenge)",
			payload[4])
	}

	if err := d.StopSession(302, 402); err != nil {
		t.Logf("StopSession: %v", err)
	}
	drainEventsBest(t, d.EventsOut(), 2, 500*time.Millisecond)
}

// nakThenAckPeer extends the Phase 8 authProtoReplyPeer contract with a
// third step: after ze's second CONFREQ is received, the peer ACKs it
// (echoing whatever auth-protocol option ze advertised post-fallback),
// sends its own CONFREQ, and reads ze's CA. The idle loop is omitted
// so the caller can read subsequent auth frames directly from peerEnd.
//
// Parameters:
//
//   - wantFirstProto: expected Auth-Protocol value in ze's first CR.
//   - nakProto + nakAlgo: suggestion ze will receive in the Configure-
//     Nak.
//
// Returns nothing on its own; the done channel closes after step 5.
func nakThenAckPeer(
	t *testing.T,
	conn net.Conn,
	wantFirstProto uint16,
	nakProto uint16,
	nakAlgo []byte,
	done chan<- struct{},
) {
	t.Helper()
	defer close(done)

	buf := make([]byte, MaxFrameLen)

	// 1. Read ze's first CR; verify Auth-Protocol.
	if err := conn.SetReadDeadline(time.Now().Add(peerNegotiateReadTimeout)); err != nil {
		t.Errorf("peer: SetReadDeadline: %v", err)
		return
	}
	n, err := conn.Read(buf)
	if err != nil {
		t.Errorf("peer: read first CR: %v", err)
		return
	}
	proto, payload, _, err := ParseFrame(buf[:n])
	if err != nil || proto != ProtoLCP {
		t.Errorf("peer: first frame bad: proto=0x%04x err=%v", proto, err)
		return
	}
	firstCR, err := ParseLCPPacket(payload)
	if err != nil || firstCR.Code != LCPConfigureRequest {
		t.Errorf("peer: expected CR, got code=%d err=%v", firstCR.Code, err)
		return
	}
	firstOpts, err := ParseLCPOptions(firstCR.Data)
	if err != nil {
		t.Errorf("peer: parse CR opts: %v", err)
		return
	}
	authData, hasAuth := lookupOption(firstOpts, LCPOptAuthProto)
	if !hasAuth || len(authData) < 2 || binary.BigEndian.Uint16(authData[:2]) != wantFirstProto {
		t.Errorf("peer: first CR Auth-Protocol = %x, want 0x%04x",
			authData, wantFirstProto)
		return
	}

	// 2. Send Configure-Nak suggesting nakProto.
	out := make([]byte, MaxFrameLen)
	off := WriteFrame(out, 0, ProtoLCP, nil)
	dataOff := off + lcpHeaderLen
	nakOpt := authProtoOpt(nakProto, nakAlgo...)
	dataLen := WriteLCPOptions(out, dataOff, []LCPOption{nakOpt})
	off += WriteLCPPacket(out, off, LCPConfigureNak, firstCR.Identifier, out[dataOff:dataOff+dataLen])
	if _, err := conn.Write(out[:off]); err != nil {
		t.Errorf("peer: write Nak: %v", err)
		return
	}

	// 3. Read ze's second CR (the post-fallback one).
	if err := conn.SetReadDeadline(time.Now().Add(peerNegotiateReadTimeout)); err != nil {
		t.Errorf("peer: SetReadDeadline (second): %v", err)
		return
	}
	n, err = conn.Read(buf)
	if err != nil {
		t.Errorf("peer: read second CR: %v", err)
		return
	}
	proto, payload, _, err = ParseFrame(buf[:n])
	if err != nil || proto != ProtoLCP {
		t.Errorf("peer: second frame bad: proto=0x%04x err=%v", proto, err)
		return
	}
	secondCR, err := ParseLCPPacket(payload)
	if err != nil || secondCR.Code != LCPConfigureRequest {
		t.Errorf("peer: expected second CR, got code=%d err=%v",
			secondCR.Code, err)
		return
	}

	// 4. ACK the second CR echoing its options verbatim. Clear the
	// read deadline for the subsequent CA read because net.Pipe's
	// SetReadDeadline sticks until explicitly reset.
	if err := conn.SetReadDeadline(time.Time{}); err != nil {
		t.Errorf("peer: clear SetReadDeadline: %v", err)
		return
	}
	off = WriteFrame(out, 0, ProtoLCP, nil)
	off += WriteLCPPacket(out, off, LCPConfigureAck, secondCR.Identifier, secondCR.Data)
	if _, err := conn.Write(out[:off]); err != nil {
		t.Errorf("peer: write CA to second CR: %v", err)
		return
	}

	// 5. Send peer's own CR and read ze's CA.
	peerMRU := []byte{LCPOptMRU, 0x04, 0x05, 0xDC}
	off = WriteFrame(out, 0, ProtoLCP, nil)
	off += WriteLCPPacket(out, off, LCPConfigureRequest, 1, peerMRU)
	if _, err := conn.Write(out[:off]); err != nil {
		t.Errorf("peer: write own CR: %v", err)
		return
	}
	n, err = conn.Read(buf)
	if err != nil {
		t.Errorf("peer: read CA: %v", err)
		return
	}
	_, payload, _, _ = ParseFrame(buf[:n])
	ca, err := ParseLCPPacket(payload)
	if err != nil || ca.Code != LCPConfigureAck {
		t.Errorf("peer: expected CA, got code=%d err=%v", ca.Code, err)
		return
	}
}

// VALIDATES: Decision E, end-to-end Nak fallback. When ze advertises
//
//	PAP and the peer Naks suggesting CHAP-MD5, ze's second
//	CONFREQ carries CHAP-MD5, the peer ACKs it, LCP reaches
//	Opened, and runAuthPhase dispatches to runCHAPAuthPhase
//	-- observable as a CHAP Challenge frame on peerEnd.
//
// PREVENTS: regression where Phase 8 mutates configuredAuthMethod on
//
//	Nak but the Opened transition's
//	negotiatedAuthMethod = configuredAuthMethod assignment
//	gets reordered or lost, leaving the handler on the
//	original (rejected) method.
func TestAuthFallbackOnNakProceedsToCHAP(t *testing.T) {
	reg := newPipeRegistry()
	installPipeRegistry(t, reg)
	pair := newPipePair(reg, 17201)
	defer closeConn(pair.peerEnd)

	ops, _, _ := newFakeOps()
	d := newTestDriverNoResponder(&fakeBackend{}, ops)
	if err := d.Start(); err != nil {
		t.Fatalf("Start: %v", err)
	}
	defer d.Stop()

	peerDone := make(chan struct{})
	go nakThenAckPeer(
		t, pair.peerEnd,
		authProtoPAP,
		authProtoCHAP, []byte{chapAlgorithmMD5},
		peerDone,
	)
	t.Cleanup(func() { <-peerDone })

	d.SessionsIn() <- StartSession{
		TunnelID:          303,
		SessionID:         403,
		ChanFD:            17201,
		UnitFD:            919,
		UnitNum:           19,
		LNSMode:           true,
		MaxMRU:            1500,
		AuthMethod:        AuthMethodPAP,
		AuthFallbackOrder: []AuthMethod{AuthMethodCHAPMD5, AuthMethodMSCHAPv2},
	}

	<-peerDone
	proto, payload := readPeerFrame(t, pair.peerEnd)
	if proto != ProtoCHAP {
		t.Fatalf("post-Opened frame proto = 0x%04x, want ProtoCHAP 0x%04x (post-fallback dispatch)",
			proto, ProtoCHAP)
	}
	if len(payload) < chapHeaderLen+1+chapChallengeValueLen {
		t.Fatalf("CHAP frame too short: %x", payload)
	}
	if payload[0] != CHAPCodeChallenge {
		t.Errorf("CHAP code = %d, want %d (Challenge)",
			payload[0], CHAPCodeChallenge)
	}

	if err := d.StopSession(303, 403); err != nil {
		t.Logf("StopSession: %v", err)
	}
	drainEventsBest(t, d.EventsOut(), 2, 500*time.Millisecond)
}
