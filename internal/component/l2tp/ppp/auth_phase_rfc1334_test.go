package ppp

import (
	"io"
	"net"
	"net/netip"
	"testing"
	"time"
)

// RFC 1334 Section 2.2.1 binds the PAP authenticator on two sides of one
// phase boundary: an Authenticate-Request received in the Authentication
// phase MUST be answered, and one received in any other phase MUST be
// silently discarded. RFC 1332 Section 3.3 binds the IP-Address value Ze puts
// in a Configure-Nak. The producers are runPAPAuthPhase (pap.go), which
// answers every parsed request with an Ack or a Nak; rejectUnsupportedProtocol
// (session_run.go), which handleFrame reaches for a PAP frame outside that
// phase and which discards a supported protocol without a reply; and
// buildNakOrReject (ncp.go), which writes the assigned peer address.

// papAuthRequestFrame is an Authenticate-Request with Identifier 0x5a,
// Peer-ID "alice" and Password "secret", in a PPP frame.
func papAuthRequestFrame() []byte {
	frame := make([]byte, 64)
	off := WriteFrame(frame, 0, ProtoPAP, nil)
	req := []byte{
		0x01, 0x5a, 0x00, 0x11,
		0x05, 'a', 'l', 'i', 'c', 'e',
		0x06, 's', 'e', 'c', 'r', 'e', 't',
	}
	copy(frame[off:], req)
	return frame[:off+len(req)]
}

