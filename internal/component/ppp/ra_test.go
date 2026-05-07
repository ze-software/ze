package ppp

import (
	"encoding/binary"
	"net/netip"
	"testing"
)

func TestBuildRA(t *testing.T) {
	var buf [256]byte
	n := BuildRA(buf[:], RAConfig{
		CurHopLimit:    64,
		Managed:        true,
		OtherConfig:    true,
		RouterLifetime: 1800,
	})

	if n < 16 {
		t.Fatalf("RA too short: %d bytes", n)
	}

	// ICMPv6 type = 134 (Router Advertisement)
	if buf[0] != 134 {
		t.Errorf("type = %d, want 134", buf[0])
	}
	// Code = 0
	if buf[1] != 0 {
		t.Errorf("code = %d, want 0", buf[1])
	}
	// Cur Hop Limit
	if buf[4] != 64 {
		t.Errorf("hop limit = %d, want 64", buf[4])
	}
	// M + O flags
	if buf[5] != 0xc0 {
		t.Errorf("flags = 0x%02x, want 0xc0 (M+O)", buf[5])
	}
	// Router Lifetime
	lifetime := binary.BigEndian.Uint16(buf[6:8])
	if lifetime != 1800 {
		t.Errorf("lifetime = %d, want 1800", lifetime)
	}
}

func TestBuildRAWithRDNSS(t *testing.T) {
	dns := netip.MustParseAddr("2001:4860:4860::8888")
	var buf [256]byte
	n := BuildRA(buf[:], RAConfig{
		CurHopLimit:    64,
		Managed:        true,
		OtherConfig:    true,
		RouterLifetime: 1800,
		RDNSS:          []netip.Addr{dns},
		RDNSSLifetime:  3600,
	})

	// Base RA header is 16 bytes, RDNSS option is 8 + 16*count bytes
	expectedMin := 16 + 8 + 16
	if n < expectedMin {
		t.Fatalf("RA with RDNSS too short: %d bytes, want >= %d", n, expectedMin)
	}

	// Find RDNSS option (type 25) after the 16-byte RA header
	off := 16
	found := false
	for off < n {
		optType := buf[off]
		optLen := int(buf[off+1]) * 8
		if optLen == 0 {
			break
		}
		if optType == 25 {
			found = true
			rdnssLifetime := binary.BigEndian.Uint32(buf[off+4 : off+8])
			if rdnssLifetime != 3600 {
				t.Errorf("RDNSS lifetime = %d, want 3600", rdnssLifetime)
			}
			var addr [16]byte
			copy(addr[:], buf[off+8:off+24])
			got := netip.AddrFrom16(addr)
			if got != dns {
				t.Errorf("RDNSS addr = %s, want %s", got, dns)
			}
			break
		}
		off += optLen
	}
	if !found {
		t.Error("RDNSS option not found in RA")
	}
}

func TestBuildRAManagedOnly(t *testing.T) {
	var buf [256]byte
	BuildRA(buf[:], RAConfig{
		Managed:        true,
		OtherConfig:    false,
		RouterLifetime: 600,
	})

	// M=1, O=0
	if buf[5] != 0x80 {
		t.Errorf("flags = 0x%02x, want 0x80 (M only)", buf[5])
	}
}

func TestBuildRANoFlags(t *testing.T) {
	var buf [256]byte
	BuildRA(buf[:], RAConfig{
		Managed:        false,
		OtherConfig:    false,
		RouterLifetime: 600,
	})

	if buf[5] != 0x00 {
		t.Errorf("flags = 0x%02x, want 0x00", buf[5])
	}
}
