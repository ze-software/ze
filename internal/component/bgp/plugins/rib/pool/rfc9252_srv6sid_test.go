package pool

import (
	"net/netip"
	"testing"
)

// buildSIDInfoSubTLV builds an SRv6 SID Information Sub-TLV (type 1) per RFC 9252.
// Value: Reserved(1) + SID(16) + Flags(1) + Behavior(2) + Reserved(1) = 21 bytes min.
func buildSIDInfoSubTLV(sid [16]byte) []byte {
	value := make([]byte, 0, 21)
	value = append(value, 0) // Reserved
	value = append(value, sid[:]...)
	value = append(value, 0, 0, 0, 0) // Flags(1) + Behavior(2) + Reserved(1)
	subTLV := append([]byte{1, byte(len(value) >> 8), byte(len(value))}, value...)
	return subTLV
}

// buildServiceTLV wraps sub-TLVs in a Service TLV (type 5 or 6) per RFC 9252.
// Value: Reserved(1) + Sub-TLVs.
func buildServiceTLV(tlvType byte, subTLVs []byte) []byte {
	value := append([]byte{0}, subTLVs...) // Reserved(1) + Sub-TLVs
	return append([]byte{tlvType, byte(len(value) >> 8), byte(len(value))}, value...)
}

func TestExtractSRv6SID_L3Service(t *testing.T) {
	sid := netip.MustParseAddr("2001:db8::1")
	ip6 := sid.As16()

	subTLV := buildSIDInfoSubTLV(ip6)
	tlv := buildServiceTLV(5, subTLV)

	got := ExtractSRv6SID(tlv)
	if got != sid {
		t.Errorf("ExtractSRv6SID() = %v, want %v", got, sid)
	}
}

func TestExtractSRv6SID_L2Service(t *testing.T) {
	sid := netip.MustParseAddr("fc00::42")
	ip6 := sid.As16()

	subTLV := buildSIDInfoSubTLV(ip6)
	tlv := buildServiceTLV(6, subTLV)

	got := ExtractSRv6SID(tlv)
	if got != sid {
		t.Errorf("ExtractSRv6SID() = %v, want %v", got, sid)
	}
}

func TestExtractSRv6SID_LabelIndexTLV(t *testing.T) {
	// TLV type 1 is SR-MPLS Label Index, not SRv6. Should return invalid.
	tlv := []byte{1, 0, 7, 0, 0, 0, 0, 0, 1, 0x2C}

	got := ExtractSRv6SID(tlv)
	if got.IsValid() {
		t.Errorf("ExtractSRv6SID() for label index = %v, want invalid", got)
	}
}

func TestExtractSRv6SID_Empty(t *testing.T) {
	got := ExtractSRv6SID(nil)
	if got.IsValid() {
		t.Errorf("ExtractSRv6SID(nil) = %v, want invalid", got)
	}
}

func TestExtractSRv6SID_Truncated(t *testing.T) {
	// SID Info Sub-TLV with length < 17 (too short for Reserved + SID).
	shortSubTLV := []byte{1, 0, 10, 0, 0x20, 0x01, 0x0d, 0xb8, 0, 0, 0, 0, 0}
	tlv := buildServiceTLV(5, shortSubTLV)

	got := ExtractSRv6SID(tlv)
	if got.IsValid() {
		t.Errorf("ExtractSRv6SID() with short SID = %v, want invalid", got)
	}
}

func TestExtractSRv6SID_WithBehavior(t *testing.T) {
	sid := netip.MustParseAddr("2001:db8:cafe::1")
	ip6 := sid.As16()

	// Full SID Info Sub-TLV with End.DT6 behavior (0x003E).
	value := make([]byte, 0, 21)
	value = append(value, 0) // Reserved
	value = append(value, ip6[:]...)
	value = append(value, 0, 0, 0x3E, 0) // Flags(1) + End.DT6(2) + Reserved(1)
	subTLV := append([]byte{1, byte(len(value) >> 8), byte(len(value))}, value...)
	tlv := buildServiceTLV(5, subTLV)

	got := ExtractSRv6SID(tlv)
	if got != sid {
		t.Errorf("ExtractSRv6SID() = %v, want %v", got, sid)
	}
}

func TestExtractSRv6SID_EmptyServiceTLV(t *testing.T) {
	// Service TLV with only the Reserved byte and no sub-TLVs.
	tlv := []byte{5, 0, 1, 0}

	got := ExtractSRv6SID(tlv)
	if got.IsValid() {
		t.Errorf("ExtractSRv6SID() for empty service TLV = %v, want invalid", got)
	}
}

