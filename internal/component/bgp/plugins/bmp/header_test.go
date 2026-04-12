package bmp

import (
	"testing"
)

func TestBMPCommonHeaderDecode(t *testing.T) {
	// VALIDATES: AC-1 -- Common Header decoded correctly
	// PREVENTS: bad version or type silently accepted
	tests := []struct {
		name    string
		buf     []byte
		wantVer uint8
		wantLen uint32
		wantTyp uint8
		wantErr bool
	}{
		{
			name:    "valid initiation",
			buf:     []byte{3, 0, 0, 0, 20, MsgInitiation},
			wantVer: 3, wantLen: 20, wantTyp: MsgInitiation,
		},
		{
			name:    "valid route monitoring",
			buf:     []byte{3, 0, 0, 1, 0, MsgRouteMonitoring},
			wantVer: 3, wantLen: 256, wantTyp: MsgRouteMonitoring,
		},
		{
			name:    "too short",
			buf:     []byte{3, 0, 0},
			wantErr: true,
		},
		{
			name:    "bad version 2",
			buf:     []byte{2, 0, 0, 0, 6, MsgInitiation},
			wantErr: true,
		},
		{
			name:    "bad version 4",
			buf:     []byte{4, 0, 0, 0, 6, MsgInitiation},
			wantErr: true,
		},
		{
			name:    "empty buffer",
			buf:     []byte{},
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h, n, err := DecodeCommonHeader(tt.buf, 0)
			if tt.wantErr {
				if err == nil {
					t.Fatal("expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if n != CommonHeaderSize {
				t.Errorf("consumed %d, want %d", n, CommonHeaderSize)
			}
			if h.Version != tt.wantVer {
				t.Errorf("version = %d, want %d", h.Version, tt.wantVer)
			}
			if h.Length != tt.wantLen {
				t.Errorf("length = %d, want %d", h.Length, tt.wantLen)
			}
			if h.Type != tt.wantTyp {
				t.Errorf("type = %d, want %d", h.Type, tt.wantTyp)
			}
		})
	}
}

func TestBMPCommonHeaderEncode(t *testing.T) {
	// VALIDATES: AC-9 -- Common Header serialization
	buf := make([]byte, CommonHeaderSize)
	h := CommonHeader{Version: Version, Length: 48, Type: MsgPeerUpNotify}
	n := WriteCommonHeader(buf, 0, h)
	if n != CommonHeaderSize {
		t.Fatalf("wrote %d, want %d", n, CommonHeaderSize)
	}
	if buf[0] != 3 {
		t.Errorf("version = %d, want 3", buf[0])
	}
	if buf[5] != MsgPeerUpNotify {
		t.Errorf("type = %d, want %d", buf[5], MsgPeerUpNotify)
	}
}

func TestBMPCommonHeaderRoundTrip(t *testing.T) {
	// VALIDATES: AC-9 -- encode then decode produces identical fields
	original := CommonHeader{Version: Version, Length: 1234, Type: MsgStatisticsReport}
	buf := make([]byte, CommonHeaderSize)
	WriteCommonHeader(buf, 0, original)

	decoded, _, err := DecodeCommonHeader(buf, 0)
	if err != nil {
		t.Fatalf("decode failed: %v", err)
	}
	if decoded != original {
		t.Errorf("round-trip mismatch: got %+v, want %+v", decoded, original)
	}
}

func TestBMPPeerHeaderDecode(t *testing.T) {
	// VALIDATES: AC-2 -- Per-Peer Header decoded correctly
	buf := make([]byte, PeerHeaderSize)
	buf[0] = PeerTypeGlobal // peer type
	buf[1] = PeerFlagV      // flags: IPv6

	// Peer distinguisher = 0x0102030405060708
	buf[2], buf[3], buf[4], buf[5] = 0x01, 0x02, 0x03, 0x04
	buf[6], buf[7], buf[8], buf[9] = 0x05, 0x06, 0x07, 0x08

	// Peer address: ::ffff:10.0.0.1 at bytes 10-25
	buf[20] = 0xff
	buf[21] = 0xff
	buf[22] = 10
	buf[23] = 0
	buf[24] = 0
	buf[25] = 1

	// Peer AS = 65001
	buf[26], buf[27], buf[28], buf[29] = 0, 0, 0xFD, 0xE9

	// Peer BGP ID = 1.1.1.1
	buf[30], buf[31], buf[32], buf[33] = 1, 1, 1, 1

	// Timestamp: sec=1000, usec=500
	buf[34], buf[35], buf[36], buf[37] = 0, 0, 0x03, 0xE8
	buf[38], buf[39], buf[40], buf[41] = 0, 0, 0x01, 0xF4

	p, n, err := DecodePeerHeader(buf, 0)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if n != PeerHeaderSize {
		t.Errorf("consumed %d, want %d", n, PeerHeaderSize)
	}
	if p.PeerType != PeerTypeGlobal {
		t.Errorf("peer type = %d, want %d", p.PeerType, PeerTypeGlobal)
	}
	if p.Distinguisher != 0x0102030405060708 {
		t.Errorf("distinguisher = %x, want 0102030405060708", p.Distinguisher)
	}
	if p.PeerAS != 65001 {
		t.Errorf("peer AS = %d, want 65001", p.PeerAS)
	}
	if p.PeerBGPID != 0x01010101 {
		t.Errorf("peer BGP ID = %x, want 01010101", p.PeerBGPID)
	}
	if p.TimestampSec != 1000 {
		t.Errorf("timestamp sec = %d, want 1000", p.TimestampSec)
	}
	if p.TimestampUsec != 500 {
		t.Errorf("timestamp usec = %d, want 500", p.TimestampUsec)
	}
}

func TestBMPPeerHeaderEncode(t *testing.T) {
	// VALIDATES: AC-9 -- Per-Peer Header serialization
	p := PeerHeader{
		PeerType:      PeerTypeL3VPN,
		Flags:         PeerFlagL,
		Distinguisher: 42,
		PeerAS:        65002,
		PeerBGPID:     0x02020202,
		TimestampSec:  2000,
		TimestampUsec: 999,
	}
	p.Address[12], p.Address[13], p.Address[14], p.Address[15] = 192, 168, 1, 1

	buf := make([]byte, PeerHeaderSize)
	n := WritePeerHeader(buf, 0, p)
	if n != PeerHeaderSize {
		t.Fatalf("wrote %d, want %d", n, PeerHeaderSize)
	}

	// Round-trip decode.
	decoded, _, err := DecodePeerHeader(buf, 0)
	if err != nil {
		t.Fatalf("decode failed: %v", err)
	}
	if decoded.PeerType != p.PeerType {
		t.Errorf("peer type = %d, want %d", decoded.PeerType, p.PeerType)
	}
	if decoded.Flags != p.Flags {
		t.Errorf("flags = %d, want %d", decoded.Flags, p.Flags)
	}
	if decoded.Distinguisher != p.Distinguisher {
		t.Errorf("distinguisher = %d, want %d", decoded.Distinguisher, p.Distinguisher)
	}
	if decoded.PeerAS != p.PeerAS {
		t.Errorf("peer AS = %d, want %d", decoded.PeerAS, p.PeerAS)
	}
	if decoded.PeerBGPID != p.PeerBGPID {
		t.Errorf("peer BGP ID = %x, want %x", decoded.PeerBGPID, p.PeerBGPID)
	}
	if decoded.Address != p.Address {
		t.Errorf("address = %v, want %v", decoded.Address, p.Address)
	}
}

func TestBMPPeerHeaderFlags(t *testing.T) {
	// VALIDATES: AC-3, AC-4 -- V, L, A, O flag interpretation
	tests := []struct {
		name       string
		peerType   uint8
		flags      uint8
		wantIPv6   bool
		wantPost   bool
		wantTwoAS  bool
		wantRIBOut bool
	}{
		{"global ipv4 pre-policy", PeerTypeGlobal, 0, false, false, false, false},
		{"global ipv6", PeerTypeGlobal, PeerFlagV, true, false, false, false},
		{"post-policy", PeerTypeGlobal, PeerFlagL, false, true, false, false},
		{"2-byte AS", PeerTypeGlobal, PeerFlagA, false, false, true, false},
		{"adj-rib-out", PeerTypeGlobal, PeerFlagO, false, false, false, true},
		{"all flags", PeerTypeGlobal, PeerFlagV | PeerFlagL | PeerFlagA | PeerFlagO, true, true, true, true},
		{"loc-rib V flag reused as F", PeerTypeLocRIB, PeerFlagV, false, false, false, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := PeerHeader{PeerType: tt.peerType, Flags: tt.flags}
			if got := p.IsIPv6(); got != tt.wantIPv6 {
				t.Errorf("IsIPv6() = %v, want %v", got, tt.wantIPv6)
			}
			if got := p.IsPostPolicy(); got != tt.wantPost {
				t.Errorf("IsPostPolicy() = %v, want %v", got, tt.wantPost)
			}
			if got := p.Is2ByteAS(); got != tt.wantTwoAS {
				t.Errorf("Is2ByteAS() = %v, want %v", got, tt.wantTwoAS)
			}
			if got := p.IsAdjRIBOut(); got != tt.wantRIBOut {
				t.Errorf("IsAdjRIBOut() = %v, want %v", got, tt.wantRIBOut)
			}
		})
	}
}

