package ppp

import (
	"bytes"
	"encoding/binary"
	"net"
	"testing"
	"time"
)

// VALIDATES: Phase 9 fix for /ze-review ISSUE 1. A non-CHAP frame
//
//	arriving during the CHAP wait (initial auth window or --
//	more critically -- periodic re-auth window) is routed
//	through handleFrame and silently dropped so the auth
//	wait continues, rather than failing the session on
//	"unexpected protocol".
//
// Uses a non-LCP non-CHAP protocol (IPCP, 0x8021) because handleFrame
// drops that with a Debug log; using LCP would exercise a pre-
// existing FSM re-entrance in `handleLCPPacket`'s Opened branch that
// is outside Phase 9 scope.
//
// PREVENTS: regression where a stray non-CHAP frame during the
//
//	reauth window tears the session down, which would make
//	any long-lived L2TP session with reauth enabled fragile
//	to incidental traffic.
func TestCHAPWaitSurvivesNonCHAPFrame(t *testing.T) {
	peerEnd, driverEnd := net.Pipe()
	defer closeConn(peerEnd)
	s, authEventsOut := newAuthTestSession(driverEnd)

	handlerDone := make(chan bool, 1)
	go func() {
		handlerDone <- s.runCHAPAuthPhase()
	}()

	// Drain the Challenge that runCHAPAuthPhase writes first.
	proto, payload := readPeerFrame(t, peerEnd)
	if proto != ProtoCHAP || payload[0] != CHAPCodeChallenge {
		t.Fatalf("first frame not CHAP Challenge: proto=0x%04x code=%d",
			proto, payload[0])
	}
	want := payload[1]

	// Peer sends an IPCP frame (0x8021) -- a valid PPP protocol but
	// not one the auth wait expects. handleFrame drops it at Debug
	// level; waitCHAPLike must continue waiting, not fail.
	const protoIPCP uint16 = 0x8021
	ipcp := make([]byte, MaxFrameLen)
	off := WriteFrame(ipcp, 0, protoIPCP, nil)
	// Minimal IPCP body: Code=1 (Configure-Request), Identifier, Len=4.
	ipcp[off+0] = 0x01
	ipcp[off+1] = 0x55
	ipcp[off+2] = 0x00
	ipcp[off+3] = 0x04
	off += 4
	if _, err := peerEnd.Write(ipcp[:off]); err != nil {
		t.Fatalf("peer write IPCP: %v", err)
	}

	// Assert no auth event fires -- handler is still waiting.
	select {
	case ev := <-authEventsOut:
		t.Fatalf("EventAuthRequest emitted during non-CHAP interlude: %T", ev)
	case <-time.After(100 * time.Millisecond):
	}

	// Deliver a matching-Identifier CHAP Response; handler must
	// dispatch EventAuthRequest, proving the wait survived.
	digest := bytes.Repeat([]byte{0x5A}, chapMD5DigestLen)
	writeCHAPResponseFrame(t, peerEnd, want, digest)

	select {
	case raw := <-authEventsOut:
		req, ok := raw.(EventAuthRequest)
		if !ok {
			t.Fatalf("auth event %T, want EventAuthRequest", raw)
		}
		if req.Method != AuthMethodCHAPMD5 {
			t.Errorf("Method = %v, want AuthMethodCHAPMD5", req.Method)
		}
		if req.Identifier != want {
			t.Errorf("Identifier = 0x%02x, want 0x%02x",
				req.Identifier, want)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("EventAuthRequest never fired after non-CHAP interlude + matching Response")
	}

	// Accept and drain Success so the handler exits cleanly.
	s.authRespCh <- authResponseMsg{accept: true, message: ""}
	successProto, successPayload := readPeerFrame(t, peerEnd)
	if successProto != ProtoCHAP || successPayload[0] != CHAPCodeSuccess {
		t.Errorf("expected CHAP Success after accept, got proto=0x%04x code=%d",
			successProto, successPayload[0])
	}

	select {
	case ok := <-handlerDone:
		if !ok {
			t.Errorf("runCHAPAuthPhase returned false after non-CHAP interlude + accept")
		}
	case <-time.After(2 * time.Second):
		t.Fatal("handler never returned")
	}
}

// chapReauthPeerName is the Name field the Phase 9 reauth tests
// embed in every peer-sent CHAP Response. Kept constant because ze
// does not validate the Name at any layer (RADIUS does, in
// production); varying it across subtests would only add noise.
const chapReauthPeerName = "bob"

// writeCHAPResponseFrame assembles a CHAP Response (code 2) with the
// given Identifier and MD5-sized Value, wrapped in a PPP frame, and
// writes it to peerEnd. Used by both silent-discard and periodic-
// reauth tests.
func writeCHAPResponseFrame(t *testing.T, peerEnd net.Conn, identifier uint8, digest []byte) {
	t.Helper()
	buf := make([]byte, MaxFrameLen)
	off := WriteFrame(buf, 0, ProtoCHAP, nil)
	resp := []byte{
		CHAPCodeResponse, identifier, 0x00, 0x00,
		byte(len(digest)),
	}
	resp = append(resp, digest...)
	resp = append(resp, []byte(chapReauthPeerName)...)
	binary.BigEndian.PutUint16(resp[2:4], uint16(len(resp)))
	copy(buf[off:], resp)
	off += len(resp)
	if _, err := peerEnd.Write(buf[:off]); err != nil {
		t.Fatalf("peer write CHAP Response: %v", err)
	}
}

// VALIDATES: Phase 9 AC-16, RFC 1994 §4.1 silent-discard. A CHAP
//
//	Response whose Identifier does not match the outstanding
//	Challenge is dropped with a debug log; the handler keeps
//	reading until a matching Response arrives (or the auth
//	timer fires). On the matching Response the handler emits
//	EventAuthRequest as usual.
//
// PREVENTS: regression where the pre-Phase-9 fail-immediately path is
//
//	restored, which would kill the session on any stale or
//	duplicated CHAP Response packet.
func TestCHAPIdentifierMismatchSilentDiscard(t *testing.T) {
	peerEnd, driverEnd := net.Pipe()
	defer closeConn(peerEnd)
	s, authEventsOut := newAuthTestSession(driverEnd)

	handlerDone := make(chan bool, 1)
	go func() {
		handlerDone <- s.runCHAPAuthPhase()
	}()

	// Read the Challenge frame so we know the outstanding Identifier.
	proto, payload := readPeerFrame(t, peerEnd)
	if proto != ProtoCHAP || payload[0] != CHAPCodeChallenge {
		t.Fatalf("first driver frame proto=0x%04x code=%d, want CHAP Challenge",
			proto, payload[0])
	}
	want := payload[1]

	digest := bytes.Repeat([]byte{0x5A}, chapMD5DigestLen)

	// Send two mismatched-Identifier Responses; both must be silently
	// discarded.
	writeCHAPResponseFrame(t, peerEnd, want+1, digest)
	writeCHAPResponseFrame(t, peerEnd, want+2, digest)

	// Assert no EventAuthRequest fires within a generous window --
	// proves the handler is truly waiting, not dispatching.
	select {
	case ev := <-authEventsOut:
		t.Fatalf("EventAuthRequest emitted on mismatched identifier: %T %+v", ev, ev)
	case <-time.After(100 * time.Millisecond):
	}

	// Now deliver the matching Identifier; handler must dispatch.
	writeCHAPResponseFrame(t, peerEnd, want, digest)

	select {
	case raw := <-authEventsOut:
		req, ok := raw.(EventAuthRequest)
		if !ok {
			t.Fatalf("auth event %T, want EventAuthRequest", raw)
		}
		if req.Method != AuthMethodCHAPMD5 {
			t.Errorf("Method = %v, want AuthMethodCHAPMD5", req.Method)
		}
		if req.Identifier != want {
			t.Errorf("Identifier = 0x%02x, want 0x%02x", req.Identifier, want)
		}
		if req.Username != "bob" {
			t.Errorf("Username = %q, want %q", req.Username, "bob")
		}
	case <-time.After(2 * time.Second):
		t.Fatal("EventAuthRequest never fired after matching Response")
	}

	// Accept so the handler proceeds to write its Success frame.
	// The pipe is synchronous: drain the Success frame off peerEnd
	// before waiting for the handler's return value or the write
	// blocks forever.
	s.authRespCh <- authResponseMsg{accept: true, message: ""}
	successProto, successPayload := readPeerFrame(t, peerEnd)
	if successProto != ProtoCHAP {
		t.Errorf("Success proto = 0x%04x, want ProtoCHAP", successProto)
	}
	if len(successPayload) < 1 || successPayload[0] != CHAPCodeSuccess {
		t.Errorf("expected CHAP Success, got code=%d payload=%x",
			successPayload[0], successPayload)
	}

	select {
	case ok := <-handlerDone:
		if !ok {
			t.Errorf("runCHAPAuthPhase returned false after matching Response + accept")
		}
	case <-time.After(2 * time.Second):
		t.Fatal("handler never returned")
	}
}

// VALIDATES: Phase 9 AC-14 periodic re-auth for CHAP-MD5. After
//
//	initial authentication succeeds and LCP reaches Opened,
//	the session's reauth ticker fires every ReauthInterval
//	and runs a fresh CHAP exchange (new Challenge with new
//	Identifier). When the auth handler rejects the re-auth,
//	the session tears down with EventSessionDown.
//
// PREVENTS: regression where ReauthInterval is threaded through config
//
//	but never activates a ticker, or where the re-auth path
//	reuses a stale Challenge Identifier, or where a rejected
//	re-auth fails to tear the session down.
func TestPeriodicReauthCHAPTearsDownOnReject(t *testing.T) {
	reg := newPipeRegistry()
	installPipeRegistry(t, reg)
	pair := newPipePair(reg, 18001)
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
		TunnelID:       411,
		SessionID:      511,
		ChanFD:         18001,
		UnitFD:         911,
		UnitNum:        31,
		LNSMode:        true,
		MaxMRU:         1500,
		AuthMethod:     AuthMethodCHAPMD5,
		AuthTimeout:    3 * time.Second,
		ReauthInterval: 150 * time.Millisecond,
		DisableIPCP:    true,
		DisableIPv6CP:  true,
	}

	<-peerDone

	// Round 1 -- initial auth.
	initialID := readCHAPChallengeAndRespond(t, pair.peerEnd)
	// Handler parks on authRespCh; wait for EventAuthRequest so we
	// know it saw our Response, then accept. readCHAPReplyCode
	// drains the Success frame that runCHAPAuthPhase writes back so
	// the main loop can move on.
	awaitAuthEventOfType[EventAuthRequest](t, d.AuthEventsOut(), 2*time.Second)
	if err := d.AuthResponse(411, 511, true, "", nil); err != nil {
		t.Fatalf("AuthResponse(initial accept): %v", err)
	}
	if code := readCHAPReplyCode(t, pair.peerEnd); code != CHAPCodeSuccess {
		t.Fatalf("initial reply code = %d, want CHAPCodeSuccess", code)
	}
	awaitAuthEventOfType[EventAuthSuccess](t, d.AuthEventsOut(), 2*time.Second)
	// EventLCPUp + EventSessionUp fire after initial auth; drain
	// them so the reauth-tick lifecycle events stand alone.
	drainEventsBest(t, d.EventsOut(), 2, 500*time.Millisecond)

	// Round 2 -- periodic re-auth fires ~150ms later. Identifier
	// MUST differ from round 1 (RFC 1994 §4.1).
	reauthID := readCHAPChallengeAndRespond(t, pair.peerEnd)
	if reauthID == initialID {
		t.Errorf("reauth Challenge Identifier = 0x%02x, want fresh (!= 0x%02x)",
			reauthID, initialID)
	}

	awaitAuthEventOfType[EventAuthRequest](t, d.AuthEventsOut(), 2*time.Second)
	if err := d.AuthResponse(411, 511, false, "credentials expired", nil); err != nil {
		t.Fatalf("AuthResponse(reauth reject): %v", err)
	}
	if code := readCHAPReplyCode(t, pair.peerEnd); code != CHAPCodeFailure {
		t.Errorf("reauth reply code = %d, want CHAPCodeFailure", code)
	}
	awaitAuthEventOfType[EventAuthFailure](t, d.AuthEventsOut(), 2*time.Second)

	gotDown := false
	deadline := time.After(2 * time.Second)
	for !gotDown {
		select {
		case ev := <-d.EventsOut():
			if _, ok := ev.(EventSessionDown); ok {
				gotDown = true
			}
		case <-deadline:
			t.Fatal("EventSessionDown never emitted after reauth reject")
		}
	}
}

