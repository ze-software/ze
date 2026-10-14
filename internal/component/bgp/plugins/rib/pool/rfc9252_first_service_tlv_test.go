package pool

import (
	"net/netip"
	"testing"
)

// RFC 9252 Section 7: "If multiple instances of the SRv6 L3 Service TLV are encountered,
// all but the first instance MUST be ignored", and the same for the L2 Service TLV.
// ExtractSRv6SIDFull reads only the first instance of each Service TLV type; the case of
// a first instance that carries no valid SID is in rfc9252_service_tlv_first_instance_test.go.

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
			// RFC requirement: RFC9252-7-1 negative -- with a valid first instance, the second SRv6 L3 Service TLV instance's SID is not used (§7).
			// RFC requirement: RFC9252-7-2 positive -- with two SRv6 L2 Service TLVs each carrying a SID Information Sub-TLV, the SID used is the first instance's (§7).
			// RFC requirement: RFC9252-7-2 negative -- with a valid first instance, the second SRv6 L2 Service TLV instance's SID is not used (§7).
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
