// Design: internal/component/l2tp/ppp/session_run.go -- the Code-Reject writer
// Related: ncp.go -- sendNCPCodeReject and nextNCPCodeRejectID
// Related: rfc1661_test.go -- newRFC1661Session, decodeFrames and the frame recorder
// Related: rfc/short/rfc1661.md -- RFC1661-5.6-2 and RFC1661-5.6-3
//
// Drives the LNS Code-Reject through its real entry points, handleLCPPacket
// for LCP and handleIPCPPacket and handleIPv6CPPacket for the NCPs, and
// reads the Identifier and the Rejected-Packet of what ze writes.

package ppp

import (
	"bytes"
	"encoding/binary"
	"testing"
)

// unknownLCPCode is a Code RFC 1661 Section 5 does not assign, so LCP
// answers it with a Code-Reject.
const unknownLCPCode uint8 = 99

// unusedNCPCode is a Code above the seven RFC 1332 Section 2 and RFC 5072
// Section 3 let IPCP and IPv6CP use, so either NCP answers it with a
// Code-Reject.
const unusedNCPCode uint8 = 12

// codeRejects returns the Code-Rejects recorded on proto, in send order.
func codeRejects(t *testing.T, rec *frameRecorder, proto uint16) []LCPPacket {
	t.Helper()
	var out []LCPPacket
	for _, f := range decodeFrames(t, rec) {
		if f.Proto == proto && f.Pkt.Code == LCPCodeReject {
			out = append(out, f.Pkt)
		}
	}
	return out
}

// TestRFC1661LCPCodeRejectIdentifierChanges sends LCP two packets with the
// same unknown Code and the same Identifier, and compares the Identifiers of
// the two Code-Rejects ze writes.
//
// VALIDATES: each LCP Code-Reject carries an Identifier the last one did not.
// PREVENTS: the copy of the rejected packet's Identifier that stood in
// sendCodeReject, which gave two rejected packets with one Identifier two
// Code-Rejects with one Identifier.
//
// RFC requirement: RFC1661-5.6-2 positive -- two LCP packets of Code 99 with Identifier 0x41, received in Opened, draw two LCP Code-Rejects whose Identifiers differ.
func TestRFC1661LCPCodeRejectIdentifierChanges(t *testing.T) {
	s, rec, _ := newRFC1661Session(LCPStateOpened)
	for range 2 {
		if term := s.handleLCPPacket(LCPPacket{Code: unknownLCPCode, Identifier: 0x41, Data: []byte{0xaa, 0xbb}}); term {
			t.Fatal("session terminated on an unknown Code")
		}
	}

	got := codeRejects(t, rec, ProtoLCP)
	if len(got) != 2 {
		t.Fatalf("recorded %d LCP Code-Rejects, want 2", len(got))
	}
	if got[0].Identifier == got[1].Identifier {
		t.Fatalf("both Code-Rejects carry Identifier %d; the Identifier MUST change for each one",
			got[0].Identifier)
	}
}

// TestRFC1661NCPCodeRejectIdentifierChanges sends IPCP, then IPv6CP, two
// packets with Code 12 and the same Identifier, and compares the Identifiers
// of the two Code-Rejects each NCP writes.
//
// VALIDATES: each IPCP and each IPv6CP Code-Reject carries an Identifier the
// last one on that protocol did not.
// PREVENTS: the copy of the rejected packet's Identifier that stood in
// sendNCPCodeReject.
//
// RFC requirement: RFC1661-5.6-2 positive -- two IPCP packets, and two IPv6CP packets, of Code 12 with Identifier 0x51, received in Req-Sent, draw two Code-Rejects on that NCP's protocol whose Identifiers differ.
func TestRFC1661NCPCodeRejectIdentifierChanges(t *testing.T) {
	cases := []struct {
		name   string
		family AddressFamily
		proto  uint16
	}{
		{"ipcp", AddressFamilyIPv4, ProtoIPCP},
		{"ipv6cp", AddressFamilyIPv6, ProtoIPv6CP},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			s, rec, _ := newRFC1661Session(LCPStateOpened)
			s.setNCPState(tc.family, LCPStateReqSent)
			handle := s.handleIPCPPacket
			if tc.family == AddressFamilyIPv6 {
				handle = s.handleIPv6CPPacket
			}
			for range 2 {
				if term := handle(LCPPacket{Code: unusedNCPCode, Identifier: 0x51, Data: []byte{0xde, 0xad}}); term {
					t.Fatal("session terminated on an unused NCP Code")
				}
			}

			got := codeRejects(t, rec, tc.proto)
			if len(got) != 2 {
				t.Fatalf("recorded %d Code-Rejects on %#04x, want 2", len(got), tc.proto)
			}
			if got[0].Identifier == got[1].Identifier {
				t.Fatalf("both Code-Rejects carry Identifier %d; the Identifier MUST change for each one",
					got[0].Identifier)
			}
		})
	}
}

