// VALIDATES: RFC 8666 (OSPFv3 Extensions for Segment Routing) wire-codec obligations --
// the reserved Flags octet of the Extended Prefix Range TLV, the reserved flag bits and
// the Reserved fields of the Prefix-SID / Adj-SID / LAN-Adj-SID sub-TLVs are zero on
// transmit and ignored on receive, and "ignored" stays confined to those fields.
// PREVENTS: a reserved bit leaking a meaning onto the wire, a decoder that silently
// swallows a field it must honor (Algorithm, Weight, Neighbor ID) under the guise of
// ignoring the neighboring Reserved octets.
package sr

import (
	"bytes"
	"testing"
)

// RFC requirement: RFC8666-5-2 positive -- the Flags octet of the OSPFv3 Extended Prefix
// Range TLV is reserved: the encoder never sets a bit in it (offset 4 of the value).
func TestRFC8666ExtPrefixRangeFlagsZeroOnSend(t *testing.T) {
	addr := []byte{0x20, 0x01, 0x0d, 0xb8, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 1}
	v := EncodeExtPrefixRangeValueV6(128, addr, 1, PrefixSID{Flags: SIDFlags{NP: true}, Index: 50})
	if v[4] != 0 {
		t.Fatalf("Extended Prefix Range Flags octet = %#02x, must be zero when sent", v[4])
	}
}

// RFC requirement: RFC8666-5-2 negative -- a received Flags octet with every bit set is
// ignored: the prefix, Range Size and Prefix-SID decode exactly as with a zero Flags octet.
func TestRFC8666ExtPrefixRangeFlagsIgnoredOnReceive(t *testing.T) {
	addr := []byte{0x20, 0x01, 0x0d, 0xb8, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 1}
	clean := EncodeExtPrefixRangeValueV6(128, addr, 4, PrefixSID{Flags: SIDFlags{NP: true}, Index: 50})
	want, err := DecodeExtPrefixRangeValueV6(clean)
	if err != nil {
		t.Fatalf("clean decode: %v", err)
	}
	dirty := make([]byte, len(clean))
	copy(dirty, clean)
	dirty[4] = 0xFF
	got, err := DecodeExtPrefixRangeValueV6(dirty)
	if err != nil {
		t.Fatalf("decode with Flags=0xFF: %v", err)
	}
	if got.PrefixLength != want.PrefixLength || got.AF != want.AF || got.RangeSize != want.RangeSize {
		t.Fatalf("Flags octet changed the header decode: %+v vs %+v", got, want)
	}
	if len(got.PrefixSIDs) != 1 || got.PrefixSIDs[0] != want.PrefixSIDs[0] {
		t.Fatalf("Flags octet changed the Prefix-SID decode: %+v vs %+v", got.PrefixSIDs, want.PrefixSIDs)
	}
	if !bytes.Equal(got.AddressV6, want.AddressV6) {
		t.Fatalf("Flags octet changed the address decode")
	}
}

// RFC requirement: RFC8666-6-3 positive -- with every DEFINED Prefix-SID flag set, the
// encoded flags octet still leaves reserved bits 0, 6 and 7 clear.
func TestRFC8666PrefixSIDReservedFlagBitsZeroOnSend(t *testing.T) {
	all := PrefixSID{Flags: SIDFlags{NP: true, M: true, E: true, V: true, L: true}, Label: 100}
	v := EncodePrefixSIDValueV6(all)
	// Bit 0 is 0x80, bit 6 is 0x02, bit 7 is 0x01 (bit 0 = most significant).
	if v[0]&0x83 != 0 {
		t.Fatalf("Prefix-SID flags = %#02x, reserved bits 0/6/7 must be zero when sent", v[0])
	}
	if v[0] != 0x7C {
		t.Fatalf("Prefix-SID flags = %#02x, want NP|M|E|V|L = 0x7C", v[0])
	}
}

