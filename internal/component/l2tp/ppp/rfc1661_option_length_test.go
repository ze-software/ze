// VALIDATES: RFC 1661 Section 6 option Length and Data handling on the LCP and
// NCP receive paths -- an option whose declared extent leaves the packet makes
// ze drop the whole packet without a reply and without a state change, while an
// option whose Length is merely invalid still draws a Configure-Nak carrying
// ze's desired options.
// PREVENTS: answering a malformed Configure-Request, which both violates
// Section 6 and turns ze into a reflector for a packet an off-path sender can
// forge; and the opposite error of falling silent on a packet that is only
// unacceptable, which would stall the negotiation.
//
// RFC: rfc/short/rfc1661.md
package ppp

import (
	"bytes"
	"encoding/binary"
	"testing"
	"time"
)

// ncpSilenceWindow is how long a test waits to conclude that ze answered
// nothing. It has to outlast the session goroutine's read-dispatch-write turn
// and stay well inside the 2s deadlines the other NCP helpers use.
const ncpSilenceWindow = 400 * time.Millisecond

// ncpRepliesWithin reads frames from the peer end for the given window and
// returns every packet ze sent for wantProto EXCEPT its own Configure-Request.
// Ze's Configure-Request is its own unacknowledged offer repeating, not a reply
// to what the test just sent, so counting it would report noise as a violation.
func ncpRepliesWithin(t *testing.T, td *ncpTestDriver, wantProto uint16, window time.Duration) []LCPPacket {
	t.Helper()
	deadline := time.Now().Add(window)
	var out []LCPPacket
	buf := make([]byte, MaxFrameLen)
	for {
		remaining := time.Until(deadline)
		if remaining <= 0 {
			return out
		}
		_ = td.peer.SetReadDeadline(time.Now().Add(remaining)) //nolint:errcheck // test helper
		n, err := td.peer.Read(buf)
		if err != nil {
			return out
		}
		proto, payload, _, ferr := ParseFrame(buf[:n])
		if ferr != nil {
			t.Fatalf("peer ParseFrame: %v", ferr)
		}
		if proto != wantProto {
			continue
		}
		pkt, lerr := ParseLCPPacket(payload)
		if lerr != nil {
			t.Fatalf("peer ParseLCPPacket: %v", lerr)
		}
		if pkt.Code == LCPConfigureRequest {
			continue
		}
		out = append(out, pkt)
	}
}

// settleNCPRequest reads ze's opening Configure-Request for proto and Acks it,
// so the family's FSM stops offering and the test's own Configure-Requests are
// the only thing left to answer.
func settleNCPRequest(t *testing.T, td *ncpTestDriver, proto uint16) {
	t.Helper()
	cr := td.readPeerNCPPacket(t, proto)
	if cr.Code != LCPConfigureRequest {
		t.Fatalf("first packet code = %d, want Configure-Request", cr.Code)
	}
	td.writePeerNCPPacket(t, proto, LCPConfigureAck, cr.Identifier, cr.Data)
}

// RFC requirement: RFC1661-6-2 positive -- an option whose Data is indicated by
// its Length to extend beyond the end of the Information field makes ze discard
// the entire packet: no Configure-Ack, Nak or Reject is emitted, and the
// automaton is untouched, which the following valid Configure-Request proves by
// still drawing an Ack (producer handleNCPPacket's ncpRequestDiscard arm, fed by
// scanNCPOptions, internal/component/l2tp/ppp/ncp.go).
// RFC requirement: RFC1661-6-2 negative -- the silence is confined to packets
// that are invalid. A Configure-Request whose options all fit inside the packet
// but carry an unacceptable value still draws a Configure-Nak, so ze is not
// simply refusing to answer.
func TestRFC1661TruncatedOptionSilentlyDiscarded(t *testing.T) {
	t.Run("IPCP truncated option draws no reply", func(t *testing.T) {
		td := newNCPTestDriverCfg(t, &StartSession{DisableIPv6CP: true})
		defer td.cleanup()
		settleNCPRequest(t, td, ProtoIPCP)

		// IP-Address declares Length 6 but only 4 octets of the option are
		// present, so its Data runs two octets past the packet.
		truncated := []byte{IPCPOptIPAddress, 6, 10, 0}
		td.writePeerNCPPacket(t, ProtoIPCP, LCPConfigureRequest, 0x41, truncated)

		if replies := ncpRepliesWithin(t, td, ProtoIPCP, ncpSilenceWindow); len(replies) != 0 {
			t.Fatalf("ze answered a truncated Configure-Request with %d packet(s), first code %d; RFC 1661 Section 6 discards it", len(replies), replies[0].Code)
		}

		// The automaton must be where it was: a valid, acceptable request is
		// still Acked.
		acceptable := []byte{
			IPCPOptIPAddress, 6, 10, 0, 0, 2,
			IPCPOptPrimaryDNS, 6, 1, 1, 1, 1,
			IPCPOptSecondaryDNS, 6, 8, 8, 8, 8,
		}
		td.writePeerNCPPacket(t, ProtoIPCP, LCPConfigureRequest, 0x42, acceptable)
		ack := readIPCPUntil(t, td, LCPConfigureAck)
		if ack.Identifier != 0x42 {
			t.Errorf("Configure-Ack Identifier = 0x%02x, want 0x42", ack.Identifier)
		}
	})

	t.Run("IPv6CP truncated option draws no reply", func(t *testing.T) {
		td := newNCPTestDriverCfg(t, &StartSession{DisableIPCP: true})
		defer td.cleanup()
		settleNCPRequest(t, td, ProtoIPv6CP)

		// Interface-Identifier declares Length 10 with only 5 octets present.
		truncated := []byte{IPv6CPOptInterfaceID, ipv6cpInterfaceIDOptLen, 0x02, 0x00, 0x5E}
		td.writePeerNCPPacket(t, ProtoIPv6CP, LCPConfigureRequest, 0x51, truncated)

		if replies := ncpRepliesWithin(t, td, ProtoIPv6CP, ncpSilenceWindow); len(replies) != 0 {
			t.Fatalf("ze answered a truncated IPv6CP Configure-Request with %d packet(s), first code %d; RFC 1661 Section 6 discards it", len(replies), replies[0].Code)
		}

		td.writePeerNCPPacket(t, ProtoIPv6CP, LCPConfigureRequest, 0x52, ipv6cpInterfaceIDOption(ipv6cpTestPeerID))
		ack := readIPv6CPUntil(t, td, LCPConfigureAck)
		if ack.Identifier != 0x52 {
			t.Errorf("Configure-Ack Identifier = 0x%02x, want 0x52", ack.Identifier)
		}
	})

	t.Run("a contained but unacceptable option still draws a Nak", func(t *testing.T) {
		td := newNCPTestDriverCfg(t, &StartSession{DisableIPv6CP: true})
		defer td.cleanup()
		settleNCPRequest(t, td, ProtoIPCP)

		// Well-formed: Length 6 with all six octets present. The address is not
		// the one ze assigned, so this is unacceptable rather than invalid.
		unacceptable := []byte{IPCPOptIPAddress, 6, 9, 9, 9, 9}
		td.writePeerNCPPacket(t, ProtoIPCP, LCPConfigureRequest, 0x43, unacceptable)

		replies := ncpRepliesWithin(t, td, ProtoIPCP, 2*time.Second)
		if len(replies) == 0 {
			t.Fatal("ze answered nothing to a well-formed but unacceptable Configure-Request; the discard must be confined to invalid packets")
		}
		if replies[0].Code != LCPConfigureNak {
			t.Errorf("reply code = %d, want Configure-Nak", replies[0].Code)
		}
	})
}

