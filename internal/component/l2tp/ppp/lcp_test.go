package ppp

import (
	"bytes"
	"errors"
	"testing"
)

// VALIDATES: ParseLCPPacket decodes Code, Identifier, Length and
//
//	returns Data as a sub-slice of the input.
//
// PREVENTS: regressions where Data is allocated or Length is treated
//
//	as the data length instead of the total packet length.
func TestLCPPacketParse(t *testing.T) {
	// Configure-Request, id=0x42, length=8, data=[0x01 0x02 0x03 0x04]
	buf := []byte{0x01, 0x42, 0x00, 0x08, 0x01, 0x02, 0x03, 0x04}
	pkt, err := ParseLCPPacket(buf)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if pkt.Code != LCPConfigureRequest {
		t.Errorf("Code = %d, want %d", pkt.Code, LCPConfigureRequest)
	}
	if pkt.Identifier != 0x42 {
		t.Errorf("Identifier = 0x%02x, want 0x42", pkt.Identifier)
	}
	if !bytes.Equal(pkt.Data, []byte{0x01, 0x02, 0x03, 0x04}) {
		t.Errorf("Data = %x, want 01020304", pkt.Data)
	}
	if &pkt.Data[0] != &buf[lcpHeaderLen] {
		t.Errorf("Data should sub-slice into buf; got fresh allocation")
	}
}

// VALIDATES: ParseLCPPacket honors Length and ignores trailing padding.
// PREVENTS: parser consuming bytes beyond Length, which would corrupt
//
//	the next packet in a batched read.
func TestLCPPacketParseIgnoresPadding(t *testing.T) {
	// Length=4 (header only). Trailing bytes are padding.
	buf := []byte{0x01, 0x01, 0x00, 0x04, 0xFF, 0xFF, 0xFF}
	pkt, err := ParseLCPPacket(buf)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(pkt.Data) != 0 {
		t.Errorf("Data = %x, want empty (Length=4 means header only)", pkt.Data)
	}
}

// VALIDATES: ParseLCPPacket rejects buffers shorter than the header.
func TestLCPPacketParseTooShort(t *testing.T) {
	for _, n := range []int{0, 1, 2, 3} {
		_, err := ParseLCPPacket(make([]byte, n))
		if !errors.Is(err, errLCPTooShort) {
			t.Errorf("len=%d: err = %v, want errLCPTooShort", n, err)
		}
	}
}

// VALIDATES: ParseLCPPacket rejects Length field below 4, above buf
//
//	length, or above MaxFrameLen.
func TestLCPPacketParseInvalidLength(t *testing.T) {
	cases := []struct {
		name string
		buf  []byte
	}{
		{"length 3 (below header)", []byte{0x01, 0x00, 0x00, 0x03, 0xAA}},
		{"length exceeds buffer", []byte{0x01, 0x00, 0x00, 0x10, 0xAA, 0xBB}},
		{"length exceeds frame max", append([]byte{0x01, 0x00, 0xFA, 0x00}, bytes.Repeat([]byte{0xAA}, 64000)...)},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := ParseLCPPacket(tc.buf)
			if !errors.Is(err, errLCPLengthMismatch) {
				t.Errorf("err = %v, want errLCPLengthMismatch", err)
			}
		})
	}
}

// VALIDATES: WriteLCPPacket backfills the Length field with the total
//
//	packet length and writes Code/Identifier/Data correctly.
//
// PREVENTS: regressions where Length is set to the data length instead
//
//	of total length, or where Length is left zeroed.
func TestLCPPacketWriteTo(t *testing.T) {
	data := []byte{0x05, 0x06, 0x00, 0x00}
	buf := make([]byte, 16)
	n := WriteLCPPacket(buf, 0, LCPConfigureAck, 0x37, data)
	if n != 8 {
		t.Errorf("n = %d, want 8", n)
	}
	want := []byte{0x02, 0x37, 0x00, 0x08, 0x05, 0x06, 0x00, 0x00}
	if !bytes.Equal(buf[:n], want) {
		t.Errorf("buf = %x, want %x", buf[:n], want)
	}
}

