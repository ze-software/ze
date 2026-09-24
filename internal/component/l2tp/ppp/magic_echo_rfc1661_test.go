// Design: docs/architecture/l2tp/bng-5-pppoe.md -- RFC 1661 conformance coverage
//
// Proves the RFC 1661 Section 5.5, 5.8, 6.1 and 6.4 obligations on the
// Identifier and Magic-Number fields of the Terminate and Echo packets, and
// the 1500-octet Information field ze must receive whatever MRU it asked for.

package ppp

import (
	"encoding/binary"
	"testing"
)

// lcpOptionsPacket builds an LCP packet of the given code whose Data field
// carries the options, the shape of a Configure-Ack or Configure-Nak.
func lcpOptionsPacket(t *testing.T, code, id uint8, opts []LCPOption) LCPPacket {
	t.Helper()
	buf := make([]byte, MaxFrameLen)
	n, fits := WriteLCPOptions(buf, 0, opts)
	if !fits {
		t.Fatal("options do not fit a frame")
	}
	return LCPPacket{Code: code, Identifier: id, Data: buf[:n]}
}

// echoMagic reads the Magic-Number field of a recorded Echo packet.
func echoMagic(t *testing.T, pkt LCPPacket) uint32 {
	t.Helper()
	m, err := parseLCPEchoMagic(pkt.Data)
	if err != nil {
		t.Fatalf("Echo packet without a Magic-Number field: %v", err)
	}
	return m
}

// TestTerminateRequestIdentifierChanges sends two Terminate-Requests from one
// session and reads the Identifier each carries.
//
// RFC requirement: RFC1661-5.5-2 positive — the second Terminate-Request sendTerminateRequest emits carries an Identifier different from the first one.
// RFC requirement: RFC1661-5.5-2 negative — no two Terminate-Requests sendTerminateRequest emits in a row repeat one Identifier.
func TestTerminateRequestIdentifierChanges(t *testing.T) {
	t.Parallel()

	s, rec, _ := newRFC1661Session(LCPStateClosing)
	for range 2 {
		if !s.sendTerminateRequest() {
			t.Fatal("sendTerminateRequest reported a write failure")
		}
	}
	frames := decodeFrames(t, rec)
	if len(frames) != 2 {
		t.Fatalf("recorded %d frames, want 2 Terminate-Requests", len(frames))
	}
	for i, f := range frames {
		if f.Pkt.Code != LCPTerminateRequest {
			t.Fatalf("frame %d code = %s, want Terminate-Request", i, LCPCodeName(f.Pkt.Code))
		}
	}
	first, second := frames[0].Pkt.Identifier, frames[1].Pkt.Identifier
	if first == second {
		t.Fatalf("both Terminate-Requests carry Identifier %#x; Section 5.5 requires a change", first)
	}
}

// TestEchoRequestIdentifierChanges sends two Echo-Requests from one session
// and reads the Identifier each carries, then reads the Identifier an
// Echo-Reply copies from its request.
//
// RFC requirement: RFC1661-5.8-4 positive — the second Echo-Request sendEchoRequest emits carries an Identifier different from the first one, and sendEchoReply copies the request's Identifier into the reply.
// RFC requirement: RFC1661-5.8-4 negative — no two Echo-Requests sendEchoRequest emits in a row repeat one Identifier.
func TestEchoRequestIdentifierChanges(t *testing.T) {
	t.Parallel()

	s, rec, _ := newRFC1661Session(LCPStateOpened)
	for range 2 {
		if !s.sendEchoRequest() {
			t.Fatal("sendEchoRequest reported a write failure")
		}
	}
	frames := decodeFrames(t, rec)
	if len(frames) != 2 {
		t.Fatalf("recorded %d frames, want 2 Echo-Requests", len(frames))
	}
	for i, f := range frames {
		if f.Pkt.Code != LCPEchoRequest {
			t.Fatalf("frame %d code = %s, want Echo-Request", i, LCPCodeName(f.Pkt.Code))
		}
	}
	first, second := frames[0].Pkt.Identifier, frames[1].Pkt.Identifier
	if first == second {
		t.Fatalf("both Echo-Requests carry Identifier %#x; Section 5.8 requires a change", first)
	}

	req := LCPPacket{Code: LCPEchoRequest, Identifier: 0x7E, Data: []byte{0, 0, 0, 0}}
	if !s.sendEchoReply(req) {
		t.Fatal("sendEchoReply reported a write failure")
	}
	frames = decodeFrames(t, rec)
	reply := frames[len(frames)-1].Pkt
	if reply.Code != LCPEchoReply || reply.Identifier != 0x7E {
		t.Fatalf("reply = %s/%#x, want Echo-Reply/0x7e", LCPCodeName(reply.Code), reply.Identifier)
	}
}

