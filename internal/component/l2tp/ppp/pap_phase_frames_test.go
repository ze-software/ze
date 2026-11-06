// Design: docs/guide/l2tp.md -- LCP stays live while PAP waits for its first request.

package ppp

import (
	"testing"
)

// papPhaseQueue loads pool buffers the way readFrames does, because the PAP wait
// returns every frame it consumes to the pool.
func papPhaseQueue(frames ...[]byte) chan []byte {
	queue := make(chan []byte, len(frames))
	for _, frame := range frames {
		buf := getFrameBuf()
		n := copy(buf, frame)
		queue <- buf[:n]
	}
	return queue
}

// TestPAPWaitAnswersTerminateRequest: a peer that ends the link during the wait
// for its first PAP request MUST be answered. RFC 1661 Section 5.5: "Upon
// reception of a Terminate-Request, a Terminate-Ack MUST be transmitted."
func TestPAPWaitAnswersTerminateRequest(t *testing.T) {
	s, rec, _ := newRFC1661Session(LCPStateOpened)
	s.framesIn = papPhaseQueue(lcpFrame(ProtoLCP, LCPTerminateRequest, 0x21, nil))
	if s.runPAPAuthPhase() {
		t.Fatal("a Terminate-Request completed authentication")
	}
	ack, found := findCode(t, rec, LCPTerminateAck)
	if !found || ack.Identifier != 0x21 {
		t.Fatalf("Terminate-Ack = %+v, found=%t; want Identifier 0x21", ack, found)
	}
	if state := s.currentState(); state != LCPStateStopping {
		t.Fatalf("state = %s, want stopping", state)
	}
	if len(s.authEventsOut) != 0 {
		t.Fatal("a Terminate-Request reached the auth handler")
	}
}

// TestPAPWaitRenegotiatesOnRetransmittedRequest: a peer that lost Ze's
// Configure-Ack retransmits its Configure-Request, which leaves Opened through
// the shared automaton instead of being discarded.
func TestPAPWaitRenegotiatesOnRetransmittedRequest(t *testing.T) {
	s, rec, _ := newRFC1661Session(LCPStateOpened)
	s.framesIn = papPhaseQueue(lcpFrame(ProtoLCP, LCPConfigureRequest, 0x22, nil))
	if s.runPAPAuthPhase() {
		t.Fatal("a Configure-Request completed authentication")
	}
	ack, found := findCode(t, rec, LCPConfigureAck)
	if !found || ack.Identifier != 0x22 {
		t.Fatalf("Configure-Ack = %+v, found=%t; want Identifier 0x22", ack, found)
	}
	if _, found := findCode(t, rec, LCPConfigureRequest); !found {
		t.Fatal("renegotiation sent no Configure-Request")
	}
	if state := s.currentState(); state != LCPStateAckSent {
		t.Fatalf("state = %s, want ack-sent", state)
	}
}

// TestPAPWaitContinuesAfterOtherFrames: frames that change no LCP state keep the
// wait going. An Echo-Request is answered, an unsupported protocol is rejected,
// an IPCP packet before authentication completes is silently discarded, and the
// PAP request behind them still authenticates. A wait that left on the first
// non-PAP frame would pass the two tests above and fail here.
func TestPAPWaitContinuesAfterOtherFrames(t *testing.T) {
	// IPCP stays disabled here, where a handled frame would draw a fourth reply,
	// a Protocol-Reject: the silence proves the discard before authentication.
	s, rec, _ := newRFC1661Session(LCPStateOpened)
	s.authRespCh <- authResponseMsg{accept: true}
	s.framesIn = papPhaseQueue(
		lcpFrame(ProtoLCP, LCPEchoRequest, 0x23, []byte{0, 0, 0, 0}),
		lcpFrame(protoMPLSCP, LCPConfigureRequest, 0x24, nil),
		lcpFrame(ProtoIPCP, LCPConfigureRequest, 0x25, nil),
		papAuthRequestFrame(),
	)
	if !s.runPAPAuthPhase() {
		t.Fatal("PAP request behind other frames did not authenticate")
	}
	replies := decodeFrames(t, rec)
	if len(replies) != 3 {
		t.Fatalf("replies = %+v, want Echo-Reply, Protocol-Reject, PAP Ack", replies)
	}
	if replies[0].Proto != ProtoLCP || replies[0].Pkt.Code != LCPEchoReply || replies[0].Pkt.Identifier != 0x23 {
		t.Fatalf("first reply = %+v, want Echo-Reply", replies[0])
	}
	if replies[1].Proto != ProtoLCP || replies[1].Pkt.Code != LCPProtocolReject {
		t.Fatalf("second reply = %+v, want Protocol-Reject", replies[1])
	}
	if replies[2].Proto != ProtoPAP || replies[2].Pkt.Code != PAPAuthenticateAck || replies[2].Pkt.Identifier != 0x5a {
		t.Fatalf("third reply = %+v, want PAP Ack for the request", replies[2])
	}
}