// VALIDATES: WriteLCPPacket writes at the requested offset, not 0.
func TestLCPPacketWriteToOffset(t *testing.T) {
	data := []byte{0xAA}
	buf := []byte{0xFF, 0xFF, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00}
	n := WriteLCPPacket(buf, 2, LCPEchoReply, 0x01, data)
	if n != 5 {
		t.Errorf("n = %d, want 5", n)
	}
	if buf[0] != 0xFF || buf[1] != 0xFF {
		t.Errorf("prefix overwritten: %x", buf[:2])
	}
	if buf[2] != LCPEchoReply || buf[3] != 0x01 || buf[4] != 0x00 || buf[5] != 0x05 {
		t.Errorf("packet bytes wrong: %x", buf[2:6])
	}
}

// VALIDATES: WriteLCPPacket / ParseLCPPacket round-trip.
func TestLCPPacketRoundTrip(t *testing.T) {
	cases := []struct {
		name string
		code uint8
		id   uint8
		data []byte
	}{
		{"echo-request empty", LCPEchoRequest, 1, nil},
		{"configure-request typical", LCPConfigureRequest, 1, []byte{0x01, 0x04, 0x05, 0xDC, 0x05, 0x06, 0x00, 0x01, 0x02, 0x03}},
		{"large", LCPConfigureRequest, 0, bytes.Repeat([]byte{0x55}, 256)},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			buf := make([]byte, MaxFrameLen)
			n := WriteLCPPacket(buf, 0, tc.code, tc.id, tc.data)
			pkt, err := ParseLCPPacket(buf[:n])
			if err != nil {
				t.Fatalf("parse error: %v", err)
			}
			if pkt.Code != tc.code {
				t.Errorf("code mismatch")
			}
			if pkt.Identifier != tc.id {
				t.Errorf("id mismatch")
			}
			if !bytes.Equal(pkt.Data, tc.data) {
				t.Errorf("data mismatch")
			}
		})
	}
}

// VALIDATES: LCPCodeName returns lowercase well-known names and a
//
//	"code-N" fallback for unknown codes.
func TestLCPCodeName(t *testing.T) {
	cases := []struct {
		code uint8
		want string
	}{
		{LCPConfigureRequest, "configure-request"},
		{LCPEchoReply, "echo-reply"},
		{LCPCodeReject, "code-reject"},
		{0, "code-0"},
		{255, "code-255"},
		{99, "code-99"},
	}
	for _, tc := range cases {
		got := LCPCodeName(tc.code)
		if got != tc.want {
			t.Errorf("LCPCodeName(%d) = %q, want %q", tc.code, got, tc.want)
		}
	}
}

// VALIDATES: WriteLCPOptions refuses a list that does not fit the buffer it
//
//	was given, rather than writing past the end of it, and it refuses the
//	whole list rather than the prefix that fit.
//
// PREVENTS: the writer indexing past a MaxFrameLen frame buffer. Every option
// it emits used to be written with no bound at all, on the caller's promise
// that the buffer was large enough. The peer sizes a Configure-Nak, so that
// promise was the peer's to keep.
func TestWriteLCPOptionsRefusesAListThatDoesNotFit(t *testing.T) {
	opts := []LCPOption{
		{Type: LCPOptMagic, Data: []byte{0xDE, 0xAD, 0xBE, 0xEF}},
		{Type: LCPOptMRU, Data: []byte{0x05, 0xDC}},
	}

	// Ten octets of options into a nine-octet buffer: the Magic-Number fits
	// and the MRU does not.
	buf := make([]byte, 9)
	n, fits := WriteLCPOptions(buf, 0, opts)
	if fits {
		t.Fatalf("WriteLCPOptions reported that %d octets of options fit a %d-octet buffer", 10, len(buf))
	}
	if n != 6 {
		t.Errorf("octets written = %d, want the 6 of the option that fit", n)
	}

	// The same list fits a buffer sized for it, so the refusal above follows
	// from the room and not from the list.
	buf = make([]byte, 10)
	n, fits = WriteLCPOptions(buf, 0, opts)
	if !fits {
		t.Fatalf("WriteLCPOptions refused %d octets of options in a %d-octet buffer", 10, len(buf))
	}
	if n != 10 {
		t.Errorf("octets written = %d, want 10", n)
	}
}

