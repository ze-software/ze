// VALIDATES: RFC 1661 Section 5 packet Length handling on the LCP receive
// path -- a packet whose Length is below the four-octet header or reaches
// past the Information field is dropped with no reply, no state change and no
// lifecycle event, while octets past a valid Length are padding and the packet
// is still answered.
// PREVENTS: acting on a packet whose extent ze cannot trust, and the opposite
// error of treating link-layer padding as an invalid Length.
//
// RFC: rfc/short/rfc1661.md
package ppp

import (
	"testing"
)

// lcpEchoRequestBody returns an Echo-Request LCP packet, Identifier id, whose
// Length field holds length and whose Data is the Magic-Number magic followed
// by pad octets of padding. The Length field is written as given, so a caller
// can declare an extent the octets do not match.
func lcpEchoRequestBody(id uint8, length uint16, magic [4]byte, pad int) []byte {
	body := []byte{LCPEchoRequest, id, byte(length >> 8), byte(length)}
	body = append(body, magic[:]...)
	for range pad {
		body = append(body, 0)
	}
	return body
}

// VALIDATES: an LCP packet received with an invalid Length field is dropped
// without a reply, without a state change and without a lifecycle event.
// METHOD: an Opened session whose peer Magic-Number is set receives, through
// handleFrame, an Echo-Request that would draw an Echo-Reply, once with Length
// 3 (below the header) and once with Length 16 (past the eight octets that
// follow the Protocol field).
//
// RFC requirement: RFC1661-5-2 positive -- an Echo-Request whose Length is
// below the header or exceeds the Information field draws no frame, leaves
// LCP in Opened and emits no lifecycle event.
func TestRFC1661InvalidLengthPacketSilentlyDiscarded(t *testing.T) {
	magic := [4]byte{0x99, 0x88, 0x77, 0x66}
	cases := []struct {
		name   string
		length uint16
	}{
		{"length below header", 3},
		{"length past information field", 16},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			s, rec, events := newRFC1661Session(LCPStateOpened)
			s.peerMagic = 0x99887766
			buf := make([]byte, MaxFrameLen)
			off := WriteFrame(buf, 0, ProtoLCP, lcpEchoRequestBody(0x61, tc.length, magic, 0))
			if term := s.handleFrame(buf[:off]); term {
				t.Fatal("session terminated on a packet with an invalid Length")
			}
			if n := rec.count(); n != 0 {
				t.Fatalf("wrote %d frames in reply to a packet with an invalid Length, want 0", n)
			}
			if got := s.currentState(); got != LCPStateOpened {
				t.Fatalf("state = %s, want opened", got)
			}
			select {
			case ev := <-events:
				t.Fatalf("unexpected lifecycle event %T on a packet with an invalid Length", ev)
			default:
			}
		})
	}
}

// VALIDATES: the discard is confined to an invalid Length: octets after a
// valid Length are padding, and the packet they trail is answered.
// METHOD: the same Opened session receives an Echo-Request whose Length 8
// covers the header and the Magic-Number, followed by four padding octets.
//
// RFC requirement: RFC1661-5-2 negative -- an Echo-Request with a valid Length
// and trailing padding is not discarded: it draws an Echo-Reply with the
// request's Identifier, and LCP stays in Opened.
func TestRFC1661ValidLengthWithPaddingIsAnswered(t *testing.T) {
	s, rec, _ := newRFC1661Session(LCPStateOpened)
	s.peerMagic = 0x99887766
	buf := make([]byte, MaxFrameLen)
	off := WriteFrame(buf, 0, ProtoLCP, lcpEchoRequestBody(0x62, 8, [4]byte{0x99, 0x88, 0x77, 0x66}, 4))
	if term := s.handleFrame(buf[:off]); term {
		t.Fatal("session terminated on a padded Echo-Request")
	}
	pkt, ok := findCode(t, rec, LCPEchoReply)
	if !ok {
		t.Fatalf("no Echo-Reply to a padded Echo-Request with a valid Length; frames=%d", rec.count())
	}
	if pkt.Identifier != 0x62 {
		t.Fatalf("Echo-Reply Identifier = 0x%02x, want 0x62", pkt.Identifier)
	}
	if got := s.currentState(); got != LCPStateOpened {
		t.Fatalf("state = %s, want opened", got)
	}
}
