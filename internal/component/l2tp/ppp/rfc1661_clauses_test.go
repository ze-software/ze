// Design: docs/architecture/l2tp/bng-5-pppoe.md -- RFC 1661 conformance coverage
// Related: rfc1661_test.go -- the first tagged units of the same rows
//
// Drives the clauses of RFC 1661 Sections 2, 4.3, 5.1, 5.3 and 5.4 that the
// first tagged units of each row left unasserted: a Protocol whose most
// significant octet is odd, a well-formed boolean option ze declines, an
// acceptable option beside Nak-earning ones, an option refused by
// configuration rather than unrecognized, a Configure-Request received in
// Opened, and the Configure-Request an Open puts on the wire.

package ppp

import (
	"bytes"
	"testing"
	"time"
)

// lcpReplyOptionTypes returns the option Types of the last reply of code the
// session wrote, or fails when it wrote none.
func lcpReplyOptionTypes(t *testing.T, rec *frameRecorder, code uint8) []uint8 {
	t.Helper()
	pkt, ok := findCode(t, rec, code)
	if !ok {
		t.Fatalf("no %s written; frames=%d", LCPCodeName(code), rec.count())
	}
	opts, err := ParseLCPOptions(pkt.Data)
	if err != nil {
		t.Fatalf("%s options do not parse: %v", LCPCodeName(code), err)
	}
	types := make([]uint8, 0, len(opts))
	for _, opt := range opts {
		types = append(types, opt.Type)
	}
	return types
}

// TestRFC1661ProtocolWithOddHighOctetTreatedUnrecognized sends the LCP body
// under 0xC121: the least significant octet is odd, but the most significant
// octet's least significant bit is 1, the second rule of Section 2. It also
// reads every Protocol value ze writes into a frame.
//
// VALIDATES: RFC 1661 Section 2, both parity rules on receive and on send.
// PREVENTS: a dispatcher that masks the high octet and reads 0xC121 as LCP.
//
// RFC requirement: RFC1661-2-1 positive -- every Protocol value ze transmits (LCP, PAP, CHAP, IPCP, IPV6CP, IPv4, IPv6) has the least significant bit of its least significant octet set and that of its most significant octet clear.
// RFC requirement: RFC1661-2-1 negative -- an Opened session that receives an Echo-Request under Protocol 0xC121 answers with a Protocol-Reject naming 0xC121, writes no Echo-Reply and stays in Opened.
func TestRFC1661ProtocolWithOddHighOctetTreatedUnrecognized(t *testing.T) {
	t.Parallel()

	for _, proto := range []uint16{ProtoLCP, ProtoPAP, ProtoCHAP, ProtoIPCP, ProtoIPv6CP, ProtoIPv4, ProtoIPv6} {
		if proto&0x0001 == 0 {
			t.Fatalf("Protocol %#04x is even", proto)
		}
		if proto&0x0100 != 0 {
			t.Fatalf("Protocol %#04x has the low bit of its high octet set", proto)
		}
	}

	s, rec, _ := newRFC1661Session(LCPStateOpened)
	s.peerMagic = 0x11223344
	body := []byte{0x11, 0x22, 0x33, 0x44}
	if term := s.handleFrame(lcpFrame(0xC121, LCPEchoRequest, 0x32, body)); term {
		t.Fatal("handleFrame ended the session on an unrecognized Protocol")
	}
	frames := decodeFrames(t, rec)
	if len(frames) != 1 {
		t.Fatalf("wrote %d frames for Protocol 0xC121, want one Protocol-Reject", len(frames))
	}
	if frames[0].Pkt.Code != LCPProtocolReject {
		t.Fatalf("answered %s, want Protocol-Reject", LCPCodeName(frames[0].Pkt.Code))
	}
	if rp := rejectedProtocol(t, frames[0].Pkt); rp != 0xC121 {
		t.Fatalf("Rejected-Protocol = %#04x, want 0xc121", rp)
	}
	if got := s.currentState(); got != LCPStateOpened {
		t.Fatalf("state = %s, want opened", got)
	}
}

