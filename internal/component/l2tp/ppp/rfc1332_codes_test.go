// Design: docs/architecture/l2tp/bng-5-pppoe.md -- RFC 1332 conformance coverage
// Related: ncp.go -- ncpCodeToEvent, sendNCPCodeReject and the IPCP option scan under test
// Related: ncp_invalid_reply_test.go -- newIPCPSession and ipcpPacket
//
// Drives two RFC 1332 rows through handleIPCPPacket, the LNS IPCP entry
// point: the Codes IPCP uses (Section 2), and the slot identifier
// compression ze never agrees to because it never negotiates Van Jacobson
// compression at all (Section 4.1).

package ppp

import (
	"bytes"
	"encoding/binary"
	"testing"
)

// ipcpVJOption is an IP-Compression-Protocol option asking for Van Jacobson
// Compressed TCP/IP (0x002d) with Max-Slot-Id 15 and Comp-Slot-Id 1, the
// value that lets the sender compress the slot identifier (RFC 1332 Section
// 4.1).
var ipcpVJOption = []byte{2, 6, 0x00, 0x2d, 15, 1}

// TestIPCPUnusedCodesAreCodeRejected sends IPCP packets carrying the four LCP
// maintenance Codes and a Code above them, and reads what ze answers.
//
// VALIDATES: RFC 1332 Section 2: "Only Codes 1 through 7 (...) are used.
// Other Codes should be treated as unrecognized and should result in
// Code-Rejects."
// PREVENTS: an NCP Protocol-Reject, Echo-Request, Echo-Reply or
// Discard-Request read as the LCP packet of the same Code, which ze once did:
// codeToEvent mapped them to RXJ+ and RXR, and the IPCP automaton dropped
// them in silence.
//
// RFC requirement: RFC1332-2-2 positive -- an IPCP packet with Code 8, 9, 10, 11 or 12 draws one IPCP Code-Reject (protocol 0x8021) whose Rejected-Packet holds that packet's Code, Identifier, Length and Data, and the IPCP state stays Req-Sent.
func TestIPCPUnusedCodesAreCodeRejected(t *testing.T) {
	for _, code := range []uint8{LCPProtocolReject, LCPEchoRequest, LCPEchoReply, LCPDiscardRequest, 12} {
		s, rec, _ := newIPCPSession(t)
		data := []byte{0xde, 0xad, 0xbe, 0xef}

		if term := s.handleIPCPPacket(ipcpPacket(code, 0x51, data)); term {
			t.Fatalf("code %d: an IPCP packet with an unused Code ended the session", code)
		}
		frames := decodeFrames(t, rec)
		if len(frames) != 2 {
			t.Fatalf("code %d: %d frames after the packet, want the initial request and one Code-Reject", code, len(frames))
		}
		reply := frames[1]
		if reply.Proto != ProtoIPCP {
			t.Fatalf("code %d: reply on protocol %#04x, want IPCP %#04x", code, reply.Proto, ProtoIPCP)
		}
		if reply.Pkt.Code != LCPCodeReject {
			t.Fatalf("code %d: reply Code = %d, want Code-Reject %d", code, reply.Pkt.Code, LCPCodeReject)
		}
		want := []byte{code, 0x51, 0, 8, 0xde, 0xad, 0xbe, 0xef}
		if !bytes.Equal(reply.Pkt.Data, want) {
			t.Fatalf("code %d: Rejected-Packet = % x, want % x", code, reply.Pkt.Data, want)
		}
		if got := s.ncpState(AddressFamilyIPv4); got != LCPStateReqSent {
			t.Fatalf("code %d: IPCP state = %s, want Req-Sent: RUC in Req-Sent is scj/6", code, got)
		}
	}
}

// TestIPCPUsedCodesAreNotCodeRejected sends an IPCP Terminate-Request and an
// IPCP Code-Reject, two of the seven Codes IPCP uses, and reads what ze
// answers.
//
// VALIDATES: the Code-Reject above is specific to the Codes IPCP does not
// use. A Code inside 1 to 7 keeps its own meaning.
// PREVENTS: an NCP code map that rejects every packet, or every packet whose
// Code is not a Configure-* Code.
//
// RFC requirement: RFC1332-2-2 negative -- an IPCP Terminate-Request (Code 5) draws a Terminate-Ack and an IPCP Code-Reject (Code 7) of an extended code draws nothing, so neither of the two Codes 1 through 7 sent is Code-Rejected.
func TestIPCPUsedCodesAreNotCodeRejected(t *testing.T) {
	s, rec, _ := newIPCPSession(t)

	if term := s.handleIPCPPacket(ipcpPacket(LCPTerminateRequest, 0x52, nil)); term {
		t.Fatal("an IPCP Terminate-Request in Req-Sent ended the session")
	}
	frames := decodeFrames(t, rec)
	if len(frames) != 2 {
		t.Fatalf("%d frames after the Terminate-Request, want the initial request and one Terminate-Ack", len(frames))
	}
	if reply := frames[1]; reply.Proto != ProtoIPCP || reply.Pkt.Code != LCPTerminateAck || reply.Pkt.Identifier != 0x52 {
		t.Fatalf("reply = %+v, want an IPCP Terminate-Ack with Identifier 0x52", reply)
	}

	rejected := []byte{12, 0x01, 0, 4}
	if term := s.handleIPCPPacket(ipcpPacket(LCPCodeReject, 0x53, rejected)); term {
		t.Fatal("an IPCP Code-Reject of an extended code ended the session")
	}
	for i, f := range decodeFrames(t, rec) {
		if f.Pkt.Code == LCPCodeReject {
			t.Fatalf("frame %d is a Code-Reject (% x): a Code IPCP uses was rejected", i, f.Pkt.Data)
		}
	}
}

