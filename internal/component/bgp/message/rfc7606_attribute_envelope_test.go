package message

import (
	"testing"

	"github.com/stretchr/testify/require"
)

// RFC requirement: RFC7606-5.2-1 negative -- an empty MP_REACH does not prevent
// reset for a malformed ORIGIN; actual reachable NLRI permits withdrawal and
// attribute-discard-only errors remain exempt from escalation.
func TestRFC7606EmptyMPReachEscalatesToReset(t *testing.T) {
	for _, tc := range []struct {
		name       string
		mpNLRI     []byte
		legacyNLRI bool
		origin     byte
		extra      []byte
		want       RFC7606Action
	}{
		{"empty_bad_origin", nil, false, 3, nil, RFC7606ActionSessionReset},
		{"reachable_bad_origin", []byte{24, 192, 0, 2}, false, 3, nil, RFC7606ActionTreatAsWithdraw},
		{"legacy_bad_origin", nil, true, 3, nil, RFC7606ActionTreatAsWithdraw},
		{"empty_valid", nil, false, 0, nil, RFC7606ActionNone},
		{"empty_discard_only", nil, false, 0, optAttr(0xc0, 7, []byte{0}), RFC7606ActionAttributeDiscard},
		{"empty_abandoned_walk", nil, false, 0, []byte{0xc0, 8, 4, 0}, RFC7606ActionSessionReset},
		{"reachable_abandoned_walk", []byte{24, 192, 0, 2}, false, 0, []byte{0xc0, 8, 4, 0}, RFC7606ActionTreatAsWithdraw},
	} {
		t.Run(tc.name, func(t *testing.T) {
			// RFC 4760 Section 3: AFI(2), SAFI(1), next-hop length(1),
			// next hop(4), reserved(1), then the reachable NLRI bytes.
			mp := append([]byte{0, 1, 1, 4, 192, 0, 2, 1, 0}, tc.mpNLRI...)
			attrs := optAttr(0x80, 14, mp)
			mandatory := append([]byte(nil), rfc7606MandatoryAttrs...)
			mandatory[3] = tc.origin
			attrs = append(attrs, mandatory...)
			attrs = append(attrs, tc.extra...)
			// RFC 7606 Section 5.2: actual reachable contents, not MP presence.
			result := ValidateUpdateRFC7606AddPath(attrs, tc.legacyNLRI, false, false, nil)
			require.Equal(t, tc.want, result.Action, result.Description)
			if tc.origin == 3 {
				require.Equal(t, uint8(1), result.AttrCode)
			}
			if tc.want == RFC7606ActionAttributeDiscard {
				require.Len(t, result.DiscardEntries, 1)
				require.Equal(t, uint8(7), result.DiscardEntries[0].Code)
			}
		})
	}
}

// RFC requirement: RFC7606-3.c-1 negative -- implemented TE and ATTR_SET
// attributes withdraw on either Optional or Transitive conflicts, while their
// correctly flagged, valid-length counterparts are accepted by UPDATE validation.
func TestRFC7606ImplementedAttributeFlagConflicts(t *testing.T) {
	for _, attr := range []struct {
		name  string
		code  byte
		flags byte
		value []byte
	}{
		{"traffic_engineering", 24, 0x80, make([]byte, trafficEngDescriptorLen)},
		{"attr_set", 128, 0xc0, attrSetValue(65000, nil)},
	} {
		t.Run(attr.name, func(t *testing.T) {
			for _, flags := range []struct {
				name string
				xor  byte
				want RFC7606Action
			}{
				{"correct", 0, RFC7606ActionNone},
				{"optional_conflict", 0x80, RFC7606ActionTreatAsWithdraw},
				{"transitive_conflict", 0x40, RFC7606ActionTreatAsWithdraw},
			} {
				t.Run(flags.name, func(t *testing.T) {
					attrs := updateWith(optAttr(attr.flags^flags.xor, attr.code, attr.value))
					// RFC 7606 Section 3(c): exercise registration and validation together.
					result := ValidateUpdateRFC7606AddPath(attrs, true, false, false, nil)
					require.Equal(t, flags.want, result.Action, result.Description)
					if flags.want != RFC7606ActionNone {
						require.Equal(t, attr.code, result.AttrCode)
					}
				})
			}
		})
	}
}

// RFC requirement: RFC7606-7.16-1 negative -- malformed inner AGGREGATOR width
// and ORIGIN flags make ATTR_SET malformed; four-octet-AS values and correct
// flags remain valid independently of the outer session's ASN width.
func TestRFC7606AttrSetMalformedInnerAttributes(t *testing.T) {
	for _, asn4 := range []bool{false, true} {
		name := "outer_asn2"
		if asn4 {
			name = "outer_asn4"
		}
		t.Run(name, func(t *testing.T) {
			for _, tc := range []struct {
				name  string
				inner []byte
				want  RFC7606Action
			}{
				{"aggregator_asn2", optAttr(0xc0, 7, []byte{0xfd, 0xe8, 192, 0, 2, 1}), RFC7606ActionTreatAsWithdraw},
				{"aggregator_asn4", optAttr(0xc0, 7, []byte{0, 0, 0xfd, 0xe8, 192, 0, 2, 1}), RFC7606ActionNone},
				{"origin_optional_conflict", optAttr(0xc0, 1, []byte{0}), RFC7606ActionTreatAsWithdraw},
				{"origin_correct", optAttr(0x40, 1, []byte{0}), RFC7606ActionNone},
				{"as_path_asn4", innerASPath4Octet, RFC7606ActionNone},
				{"unknown_optional", optAttr(0xc0, 99, []byte{1}), RFC7606ActionNone},
				// RFC 6368 Section 5: a framed NEXT_HOP is ignored, not
				// judged using the outer UPDATE's next-hop semantics.
				{"next_hop_ignored", optAttr(0x40, 3, []byte{0, 0, 0, 0}), RFC7606ActionNone},
				{"next_hop_framing_overrun", []byte{0x40, 3, 4, 0}, RFC7606ActionTreatAsWithdraw},
			} {
				t.Run(tc.name, func(t *testing.T) {
					attrs := updateWith(optAttr(0xc0, 128, attrSetValue(65000, tc.inner)))
					// RFC 7606 Section 7.16 retains RFC 6368 Section 5's malformed definition.
					result := ValidateUpdateRFC7606AddPath(attrs, true, false, asn4, nil)
					require.Equal(t, tc.want, result.Action, result.Description)
					if tc.want != RFC7606ActionNone {
						require.Equal(t, uint8(128), result.AttrCode)
					}
				})
			}
		})
	}
}