// TestRFC1661WellFormedBooleanDeclinedWithReject offers a PPPoE session a
// well-formed ACFC (Length 2, no value field), which RFC 2516 makes ze
// decline, first beside an acceptable MRU and then beside an MRU that earns a
// Nak. The option carries no malformation, so only its being a boolean
// decides the Reject.
//
// VALIDATES: RFC 1661 Section 5.3, a declined boolean option is Rejected.
// PREVENTS: a boolean option ze declines being answered with a Configure-Nak.
//
// RFC requirement: RFC1661-5.3-2 positive -- a Configure-Request carrying a well-formed ACFC that the PPPoE session declines draws a Configure-Reject naming exactly that ACFC option, and no Configure-Nak.
// RFC requirement: RFC1661-5.3-2 negative -- when the same request also carries an MRU that earns a Nak, NegotiatePeerOptions puts the ACFC in the reject list and never in the Nak list, and the wire reply is the Configure-Reject.
func TestRFC1661WellFormedBooleanDeclinedWithReject(t *testing.T) {
	t.Parallel()

	acfc := LCPOption{Type: LCPOptACFC}
	s, rec, _ := newRFC1661Session(LCPStateReqSent)
	s.pppoe = true
	s.handleLCPPacket(LCPPacket{Code: LCPConfigureRequest, Identifier: 0x40, Data: optStream(mruOption(1400), acfc)})
	if _, ok := findCode(t, rec, LCPConfigureNak); ok {
		t.Fatal("a declined boolean option drew a Configure-Nak")
	}
	reject, ok := findCode(t, rec, LCPConfigureReject)
	if !ok {
		t.Fatal("a declined boolean option drew no Configure-Reject")
	}
	if want := optStream(acfc); !bytes.Equal(reject.Data, want) {
		t.Fatalf("Configure-Reject options = %x, want exactly %x", reject.Data, want)
	}

	_, naks, rejects := NegotiatePeerOptions([]LCPOption{mruOption(2000), acfc}, s.negPolicy())
	if len(naks) != 1 || naks[0].Type != LCPOptMRU {
		t.Fatalf("naks = %+v, want the MRU alone", naks)
	}
	if len(rejects) != 1 || rejects[0].Type != LCPOptACFC {
		t.Fatalf("rejects = %+v, want the ACFC alone", rejects)
	}
	s, rec, _ = newRFC1661Session(LCPStateReqSent)
	s.pppoe = true
	s.handleLCPPacket(LCPPacket{Code: LCPConfigureRequest, Identifier: 0x41, Data: optStream(mruOption(2000), acfc)})
	if _, ok := findCode(t, rec, LCPConfigureNak); ok {
		t.Fatal("a request carrying a declined boolean option drew a Configure-Nak")
	}
	if types := lcpReplyOptionTypes(t, rec, LCPConfigureReject); len(types) != 1 || types[0] != LCPOptACFC {
		t.Fatalf("Configure-Reject option Types = %v, want [ACFC]", types)
	}
}

// TestRFC1661NakFiltersAcceptableOptions sends a Configure-Request whose
// acceptable ACCM sits between two options that earn a Nak, in both orders of
// the two, and reads the Configure-Nak on the wire.
//
// VALIDATES: RFC 1661 Section 5.3, the Nak carries only the unacceptable
// options, in request order.
// PREVENTS: an acceptable option copied into the Configure-Nak.
//
// RFC requirement: RFC1661-5.3-6 positive -- the Configure-Nak handleLCPPacket writes carries the Magic-Number and MRU options in the order of the request, for both orders.
// RFC requirement: RFC1661-5.3-6 negative -- the acceptable ACCM option that sat between them in the request is absent from the Configure-Nak.
func TestRFC1661NakFiltersAcceptableOptions(t *testing.T) {
	t.Parallel()

	accm := LCPOption{Type: LCPOptACCM, Data: []byte{0, 0, 0, 0}}
	for _, order := range [][]LCPOption{
		{magicOption(0), accm, mruOption(2000)},
		{mruOption(2000), accm, magicOption(0)},
	} {
		s, rec, _ := newRFC1661Session(LCPStateReqSent)
		s.handleLCPPacket(LCPPacket{Code: LCPConfigureRequest, Identifier: 0x42, Data: optStream(order...)})
		types := lcpReplyOptionTypes(t, rec, LCPConfigureNak)
		if len(types) != 2 || types[0] != order[0].Type || types[1] != order[2].Type {
			t.Fatalf("Configure-Nak option Types = %v, want [%d %d] without the ACCM",
				types, order[0].Type, order[2].Type)
		}
	}
}