// RFC requirement: RFC1661-6-1 positive -- a negotiable option received with an
// invalid Length draws a Configure-Nak carrying the desired option with an
// appropriate Length and Data: ze answers with IP-Address at Length 6 holding
// the address it assigned (producer buildNakOrReject, whose Reject arm is
// entered only for ncpOptionsUnknownType, internal/component/l2tp/ppp/ncp.go).
// RFC requirement: RFC1661-6-1 negative -- a Configure-Request whose options
// all carry a valid Length and acceptable values draws a Configure-Ack, so the
// Nak is confined to the invalid-Length case and is not ze's default answer.
func TestRFC1661InvalidOptionLengthDrawsNak(t *testing.T) {
	nakCases := []struct {
		name string
		data []byte
	}{
		// Length 4 on a 6-octet option: it fits inside the packet, so the
		// packet is valid and only the option's own Length is wrong.
		{"length short for the option type", []byte{IPCPOptIPAddress, 4, 10, 0}},
		// Length 1 counts fewer octets than the Type and Length fields occupy.
		{"length below the two-octet header", []byte{IPCPOptIPAddress, 1, 0, 0}},
	}
	for _, tc := range nakCases {
		t.Run(tc.name, func(t *testing.T) {
			td := newNCPTestDriverCfg(t, &StartSession{DisableIPv6CP: true})
			defer td.cleanup()
			settleNCPRequest(t, td, ProtoIPCP)

			td.writePeerNCPPacket(t, ProtoIPCP, LCPConfigureRequest, 0x61, tc.data)
			nak := readIPCPUntil(t, td, LCPConfigureNak)

			opts, err := ParseIPCPOptions(nak.Data)
			if err != nil {
				t.Fatalf("Configure-Nak Data % x does not parse: %v; RFC 1661 Section 6 wants the desired option at an appropriate Length", nak.Data, err)
			}
			if !opts.HasIPAddress {
				t.Fatalf("Configure-Nak Data % x carries no IP-Address option", nak.Data)
			}
			if opts.IPAddress != ipcpTestPeer {
				t.Errorf("Nak'd IP-Address = %v, want the assigned %v", opts.IPAddress, ipcpTestPeer)
			}
		})
	}

	t.Run("valid length and acceptable value draws an Ack", func(t *testing.T) {
		td := newNCPTestDriverCfg(t, &StartSession{DisableIPv6CP: true})
		defer td.cleanup()
		settleNCPRequest(t, td, ProtoIPCP)

		acceptable := []byte{IPCPOptIPAddress, 6, 10, 0, 0, 2}
		td.writePeerNCPPacket(t, ProtoIPCP, LCPConfigureRequest, 0x62, acceptable)

		replies := ncpRepliesWithin(t, td, ProtoIPCP, 2*time.Second)
		if len(replies) == 0 {
			t.Fatal("ze answered nothing to a well-formed acceptable Configure-Request")
		}
		if replies[0].Code != LCPConfigureAck {
			t.Errorf("reply code = %d, want Configure-Ack", replies[0].Code)
		}
	})
}

// lcpReqFrame wraps an option list in an LCP Configure-Request frame.
func lcpReqFrame(id uint8, data []byte) []byte {
	return lcpFrame(ProtoLCP, LCPConfigureRequest, id, data)
}