func TestBMPPeerHeaderIPv4Mapped(t *testing.T) {
	// VALIDATES: AC-4 -- IPv4 stored as ::ffff:x.x.x.x in 16-byte field
	p := PeerHeader{PeerType: PeerTypeGlobal}
	// Set IPv4-mapped IPv6: ::ffff:10.0.0.1
	p.Address[10] = 0xff
	p.Address[11] = 0xff
	p.Address[12] = 10
	p.Address[13] = 0
	p.Address[14] = 0
	p.Address[15] = 1

	buf := make([]byte, PeerHeaderSize)
	WritePeerHeader(buf, 0, p)
	decoded, _, err := DecodePeerHeader(buf, 0)
	if err != nil {
		t.Fatalf("decode failed: %v", err)
	}

	if decoded.Address != p.Address {
		t.Errorf("address mismatch: got %v, want %v", decoded.Address, p.Address)
	}
	// Not IPv6 (V flag not set).
	if decoded.IsIPv6() {
		t.Error("IPv4-mapped address should not be IPv6")
	}
}

func TestBMPPeerHeaderTooShort(t *testing.T) {
	// VALIDATES: AC-6 -- short per-peer header returns error
	buf := make([]byte, PeerHeaderSize-1)
	_, _, err := DecodePeerHeader(buf, 0)
	if err == nil {
		t.Fatal("expected error for short peer header")
	}
}

func TestHasPeerHeader(t *testing.T) {
	// VALIDATES: RFC 7854 -- Initiation and Termination have no per-peer header
	if HasPeerHeader(MsgInitiation) {
		t.Error("Initiation should not have per-peer header")
	}
	if HasPeerHeader(MsgTermination) {
		t.Error("Termination should not have per-peer header")
	}
	for _, mt := range []uint8{MsgRouteMonitoring, MsgStatisticsReport, MsgPeerDownNotify, MsgPeerUpNotify, MsgRouteMirroring} {
		if !HasPeerHeader(mt) {
			t.Errorf("message type %d should have per-peer header", mt)
		}
	}
}