// TestRFC1661ConfigureRejectForOptionRefusedByConfiguration offers options ze
// recognizes but its configuration declines: an Authentication-Protocol
// request (ze is the authenticator and accepts no peer demand) on any session,
// and an ACCM on a PPPoE session. The same ACCM on an L2TP session, where the
// configuration accepts it, is the control.
//
// VALIDATES: RFC 1661 Section 5.4, "not acceptable for negotiation (as
// configured by a network administrator)" draws a Configure-Reject.
// PREVENTS: a recognized option that configuration declines being Acked or
// Nak'd.
//
// RFC requirement: RFC1661-5.4-1 positive -- a Configure-Request carrying an Authentication-Protocol option, and on a PPPoE session one carrying an ACCM option, each draws a Configure-Reject naming exactly that option and no Configure-Ack.
// RFC requirement: RFC1661-5.4-1 negative -- the same ACCM option on an L2TP session, whose configuration accepts it, draws a Configure-Ack and no Configure-Reject.
func TestRFC1661ConfigureRejectForOptionRefusedByConfiguration(t *testing.T) {
	t.Parallel()

	auth := LCPOption{Type: LCPOptAuthProto, Data: []byte{0xC0, 0x23}}
	accm := LCPOption{Type: LCPOptACCM, Data: []byte{0, 0, 0, 0}}
	for _, tc := range []struct {
		name   string
		pppoe  bool
		option LCPOption
	}{
		{"auth-protocol", false, auth},
		{"pppoe-accm", true, accm},
	} {
		s, rec, _ := newRFC1661Session(LCPStateReqSent)
		s.pppoe = tc.pppoe
		s.handleLCPPacket(LCPPacket{Code: LCPConfigureRequest, Identifier: 0x43, Data: optStream(mruOption(1400), tc.option)})
		if _, ok := findCode(t, rec, LCPConfigureAck); ok {
			t.Fatalf("%s: an option the configuration declines was Acked", tc.name)
		}
		reject, ok := findCode(t, rec, LCPConfigureReject)
		if !ok {
			t.Fatalf("%s: no Configure-Reject written", tc.name)
		}
		if want := optStream(tc.option); !bytes.Equal(reject.Data, want) {
			t.Fatalf("%s: Configure-Reject options = %x, want exactly %x", tc.name, reject.Data, want)
		}
	}

	s, rec, _ := newRFC1661Session(LCPStateReqSent)
	s.handleLCPPacket(LCPPacket{Code: LCPConfigureRequest, Identifier: 0x44, Data: optStream(mruOption(1400), accm)})
	if _, ok := findCode(t, rec, LCPConfigureReject); ok {
		t.Fatal("an ACCM the L2TP configuration accepts drew a Configure-Reject")
	}
	if _, ok := findCode(t, rec, LCPConfigureAck); !ok {
		t.Fatal("an ACCM the L2TP configuration accepts drew no Configure-Ack")
	}
}

// TestRFC1661ConfigureRequestInOpenedRenegotiates hands an Opened session an
// acceptable Configure-Request and reads what handleLCPPacket writes and
// emits.
//
// VALIDATES: RFC 1661 Section 4.3, a session in Opened renegotiates at once.
// PREVENTS: the tld action ending the session instead of renegotiating.
//
// RFC requirement: RFC1661-4.3-1 positive -- an acceptable Configure-Request received in Opened makes handleLCPPacket write ze's own Configure-Request and a Configure-Ack, move to Ack-Sent, and neither end the session nor emit EventSessionDown.
func TestRFC1661ConfigureRequestInOpenedRenegotiates(t *testing.T) {
	t.Parallel()

	s, rec, events := newRFC1661Session(LCPStateOpened)
	if term := s.handleLCPPacket(LCPPacket{Code: LCPConfigureRequest, Identifier: 0x45, Data: optStream(mruOption(1400))}); term {
		t.Fatal("session ended on a Configure-Request in Opened")
	}
	if _, ok := findCode(t, rec, LCPConfigureRequest); !ok {
		t.Fatal("no Configure-Request written: ze did not renegotiate")
	}
	if _, ok := findCode(t, rec, LCPConfigureAck); !ok {
		t.Fatal("the peer's Configure-Request was not Acked")
	}
	if got := s.currentState(); got != LCPStateAckSent {
		t.Fatalf("state = %s, want ack-sent", got)
	}
	for len(events) > 0 {
		if ev, ok := (<-events).(EventSessionDown); ok {
			t.Fatalf("renegotiation emitted EventSessionDown: %+v", ev)
		}
	}
}