// onlyLCPNak reads the single frame ze wrote, asserts it is a Configure-Nak,
// and returns it. A Configure-Reject here is the failure the test is looking
// for, so it is reported as one rather than as "no Nak found".
func onlyLCPNak(t *testing.T, rec *frameRecorder) LCPPacket {
	t.Helper()
	if rej, ok := findCode(t, rec, LCPConfigureReject); ok {
		t.Fatalf("ze answered with a Configure-Reject carrying % x; RFC 1661 Section 6 wants a Configure-Nak with the desired option", rej.Data)
	}
	if cj, ok := findCode(t, rec, LCPCodeReject); ok {
		t.Fatalf("ze answered with a Code-Reject carrying % x; RFC 1661 Section 5.6 reserves that reply for an unknown Code", cj.Data)
	}
	nak, ok := findCode(t, rec, LCPConfigureNak)
	if !ok {
		t.Fatalf("no Configure-Nak emitted; frames=%d", rec.count())
	}
	return nak
}

// RFC requirement: RFC1661-6-2 positive -- an LCP option whose Data is
// indicated by its Length to extend beyond the end of the Information field
// makes ze discard the entire packet: it writes no frame at all, and the
// automaton stays in ReqSent, which the following valid Configure-Request
// proves by still drawing an Ack and moving to AckSent (producer
// handleLCPPacket's lcpRequestDiscard arm, fed by WalkLCPOptions,
// internal/component/l2tp/ppp/session_run.go).
// RFC requirement: RFC1661-6-2 negative -- the silence is confined to packets
// that are invalid. A Configure-Request whose options all fit inside the packet
// but carry a value ze refuses still draws a Configure-Nak. Ze is not refusing
// to answer.
func TestRFC1661LCPTruncatedOptionSilentlyDiscarded(t *testing.T) {
	discardCases := []struct {
		name string
		data []byte
	}{
		// MRU declares Length 4 with only two of those four octets present,
		// so its Data runs two octets past the packet.
		{"data runs past the end", []byte{LCPOptMRU, 4, 0x05}},
		// One octet left over cannot hold a Type and a Length, so the
		// option's own Length field is already past the end.
		{"header does not fit", []byte{LCPOptMRU}},
		// A well-formed option followed by a truncated one: the fault is not
		// at offset zero, and the packet still goes as a whole.
		{"second option runs past the end", []byte{LCPOptMagic, 6, 0xDE, 0xAD, 0xBE, 0xEF, LCPOptMRU, 4, 0x05}},
	}
	// Both states answer RCR- with scn, and RFC 1661 Section 4.1 gives them
	// different NEXT states for it: ReqSent stays where it is, AckSent falls
	// back to ReqSent. Running the table over both is what separates "ze sent
	// no reply" from "ze ran the automaton and the reply was suppressed
	// downstream" -- only the AckSent row moves when the discard is lost.
	for _, start := range []LCPState{LCPStateReqSent, LCPStateAckSent} {
		for _, tc := range discardCases {
			t.Run(start.String()+"/"+tc.name, func(t *testing.T) {
				s, rec, _ := newRFC1661Session(start)

				if term := s.handleFrame(lcpReqFrame(0x71, tc.data)); term {
					t.Fatal("session terminated on a Configure-Request RFC 1661 Section 6 discards")
				}
				if n := rec.count(); n != 0 {
					got := decodeFrames(t, rec)
					t.Fatalf("ze answered a truncated Configure-Request with %d frame(s), first code %d; RFC 1661 Section 6 discards it", n, got[0].Pkt.Code)
				}
				if got := s.currentState(); got != start {
					t.Fatalf("state = %s, want %s; RFC 1661 Section 6 discards without affecting the automaton", got, start)
				}

				// The automaton must still be live: a valid, acceptable
				// request is Acked, and both start states answer RCR+ with
				// sca and land in AckSent.
				if term := s.handleFrame(lcpReqFrame(0x72, optStream(mruOption(1460)))); term {
					t.Fatal("session terminated on an acceptable Configure-Request")
				}
				ack, ok := findCode(t, rec, LCPConfigureAck)
				if !ok {
					t.Fatalf("no Configure-Ack after the discard; frames=%d", rec.count())
				}
				if ack.Identifier != 0x72 {
					t.Errorf("Configure-Ack Identifier = 0x%02x, want 0x72", ack.Identifier)
				}
				if got := s.currentState(); got != LCPStateAckSent {
					t.Errorf("state = %s, want ack-sent", got)
				}
			})
		}
	}

	t.Run("a contained but unacceptable option still draws a Nak", func(t *testing.T) {
		s, rec, _ := newRFC1661Session(LCPStateReqSent)

		// Well-formed: Length 4 with all four octets present. 2000 exceeds
		// the 1500 ze accepts, so this is unacceptable rather than invalid.
		if term := s.handleFrame(lcpReqFrame(0x73, optStream(mruOption(2000)))); term {
			t.Fatal("session terminated on an unacceptable Configure-Request")
		}
		nak := onlyLCPNak(t, rec)
		if nak.Identifier != 0x73 {
			t.Errorf("Configure-Nak Identifier = 0x%02x, want 0x73", nak.Identifier)
		}
	})
}

