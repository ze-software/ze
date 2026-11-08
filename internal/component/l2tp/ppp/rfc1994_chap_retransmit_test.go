package ppp

import (
	"bytes"
	"encoding/binary"
	"net"
	"strings"
	"testing"
	"time"
)

// collectPeerFrames reads every frame the session writes to peerEnd and
// delivers its payload on the returned channel. A Challenge the session
// retransmits while the test is busy therefore never blocks the session's
// write on the synchronous net.Pipe. The reader ends, closing the channel,
// when peerEnd is closed.
func collectPeerFrames(peerEnd net.Conn) <-chan []byte {
	out := make(chan []byte, 2*chapChallengeMax)
	go collectPeerFramesLoop(peerEnd, out)
	return out
}

func collectPeerFramesLoop(peerEnd net.Conn, out chan<- []byte) {
	defer close(out)
	for {
		buf := make([]byte, MaxFrameBufLen)
		n, err := peerEnd.Read(buf)
		if err != nil {
			return
		}
		_, payload, _, perr := ParseFrame(buf[:n])
		if perr != nil {
			return
		}
		out <- payload
	}
}

// nextPeerPayload returns the next payload collectPeerFrames delivered,
// failing the test when none arrives within peerFrameReadTimeout.
func nextPeerPayload(t *testing.T, frames <-chan []byte) []byte {
	t.Helper()
	select {
	case p, ok := <-frames:
		if !ok {
			t.Fatal("peer end closed before the next frame")
		}
		return p
	case <-time.After(peerFrameReadTimeout):
		t.Fatal("timed out waiting for the next frame from the session")
	}
	return nil
}

// writeCHAPMD5Response writes a CHAP-MD5 Response frame answering the
// Challenge with the given Identifier, carrying a stand-in digest.
func writeCHAPMD5Response(t *testing.T, peerEnd net.Conn, identifier, digest byte) {
	t.Helper()
	resp := []byte{CHAPCodeResponse, identifier, 0x00, 0x00, chapMD5DigestLen}
	resp = append(resp, bytes.Repeat([]byte{digest}, chapMD5DigestLen)...)
	resp = append(resp, 'b', 'o', 'b')
	binary.BigEndian.PutUint16(resp[2:4], uint16(len(resp)))
	frame := make([]byte, MaxFrameBufLen)
	off := WriteFrame(frame, 0, ProtoCHAP, nil)
	off += copy(frame[off:], resp)
	if _, err := peerEnd.Write(frame[:off]); err != nil {
		t.Fatalf("peer write response: %v", err)
	}
}

// challengeOf checks that payload is a Challenge with a 16-octet Value
// and returns its Identifier and a copy of the Value. CHAP-MD5 and
// MS-CHAPv2 Challenges both carry 16 octets.
func challengeOf(t *testing.T, payload []byte) (uint8, []byte) {
	t.Helper()
	const valueLen = chapChallengeValueLen
	if len(payload) < chapHeaderLen+1+valueLen {
		t.Fatalf("challenge payload too short: %d bytes", len(payload))
	}
	if payload[0] != CHAPCodeChallenge {
		t.Fatalf("code = %d, want CHAPCodeChallenge", payload[0])
	}
	if int(payload[4]) != valueLen {
		t.Fatalf("Value-Size = %d, want %d", payload[4], valueLen)
	}
	return payload[1], bytes.Clone(payload[5 : 5+valueLen])
}