// TestMagicNumberZeroUntilNegotiated reads the Magic-Number field of the
// Echo-Request and Echo-Reply ze sends before the peer acknowledges the
// option, after a Configure-Ack that carries it, and after one that does not.
//
// RFC requirement: RFC1661-5.8-5 positive — before any Configure-Ack, Echo-Request and Echo-Reply carry Magic-Number zero; after a Configure-Ack carrying option 5 with ze's value, both carry that value.
// RFC requirement: RFC1661-5.8-5 negative — a Configure-Ack that carries no Magic-Number option leaves the Echo packets' Magic-Number at zero, so ze's chosen value never reaches the wire unnegotiated.
func TestMagicNumberZeroUntilNegotiated(t *testing.T) {
	t.Parallel()

	const chosen uint32 = 0x0BADCAFE
	newSession := func() (*pppSession, *frameRecorder) {
		s, rec, _ := newRFC1661Session(LCPStateReqSent)
		s.magic = chosen
		s.magicNegotiated = false
		if !s.sendConfigureRequest() {
			t.Fatal("sendConfigureRequest reported a write failure")
		}
		return s, rec
	}
	readMagics := func(t *testing.T, s *pppSession, rec *frameRecorder) (request, reply uint32) {
		t.Helper()
		before := rec.count()
		if !s.sendEchoRequest() {
			t.Fatal("sendEchoRequest reported a write failure")
		}
		if !s.sendEchoReply(LCPPacket{Code: LCPEchoRequest, Identifier: 1, Data: []byte{0, 0, 0, 0}}) {
			t.Fatal("sendEchoReply reported a write failure")
		}
		frames := decodeFrames(t, rec)[before:]
		if len(frames) != 2 {
			t.Fatalf("recorded %d frames, want an Echo-Request and an Echo-Reply", len(frames))
		}
		return echoMagic(t, frames[0].Pkt), echoMagic(t, frames[1].Pkt)
	}

	s, rec := newSession()
	if req, rep := readMagics(t, s, rec); req != 0 || rep != 0 {
		t.Fatalf("unnegotiated Magic-Number transmitted as %#x/%#x, want zero", req, rep)
	}

	ack := lcpOptionsPacket(t, LCPConfigureAck, 1, BuildLocalConfigRequest(LCPOptions{MRU: MaxFrameLen, Magic: chosen}))
	if term := s.handleLCPPacket(ack); term {
		t.Fatal("session terminated on a Configure-Ack")
	}
	if req, rep := readMagics(t, s, rec); req != chosen || rep != chosen {
		t.Fatalf("negotiated Magic-Number transmitted as %#x/%#x, want %#x", req, rep, chosen)
	}

	s, rec = newSession()
	bare := lcpOptionsPacket(t, LCPConfigureAck, 1, BuildLocalConfigRequest(LCPOptions{MRU: MaxFrameLen}))
	if term := s.handleLCPPacket(bare); term {
		t.Fatal("session terminated on a Configure-Ack")
	}
	if req, rep := readMagics(t, s, rec); req != 0 || rep != 0 {
		t.Fatalf("Magic-Number transmitted as %#x/%#x after an Ack without option 5, want zero", req, rep)
	}
}