// TestRFC1661TerminateAckHoldsLinkForRestartTime hands an Opened session a
// Terminate-Request, then a Configure-Request and an Echo-Request while it
// waits, and reads the Restart timer. The session leaves Stopping only on a
// Timeout, which the Restart timer raises no earlier than one Restart time
// after zrc armed it beside the Terminate-Ack.
//
// VALIDATES: RFC 1661 Section 3.7, no disconnect until at least one Restart
// time has passed after the Terminate-Ack.
// PREVENTS: a disconnect on the next received packet, or a Terminate-Ack sent
// with no Restart timer running to hold the link.
//
// RFC requirement: RFC1661-3.7-1 positive -- after the Terminate-Ack, the Restart timer is running with defaultRestartTimer (at least the three seconds Section 4.6 defaults to), and a Configure-Request and an Echo-Request received meanwhile leave the session in Stopping with no EventSessionDown and no end.
func TestRFC1661TerminateAckHoldsLinkForRestartTime(t *testing.T) {
	t.Parallel()

	if defaultRestartTimer < 3*time.Second {
		t.Fatalf("defaultRestartTimer = %s, below the 3 s Restart time", defaultRestartTimer)
	}
	s, rec, events := newRFC1661Session(LCPStateOpened)
	s.restartTimer = time.NewTimer(time.Hour)
	s.restartTimer.Stop()
	s.peerMagic = 0x11223344

	if term := s.handleLCPPacket(LCPPacket{Code: LCPTerminateRequest, Identifier: 0x78}); term {
		t.Fatal("session ended on a Terminate-Request")
	}
	if _, ok := findCode(t, rec, LCPTerminateAck); !ok {
		t.Fatal("no Terminate-Ack written")
	}
	for _, pkt := range []LCPPacket{
		{Code: LCPConfigureRequest, Identifier: 0x79, Data: optStream(mruOption(1400))},
		{Code: LCPEchoRequest, Identifier: 0x7A, Data: []byte{0x11, 0x22, 0x33, 0x44}},
	} {
		if term := s.handleLCPPacket(pkt); term {
			t.Fatalf("session ended on a %s received after the Terminate-Ack", LCPCodeName(pkt.Code))
		}
		if got := s.currentState(); got != LCPStateStopping {
			t.Fatalf("state = %s after a %s, want stopping", got, LCPCodeName(pkt.Code))
		}
	}
	for len(events) > 0 {
		if ev, ok := (<-events).(EventSessionDown); ok {
			t.Fatalf("session went down before one Restart time passed: %+v", ev)
		}
	}
	if !s.restartTimer.Stop() {
		t.Fatal("no Restart timer runs after the Terminate-Ack, so nothing holds the link for one Restart time")
	}
}

// TestRFC1661OpenTransmitsConfigureRequestOnWire runs the transitions an
// administrative Open drives through applyTransition and reads the wire. The
// lower layer coming Up with no Open is the control: that session does not
// wish to open a connection yet.
//
// VALIDATES: RFC 1661 Section 5.1, opening a connection transmits a
// Configure-Request.
// PREVENTS: an scr action that never reaches the wire.
//
// RFC requirement: RFC1661-5.1-1 positive -- applyTransition for Open in Closed, and for Up in Starting (Open already given), each writes one LCP Configure-Request and moves to Req-Sent.
// RFC requirement: RFC1661-5.1-1 negative -- applyTransition for Up in Initial, where no Open was given, writes no frame and moves to Closed.
func TestRFC1661OpenTransmitsConfigureRequestOnWire(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		from LCPState
		ev   lCPEvent
	}{
		{LCPStateClosed, LCPEventOpen},
		{LCPStateStarting, LCPEventUp},
	} {
		s, rec, _ := newRFC1661Session(tc.from)
		s.applyTransition(tc.from, LCPDoTransition(tc.from, tc.ev), LCPPacket{})
		frames := decodeFrames(t, rec)
		if len(frames) != 1 || frames[0].Proto != ProtoLCP || frames[0].Pkt.Code != LCPConfigureRequest {
			t.Fatalf("%s: wrote %d frames, want one LCP Configure-Request", tc.from, len(frames))
		}
		if got := s.currentState(); got != LCPStateReqSent {
			t.Fatalf("%s: state = %s, want req-sent", tc.from, got)
		}
	}

	s, rec, _ := newRFC1661Session(LCPStateInitial)
	s.applyTransition(LCPStateInitial, LCPDoTransition(LCPStateInitial, LCPEventUp), LCPPacket{})
	if n := rec.count(); n != 0 {
		t.Fatalf("Up without Open wrote %d frames, want none", n)
	}
	if got := s.currentState(); got != LCPStateClosed {
		t.Fatalf("state = %s, want closed", got)
	}
}