// RFC requirement: RFC1661-6-1 positive -- a negotiable LCP option received
// with an invalid Length draws a Configure-Nak carrying the desired option with
// an appropriate Length and Data: ze answers with MRU at Length 4 holding the
// 1500 it accepts (producers LCPNakOrReject in
// internal/component/l2tp/ppp/session_run.go for a Length below the option
// header, and refusedOptionOutcome in internal/component/l2tp/ppp/lcp_options.go
// for a Length wrong for the option Type).
// RFC requirement: RFC1661-6-1 negative -- a Configure-Request whose options all
// carry a valid Length and acceptable values draws a Configure-Ack, so the Nak
// is confined to the invalid-Length case and is not ze's default answer.
func TestRFC1661LCPInvalidOptionLengthDrawsNak(t *testing.T) {
	nakCases := []struct {
		name string
		data []byte
	}{
		// Length 3 on a 4-octet option: it fits inside the packet, so the
		// packet is valid and only the option's own Length is wrong.
		{"length short for the option type", []byte{LCPOptMRU, 3, 0x05}},
		// Length 1 counts fewer octets than the Type and Length fields occupy.
		{"length below the two-octet header", []byte{LCPOptMRU, 1, 0x05, 0xDC}},
		// Length 0 would never advance a walk that trusted it.
		{"length zero", []byte{LCPOptMRU, 0, 0x05, 0xDC}},
	}
	for _, tc := range nakCases {
		t.Run(tc.name, func(t *testing.T) {
			s, rec, _ := newRFC1661Session(LCPStateReqSent)

			if term := s.handleFrame(lcpReqFrame(0x81, tc.data)); term {
				t.Fatal("session terminated on an invalid-Length Configure-Request")
			}
			nak := onlyLCPNak(t, rec)
			if nak.Identifier != 0x81 {
				t.Errorf("Configure-Nak Identifier = 0x%02x, want 0x81", nak.Identifier)
			}
			opts, err := ParseLCPOptions(nak.Data)
			if err != nil {
				t.Fatalf("Configure-Nak Data % x does not parse: %v; RFC 1661 Section 6 wants the desired option at an appropriate Length", nak.Data, err)
			}
			if len(opts) != 1 || opts[0].Type != LCPOptMRU {
				t.Fatalf("Configure-Nak Data % x carries no MRU option", nak.Data)
			}
			if len(opts[0].Data) != 2 {
				t.Fatalf("Nak'd MRU carries %d data octets, want the 2 RFC 1661 Section 6.1 gives it", len(opts[0].Data))
			}
			if got := binary.BigEndian.Uint16(opts[0].Data); got != MaxFrameLen {
				t.Errorf("Nak'd MRU = %d, want the %d ze accepts", got, MaxFrameLen)
			}
			// RFC 1661 Section 6 discards without affecting the automaton, and
			// this packet is NOT one it discards: RCR- keeps ReqSent and emits
			// scn, so the reply above proves the automaton ran.
			if got := s.currentState(); got != LCPStateReqSent {
				t.Errorf("state = %s, want req-sent", got)
			}
		})
	}

	t.Run("valid length and acceptable value draws an Ack", func(t *testing.T) {
		s, rec, _ := newRFC1661Session(LCPStateReqSent)

		if term := s.handleFrame(lcpReqFrame(0x82, optStream(mruOption(1460)))); term {
			t.Fatal("session terminated on an acceptable Configure-Request")
		}
		if _, ok := findCode(t, rec, LCPConfigureNak); ok {
			t.Fatal("Configure-Nak emitted for an option whose Length and value are both good")
		}
		ack, ok := findCode(t, rec, LCPConfigureAck)
		if !ok {
			t.Fatalf("no Configure-Ack emitted; frames=%d", rec.count())
		}
		if ack.Identifier != 0x82 {
			t.Errorf("Configure-Ack Identifier = 0x%02x, want 0x82", ack.Identifier)
		}
	})
}

// VALIDATES: the desired option a Configure-Nak carries is per-Type, not a
//
//	single hardcoded answer -- a wrong-length Magic-Number draws a
//	Magic-Number, where a wrong-length MRU draws an MRU.
//
// This also covers the wrong-Length half of RFC1661-6.4-1, which forbids
// Configure-Rejecting a peer Magic-Number option outright once ze transmits
// one, as it always does. The zero-value half is TestRFC1661ZeroMagicNumberRefused
// (internal/component/l2tp/ppp/rfc1661_test.go), which carries the
// RFC1661-6.4-1 tag, so this test carries none.
//
// RFC requirement: RFC1661-6-1 positive -- Magic-Number is a negotiable option
// and ze holds a desired value for it, so its invalid Length draws a
// Configure-Nak carrying that value at an appropriate Length (producer
// negotiatePeerOption's LCPOptMagic arm, internal/component/l2tp/ppp/lcp_options.go).
// RFC requirement: RFC1661-6-1 negative -- an option whose Length is valid and
// whose Type ze does not recognize draws a Configure-Reject instead, so the Nak
// answers the invalid Length rather than every malformed option.
func TestRFC1661LCPWrongLengthMagicIsNakedNotRejected(t *testing.T) {
	t.Run("wrong-length Magic-Number draws a Nak", func(t *testing.T) {
		s, rec, _ := newRFC1661Session(LCPStateReqSent)

		// Length 5 on a 6-octet option: three Magic-Number octets, one short.
		if term := s.handleFrame(lcpReqFrame(0x91, []byte{LCPOptMagic, 5, 0xDE, 0xAD, 0xBE})); term {
			t.Fatal("session terminated on a wrong-length Magic-Number")
		}
		nak := onlyLCPNak(t, rec)
		opts, err := ParseLCPOptions(nak.Data)
		if err != nil {
			t.Fatalf("Configure-Nak Data % x does not parse: %v", nak.Data, err)
		}
		if len(opts) != 1 || opts[0].Type != LCPOptMagic {
			t.Fatalf("Configure-Nak Data % x carries no Magic-Number option", nak.Data)
		}
		if len(opts[0].Data) != 4 {
			t.Fatalf("Nak'd Magic-Number carries %d octets, want the 4 RFC 1661 Section 6.4 gives it", len(opts[0].Data))
		}
		got := binary.BigEndian.Uint32(opts[0].Data)
		if got == 0 {
			t.Error("Nak'd Magic-Number is zero, which RFC 1661 Section 6.4 calls illegal")
		}
		if got == s.magic {
			t.Errorf("Nak'd Magic-Number = 0x%08x, which is ze's own; RFC 1661 Section 6.4 reads two equal Magic-Numbers as a looped-back link", got)
		}
	})

	t.Run("an unrecognized option is still Rejected", func(t *testing.T) {
		s, rec, _ := newRFC1661Session(LCPStateReqSent)

		if term := s.handleFrame(lcpReqFrame(0x92, []byte{99, 3, 0xAA})); term {
			t.Fatal("session terminated on an unrecognized option")
		}
		rej, ok := findCode(t, rec, LCPConfigureReject)
		if !ok {
			t.Fatalf("no Configure-Reject for an unrecognized option Type; frames=%d", rec.count())
		}
		opts, err := ParseLCPOptions(rej.Data)
		if err != nil {
			t.Fatalf("Configure-Reject Data % x does not parse: %v", rej.Data, err)
		}
		if len(opts) != 1 || opts[0].Type != 99 {
			t.Fatalf("Configure-Reject Data % x does not carry the unrecognized option", rej.Data)
		}
	})
}

