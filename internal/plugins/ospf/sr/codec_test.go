package sr

import (
	"bytes"
	"testing"
)

func TestSIDLabelSubTLVRoundTrip(t *testing.T) {
	// 3-octet local label form.
	v := encodeSIDLabelSubTLV(true, 100000)
	isLabel, val, err := decodeSIDLabelSubTLV(v)
	if err != nil || !isLabel || val != 100000 {
		t.Fatalf("label round-trip = %v,%d,%v want true,100000,nil", isLabel, val, err)
	}
	// 4-octet index form.
	v = encodeSIDLabelSubTLV(false, 4242)
	isLabel, val, err = decodeSIDLabelSubTLV(v)
	if err != nil || isLabel || val != 4242 {
		t.Fatalf("index round-trip = %v,%d,%v want false,4242,nil", isLabel, val, err)
	}
}

func TestSRAlgorithmTLVRoundTrip(t *testing.T) {
	// RFC 8665 §3.1: if advertised, Algorithm 0 MUST be present.
	v := EncodeAlgorithmValue([]uint8{0})
	algos, err := DecodeAlgorithmValue(v)
	if err != nil || len(algos) != 1 || algos[0] != 0 {
		t.Fatalf("algorithm round-trip = %v,%v", algos, err)
	}
	if !HasAlgorithm(algos, 0) {
		t.Fatalf("algorithm 0 must be present")
	}
}

func TestSRGBRangeTLVRoundTrip(t *testing.T) {
	v := EncodeRangeValue(LabelRange{Base: 16000, Size: 8000})
	r, err := DecodeRangeValue(v)
	if err != nil {
		t.Fatalf("range decode: %v", err)
	}
	if r.Base != 16000 || r.Size != 8000 {
		t.Fatalf("range round-trip = %+v want {16000 8000}", r)
	}
}

func TestSRLBTLVRoundTrip(t *testing.T) {
	v := EncodeRangeValue(LabelRange{Base: 40000, Size: 1000})
	r, err := DecodeRangeValue(v)
	if err != nil || r.Base != 40000 || r.Size != 1000 {
		t.Fatalf("SRLB range round-trip = %+v,%v", r, err)
	}
}

func TestSRRangeSizeZeroRejected(t *testing.T) {
	// RangeSize 0 in a received range TLV is malformed (RFC 8665 §3.2).
	v := EncodeRangeValue(LabelRange{Base: 16000, Size: 1})
	v[0], v[1], v[2] = 0, 0, 0
	if _, err := DecodeRangeValue(v); err == nil {
		t.Fatalf("Range Size 0 must be rejected")
	}
}

func TestSRRangeMultipleSIDLabelSubTLVsRejected(t *testing.T) {
	// RFC 8665 §3.2: more than one SID/Label sub-TLV -> the range TLV is ignored.
	base := EncodeRangeValue(LabelRange{Base: 16000, Size: 10})
	second := encodeSIDLabelSubTLV(true, 16001)
	doubled := append(append([]byte{}, base...), second...)
	if _, err := DecodeRangeValue(doubled); err == nil {
		t.Fatalf("range with two SID/Label sub-TLVs must be rejected")
	}
}

func TestSRGBRangeReservedBaseRejected(t *testing.T) {
	// RFC 8665 §10 / RFC 8666 §11 receive hardening: a range whose base is a reserved MPLS
	// label (0..15) must be rejected so it can never source a reserved label.
	for _, base := range []uint32{0, 3, 15} {
		if _, err := DecodeRangeValue(EncodeRangeValue(LabelRange{Base: base, Size: 10})); err == nil {
			t.Fatalf("reserved base %d must be rejected", base)
		}
	}
	// The first non-reserved label (16) is accepted.
	if _, err := DecodeRangeValue(EncodeRangeValue(LabelRange{Base: MinLabel, Size: 10})); err != nil {
		t.Fatalf("base 16 must be accepted: %v", err)
	}
}

func TestSRGBRangeAboveLabelSpaceRejected(t *testing.T) {
	// A range that extends past the largest 20-bit label is rejected.
	if _, err := DecodeRangeValue(EncodeRangeValue(LabelRange{Base: MaxLabel - 5, Size: 100})); err == nil {
		t.Fatalf("a range running past MaxLabel must be rejected")
	}
	// A base beyond the 20-bit label space, encoded as a 4-octet (index-form) SID sub-TLV
	// (value 0x00110000 > 2^20-1), is rejected rather than truncated into a valid label.
	v := make([]byte, 4)
	put24(v, 10) // Range Size 10; v[3] reserved
	v = append(v, 0x00, 0x01, 0x00, 0x04, 0x00, 0x11, 0x00, 0x00)
	if _, err := DecodeRangeValue(v); err == nil {
		t.Fatalf("a base above the 20-bit label space (0x00110000) must be rejected")
	}
}

func TestSRMSPrefTLVRoundTrip(t *testing.T) {
	v := EncodeSRMSValue(200)
	pref, err := DecodeSRMSValue(v)
	if err != nil || pref != 200 {
		t.Fatalf("SRMS round-trip = %d,%v want 200,nil", pref, err)
	}
}

