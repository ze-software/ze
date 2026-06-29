package ppp

import (
	"encoding/binary"
	"net"
	"testing"
	"time"
)

// peerNegotiateReadTimeout bounds the two critical reads in
// authProtoReplyPeer. Long enough to absorb scheduler jitter, short
// enough to surface a "ze never sent the expected frame" failure with
// a clear error instead of deadlocking on an idle net.Pipe.
const peerNegotiateReadTimeout = 2 * time.Second

// authProtoReplyPeer scripts one side of an LCP negotiation for the
// NAK / Reject fallback tests. It reads ze's first Configure-Request,
// checks the Auth-Protocol option matches wantProto, then sends either
// a Configure-Reject echoing the Auth-Protocol option (when code ==
// LCPConfigureReject) or a Configure-Nak containing the nakProto
// suggestion (when code == LCPConfigureNak). The second CONFREQ that
// ze emits in response is returned on secondCR for the main test to
// inspect.
//
// Both critical reads use SetReadDeadline so a regression where ze
// stops sending frames surfaces as a localized peer-side timeout with
// a specific error message rather than deadlocking until the outer
// test timeout fires. After the second CR is published the goroutine
// returns; no idle Read loop is kept open, because in these tests ze
// performs no further wire activity until negotiation timer fires
// (30 s, well outside the test's scope) or StopSession closes the fd.
func authProtoReplyPeer(
	t *testing.T,
	conn net.Conn,
	wantProto uint16,
	code uint8,
	nakProto uint16,
	nakAlgo []byte,
	secondCR chan<- LCPPacket,
	done chan<- struct{},
) {
	t.Helper()
	defer close(done)

	buf := make([]byte, MaxFrameLen)

	// Read ze's first Configure-Request.
	if err := conn.SetReadDeadline(time.Now().Add(peerNegotiateReadTimeout)); err != nil {
		t.Errorf("peer: SetReadDeadline: %v", err)
		return
	}
	n, err := conn.Read(buf)
	if err != nil {
		t.Errorf("peer: read first CR: %v", err)
		return
	}
	firstProto, firstPayload, _, err := ParseFrame(buf[:n])
	if err != nil || firstProto != ProtoLCP {
		t.Errorf("peer: first frame bad: proto=0x%04x err=%v", firstProto, err)
		return
	}
	firstCR, err := ParseLCPPacket(firstPayload)
	if err != nil || firstCR.Code != LCPConfigureRequest {
		t.Errorf("peer: expected CR, got code=%d err=%v", firstCR.Code, err)
		return
	}
	firstOpts, err := ParseLCPOptions(firstCR.Data)
	if err != nil {
		t.Errorf("peer: parse CR options: %v", err)
		return
	}
	authData, hasAuth := lookupOption(firstOpts, LCPOptAuthProto)
	if !hasAuth {
		t.Errorf("peer: first CR missing Auth-Protocol option")
		return
	}
	if len(authData) < 2 || binary.BigEndian.Uint16(authData[:2]) != wantProto {
		t.Errorf("peer: first CR Auth-Protocol = %x, want proto 0x%04x", authData, wantProto)
		return
	}

	// Build Nak or Reject body containing only the Auth-Protocol
	// option. For Reject, echo the option verbatim per RFC 1661 §5.4.
	// For Nak, carry the new suggested protocol per §5.3.
	out := make([]byte, MaxFrameLen)
	off := WriteFrame(out, 0, ProtoLCP, nil)
	dataOff := off + lcpHeaderLen
	var replyOpt LCPOption
	if code == LCPConfigureReject {
		replyOpt = LCPOption{Type: LCPOptAuthProto, Data: authData}
	} else {
		replyOpt = authProtoOpt(nakProto, nakAlgo...)
	}
	dataLen := WriteLCPOptions(out, dataOff, []LCPOption{replyOpt})
	off += WriteLCPPacket(out, off, code, firstCR.Identifier, out[dataOff:dataOff+dataLen])
	if _, err := conn.Write(out[:off]); err != nil {
		t.Errorf("peer: write reject/nak: %v", err)
		return
	}

	// Read ze's second Configure-Request (the resent one).
	if err := conn.SetReadDeadline(time.Now().Add(peerNegotiateReadTimeout)); err != nil {
		t.Errorf("peer: SetReadDeadline (second): %v", err)
		return
	}
	n, err = conn.Read(buf)
	if err != nil {
		t.Errorf("peer: read second CR: %v", err)
		return
	}
	secondProto, secondPayload, _, err := ParseFrame(buf[:n])
	if err != nil || secondProto != ProtoLCP {
		t.Errorf("peer: second frame bad: proto=0x%04x err=%v", secondProto, err)
		return
	}
	secondPkt, err := ParseLCPPacket(secondPayload)
	if err != nil {
		t.Errorf("peer: parse second CR: %v", err)
		return
	}
	// Copy the Data slice before sending on the channel because buf
	// is reused if this goroutine ever re-enters Read (it does not
	// today, but the copy keeps the channel send decoupled from buf's
	// lifetime for future refactors).
	dataCopy := make([]byte, len(secondPkt.Data))
	copy(dataCopy, secondPkt.Data)
	secondPkt.Data = dataCopy
	secondCR <- secondPkt
}

