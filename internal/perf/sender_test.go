package perf

import (
	"net/netip"
	"testing"
)

func TestSenderInlineNLRI(t *testing.T) {
	t.Parallel()

	// IPv4/unicast UPDATE uses inline NLRI field (no MP_REACH_NLRI).
	sender := NewSender(SenderConfig{
		ASN:     65001,
		IsEBGP:  true,
		NextHop: netip.MustParseAddr("192.168.1.1"),
		Family:  "ipv4/unicast",
		ForceMP: false,
	})

	prefix := netip.MustParsePrefix("10.0.0.0/24")
	data := sender.BuildRoute(prefix)

	if data == nil {
		t.Fatal("BuildRoute returned nil")
	}

	// BGP marker check.
	for i := range 16 {
		if data[i] != 0xFF {
			t.Fatalf("marker byte %d: got 0x%02x, want 0xFF", i, data[i])
		}
	}

	// Type byte at offset 18 must be 2 (UPDATE).
	if data[18] != 2 {
		t.Fatalf("message type: got %d, want 2 (UPDATE)", data[18])
	}

	// Parse UPDATE structure:
	// offset 19: withdrawn routes length (2 bytes)
	withdrawnLen := int(data[19])<<8 | int(data[20])
	if withdrawnLen != 0 {
		t.Fatalf("withdrawn length: got %d, want 0", withdrawnLen)
	}

	// offset 21: total path attribute length (2 bytes)
	attrStart := 21
	attrLen := int(data[attrStart])<<8 | int(data[attrStart+1])
	if attrLen == 0 {
		t.Fatal("path attributes length is 0, expected attributes")
	}

	// Walk attributes to verify no MP_REACH_NLRI (type 14).
	attrDataStart := attrStart + 2
	pos := attrDataStart
	attrEnd := attrDataStart + attrLen

	for pos < attrEnd {
		flags := data[pos]
		code := data[pos+1]
		pos += 2

		var aLen int
		if flags&0x10 != 0 { // Extended length
			aLen = int(data[pos])<<8 | int(data[pos+1])
			pos += 2
		} else {
			aLen = int(data[pos])
			pos++
		}

		if code == 14 {
			t.Fatal("found MP_REACH_NLRI (type 14) in inline NLRI UPDATE")
		}

		pos += aLen
	}

	// Trailing NLRI must be present (non-zero bytes after attributes).
	nlriStart := attrEnd
	nlriLen := len(data) - nlriStart
	if nlriLen == 0 {
		t.Fatal("no trailing NLRI in IPv4/unicast UPDATE")
	}
}

func TestSenderForceMP(t *testing.T) {
	t.Parallel()

	// IPv4/unicast with force-mp encodes in MP_REACH_NLRI (AFI=1/SAFI=1).
	sender := NewSender(SenderConfig{
		ASN:     65001,
		IsEBGP:  true,
		NextHop: netip.MustParseAddr("192.168.1.1"),
		Family:  "ipv4/unicast",
		ForceMP: true,
	})

	prefix := netip.MustParsePrefix("10.0.0.0/24")
	data := sender.BuildRoute(prefix)

	if data == nil {
		t.Fatal("BuildRoute returned nil")
	}

	// Type byte at offset 18 must be 2 (UPDATE).
	if data[18] != 2 {
		t.Fatalf("message type: got %d, want 2 (UPDATE)", data[18])
	}

	// Parse UPDATE structure.
	withdrawnLen := int(data[19])<<8 | int(data[20])
	if withdrawnLen != 0 {
		t.Fatalf("withdrawn length: got %d, want 0", withdrawnLen)
	}

	attrStart := 21
	attrLen := int(data[attrStart])<<8 | int(data[attrStart+1])

	// Walk attributes to find MP_REACH_NLRI (type 14).
	attrDataStart := attrStart + 2
	pos := attrDataStart
	attrEnd := attrDataStart + attrLen

	foundMP := false
	var mpValueStart int
	var mpValueLen int

	for pos < attrEnd {
		flags := data[pos]
		code := data[pos+1]
		pos += 2

		var aLen int
		if flags&0x10 != 0 { // Extended length
			aLen = int(data[pos])<<8 | int(data[pos+1])
			pos += 2
		} else {
			aLen = int(data[pos])
			pos++
		}

		if code == 14 {
			foundMP = true
			mpValueStart = pos
			mpValueLen = aLen
		}

		pos += aLen
	}

	if !foundMP {
		t.Fatal("MP_REACH_NLRI (type 14) not found in force-mp UPDATE")
	}

	// MP_REACH_NLRI value: AFI(2) + SAFI(1) + NH_len(1) + NH + Reserved(1) + NLRI
	if mpValueLen < 5 {
		t.Fatalf("MP_REACH_NLRI value too short: %d bytes", mpValueLen)
	}

	afi := uint16(data[mpValueStart])<<8 | uint16(data[mpValueStart+1])
	safi := data[mpValueStart+2]

	if afi != 1 {
		t.Fatalf("MP_REACH_NLRI AFI: got %d, want 1 (IPv4)", afi)
	}
	if safi != 1 {
		t.Fatalf("MP_REACH_NLRI SAFI: got %d, want 1 (Unicast)", safi)
	}

	// No trailing NLRI after attributes (all NLRI is inside MP_REACH_NLRI).
	nlriStart := attrEnd
	nlriLen := len(data) - nlriStart
	if nlriLen != 0 {
		t.Fatalf("trailing NLRI length: got %d, want 0 (all NLRI in MP_REACH_NLRI)", nlriLen)
	}
}

func TestSenderIPv6(t *testing.T) {
	t.Parallel()

	// IPv6/unicast UPDATE uses MP_REACH_NLRI with AFI=2/SAFI=1.
	sender := NewSender(SenderConfig{
		ASN:     65001,
		IsEBGP:  true,
		NextHop: netip.MustParseAddr("2001:db8::1"),
		Family:  "ipv6/unicast",
		ForceMP: false,
	})

	prefix := netip.MustParsePrefix("2001:db8:1::/48")
	data := sender.BuildRoute(prefix)

	if data == nil {
		t.Fatal("BuildRoute returned nil")
	}

	if data[18] != 2 {
		t.Fatalf("message type: got %d, want 2 (UPDATE)", data[18])
	}

	// Walk attributes to find MP_REACH_NLRI with AFI=2.
	attrStart := 21
	attrLen := int(data[attrStart])<<8 | int(data[attrStart+1])
	attrDataStart := attrStart + 2
	pos := attrDataStart
	attrEnd := attrDataStart + attrLen

	foundMP := false

	for pos < attrEnd {
		flags := data[pos]
		code := data[pos+1]
		pos += 2

		var aLen int
		if flags&0x10 != 0 {
			aLen = int(data[pos])<<8 | int(data[pos+1])
			pos += 2
		} else {
			aLen = int(data[pos])
			pos++
		}

		if code == 14 && aLen >= 3 {
			afi := uint16(data[pos])<<8 | uint16(data[pos+1])
			safi := data[pos+2]
			if afi == 2 && safi == 1 {
				foundMP = true
			}
		}

		pos += aLen
	}

	if !foundMP {
		t.Fatal("MP_REACH_NLRI with AFI=2/SAFI=1 not found in IPv6 UPDATE")
	}
}