// VALIDATES: LCPNakOrReject reports that there is nothing to send when the
//
//	walk it reads earned no reply entry, instead of naming a Configure-Nak
//	with an empty Options field.
//
// PREVENTS: ze transmitting a Configure-Nak that carries no unacceptable
// option. RFC 1661 Section 5.3: "The Options field is filled with only the
// unacceptable Configuration Options from the Configure-Request." A walk that
// reports an invalid option Length without the octets it read leaves ze
// nothing to name, because Section 5.4 forbids modifying the option it would
// have to rebuild instead. WalkLCPOptions sets FaultRaw on every such walk, so
// only a Ze defect assembles this one, and the reply it used to draw is a
// packet neither section describes.
func TestLCPNakOrRejectSendsNothingWithoutAnEntry(t *testing.T) {
	policy := LCPNegPolicy{MaxMRU: MaxFrameLen, LocalMagic: 0x01020304}

	// Type 99 is unrecognized, so ze holds no desired value to Nak it with,
	// and the walk carries no FaultRaw for a Reject to echo.
	code, opts, reply := LCPNakOrReject(LCPOptionWalk{Fault: LCPOptionsBadLength, FaultOpt: 99}, policy)
	if reply {
		t.Errorf("LCPNakOrReject asked for a %s carrying %d options; a walk that earned no entry earns no reply", LCPCodeName(code), len(opts))
	}

	// The same walk with the octets it read earns a Configure-Reject, so the
	// silence above follows from the missing entry and not from the fault.
	code, opts, reply = LCPNakOrReject(LCPOptionWalk{Fault: LCPOptionsBadLength, FaultOpt: 99, FaultRaw: []byte{99, 0}}, policy)
	if !reply {
		t.Fatal("LCPNakOrReject sent nothing for a walk carrying the refused option")
	}
	if code != LCPConfigureReject {
		t.Errorf("reply code = %s, want Configure-Reject", LCPCodeName(code))
	}
	if len(opts) != 1 {
		t.Fatalf("reply carries %d options, want the 1 that earned it", len(opts))
	}
	if !bytes.Equal(opts[0].Raw, []byte{99, 0}) {
		t.Errorf("Configure-Reject option = % x, want the % x that arrived", opts[0].Raw, []byte{99, 0})
	}
}

// VALIDATES: an option list whose octets do not fit the buffer is not written
//
//	as the prefix of itself, and a Configure-Request repeating one option is
//	answered with one entry for it rather than one per instance.
//
// PREVENTS: the daemon panicking on a packet any unauthenticated peer can
// send. A Magic-Number received at Length 2 is answered by the six octets RFC
// 1661 Section 6.4 gives the option, so a frame filled with them asked for a
// Configure-Nak three times the size of the request. WriteLCPOptions wrote
// every one of those octets into a MaxFrameLen buffer with no bound, and the
// index past the end of it took the session goroutine down.
//
// NegotiatePeerOptions now names each option Type once, so that request can no
// longer build an oversized reply. The bound is still asserted directly:
// WriteLCPOptions serves the PPPoE client and LCPNakOrReject as well, and a
// writer with no bound is one caller away from the panic again.
func TestLCPReplyLargerThanAFrameIsNotSent(t *testing.T) {
	// 200 distinct option Types, each carrying eight octets of Data, is 2000
	// octets of options. No Configure-Request can ask for this today; the
	// bound is what keeps that true for the next caller.
	const distinctTypes = 200
	opts := make([]LCPOption, 0, distinctTypes)
	for i := range distinctTypes {
		opts = append(opts, LCPOption{Type: uint8(i + 1), Data: make([]byte, 8)})
	}

	buf := make([]byte, MaxFrameLen)
	written, fits := WriteLCPOptions(buf, 0, opts)
	if fits {
		t.Fatalf("WriteLCPOptions reported %d octets of a %d-octet list fit a %d-octet buffer", written, distinctTypes*10, MaxFrameLen)
	}
	if written > MaxFrameLen {
		t.Fatalf("WriteLCPOptions wrote %d octets into a %d-octet buffer", written, MaxFrameLen)
	}
}