// RFC requirement: RFC1661-5.4-1 positive -- a Configure-Request carrying both
// an unrecognized option Type and an invalid option Length draws the
// Configure-Reject Section 5.4 makes mandatory, not the Configure-Nak Section 6
// only recommends for the Length. The reply carries the unrecognized Type and
// no Nak is sent (producer LCPNakOrReject,
// internal/component/l2tp/ppp/session_run.go).
// RFC requirement: RFC1661-6-1 negative -- the Nak is confined to a request
// whose every recognized option is one ze holds a value for. An invalid Length
// beside an unrecognized Type loses the Nak to the Reject, so Section 6's
// SHOULD is not applied over Section 5.4's MUST.
func TestRFC1661LCPUnrecognizedTypeOutranksInvalidLength(t *testing.T) {
	t.Run("unrecognized Type before an invalid Length draws the Reject", func(t *testing.T) {
		s, rec, _ := newRFC1661Session(LCPStateReqSent)

		// Type 99 at Length 3 is well formed and unrecognized. The MRU that
		// follows declares Length 1, fewer octets than its own header holds,
		// so the walk stops there carrying the option already read.
		data := []byte{99, 3, 0xAA, LCPOptMRU, 1, 0x05}
		if term := s.handleFrame(lcpReqFrame(0xA1, data)); term {
			t.Fatal("session terminated on a Configure-Request carrying one fault of each kind")
		}
		if nak, ok := findCode(t, rec, LCPConfigureNak); ok {
			t.Fatalf("ze answered with a Configure-Nak carrying % x; RFC 1661 Section 5.4 makes the Configure-Reject mandatory and Section 6 only recommends the Nak", nak.Data)
		}
		rej, ok := findCode(t, rec, LCPConfigureReject)
		if !ok {
			t.Fatalf("no Configure-Reject for the unrecognized option Type; frames=%d", rec.count())
		}
		if rej.Identifier != 0xA1 {
			t.Errorf("Configure-Reject Identifier = 0x%02x, want 0xA1", rej.Identifier)
		}
		opts, err := ParseLCPOptions(rej.Data)
		if err != nil {
			t.Fatalf("Configure-Reject Data % x does not parse: %v", rej.Data, err)
		}
		if len(opts) != 1 || opts[0].Type != 99 {
			t.Fatalf("Configure-Reject Data % x does not carry the unrecognized option Type 99", rej.Data)
		}
	})

	t.Run("an invalid Length alone still draws the Nak", func(t *testing.T) {
		s, rec, _ := newRFC1661Session(LCPStateReqSent)

		// Magic-Number is recognized and ze holds a value for it, so nothing
		// in this request is Reject material. Only the trailing MRU Length is
		// invalid.
		data := append(optStream(magicOption(0xDEADBEEF)), LCPOptMRU, 1, 0x05)
		if term := s.handleFrame(lcpReqFrame(0xA2, data)); term {
			t.Fatal("session terminated on an invalid-Length Configure-Request")
		}
		nak := onlyLCPNak(t, rec)
		opts, err := ParseLCPOptions(nak.Data)
		if err != nil {
			t.Fatalf("Configure-Nak Data % x does not parse: %v", nak.Data, err)
		}
		found := false
		for _, opt := range opts {
			if opt.Type == LCPOptMRU {
				found = true
			}
		}
		if !found {
			t.Fatalf("Configure-Nak Data % x carries no MRU option; RFC 1661 Section 6 wants the desired option at an appropriate Length", nak.Data)
		}
	})
}