// TestMagicNumberRedrawnOnNak feeds a session a Configure-Nak that names its
// Magic-Number and one that does not, and reads the value the session holds
// after each.
//
// RFC requirement: RFC1661-6.4-7 positive — a Configure-Nak carrying a Magic-Number option makes redrawMagicOnNak replace the session's Magic-Number with a new non-zero value.
// RFC requirement: RFC1661-6.4-7 negative — a Configure-Nak carrying no Magic-Number option leaves the session's Magic-Number unchanged.
func TestMagicNumberRedrawnOnNak(t *testing.T) {
	t.Parallel()

	const chosen uint32 = 0x13572468
	s, _, _ := newRFC1661Session(LCPStateReqSent)
	s.magic = chosen
	if !s.sendConfigureRequest() {
		t.Fatal("initial Configure-Request failed")
	}

	mru := make([]byte, 2)
	binary.BigEndian.PutUint16(mru, 1400)
	mruNak := lcpOptionsPacket(t, LCPConfigureNak, 1, []LCPOption{{Type: LCPOptMRU, Data: mru}})
	if term := s.handleLCPPacket(mruNak); term {
		t.Fatal("session terminated on a Configure-Nak")
	}
	if s.magic != chosen {
		t.Fatalf("Magic-Number changed to %#x on a Nak that named only the MRU, want %#x kept", s.magic, chosen)
	}

	magicNak := lcpOptionsPacket(t, LCPConfigureNak, 2, []LCPOption{magicOption(chosen)})
	if term := s.handleLCPPacket(magicNak); term {
		t.Fatal("session terminated on a Configure-Nak")
	}
	if s.magic == chosen {
		t.Fatal("Magic-Number kept after a Configure-Nak named it; Section 6.4 requires a new one")
	}
	if s.magic == 0 {
		t.Fatal("Magic-Number redrawn as zero, which Section 6.4 makes illegal")
	}
}

// TestReceivedMagicNumberMustBePeers feeds an Opened session Echo packets
// whose Magic-Number is the peer's negotiated value, zero from a peer that
// negotiated one, and a foreign value, and reads what each earns.
//
// RFC requirement: RFC1661-6.4-8 positive — an Echo-Request whose Magic-Number equals the peer's negotiated value draws an Echo-Reply, and an Echo-Reply carrying it clears the outstanding echo count.
// RFC requirement: RFC1661-6.4-8 negative — an Echo-Request whose Magic-Number is zero or a value other than the peer's negotiated one draws no reply, and an Echo-Reply carrying such a value leaves the outstanding echo count untouched.
func TestReceivedMagicNumberMustBePeers(t *testing.T) {
	t.Parallel()

	const peer uint32 = 0x2468ACE0
	s, rec, _ := newRFC1661Session(LCPStateOpened)
	s.peerMagic = peer

	magic := func(v uint32) []byte {
		d := make([]byte, 4)
		binary.BigEndian.PutUint32(d, v)
		return d
	}

	for _, wrong := range []uint32{0, peer + 1, s.magic} {
		before := rec.count()
		if term := s.handleLCPPacket(LCPPacket{Code: LCPEchoRequest, Identifier: 5, Data: magic(wrong)}); term {
			t.Fatal("session terminated on an Echo-Request")
		}
		if rec.count() != before {
			t.Fatalf("Echo-Request with Magic-Number %#x was answered; the peer's is %#x", wrong, peer)
		}
		s.echoOutstanding = 2
		if term := s.handleLCPPacket(LCPPacket{Code: LCPEchoReply, Identifier: 5, Data: magic(wrong)}); term {
			t.Fatal("session terminated on an Echo-Reply")
		}
		if s.echoOutstanding != 2 {
			t.Fatalf("Echo-Reply with Magic-Number %#x counted as a reply; the peer's is %#x", wrong, peer)
		}
	}

	if term := s.handleLCPPacket(LCPPacket{Code: LCPEchoRequest, Identifier: 6, Data: magic(peer)}); term {
		t.Fatal("session terminated on an Echo-Request")
	}
	reply, ok := findCode(t, rec, LCPEchoReply)
	if !ok || reply.Identifier != 6 {
		t.Fatalf("Echo-Request with the peer's Magic-Number drew no Echo-Reply (found=%v id=%#x)", ok, reply.Identifier)
	}
	s.echoOutstanding = 2
	if term := s.handleLCPPacket(LCPPacket{Code: LCPEchoReply, Identifier: 6, Data: magic(peer)}); term {
		t.Fatal("session terminated on an Echo-Reply")
	}
	if s.echoOutstanding != 0 {
		t.Fatalf("echoOutstanding = %d after an Echo-Reply with the peer's Magic-Number, want 0", s.echoOutstanding)
	}
}

