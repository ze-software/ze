package bmp

import (
	"encoding/binary"
	"testing"
)

// wireCase is one BMP message as the sender's own writer puts it on the wire.
type wireCase struct {
	name     string
	msgType  uint8
	perPeer  bool
	write    func(buf []byte) int
	bodyWant int // octets after the common header and, when present, the per-peer header
}

// rfc7854WireCases returns one case for each of the seven message types, each
// built through the writer the sender calls. The per-peer header of every case
// carries a 4-octet AS so its width is visible on the wire.
func rfc7854WireCases() []wireCase {
	peer := testPeerHeader()
	peer.PeerAS = 4200000001
	update := []byte{0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0x00, 0x17, 0x02, 0x00, 0x00, 0x00, 0x00}
	sentOpen := makeBGPOpen(65001, 0x01020304)
	recvOpen := makeBGPOpen(65002, 0x05060708)
	sysDescr := makeStringTLV(InitTLVSysDescr, "ze")
	sysName := makeStringTLV(InitTLVSysName, "r1")
	reason := TLV{Type: TermTLVReason, Length: 2, Value: []byte{0x00, 0x01}}
	mirrored := TLV{Type: MirrorTLVBGPMsg, Length: uint16(len(update)), Value: update}
	return []wireCase{
		{"route-monitoring", MsgRouteMonitoring, true, func(b []byte) int {
			return writeRouteMonitoring(b, 0, &RouteMonitoring{Peer: peer, BGPUpdate: update})
		}, len(update)},
		{"statistics-report", MsgStatisticsReport, true, func(b []byte) int {
			return writeStatisticsReport(b, 0, &statisticsReport{Peer: peer, Stats: []StatEntry{{Type: 0, Value: []byte{0, 0, 0, 7}}}})
		}, 4 + TLVHeaderSize + 4},
		{"peer-down", MsgPeerDownNotify, true, func(b []byte) int {
			return writePeerDown(b, 0, &PeerDown{Peer: peer, Reason: PeerDownLocalNoNotify, Data: []byte{0x00, 0x02}})
		}, 3},
		{"peer-up", MsgPeerUpNotify, true, func(b []byte) int {
			return writePeerUp(b, 0, &PeerUp{Peer: peer, LocalPort: 179, RemotePort: 50000, SentOpenMsg: sentOpen, ReceivedOpenMsg: recvOpen})
		}, peerUpFixedSize + len(sentOpen) + len(recvOpen)},
		{"initiation", MsgInitiation, false, func(b []byte) int {
			return writeInitiation(b, 0, &Initiation{TLVs: []TLV{sysDescr, sysName}})
		}, 2*TLVHeaderSize + 4},
		{"termination", MsgTermination, false, func(b []byte) int {
			return writeTermination(b, 0, &Termination{TLVs: []TLV{reason}})
		}, TLVHeaderSize + 2},
		{"route-mirroring", MsgRouteMirroring, true, func(b []byte) int {
			return writeRouteMirroring(b, 0, &routeMirroring{Peer: peer, TLVs: []TLV{mirrored}})
		}, TLVHeaderSize + len(update)},
	}
}

// writeWireCase runs one writer into a buffer pre-filled with a non-zero
// pattern, so a field the writer never sets reads as 0xA5 rather than as a
// plausible zero. It returns exactly the octets the writer reported.
func writeWireCase(t *testing.T, c wireCase) []byte {
	t.Helper()
	buf := make([]byte, 1024)
	for i := range buf {
		buf[i] = 0xA5
	}
	n := c.write(buf)
	return buf[:n]
}