// VALIDATES: Phase 8 AC: when the peer Configure-Rejects ze's
//
//	Auth-Protocol option, the FSM resends CONFREQ WITHOUT
//	Auth-Protocol and configuredAuthMethod is cleared so the
//	session later enters runNoAuthPhase instead of a handler
//	that would wait for a wire message the peer will not send.
//
// PREVENTS: regression (deferral 2026-04-17) where the Reject path
//
//	keeps configuredAuthMethod in force, the session reaches
//	Opened advertising an auth method the peer refused, and
//	times out on the authentication phase.
func TestAuthProtoRejectClearsMethod(t *testing.T) {
	reg := newPipeRegistry()
	installPipeRegistry(t, reg)
	pair := newPipePair(reg, 16001)
	defer closeConn(pair.peerEnd)

	ops, _, _ := newFakeOps()
	d := newTestDriverNoResponder(&fakeBackend{}, ops)
	if err := d.Start(); err != nil {
		t.Fatalf("Start: %v", err)
	}
	defer d.Stop()

	secondCR := make(chan LCPPacket, 1)
	peerDone := make(chan struct{})
	go authProtoReplyPeer(
		t, pair.peerEnd,
		authProtoPAP, LCPConfigureReject, 0, nil,
		secondCR, peerDone,
	)
	t.Cleanup(func() { <-peerDone })

	d.SessionsIn() <- StartSession{
		TunnelID:   181,
		SessionID:  281,
		ChanFD:     16001,
		UnitFD:     981,
		UnitNum:    21,
		LNSMode:    true,
		MaxMRU:     1500,
		AuthMethod: AuthMethodPAP,
	}

	select {
	case pkt := <-secondCR:
		if pkt.Code != LCPConfigureRequest {
			t.Fatalf("second frame code = %d, want LCPConfigureRequest", pkt.Code)
		}
		opts, err := ParseLCPOptions(pkt.Data)
		if err != nil {
			t.Fatalf("ParseLCPOptions on second CR: %v", err)
		}
		if _, ok := lookupOption(opts, LCPOptAuthProto); ok {
			t.Errorf("second CONFREQ still carries Auth-Protocol after Configure-Reject")
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for second CONFREQ")
	}

	if err := d.StopSession(181, 281); err != nil {
		t.Logf("StopSession: %v", err)
	}
	drainEventsBest(t, d.EventsOut(), 2, 500*time.Millisecond)
}

// VALIDATES: Phase 8 AC-13: when the peer Configure-Naks ze's
//
//	Auth-Protocol with a different value that IS in ze's
//	AuthFallbackOrder, the second CONFREQ advertises the
//	peer's suggested method and configuredAuthMethod tracks
//	the change.
//
// PREVENTS: regression where a Nak triggers either a blind drop of
//
//	Auth-Protocol (losing the opportunity to fall back to
//	CHAP/MS-CHAPv2) or an infinite resend of the same
//	rejected value.
func TestAuthFallbackOnNakDispatches(t *testing.T) {
	reg := newPipeRegistry()
	installPipeRegistry(t, reg)
	pair := newPipePair(reg, 16101)
	defer closeConn(pair.peerEnd)

	ops, _, _ := newFakeOps()
	d := newTestDriverNoResponder(&fakeBackend{}, ops)
	if err := d.Start(); err != nil {
		t.Fatalf("Start: %v", err)
	}
	defer d.Stop()

	secondCR := make(chan LCPPacket, 1)
	peerDone := make(chan struct{})
	go authProtoReplyPeer(
		t, pair.peerEnd,
		authProtoPAP, LCPConfigureNak,
		authProtoCHAP, []byte{chapAlgorithmMD5},
		secondCR, peerDone,
	)
	t.Cleanup(func() { <-peerDone })

	d.SessionsIn() <- StartSession{
		TunnelID:   182,
		SessionID:  282,
		ChanFD:     16101,
		UnitFD:     982,
		UnitNum:    22,
		LNSMode:    true,
		MaxMRU:     1500,
		AuthMethod: AuthMethodPAP,
		// Explicit order so the test does not depend on the package
		// default silently matching CHAP-MD5.
		AuthFallbackOrder: []AuthMethod{AuthMethodCHAPMD5, AuthMethodMSCHAPv2},
	}

	select {
	case pkt := <-secondCR:
		if pkt.Code != LCPConfigureRequest {
			t.Fatalf("second frame code = %d, want LCPConfigureRequest", pkt.Code)
		}
		opts, err := ParseLCPOptions(pkt.Data)
		if err != nil {
			t.Fatalf("ParseLCPOptions on second CR: %v", err)
		}
		data, ok := lookupOption(opts, LCPOptAuthProto)
		if !ok {
			t.Fatalf("second CONFREQ missing Auth-Protocol after Nak (want CHAP-MD5)")
		}
		if len(data) < 3 {
			t.Fatalf("Auth-Protocol data too short: %x", data)
		}
		proto := binary.BigEndian.Uint16(data[:2])
		if proto != authProtoCHAP {
			t.Errorf("second CONFREQ Auth-Protocol = 0x%04x, want 0x%04x (CHAP)",
				proto, authProtoCHAP)
		}
		if data[2] != chapAlgorithmMD5 {
			t.Errorf("second CONFREQ CHAP Algorithm = 0x%02x, want 0x%02x (MD5)",
				data[2], chapAlgorithmMD5)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for second CONFREQ")
	}

	if err := d.StopSession(182, 282); err != nil {
		t.Logf("StopSession: %v", err)
	}
	drainEventsBest(t, d.EventsOut(), 2, 500*time.Millisecond)
}

// VALIDATES: Phase 8: when the peer Configure-Naks with a suggestion
//
//	that is NOT in ze's AuthFallbackOrder, the next
//	CONFREQ drops the Auth-Protocol option entirely so the
//	session proceeds with runNoAuthPhase rather than
//	re-sending a method the peer already rejected.
//
// PREVENTS: regression where a non-matching Nak silently keeps the
//
//	rejected method in force, or where fallback ignores the
//	order list.
func TestAuthFallbackOnNakExhaustsToNone(t *testing.T) {
	reg := newPipeRegistry()
	installPipeRegistry(t, reg)
	pair := newPipePair(reg, 16201)
	defer closeConn(pair.peerEnd)

	ops, _, _ := newFakeOps()
	d := newTestDriverNoResponder(&fakeBackend{}, ops)
	if err := d.Start(); err != nil {
		t.Fatalf("Start: %v", err)
	}
	defer d.Stop()

	secondCR := make(chan LCPPacket, 1)
	peerDone := make(chan struct{})
	// Peer suggests PAP, but order only accepts CHAP-MD5 -- so the
	// fallback should collapse to None.
	go authProtoReplyPeer(
		t, pair.peerEnd,
		authProtoCHAP, LCPConfigureNak,
		authProtoPAP, nil,
		secondCR, peerDone,
	)
	t.Cleanup(func() { <-peerDone })

	d.SessionsIn() <- StartSession{
		TunnelID:          183,
		SessionID:         283,
		ChanFD:            16201,
		UnitFD:            983,
		UnitNum:           23,
		LNSMode:           true,
		MaxMRU:            1500,
		AuthMethod:        AuthMethodCHAPMD5,
		AuthFallbackOrder: []AuthMethod{AuthMethodCHAPMD5},
	}

	select {
	case pkt := <-secondCR:
		opts, err := ParseLCPOptions(pkt.Data)
		if err != nil {
			t.Fatalf("ParseLCPOptions on second CR: %v", err)
		}
		if _, ok := lookupOption(opts, LCPOptAuthProto); ok {
			t.Errorf("second CONFREQ carries Auth-Protocol after non-matching Nak; want it cleared to None")
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for second CONFREQ")
	}

	if err := d.StopSession(183, 283); err != nil {
		t.Logf("StopSession: %v", err)
	}
	drainEventsBest(t, d.EventsOut(), 2, 500*time.Millisecond)
}

// buildPAPRequestFrame assembles a full PAP Authenticate-Request
// wrapped in a PPP frame for delivery over a net.Pipe. Used by the
// dispatch tests to prove PAP dispatch reaches ParsePAPRequest.
func buildPAPRequestFrame(identifier uint8, user, pass string) []byte {
	total := papHeaderLen + 1 + len(user) + 1 + len(pass)
	papPkt := make([]byte, total)
	papPkt[0] = PAPAuthenticateRequest
	papPkt[1] = identifier
	binary.BigEndian.PutUint16(papPkt[2:4], uint16(total))
	papPkt[4] = byte(len(user))
	copy(papPkt[5:], user)
	papPkt[5+len(user)] = byte(len(pass))
	copy(papPkt[5+len(user)+1:], pass)

	out := make([]byte, MaxFrameLen)
	off := WriteFrame(out, 0, ProtoPAP, nil)
	off += copy(out[off:], papPkt)
	return out[:off]
}

// VALIDATES: StartSession.AuthMethod is threaded into pppSession, so
//
//	on the normal (non-proxy) LCP path the scripted peer
//	ACKs ze's CONFREQ (including the Auth-Protocol option ze
//	advertised) and negotiatedAuthMethod equals the
//	configured one. runAuthPhase then dispatches to
//	runPAPAuthPhase, which reads a PAP Authenticate-Request
//	from the peer and emits EventAuthRequest with
//	AuthMethodPAP.
//
// PREVENTS: regression where the AuthMethod field is silently dropped
//
//	between StartSession and pppSession, or where normal-
//	path negotiation does not set negotiatedAuthMethod.
func TestStartSessionAuthMethodThreaded(t *testing.T) {
	reg := newPipeRegistry()
	installPipeRegistry(t, reg)
	pair := newPipePair(reg, 15001)
	defer closeConn(pair.peerEnd)

	ops, _, _ := newFakeOps()
	d := newTestDriverNoResponder(&fakeBackend{}, ops)
	if err := d.Start(); err != nil {
		t.Fatalf("Start: %v", err)
	}
	defer d.Stop()

	peerDone := make(chan struct{})
	go scriptedPeer(t, pair.peerEnd, peerDone)
	t.Cleanup(func() { <-peerDone })

	d.SessionsIn() <- StartSession{
		TunnelID:   101,
		SessionID:  201,
		ChanFD:     15001,
		UnitFD:     901,
		UnitNum:    1,
		LNSMode:    true,
		MaxMRU:     1500,
		AuthMethod: AuthMethodPAP,
	}

	// Wait for EventLCPUp before writing PAP bytes. Without this the
	// test's write races scriptedPeer's CA/CR writes on the shared
	// pipe, which net.Pipe cannot frame.
	select {
	case ev := <-d.EventsOut():
		if _, ok := ev.(EventLCPUp); !ok {
			t.Fatalf("first lifecycle event %T, want EventLCPUp", ev)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for EventLCPUp")
	}

	if _, err := pair.peerEnd.Write(buildPAPRequestFrame(42, "alice", "sekret")); err != nil {
		t.Fatalf("peer write: %v", err)
	}

	select {
	case ev := <-d.AuthEventsOut():
		req, ok := ev.(EventAuthRequest)
		if !ok {
			t.Fatalf("auth event %T, want EventAuthRequest", ev)
		}
		if req.Method != AuthMethodPAP {
			t.Errorf("method = %v, want AuthMethodPAP", req.Method)
		}
		if req.Username != "alice" {
			t.Errorf("username = %q, want \"alice\"", req.Username)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for EventAuthRequest from PAP handler")
	}

	// Accept so the session goroutine exits cleanly before Stop
	// tears down the pipe the scripted peer is reading from.
	if err := d.AuthResponse(101, 201, true, "", nil); err != nil {
		t.Logf("AuthResponse: %v", err)
	}
	drainEventsBest(t, d.EventsOut(), 2, 500*time.Millisecond)
}

// VALIDATES: proxy LCP with Auth-Protocol = PAP selects runPAPAuthPhase
//
//	even when StartSession.AuthMethod is AuthMethodNone. The
//	LAC's already-negotiated choice is authoritative.
//
// PREVENTS: regression where StartSession.AuthMethod=None bypasses the
//
//	proxy-supplied auth method and silently skips authentication
//	when the LAC actually negotiated PAP with the peer.
func TestProxyLCPAuthMethodOverrides(t *testing.T) {
	reg := newPipeRegistry()
	installPipeRegistry(t, reg)
	pair := newPipePair(reg, 15101)
	defer closeConn(pair.peerEnd)

	ops, _, _ := newFakeOps()
	d := newTestDriverNoResponder(&fakeBackend{}, ops)
	if err := d.Start(); err != nil {
		t.Fatalf("Start: %v", err)
	}
	defer d.Stop()

	// LAC's Last-Sent CONFREQ carries Auth-Protocol = PAP.
	lastSent := buildOptionStream([]LCPOption{
		mruOpt(1500),
		authProtoOpt(0xC023),
	})
	plain := buildOptionStream([]LCPOption{
		mruOpt(1500),
		magicOpt(0xCAFEBABE),
	})
	d.SessionsIn() <- StartSession{
		TunnelID:            111,
		SessionID:           222,
		ChanFD:              15101,
		UnitFD:              902,
		UnitNum:             2,
		LNSMode:             true,
		MaxMRU:              1500,
		AuthMethod:          AuthMethodNone, // overridden by proxy
		ProxyLCPInitialRecv: plain,
		ProxyLCPLastSent:    lastSent,
		ProxyLCPLastRecv:    plain,
	}

	// Peer sends PAP Authenticate-Request; if dispatch works we see
	// AuthMethodPAP on the auth channel.
	if _, err := pair.peerEnd.Write(buildPAPRequestFrame(7, "bob", "pw")); err != nil {
		t.Fatalf("peer write: %v", err)
	}

	select {
	case ev := <-d.AuthEventsOut():
		req, ok := ev.(EventAuthRequest)
		if !ok {
			t.Fatalf("auth event %T, want EventAuthRequest", ev)
		}
		if req.Method != AuthMethodPAP {
			t.Errorf("method = %v, want AuthMethodPAP (proxy override)", req.Method)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for EventAuthRequest")
	}

	if err := d.StopSession(111, 222); err != nil {
		t.Logf("StopSession: %v", err)
	}
	drainEventsBest(t, d.EventsOut(), 2, 500*time.Millisecond)
}

// VALIDATES: proxy LCP with Auth-Protocol = CHAP-MD5 dispatches to
//
//	runCHAPAuthPhase: the session writes a CHAP Challenge
//	frame toward the peer as its first act after LCP-Opened.
//
// PREVENTS: regression where CHAP-MD5 selection drops back to PAP or
//
//	None and the session would deadlock waiting for a frame
//	that the authenticator should have initiated.
func TestProxyLCPDispatchesCHAPMD5(t *testing.T) {
	reg := newPipeRegistry()
	installPipeRegistry(t, reg)
	pair := newPipePair(reg, 15201)
	defer closeConn(pair.peerEnd)

	ops, _, _ := newFakeOps()
	d := newTestDriverNoResponder(&fakeBackend{}, ops)
	if err := d.Start(); err != nil {
		t.Fatalf("Start: %v", err)
	}
	defer d.Stop()

	lastSent := buildOptionStream([]LCPOption{
		mruOpt(1500),
		authProtoOpt(0xC223, 0x05),
	})
	plain := buildOptionStream([]LCPOption{
		mruOpt(1500),
		magicOpt(0xCAFEBABE),
	})
	d.SessionsIn() <- StartSession{
		TunnelID:            121,
		SessionID:           232,
		ChanFD:              15201,
		UnitFD:              903,
		UnitNum:             3,
		LNSMode:             true,
		MaxMRU:              1500,
		ProxyLCPInitialRecv: plain,
		ProxyLCPLastSent:    lastSent,
		ProxyLCPLastRecv:    plain,
	}

	proto, payload := readPeerFrame(t, pair.peerEnd)
	if proto != ProtoCHAP {
		t.Fatalf("first peer frame proto = 0x%04x, want ProtoCHAP 0x%04x",
			proto, ProtoCHAP)
	}
	if len(payload) < 4 || payload[0] != 1 {
		t.Errorf("first CHAP packet code = %d, want 1 (Challenge)", payload[0])
	}

	if err := d.StopSession(121, 232); err != nil {
		t.Logf("StopSession: %v", err)
	}
	drainEventsBest(t, d.EventsOut(), 2, 500*time.Millisecond)
}

// VALIDATES: proxy LCP with Auth-Protocol = MS-CHAPv2 (CHAP Algorithm
//
//	0x81) dispatches to runMSCHAPv2AuthPhase.
//
// PREVENTS: regression where MS-CHAPv2 collapses into CHAP-MD5 because
//
//	the algorithm byte was not consulted during method
//	selection.
func TestProxyLCPDispatchesMSCHAPv2(t *testing.T) {
	reg := newPipeRegistry()
	installPipeRegistry(t, reg)
	pair := newPipePair(reg, 15301)
	defer closeConn(pair.peerEnd)

	ops, _, _ := newFakeOps()
	d := newTestDriverNoResponder(&fakeBackend{}, ops)
	if err := d.Start(); err != nil {
		t.Fatalf("Start: %v", err)
	}
	defer d.Stop()

	lastSent := buildOptionStream([]LCPOption{
		mruOpt(1500),
		authProtoOpt(0xC223, 0x81),
	})
	plain := buildOptionStream([]LCPOption{
		mruOpt(1500),
		magicOpt(0xCAFEBABE),
	})
	d.SessionsIn() <- StartSession{
		TunnelID:            131,
		SessionID:           242,
		ChanFD:              15301,
		UnitFD:              904,
		UnitNum:             4,
		LNSMode:             true,
		MaxMRU:              1500,
		ProxyLCPInitialRecv: plain,
		ProxyLCPLastSent:    lastSent,
		ProxyLCPLastRecv:    plain,
	}

	proto, payload := readPeerFrame(t, pair.peerEnd)
	if proto != ProtoCHAP {
		t.Fatalf("first peer frame proto = 0x%04x, want ProtoCHAP 0x%04x",
			proto, ProtoCHAP)
	}
	// MS-CHAPv2 Challenge carries a 16-byte Value; frame layout is
	// Code=1, Identifier, Length(2), Value-Size=16, Value[16], Name...
	if len(payload) < 4+1+16 || payload[0] != 1 {
		t.Fatalf("first CHAP packet too short or wrong code: %x", payload)
	}
	if payload[4] != 16 {
		t.Errorf("Value-Size = %d, want 16 (MS-CHAPv2 Authenticator Challenge)",
			payload[4])
	}

	if err := d.StopSession(131, 242); err != nil {
		t.Logf("StopSession: %v", err)
	}
	drainEventsBest(t, d.EventsOut(), 2, 500*time.Millisecond)
}

// VALIDATES: sendConfigureRequest in the normal LCP path includes the
//
//	Auth-Protocol option (with CHAP Algorithm byte when
//	CHAP-MD5 is configured) based on configuredAuthMethod.
//
// PREVENTS: regression where StartSession.AuthMethod is set but the
//
//	CONFREQ still carries no Auth-Protocol option -- peers
//	would never authenticate.
func TestLocalCONFREQAdvertisesAuthMethod(t *testing.T) {
	cases := []struct {
		name      string
		method    AuthMethod
		wantProto uint16
		wantAlgo  byte
		wantAlgo1 bool
	}{
		{"PAP", AuthMethodPAP, 0xC023, 0, false},
		{"CHAP-MD5", AuthMethodCHAPMD5, 0xC223, 0x05, true},
		{"MS-CHAPv2", AuthMethodMSCHAPv2, 0xC223, 0x81, true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			reg := newPipeRegistry()
			installPipeRegistry(t, reg)
			pair := newPipePair(reg, 15400+int(tc.method))
			defer closeConn(pair.peerEnd)

			ops, _, _ := newFakeOps()
			d := newTestDriverNoResponder(&fakeBackend{}, ops)
			if err := d.Start(); err != nil {
				t.Fatalf("Start: %v", err)
			}
			defer d.Stop()

			d.SessionsIn() <- StartSession{
				TunnelID:   uint16(140 + int(tc.method)),
				SessionID:  uint16(240 + int(tc.method)),
				ChanFD:     15400 + int(tc.method),
				UnitFD:     905 + int(tc.method),
				UnitNum:    5 + int(tc.method),
				LNSMode:    true,
				MaxMRU:     1500,
				AuthMethod: tc.method,
			}

			proto, payload := readPeerFrame(t, pair.peerEnd)
			if proto != ProtoLCP {
				t.Fatalf("first peer frame proto = 0x%04x, want ProtoLCP", proto)
			}
			pkt, err := ParseLCPPacket(payload)
			if err != nil {
				t.Fatalf("ParseLCPPacket: %v", err)
			}
			if pkt.Code != LCPConfigureRequest {
				t.Fatalf("code = %d, want LCPConfigureRequest", pkt.Code)
			}
			opts, err := ParseLCPOptions(pkt.Data)
			if err != nil {
				t.Fatalf("ParseLCPOptions: %v", err)
			}
			data, ok := lookupOption(opts, LCPOptAuthProto)
			if !ok {
				t.Fatalf("local CONFREQ missing Auth-Protocol option")
			}
			if len(data) < 2 {
				t.Fatalf("Auth-Protocol data too short: %x", data)
			}
			gotProto := uint16(data[0])<<8 | uint16(data[1])
			if gotProto != tc.wantProto {
				t.Errorf("Auth-Protocol = 0x%04x, want 0x%04x",
					gotProto, tc.wantProto)
			}
			if tc.wantAlgo1 {
				if len(data) < 3 || data[2] != tc.wantAlgo {
					t.Errorf("Algorithm = %x, want 0x%02x", data, tc.wantAlgo)
				}
			} else if len(data) != 2 {
				t.Errorf("Auth-Protocol data = %x, want 2 bytes for PAP", data)
			}

			if err := d.StopSession(
				uint16(140+int(tc.method)),
				uint16(240+int(tc.method)),
			); err != nil {
				t.Logf("StopSession: %v", err)
			}
			drainEventsBest(t, d.EventsOut(), 2, 500*time.Millisecond)
		})
	}
}
