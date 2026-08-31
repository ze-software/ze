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
//	length, or above MaxFrameLen-2.
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

// VALIDATES: a Configure-Request whose reply would not fit a PPP frame draws
//
//	no reply at all, and leaves ze running with its automaton usable.
//
// PREVENTS: the daemon panicking on a packet any unauthenticated peer can
// send. A Magic-Number received at Length 2 is answered by the six octets RFC
// 1661 Section 6.4 gives the option, so a frame filled with them asks for a
// Configure-Nak three times the size of the request. WriteLCPOptions wrote
// every one of those octets into a MaxFrameLen buffer with no bound, and the
// index past the end of it took the session goroutine down.
func TestLCPReplyLargerThanAFrameIsNotSent(t *testing.T) {
	s, rec, _ := newRFC1661Session(LCPStateReqSent)

	// 700 Magic-Number options at Length 2, 1400 octets, inside a frame. Each
	// one is well formed as a header and invalid for its Type, so each earns
	// a six-octet Nak entry.
	const magicOptions = 700
	request := make([]byte, 0, magicOptions*2)
	for range magicOptions {
		request = append(request, LCPOptMagic, 2)
	}

	if term := s.handleFrame(lcpReqFrame(0xD1, request)); term {
		t.Fatal("session terminated on a Configure-Request whose reply does not fit a frame")
	}

	// Ze answered nothing. RFC 1661 Section 5.3: "The Options field is
	// filled with only the unacceptable Configuration Options from the
	// Configure-Request." A Configure-Nak carrying the prefix of them that
	// fits is a packet that section does not describe, so the reply ze owes
	// and cannot send is no reply at all, and sendConfigureNakOrReject
	// returns before it writes a frame.
	//
	// The parse below is the counterfactual: if a frame ever does carry this
	// Identifier, it is a whole option list, because the prefix that fit
	// would not parse.
	for _, frame := range rec.all() {
		proto, payload, _, err := ParseFrame(frame)
		if err != nil {
			t.Fatalf("ParseFrame(% x): %v", frame, err)
		}
		if proto != ProtoLCP {
			continue
		}
		pkt, err := ParseLCPPacket(payload)
		if err != nil {
			t.Fatalf("ParseLCPPacket(% x): %v", payload, err)
		}
		if pkt.Identifier != 0xD1 {
			continue
		}
		t.Errorf("ze answered the oversized request with %s carrying %d octets of options; RFC 1661 Section 5.3 has no reply that carries some of the unacceptable options",
			LCPCodeName(pkt.Code), len(pkt.Data))
		if _, err := ParseLCPOptions(pkt.Data); err != nil {
			t.Errorf("that reply's option list does not parse: % x: %v", pkt.Data, err)
		}
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
