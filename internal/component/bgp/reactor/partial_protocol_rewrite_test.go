// Design: docs/architecture/bgp/egress-attribute-rules.md -- surviving attribute flags
// Related: forward_prefix_sid.go -- automatic next-hop companion cleanup
// Related: forward_build.go -- the default registered-handler rebuild

package reactor

import (
	"net/netip"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/bgp/filterapi"
	"github.com/ze-software/ze/internal/core/bgp/attribute"
	"github.com/ze-software/ze/internal/core/bgp/wire"
	"github.com/ze-software/ze/internal/core/family"
)

// TestPrefixSIDPartialSurvivesNextHopCleanup drives the real next-hop decision,
// automatic Prefix-SID cleanup and default rebuild, retaining useful unknown TLVs.
// Complete twins, unchanged next hops and legal Service-only suppression remain.
//
// RFC 4271 Section 5: "If a path with a recognized, transitive optional attribute
// is accepted and passed along to other BGP peers and the Partial bit in the
// Attribute Flags octet is set to 1 by some previous AS, it MUST NOT be set back
// to 0 by the current AS."
// This is the automatic cleanup/rebuild boundary, not a Session or socket proof.
func TestPrefixSIDPartialSurvivesNextHopCleanup(t *testing.T) {
	service := rfc9252ReservedServiceTLV(attribute.PrefixSIDTLVSRv6L3Service)
	unknown := []byte{0xf0, 0, 2, 0x77, 0x88}
	mixed := append(append([]byte(nil), service...), unknown...)
	receivedHop := netip.MustParseAddr("192.0.2.1")
	changedHop := netip.MustParseAddr("192.0.2.2")
	for _, flagCase := range []struct {
		name  string
		flags byte
	}{
		{"complete", 0xc0},
		{"partial", 0xe0},
	} {
		for _, tc := range []struct {
			name    string
			value   []byte
			want    []byte
			nextHop netip.Addr
		}{
			{"Service removed and unknown retained", mixed, unknown, changedHop},
			{"unknown only survives cleanup", unknown, unknown, changedHop},
			{"Service only leaves no attribute", service, nil, changedHop},
			{"unchanged next hop retains all TLVs", mixed, mixed, receivedHop},
		} {
			t.Run(flagCase.name+"/"+tc.name, func(t *testing.T) {
				_, err := attribute.ParsePrefixSID(tc.value)
				require.NoError(t, err, "the source TLV envelope is valid")
				payload := partialPrefixSIDPayload(flagCase.flags, tc.value)
				original := append([]byte(nil), payload...)
				settings := NewPeerSettings(netip.MustParseAddr("192.0.2.9"), 65000, 65000, 0x01020304)
				settings.NextHopMode = NextHopExplicit
				settings.NextHopAddress = tc.nextHop
				facts := peerForwardFacts{isEBGP: !settings.IsIBGP()}
				precomputeNextHop(settings, &facts)
				var mods filterapi.ModAccumulator
				applyFactsNextHop(&facts, &mods, family.IPv4Unicast)
				received := nextHopValue{legacy: receivedHop}
				applyEgressPrefixSIDNextHop(&facts, &mods, received, received,
					payloadHasAttr(payload, attribute.AttrPrefixSID))
				result, _, failure := buildModifiedPayload(payload, &mods,
					attrModHandlersWithDefaults(), nil, nil)
				require.Equal(t, modifyFailureNone, failure)
				require.Equal(t, original, payload, "the borrowed input stays unchanged")
				require.NotNil(t, result, "the next-hop operation materializes a useful UPDATE")
				requirePartialPrefixSIDResult(t, result, tc.nextHop, tc.want, flagCase.flags)
			})
		}
	}
}

// partialPrefixSIDPayload supplies a valid canonical path and a Prefix-SID whose
// TLV bytes come from the existing propagation fixture, without running its rails.
func partialPrefixSIDPayload(flags byte, value []byte) []byte {
	attrs := collapseAttr(0x40, 1, []byte{0})
	attrs = append(attrs, collapseAttr(0x40, 2, []byte{2, 1, 0, 0, 0xfd, 0xe9})...)
	attrs = append(attrs, collapseAttr(0x40, 3, []byte{192, 0, 2, 1})...)
	attrs = append(attrs, collapseAttr(0x80, 4, []byte{0, 0, 0, 10})...)
	attrs = append(attrs, collapseAttr(flags, byte(attribute.AttrPrefixSID), value)...)
	attrs = append(attrs, collapseAttr(0xe0, 250, []byte{0x42})...)
	return makeUpdateBody(nil, attrs, []byte{24, 203, 0, 113})
}

// requirePartialPrefixSIDResult checks the actual next hop, retained route and
// TLV bytes before the final flag assertion; legitimate empty-result removal wins.
func requirePartialPrefixSIDResult(t *testing.T, payload []byte, nextHop netip.Addr, value []byte, flags byte) {
	t.Helper()
	sections, err := wire.ParseUpdateSections(payload)
	require.NoError(t, err)
	require.Empty(t, sections.Withdrawn(payload))
	require.Equal(t, []byte{24, 203, 0, 113}, sections.NLRI(payload))
	attrs := sections.Attrs(payload)
	for _, control := range []struct {
		code  attribute.AttributeCode
		flags byte
		value []byte
	}{
		{attribute.AttrOrigin, 0x40, []byte{0}},
		{attribute.AttrASPath, 0x40, []byte{2, 1, 0, 0, 0xfd, 0xe9}},
		{attribute.AttrNextHop, 0x40, nextHop.AsSlice()},
		{attribute.AttrMED, 0x80, []byte{0, 0, 0, 10}},
		{attribute.AttributeCode(250), 0xe0, []byte{0x42}},
	} {
		_, gotFlags, gotValue, found := attribute.AttrFind(attrs, control.code)
		require.True(t, found, "useful attribute %d survives", control.code)
		require.Equal(t, control.value, gotValue, "attribute %d value", control.code)
		require.Equal(t, control.flags, byte(gotFlags), "attribute %d flags", control.code)
	}
	_, gotFlags, gotValue, found := attribute.AttrFind(attrs, attribute.AttrPrefixSID)
	if value == nil {
		require.False(t, found, "Service-only cleanup may remove the whole attribute")
		return
	}
	require.True(t, found, "useful non-Service TLVs remain")
	require.Equal(t, value, gotValue, "retained TLVs remain byte-identical")
	require.Equal(t, flags, byte(gotFlags), "the surviving Prefix-SID retains its Partial bit")
}
