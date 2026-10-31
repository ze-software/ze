package redistribute

import (
	"testing"

	"github.com/ze-software/ze/internal/core/family"
	ribpolicy "github.com/ze-software/ze/internal/core/rib/policy"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestEvaluatorAccept verifies the evaluator delegates to evaluate correctly.
//
// VALIDATES: Evaluator.Accept applies rules with loop prevention.
// PREVENTS: Rules not loaded or not checked.
func TestEvaluatorAccept(t *testing.T) {
	ev := NewEvaluator([]ImportRule{
		{Source: "ospf"},
		{Source: "ebgp", Final: "accept", Chain: familyChain{family.IPv4Unicast}},
	})

	// OSPF route accepted into BGP
	assert.True(t, accepts(ev, RedistRoute{Origin: "ospf", Source: "ospf", Route: ribpolicy.Route{Family: family.IPv4Unicast}}, "bgp"))

	// eBGP route blocked from BGP (loop)
	assert.False(t, accepts(ev, RedistRoute{Origin: "bgp", Source: "ebgp", Route: ribpolicy.Route{Family: family.IPv4Unicast}}, "bgp"))

	// eBGP ipv6 blocked by family filter
	assert.False(t, accepts(ev, RedistRoute{Origin: "bgp", Source: "ebgp", Route: ribpolicy.Route{Family: family.IPv6Unicast}}, "ospf"))

	// eBGP ipv4 accepted into OSPF
	assert.True(t, accepts(ev, RedistRoute{Origin: "bgp", Source: "ebgp", Route: ribpolicy.Route{Family: family.IPv4Unicast}}, "ospf"))
}

// TestEvaluatorReload verifies rules are replaced on reload.
//
// VALIDATES: Reload swaps rules; old rules no longer apply.
// PREVENTS: Stale rules persisting after config reload.
func TestEvaluatorReload(t *testing.T) {
	ev := NewEvaluator([]ImportRule{
		{Source: "ospf"},
	})

	route := RedistRoute{Origin: "ospf", Source: "ospf", Route: ribpolicy.Route{Family: family.IPv4Unicast}}
	assert.True(t, accepts(ev, route, "bgp"))

	// Reload with no rules
	ev.Reload(nil)
	assert.False(t, accepts(ev, route, "bgp"))

	// Reload with different rules
	ev.Reload([]ImportRule{
		{Source: "connected"},
	})
	assert.False(t, accepts(ev, route, "bgp"))
	assert.True(t, accepts(ev, RedistRoute{Origin: "connected", Source: "connected", Route: ribpolicy.Route{Family: family.IPv4Unicast}}, "bgp"))
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

	// Mutating the Filters slice should not affect the evaluator
	ev2 := NewEvaluator([]ImportRule{
		{Source: "ebgp", Filters: []string{"V4", "VPN"}, Final: "accept", Chain: familyChain{family.IPv4Unicast}},
	})
	r2 := ev2.Rules()
	r2[0].Filters[0] = ""
	assert.Equal(t, "V4", ev2.Rules()[0].Filters[0])
	assert.Equal(t, "accept", ev2.Rules()[0].Final)
	assert.NotNil(t, ev2.Rules()[0].Chain, "the copy keeps the compiled chain")
}

// TestEvaluatorNilRules verifies evaluator works with nil rules.
//
// VALIDATES: NewEvaluator(nil) and Reload(nil) produce a working evaluator that rejects all.
// PREVENTS: Nil pointer dereference on empty config.
func TestEvaluatorNilRules(t *testing.T) {
	ev := NewEvaluator(nil)
	assert.False(t, accepts(ev, RedistRoute{Origin: "ospf", Source: "ospf", Route: ribpolicy.Route{Family: family.IPv4Unicast}}, "bgp"))
	assert.Empty(t, ev.Rules())
}
