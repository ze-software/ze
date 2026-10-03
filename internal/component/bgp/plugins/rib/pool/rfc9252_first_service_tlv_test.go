package pool

import (
	"net/netip"
	"testing"
)

// RFC 9252 Section 7: "If multiple instances of the SRv6 L3 Service TLV are encountered,
// all but the first instance MUST be ignored", and the same for the L2 Service TLV.
// ExtractSRv6SIDFull walks the Prefix-SID attribute in order and returns the SID of the
// first Service TLV that carries one.

// TestRFC9252FirstServiceTLVWins pins which instance the SID comes from.
//
// VALIDATES: RFC9252-7-1 and RFC9252-7-2, with two instances of one Service TLV type the
// first instance's SID is the one used and the second's never is.
// PREVENTS: a later instance overriding the first, or the last one winning.
func TestRFC9252FirstServiceTLVWins(t *testing.T) {
	cases := []struct {
		name    string
		tlvType byte
	}{
		{"l3 service tlv", tlvTypeSRv6L3Service},
		{"l2 service tlv", tlvTypeSRv6L2Service},
	}
	first := netip.MustParseAddr("2001:db8::1")
	second := netip.MustParseAddr("2001:db8::2")

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			// RFC requirement: RFC9252-7-1 positive -- with two SRv6 L3 Service TLVs each carrying a SID Information Sub-TLV, the SID used is the first instance's (§7).
			// RFC requirement: RFC9252-7-1 negative -- the second SRv6 L3 Service TLV instance's SID is never the one used (§7).
			// RFC requirement: RFC9252-7-2 positive -- with two SRv6 L2 Service TLVs each carrying a SID Information Sub-TLV, the SID used is the first instance's (§7).
			// RFC requirement: RFC9252-7-2 negative -- the second SRv6 L2 Service TLV instance's SID is never the one used (§7).
			attr := append(
				buildServiceTLV(tc.tlvType, buildSIDInfoSubTLV(first.As16())),
				buildServiceTLV(tc.tlvType, buildSIDInfoSubTLV(second.As16()))...,
			)

			got := ExtractSRv6SIDFull(attr)
			if got.SID != first {
				t.Fatalf("ExtractSRv6SIDFull() SID = %v, want the first instance %v", got.SID, first)
			}
			if got.SID == second {
				t.Fatalf("the second instance %v was used; all but the first MUST be ignored", second)
			}
		})
	}
}
