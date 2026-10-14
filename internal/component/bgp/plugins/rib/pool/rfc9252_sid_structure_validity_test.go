// RFC: rfc/short/rfc9252.md -- Section 3.2.1 SID Structure conditions, Section 7 invalid SID
// Related: srv6sid.go -- parseSIDStructure, the judge under test

package pool

import (
	"net/netip"
	"testing"
)

// sidInfoWithStructureLength builds a SID Information Sub-TLV whose SID
// Structure Sub-Sub-TLV declares length structLen and carries structLen value
// octets, the first six a valid structure (32, 16, 16, 0, 0, 0).
func sidInfoWithStructureLength(sid [16]byte, structLen int) []byte {
	structure := []byte{1, byte(structLen >> 8), byte(structLen)}
	value := make([]byte, structLen)
	copy(value, []byte{32, 16, 16, 0, 0, 0})
	structure = append(structure, value...)

	info := make([]byte, 0, 21+len(structure))
	info = append(info, 0)
	info = append(info, sid[:]...)
	info = append(info, 0, 0, 0, 0)
	info = append(info, structure...)
	return append([]byte{1, byte(len(info) >> 8), byte(len(info))}, info...)
}

// TestExtractSRv6SIDFull_SIDStructureLengthNotSix pins that a SID Structure
// Sub-Sub-TLV of a length other than 6 makes the SID invalid.
//
// VALIDATES: RFC 9252 Section 3.2.1 ("This field contains a total length of 6
// octets") read through Section 7 ("any of the conditions for the fields of
// the Sub-Sub-TLV, as specified in Section 3.2.1, is not met").
// PREVENTS: a SID installed from a structure ze did not read whole.
//
// RFC requirement: RFC9252-3.2.1-3 negative -- a SID Structure Sub-Sub-TLV whose length is not 6 makes the SID invalid: no SID is answered, while length 6 yields the SID.
func TestExtractSRv6SIDFull_SIDStructureLengthNotSix(t *testing.T) {
	sid := netip.MustParseAddr("2001:db8::1")
	for _, structLen := range []int{5, 7} {
		r := ExtractSRv6SIDFull(buildServiceTLV(tlvTypeSRv6L3Service, sidInfoWithStructureLength(sid.As16(), structLen)))
		if r.SID.IsValid() {
			t.Errorf("structure length %d: SID = %v, want none", structLen, r.SID)
		}
	}
	if r := ExtractSRv6SIDFull(buildServiceTLV(tlvTypeSRv6L3Service, sidInfoWithStructureLength(sid.As16(), 6))); r.SID != sid {
		t.Errorf("structure length 6: SID = %v, want %v", r.SID, sid)
	}
}

// TestExtractSRv6SIDFull_ZeroLengthNonZeroOffset pins the Transposition
// Offset rule for a Transposition Length of 0.
//
// VALIDATES: RFC 9252 Section 3.2.1: "A Transposition Length of 0 indicates
// nothing is transposed and that the entire SRv6 SID value is encoded in the
// SID Information Sub-TLV. In this case, the Transposition Offset MUST be set
// to 0."
// PREVENTS: a structure that contradicts itself being trusted.
//
// RFC requirement: RFC9252-3.2.1-1 negative -- a SID Structure with Transposition Length 0 and a non-zero Transposition Offset makes the SID invalid.
// RFC requirement: RFC9252-3.2.1-1 positive -- a SID Structure with Transposition Length 0 and Transposition Offset 0 keeps the SID valid, with no transposition.
func TestExtractSRv6SIDFull_ZeroLengthNonZeroOffset(t *testing.T) {
	sid := netip.MustParseAddr("2001:db8::1")
	bad := ExtractSRv6SIDFull(buildServiceTLV(tlvTypeSRv6L3Service, buildSIDInfoSubTLVWithStructure(sid.As16(), 32, 16, 16, 0, 0, 48)))
	if bad.SID.IsValid() {
		t.Errorf("length 0, offset 48: SID = %v, want none", bad.SID)
	}
	good := ExtractSRv6SIDFull(buildServiceTLV(tlvTypeSRv6L3Service, buildSIDInfoSubTLVWithStructure(sid.As16(), 32, 16, 16, 0, 0, 0)))
	if good.SID != sid || good.HasTranspos {
		t.Errorf("length 0, offset 0: got SID %v HasTranspos %v, want %v and false", good.SID, good.HasTranspos, sid)
	}
}

