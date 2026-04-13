package redistribute

import (
	"testing"

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
		{Source: "ebgp", Families: []string{"ipv4/unicast"}},
	})

	// OSPF route accepted into BGP
	assert.True(t, ev.Accept(RedistRoute{Origin: "ospf", Family: "ipv4/unicast", Source: "ospf"}, "bgp"))

	// eBGP route blocked from BGP (loop)
	assert.False(t, ev.Accept(RedistRoute{Origin: "bgp", Family: "ipv4/unicast", Source: "ebgp"}, "bgp"))

	// eBGP ipv6 blocked by family filter
	assert.False(t, ev.Accept(RedistRoute{Origin: "bgp", Family: "ipv6/unicast", Source: "ebgp"}, "ospf"))

	// eBGP ipv4 accepted into OSPF
	assert.True(t, ev.Accept(RedistRoute{Origin: "bgp", Family: "ipv4/unicast", Source: "ebgp"}, "ospf"))
}

// TestEvaluatorReload verifies rules are replaced on reload.
//
// VALIDATES: Reload swaps rules; old rules no longer apply.
// PREVENTS: Stale rules persisting after config reload.
func TestEvaluatorReload(t *testing.T) {
	ev := NewEvaluator([]ImportRule{
		{Source: "ospf"},
	})

	route := RedistRoute{Origin: "ospf", Family: "ipv4/unicast", Source: "ospf"}
	assert.True(t, ev.Accept(route, "bgp"))

	// Reload with no rules
	ev.Reload(nil)
	assert.False(t, ev.Accept(route, "bgp"))

	// Reload with different rules
	ev.Reload([]ImportRule{
		{Source: "connected"},
	})
	assert.False(t, ev.Accept(route, "bgp"))
	assert.True(t, ev.Accept(RedistRoute{Origin: "connected", Family: "ipv4/unicast", Source: "connected"}, "bgp"))
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
	ev2 := NewEvaluator([]ImportRule{
		{Source: "ebgp", Families: []string{"ipv4/unicast", "ipv4/vpn"}},
	})
	r2 := ev2.Rules()
	r2[0].Families[0] = "modified"
	assert.Equal(t, "ipv4/unicast", ev2.Rules()[0].Families[0])
}

// TestEvaluatorNilRules verifies evaluator works with nil rules.
//
// VALIDATES: NewEvaluator(nil) and Reload(nil) produce a working evaluator that rejects all.
// PREVENTS: Nil pointer dereference on empty config.
func TestEvaluatorNilRules(t *testing.T) {
	ev := NewEvaluator(nil)
	assert.False(t, ev.Accept(RedistRoute{Origin: "ospf", Family: "ipv4/unicast", Source: "ospf"}, "bgp"))
	assert.Empty(t, ev.Rules())
}