// VALIDATES: the same bound, reached the way a peer reaches it. A
//
//	Configure-Request carrying many DISTINCT option Types still asks for a
//	reply larger than a frame, so ze sends nothing rather than a prefix, and
//	its automaton stays usable afterwards.
//
// PREVENTS: the bound surviving only as a helper test. Naming each Type once
// (NegotiatePeerOptions) stopped a REPEATED option from building an oversized
// reply, and that is the whole of what it stopped: distinct Types are not
// deduplicated and cannot be, because RFC 1661 Section 5.4 requires every
// unrecognizable option to appear in the Configure-Reject. So the entry point
// still reaches the writer's bound and is still the place to assert it.
func TestLCPReplyLargerThanAFrameIsNotSentFromTheWire(t *testing.T) {
	s, rec, _ := newRFC1661Session(LCPStateReqSent)

	// 200 option Types ze does not recognize, each carrying eight octets, is
	// 2000 octets of Configure-Reject entries for a 1406-octet frame. Types
	// from 0xC0 up are outside the set RFC 1661 Section 6 defines.
	const distinctTypes = 200
	request := make([]byte, 0, distinctTypes*10)
	for i := range distinctTypes {
		request = append(request, uint8(0xC0+i%0x40), 10)
		request = append(request, make([]byte, 8)...)
	}

	if term := s.handleFrame(lcpReqFrame(0xE1, request)); term {
		t.Fatal("session terminated on a Configure-Request whose reply does not fit a frame")
	}
	if _, ok := findCode(t, rec, LCPConfigureReject); ok {
		t.Fatal("a Configure-Reject was sent for a reply that cannot fit a frame")
	}

	// The automaton is where it was: a well-formed acceptable request is
	// still acknowledged.
	if term := s.handleFrame(lcpReqFrame(0xE2, optStream(mruOption(MaxFrameLen)))); term {
		t.Fatal("session terminated on the well-formed request that followed")
	}
	ack, ok := findCode(t, rec, LCPConfigureAck)
	if !ok {
		t.Fatalf("no Configure-Ack for the well-formed request that followed; frames=%d", rec.count())
	}
	if ack.Identifier != 0xE2 {
		t.Errorf("Configure-Ack Identifier = 0x%02x, want 0xE2", ack.Identifier)
	}
}

// VALIDATES: a Configure-Request repeating one option draws a reply naming
//
//	that option once, and leaves ze running with its automaton usable.
//
// PREVENTS: the reply the peer sizes. 700 Magic-Number options at Length 2 fit
// a 1406-octet frame and each earns a six-octet Nak entry, so the answer ze
// owed was 4200 octets and the answer ze sent was nothing at all.
//
// RFC requirement: RFC1661-5.3-3 positive -- RFC 1661 Section 5.3: "Each
// Configuration Option which is allowed only a single instance MUST be
// modified to a value acceptable to the Configure-Nak sender." Magic-Number is
// one of those, by RFC 1661 Section 6: "(None of the Configuration Options in
// this specification can be listed more than once.)" So the Configure-Nak
// NegotiatePeerOptions (lcp_options.go) builds carries one Magic-Number entry
// whatever the request repeated, and it carries a value ze accepts.
func TestRFC1661RepeatedOptionDrawsOneNakEntry(t *testing.T) {
	s, rec, _ := newRFC1661Session(LCPStateReqSent)

	// 700 Magic-Number options at Length 2, 1400 octets, inside a frame. Each
	// one is well formed as a header and invalid for its Type.
	const magicOptions = 700
	request := make([]byte, 0, magicOptions*2)
	for range magicOptions {
		request = append(request, LCPOptMagic, 2)
	}

	if term := s.handleFrame(lcpReqFrame(0xD1, request)); term {
		t.Fatal("session terminated on a Configure-Request repeating one option")
	}

	nak, ok := findCode(t, rec, LCPConfigureNak)
	if !ok {
		t.Fatalf("no Configure-Nak for a request of %d invalid Magic-Number options; frames=%d", magicOptions, rec.count())
	}
	opts, err := ParseLCPOptions(nak.Data)
	if err != nil {
		t.Fatalf("Configure-Nak Data % x does not parse: %v", nak.Data, err)
	}
	if len(opts) != 1 {
		t.Fatalf("Configure-Nak carries %d options for %d repetitions of one Type, want 1", len(opts), magicOptions)
	}
	if opts[0].Type != LCPOptMagic {
		t.Fatalf("Configure-Nak carries option Type %d, want Magic-Number %d", opts[0].Type, LCPOptMagic)
	}
	if len(opts[0].Data) != 4 {
		t.Fatalf("Nak'd Magic-Number carries %d octets, want the 4 RFC 1661 Section 6.4 gives it", len(opts[0].Data))
	}

	// The automaton is where it was: a well-formed acceptable request is
	// still acknowledged.
	if term := s.handleFrame(lcpReqFrame(0xD2, optStream(mruOption(MaxFrameLen)))); term {
		t.Fatal("session terminated on the well-formed request that followed")
	}
	ack, ok := findCode(t, rec, LCPConfigureAck)
	if !ok {
		t.Fatalf("no Configure-Ack for the well-formed request that followed; frames=%d", rec.count())
	}
	if ack.Identifier != 0xD2 {
		t.Errorf("Configure-Ack Identifier = 0x%02x, want 0xD2", ack.Identifier)
	}
}