// VALIDATES: a CHAP-MD5 Challenge the peer never answers is followed by
// another Challenge one restart interval later, carrying a new Identifier
// and a new Value; a late Response to the first Challenge is silently
// discarded, and the Response to the second completes the handshake with
// a Success carrying the second Identifier.
// PREVENTS: the Ze LAC to Ze LNS loss, where the LNS's only Challenge is
// lost before the LAC's bridge exists and the session dies at the auth
// timeout.
//
// RFC requirement: RFC1994-4.1-2 positive -- with the first Challenge unanswered, runCHAPAuthPhase sends a second Challenge and completes on the Response to it.
func TestCHAPChallengeRetransmittedUntilResponse(t *testing.T) {
	peerEnd, driverEnd := net.Pipe()
	defer closeConn(peerEnd)
	s, authEventsOut := newAuthTestSession(driverEnd)
	s.chapRestart = 300 * time.Millisecond
	frames := collectPeerFrames(peerEnd)

	handlerDone := make(chan bool, 1)
	go func() {
		handlerDone <- s.runCHAPAuthPhase()
	}()

	firstID, firstValue := challengeOf(t, nextPeerPayload(t, frames))
	// The first Challenge is "lost": the peer does not answer it.
	secondID, secondValue := challengeOf(t, nextPeerPayload(t, frames))
	if secondID == firstID {
		t.Errorf("retransmitted Challenge kept Identifier 0x%02x; RFC 1994 Section 4.1 changes it each time a Challenge is sent", firstID)
	}
	if bytes.Equal(secondValue, firstValue) {
		t.Errorf("retransmitted Challenge kept Value %x; RFC 1994 Section 4.1 changes it each time a Challenge is sent", firstValue)
	}

	// A Response to the earlier Challenge arrives late: it does not carry
	// the current Identifier, so it is silently discarded.
	writeCHAPMD5Response(t, peerEnd, firstID, 0x11)
	writeCHAPMD5Response(t, peerEnd, secondID, 0x22)

	select {
	case raw := <-authEventsOut:
		evt, ok := raw.(EventAuthRequest)
		if !ok {
			t.Fatalf("first auth event %T, want EventAuthRequest", raw)
		}
		if evt.Identifier != secondID {
			t.Fatalf("EventAuthRequest Identifier = 0x%02x, want 0x%02x (the late Response to 0x%02x must be discarded)",
				evt.Identifier, secondID, firstID)
		}
		if !bytes.Equal(evt.Challenge, secondValue) {
			t.Errorf("EventAuthRequest Challenge = %x, want the second Value %x", evt.Challenge, secondValue)
		}
		if !bytes.Equal(evt.Response, bytes.Repeat([]byte{0x22}, chapMD5DigestLen)) {
			t.Errorf("EventAuthRequest Response = %x, want the second Response's digest", evt.Response)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for EventAuthRequest")
	}

	s.authRespCh <- authResponseMsg{accept: true, message: "welcome"}
	reply := nextPeerPayload(t, frames)
	if reply[0] != CHAPCodeSuccess {
		t.Fatalf("reply code = %d, want CHAPCodeSuccess", reply[0])
	}
	if reply[1] != secondID {
		t.Errorf("Success Identifier = 0x%02x, want 0x%02x", reply[1], secondID)
	}
	if !<-handlerDone {
		t.Error("runCHAPAuthPhase returned false, want true after the Response to the retransmitted Challenge")
	}
}

// VALIDATES: once a Response carrying the current Identifier is handed to
// the authenticator, no further Challenge is sent however long the
// verification takes, and a refusal ends the round through the Failure
// path rather than by retransmitting.
// PREVENTS: a retransmit timer left running beside a slow RADIUS or local
// verification, which would re-challenge a peer whose Response is valid.
func TestCHAPNoChallengeWhileVerifying(t *testing.T) {
	peerEnd, driverEnd := net.Pipe()
	defer closeConn(peerEnd)
	s, authEventsOut := newAuthTestSession(driverEnd)
	s.chapRestart = 20 * time.Millisecond
	frames := collectPeerFrames(peerEnd)

	handlerDone := make(chan bool, 1)
	go func() {
		handlerDone <- s.runCHAPAuthPhase()
	}()

	id, _ := challengeOf(t, nextPeerPayload(t, frames))
	writeCHAPMD5Response(t, peerEnd, id, 0x33)
	select {
	case raw := <-authEventsOut:
		if _, ok := raw.(EventAuthRequest); !ok {
			t.Fatalf("first auth event %T, want EventAuthRequest", raw)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for EventAuthRequest")
	}

	// The verifier is slow: ten restart intervals pass with no decision.
	select {
	case p := <-frames:
		t.Fatalf("frame written while the Response is being verified: %x", p)
	case <-time.After(10 * s.chapRestart):
	}

	s.authRespCh <- authResponseMsg{accept: false, message: "nope"}
	reply := nextPeerPayload(t, frames)
	if reply[0] != CHAPCodeFailure {
		t.Fatalf("reply code = %d, want CHAPCodeFailure", reply[0])
	}
	if reply[1] != id {
		t.Errorf("Failure Identifier = 0x%02x, want 0x%02x", reply[1], id)
	}
	select {
	case ok := <-handlerDone:
		if ok {
			t.Error("runCHAPAuthPhase returned true after a refusal")
		}
	case <-time.After(2 * time.Second):
		t.Fatal("runCHAPAuthPhase did not return after the refusal")
	}
}

// VALIDATES: a peer that answers no Challenge receives exactly
// chapChallengeMax Challenges, each with its own Identifier, after which
// the session fails, before the longer authTimeout, with a reason naming
// the unanswered Challenges and an eventAuthFailure.
// PREVENTS: an unbounded retransmission loop, and an exhaustion that ends
// the session without saying why.
func TestCHAPChallengeRetryLimitEndsSession(t *testing.T) {
	peerEnd, driverEnd := net.Pipe()
	s, authEventsOut := newAuthTestSession(driverEnd)
	events := make(chan Event, 4)
	s.eventsOut = events
	s.chapRestart = 20 * time.Millisecond
	s.authTimeout = 10 * time.Second
	frames := collectPeerFrames(peerEnd)

	handlerDone := make(chan bool, 1)
	go func() {
		handlerDone <- s.runCHAPAuthPhase()
	}()

	select {
	case ok := <-handlerDone:
		if ok {
			t.Fatal("runCHAPAuthPhase returned true with no Response")
		}
	case <-time.After(5 * time.Second):
		t.Fatal("runCHAPAuthPhase did not give up after the retry limit")
	}
	// Every write completed before the handler returned, so closing the
	// peer end now leaves exactly the frames it sent in the channel.
	closeConn(peerEnd)
	seen := make(map[uint8]bool)
	count := 0
	for p := range frames {
		id, _ := challengeOf(t, p)
		if seen[id] {
			t.Errorf("Identifier 0x%02x sent twice", id)
		}
		seen[id] = true
		count++
	}
	if count != chapChallengeMax {
		t.Errorf("Challenges sent = %d, want %d", count, chapChallengeMax)
	}

	select {
	case raw := <-authEventsOut:
		fail, ok := raw.(eventAuthFailure)
		if !ok {
			t.Fatalf("auth event %T, want eventAuthFailure", raw)
		}
		if fail.Reason != "timeout" {
			t.Errorf("Reason = %q, want \"timeout\"", fail.Reason)
		}
	default:
		t.Fatal("no eventAuthFailure after the retry limit")
	}
	select {
	case raw := <-events:
		down, ok := raw.(EventSessionDown)
		if !ok {
			t.Fatalf("session event %T, want EventSessionDown", raw)
		}
		if !strings.Contains(down.Reason, "Challenges") {
			t.Errorf("EventSessionDown Reason = %q, want it to name the unanswered Challenges", down.Reason)
		}
	default:
		t.Fatal("no EventSessionDown after the retry limit")
	}
}

// VALIDATES: an MS-CHAPv2 Challenge the peer never answers is followed by
// another with a new Identifier and Value, and the Response to it reaches
// the auth handler with that Identifier and Value.
// PREVENTS: MS-CHAPv2 keeping the single-Challenge behavior: it shares
// waitCHAPLike with CHAP-MD5.
func TestMSCHAPv2ChallengeRetransmittedUntilResponse(t *testing.T) {
	peerEnd, driverEnd := net.Pipe()
	defer closeConn(peerEnd)
	s, authEventsOut := newAuthTestSession(driverEnd)
	s.chapRestart = 300 * time.Millisecond
	frames := collectPeerFrames(peerEnd)

	handlerDone := make(chan bool, 1)
	go func() {
		handlerDone <- s.runMSCHAPv2AuthPhase()
	}()

	firstID, firstValue := challengeOf(t, nextPeerPayload(t, frames))
	secondID, secondValue := challengeOf(t, nextPeerPayload(t, frames))
	if secondID == firstID {
		t.Errorf("retransmitted Challenge kept Identifier 0x%02x", firstID)
	}
	if bytes.Equal(secondValue, firstValue) {
		t.Errorf("retransmitted Challenge kept Value %x", firstValue)
	}

	peerChallenge := bytes.Repeat([]byte{0xAA}, mschapv2PeerChallengeLen)
	ntResponse := bytes.Repeat([]byte{0xBB}, mschapv2NTResponseLen)
	respPayload := buildMSCHAPv2ResponsePayload(secondID, peerChallenge, ntResponse, "bob")
	frame := make([]byte, MaxFrameBufLen)
	off := WriteFrame(frame, 0, ProtoCHAP, nil)
	off += copy(frame[off:], respPayload)
	if _, err := peerEnd.Write(frame[:off]); err != nil {
		t.Fatalf("peer write response: %v", err)
	}

	select {
	case raw := <-authEventsOut:
		evt, ok := raw.(EventAuthRequest)
		if !ok {
			t.Fatalf("first auth event %T, want EventAuthRequest", raw)
		}
		if evt.Identifier != secondID {
			t.Errorf("EventAuthRequest Identifier = 0x%02x, want 0x%02x", evt.Identifier, secondID)
		}
		if !bytes.Equal(evt.Challenge, secondValue) {
			t.Errorf("EventAuthRequest Challenge = %x, want the second Value %x", evt.Challenge, secondValue)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for EventAuthRequest")
	}
	// End the round with a refusal so the handler goroutine returns.
	s.authRespCh <- authResponseMsg{accept: false, message: "nope"}
	select {
	case <-handlerDone:
	case <-time.After(2 * time.Second):
		t.Fatal("runMSCHAPv2AuthPhase did not return after the refusal")
	}
}