func TestPrefixSIDSubTLVRoundTrip(t *testing.T) {
	in := PrefixSID{Flags: SIDFlags{NP: true}, MTID: 0, Algorithm: 0, Index: 5}
	v := EncodePrefixSIDValue(in)
	out, err := DecodePrefixSIDValue(v)
	if err != nil {
		t.Fatalf("prefix-SID decode: %v", err)
	}
	if !out.Flags.NP || out.Index != 5 || out.Algorithm != 0 || out.IsLabel {
		t.Fatalf("prefix-SID round-trip = %+v", out)
	}
	in = PrefixSID{Flags: SIDFlags{NP: true, V: true, L: true}, Label: 100500}
	v = EncodePrefixSIDValue(in)
	out, err = DecodePrefixSIDValue(v)
	if err != nil || !out.IsLabel || out.Label != 100500 {
		t.Fatalf("prefix-SID label form = %+v,%v", out, err)
	}
}

func TestSRSIDFieldVL(t *testing.T) {
	// Any V/L combination other than 00 or 11 is invalid (RFC 8665 §5).
	invalid := []byte{0x08, 0, 0, 0, 0, 0, 0, 0} // V set, L clear, 4-octet SID
	if _, err := DecodePrefixSIDValue(invalid); err == nil {
		t.Fatalf("V=1/L=0 must be rejected")
	}
	invalid[0] = 0x04 // L set, V clear
	if _, err := DecodePrefixSIDValue(invalid); err == nil {
		t.Fatalf("V=0/L=1 must be rejected")
	}
}

func TestAdjSIDSubTLVRoundTrip(t *testing.T) {
	in := AdjSID{Flags: AdjSIDFlags{V: true, L: true}, Weight: 1, Label: 40001}
	v := EncodeAdjSIDValue(in)
	out, err := DecodeAdjSIDValue(v)
	if err != nil {
		t.Fatalf("adj-SID decode: %v", err)
	}
	if !out.IsLabel || out.Label != 40001 || out.Weight != 1 || !out.Flags.V || !out.Flags.L {
		t.Fatalf("adj-SID round-trip = %+v", out)
	}
}

func TestLANAdjSIDSubTLVRoundTrip(t *testing.T) {
	nbr := [4]byte{10, 0, 0, 2}
	in := AdjSID{Flags: AdjSIDFlags{V: true, L: true}, Weight: 0, Label: 40002, NeighborID: nbr}
	v := EncodeLANAdjSIDValue(in)
	out, err := DecodeLANAdjSIDValue(v)
	if err != nil {
		t.Fatalf("lan-adj-SID decode: %v", err)
	}
	if out.NeighborID != nbr || out.Label != 40002 || !out.IsLabel {
		t.Fatalf("lan-adj-SID round-trip = %+v", out)
	}
}

func TestExtPrefixRangeTLVRoundTrip(t *testing.T) {
	pfx := [4]byte{10, 1, 0, 0}
	v := EncodeExtPrefixRangeValueV4(24, pfx, 8, true, PrefixSID{Flags: SIDFlags{NP: true}, Index: 100})
	rng, err := DecodeExtPrefixRangeValueV4(v)
	if err != nil {
		t.Fatalf("ext-prefix-range decode: %v", err)
	}
	if rng.PrefixLength != 24 || rng.RangeSize != 8 || !rng.IAFlag || rng.Address != pfx {
		t.Fatalf("ext-prefix-range round-trip = %+v", rng)
	}
	if len(rng.PrefixSIDs) != 1 || rng.PrefixSIDs[0].Index != 100 {
		t.Fatalf("ext-prefix-range sub-SID lost: %+v", rng.PrefixSIDs)
	}
}

func TestSRParserMalformed(t *testing.T) {
	// Truncated / oversize inputs must never panic; an error is acceptable.
	inputs := [][]byte{
		nil,
		{},
		{0x01},
		{0, 0, 0},
		{0, 0, 1, 0, 1, 0},
		{0x40},
		{0x48, 0, 0, 0, 0},
	}
	for i, in := range inputs {
		func() {
			defer func() {
				if r := recover(); r != nil {
					t.Fatalf("input %d panicked: %v", i, r)
				}
			}()
			if _, err := DecodeRangeValue(in); err != nil {
				_ = err
			}
			if _, err := DecodePrefixSIDValue(in); err != nil {
				_ = err
			}
			if _, err := DecodeAdjSIDValue(in); err != nil {
				_ = err
			}
			if _, err := DecodeLANAdjSIDValue(in); err != nil {
				_ = err
			}
			if _, err := DecodeAlgorithmValue(in); err != nil {
				_ = err
			}
			if _, err := DecodeSRMSValue(in); err != nil {
				_ = err
			}
		}()
	}
}

func TestPrefixSIDValueBytesStable(t *testing.T) {
	in := PrefixSID{Flags: SIDFlags{NP: true}, Algorithm: 0, Index: 7}
	if !bytes.Equal(EncodePrefixSIDValue(in), EncodePrefixSIDValue(in)) {
		t.Fatalf("prefix-SID encoding not deterministic")
	}
}
