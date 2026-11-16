// Design: docs/architecture/update-building.md -- family decisions precede next-hop scope
// Related: flowspec_redirect_egress_test.go -- forwarding to the recipient socket
// Related: next_hop_scope_materialized_field_test.go -- forwarding-family Set selection
package reactor

import (
	"bytes"
	"net/netip"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/bgp/filterapi"
	"github.com/ze-software/ze/internal/core/bgp/attribute"
	"github.com/ze-software/ze/internal/core/bgp/wire"
	"github.com/ze-software/ze/internal/core/family"
)

// TestNextHopScopePreservesFlowSpecFamilyDecision applies family normalization
// before scope, in the order both forwarding rails use, then materializes the
// accumulated edits. An ignored pair must stay empty; single redirect targets
// and useful ordinary/unknown-family scope transformations must still survive.
func TestNextHopScopePreservesFlowSpecFamilyDecision(t *testing.T) {
	global := netip.MustParseAddr("2001:db8::77").AsSlice()
	pair := append(bytes.Clone(global), netip.MustParseAddr("fe80::77").AsSlice()...)
	ipv4 := netip.MustParseAddr("192.0.2.77").AsSlice()
	settings := NewPeerSettings(netip.MustParseAddr("192.0.2.90"), 65000, 65000, 1)
	settings.NextHopMode = NextHopUnchanged
	peer := NewPeer(settings)
	peer.refreshForwardFacts()
	facts := peer.forwardFacts()
	require.NotNil(t, facts)
	peer.llScope.Store(nil)

	for _, tc := range []struct {
		name   string
		family family.Family
		nlri   string
		hop    []byte
		want   []byte
	}{
		{"flow-ignored-pair", flowRedirectIPv4, "050118c00002", pair, nil},
		{"flow-vpn-ignored-pair", family.Family{AFI: family.AFIIPv4, SAFI: family.SAFIFlowSpecVPN},
			"0d0000fdea000000010118c00002", pair, nil},
		{"redirect-ipv4", flowRedirectIPv4, "050118c00002", ipv4, ipv4},
		{"redirect-ipv6", flowRedirectIPv4, "050118c00002", global, global},
		{"ordinary-off-link-pair", family.Family{AFI: family.AFIIPv6, SAFI: family.SAFIUnicast},
			"3020010db80001", pair, global},
		{"unknown-profile-off-link-pair", family.Family{AFI: family.AFI(65000), SAFI: family.SAFI(250)},
			"18cb0071", pair, global},
	} {
		t.Run(tc.name, func(t *testing.T) {
			raw := mustHex(t, tc.nlri)
			body := flowRedirectPayloadWith(t, tc.family, raw, tc.hop, flowRedirectAction)
			original := bytes.Clone(body)
			base := payloadNextHop(body)
			require.Equal(t, tc.family, base.mpFamily)
			var mods filterapi.ModAccumulator
			applyNextHopFamily(facts, &mods, base, body)
			applyEgressNextHopScope(peer, facts, &mods, body, base.mpFamily)
			output, _, failed := buildModifiedPayload(body, &mods, attrModHandlersWithDefaults(), nil, nil)
			require.Equal(t, modifyFailureNone, failed)
			if output == nil {
				output = body
			}
			sections, err := wire.ParseUpdateSections(output)
			require.NoError(t, err)
			_, _, reach, found := attribute.AttrFind(sections.Attrs(output), attribute.AttrMPReachNLRI)
			require.True(t, found, "the useful route must remain an announcement")
			want := []byte{byte(tc.family.AFI >> 8), byte(tc.family.AFI), byte(tc.family.SAFI), byte(len(tc.want))}
			want = append(want, tc.want...)
			want = append(want, 0)
			want = append(want, raw...)
			require.Equal(t, want, reach, "scope must preserve the family's final next-hop decision and exact NLRI")
			require.Equal(t, original, body, "normalization must not rewrite the borrowed input")
		})
	}
}
