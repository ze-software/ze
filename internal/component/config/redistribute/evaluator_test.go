package redistribute

import (
	"testing"

	"codeberg.org/thomas-mangin/ze/internal/core/family"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestEvaluatorAccept verifies the evaluator delegates to Evaluate correctly.
//
// VALIDATES: Evaluator.Accept applies rules with loop prevention.
// PREVENTS: Rules not loaded or not checked.
func TestEvaluatorAccept(t *testing.T) {
	ev := NewEvaluator([]ImportRule{
		{Source: "ospf"},
		{Source: "ebgp", Families: []family.Family{family.IPv4Unicast}},
	})

	// OSPF route accepted into BGP
	assert.True(t, ev.Accept(RedistRoute{Origin: "ospf", Family: family.IPv4Unicast, Source: "ospf"}, "bgp"))

	// eBGP route blocked from BGP (loop)
	assert.False(t, ev.Accept(RedistRoute{Origin: "bgp", Family: family.IPv4Unicast, Source: "ebgp"}, "bgp"))

	// eBGP ipv6 blocked by family filter
	assert.False(t, ev.Accept(RedistRoute{Origin: "bgp", Family: family.IPv6Unicast, Source: "ebgp"}, "ospf"))

	// eBGP ipv4 accepted into OSPF
	assert.True(t, ev.Accept(RedistRoute{Origin: "bgp", Family: family.IPv4Unicast, Source: "ebgp"}, "ospf"))
}

// TestEvaluatorReload verifies rules are replaced on reload.
//
// VALIDATES: Reload swaps rules; old rules no longer apply.
// PREVENTS: Stale rules persisting after config reload.
func TestEvaluatorReload(t *testing.T) {
	ev := NewEvaluator([]ImportRule{
		{Source: "ospf"},
	})

	route := RedistRoute{Origin: "ospf", Family: family.IPv4Unicast, Source: "ospf"}
	assert.True(t, ev.Accept(route, "bgp"))

	// Reload with no rules
	ev.Reload(nil)
	assert.False(t, ev.Accept(route, "bgp"))

	// Reload with different rules
	ev.Reload([]ImportRule{
		{Source: "connected"},
	})
	assert.False(t, ev.Accept(route, "bgp"))
	assert.True(t, ev.Accept(RedistRoute{Origin: "connected", Family: family.IPv4Unicast, Source: "connected"}, "bgp"))
}

// TestEvaluatorRules verifies Rules returns a copy.
//
// VALIDATES: Rules returns a snapshot that doesn't mutate the evaluator.
// PREVENTS: Caller mutating internal state.
func TestEvaluatorRules(t *testing.T) {
	ev := NewEvaluator([]ImportRule{
		{Source: "ospf"},
		{Source: "ebgp"},
	})

	rules := ev.Rules()
	require.Len(t, rules, 2)
	assert.Equal(t, "ospf", rules[0].Source)
	assert.Equal(t, "ebgp", rules[1].Source)

	// Mutating the copy should not affect the evaluator
	rules[0].Source = "modified"
	assert.Equal(t, "ospf", ev.Rules()[0].Source)

	// Mutating Families slice should not affect the evaluator
	ipv4VPN := family.Family{AFI: family.AFIIPv4, SAFI: family.SAFIVPN}
	ev2 := NewEvaluator([]ImportRule{
		{Source: "ebgp", Families: []family.Family{family.IPv4Unicast, ipv4VPN}},
	})
	r2 := ev2.Rules()
	r2[0].Families[0] = family.Family{}
	assert.Equal(t, family.IPv4Unicast, ev2.Rules()[0].Families[0])
}

// TestEvaluatorNilRules verifies evaluator works with nil rules.
//
// VALIDATES: NewEvaluator(nil) and Reload(nil) produce a working evaluator that rejects all.
// PREVENTS: Nil pointer dereference on empty config.
func TestEvaluatorNilRules(t *testing.T) {
	ev := NewEvaluator(nil)
	assert.False(t, ev.Accept(RedistRoute{Origin: "ospf", Family: family.IPv4Unicast, Source: "ospf"}, "bgp"))
	assert.Empty(t, ev.Rules())
}
