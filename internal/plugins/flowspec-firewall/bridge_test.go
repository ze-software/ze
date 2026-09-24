package flowspecfirewall

import (
	"net/netip"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/ze-software/ze/internal/component/bgp/plugins/nlri/flowspec"
	"github.com/ze-software/ze/internal/component/firewall"
	"github.com/ze-software/ze/internal/core/bgp/ribevents"
)

// Received-peer lifecycle and authorization are exercised at the RIB producer
// in rib_flowspec_validation_test.go. This consumer only receives selected rules.
func TestSelectedRulePreservesMatchesInBothPacketHooks(t *testing.T) {
	b := testBridge(t)
	b.handleSelected(selectedFixture(t, "10.1.0.0/24", discardTrafficRate,
		flowspec.NewFlowIPProtocolComponent(6), flowspec.NewFlowDestPortComponent(80)))
	tables := b.rules.buildTable()
	require.Len(t, tables, 1)
	require.Len(t, tables[0].Chains, 2)
	hooks := []firewall.ChainHook{}
	for _, chain := range tables[0].Chains {
		hooks = append(hooks, chain.Hook)
		require.Len(t, chain.Terms, 1)
		assert.ElementsMatch(t, []firewall.Match{
			firewall.MatchDestinationAddress{Prefix: netip.MustParsePrefix("10.1.0.0/24")},
			firewall.MatchProtocol{Protocol: "tcp"},
			firewall.MatchDestinationPort{Ranges: []firewall.PortRange{{Lo: 80, Hi: 80}}},
		}, chain.Terms[0].Matches)
		assert.Equal(t, []firewall.Action{firewall.Drop{}}, chain.Terms[0].Actions)
	}
	assert.ElementsMatch(t, []firewall.ChainHook{firewall.HookInput, firewall.HookForward}, hooks)
}

func TestSelectedWithdrawalKeepsOtherRules(t *testing.T) {
	b := testBridge(t)
	first := selectedFixture(t, "10.1.0.0/24", discardTrafficRate)
	second := selectedFixture(t, "10.2.0.0/24", discardTrafficRate)
	b.handleSelected(first)
	b.handleSelected(second)
	require.Len(t, b.rules.buildTable()[0].Chains[0].Terms, 2)
	first.Withdraw = true
	b.handleSelected(first)
	terms := b.rules.buildTable()[0].Chains[0].Terms
	require.Len(t, terms, 1)
	assert.Contains(t, terms[0].Matches, firewall.MatchDestinationAddress{Prefix: netip.MustParsePrefix("10.2.0.0/24")})
	second.Withdraw = true
	b.handleSelected(second)
	assert.Nil(t, b.rules.buildTable())
}

func TestSelectedRuleCapAndUnsupportedRule(t *testing.T) {
	b := testBridge(t)
	b.rules = newRuleMap(1)
	first := selectedFixture(t, "10.1.0.0/24", discardTrafficRate)
	b.handleSelected(first)
	b.handleSelected(selectedFixture(t, "10.2.0.0/24", discardTrafficRate))
	terms := b.rules.buildTable()[0].Chains[0].Terms
	require.Len(t, terms, 1)
	assert.Contains(t, terms[0].Matches, firewall.MatchDestinationAddress{Prefix: netip.MustParsePrefix("10.1.0.0/24")})
	// A rejected replacement removes its previous action, and does not
	// consume the cap forever. This is not a second, independently keyed rule.
	first.ExtendedCommunities = []byte{0x80, 8, 0xfd, 0xe9, 0, 0, 0, 1}
	b.handleSelected(first)
	assert.Nil(t, b.rules.buildTable())
	b.handleSelected(selectedFixture(t, "10.3.0.0/24", discardTrafficRate, flowspec.NewFlowPacketLengthComponent(128)))
	assert.Nil(t, b.rules.buildTable())
	b.handleSelected(&ribevents.FlowSpecChange{Family: flowFamily(), NLRI: []byte{5, 1, 24, 10}})
	assert.Nil(t, b.rules.buildTable(), "malformed selected bytes cannot install a widened rule")
}
