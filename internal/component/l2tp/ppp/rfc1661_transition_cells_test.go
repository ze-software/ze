// Design: docs/research/l2tpv2-implementation-guide.md -- shared PPP automaton.
package ppp

import "testing"

// transitionControlData supplies a complete reject payload rather than relying
// on the current consumer's lack of payload validation.
func transitionControlData(code uint8) []byte {
	switch code {
	case LCPCodeReject:
		return []byte{255, 3, 0, 4}
	case LCPProtocolReject:
		return []byte{0x12, 0x35}
	default:
		return nil
	}
}

// TestRFC1661CorrectedTransitionCells checks the six RFC 1661 Section 4.1
// cells against their state/action pairs, including RXJ- as an abstract event.
// Section 4.1: "State transitions and actions are represented in the form
// action/new-state." The RXJ- row is tlf/2, tlf/2, tlf/3 in Closed, Closing,
// Stopping; RTA and RXJ+ in Ack-Rcvd are 6; RXJ+ in Opened is 9 without ser.
func TestRFC1661CorrectedTransitionCells(t *testing.T) {
	runFSMCases(t, []fsmCase{
		{"closed catastrophic reject", LCPStateClosed, LCPEventRXJMinus, LCPStateClosed, []LCPAction{LCPActTLF}},
		{"closing catastrophic reject", LCPStateClosing, LCPEventRXJMinus, LCPStateClosed, []LCPAction{LCPActTLF}},
		{"stopping catastrophic reject", LCPStateStopping, LCPEventRXJMinus, LCPStateStopped, []LCPAction{LCPActTLF}},
		{"ack received terminate ack", LCPStateAckRcvd, LCPEventRTA, LCPStateReqSent, nil},
		{"ack received permitted reject", LCPStateAckRcvd, LCPEventRXJPlus, LCPStateReqSent, nil},
		{"opened permitted reject", LCPStateOpened, LCPEventRXJPlus, LCPStateOpened, nil},
	})
}

// TestRFC1661AckRcvdControlPackets drives actual LCP frame and NCP packet
// consumers. Reject codes currently classify as RXJ+, never RXJ-; this test
// makes no claim that a catastrophic rejection is reachable from wire input.
func TestRFC1661AckRcvdControlPackets(t *testing.T) {
	for _, code := range []uint8{LCPTerminateAck, LCPCodeReject, LCPProtocolReject} {
		t.Run(LCPCodeName(code), func(t *testing.T) {
			s, rec, events := newRFC1661Session(LCPStateAckRcvd)
			if s.handleFrame(lcpFrame(ProtoLCP, code, 17, transitionControlData(code))) {
				t.Fatal("LCP consumer ended negotiation")
			}
			if got := s.currentState(); got != LCPStateReqSent {
				t.Errorf("LCP state = %v, want ReqSent", got)
			}
			if rec.count() != 0 || len(events) != 0 {
				t.Fatal("action-free transition emitted a frame or lifecycle event")
			}
			for _, family := range []AddressFamily{AddressFamilyIPv4, AddressFamilyIPv6} {
				s.setNCPState(family, LCPStateAckRcvd)
				pkt := LCPPacket{Code: code, Identifier: 17, Data: transitionControlData(code)}
				var ended bool
				if family == AddressFamilyIPv4 {
					ended = s.handleIPCPPacket(pkt)
				} else {
					ended = s.handleIPv6CPPacket(pkt)
				}
				if ended || s.ncpState(family) != LCPStateReqSent {
					t.Errorf("%v ended=%v state=%v, want ongoing ReqSent", family, ended, s.ncpState(family))
				}
			}
			if rec.count() != 0 || len(events) != 0 {
				t.Fatal("NCP transition emitted a frame or lifecycle event")
			}
		})
	}
}

// TestRFC1661CatastrophicRejectConsumer supplies abstract RXJ- directly to
// the real LCP transition consumer and observes its lower-layer notification.
func TestRFC1661CatastrophicRejectConsumer(t *testing.T) {
	for _, state := range []LCPState{LCPStateClosed, LCPStateClosing, LCPStateStopping} {
		t.Run(state.String(), func(t *testing.T) {
			s, rec, events := newRFC1661Session(state)
			tr := LCPDoTransition(state, LCPEventRXJMinus)
			s.applyTransition(state, tr, LCPPacket{})
			want := LCPStateClosed
			if state == LCPStateStopping {
				want = LCPStateStopped
			}
			if got := s.currentState(); got != want {
				t.Errorf("state = %v, want %v", got, want)
			}
			if rec.count() != 0 {
				t.Fatal("catastrophic rejection sent an unexpected frame")
			}
			select {
			case event := <-events:
				if _, ok := event.(EventSessionDown); !ok {
					t.Fatalf("event = %T, want EventSessionDown", event)
				}
			default:
				t.Fatal("missing lower-layer finished notification")
			}
		})
	}
}

// TestRFC1661OpenedPermittedRejectIsQuiet checks the actual frame consumers
// retain Opened with no wire or lifecycle output. The table test separately
// proves the absence of SER, whose consumers also suppress reject replies.
func TestRFC1661OpenedPermittedRejectIsQuiet(t *testing.T) {
	for _, proto := range []uint16{ProtoLCP, ProtoIPCP, ProtoIPv6CP} {
		for _, code := range []uint8{LCPCodeReject, LCPProtocolReject} {
			s, rec, events := newRFC1661Session(LCPStateOpened)
			s.disableIPCP = false
			s.disableIPv6CP = false
			s.ncpStarted = true
			s.setNCPState(AddressFamilyIPv4, LCPStateOpened)
			s.setNCPState(AddressFamilyIPv6, LCPStateOpened)
			if s.handleFrame(lcpFrame(proto, code, 17, transitionControlData(code))) {
				t.Fatalf("protocol %x code %d ended session", proto, code)
			}
			if s.currentState() != LCPStateOpened {
				t.Fatal("permitted reject changed LCP Opened")
			}
			for _, family := range []AddressFamily{AddressFamilyIPv4, AddressFamilyIPv6} {
				if s.ncpState(family) != LCPStateOpened {
					t.Fatalf("permitted reject changed %v Opened", family)
				}
			}
			if rec.count() != 0 || len(events) != 0 {
				t.Fatalf("protocol %x code %d emitted frame or event", proto, code)
			}
		}
	}
}
