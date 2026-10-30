// Design: docs/research/l2tpv2-implementation-guide.md -- shared PPP automaton.
package ppp

// RFC naming: untagged -- these cells check the RFC 1661 Section 4.1 transition table, which states no MUST-level obligation, so rfc/short/rfc1661.md holds no requirement row for them to tag.

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
//
// Protocol-Reject is an LCP Code only. RFC 1332 Section 2 and RFC 5072
// Section 3: "Only Codes 1 through 7 (...) are used.  Other Codes should be
// treated as unrecognized and should result in Code-Rejects." So on IPCP and
// IPv6CP it is RUC, whose Ack-Rcvd edge is scj/7: one Code-Reject on the
// NCP's own protocol, and the NCP stays in Ack-Rcvd.
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
				if code == LCPProtocolReject {
					if ended || s.ncpState(family) != LCPStateAckRcvd {
						t.Errorf("%v ended=%v state=%v, want ongoing AckRcvd", family, ended, s.ncpState(family))
					}
					continue
				}
				if ended || s.ncpState(family) != LCPStateReqSent {
					t.Errorf("%v ended=%v state=%v, want ongoing ReqSent", family, ended, s.ncpState(family))
				}
			}
			if len(events) != 0 {
				t.Fatal("NCP transition emitted a lifecycle event")
			}
			if code != LCPProtocolReject {
				if rec.count() != 0 {
					t.Fatal("NCP transition emitted a frame")
				}
				return
			}
			frames := decodeFrames(t, rec)
			if len(frames) != 2 {
				t.Fatalf("NCP Protocol-Reject drew %d frames, want one Code-Reject per NCP", len(frames))
			}
			for i, proto := range []uint16{ProtoIPCP, ProtoIPv6CP} {
				if frames[i].Proto != proto || frames[i].Pkt.Code != LCPCodeReject || frames[i].Pkt.Data[0] != LCPProtocolReject {
					t.Fatalf("frame %d = %+v, want a Code-Reject of the Protocol-Reject on %#04x", i, frames[i], proto)
				}
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
//
// A Protocol-Reject on IPCP or IPv6CP is not a permitted reject but a Code
// those NCPs do not use (RFC 1332 Section 2, RFC 5072 Section 3), so it
// draws exactly one Code-Reject on that NCP's protocol and changes no state.
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
			if len(events) != 0 {
				t.Fatalf("protocol %x code %d emitted a lifecycle event", proto, code)
			}
			if proto == ProtoLCP || code != LCPProtocolReject {
				if rec.count() != 0 {
					t.Fatalf("protocol %x code %d emitted a frame", proto, code)
				}
				continue
			}
			frames := decodeFrames(t, rec)
			if len(frames) != 1 || frames[0].Proto != proto || frames[0].Pkt.Code != LCPCodeReject || frames[0].Pkt.Data[0] != LCPProtocolReject {
				t.Fatalf("protocol %x code %d drew %+v, want one Code-Reject of it on the same protocol", proto, code, frames)
			}
		}
	}
}
