// Design: docs/research/l2tpv2-ze-integration.md -- PPP auth-phase CHAP-MD5, the reject teardown
// Related: chap.go -- runCHAPAuthPhase, the Success/Failure reply and the reject teardown
// Related: session_run.go -- fail, the EventSessionDown the transport tears the link down on
// Related: auth_pipe_helpers_test.go -- readPeerFrame

package ppp

import (
	"bytes"
	"encoding/binary"
	"net"
	"strings"
	"testing"
	"time"

	l2tpevents "github.com/ze-software/ze/internal/component/l2tp/events"
)

// newCHAPLinkTestSession builds an authenticator session on driverEnd whose
// lifecycle events (EventSessionDown, the request to the transport to take the
// link down) arrive on the returned channel.
func newCHAPLinkTestSession(driverEnd net.Conn) (*pppSession, chan AuthEvent, chan Event) {
	authEventsOut := make(chan AuthEvent, 4)
	eventsOut := make(chan Event, 4)
	frames := make(chan []byte, 4)
	s := &pppSession{
		tunnelID:      55,
		sessionID:     66,
		chanFile:      driverEnd,
		framesIn:      frames,
		eventsOut:     eventsOut,
		authEventsOut: authEventsOut,
		authRespCh:    make(chan authResponseMsg, 1),
		stopCh:        make(chan struct{}),
		sessStop:      make(chan struct{}),
		done:          make(chan struct{}),
		authTimeout:   2 * time.Second,
		logger:        discardLogger(),
	}
	readDone := make(chan error, 1)
	go s.readFrames(frames, readDone)
	return s, authEventsOut, eventsOut
}

// runCHAPExchange drives one CHAP exchange to the reply: it reads the
// Challenge, answers it with a Response, waits for the EventAuthRequest, hands
// the session the verifier's decision, and answers the reply packet read off the
// wire and the value runCHAPAuthPhase returned.
func runCHAPExchange(t *testing.T, peerEnd net.Conn, s *pppSession, authEventsOut chan AuthEvent, accept bool) ([]byte, bool) {
	t.Helper()
	handlerDone := make(chan bool, 1)
	go func() {
		handlerDone <- s.runCHAPAuthPhase()
	}()

	_, payload := readPeerFrame(t, peerEnd)
	challengeID := payload[1]

	resp := []byte{CHAPCodeResponse, challengeID, 0x00, 0x00, chapMD5DigestLen}
	resp = append(resp, bytes.Repeat([]byte{0x11}, chapMD5DigestLen)...)
	resp = append(resp, 'u')
	binary.BigEndian.PutUint16(resp[2:4], uint16(len(resp)))
	peerWrite := make([]byte, 64)
	off := WriteFrame(peerWrite, 0, ProtoCHAP, nil)
	off += copy(peerWrite[off:], resp)
	if _, err := peerEnd.Write(peerWrite[:off]); err != nil {
		t.Fatalf("peer write response: %v", err)
	}

	select {
	case <-authEventsOut:
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for EventAuthRequest")
	}
	s.authRespCh <- authResponseMsg{accept: accept, message: "decided"}

	_, reply := readPeerFrame(t, peerEnd)
	select {
	case ok := <-handlerDone:
		return reply, ok
	case <-time.After(2 * time.Second):
		t.Fatal("runCHAPAuthPhase did not return after the reply")
	}
	return nil, false
}

// RFC requirement: RFC1994-4.2-5 positive -- a Response whose Value the verifier
// rejects draws a CHAP Failure (Code 4) and then an EventSessionDown carrying
// the auth-rejected reason and Terminate-Cause NAS-Error, the request that makes
// the transport take the link down, and the authentication phase ends (false),
// so the session never reaches the Network-Layer Protocol phase.
// RFC requirement: RFC1994-2-1 positive -- the same exchange: the values do not
// match, and the connection is terminated.
func TestRFC1994CHAPFailureTerminatesTheLink(t *testing.T) {
	peerEnd, driverEnd := net.Pipe()
	defer closeConn(peerEnd)
	s, authEventsOut, eventsOut := newCHAPLinkTestSession(driverEnd)

	reply, ok := runCHAPExchange(t, peerEnd, s, authEventsOut, false)
	if reply[0] != CHAPCodeFailure {
		t.Fatalf("reply code %d, want %d (Failure)", reply[0], CHAPCodeFailure)
	}
	if ok {
		t.Error("runCHAPAuthPhase returned true after a Failure, want false (link terminated)")
	}
	select {
	case ev := <-eventsOut:
		down, isDown := ev.(EventSessionDown)
		if !isDown {
			t.Fatalf("lifecycle event %T, want EventSessionDown", ev)
		}
		if down.TunnelID != 55 || down.SessionID != 66 {
			t.Errorf("EventSessionDown for %d/%d, want 55/66", down.TunnelID, down.SessionID)
		}
		if !strings.Contains(down.Reason, "auth rejected") {
			t.Errorf("EventSessionDown reason %q, want it to name the auth rejection", down.Reason)
		}
		if down.Cause != l2tpevents.TerminateCauseNASError {
			t.Errorf("EventSessionDown cause %v, want NAS-Error", down.Cause)
		}
	default:
		t.Fatal("no EventSessionDown after a CHAP Failure: the link is not terminated")
	}
}

// RFC requirement: RFC1994-4.2-5 negative -- a Response the verifier accepts
// draws a CHAP Success (Code 3), no EventSessionDown, and the authentication
// phase continues (true), so the termination is owed to the Failure alone.
// RFC requirement: RFC1994-2-1 negative -- the values match, the authentication
// is acknowledged, and the connection stays up.
func TestRFC1994CHAPSuccessKeepsTheLink(t *testing.T) {
	peerEnd, driverEnd := net.Pipe()
	defer closeConn(peerEnd)
	s, authEventsOut, eventsOut := newCHAPLinkTestSession(driverEnd)

	reply, ok := runCHAPExchange(t, peerEnd, s, authEventsOut, true)
	if reply[0] != CHAPCodeSuccess {
		t.Fatalf("reply code %d, want %d (Success)", reply[0], CHAPCodeSuccess)
	}
	if !ok {
		t.Error("runCHAPAuthPhase returned false after a Success, want true (link kept)")
	}
	select {
	case ev := <-eventsOut:
		t.Errorf("lifecycle event %T after a CHAP Success, want none", ev)
	default:
	}
}