// buildSIDInfoSubTLVWithStructure builds a SID Info Sub-TLV with a SID Structure Sub-Sub-TLV.
func buildSIDInfoSubTLVWithStructure(sid [16]byte, lbl, lnl, fl, al, transposLen, transposOff uint8) []byte {
	// SID Structure Sub-Sub-TLV: type(1) + length(2) + 6 bytes
	sidStructure := []byte{1, 0, 6, lbl, lnl, fl, al, transposLen, transposOff}

	// SID Info Sub-TLV value: Reserved(1) + SID(16) + Flags(1) + Behavior(2) + Reserved(1) + Sub-Sub-TLVs
	value := make([]byte, 0, 21+len(sidStructure))
	value = append(value, 0) // Reserved
	value = append(value, sid[:]...)
	value = append(value, 0, 0, 0, 0) // Flags(1) + Behavior(2) + Reserved(1)
	value = append(value, sidStructure...)

	return append([]byte{1, byte(len(value) >> 8), byte(len(value))}, value...)
}

func TestExtractSRv6SIDFull_NoTransposition(t *testing.T) {
	sid := netip.MustParseAddr("2001:db8::1")
	ip6 := sid.As16()

	subTLV := buildSIDInfoSubTLV(ip6)
	tlv := buildServiceTLV(5, subTLV)

	r := ExtractSRv6SIDFull(tlv)
	if r.SID != sid {
		t.Errorf("SID = %v, want %v", r.SID, sid)
	}
	if r.HasTranspos {
		t.Error("HasTranspos should be false without SID Structure")
	}
}

// RFC requirement: RFC9252-3.2.1-3 positive -- a SID Structure whose LBL+LNL+FL+AL sum is <= 128 and >= TransposOffset+TransposLen is accepted and its transposition parameters extracted.
func TestExtractSRv6SIDFull_WithTransposition(t *testing.T) {
	// SID with function bits zeroed (transposition will fill them from label).
	sid := netip.MustParseAddr("2001:db8:1::")
	ip6 := sid.As16()

	// LBL=32, LNL=16, FL=16, AL=0, TransposLen=16, TransposOffset=48
	subTLV := buildSIDInfoSubTLVWithStructure(ip6, 32, 16, 16, 0, 16, 48)
	tlv := buildServiceTLV(5, subTLV)

	r := ExtractSRv6SIDFull(tlv)
	if r.SID != sid {
		t.Errorf("SID = %v, want %v", r.SID, sid)
	}
	if !r.HasTranspos {
		t.Fatal("HasTranspos should be true")
	}
	if r.TransposLen != 16 {
		t.Errorf("TransposLen = %d, want 16", r.TransposLen)
	}
	if r.TransposOffset != 48 {
		t.Errorf("TransposOffset = %d, want 48", r.TransposOffset)
	}
}

func TestApplyTransposition(t *testing.T) {
	// SID: 2001:db8:1:: with function bits (bits 48-63) zeroed.
	// Label carries the function value 0xABCD in high-order 16 bits of 20-bit
	// VPN label field (errata 7652: transposed bits occupy high-order positions).
	// After transposition at offset 48, length 16: SID becomes 2001:db8:1:abcd::
	baseSID := netip.MustParseAddr("2001:db8:1::")
	label := uint32(0xABCD) << 4 // 16-bit value in high-order positions of 20-bit label

	got := ApplyTransposition(baseSID, label, 48, 16, 20)
	want := netip.MustParseAddr("2001:db8:1:abcd::")
	if got != want {
		t.Errorf("ApplyTransposition() = %v, want %v", got, want)
	}
}

func TestApplyTransposition_20BitVPN(t *testing.T) {
	// Typical VPN case: 20-bit transposition (MPLS label width).
	// SID: fc00:1:: with function bits (bits 32-51) zeroed.
	// Label = 0x12345 (all 20 bits used, transposLen == labelWidth).
	// After transposition at offset 32, length 20: result is fc00:1:1234:5000::
	baseSID := netip.MustParseAddr("fc00:1::")
	label := uint32(0x12345)

	got := ApplyTransposition(baseSID, label, 32, 20, 20)
	want := netip.MustParseAddr("fc00:1:1234:5000::")
	if got != want {
		t.Errorf("ApplyTransposition(20-bit) = %v, want %v", got, want)
	}
}

func TestApplyTransposition_24BitEVPN(t *testing.T) {
	// EVPN case: 16-bit transposition in a 24-bit label field.
	// SID: 2001:db8:1:: with function bits (bits 48-63) zeroed.
	// Label carries 0xABCD in high-order 16 bits of 24-bit EVPN label.
	// After transposition at offset 48, length 16: SID becomes 2001:db8:1:abcd::
	baseSID := netip.MustParseAddr("2001:db8:1::")
	label := uint32(0xABCD) << 8 // 16-bit value in high-order positions of 24-bit label

	got := ApplyTransposition(baseSID, label, 48, 16, 24)
	want := netip.MustParseAddr("2001:db8:1:abcd::")
	if got != want {
		t.Errorf("ApplyTransposition(EVPN 24-bit) = %v, want %v", got, want)
	}
}