// TestIPCPNeverOffersVanJacobsonCompression reads every IPCP
// Configure-Request ze sends, the first and the one a peer's Configure-Nak
// suggesting Van Jacobson compression draws.
//
// VALIDATES: RFC 1332 Section 4.1: "The slot identifier must not be
// compressed if there is no ability for the PPP link level to indicate an
// error in reception to the decompression module." ze never asks the peer to
// compress toward it, so it never asks for a compressed slot identifier.
// PREVENTS: an IPCP writer that offers IP-Compression-Protocol, or an
// absorbIPCPNak that copies a suggested option into the next request.
//
// RFC requirement: RFC1332-4.1-1 positive -- neither ze's first IPCP Configure-Request nor the one it sends after a Configure-Nak carrying IP-Compression-Protocol 0x002d with Comp-Slot-Id 1 carries an option of Type 2, so ze never asks a peer to compress the slot identifier.
func TestIPCPNeverOffersVanJacobsonCompression(t *testing.T) {
	s, rec, request := newIPCPSession(t)

	nak := append([]byte{IPCPOptIPAddress, 6, 10, 0, 0, 254}, ipcpVJOption...)
	if term := s.handleIPCPPacket(ipcpPacket(LCPConfigureNak, request.Identifier, nak)); term {
		t.Fatal("a Configure-Nak suggesting compression ended the session")
	}
	frames := decodeFrames(t, rec)
	if len(frames) != 2 {
		t.Fatalf("%d frames after the Configure-Nak, want the initial and the replacement request", len(frames))
	}
	for i, f := range frames {
		if f.Proto != ProtoIPCP || f.Pkt.Code != LCPConfigureRequest {
			t.Fatalf("frame %d = %+v, want an IPCP Configure-Request", i, f)
		}
		opts, err := ParseLCPOptions(f.Pkt.Data)
		if err != nil {
			t.Fatalf("frame %d: options do not parse: %v", i, err)
		}
		if len(opts) == 0 {
			t.Fatalf("frame %d carries no option, want at least IP-Address", i)
		}
		for _, o := range opts {
			if o.Type == 2 {
				t.Fatalf("frame %d offers IP-Compression-Protocol % x", i, o.Data)
			}
		}
	}
}

// TestIPCPRejectsVanJacobsonCompression sends a peer IPCP Configure-Request
// asking ze to send Van Jacobson compressed TCP/IP with a compressed slot
// identifier, and reads ze's answer.
//
// VALIDATES: ze never agrees to compress toward the peer, so it never
// compresses a slot identifier on a link whatever its error indication.
// PREVENTS: an IPCP option scan that recognizes Type 2 and Acks it, which
// would commit ze to a compression it does not perform.
//
// RFC requirement: RFC1332-4.1-1 negative -- a peer Configure-Request carrying IP-Compression-Protocol 0x002d with Comp-Slot-Id 1 draws a Configure-Reject whose options are exactly that option, never a Configure-Ack.
func TestIPCPRejectsVanJacobsonCompression(t *testing.T) {
	s, rec, _ := newIPCPSession(t)
	s.peerIPv4 = ipcpTestPeer

	addr := ipcpTestPeer.As4()
	data := append([]byte{IPCPOptIPAddress, 6, addr[0], addr[1], addr[2], addr[3]}, ipcpVJOption...)
	if term := s.handleIPCPPacket(ipcpPacket(LCPConfigureRequest, 0x61, data)); term {
		t.Fatal("a Configure-Request asking for compression ended the session")
	}
	frames := decodeFrames(t, rec)
	if len(frames) != 2 {
		t.Fatalf("%d frames after the Configure-Request, want the initial request and one reply", len(frames))
	}
	reply := frames[1]
	if reply.Proto != ProtoIPCP || reply.Pkt.Code != LCPConfigureReject || reply.Pkt.Identifier != 0x61 {
		t.Fatalf("reply = %+v, want an IPCP Configure-Reject with Identifier 0x61", reply)
	}
	if !bytes.Equal(reply.Pkt.Data, ipcpVJOption) {
		t.Fatalf("Configure-Reject options = % x, want only the compression option % x", reply.Pkt.Data, ipcpVJOption)
	}
	if got := binary.BigEndian.Uint16(reply.Pkt.Data[2:4]); got != 0x002d {
		t.Fatalf("rejected IP-Compression-Protocol = %#04x, want 0x002d", got)
	}
}