// RFC requirement: RFC8666-6-3 negative -- reserved bits 0, 6 and 7 set on the wire are
// ignored on receive: the decoded Prefix-SID is identical to the one without them.
func TestRFC8666PrefixSIDReservedFlagBitsIgnoredOnReceive(t *testing.T) {
	clean := EncodePrefixSIDValueV6(PrefixSID{Flags: SIDFlags{NP: true}, Algorithm: 0, Index: 9})
	want, err := DecodePrefixSIDValueV6(clean)
	if err != nil {
		t.Fatalf("clean decode: %v", err)
	}
	dirty := make([]byte, len(clean))
	copy(dirty, clean)
	dirty[0] |= 0x83 // reserved bits 0, 6, 7
	got, err := DecodePrefixSIDValueV6(dirty)
	if err != nil {
		t.Fatalf("decode with reserved flag bits set: %v", err)
	}
	if got != want {
		t.Fatalf("reserved flag bits changed the decode: %+v vs %+v", got, want)
	}
}

// RFC requirement: RFC8666-6-4 positive -- the 2-octet Reserved field of the Prefix-SID
// sub-TLV is ignored on reception: a value with 0xFFFF there decodes to the same SID.
func TestRFC8666PrefixSIDReservedFieldIgnoredOnReceive(t *testing.T) {
	clean := EncodePrefixSIDValueV6(PrefixSID{Flags: SIDFlags{NP: true}, Algorithm: 0, Index: 9})
	want, err := DecodePrefixSIDValueV6(clean)
	if err != nil {
		t.Fatalf("clean decode: %v", err)
	}
	dirty := make([]byte, len(clean))
	copy(dirty, clean)
	dirty[2], dirty[3] = 0xFF, 0xFF
	got, err := DecodePrefixSIDValueV6(dirty)
	if err != nil {
		t.Fatalf("decode with Reserved=0xFFFF: %v", err)
	}
	if got != want {
		t.Fatalf("Reserved field changed the decode: %+v vs %+v", got, want)
	}
	if clean[2] != 0 || clean[3] != 0 {
		t.Fatalf("Prefix-SID Reserved field must be zero on transmission: %#02x%02x", clean[2], clean[3])
	}
}

// RFC requirement: RFC8666-6-4 negative -- "ignored" is confined to the Reserved octets:
// the adjacent Algorithm octet is NOT ignored, so changing it changes the decode.
func TestRFC8666PrefixSIDAlgorithmOctetNotIgnored(t *testing.T) {
	clean := EncodePrefixSIDValueV6(PrefixSID{Flags: SIDFlags{NP: true}, Algorithm: 0, Index: 9})
	dirty := make([]byte, len(clean))
	copy(dirty, clean)
	dirty[1] = 7
	got, err := DecodePrefixSIDValueV6(dirty)
	if err != nil {
		t.Fatalf("decode: %v", err)
	}
	if got.Algorithm != 7 {
		t.Fatalf("Algorithm octet must be honored, got %d", got.Algorithm)
	}
}

// RFC requirement: RFC8666-7.1-1 positive -- with every DEFINED Adj-SID flag set, the
// encoded flags octet still leaves reserved bits 5, 6 and 7 clear.
func TestRFC8666AdjSIDReservedFlagBitsZeroOnSend(t *testing.T) {
	all := AdjSID{Flags: AdjSIDFlags{B: true, V: true, L: true, G: true, P: true}, Label: 100}
	v := EncodeAdjSIDValueV6(all)
	if v[0]&0x07 != 0 {
		t.Fatalf("Adj-SID flags = %#02x, reserved bits 5/6/7 must be zero when sent", v[0])
	}
	if v[0] != 0xF8 {
		t.Fatalf("Adj-SID flags = %#02x, want B|V|L|G|P = 0xF8", v[0])
	}
}

// RFC requirement: RFC8666-7.1-1 negative -- reserved bits 5, 6 and 7 set on the wire are
// ignored on receive: the decoded Adj-SID is identical to the one without them.
func TestRFC8666AdjSIDReservedFlagBitsIgnoredOnReceive(t *testing.T) {
	clean := EncodeAdjSIDValueV6(AdjSID{Flags: AdjSIDFlags{V: true, L: true}, Weight: 3, Label: 40010})
	want, err := DecodeAdjSIDValueV6(clean)
	if err != nil {
		t.Fatalf("clean decode: %v", err)
	}
	dirty := make([]byte, len(clean))
	copy(dirty, clean)
	dirty[0] |= 0x07
	got, err := DecodeAdjSIDValueV6(dirty)
	if err != nil {
		t.Fatalf("decode with reserved flag bits set: %v", err)
	}
	if got != want {
		t.Fatalf("reserved flag bits changed the decode: %+v vs %+v", got, want)
	}
}