// RFC requirement: RFC1661-5.2-3 positive -- a matching Configure-Ack advances Req-Sent to Ack-Rcvd.
// RFC requirement: RFC1661-5.2-3 negative -- an Ack for another Identifier leaves the FSM and wire unchanged.
// RFC requirement: RFC1661-5.2-4 positive -- the Ack must echo the complete request options before negotiation advances.
// RFC requirement: RFC1661-5.2-4 negative -- changed, missing, extra or reordered Ack options leave negotiation unchanged.
// RFC requirement: RFC1661-5.3-7 positive -- a Nak for the outstanding Identifier produces the next Configure-Request.
// RFC requirement: RFC1661-5.3-7 negative -- a Nak for another Identifier neither changes the request nor enters the FSM.
// RFC requirement: RFC1661-5.4-3 positive -- a Reject for the outstanding Identifier produces a request without the rejected option.
// RFC requirement: RFC1661-5.4-3 negative -- a Reject for another Identifier leaves the request and FSM unchanged.
// RFC requirement: RFC1661-5.4-4 positive -- an ordered unchanged subset in a Reject is accepted.
// RFC requirement: RFC1661-5.4-4 negative -- added, changed or reordered Reject options are silently discarded.
func TestLCPRepliesMatchOutstandingRequest(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name string
		code uint8
		breakReply func(*LCPPacket)
	}{
		{"Ack wrong Identifier", LCPConfigureAck, func(p *LCPPacket) { p.Identifier++ }},
		{"Ack changed value", LCPConfigureAck, func(p *LCPPacket) { p.Data[len(p.Data)-1] ^= 1 }},
		{"Ack missing option", LCPConfigureAck, func(p *LCPPacket) { p.Data = p.Data[:4] }},
		{"Ack extra option", LCPConfigureAck, func(p *LCPPacket) { p.Data = append(p.Data, LCPOptPFC, 2) }},
		{"Ack reordered options", LCPConfigureAck, func(p *LCPPacket) {
			p.Data = append(append([]byte(nil), p.Data[4:]...), p.Data[:4]...)
		}},
		{"Nak wrong Identifier", LCPConfigureNak, func(p *LCPPacket) { p.Identifier++ }},
		{"Nak reordered options", LCPConfigureNak, func(p *LCPPacket) {
			p.Data = optStream(magicOption(0x22223333), mruOption(1400))
		}},
		{"Nak duplicate option", LCPConfigureNak, func(p *LCPPacket) { p.Data = append(p.Data, p.Data...) }},
		{"Nak truncated option", LCPConfigureNak, func(p *LCPPacket) { p.Data = p.Data[:3] }},
		{"Reject wrong Identifier", LCPConfigureReject, func(p *LCPPacket) { p.Identifier++ }},
		{"Nak invalid MRU length", LCPConfigureNak, func(p *LCPPacket) { p.Data = []byte{LCPOptMRU, 3, 5} }},
		{"Reject changed value", LCPConfigureReject, func(p *LCPPacket) { p.Data[3] ^= 1 }},
		{"Reject unrequested option", LCPConfigureReject, func(p *LCPPacket) { p.Data = []byte{222, 2} }},
		{"Reject reordered options", LCPConfigureReject, func(p *LCPPacket) {
			p.Data = optStream(magicOption(0x01020304), mruOption(MaxFrameLen))
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			s, rec, _ := newRFC1661Session(LCPStateReqSent)
			if !s.sendConfigureRequest() {
				t.Fatal("initial request failed")
			}
			request := lastLCPConfigureRequest(t, rec)
			data := request.Data
			switch tc.code {
			case LCPConfigureNak:
				data = optStream(mruOption(1400))
			case LCPConfigureReject:
				data = request.Data[:4]
			}
			reply := LCPPacket{Code: tc.code, Identifier: request.Identifier, Data: bytes.Clone(data)}
			tc.breakReply(&reply)
			before := rec.count()
			if s.handleLCPPacket(reply) {
				t.Fatal("invalid reply terminated the session")
			}
			if s.currentState() != LCPStateReqSent || rec.count() != before {
				t.Fatal("invalid reply changed the FSM or emitted a packet")
			}
			if s.magic != 0x01020304 {
				t.Fatal("invalid reply changed the local Magic-Number")
			}
			reply = LCPPacket{Code: tc.code, Identifier: request.Identifier, Data: data}
			if s.handleLCPPacket(reply) {
				t.Fatal("matching reply terminated the session")
			}
			if tc.code == LCPConfigureAck {
				if s.currentState() != LCPStateAckRcvd || !s.magicNegotiated {
					t.Fatal("matching Ack did not advance negotiation")
				}
				return
			}
			next := lastLCPConfigureRequest(t, rec)
			if rec.count() != before+1 || next.Identifier == request.Identifier {
				t.Fatal("matching negative reply did not produce a fresh request")
			}
			opts, err := ParseLCPOptions(next.Data)
			if err != nil {
				t.Fatal(err)
			}
			mru, hasMRU := lookupMRUOption(opts)
			if tc.code == LCPConfigureNak && (!hasMRU || mru != 1400) {
				t.Fatal("next request did not carry the accepted MRU suggestion")
			}
			if tc.code == LCPConfigureReject && hasMRU {
				t.Fatal("next request repeated the rejected MRU option")
			}
		})
	}
}