// RFC requirement: RFC7854-x-1 positive -- every one of the seven message types
// the sender's writers produce carries Version 3 in the first octet it transmits.
// RFC requirement: RFC7854-x-2 positive -- every transmitted message carries
// Version 3 (never the reserved 0), a Message Length equal to the whole message
// (common header, per-peer header when present, and the data), and its own
// Message Type in the sixth octet.
//
// VALIDATES:prove the common header on the octets a collector receives, not on a
// header the test built. Method: build each type through its writer into a
// patterned buffer, then read octets 0, 1-4 and 5 raw and compare the length
// against the octets the writer produced and the octets the case put in.
func TestRFC7854EveryTransmittedMessageCarriesVersion3AndItsTotalLength(t *testing.T) {
	for _, c := range rfc7854WireCases() {
		t.Run(c.name, func(t *testing.T) {
			msg := writeWireCase(t, c)
			if len(msg) < CommonHeaderSize {
				t.Fatalf("writer produced %d octets, shorter than the common header", len(msg))
			}
			if msg[0] != 3 {
				t.Errorf("Version octet = %d, want 3", msg[0])
			}
			want := CommonHeaderSize + c.bodyWant
			if c.perPeer {
				want += PeerHeaderSize
			}
			if len(msg) != want {
				t.Errorf("writer produced %d octets, want %d", len(msg), want)
			}
			if got := binary.BigEndian.Uint32(msg[1:5]); got != uint32(want) {
				t.Errorf("Message Length = %d, want %d (the whole message)", got, want)
			}
			if msg[5] != c.msgType {
				t.Errorf("Message Type = %d, want %d", msg[5], c.msgType)
			}
		})
	}
}

// RFC requirement: RFC7854-x-3 positive -- on the wire, Route Monitoring,
// Statistics Report, Peer Down, Peer Up and Route Mirroring carry the per-peer
// header in the 42 octets that follow the common header: Peer Type, Flags, the
// 4-octet AS and the BGP ID written there decode to the peer the writer was given.
// RFC requirement: RFC7854-x-3 negative -- Initiation and Termination carry no
// per-peer header: the first Information TLV starts at octet 6, right after the
// common header, so these two types are not among the messages it follows.
//
// VALIDATES:prove the per-peer header sits after the common header for the types
// that carry one, and is absent from the two that do not. Method: write each
// type and read octet 6 onward raw.
func TestRFC7854PerPeerHeaderFollowsTheCommonHeader(t *testing.T) {
	want := testPeerHeader()
	want.PeerAS = 4200000001
	for _, c := range rfc7854WireCases() {
		t.Run(c.name, func(t *testing.T) {
			msg := writeWireCase(t, c)
			if !c.perPeer {
				// Both cases write a type-1 TLV first (sysDescr, Reason), then its
				// 2-octet length: a per-peer header would put Peer Type and Flags here.
				if got := binary.BigEndian.Uint16(msg[CommonHeaderSize : CommonHeaderSize+2]); got != 1 {
					t.Errorf("octets 6-7 = %d, want the first Information TLV type 1", got)
				}
				if got := binary.BigEndian.Uint16(msg[CommonHeaderSize+2 : CommonHeaderSize+4]); got != 2 {
					t.Errorf("octets 8-9 = %d, want the first TLV length 2", got)
				}
				if len(msg) >= CommonHeaderSize+PeerHeaderSize {
					t.Errorf("message is %d octets, room for a per-peer header it must not carry", len(msg))
				}
				return
			}
			got, _, err := decodePeerHeader(msg, CommonHeaderSize)
			if err != nil {
				t.Fatalf("no per-peer header after the common header: %v", err)
			}
			if got != want {
				t.Errorf("per-peer header after the common header = %+v, want %+v", got, want)
			}
		})
	}
}

// RFC requirement: RFC7854-x-4 positive -- the Peer AS is the 4 octets at offset
// 26 of the per-peer header: a 16-bit AS is transmitted with its 16 most
// significant bits zero (00 00 FD E9 for 65001), and an AS above 65535 fills all
// four octets, so neither the padding nor the width is lost.
//
// VALIDATES:prove the field width and the padding on transmitted octets. Method: write
// a Peer Down, which carries the per-peer header, into a patterned buffer and
// read octets 26-29 of the per-peer header raw.
func TestRFC7854PeerASIsFourOctetsWithTheShortASZeroPadded(t *testing.T) {
	cases := []struct {
		name string
		as   uint32
		want [4]byte
	}{
		{"16-bit AS zero-padded", 65001, [4]byte{0x00, 0x00, 0xFD, 0xE9}},
		{"32-bit AS", 4200000001, [4]byte{0xFA, 0x56, 0xEA, 0x01}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			peer := testPeerHeader()
			peer.PeerAS = tc.as
			buf := make([]byte, 128)
			for i := range buf {
				buf[i] = 0xA5
			}
			writePeerDown(buf, 0, &PeerDown{Peer: peer, Reason: PeerDownDeconfigured})
			field := buf[CommonHeaderSize+26 : CommonHeaderSize+30]
			if [4]byte(field) != tc.want {
				t.Errorf("Peer AS octets = % X, want % X", field, tc.want)
			}
		})
	}
}
