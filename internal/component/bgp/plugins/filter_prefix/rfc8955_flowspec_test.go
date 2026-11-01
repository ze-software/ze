package filter_prefix

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/bgp/policychain"
	"github.com/ze-software/ze/internal/component/bgp/policyview"
	"github.com/ze-software/ze/internal/core/family"
)

// RFC requirement: RFC8955-3-1 positive -- the configured unicast prefix/ge/le policy accepts FlowSpec destinations in the same permitted range (§3).
// RFC requirement: RFC8955-3-1 negative -- a denied, missing, or mixed permitted/denied FlowSpec destination cannot bypass prefix policy or become a unicast rewrite (§3).
//
// Method: a match whose prefix-list condition names the protected list
// (10.0.0.0/8 ge 16 le 24 accept, matchChain) judges each FlowSpec UPDATE at a
// BGP chain; an accepted rule drops nothing from its MP_REACH_NLRI and a
// refused one rejects the whole UPDATE, so no destination projection
// rewrites a FlowSpec rule.
func TestPrefixPolicyFiltersFlowSpecDestination(t *testing.T) {
	family.RegisterTestFamilies()
	destination := func(bits byte, octets ...byte) []byte { return append([]byte{1, bits}, octets...) }
	source := []byte{2, 24, 10, 1, 0} // a source prefix component only
	chain := matchChain(t)
	for _, tc := range []struct {
		name  string
		safi  uint8
		rules []byte
		want  policychain.OutcomeAction
	}{
		{"ipv4/flow 10.1.0.0/24", 133, flowRule(false, destination(24, 10, 1, 0)...), policychain.OutcomeAccept},
		{"ipv4/flow 10.1.0.0/25", 133, flowRule(false, destination(25, 10, 1, 0, 0)...), policychain.OutcomeReject},
		{"ipv4/flow 192.0.2.0/24", 133, flowRule(false, destination(24, 192, 0, 2)...), policychain.OutcomeReject},
		{"ipv4/flow no destination", 133, flowRule(false, source...), policychain.OutcomeReject},
		{"ipv4/flow 10.1.0.0/24 and 192.0.2.0/24", 133, append(flowRule(false, destination(24, 10, 1, 0)...),
			flowRule(false, destination(24, 192, 0, 2)...)...), policychain.OutcomeReject},
		{"ipv4/flow-vpn 10.1.0.0/24", 134, flowRule(true, destination(24, 10, 1, 0)...), policychain.OutcomeAccept},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var view policyview.View
			view.Reset(bgpUpdate(t, nil, mpReach(1, tc.safi, nil, tc.rules), nil), policyview.Session{})
			defer view.Clear()
			sets := new(policychain.RunSets)
			outcome := chain.Run(&view, sets)
			require.False(t, outcome.Failed, "the chain failed the route: %+v", outcome)
			require.Equal(t, tc.want, outcome.Action)
			if tc.want == policychain.OutcomeAccept {
				require.Empty(t, sets.Folded().NLRIDrops(policychain.NLRISectionMPReach),
					"FlowSpec components are never replaced by a destination projection")
			}
		})
	}
}
