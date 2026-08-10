package sr

import (
	"bytes"
	"testing"
)

func TestOSPFv3SRTypeCodes(t *testing.T) {
	// RFC 8666 §9: the OSPFv3 Extended-LSA registry values, distinct from the
	// OSPFv2 RFC 8665 numbers.
	if V6TypePrefixSID != 4 {
		t.Fatalf("Prefix-SID v3 type = %d want 4", V6TypePrefixSID)
	}
	if V6TypeAdjSID != 5 {
		t.Fatalf("Adj-SID v3 type = %d want 5", V6TypeAdjSID)
	}
	if V6TypeLANAdjSID != 6 {
		t.Fatalf("LAN-Adj-SID v3 type = %d want 6", V6TypeLANAdjSID)
	}
	if V6TypeSIDLabel != 7 {
		t.Fatalf("SID/Label v3 type = %d want 7", V6TypeSIDLabel)
	}
	if V6TypeExtPrefixRange != 9 {
		t.Fatalf("Ext-Prefix-Range v3 type = %d want 9", V6TypeExtPrefixRange)
	}
	// The RFC 8665 (IPv4) numbers must NOT equal the RFC 8666 ones where they differ.
	if V4TypePrefixSID == V6TypePrefixSID {
		t.Fatalf("IPv4 and IPv6 Prefix-SID codes must differ (%d)", V4TypePrefixSID)
	}
}

// RFC requirement: RFC8666-6-6 negative -- the two VALID V/L combinations are accepted,
// not ignored: V=0/L=0 decodes a 4-octet index and V=1/L=1 a 3-octet local label. The
// "MUST be ignored" of an invalid setting is not a blanket rejection.
func TestOSPFv3PrefixSIDCodec(t *testing.T) {
	// OSPFv3 Prefix-SID layout: Flags, Algorithm, Reserved(2), SID -- no MT-ID.
	in := PrefixSID{Flags: SIDFlags{NP: true}, Algorithm: 0, Index: 9}
	v := EncodePrefixSIDValueV6(in)
	if len(v) != 8 {
		t.Fatalf("v3 index prefix-SID value len = %d want 8", len(v))
	}
	out, err := DecodePrefixSIDValueV6(v)
	if err != nil || !out.Flags.NP || out.Index != 9 || out.IsLabel {
		t.Fatalf("v3 prefix-SID round-trip = %+v,%v", out, err)
	}
	// Algorithm byte is at offset 1 in v3 (offset 3 in v2).
	if v[1] != 0 {
		t.Fatalf("v3 algorithm must be at offset 1")
	}
	// Local-label form.
	in = PrefixSID{Flags: SIDFlags{NP: true, V: true, L: true}, Label: 100700}
	v = EncodePrefixSIDValueV6(in)
	out, err = DecodePrefixSIDValueV6(v)
	if err != nil || !out.IsLabel || out.Label != 100700 {
		t.Fatalf("v3 prefix-SID label form = %+v,%v", out, err)
	}
}

// RFC requirement: RFC8666-6-6 positive -- a SID advertisement with an invalid V-/L-Flag
// combination (V=1/L=0 or V=0/L=1) is ignored: the decoder rejects it so no SID is used.
func TestOSPFv3SIDWidthFromVL(t *testing.T) {
	// Invalid V/L combinations must be rejected (RFC 8666 §6).
	invalid := []byte{0x08, 0, 0, 0, 0, 0, 0, 0} // V set, L clear
	if _, err := DecodePrefixSIDValueV6(invalid); err == nil {
		t.Fatalf("v3 V=1/L=0 must be rejected")
	}
	invalid[0] = 0x04 // L set, V clear
	if _, err := DecodePrefixSIDValueV6(invalid); err == nil {
		t.Fatalf("v3 V=0/L=1 must be rejected")
	}
}

func TestOSPFv3AdjSIDCodec(t *testing.T) {
	// OSPFv3 Adj-SID layout: Flags, Weight, Reserved(2), SID -- no MT-ID.
	in := AdjSID{Flags: AdjSIDFlags{V: true, L: true}, Weight: 3, Label: 40010}
	v := EncodeAdjSIDValueV6(in)
	out, err := DecodeAdjSIDValueV6(v)
	if err != nil || !out.IsLabel || out.Label != 40010 || out.Weight != 3 {
		t.Fatalf("v3 adj-SID round-trip = %+v,%v", out, err)
	}
	if v[1] != 3 {
		t.Fatalf("v3 weight must be at offset 1")
	}
}

