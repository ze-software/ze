// Design: docs/research/l2tpv2-implementation-guide.md -- PPP FSM (RFC 1661 §4)
// Related: ppp_fsm.go -- LCPDoTransition RTR and RUC cells across all states
// Related: session_run.go -- handleLCPPacket, sendTerminateAck, sendCodeReject
// Related: rfc1661_test.go -- newRFC1661Session, decodeFrames and the frame recorder
// Related: rfc/short/rfc1661.md -- RFC1661-5.5-1 and RFC1661-5.6-1

package ppp

import (
	"bytes"
	"testing"
)

// TestRFC1661TerminateAckAcrossAllStates drives handleLCPPacket with a
// Terminate-Request in every packet-receiving LCP state defined by RFC 1661
// Section 4.1 (Closed, Stopped, Closing, Stopping, Req-Sent, Ack-Rcvd,
// Ack-Sent and Opened).
//
// VALIDATES: RFC 1661 Section 5.5 ("Upon reception of a Terminate-Request, a
// Terminate-Ack MUST be transmitted.") holds in every state of the automaton,
// not only Opened, with the request Identifier echoed and the exact Section 4.1
// next state entered.
// PREVENTS: a missing sta action in any non-Opened state's LCPEventRTR cell of
// LCPDoTransition (ppp_fsm.go) going undetected by Opened-only tests.
//
// RFC requirement: RFC1661-5.5-1 positive -- handleLCPPacket (session_run.go)
// transmits exactly one LCP Terminate-Ack echoing the request Identifier and
// transitions to the RFC 1661 Section 4.1 next state for a Terminate-Request
// received in Closed, Stopped, Closing, Stopping, Req-Sent, Ack-Rcvd, Ack-Sent
// and Opened.
//
// MUTATION: drop LCPActSTA from any state's LCPEventRTR case in
// LCPDoTransition (ppp_fsm.go), or break sendTerminateAck (session_run.go):
// that state's subtest observes zero Terminate-Ack frames.
func TestRFC1661TerminateAckAcrossAllStates(t *testing.T) {
	cases := []struct {
		state    LCPState
		wantNext LCPState
		wantEnd  bool
	}{
		{LCPStateClosed, LCPStateClosed, true},
		{LCPStateStopped, LCPStateStopped, true},
		{LCPStateClosing, LCPStateClosing, false},
		{LCPStateStopping, LCPStateStopping, false},
		{LCPStateReqSent, LCPStateReqSent, false},
		{LCPStateAckRcvd, LCPStateReqSent, false},
		{LCPStateAckSent, LCPStateReqSent, false},
		{LCPStateOpened, LCPStateStopping, false},
	}

	const reqID uint8 = 0x77
	for _, tc := range cases {
		t.Run(tc.state.String(), func(t *testing.T) {
			s, rec, _ := newRFC1661Session(tc.state)
			ended := s.handleLCPPacket(LCPPacket{Code: LCPTerminateRequest, Identifier: reqID})
			if ended != tc.wantEnd {
				t.Fatalf("handleLCPPacket ended = %v, want %v", ended, tc.wantEnd)
			}
			if got := s.currentState(); got != tc.wantNext {
				t.Fatalf("state = %s, want %s", got, tc.wantNext)
			}
			frames := decodeFrames(t, rec)
			if len(frames) != 1 {
				t.Fatalf("recorded %d frames, want exactly 1 Terminate-Ack", len(frames))
			}
			if frames[0].Proto != ProtoLCP {
				t.Fatalf("reply proto = %#04x, want ProtoLCP (%#04x)", frames[0].Proto, ProtoLCP)
			}
			if frames[0].Pkt.Code != LCPTerminateAck {
				t.Fatalf("reply code = %d (%s), want Terminate-Ack", frames[0].Pkt.Code, LCPCodeName(frames[0].Pkt.Code))
			}
			if frames[0].Pkt.Identifier != reqID {
				t.Fatalf("Terminate-Ack Identifier = 0x%02x, want 0x%02x", frames[0].Pkt.Identifier, reqID)
			}
		})
	}
}

// TestRFC1661CodeRejectAcrossAllStates drives handleLCPPacket with an unknown
// LCP Code in every packet-receiving LCP state defined by RFC 1661 Section 4.1
// (Closed, Stopped, Closing, Stopping, Req-Sent, Ack-Rcvd, Ack-Sent and
// Opened).
//
// VALIDATES: RFC 1661 Section 5.6 ("This MUST be reported back to the sender
// of the unknown Code by transmitting a Code-Reject.") holds in every state of
// the automaton, Closing and Stopping included, preserving the state and
// echoing the rejected packet in Rejected-Packet.
// PREVENTS: a missing scj action in Closing, Stopping, or any other non-Opened
// state's LCPEventRUC cell of LCPDoTransition (ppp_fsm.go) going undetected by
// Opened-only tests.
//
// RFC requirement: RFC1661-5.6-1 positive -- handleLCPPacket (session_run.go)
// transmits exactly one LCP Code-Reject carrying the offending packet's Code,
// Identifier, Length and Data, and preserves the RFC 1661 Section 4.1 state
// for an unknown Code received in Closed, Stopped, Closing, Stopping, Req-Sent,
// Ack-Rcvd, Ack-Sent and Opened.
//
// MUTATION: drop LCPActSCJ from any state's LCPEventRUC case in
// LCPDoTransition (ppp_fsm.go), or break sendCodeReject (session_run.go):
// that state's subtest observes zero Code-Reject frames.
func TestRFC1661CodeRejectAcrossAllStates(t *testing.T) {
	cases := []struct {
		state   LCPState
		wantEnd bool
	}{
		{LCPStateClosed, true},
		{LCPStateStopped, true},
		{LCPStateClosing, false},
		{LCPStateStopping, false},
		{LCPStateReqSent, false},
		{LCPStateAckRcvd, false},
		{LCPStateAckSent, false},
		{LCPStateOpened, false},
	}

	const reqID uint8 = 0x41
	payload := []byte{0xaa, 0xbb}
	wantRejected := []byte{unknownLCPCode, reqID, 0, 6, 0xaa, 0xbb}
	for _, tc := range cases {
		t.Run(tc.state.String(), func(t *testing.T) {
			s, rec, _ := newRFC1661Session(tc.state)
			ended := s.handleLCPPacket(LCPPacket{Code: unknownLCPCode, Identifier: reqID, Data: payload})
			if ended != tc.wantEnd {
				t.Fatalf("handleLCPPacket ended = %v, want %v", ended, tc.wantEnd)
			}
			if got := s.currentState(); got != tc.state {
				t.Fatalf("state = %s, want unchanged %s", got, tc.state)
			}
			frames := decodeFrames(t, rec)
			if len(frames) != 1 {
				t.Fatalf("recorded %d frames, want exactly 1 Code-Reject", len(frames))
			}
			if frames[0].Proto != ProtoLCP {
				t.Fatalf("reply proto = %#04x, want ProtoLCP (%#04x)", frames[0].Proto, ProtoLCP)
			}
			if frames[0].Pkt.Code != LCPCodeReject {
				t.Fatalf("reply code = %d (%s), want Code-Reject", frames[0].Pkt.Code, LCPCodeName(frames[0].Pkt.Code))
			}
			if !bytes.Equal(frames[0].Pkt.Data, wantRejected) {
				t.Fatalf("Rejected-Packet = %x, want %x", frames[0].Pkt.Data, wantRejected)
			}
		})
	}
}