// RFC requirement: RFC1661-5.1-3 positive -- changed request data and a valid reply both cause a fresh Configure-Request Identifier.
// RFC requirement: RFC1661-5.1-3 negative -- a lost reply retransmits the outstanding request unchanged, while a mismatching Ack cannot advance it.
func TestLCPConfigureRequestIdentifierLifecycle(t *testing.T) {
	s, rec, _ := newRFC1661Session(LCPStateClosed)
	if s.applyTransition(LCPStateClosed, LCPDoTransition(LCPStateClosed, LCPEventOpen), LCPPacket{}) {
		t.Fatal("opening session failed")
	}
	first := lastLCPConfigureRequest(t, rec)
	if s.handleRestartTimeout() {
		t.Fatal("first retry exhausted the session")
	}
	retry := lastLCPConfigureRequest(t, rec)
	if retry.Identifier != first.Identifier || !bytes.Equal(retry.Data, first.Data) {
		t.Fatal("timeout retransmission did not preserve the outstanding request")
	}
	s.magic ^= 1
	if !s.sendConfigureRequest() {
		t.Fatal("changed request failed")
	}
	changed := lastLCPConfigureRequest(t, rec)
	if changed.Identifier == retry.Identifier || bytes.Equal(changed.Data, retry.Data) {
		t.Fatal("changed options did not receive a fresh Identifier")
	}
	before := rec.count()
	s.handleLCPPacket(LCPPacket{Code: LCPConfigureAck, Identifier: retry.Identifier, Data: retry.Data})
	if rec.count() != before || s.currentState() != LCPStateReqSent {
		t.Fatal("stale Ack advanced the current request")
	}
	s.handleLCPPacket(LCPPacket{Code: LCPConfigureAck, Identifier: changed.Identifier, Data: changed.Data})
	if s.currentState() != LCPStateAckRcvd {
		t.Fatal("matching Ack did not enter Ack-Rcvd")
	}
	if s.handleRestartTimeout() {
		t.Fatal("timeout after Ack terminated negotiation")
	}
	afterAck := lastLCPConfigureRequest(t, rec)
	if afterAck.Identifier == changed.Identifier || !bytes.Equal(afterAck.Data, changed.Data) {
		t.Fatal("new request after a valid reply did not change Identifier")
	}
}