func TestOSPFv3LANAdjSIDCodec(t *testing.T) {
	nbr := [4]byte{0, 0, 0, 9}
	in := AdjSID{Flags: AdjSIDFlags{V: true, L: true}, Weight: 0, Label: 40011, NeighborID: nbr}
	v := EncodeLANAdjSIDValueV6(in)
	out, err := decodeLANAdjSIDValueV6(v)
	if err != nil || out.NeighborID != nbr || out.Label != 40011 {
		t.Fatalf("v3 lan-adj-SID round-trip = %+v,%v", out, err)
	}
}

// RFC requirement: RFC8666-11-1 negative -- malformed-input detection is not achieved by
// rejecting everything: a well-formed Extended Prefix Range TLV (including the /0 default
// route and the ((PrefixLength+31)/32)-word padding) decodes intact.
func TestOSPFv3ExtPrefixRangeTLVRoundTrip(t *testing.T) {
	cases := []struct {
		plen uint8
		addr []byte
	}{
		{0, nil}, // default route
		{64, []byte{0x20, 0x01, 0x0d, 0xb8, 0, 0, 0, 0}},                          // /64 -> 2 words
		{128, []byte{0x20, 0x01, 0x0d, 0xb8, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 1}}, // /128 -> 4 words
	}
	for _, c := range cases {
		v := EncodeExtPrefixRangeValueV6(c.plen, c.addr, 4, PrefixSID{Flags: SIDFlags{NP: true}, Index: 50})
		rng, err := DecodeExtPrefixRangeValueV6(v)
		if err != nil {
			t.Fatalf("plen %d decode: %v", c.plen, err)
		}
		if rng.PrefixLength != c.plen || rng.AF != 1 || rng.RangeSize != 4 {
			t.Fatalf("plen %d header round-trip = %+v", c.plen, rng)
		}
		wantWords := (int(c.plen) + 31) / 32
		if len(rng.AddressV6) != wantWords*4 {
			t.Fatalf("plen %d address len = %d want %d", c.plen, len(rng.AddressV6), wantWords*4)
		}
		if len(rng.PrefixSIDs) != 1 || rng.PrefixSIDs[0].Index != 50 {
			t.Fatalf("plen %d sub-SID lost: %+v", c.plen, rng.PrefixSIDs)
		}
	}
}

// RFC requirement: RFC8666-11-1 positive -- malformed RFC 8666 sub-TLV values (truncated,
// empty, or shorter than the width their V/L flags imply) are detected and rejected by
// every OSPFv3 SR decoder without panicking, so a hostile advertisement cannot crash the
// routing process.
func TestOSPFv3SRTLVMalformed(t *testing.T) {
	inputs := [][]byte{nil, {}, {0x40}, {0x48, 0, 0, 0}, {0, 1}, {0, 1, 0, 0}}
	for i, in := range inputs {
		func() {
			defer func() {
				if r := recover(); r != nil {
					t.Fatalf("v3 input %d panicked: %v", i, r)
				}
			}()
			// Every input is malformed for every decoder: nil/{}/{0x40}/{0,1} are shorter
			// than the 4-octet (8 for the LAN and Range forms) fixed header; {0x48,...}
			// carries V=1/L=0, an invalid V/L pair; {0,1,0,0} passes the fixed header but
			// is one octet short of the 4-octet index its V=0/L=0 flags imply. Asserting
			// the ERROR, not merely the absence of a panic, is what makes this test fail
			// if a length or V/L guard stops rejecting -- a silent zero-valued SID would
			// otherwise be installed from a truncated advertisement
			// (v6PrefixSIDFromPrefixTLV, internal/plugins/ospf/sr_reception_v6.go:137-140).
			if got, err := DecodePrefixSIDValueV6(in); err == nil {
				t.Fatalf("v3 input %d: DecodePrefixSIDValueV6 accepted a malformed value: %+v", i, got)
			}
			if got, err := DecodeAdjSIDValueV6(in); err == nil {
				t.Fatalf("v3 input %d: DecodeAdjSIDValueV6 accepted a malformed value: %+v", i, got)
			}
			if got, err := decodeLANAdjSIDValueV6(in); err == nil {
				t.Fatalf("v3 input %d: DecodeLANAdjSIDValueV6 accepted a malformed value: %+v", i, got)
			}
			if got, err := DecodeExtPrefixRangeValueV6(in); err == nil {
				t.Fatalf("v3 input %d: DecodeExtPrefixRangeValueV6 accepted a malformed value: %+v", i, got)
			}
		}()
	}
}

