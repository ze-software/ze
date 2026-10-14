// RFC: rfc/short/rfc9252.md -- Section 7, only the first instance of each Service TLV type
// Related: srv6sid.go -- ExtractSRv6SIDFull, the reader under test

package pool

import (
	"net/netip"
	"slices"
	"testing"
)

// firstInstanceWithoutSID returns the three shapes of a first Service TLV
// instance that yields no valid SID: a SID Information Sub-TLV of 20 octets,
// no SID Information Sub-TLV at all, and a SID Information whose SID
// Structure fails Section 3.2.1.
func firstInstanceWithoutSID(tlvType byte) map[string][]byte {
	short := append([]byte{1, 0, 20}, make([]byte, 20)...)
	invalid := buildSIDInfoSubTLVWithStructure(netip.MustParseAddr("2001:db8::99").As16(), 64, 64, 64, 0, 0, 0)
	return map[string][]byte{
		"sid information of 20 octets": buildServiceTLV(tlvType, short),
		"no sid information":           buildServiceTLV(tlvType, nil),
		"invalid sid structure":        buildServiceTLV(tlvType, invalid),
	}
}

// TestRFC9252SecondL3ServiceTLVIgnoredWhenFirstHasNoSID pins that a second
// L3 Service TLV is ignored even when the first carries no valid SID.
//
// VALIDATES: RFC 9252 Section 7: "If multiple instances of the SRv6 L3
// Service TLV are encountered, all but the first instance MUST be ignored."
// PREVENTS: the reader falling back to the second instance.
//
// RFC requirement: RFC9252-7-1 negative -- when the first SRv6 L3 Service TLV yields no valid SID, the second instance's SID is not used: no SID is answered.
func TestRFC9252SecondL3ServiceTLVIgnoredWhenFirstHasNoSID(t *testing.T) {
	second := netip.MustParseAddr("2001:db8::2")
	for name, first := range firstInstanceWithoutSID(tlvTypeSRv6L3Service) {
		attr := append(slices.Clone(first), buildServiceTLV(tlvTypeSRv6L3Service, buildSIDInfoSubTLV(second.As16()))...)
		if got := ExtractSRv6SIDFull(attr); got.SID.IsValid() {
			t.Errorf("%s: SID = %v, want none: the second L3 instance MUST be ignored", name, got.SID)
		}
	}
}

// TestRFC9252SecondL2ServiceTLVIgnoredWhenFirstHasNoSID is the L2 half.
//
// VALIDATES: RFC 9252 Section 7: "If multiple instances of the SRv6 L2
// Service TLV are encountered, all but the first instance MUST be ignored."
// PREVENTS: the reader falling back to the second instance.
//
// RFC requirement: RFC9252-7-2 negative -- when the first SRv6 L2 Service TLV yields no valid SID, the second instance's SID is not used: no SID is answered.
func TestRFC9252SecondL2ServiceTLVIgnoredWhenFirstHasNoSID(t *testing.T) {
	second := netip.MustParseAddr("2001:db8::2")
	for name, first := range firstInstanceWithoutSID(tlvTypeSRv6L2Service) {
		attr := append(slices.Clone(first), buildServiceTLV(tlvTypeSRv6L2Service, buildSIDInfoSubTLV(second.As16()))...)
		if got := ExtractSRv6SIDFull(attr); got.SID.IsValid() {
			t.Errorf("%s: SID = %v, want none: the second L2 instance MUST be ignored", name, got.SID)
		}
	}
}

// TestRFC9252FirstL3AndFirstL2BothConsulted guards today's cross-type read:
// an L3 instance with no SID then an L2 instance with one answers the L2 SID.
// The Section 7 sentences each bind one type, so this carries no tag.
func TestRFC9252FirstL3AndFirstL2BothConsulted(t *testing.T) {
	l2 := netip.MustParseAddr("2001:db8::6")
	attr := append(buildServiceTLV(tlvTypeSRv6L3Service, nil), buildServiceTLV(tlvTypeSRv6L2Service, buildSIDInfoSubTLV(l2.As16()))...)
	if got := ExtractSRv6SIDFull(attr); got.SID != l2 {
		t.Errorf("SID = %v, want the first L2 instance's %v", got.SID, l2)
	}
}