// RFC requirement: RFC1661-5.4-5 positive -- all rejected MRU, Authentication-Protocol and Magic-Number options disappear from the next request, including when every option was rejected.
// RFC requirement: RFC1661-5.4-5 negative -- rejecting only MRU leaves the un-rejected Authentication-Protocol and Magic-Number unchanged.
func TestLCPRejectedOptionsStayRemoved(t *testing.T) {
	for _, all := range []bool{false, true} {
		s, rec, _ := newRFC1661Session(LCPStateReqSent)
		s.configuredAuthMethod = AuthMethodPAP
		if !s.sendConfigureRequest() {
			t.Fatal("initial request failed")
		}
		request := lastLCPConfigureRequest(t, rec)
		rejected := request.Data[:4]
		if all {
			rejected = request.Data
		}
		if s.handleLCPPacket(LCPPacket{Code: LCPConfigureReject, Identifier: request.Identifier, Data: rejected}) {
			t.Fatal("valid Reject terminated negotiation")
		}
		next := lastLCPConfigureRequest(t, rec)
		if next.Identifier == request.Identifier {
			t.Fatal("Reject did not trigger a new request")
		}
		if all {
			if len(next.Data) != 0 {
				t.Fatalf("rejected options remain: % x", next.Data)
			}
			if s.handleLCPPacket(LCPPacket{Code: LCPConfigureAck, Identifier: next.Identifier, Data: nil}) ||
				s.currentState() != LCPStateAckRcvd {
				t.Fatal("empty request could not be acknowledged")
			}
		} else if !bytes.Equal(next.Data, request.Data[4:]) {
			t.Fatalf("un-rejected options changed: got % x, want % x", next.Data, request.Data[4:])
		}
	}
}

// TestPAPDecisionDoesNotSurviveLCPDown prevents a cached authentication
// decision from authorizing replies after renegotiation or termination.
func TestPAPDecisionDoesNotSurviveLCPDown(t *testing.T) {
	for _, code := range []uint8{LCPConfigureRequest, LCPTerminateRequest} {
		t.Run(LCPCodeName(code), func(t *testing.T) {
			s, rec, _ := newRFC1661Session(LCPStateOpened)
			s.papReplyCode = PAPAuthenticateAck
			pap := lcpFrame(ProtoPAP, PAPAuthenticateRequest, 7, []byte{1, 'u', 1, 'p'})
			if s.handleFrame(pap) {
				t.Fatal("post-authentication PAP repeat terminated the session")
			}
			frames := decodeFrames(t, rec)
			if len(frames) != 1 || frames[0].Proto != ProtoPAP ||
				frames[0].Pkt.Code != PAPAuthenticateAck || frames[0].Pkt.Identifier != 7 {
				t.Fatal("completed authentication did not reanswer its PAP request")
			}
			var data []byte
			if code == LCPConfigureRequest {
				data = optStream(mruOption(1400), magicOption(0xaabbccdd))
			}
			if s.handleFrame(lcpFrame(ProtoLCP, code, 8, data)) {
				t.Fatal("LCP down transition prematurely terminated the session")
			}
			before := rec.count()
			s.handleFrame(pap)
			if rec.count() != before {
				t.Fatal("cached PAP decision was reused after LCP went down")
			}
		})
	}
}