// rangeDecodeEqual compares two decoded Extended Prefix Range TLVs field by field
// (ExtPrefixRange holds slices, so it is not comparable with ==).
func rangeDecodeEqual(a, b ExtPrefixRange) bool {
	if a.PrefixLength != b.PrefixLength || a.AF != b.AF || a.RangeSize != b.RangeSize ||
		a.IAFlag != b.IAFlag || !bytes.Equal(a.AddressV6, b.AddressV6) ||
		len(a.PrefixSIDs) != len(b.PrefixSIDs) {
		return false
	}
	for i := range a.PrefixSIDs {
		if a.PrefixSIDs[i] != b.PrefixSIDs[i] {
			return false
		}
	}
	return true
}

// TestRFC8666V6ExtPrefixRangeReservedIgnoredOnReceive verifies the 3-octet Reserved field
// of the IPv6 Extended Prefix Range TLV does not reach the decoded value.
//
// VALIDATES: octets 5..7 set to 0xFF decode to exactly the same range as all-zero
// Reserved, and the encoder leaves them zero on transmission.
//
// PREVENTS: a peer that fills Reserved (a future extension, or padding garbage) making an
// otherwise valid Prefix-SID range decode differently or be discarded.
//
// RFC requirement: RFC8666-5-7 positive -- the Reserved field of the Extended Prefix Range
// TLV MUST be ignored on reception (RFC 8666 §5), and DecodeExtPrefixRangeValueV6 never
// reads v[5:8] (internal/plugins/ospf/sr/codec_v6.go:204-220).
func TestRFC8666V6ExtPrefixRangeReservedIgnoredOnReceive(t *testing.T) {
	addr := [16]byte{0x20, 0x01, 0x0d, 0xb8, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 1}
	clean := EncodeExtPrefixRangeValueV6(128, addr[:], 4, PrefixSID{Flags: SIDFlags{NP: true}, Index: 50})
	want, err := DecodeExtPrefixRangeValueV6(clean)
	if err != nil {
		t.Fatalf("clean decode: %v", err)
	}
	if clean[5] != 0 || clean[6] != 0 || clean[7] != 0 {
		t.Fatalf("Reserved must be zero on transmission: %#02x%02x%02x", clean[5], clean[6], clean[7])
	}
	dirty := append([]byte(nil), clean...)
	dirty[5], dirty[6], dirty[7] = 0xFF, 0xFF, 0xFF
	got, err := DecodeExtPrefixRangeValueV6(dirty)
	if err != nil {
		t.Fatalf("decode with Reserved=0xFFFFFF: %v", err)
	}
	if !rangeDecodeEqual(got, want) {
		t.Fatalf("Reserved field changed the decode: %+v vs %+v", got, want)
	}
}

// TestRFC8666V6ExtPrefixRangeAFOctetNotIgnored verifies the ignoring is confined to the
// Reserved octets.
//
// VALIDATES: the adjacent AF octet is honored, so changing it changes the decode.
//
// PREVENTS: a decoder that discards the whole fixed header as reserved and so cannot tell
// an IPv4 range from an IPv6 one.
//
// RFC requirement: RFC8666-5-7 negative -- "ignored" applies to the Reserved field only:
// the AF octet at offset 1 is read into ExtPrefixRange.AF
// (internal/plugins/ospf/sr/codec_v6.go:208-211) and an AF the caller does not want is
// what lets reception reject it.
func TestRFC8666V6ExtPrefixRangeAFOctetNotIgnored(t *testing.T) {
	addr := [16]byte{0x20, 0x01, 0x0d, 0xb8, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 1}
	clean := EncodeExtPrefixRangeValueV6(128, addr[:], 4, PrefixSID{Flags: SIDFlags{NP: true}, Index: 50})
	dirty := append([]byte(nil), clean...)
	dirty[1] = 0 // AF = IPv4 unicast
	got, err := DecodeExtPrefixRangeValueV6(dirty)
	if err != nil {
		t.Fatalf("decode: %v", err)
	}
	if got.AF != 0 {
		t.Fatalf("the AF octet must be honored, got %d", got.AF)
	}
}