func TestApplyTransposition_ZeroLength(t *testing.T) {
	sid := netip.MustParseAddr("2001:db8::1")
	got := ApplyTransposition(sid, 0xFFFF, 0, 0, 20)
	if got != sid {
		t.Errorf("ApplyTransposition(len=0) = %v, want %v (unchanged)", got, sid)
	}
}

// RFC requirement: RFC9252-3.2.1-3 negative -- a SID Structure whose LBL+LNL+FL+AL sum exceeds 128 is rejected (no transposition applied).
func TestExtractSRv6SIDFull_InvalidSIDStructure(t *testing.T) {
	sid := netip.MustParseAddr("2001:db8::1")
	ip6 := sid.As16()

	// LBL+LNL+FL+AL = 200 > 128 (invalid).
	subTLV := buildSIDInfoSubTLVWithStructure(ip6, 64, 64, 64, 8, 16, 48)
	tlv := buildServiceTLV(5, subTLV)

	r := ExtractSRv6SIDFull(tlv)
	if !r.SID.IsValid() {
		t.Error("SID should still be valid even with invalid SID Structure")
	}
	if r.HasTranspos {
		t.Error("HasTranspos should be false for invalid SID Structure")
	}
}

// RFC requirement: RFC9252-3.2.1-3 negative -- a SID Structure whose LBL+LNL+FL+AL sum is
// less than TransposOffset+TransposLen is rejected (errata 7817: the constraint uses >=).
func TestExtractSRv6SIDFull_SumBelowTransposition(t *testing.T) {
	sid := netip.MustParseAddr("2001:db8::1")
	ip6 := sid.As16()

	// LBL+LNL+FL+AL = 32+16+16+0 = 64, but TransposOffset+TransposLen = 60+16 = 76 > 64.
	// Errata 7817 requires sum >= offset+len, so this SID Structure is invalid.
	subTLV := buildSIDInfoSubTLVWithStructure(ip6, 32, 16, 16, 0, 16, 60)
	tlv := buildServiceTLV(5, subTLV)

	r := ExtractSRv6SIDFull(tlv)
	if !r.SID.IsValid() {
		t.Error("SID should still be valid even with an out-of-range SID Structure")
	}
	if r.HasTranspos {
		t.Error("HasTranspos should be false when the sum is below TransposOffset+TransposLen")
	}
}

// RFC requirement: RFC9252-3.1-2 positive -- a Service TLV with its Reserved octet set to 0 parses and yields the SID.
func TestExtractSRv6SID_ServiceReservedZero(t *testing.T) {
	sid := netip.MustParseAddr("2001:db8::99")
	ip6 := sid.As16()

	subTLV := buildSIDInfoSubTLV(ip6)
	// buildServiceTLV sets the Service TLV Reserved octet to 0.
	tlv := buildServiceTLV(5, subTLV)

	if got := ExtractSRv6SID(tlv); got != sid {
		t.Errorf("ExtractSRv6SID() = %v, want %v", got, sid)
	}
}

// RFC requirement: RFC9252-3.1-2 negative -- a non-zero Service TLV Reserved octet is ignored by the
// receiver (the SID is still extracted), never treated as an error.
func TestExtractSRv6SID_ServiceReservedIgnored(t *testing.T) {
	sid := netip.MustParseAddr("2001:db8::99")
	ip6 := sid.As16()

	subTLV := buildSIDInfoSubTLV(ip6)
	// Value: Reserved(1) + Sub-TLVs, with a NON-ZERO Reserved octet (0xFF).
	value := append([]byte{0xFF}, subTLV...)
	tlv := append([]byte{5, byte(len(value) >> 8), byte(len(value))}, value...)

	if got := ExtractSRv6SID(tlv); got != sid {
		t.Errorf("non-zero Service TLV Reserved must be ignored: ExtractSRv6SID() = %v, want %v", got, sid)
	}
}

// RFC requirement: RFC9252-3.2-4 positive -- a SID carrying an unrecognized SRv6 Endpoint Behavior is
// never considered invalid; the receiver extracts it without inspecting the behavior value.
func TestExtractSRv6SID_UnknownEndpointBehavior(t *testing.T) {
	sid := netip.MustParseAddr("2001:db8:cafe::2")
	ip6 := sid.As16()

	// SID Info Sub-TLV with an unassigned/unknown Endpoint Behavior (0xFEED) and zero AL.
	value := make([]byte, 0, 21)
	value = append(value, 0) // Reserved
	value = append(value, ip6[:]...)
	value = append(value, 0, 0xFE, 0xED, 0) // Flags(1) + Behavior(2)=0xFEED + Reserved(1)
	subTLV := append([]byte{1, byte(len(value) >> 8), byte(len(value))}, value...)
	tlv := buildServiceTLV(5, subTLV)

	if got := ExtractSRv6SID(tlv); got != sid {
		t.Errorf("unknown Endpoint Behavior must not invalidate the SID: got %v, want %v", got, sid)
	}
}