// papReplyFor runs the PAP auth phase over a pipe, feeds it one
// Authenticate-Request, answers the auth event with the given decision, and
// returns the reply frame's protocol and payload.
func papReplyFor(t *testing.T, accept bool) (uint16, []byte) {
	t.Helper()
	peerEnd, driverEnd := net.Pipe()
	t.Cleanup(func() { closeConn(peerEnd) })
	s, authEventsOut := newAuthTestSession(driverEnd)

	handlerDone := make(chan bool, 1)
	go func() { handlerDone <- s.runPAPAuthPhase() }()

	if _, err := peerEnd.Write(papAuthRequestFrame()); err != nil {
		t.Fatalf("peer write request: %v", err)
	}
	select {
	case raw := <-authEventsOut:
		if _, ok := raw.(EventAuthRequest); !ok {
			t.Fatalf("first auth event %T, want EventAuthRequest", raw)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for EventAuthRequest")
	}
	s.authRespCh <- authResponseMsg{accept: accept, message: "answered"}

	proto, payload := readPeerFrame(t, peerEnd)

	// Drain the outcome event so the handler goroutine can return.
	select {
	case <-authEventsOut:
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for the auth outcome event")
	}
	if got := <-handlerDone; got != accept {
		t.Errorf("runPAPAuthPhase returned %v, want %v", got, accept)
	}
	return proto, payload
}

// RFC requirement: RFC1334-2.2.1-3 positive — an Authenticate-Request received in the Authentication phase is answered: an accepted request is returned an Authenticate-Ack carrying the request's Identifier
// RFC requirement: RFC1334-2.2.1-6 negative — the silent discard is confined to other phases: in the Authentication phase the request is not discarded but answered with a reply frame.
func TestPAPRequestInAuthPhaseIsAnswered(t *testing.T) {
	proto, payload := papReplyFor(t, true)
	if proto != ProtoPAP {
		t.Fatalf("reply proto = 0x%04x, want PAP", proto)
	}
	if len(payload) < papHeaderLen {
		t.Fatalf("reply payload too short: %d bytes", len(payload))
	}
	if payload[0] != PAPAuthenticateAck {
		t.Errorf("reply code = %d, want Authenticate-Ack", payload[0])
	}
	if payload[1] != 0x5a {
		t.Errorf("reply identifier = 0x%02x, want 0x5a", payload[1])
	}
}

// RFC requirement: RFC1334-2.2.1-3 negative — a rejected Authenticate-Request is never left without a reply: the reply is an Authenticate-Nak carrying the request's Identifier, not silence.
func TestPAPRejectedRequestIsStillAnswered(t *testing.T) {
	proto, payload := papReplyFor(t, false)
	if proto != ProtoPAP {
		t.Fatalf("reply proto = 0x%04x, want PAP", proto)
	}
	if len(payload) < papHeaderLen {
		t.Fatalf("reply payload too short: %d bytes", len(payload))
	}
	if payload[0] != PAPAuthenticateNak {
		t.Errorf("reply code = %d, want Authenticate-Nak", payload[0])
	}
	if payload[1] != 0x5a {
		t.Errorf("reply identifier = 0x%02x, want 0x5a", payload[1])
	}
}

// RFC requirement: RFC1334-2.2.1-6 positive — an Authenticate-Request received while LCP is still negotiating or already Opened outside the Authentication phase is silently discarded: no Ack, no Nak, no Protocol-Reject is written and the session goes on.
func TestPAPRequestOutsideAuthPhaseIsSilentlyDiscarded(t *testing.T) {
	for _, state := range []LCPState{LCPStateReqSent, LCPStateAckSent, LCPStateOpened} {
		s, rec, _ := newRFC1661Session(state)
		if term := s.handleFrame(papAuthRequestFrame()); term {
			t.Fatalf("state %s: handleFrame terminated the session on a PAP request", state)
		}
		if c := rec.count(); c != 0 {
			t.Fatalf("state %s: wrote %d frames for a PAP request outside the auth phase, want 0: % x",
				state, c, rec.all())
		}
	}
}

// authenticatedPAPSession completes the actual authentication handler against a
// recording transport, so repeat tests depend on the decision being cached.
func authenticatedPAPSession(t *testing.T, accept bool, state LCPState) (*pppSession, *frameRecorder) {
	t.Helper()
	s, rec, _ := newRFC1661Session(state)
	s.authTimeout = 2 * time.Second
	frames := make(chan []byte, 1)
	frame := getFrameBuf()
	n := copy(frame, papAuthRequestFrame())
	frames <- frame[:n]
	s.framesIn = frames
	s.authRespCh <- authResponseMsg{accept: accept, message: "decision"}
	if got := s.runPAPAuthPhase(); got != accept {
		t.Fatalf("authentication = %v, want %v", got, accept)
	}
	return s, rec
}

// TestPAPReanswersAfterAuthentication drops the first reply and delivers another
// request through the normal frame dispatcher during NCP negotiation and Opened.
//
// RFC requirement: RFC1334-2.2.1-4 positive -- a request repeated after PAP completes receives another reply through handleFrame.
// RFC requirement: RFC1334-2.2.1-5 positive -- a post-authentication request receives the original Ack Code with the incoming Identifier.
// RFC requirement: RFC1334-2.3-3 positive -- each emitted PAP reply copies the Identifier of the request that caused it.
// RFC requirement: RFC1334-2.3-3 negative -- reanswers cannot reuse the initial Identifier 0x5a when the repeated requests carry 0x5b and 0x5c.
// MUTATION: omit the decision cache or handleFrame PAP dispatch; the repeated request receives no reply.
// MUTATION: use the initial request's Identifier for reanswers; the 0x5b and 0x5c wire assertions fail.
func TestPAPReanswersAfterAuthentication(t *testing.T) {
	for _, state := range []LCPState{LCPStateAckSent, LCPStateOpened} {
		s, rec := authenticatedPAPSession(t, true, state)
		for _, id := range []uint8{0x5b, 0x5c} {
			frame := papAuthRequestFrame()
			frame[3] = id
			if term := s.handleFrame(frame); term {
				t.Fatal("repeat terminated the session")
			}
		}
		replies := decodeFrames(t, rec)
		if len(replies) != 3 {
			t.Fatalf("state %v: replies = %d, want 3", state, len(replies))
		}
		for i, id := range []uint8{0x5a, 0x5b, 0x5c} {
			if replies[i].Proto != ProtoPAP {
				t.Fatalf("reply protocol = %#x, want PAP", replies[i].Proto)
			}
			if replies[i].Pkt.Code != PAPAuthenticateAck || replies[i].Pkt.Identifier != id {
				t.Fatalf("reply %d = %+v, want Ack with Identifier %d", i, replies[i].Pkt, id)
			}
		}
	}
}

// TestPAPReanswerPreservesDecision changes credentials on a later request and
// verifies the cached Ack or Nak still determines the answer.
//
// RFC requirement: RFC1334-2.2.1-5 negative -- a later request cannot replace the original decision; both Ack and Nak Codes remain unchanged.
// MUTATION: hardcode Authenticate-Ack in reanswerPAP; the Nak case changes its decision.
func TestPAPReanswerPreservesDecision(t *testing.T) {
	for _, accept := range []bool{true, false} {
		s, rec := authenticatedPAPSession(t, accept, LCPStateOpened)
		frame := papAuthRequestFrame()
		frame[3] = 0xe7
		frame[7] = 'm'
		_, payload, _, err := ParseFrame(frame)
		if err != nil {
			t.Fatal(err)
		}
		if !s.reanswerPAP(payload) {
			t.Fatal("repeat write failed")
		}
		replies := decodeFrames(t, rec)
		if len(replies) != 2 {
			t.Fatalf("replies = %d, want 2", len(replies))
		}
		if replies[1].Pkt.Code != replies[0].Pkt.Code || replies[1].Pkt.Identifier != 0xe7 {
			t.Fatalf("original %+v, repeated %+v", replies[0].Pkt, replies[1].Pkt)
		}
	}
}

// TestPAPReanswerDiscardsMalformedRequest confines reanswers to valid requests.
//
// RFC requirement: RFC1334-2.2.1-4 negative -- malformed PAP packets and Ack packets after authentication receive no reply.
// MUTATION: bypass request validation in reanswerPAP; malformed requests receive replies.
func TestPAPReanswerDiscardsMalformedRequest(t *testing.T) {
	s, rec := authenticatedPAPSession(t, true, LCPStateOpened)
	for _, payload := range [][]byte{
		{PAPAuthenticateRequest},
		{PAPAuthenticateRequest, 1, 0, 6, 255, 0},
		{PAPAuthenticateRequest, 1, 0, 7, 0, 2, 'x'},
		{PAPAuthenticateAck, 1, 0, 5, 0},
	} {
		if !s.reanswerPAP(payload) {
			t.Fatal("malformed request terminated the session")
		}
	}
	if got := rec.count(); got != 1 {
		t.Fatalf("replies = %d, want the original reply only", got)
	}
}

type failingPAPReplyWriter struct {
	frameRecorder
	err error
}

func (w *failingPAPReplyWriter) Write([]byte) (int, error) { return 0, w.err }

// TestPAPReanswerWriteFailure makes failed and short reply writes terminate
// frame dispatch instead of leaving an authenticated peer on a broken link.
func TestPAPReanswerWriteFailure(t *testing.T) {
	for _, writeErr := range []error{io.ErrClosedPipe, nil} {
		s, _ := authenticatedPAPSession(t, true, LCPStateOpened)
		s.chanFile = &failingPAPReplyWriter{err: writeErr}
		if term := s.handleFrame(papAuthRequestFrame()); !term {
			t.Fatalf("write error %v: session continued", writeErr)
		}
	}
}

// TestPAPInitialReplyWriteFailureDoesNotAuthenticate refuses the initial
// decision when its reply write fails, including a short write with no error.
func TestPAPInitialReplyWriteFailureDoesNotAuthenticate(t *testing.T) {
	for _, writeErr := range []error{io.ErrClosedPipe, nil} {
		s, rec, _ := newRFC1661Session(LCPStateOpened)
		s.authTimeout = 2 * time.Second
		s.chanFile = &failingPAPReplyWriter{err: writeErr}
		frames := make(chan []byte, 1)
		frame := getFrameBuf()
		n := copy(frame, papAuthRequestFrame())
		frames <- frame[:n]
		s.framesIn = frames
		s.authRespCh <- authResponseMsg{accept: true}
		if s.runPAPAuthPhase() {
			t.Fatalf("write error %v: authentication succeeded", writeErr)
		}
		s.chanFile = rec
		if s.handleFrame(papAuthRequestFrame()) {
			t.Fatal("unauthenticated repeat terminated the session")
		}
		if rec.count() != 0 {
			t.Fatal("failed initial write left an authenticated decision to reanswer")
		}
	}
}

// ipcpRequest builds an IPCP Configure-Request whose IP-Address option
// carries the given address, or no IP-Address option when addr is invalid.
func ipcpRequest(addr netip.Addr) LCPPacket {
	buf := make([]byte, 32)
	n := WriteIPCPOptions(buf, 0, iPCPOptions{IPAddress: addr, HasIPAddress: addr.IsValid()})
	return LCPPacket{Code: LCPConfigureRequest, Identifier: 7, Data: buf[:n]}
}

// ipcpNakOptions runs one Configure-Request through buildNakOrReject on a
// session that assigns the peer the given address and returns the parsed
// options of the reply, which the test requires to be a Configure-Nak.
func ipcpNakOptions(t *testing.T, assigned netip.Addr, req LCPPacket) iPCPOptions {
	t.Helper()
	s, _, _ := newRFC1661Session(LCPStateOpened)
	s.peerIPv4 = assigned
	buf := make([]byte, 64)
	code, n := s.buildNakOrReject(AddressFamilyIPv4, req, buf, 0)
	if code != LCPConfigureNak {
		t.Fatalf("reply code = %d, want Configure-Nak", code)
	}
	opts, err := ParseIPCPOptions(buf[:n])
	if err != nil {
		t.Fatalf("Nak options do not parse: %v", err)
	}
	return opts
}

// RFC requirement: RFC1332-3.3-2 positive — the IP-Address Ze appends to a Configure-Nak is the address assigned to the peer for this session, which is what is acceptable as the remote IP-address: 10.0.0.2 when the peer asked for 10.9.9.9 and when it sent no IP-Address at all.
func TestIPCPNakCarriesTheAssignedRemoteAddress(t *testing.T) {
	assigned := netip.MustParseAddr("10.0.0.2")
	for _, req := range []LCPPacket{ipcpRequest(netip.MustParseAddr("10.9.9.9")), ipcpRequest(netip.Addr{})} {
		opts := ipcpNakOptions(t, assigned, req)
		if !opts.HasIPAddress {
			t.Fatalf("Nak carries no IP-Address for request % x", req.Data)
		}
		if opts.IPAddress != assigned {
			t.Errorf("Nak IP-Address = %s, want the assigned %s", opts.IPAddress, assigned)
		}
	}
}

// RFC requirement: RFC1332-3.3-2 negative — the Nak never carries an address that is not acceptable as the remote address: the peer's own unacceptable 10.9.9.9 is not echoed back, and with no address assigned the option is left out rather than written with an invalid value.
func TestIPCPNakNeverCarriesAnUnacceptableAddress(t *testing.T) {
	requested := netip.MustParseAddr("10.9.9.9")
	opts := ipcpNakOptions(t, netip.MustParseAddr("10.0.0.2"), ipcpRequest(requested))
	if opts.IPAddress == requested {
		t.Errorf("Nak echoed the peer's unacceptable address %s", requested)
	}

	s, _, _ := newRFC1661Session(LCPStateOpened)
	buf := make([]byte, 64)
	code, n := s.buildNakOrReject(AddressFamilyIPv4, ipcpRequest(requested), buf, 0)
	if code != LCPConfigureNak {
		t.Fatalf("reply code = %d, want Configure-Nak", code)
	}
	unassigned, err := ParseIPCPOptions(buf[:n])
	if err != nil {
		t.Fatalf("Nak options do not parse: %v", err)
	}
	if unassigned.HasIPAddress {
		t.Errorf("Nak carries IP-Address %s with no address assigned", unassigned.IPAddress)
	}
}