// RFC requirement: RFC8666-7.1-2 positive -- the 2-octet Reserved field of the Adj-SID
// sub-TLV is zero on transmission and ignored on reception.
func TestRFC8666AdjSIDReservedFieldIgnoredOnReceive(t *testing.T) {
	clean := EncodeAdjSIDValueV6(AdjSID{Flags: AdjSIDFlags{V: true, L: true}, Weight: 3, Label: 40010})
	if clean[2] != 0 || clean[3] != 0 {
		t.Fatalf("Adj-SID Reserved field must be zero on transmission: %#02x%02x", clean[2], clean[3])
	}
	want, err := DecodeAdjSIDValueV6(clean)
	if err != nil {
		t.Fatalf("clean decode: %v", err)
	}
	dirty := make([]byte, len(clean))
	copy(dirty, clean)
	dirty[2], dirty[3] = 0xFF, 0xFF
	got, err := DecodeAdjSIDValueV6(dirty)
	if err != nil {
		t.Fatalf("decode with Reserved=0xFFFF: %v", err)
	}
	if got != want {
		t.Fatalf("Reserved field changed the decode: %+v vs %+v", got, want)
	}
}

// RFC requirement: RFC8666-7.1-2 negative -- "ignored" is confined to the Reserved octets:
// the adjacent Weight octet is NOT ignored, so changing it changes the decode.
func TestRFC8666AdjSIDWeightOctetNotIgnored(t *testing.T) {
	clean := EncodeAdjSIDValueV6(AdjSID{Flags: AdjSIDFlags{V: true, L: true}, Weight: 3, Label: 40010})
	dirty := make([]byte, len(clean))
	copy(dirty, clean)
	dirty[1] = 200
	got, err := DecodeAdjSIDValueV6(dirty)
	if err != nil {
		t.Fatalf("decode: %v", err)
	}
	if got.Weight != 200 {
		t.Fatalf("Weight octet must be honored, got %d", got.Weight)
	}
}

// RFC requirement: RFC8666-7.2-1 positive -- the 2-octet Reserved field of the LAN Adj-SID
// sub-TLV is zero on transmission and ignored on reception.
func TestRFC8666LANAdjSIDReservedFieldIgnoredOnReceive(t *testing.T) {
	nbr := [4]byte{0, 0, 0, 9}
	clean := EncodeLANAdjSIDValueV6(AdjSID{Flags: AdjSIDFlags{V: true, L: true}, Label: 40011, NeighborID: nbr})
	if clean[2] != 0 || clean[3] != 0 {
		t.Fatalf("LAN Adj-SID Reserved field must be zero on transmission: %#02x%02x", clean[2], clean[3])
	}
	want, err := decodeLANAdjSIDValueV6(clean)
	if err != nil {
		t.Fatalf("clean decode: %v", err)
	}
	dirty := make([]byte, len(clean))
	copy(dirty, clean)
	dirty[2], dirty[3] = 0xFF, 0xFF
	got, err := decodeLANAdjSIDValueV6(dirty)
	if err != nil {
		t.Fatalf("decode with Reserved=0xFFFF: %v", err)
	}
	if got != want {
		t.Fatalf("Reserved field changed the decode: %+v vs %+v", got, want)
	}
}

// RFC requirement: RFC8666-7.2-1 negative -- "ignored" is confined to the Reserved octets:
// the Neighbor ID that follows them is NOT ignored, so changing it changes the decode.
func TestRFC8666LANAdjSIDNeighborIDNotIgnored(t *testing.T) {
	clean := EncodeLANAdjSIDValueV6(AdjSID{Flags: AdjSIDFlags{V: true, L: true}, Label: 40011, NeighborID: [4]byte{0, 0, 0, 9}})
	dirty := make([]byte, len(clean))
	copy(dirty, clean)
	dirty[7] = 42
	got, err := decodeLANAdjSIDValueV6(dirty)
	if err != nil {
		t.Fatalf("decode: %v", err)
	}
	if got.NeighborID != [4]byte{0, 0, 0, 42} {
		t.Fatalf("Neighbor ID must be honored, got %v", got.NeighborID)
	}
}