// TestFullInformationFieldReceived hands ParseFrame a frame whose Information
// field is the full 1500 octets behind the Protocol field, one octet more,
// and an LCP packet whose Length is 1500.
//
// RFC requirement: RFC1661-6.1-1 positive — ParseFrame accepts a 1502-octet frame and returns its 1500-octet Information field, the read buffer holds that frame, and ParseLCPPacket accepts a Length of 1500.
// RFC requirement: RFC1661-6.1-1 negative — ParseFrame refuses a frame whose Information field is 1501 octets with errFrameTooLong.
func TestFullInformationFieldReceived(t *testing.T) {
	t.Parallel()

	frame := make([]byte, ProtoFieldLen+1500)
	binary.BigEndian.PutUint16(frame, ProtoLCP)
	frame[2] = LCPEchoRequest
	frame[3] = 9
	binary.BigEndian.PutUint16(frame[4:], 1500)
	proto, payload, _, err := ParseFrame(frame)
	if err != nil {
		t.Fatalf("ParseFrame refused a 1500-octet Information field: %v", err)
	}
	if proto != ProtoLCP || len(payload) != 1500 {
		t.Fatalf("ParseFrame returned proto %#x with %d octets, want LCP with 1500", proto, len(payload))
	}
	buf := getFrameBuf()
	defer putFrameBuf(buf)
	if len(buf) < len(frame) {
		t.Fatalf("read buffer holds %d octets, want at least %d", len(buf), len(frame))
	}
	pkt, err := ParseLCPPacket(payload)
	if err != nil {
		t.Fatalf("ParseLCPPacket refused Length 1500: %v", err)
	}
	if len(pkt.Data) != 1500-lcpHeaderLen {
		t.Fatalf("LCP Data = %d octets, want %d", len(pkt.Data), 1500-lcpHeaderLen)
	}

	over := make([]byte, ProtoFieldLen+1501)
	if _, _, _, err := ParseFrame(over); err != errFrameTooLong {
		t.Fatalf("ParseFrame(1501-octet Information field) err = %v, want errFrameTooLong", err)
	}
}

// TestMagicNumberRequiresMatchingAck keeps Echo Magic-Number at zero when
// a reply has the wrong Identifier or changes any requested option.
func TestMagicNumberRequiresMatchingAck(t *testing.T) {
	for _, alter := range []struct {
		name string
		edit func(*LCPPacket)
	}{
		{"identifier", func(pkt *LCPPacket) { pkt.Identifier++ }},
		{"mru", func(pkt *LCPPacket) { pkt.Data[3] ^= 1 }},
		{"trailing option", func(pkt *LCPPacket) { pkt.Data = append(pkt.Data, LCPOptPFC, 2) }},
	} {
		t.Run(alter.name, func(t *testing.T) {
			s, rec, _ := newRFC1661Session(LCPStateReqSent)
			if !s.sendConfigureRequest() {
				t.Fatal("sendConfigureRequest failed")
			}
			request := decodeFrames(t, rec)[0].Pkt
			ack := LCPPacket{
				Code: LCPConfigureAck, Identifier: request.Identifier,
				Data: append([]byte(nil), request.Data...),
			}
			alter.edit(&ack)
			if s.handleLCPPacket(ack) {
				t.Fatal("session terminated on an unrelated Configure-Ack")
			}
			if !s.sendEchoRequest() {
				t.Fatal("sendEchoRequest failed")
			}
			frames := decodeFrames(t, rec)
			if got := echoMagic(t, frames[len(frames)-1].Pkt); got != 0 {
				t.Fatalf("unrelated Configure-Ack authorized Magic-Number %#x", got)
			}
		})
	}
}
