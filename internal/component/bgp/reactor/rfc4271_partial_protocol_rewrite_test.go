// Design: docs/architecture/bgp/egress-attribute-rules.md -- surviving attribute flags
// Related: partial_protocol_rewrite_test.go -- fixture and encoded-result assertions
// Related: forward_prefix_sid.go -- automatic next-hop companion cleanup

package reactor

import (
	"net/netip"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/bgp/filterapi"
	"github.com/ze-software/ze/internal/core/bgp/attribute"
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
	// RFC requirement: RFC4271-5-4 positive -- At the component automatic cleanup and default rebuild boundary, a configured next-hop change preserves received Partial=1 on surviving Prefix-SID attributes and retains non-Service TLVs byte-for-byte, with complete, unchanged-hop and Service-only removal controls.

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
				settings.LocalAddress = tc.nextHop
				settings.ExportPolicy = setExportNextHopSelf(settings.ExportPolicy)
				facts := peerForwardFacts{isEBGP: !settings.IsIBGP(), exportPolicy: settings.ExportPolicy}
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