// TestExtractSRv6SIDFull_NonZeroArgumentLengthInvalid pins that a SID with
// arguments is ignored: ze knows no SRv6 Endpoint Behavior, so every
// behavior is unknown to it.
//
// VALIDATES: RFC 9252 Section 3.2.1: "A receiver is unable to validate the
// applicability of arguments for SRv6 Endpoint Behaviors that are unknown to
// it and hence MUST ignore SRv6 SIDs with arguments (indicated by a non-zero
// AL) with unknown SRv6 Endpoint Behaviors."
// PREVENTS: ze installing a SID whose arguments it cannot validate.
//
// RFC requirement: RFC9252-3.2-5 negative -- a SID whose SID Structure carries a non-zero Argument Length, under a behavior ze does not know, is ignored: no SID is answered.
// RFC requirement: RFC9252-3.2-5 positive -- the same SID Structure with Argument Length 0 yields the SID.
func TestExtractSRv6SIDFull_NonZeroArgumentLengthInvalid(t *testing.T) {
	sid := netip.MustParseAddr("2001:db8::1")
	withArgs := ExtractSRv6SIDFull(buildServiceTLV(tlvTypeSRv6L3Service, buildSIDInfoSubTLVWithStructure(sid.As16(), 32, 16, 16, 8, 0, 0)))
	if withArgs.SID.IsValid() {
		t.Errorf("AL 8: SID = %v, want none", withArgs.SID)
	}
	noArgs := ExtractSRv6SIDFull(buildServiceTLV(tlvTypeSRv6L3Service, buildSIDInfoSubTLVWithStructure(sid.As16(), 32, 16, 16, 0, 0, 0)))
	if noArgs.SID != sid {
		t.Errorf("AL 0: SID = %v, want %v", noArgs.SID, sid)
	}
}

// TestExtractSRv6SIDFull_Sum128IsValid pins the upper boundary of the sum.
//
// VALIDATES: RFC 9252 Section 3.2.1: the sum "MUST be less than or equal to
// 128 and greater than [erratum 7817: or equal to] the sum of Transposition
// Offset and Transposition Length". 128 and sum = offset + length are valid.
// PREVENTS: an off-by-one refusing a conforming structure.
//
// RFC requirement: RFC9252-3.2.1-3 positive -- a SID Structure whose sum is exactly 128, or exactly Transposition Offset plus Transposition Length, keeps the SID valid with its transposition.
func TestExtractSRv6SIDFull_Sum128IsValid(t *testing.T) {
	sid := netip.MustParseAddr("2001:db8::")
	cases := []struct {
		name                string
		lbl, lnl, fl        uint8
		length, offset      uint8
		wantLen, wantOffset uint8
	}{
		{"sum 128", 64, 48, 16, 16, 112, 16, 112},
		{"sum equals offset plus length", 32, 16, 16, 16, 48, 16, 48},
	}
	for _, tc := range cases {
		r := ExtractSRv6SIDFull(buildServiceTLV(tlvTypeSRv6L3Service, buildSIDInfoSubTLVWithStructure(sid.As16(), tc.lbl, tc.lnl, tc.fl, 0, tc.length, tc.offset)))
		if r.SID != sid || !r.HasTranspos || r.TransposLen != tc.wantLen || r.TransposOffset != tc.wantOffset {
			t.Errorf("%s: got %+v, want SID %v transposition %d at %d", tc.name, r, sid, tc.wantLen, tc.wantOffset)
		}
	}
	// One above: sum 129 is invalid.
	if r := ExtractSRv6SIDFull(buildServiceTLV(tlvTypeSRv6L3Service, buildSIDInfoSubTLVWithStructure(sid.As16(), 64, 48, 17, 0, 16, 112))); r.SID.IsValid() {
		t.Errorf("sum 129: SID = %v, want none", r.SID)
	}
}
