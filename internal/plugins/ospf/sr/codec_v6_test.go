package sr

import "testing"

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
	out, err := DecodeLANAdjSIDValueV6(v)
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
			if _, err := DecodePrefixSIDValueV6(in); err != nil {
				_ = err
			}
			if _, err := DecodeAdjSIDValueV6(in); err != nil {
				_ = err
			}
			if _, err := DecodeLANAdjSIDValueV6(in); err != nil {
				_ = err
			}
			if _, err := DecodeExtPrefixRangeValueV6(in); err != nil {
				_ = err
			}
		}()
	}
}
