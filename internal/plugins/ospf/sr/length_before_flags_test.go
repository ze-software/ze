// VALIDATES: every RFC 8665 / RFC 8666 SR decoder judges the length of a TLV or sub-TLV
// before any other field. A value whose length is invalid returns ErrLength, the error
// that condemns the carrying LSA (RFC 8665 Section 9, RFC 8666 Section 10), even when
// its V/L-Flags or its Range Size are invalid too. A value whose length is valid and
// whose V/L-Flags alone are invalid returns ErrMalformed and not ErrLength, so only the
// sub-TLV is ignored.
// PREVENTS: a sub-TLV carrying two defects, a bad length and bad flags, being reported
// as a flag error, so the LSA it sits in is still applied.
package sr

import (
	"errors"
	"testing"
)

// srLengthCase is one decoder applied to one value, with the verdict owed.
type srLengthCase struct {
	name       string
	decode     func([]byte) error
	value      []byte
	lengthOnly bool // true: ErrLength owed; false: ErrMalformed owed and ErrLength refused
}

func srLengthCases() []srLengthCase {
	prefix := func(v []byte) error { _, err := DecodePrefixSIDValue(v); return err }
	adj := func(v []byte) error { _, err := DecodeAdjSIDValue(v); return err }
	lan := func(v []byte) error { _, err := DecodeLANAdjSIDValue(v); return err }
	prefix6 := func(v []byte) error { _, err := DecodePrefixSIDValueV6(v); return err }
	adj6 := func(v []byte) error { _, err := DecodeAdjSIDValueV6(v); return err }
	lan6 := func(v []byte) error { _, err := DecodeLANAdjSIDValueV6(v); return err }
	range4 := func(v []byte) error { _, err := DecodeExtPrefixRangeValueV4(v); return err }
	range6 := func(v []byte) error { _, err := DecodeExtPrefixRangeValueV6(v); return err }
	srgb := func(v []byte) error { _, err := DecodeRangeValue(v); return err }

	// V set, L clear: invalid V/L-Flags. The encoders size the SID by V AND L, so these
	// carry a 4-octet SID where the V-Flag makes the length 3 octets shorter: both the
	// length and the flags are invalid.
	vOnly := PrefixSID{Flags: SIDFlags{V: true}, Index: 7}
	vOnlyAdj := AdjSID{Flags: AdjSIDFlags{V: true}, Index: 7}
	// L set, V clear: invalid V/L-Flags with a length the V-Flag allows.
	lOnly := PrefixSID{Flags: SIDFlags{L: true}, Index: 7}
	lOnlyAdj := AdjSID{Flags: AdjSIDFlags{L: true}, Index: 7}

	// A range whose first Prefix-SID has only its flags wrong and whose second has its
	// length wrong: the length error behind the flag error still condemns the LSA.
	rangeFlagThenLength := append(EncodeExtPrefixRangeValueV4(32, [4]byte{10, 0, 1, 0}, 4, false, lOnly),
		writeSubTLV(V4TypePrefixSID, append(EncodePrefixSIDValue(PrefixSID{Index: 8}), 0))...)
	rangeFlagThenLength6 := append(EncodeExtPrefixRangeValueV6(128, make([]byte, 16), 4, lOnly),
		writeSubTLV(V6TypePrefixSID, append(EncodePrefixSIDValueV6(PrefixSID{Index: 8}), 0))...)
	// A SID/Label Range of Range Size 0 whose SID/Label sub-TLV is 5 octets long.
	srgbZeroSizeBadSub := append([]byte{0, 0, 0, 0}, writeSubTLV(1, []byte{0, 0, 0x3e, 0x80, 0})...)

	return []srLengthCase{
		{"prefix-sid length+flags", prefix, EncodePrefixSIDValue(vOnly), true},
		{"prefix-sid too short", prefix, []byte{0, 0, 0}, true},
		{"prefix-sid flags only", prefix, EncodePrefixSIDValue(lOnly), false},
		{"adj-sid length+flags", adj, EncodeAdjSIDValue(vOnlyAdj), true},
		{"adj-sid too short", adj, []byte{0, 0, 0}, true},
		{"adj-sid flags only", adj, EncodeAdjSIDValue(lOnlyAdj), false},
		{"lan-adj-sid length+flags", lan, EncodeLANAdjSIDValue(vOnlyAdj), true},
		{"lan-adj-sid too short", lan, []byte{0, 0, 0, 0, 0, 0, 0}, true},
		{"lan-adj-sid flags only", lan, EncodeLANAdjSIDValue(lOnlyAdj), false},
		{"v6 prefix-sid length+flags", prefix6, EncodePrefixSIDValueV6(vOnly), true},
		{"v6 prefix-sid too short", prefix6, []byte{0, 0, 0}, true},
		{"v6 prefix-sid flags only", prefix6, EncodePrefixSIDValueV6(lOnly), false},
		{"v6 adj-sid length+flags", adj6, EncodeAdjSIDValueV6(vOnlyAdj), true},
		{"v6 adj-sid too short", adj6, []byte{0, 0, 0}, true},
		{"v6 adj-sid flags only", adj6, EncodeAdjSIDValueV6(lOnlyAdj), false},
		{"v6 lan-adj-sid length+flags", lan6, EncodeLANAdjSIDValueV6(vOnlyAdj), true},
		{"v6 lan-adj-sid too short", lan6, []byte{0, 0, 0, 0, 0, 0, 0}, true},
		{"v6 lan-adj-sid flags only", lan6, EncodeLANAdjSIDValueV6(lOnlyAdj), false},
		{"range nested length+flags", range4, EncodeExtPrefixRangeValueV4(32, [4]byte{10, 0, 1, 0}, 4, false, vOnly), true},
		{"range flag error then length error", range4, rangeFlagThenLength, true},
		{"range nested flags only", range4, EncodeExtPrefixRangeValueV4(32, [4]byte{10, 0, 1, 0}, 4, false, lOnly), false},
		{"v6 range nested length+flags", range6, EncodeExtPrefixRangeValueV6(128, make([]byte, 16), 4, vOnly), true},
		{"v6 range flag error then length error", range6, rangeFlagThenLength6, true},
		{"v6 range nested flags only", range6, EncodeExtPrefixRangeValueV6(128, make([]byte, 16), 4, lOnly), false},
		{"srgb zero size and bad sub-TLV length", srgb, srgbZeroSizeBadSub, true},
		{"srgb zero size only", srgb, append([]byte{0, 0, 0, 0}, encodeSIDLabelSubTLV(true, 16000)...), false},
	}
}

// TestSRLengthJudgedBeforeFlags decodes each case and checks the error class it returns.
func TestSRLengthJudgedBeforeFlags(t *testing.T) {
	for _, c := range srLengthCases() {
		err := c.decode(c.value)
		if c.lengthOnly {
			if !errors.Is(err, ErrLength) {
				t.Errorf("%s: an invalid length must return ErrLength, got %v", c.name, err)
			}
			continue
		}
		if !errors.Is(err, ErrMalformed) {
			t.Errorf("%s: an invalid field must return ErrMalformed, got %v", c.name, err)
		}
		if errors.Is(err, ErrLength) {
			t.Errorf("%s: a valid length must not return ErrLength, got %v", c.name, err)
		}
	}
}
