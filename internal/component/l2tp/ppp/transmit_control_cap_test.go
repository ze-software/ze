// Design: docs/guide/l2tp.md -- one admitted transmit limit for IP and PPP control replies.

package ppp

import (
	"bytes"
	"encoding/binary"
	"testing"
)

// Every reject sender must honor the admitted session cap even when the peer
// advertised a larger receive capacity, or omitted the option entirely.
func TestControlRepliesRespectTransmitCap(t *testing.T) {
	for _, tc := range []struct {
		name string
		peer uint16
		cap  uint16
		want int
	}{
		{"omitted peer MRU", 0, 1492, 1492},
		{"peer 1500", 1500, 1492, 1492},
		{"peer 9000", 9000, 1492, 1492},
		{"smaller peer", 1400, 1492, 1400},
		{"IPv4 minimum", 68, 1492, 68},
		{"default cap", 1500, 1500, 1500},
	} {
		for _, kind := range []string{"lcp", "ipcp", "ipv6cp", "protocol", "echo"} {
			t.Run(tc.name+"/"+kind, func(t *testing.T) {
				s, rec, _ := newRFC1661Session(LCPStateOpened)
				s.negotiatedMRU = tc.peer
				s.maxMRU = tc.cap
				data := bytes.Repeat([]byte{0x5a}, MaxFrameLen-lcpHeaderLen)
				request := LCPPacket{Code: 99, Identifier: 0x61, Data: data}
				proto := ProtoLCP
				code := LCPCodeReject
				switch kind {
				case "lcp":
					if s.handleLCPPacket(request) {
						t.Fatal("unknown LCP Code ended a working session")
					}
				case "ipcp":
					proto = ProtoIPCP
					request.Code = 12
					s.setNCPState(AddressFamilyIPv4, LCPStateReqSent)
					if s.handleIPCPPacket(request) {
						t.Fatal("unknown IPCP Code ended a working session")
					}
				case "ipv6cp":
					proto = ProtoIPv6CP
					request.Code = 12
					s.setNCPState(AddressFamilyIPv6, LCPStateReqSent)
					if s.handleIPv6CPPacket(request) {
						t.Fatal("unknown IPv6CP Code ended a working session")
					}
				case "protocol":
					code = LCPProtocolReject
					if !s.sendProtocolReject(protoMPLSCP, data) {
						t.Fatal("Protocol-Reject failed")
					}
				case "echo":
					code = LCPEchoReply
					request.Code = LCPEchoRequest
					binary.BigEndian.PutUint32(data[:4], s.peerMagic)
					if s.handleLCPPacket(request) {
						t.Fatal("Echo-Request ended a working session")
					}
				}
				frames := rec.all()
				if len(frames) != 1 {
					t.Fatalf("replies = %d, want one useful control reply", len(frames))
				}
				if len(frames[0]) != frameLen(tc.want) {
					t.Fatalf("reply frame has %d octets, want %d for effective MRU %d", len(frames[0]), frameLen(tc.want), tc.want)
				}
				got := decodeFrames(t, rec)[0]
				if got.Proto != proto || got.Pkt.Code != code {
					t.Fatalf("reply protocol/Code = %#x/%d, want %#x/%d", got.Proto, got.Pkt.Code, proto, code)
				}
				switch code {
				case LCPCodeReject:
					if len(got.Pkt.Data) < lcpHeaderLen || got.Pkt.Data[0] != request.Code || got.Pkt.Data[1] != request.Identifier {
						t.Fatal("Code-Reject lost the rejected packet's header")
					}
					if binary.BigEndian.Uint16(got.Pkt.Data[2:4]) != MaxFrameLen {
						t.Fatal("Code-Reject changed the rejected packet's original Length")
					}
					if !bytes.Equal(got.Pkt.Data[4:], data[:len(got.Pkt.Data)-4]) {
						t.Fatal("Code-Reject did not retain the head of the rejected Data")
					}
				case LCPProtocolReject:
					if rejectedProtocol(t, got.Pkt) != protoMPLSCP || !bytes.Equal(got.Pkt.Data[2:], data[:len(got.Pkt.Data)-2]) {
						t.Fatal("Protocol-Reject lost the protocol or leading Information field")
					}
				default:
					if got.Pkt.Identifier != request.Identifier || binary.BigEndian.Uint32(got.Pkt.Data[:4]) != s.magic {
						t.Fatal("Echo-Reply lost the request Identifier or local Magic-Number")
					}
					if !bytes.Equal(got.Pkt.Data[4:], data[4:len(got.Pkt.Data)]) {
						t.Fatal("Echo-Reply did not preserve the fitting Data prefix")
					}
				}
				if s.negotiatedMRU != tc.peer {
					t.Fatal("transmit cap replaced the negotiated peer MRU")
				}
			})
		}
	}
}

func TestControlReplyFittingDataIsPreserved(t *testing.T) {
	s, rec, _ := newRFC1661Session(LCPStateOpened)
	s.maxMRU = 1492
	data := []byte{1, 2, 3, 4, 5}
	if !s.sendProtocolReject(protoMPLSCP, data) {
		t.Fatal("Protocol-Reject failed")
	}
	if !s.sendCodeReject(LCPPacket{Code: 99, Identifier: 0x61, Data: data}) {
		t.Fatal("Code-Reject failed")
	}
	request := LCPPacket{Code: LCPEchoRequest, Identifier: 0x62, Data: append([]byte{0, 0, 0, 0}, data...)}
	if !s.sendEchoReply(request) {
		t.Fatal("Echo-Reply failed")
	}
	got := decodeFrames(t, rec)
	if len(got) != 3 {
		t.Fatalf("replies = %d, want three", len(got))
	}
	if !bytes.Equal(got[0].Pkt.Data[2:], data) || !bytes.Equal(got[1].Pkt.Data[4:], data) || !bytes.Equal(got[2].Pkt.Data[4:], data) {
		t.Fatal("a fitting control reply was truncated or changed")
	}
}