// readCHAPReplyCode reads one CHAP frame from peerEnd and returns its
// code byte (Success / Failure). Fails the test on any non-CHAP frame.
func readCHAPReplyCode(t *testing.T, peerEnd net.Conn) uint8 {
	t.Helper()
	proto, payload := readPeerFrame(t, peerEnd)
	if proto != ProtoCHAP {
		t.Fatalf("reply proto = 0x%04x, want ProtoCHAP", proto)
	}
	if len(payload) < 1 {
		t.Fatalf("reply payload empty")
	}
	return payload[0]
}

// readCHAPChallengeAndRespond reads one CHAP Challenge from peerEnd,
// sends a matching Response, and returns the Identifier used in the
// exchange. Used by the reauth test to prove each round uses a fresh
// Identifier.
func readCHAPChallengeAndRespond(t *testing.T, peerEnd net.Conn) uint8 {
	t.Helper()
	proto, payload := readPeerFrame(t, peerEnd)
	if proto != ProtoCHAP {
		t.Fatalf("challenge proto = 0x%04x, want ProtoCHAP", proto)
	}
	if len(payload) < chapHeaderLen+1+chapChallengeValueLen {
		t.Fatalf("challenge payload too short: %x", payload)
	}
	if payload[0] != CHAPCodeChallenge {
		t.Fatalf("code = %d, want CHAPCodeChallenge", payload[0])
	}
	id := payload[1]

	// Respond with a stand-in digest (ze never validates it; the auth
	// handler in production is RADIUS, here it is the test).
	digest := bytes.Repeat([]byte{0x33}, chapMD5DigestLen)
	writeCHAPResponseFrame(t, peerEnd, id, digest)
	return id
}

// awaitAuthEventOfType pops auth events from the channel until one of
// the requested type arrives, or the timeout fires. Other event types
// are discarded. Shared helper for the reauth flow, which posts
// multiple AuthEvent shapes per round.
func awaitAuthEventOfType[T AuthEvent](t *testing.T, ch <-chan AuthEvent, timeout time.Duration) T {
	t.Helper()
	deadline := time.After(timeout)
	for {
		select {
		case ev := <-ch:
			if typed, ok := ev.(T); ok {
				return typed
			}
		case <-deadline:
			var zero T
			t.Fatalf("timed out awaiting auth event of type %T", zero)
			return zero
		}
	}
}
