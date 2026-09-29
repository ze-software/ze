// Design: docs/architecture/wire/messages.md -- RFC 4271 receive error handling
// Related: session_core4271_test.go -- the ORIGIN flags, NEXT_HOP and MED cases of the same rules

package reactor

import (
	"net/netip"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/core/bgp/attribute"
)

// recognizedAttrOutcome names what RFC 7606 makes of one malformed recognized attribute.
type recognizedAttrOutcome int

const (
	// recognizedAttrOutcomeUnspecified keeps the zero value from naming an outcome, so a
	// case that forgets its outcome fails rather than passing as a discard.
	recognizedAttrOutcomeUnspecified recognizedAttrOutcome = iota //nolint:unused // Zero value reserved as Unspecified; no case names it.
	// recognizedAttrWithdraw is "treat-as-withdraw": the prefix is withdrawn.
	recognizedAttrWithdraw
	// recognizedAttrDiscard is "attribute discard": the route stays, the attribute goes.
	recognizedAttrDiscard
)

// TestRFC4271RecognizedAttributeErrorsBeyondTheFirst drives the RFC 4271 Section 6.3
// flag, length and optional-value checks for recognized attributes other than the
// cases TestSessionRFC4271RevisedAttributeErrors covers (ORIGIN flags, NEXT_HOP
// length, MED length).
//
// Method: one eBGP session per case receives three UPDATEs for one prefix over the
// real receive path: the attribute correctly encoded, then malformed, then correctly
// encoded again. RFC 7606 revises the RFC 4271 NOTIFICATION into two outcomes, and
// each case names the one its section mandates:
//
//   - RFC 7606 Section 3(c): "If the value of either the Optional or Transitive bits
//     in the Attribute Flags is in conflict with their specified values, then the
//     attribute MUST be treated as malformed and the "treat-as-withdraw" approach
//     used, unless the specification for the attribute mandates different handling
//     for incorrect Attribute Flags."
//   - RFC 7606 Sections 7.1 (ORIGIN), 7.8 (Community) and 7.14 (Extended Community)
//     mandate treat-as-withdraw for a wrong length.
//   - RFC 7606 Sections 7.6 (ATOMIC_AGGREGATE) and 7.7 (AGGREGATOR) mandate
//     "attribute discard" for a wrong length.
//
// Each malformed UPDATE keeps the session Established (firstASReceive fails otherwise),
// so no NOTIFICATION replaces the revised handling. The correctly encoded UPDATE before
// and after must keep the prefix announced and carry the attribute byte for byte.
//
// RFC requirement: RFC4271-6.3-5 positive -- a MULTI_EXIT_DISC, AGGREGATOR or ATOMIC_AGGREGATE whose Optional or Transitive flag conflicts with its type code withdraws the prefix under RFC 7606 Section 3(c) and leaves the session Established.
// RFC requirement: RFC4271-6.3-5 negative -- the same attributes with their specified flags keep the prefix announced with the attribute value unchanged.
// RFC requirement: RFC4271-6.3-6 positive -- a wrong length for ORIGIN withdraws the prefix, and a wrong length for ATOMIC_AGGREGATE or AGGREGATOR discards only that attribute while the prefix stays announced, per RFC 7606 Sections 7.1, 7.6 and 7.7.
// RFC requirement: RFC4271-6.3-6 negative -- ORIGIN, ATOMIC_AGGREGATE and AGGREGATOR at their expected lengths keep the prefix announced with the attribute present.
// RFC requirement: RFC4271-6.3-14 positive -- a recognized optional COMMUNITIES or EXTENDED COMMUNITIES whose value fails its length check withdraws the prefix, and an AGGREGATOR whose value fails its length check is discarded while the prefix stays announced.
// RFC requirement: RFC4271-6.3-14 negative -- COMMUNITIES, EXTENDED COMMUNITIES and AGGREGATOR with a valid value keep the prefix announced with the attribute value unchanged.
func TestRFC4271RecognizedAttributeErrorsBeyondTheFirst(t *testing.T) {
	origin := collapseAttr(0x40, byte(attribute.AttrOrigin), []byte{0})
	path := collapseAttr(0x40, byte(attribute.AttrASPath), []byte{2, 1, 0xfd, 0xea})
	nextHop := collapseAttr(0x40, byte(attribute.AttrNextHop), []byte{192, 0, 2, 254})
	aggregator := []byte{0xfd, 0xea, 192, 0, 2, 1}
	// The session is two-octet (no ASN4), so ingest widens AGGREGATOR's AS to four
	// octets before any consumer reads it (RFC 6793 Section 4.2.3).
	aggregatorDispatched := []byte{0, 0, 0xfd, 0xea, 192, 0, 2, 1}
	community := []byte{0xfd, 0xea, 0, 1}
	extCommunity := []byte{0, 2, 0xfd, 0xea, 0, 0, 0, 1}

	for _, tc := range []struct {
		name    string
		code    attribute.AttributeCode
		value   []byte // The value a consumer reads from the correctly encoded attribute.
		good    []byte // The correctly encoded attribute, nil for ORIGIN (the base carries it).
		bad     []byte // The malformed attribute, sent in place of good.
		outcome recognizedAttrOutcome
	}{
		// RFC4271-6.3-5: flags that conflict with the type code.
		{"MED flags", attribute.AttrMED, []byte{0, 0, 0, 10},
			collapseAttr(0x80, byte(attribute.AttrMED), []byte{0, 0, 0, 10}),
			collapseAttr(0x40, byte(attribute.AttrMED), []byte{0, 0, 0, 10}), recognizedAttrWithdraw},
		{"AGGREGATOR flags", attribute.AttrAggregator, aggregatorDispatched,
			collapseAttr(0xC0, byte(attribute.AttrAggregator), aggregator),
			collapseAttr(0x80, byte(attribute.AttrAggregator), aggregator), recognizedAttrWithdraw},
		{"ATOMIC_AGGREGATE flags", attribute.AttrAtomicAggregate, []byte{},
			collapseAttr(0x40, byte(attribute.AttrAtomicAggregate), nil),
			collapseAttr(0xC0, byte(attribute.AttrAtomicAggregate), nil), recognizedAttrWithdraw},
		// RFC4271-6.3-6: a length that conflicts with the expected length.
		{"ORIGIN length", attribute.AttrOrigin, []byte{0},
			nil,
			collapseAttr(0x40, byte(attribute.AttrOrigin), []byte{0, 0}), recognizedAttrWithdraw},
		{"ATOMIC_AGGREGATE length", attribute.AttrAtomicAggregate, []byte{},
			collapseAttr(0x40, byte(attribute.AttrAtomicAggregate), nil),
			collapseAttr(0x40, byte(attribute.AttrAtomicAggregate), []byte{0}), recognizedAttrDiscard},
		{"AGGREGATOR length", attribute.AttrAggregator, aggregatorDispatched,
			collapseAttr(0xC0, byte(attribute.AttrAggregator), aggregator),
			collapseAttr(0xC0, byte(attribute.AttrAggregator), aggregator[:5]), recognizedAttrDiscard},
		// RFC4271-6.3-14: the value check of a recognized optional attribute.
		{"COMMUNITIES value", attribute.AttrCommunity, community,
			collapseAttr(0xC0, byte(attribute.AttrCommunity), community),
			collapseAttr(0xC0, byte(attribute.AttrCommunity), community[:3]), recognizedAttrWithdraw},
		{"EXTENDED COMMUNITIES value", attribute.AttrExtCommunity, extCommunity,
			collapseAttr(0xC0, byte(attribute.AttrExtCommunity), extCommunity),
			collapseAttr(0xC0, byte(attribute.AttrExtCommunity), extCommunity[:7]), recognizedAttrWithdraw},
	} {
		t.Run(tc.name, func(t *testing.T) {
			s, client := firstASSession(t, NewPeerSettings(netip.MustParseAddr("192.0.2.1"), 65001, 65002, 0x01020301), nil, nil)
			prefix := []byte{24, 203, 0, 113}

			// The ORIGIN case replaces the mandatory ORIGIN itself, so its malformed
			// form takes the base ORIGIN's slot.
			goodAttrs := concatRecognizedAttrs(origin, path, nextHop, tc.good)
			badAttrs := concatRecognizedAttrs(origin, path, nextHop, tc.bad)
			if tc.code == attribute.AttrOrigin {
				badAttrs = concatRecognizedAttrs(tc.bad, path, nextHop)
			}

			for i, attrs := range [][]byte{goodAttrs, badAttrs, goodAttrs} {
				wu := firstASReceive(t, s, client, makeUpdateBody(nil, attrs, prefix))
				malformed := i == 1
				if malformed && tc.outcome == recognizedAttrWithdraw {
					require.Equal(t, makeUpdateBody(prefix, nil, nil), wu.Payload(),
						"a malformed %s must withdraw the prefix and nothing else", tc.name)
					continue
				}
				nlri, err := wu.NLRI()
				require.NoError(t, err)
				require.Equal(t, prefix, nlri, "the prefix must stay announced")
				value, present := collapseAttrValue(t, wu, tc.code)
				if malformed {
					require.Equal(t, recognizedAttrDiscard, tc.outcome)
					require.False(t, present, "a malformed %s must be discarded from the route", tc.name)
					continue
				}
				require.True(t, present, "a correct %s must reach the route", tc.name)
				require.Equal(t, tc.value, value)
			}
		})
	}
}

// concatRecognizedAttrs joins wire-form path attributes into one fresh attribute section.
func concatRecognizedAttrs(parts ...[]byte) []byte {
	var out []byte
	for _, part := range parts {
		out = append(out, part...)
	}
	return out
}