// RFC requirement: RFC1661-5.4-1 positive -- the same ordering binds the NCPs,
// whose option format RFC 1332 Section 3 and RFC 5072 Section 4 take from LCP:
// an unrecognized Type read before an invalid Length draws the Configure-Reject
// Section 5.4 makes mandatory (producer scanNCPOptions,
// internal/component/l2tp/ppp/ncp.go).
// RFC requirement: RFC1661-5.4-1 negative -- an invalid Length with every Type
// recognized reports no unrecognized Type, so the scan does not turn every
// malformed option list into a Reject.
func TestRFC1661NCPUnrecognizedTypeOutranksInvalidLength(t *testing.T) {
	cases := []struct {
		name string
		buf  []byte
		want ncpOptionScan
	}{
		// Type 99 at Length 4 is well formed and unrecognized; the
		// IP-Address that follows declares Length 1.
		{"unknown type then invalid length", []byte{99, 4, 0xDE, 0xAD, 3, 1}, ncpOptionsUnknownType},
		// Every Type is recognized, so only the Length is at fault.
		{"invalid length alone", []byte{3, 6, 10, 0, 0, 2, 3, 1}, ncpOptionsBadLength},
		// The Length is at offset zero, so the walk never reaches a Type it
		// could fail to recognize.
		{"invalid length first", []byte{3, 1, 99, 4}, ncpOptionsBadLength},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := scanNCPOptions(tc.buf, isKnownIPCPOption); got != tc.want {
				t.Errorf("scanNCPOptions(% x) = %d, want %d", tc.buf, got, tc.want)
			}
		})
	}

	t.Run("IPCP answers the mixed request with a Configure-Reject", func(t *testing.T) {
		td := newNCPTestDriverCfg(t, &StartSession{DisableIPv6CP: true})
		defer td.cleanup()
		settleNCPRequest(t, td, ProtoIPCP)

		td.writePeerNCPPacket(t, ProtoIPCP, LCPConfigureRequest, 0xA3, []byte{99, 4, 0xDE, 0xAD, 3, 1})

		replies := ncpRepliesWithin(t, td, ProtoIPCP, ncpSilenceWindow)
		if len(replies) != 1 {
			t.Fatalf("ze sent %d replies to the mixed request, want exactly 1", len(replies))
		}
		reply := replies[0]
		if reply.Code == LCPConfigureNak {
			t.Fatalf("ze answered with a Configure-Nak carrying % x; RFC 1661 Section 5.4 makes the Configure-Reject mandatory and Section 6 only recommends the Nak", reply.Data)
		}
		if reply.Code != LCPConfigureReject {
			t.Fatalf("reply code = %d, want Configure-Reject", reply.Code)
		}
		if reply.Identifier != 0xA3 {
			t.Errorf("Configure-Reject Identifier = 0x%02x, want 0xA3", reply.Identifier)
		}
		if len(reply.Data) < 2 || reply.Data[0] != 99 {
			t.Errorf("Configure-Reject Data % x does not carry the unrecognized option Type 99", reply.Data)
		}
	})
}

// RFC requirement: RFC1661-6-2 positive -- the Section 6 discard is written
// under the Configuration Option Data field and names no Code, so it binds a
// Configure-Ack, Nak and Reject as it binds a Configure-Request: an option whose
// Data runs past the end of the Information field leaves the automaton where it
// was and draws no reply (producer handleLCPPacket's reply-code gate, fed by
// WalkLCPOptions, internal/component/l2tp/ppp/session_run.go).
// RFC requirement: RFC1661-6-2 negative -- a reply whose options fit inside the
// packet still runs the automaton, so ze is not simply ignoring every reply.
func TestRFC1661LCPReplyWithOptionsPastEndDiscarded(t *testing.T) {
	// MRU declares Length 4 with only two of those four octets present.
	pastEnd := []byte{LCPOptMRU, 4, 0x05}

	t.Run("Configure-Ack past the end leaves req-sent", func(t *testing.T) {
		s, rec, _ := newRFC1661Session(LCPStateReqSent)

		if term := s.handleFrame(lcpFrame(ProtoLCP, LCPConfigureAck, 0xB1, pastEnd)); term {
			t.Fatal("session terminated on a Configure-Ack RFC 1661 Section 6 discards")
		}
		if n := rec.count(); n != 0 {
			t.Fatalf("ze answered a truncated Configure-Ack with %d frame(s)", n)
		}
		if got := s.currentState(); got != LCPStateReqSent {
			t.Fatalf("state = %s, want req-sent; RFC 1661 Section 6 discards without affecting the automaton", got)
		}
	})

	t.Run("Configure-Ack that fits still runs the automaton", func(t *testing.T) {
		s, _, _ := newRFC1661Session(LCPStateReqSent)

		if term := s.handleFrame(lcpFrame(ProtoLCP, LCPConfigureAck, 0xB2, optStream(mruOption(1460)))); term {
			t.Fatal("session terminated on a well-formed Configure-Ack")
		}
		if got := s.currentState(); got != LCPStateAckRcvd {
			t.Fatalf("state = %s, want ack-rcvd", got)
		}
	})

	for _, code := range []struct {
		name string
		code uint8
	}{
		{"Configure-Nak", LCPConfigureNak},
		{"Configure-Reject", LCPConfigureReject},
	} {
		t.Run(code.name+" past the end leaves ack-rcvd", func(t *testing.T) {
			s, rec, _ := newRFC1661Session(LCPStateAckRcvd)

			if term := s.handleFrame(lcpFrame(ProtoLCP, code.code, 0xB3, pastEnd)); term {
				t.Fatalf("session terminated on a %s RFC 1661 Section 6 discards", code.name)
			}
			if n := rec.count(); n != 0 {
				t.Fatalf("ze answered a truncated %s with %d frame(s); RFC 1661 Section 4.1 makes RCN in ack-rcvd resend the Configure-Request", code.name, n)
			}
			if got := s.currentState(); got != LCPStateAckRcvd {
				t.Fatalf("state = %s, want ack-rcvd; RFC 1661 Section 6 discards without affecting the automaton", got)
			}
		})

		t.Run(code.name+" that fits still runs the automaton", func(t *testing.T) {
			s, _, _ := newRFC1661Session(LCPStateAckRcvd)

			if term := s.handleFrame(lcpFrame(ProtoLCP, code.code, 0xB4, optStream(mruOption(1460)))); term {
				t.Fatalf("session terminated on a well-formed %s", code.name)
			}
			if got := s.currentState(); got != LCPStateReqSent {
				t.Fatalf("state = %s, want req-sent", got)
			}
		})
	}
}