// TestRFC1661CodeRejectTruncatedToMRU sends LCP, then IPCP, a 404-octet
// packet with an unknown Code on a session whose peer negotiated a
// 100-octet MRU, and measures the Code-Reject.
//
// VALIDATES: the Rejected-Packet is cut so the Code-Reject's Information
// field is the peer's MRU exactly, its head is the rejected packet's own
// header and the start of its Data, and its Length still says 404.
// PREVENTS: a copy bounded only by the 1500-octet frame buffer, which sends
// a peer with a 100-octet MRU a frame it MUST discard.
//
// RFC requirement: RFC1661-5.6-3 positive -- with negotiatedMRU 100, a 404-octet LCP packet of Code 99 and a 404-octet IPCP packet of Code 12 each draw a Code-Reject whose Information field is 100 octets: the 4-octet Code-Reject header, the rejected packet's Code, Identifier and Length 404, and the first 92 octets of its Data.
func TestRFC1661CodeRejectTruncatedToMRU(t *testing.T) {
	const peerMRU = 100
	data := bytes.Repeat([]byte{0x5a}, 400)
	cases := []struct {
		name  string
		code  uint8
		proto uint16
	}{
		{"lcp", unknownLCPCode, ProtoLCP},
		{"ipcp", unusedNCPCode, ProtoIPCP},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			s, rec, _ := newRFC1661Session(LCPStateOpened)
			s.negotiatedMRU = peerMRU
			s.setNCPState(AddressFamilyIPv4, LCPStateReqSent)
			handle := s.handleLCPPacket
			if tc.proto == ProtoIPCP {
				handle = s.handleIPCPPacket
			}
			if term := handle(LCPPacket{Code: tc.code, Identifier: 0x61, Data: data}); term {
				t.Fatal("session terminated on an unknown Code")
			}

			frames := rec.all()
			if len(frames) != 1 {
				t.Fatalf("recorded %d frames, want one Code-Reject", len(frames))
			}
			if len(frames[0]) != frameLen(peerMRU) {
				t.Fatalf("the answer is %d octets, want %d: the Information field MUST fit the peer's MRU of %d",
					len(frames[0]), frameLen(peerMRU), peerMRU)
			}
			got := codeRejects(t, rec, tc.proto)
			if len(got) != 1 {
				t.Fatalf("recorded %d Code-Rejects on %#04x, want 1", len(got), tc.proto)
			}
			rejected := got[0].Data
			if rejected[0] != tc.code || rejected[1] != 0x61 {
				t.Fatalf("Rejected-Packet header = % x, want Code %d and Identifier 0x61", rejected[:2], tc.code)
			}
			if length := binary.BigEndian.Uint16(rejected[2:4]); length != 404 {
				t.Fatalf("Rejected-Packet Length = %d, want the original 404", length)
			}
			kept := rejected[lcpHeaderLen:]
			if want := peerMRU - 2*lcpHeaderLen; len(kept) != want {
				t.Fatalf("Rejected-Packet Data is %d octets, want %d", len(kept), want)
			}
			if !bytes.Equal(kept, data[:len(kept)]) {
				t.Fatal("the Rejected-Packet Data kept is not the head of the packet rejected")
			}
		})
	}
}

// TestRFC1661CodeRejectKeptWhenItFits sends LCP a 44-octet packet with an
// unknown Code on a session whose peer negotiated a 100-octet MRU.
//
// VALIDATES: a rejected packet that already fits the peer's MRU goes back
// whole.
// PREVENTS: a cut applied unconditionally, which leaves the peer unable to
// tell which packet ze refused.
//
// RFC requirement: RFC1661-5.6-3 negative -- with negotiatedMRU 100, a 44-octet LCP packet of Code 99 draws a Code-Reject whose Rejected-Packet is that whole packet: its Code, Identifier, Length 44 and all 40 octets of its Data.
func TestRFC1661CodeRejectKeptWhenItFits(t *testing.T) {
	s, rec, _ := newRFC1661Session(LCPStateOpened)
	s.negotiatedMRU = 100
	data := bytes.Repeat([]byte{0x33}, 40)
	if term := s.handleLCPPacket(LCPPacket{Code: unknownLCPCode, Identifier: 0x62, Data: data}); term {
		t.Fatal("session terminated on an unknown Code")
	}

	got := codeRejects(t, rec, ProtoLCP)
	if len(got) != 1 {
		t.Fatalf("recorded %d LCP Code-Rejects, want 1", len(got))
	}
	want := append([]byte{unknownLCPCode, 0x62, 0, 44}, data...)
	if !bytes.Equal(got[0].Data, want) {
		t.Fatalf("Rejected-Packet is %d octets, want the whole %d-octet packet back", len(got[0].Data), len(want))
	}
}