// RFC requirement: RFC1661-6-2 positive -- the same Code-blind discard binds the
// NCPs. An IPCP Configure-Reject whose option Data runs past the end of the
// packet is dropped rather than read, so the session it would otherwise tear
// down survives and keeps answering (producer handleNCPPacket's reply-code gate,
// fed by scanNCPOptions, internal/component/l2tp/ppp/ncp.go).
// RFC requirement: RFC1661-6-2 negative -- a Configure-Reject whose options fit
// is still acted on: a peer Reject of IP-Address remains fatal, so the discard
// has not disabled the path.
func TestRFC1661NCPReplyWithOptionsPastEndDiscarded(t *testing.T) {
	t.Run("truncated Configure-Reject does not tear the session down", func(t *testing.T) {
		td := newNCPTestDriverCfg(t, &StartSession{DisableIPv6CP: true})
		defer td.cleanup()
		settleNCPRequest(t, td, ProtoIPCP)

		// IP-Address declares Length 6 with only four octets of the option
		// present, so its Data runs two octets past the packet.
		td.writePeerNCPPacket(t, ProtoIPCP, LCPConfigureReject, 0xC1, []byte{IPCPOptIPAddress, 6, 10, 0})

		if replies := ncpRepliesWithin(t, td, ProtoIPCP, ncpSilenceWindow); len(replies) != 0 {
			t.Fatalf("ze answered a truncated Configure-Reject with %d packet(s), first code %d", len(replies), replies[0].Code)
		}

		// The session must still be live: a valid, acceptable request is
		// still Acked.
		acceptable := []byte{
			IPCPOptIPAddress, 6, 10, 0, 0, 2,
			IPCPOptPrimaryDNS, 6, 1, 1, 1, 1,
			IPCPOptSecondaryDNS, 6, 8, 8, 8, 8,
		}
		// A closed pipe here is the session gone: absorbIPCPReject reads a
		// truncated Reject as a peer Reject of the mandatory IP-Address
		// option and fails the session, which is the behavior Section 6
		// forbids.
		buf := make([]byte, MaxFrameLen)
		off := WriteFrame(buf, 0, ProtoIPCP, nil)
		off += WriteLCPPacket(buf, off, LCPConfigureRequest, 0xC2, acceptable)
		if _, err := td.peer.Write(buf[:off]); err != nil {
			t.Fatalf("peer write after the truncated Configure-Reject: %v; the session was torn down by a packet RFC 1661 Section 6 discards", err)
		}
		ack := readIPCPUntil(t, td, LCPConfigureAck)
		if ack.Identifier != 0xC2 {
			t.Errorf("Configure-Ack Identifier = 0x%02x, want 0xC2", ack.Identifier)
		}
	})
}

// countOptionType answers how many entries of one Type an option list carries.
func countOptionType(opts []LCPOption, optType uint8) int {
	n := 0
	for _, opt := range opts {
		if opt.Type == optType {
			n++
		}
	}
	return n
}

// VALIDATES: the reply ze builds names each Configuration Option Type once,
//
//	even when the request lists that Type twice and the second instance is
//	the one carrying the invalid Length.
//
// PREVENTS: a Configure-Nak or Configure-Reject carrying two entries for one
// Type, which happens when the entry the fault earns is appended beside the
// entry an earlier instance of the same Type already put in the list. RFC 1661
// Section 6 says of the options it defines: "(None of the Configuration
// Options in this specification can be listed more than once.)" A peer that
// reads two entries for one Type has no rule to pick between them.
//
// RFC requirement: RFC1661-6-1 positive -- the Configure-Nak an invalid Length
// draws carries the desired Configuration Option, once, whatever the request
// listed before the fault (producer LCPNakOrReject and its appendUnlessListed,
// internal/component/l2tp/ppp/session_run.go).
// RFC requirement: RFC1661-6-1 negative -- the option the fault earns is
// dropped only when the reply already names its Type. A request whose earlier
// instance of that Type was acceptable, and so is absent from the reply, still
// draws the desired option, so ze does not answer the invalid Length with
// silence about the option it wants.
func TestRFC1661ReplyListsEachOptionTypeOnce(t *testing.T) {
	t.Run("an MRU already Nak'd is not Nak'd a second time", func(t *testing.T) {
		s, rec, _ := newRFC1661Session(LCPStateReqSent)

		// The first MRU is well formed and asks for 2000, above the 1500 ze
		// accepts, so it earns a Nak of its own. The second declares Length 1,
		// fewer octets than its own header holds, so the walk stops there and
		// the fault names the same Type.
		data := append(optStream(mruOption(2000)), LCPOptMRU, 1, 0x05)
		if term := s.handleFrame(lcpReqFrame(0xB1, data)); term {
			t.Fatal("session terminated on a request listing MRU twice")
		}
		nak := onlyLCPNak(t, rec)
		opts, err := ParseLCPOptions(nak.Data)
		if err != nil {
			t.Fatalf("Configure-Nak Data % x does not parse: %v", nak.Data, err)
		}
		if got := countOptionType(opts, LCPOptMRU); got != 1 {
			t.Fatalf("Configure-Nak Data % x carries %d MRU options, want 1; RFC 1661 Section 6: \"(None of the Configuration Options in this specification can be listed more than once.)\"", nak.Data, got)
		}
		if len(opts) != 1 {
			t.Fatalf("Configure-Nak Data % x carries %d options, want the MRU alone", nak.Data, len(opts))
		}
		if got := binary.BigEndian.Uint16(opts[0].Data); got != MaxFrameLen {
			t.Errorf("Nak'd MRU = %d, want the %d ze accepts", got, MaxFrameLen)
		}
	})

	t.Run("a Type already Rejected is not Rejected a second time", func(t *testing.T) {
		s, rec, _ := newRFC1661Session(LCPStateReqSent)

		// Type 99 is unrecognized, so its first instance earns a Reject. The
		// second declares Length 1 and ze holds no desired value for the Type,
		// so the fault reaches the same Reject list.
		data := []byte{99, 3, 0xAA, 99, 1, 0xBB}
		if term := s.handleFrame(lcpReqFrame(0xB2, data)); term {
			t.Fatal("session terminated on a request listing an unrecognized Type twice")
		}
		rej, ok := findCode(t, rec, LCPConfigureReject)
		if !ok {
			t.Fatalf("no Configure-Reject for the unrecognized option Type; frames=%d", rec.count())
		}
		opts, err := ParseLCPOptions(rej.Data)
		if err != nil {
			t.Fatalf("Configure-Reject Data % x does not parse: %v", rej.Data, err)
		}
		if got := countOptionType(opts, 99); got != 1 {
			t.Fatalf("Configure-Reject Data % x carries %d entries of Type 99, want 1; RFC 1661 Section 6: \"(None of the Configuration Options in this specification can be listed more than once.)\"", rej.Data, got)
		}
		if len(opts) != 1 {
			t.Fatalf("Configure-Reject Data % x carries %d options, want Type 99 alone", rej.Data, len(opts))
		}
	})

	t.Run("an acceptable first instance still leaves the desired option in the Nak", func(t *testing.T) {
		s, rec, _ := newRFC1661Session(LCPStateReqSent)

		// 1460 is inside the range ze accepts, so the first MRU is acked and
		// nothing about it reaches the Nak. The desired option the second
		// instance earns must therefore still be listed.
		data := append(optStream(mruOption(1460)), LCPOptMRU, 1, 0x05)
		if term := s.handleFrame(lcpReqFrame(0xB3, data)); term {
			t.Fatal("session terminated on a request whose first MRU was acceptable")
		}
		nak := onlyLCPNak(t, rec)
		opts, err := ParseLCPOptions(nak.Data)
		if err != nil {
			t.Fatalf("Configure-Nak Data % x does not parse: %v", nak.Data, err)
		}
		if got := countOptionType(opts, LCPOptMRU); got != 1 {
			t.Fatalf("Configure-Nak Data % x carries %d MRU options, want the 1 RFC 1661 Section 6 asks for", nak.Data, got)
		}
	})
}

// RFC requirement: RFC1661-5.4-2 positive -- the Configure-Reject carries the
// refused option exactly as it arrived, its own Length octet included, for the
// two Lengths ze cannot re-encode. RFC 1661 Section 5.4: "The Options field is
// filled with only the unacceptable Configuration Options from the Configure-
// Request. All recognizable and negotiable Configuration Options are filtered
// out of the Configure-Reject, but otherwise the Configuration Options MUST NOT
// be reordered or modified in any way." A Length below 2 is one the option
// encoder can never write, so rebuilding the option from its Type sends back an
// option the peer never sent (producer LCPNakOrReject's LCPOptionsBadLength arm
// feeding LCPOption.Raw through WriteLCPOptions,
// internal/component/l2tp/ppp/session_run.go).
// RFC requirement: RFC1661-5.4-2 negative -- "modified in any way" governs the
// options the Reject carries, never which options it carries. An option ze
// recognizes and can negotiate is still filtered OUT, so the echo does not turn
// the Configure-Reject into a copy of the request.
func TestRFC1661LCPRejectEchoesTheRefusedOptionUnmodified(t *testing.T) {
	// Type 99 is unrecognized, so ze holds no value to Nak it with and
	// Section 5.4 takes it. Both Lengths count fewer octets than the option's
	// own Type and Length fields occupy, and both are inside the packet.
	echoCases := []struct {
		name string
		data []byte
	}{
		{"unrecognized Type at Length 0", []byte{99, 0}},
		{"unrecognized Type at Length 1", []byte{99, 1}},
	}
	for _, tc := range echoCases {
		t.Run(tc.name, func(t *testing.T) {
			s, rec, _ := newRFC1661Session(LCPStateReqSent)

			if term := s.handleFrame(lcpReqFrame(0xB1, tc.data)); term {
				t.Fatal("session terminated on a Configure-Request carrying an invalid option Length")
			}
			rej, ok := findCode(t, rec, LCPConfigureReject)
			if !ok {
				t.Fatalf("no Configure-Reject for the unrecognized option Type; frames=%d", rec.count())
			}
			if !bytes.Equal(rej.Data, tc.data) {
				t.Fatalf("Configure-Reject Data = % x, want the % x that arrived; RFC 1661 Section 5.4 forbids modifying the option in any way", rej.Data, tc.data)
			}
		})
	}

	t.Run("a recognizable negotiable option is filtered out of the Reject", func(t *testing.T) {
		s, rec, _ := newRFC1661Session(LCPStateReqSent)

		// MRU at 1500 is recognized, negotiable and acceptable, so Section 5.4
		// filters it out. The unrecognized Type at Length 0 that follows is
		// the whole of what the Reject owes.
		refused := []byte{99, 0}
		request := append(optStream(mruOption(MaxFrameLen)), refused...)
		if term := s.handleFrame(lcpReqFrame(0xB2, request)); term {
			t.Fatal("session terminated on a Configure-Request carrying an acceptable option and an invalid Length")
		}
		rej, ok := findCode(t, rec, LCPConfigureReject)
		if !ok {
			t.Fatalf("no Configure-Reject for the unrecognized option Type; frames=%d", rec.count())
		}
		if !bytes.Equal(rej.Data, refused) {
			t.Fatalf("Configure-Reject Data = % x, want the % x that earned it alone; RFC 1661 Section 5.4 filters every recognizable and negotiable option out", rej.Data, refused)
		}
	})
}
