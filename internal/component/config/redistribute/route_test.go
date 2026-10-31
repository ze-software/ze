package redistribute

import (
	"net/netip"
	"testing"

	"github.com/ze-software/ze/internal/core/family"
	ribpolicy "github.com/ze-software/ze/internal/core/rib/policy"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestAcceptLoopPrevention verifies routes are rejected when origin matches importing protocol.
//
// VALIDATES: A route from protocol X is never redistributed back into protocol X.
// PREVENTS: Redistribution loops between protocols.
func TestAcceptLoopPrevention(t *testing.T) {
	rule := ImportRule{Source: "ospf"}
	route := RedistRoute{Origin: "ospf", Source: "ospf", Route: ribpolicy.Route{Family: family.IPv4Unicast}}

	// OSPF route should not be imported back into OSPF
	assert.False(t, accepts(&rule, route, "ospf"))

	// OSPF route can be imported into BGP
	assert.True(t, accepts(&rule, route, "bgp"))
}

// TestAcceptLoopPreventionBGPSubSources verifies ibgp/ebgp routes are blocked
// from redistribution back into any BGP source.
//
// VALIDATES: ibgp and ebgp both have origin "bgp", preventing loops across BGP sub-sources.
// PREVENTS: Route from ibgp looping back via ebgp import.
func TestAcceptLoopPreventionBGPSubSources(t *testing.T) {
	tests := []struct {
		name              string
		route             RedistRoute
		rule              ImportRule
		importingProtocol string
		want              bool
	}{
		{
			name:              "ibgp route blocked from bgp import",
			route:             RedistRoute{Origin: "bgp", Source: "ibgp", Route: ribpolicy.Route{Family: family.IPv4Unicast}},
			rule:              ImportRule{Source: "ibgp"},
			importingProtocol: "bgp",
			want:              false,
		},
		{
			name:              "ebgp route blocked from bgp import",
			route:             RedistRoute{Origin: "bgp", Source: "ebgp", Route: ribpolicy.Route{Family: family.IPv4Unicast}},
			rule:              ImportRule{Source: "ebgp"},
			importingProtocol: "bgp",
			want:              false,
		},
		{
			name:              "ospf route accepted into bgp",
			route:             RedistRoute{Origin: "ospf", Source: "ospf", Route: ribpolicy.Route{Family: family.IPv4Unicast}},
			rule:              ImportRule{Source: "ospf"},
			importingProtocol: "bgp",
			want:              true,
		},
		{
			name:              "connected route accepted into bgp",
			route:             RedistRoute{Origin: "connected", Source: "connected", Route: ribpolicy.Route{Family: family.IPv4Unicast}},
			rule:              ImportRule{Source: "connected"},
			importingProtocol: "bgp",
			want:              true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, accepts(&tt.rule, tt.route, tt.importingProtocol))
		})
	}
}

func TestAcceptUmbrellaOriginSource(t *testing.T) {
	route := RedistRoute{Origin: "bgp", Source: "ebgp", Route: ribpolicy.Route{Family: family.IPv6Unicast}}

	assert.True(t, accepts(&ImportRule{Source: "bgp"}, route, "ospf"))
	assert.True(t, accepts(&ImportRule{Source: "ebgp"}, route, "ospf"))
	assert.False(t, accepts(&ImportRule{Source: "ibgp"}, route, "ospf"))
	assert.False(t, accepts(&ImportRule{Source: "bgp"}, route, "bgp"))
}

// TestAcceptSourceMismatch verifies routes are rejected when source doesn't match the rule.
//
// VALIDATES: Import rule only accepts routes from its configured source.
// PREVENTS: Routes from wrong source leaking through.
func TestAcceptSourceMismatch(t *testing.T) {
	rule := ImportRule{Source: "ebgp"}
	route := RedistRoute{Origin: "bgp", Source: "ibgp", Route: ribpolicy.Route{Family: family.IPv4Unicast}}

	// ibgp route should not match ebgp rule (even though both have origin "bgp")
	// Loop prevention fires first here since origin matches importing protocol,
	// but test the source check by using a different importing protocol.
	assert.False(t, accepts(&rule, route, "ospf"))
}

// TestAcceptFamilyFilter verifies the family restriction works.
//
// VALIDATES: A rule's chain decides which families move (a family match).
// PREVENTS: Unwanted families being redistributed.
func TestAcceptFamilyFilter(t *testing.T) {
	ipv4VPN := family.Family{AFI: family.AFIIPv4, SAFI: family.SAFIVPN}
	l2vpnEVPN := family.Family{AFI: family.AFIL2VPN, SAFI: family.SAFIEVPN}

	rule := ImportRule{
		Source: "ebgp",
		Final:  "accept", Chain: familyChain{family.IPv4Unicast, ipv4VPN},
	}

	tests := []struct {
		name   string
		family family.Family
		want   bool
	}{
		{"allowed ipv4/unicast", family.IPv4Unicast, true},
		{"allowed ipv4/vpn", ipv4VPN, true},
		{"blocked ipv6/unicast", family.IPv6Unicast, false},
		{"blocked l2vpn/evpn", l2vpnEVPN, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			route := RedistRoute{Origin: "bgp", Source: "ebgp", Route: ribpolicy.Route{Family: tt.family}}
			assert.Equal(t, tt.want, accepts(&rule, route, "ospf"))
		})
	}
}

// TestAcceptEmptyFamiliesAllowsAll verifies that a rule with no chain accepts all.
//
// VALIDATES: A rule with no chain accepts every family.
// PREVENTS: Empty list accidentally rejecting everything.
func TestAcceptEmptyFamiliesAllowsAll(t *testing.T) {
	rule := ImportRule{Source: "ospf"}
	families := []family.Family{
		family.IPv4Unicast,
		family.IPv6Unicast,
		{AFI: family.AFIL2VPN, SAFI: family.SAFIEVPN},
		{AFI: family.AFIIPv4, SAFI: family.SAFIVPN},
	}

	for _, fam := range families {
		route := RedistRoute{Origin: "ospf", Source: "ospf", Route: ribpolicy.Route{Family: fam}}
		assert.True(t, accepts(&rule, route, "bgp"), "family %s should be accepted", fam)
	}
}

// TestEvaluateMultipleRules verifies evaluation across multiple import rules.
//
// VALIDATES: evaluate returns true if any rule accepts the route.
// PREVENTS: First-match-only or all-must-match logic errors.
func TestEvaluateMultipleRules(t *testing.T) {
	rules := []ImportRule{
		{Source: "ebgp", Final: "accept", Chain: familyChain{family.IPv4Unicast}},
		{Source: "ospf"},
		{Source: "connected", Final: "accept", Chain: familyChain{family.IPv6Unicast}},
	}

	tests := []struct {
		name  string
		route RedistRoute
		proto string
		want  bool
	}{
		{
			name:  "ebgp ipv4 accepted",
			route: RedistRoute{Origin: "bgp", Source: "ebgp", Route: ribpolicy.Route{Family: family.IPv4Unicast}},
			proto: "ospf",
			want:  true,
		},
		{
			name:  "ebgp ipv6 rejected by family filter",
			route: RedistRoute{Origin: "bgp", Source: "ebgp", Route: ribpolicy.Route{Family: family.IPv6Unicast}},
			proto: "ospf",
			want:  false,
		},
		{
			name:  "ospf any family accepted",
			route: RedistRoute{Origin: "ospf", Source: "ospf", Route: ribpolicy.Route{Family: family.IPv6Unicast}},
			proto: "bgp",
			want:  true,
		},
		{
			name:  "connected ipv6 accepted",
			route: RedistRoute{Origin: "connected", Source: "connected", Route: ribpolicy.Route{Family: family.IPv6Unicast}},
			proto: "bgp",
			want:  true,
		},
		{
			name:  "connected ipv4 rejected by family filter",
			route: RedistRoute{Origin: "connected", Source: "connected", Route: ribpolicy.Route{Family: family.IPv4Unicast}},
			proto: "bgp",
			want:  false,
		},
		{
			name:  "loop prevention across all rules",
			route: RedistRoute{Origin: "bgp", Source: "ebgp", Route: ribpolicy.Route{Family: family.IPv4Unicast}},
			proto: "bgp",
			want:  false,
		},
		{
			name:  "unknown source rejected",
			route: RedistRoute{Origin: "isis", Source: "isis", Route: ribpolicy.Route{Family: family.IPv4Unicast}},
			proto: "bgp",
			want:  false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, evaluates(tt.route, rules, tt.proto))
		})
	}
}

// TestEvaluateEmptyRules verifies that no rules means nothing accepted.
//
// VALIDATES: Empty rule set rejects all routes.
// PREVENTS: Nil/empty rules accidentally accepting everything.
func TestEvaluateEmptyRules(t *testing.T) {
	route := RedistRoute{Origin: "ospf", Source: "ospf", Route: ribpolicy.Route{Family: family.IPv4Unicast}}
	assert.False(t, evaluates(route, nil, "bgp"))
	assert.False(t, evaluates(route, []ImportRule{}, "bgp"))
}

// TestImportRuleDestinationScoped verifies R-3: an ImportRule parsed under a
// `destination <proto>` block is accepted ONLY when the importing protocol
// matches that destination. A source imported under `destination bgp` must NOT
// satisfy Accept(_, "ospf"). A rule with an empty Destination stays
// destination-agnostic (back-compat for rules built before scoping / tests).
//
// VALIDATES: AC-5 / R-3 destination scoping in both directions.
// PREVENTS: cross-destination leak (an import meant for BGP reaching OSPF).
func TestImportRuleDestinationScoped(t *testing.T) {
	route := RedistRoute{Origin: "connected", Source: "connected", Route: ribpolicy.Route{Family: family.IPv4Unicast}}

	bgpOnly := ImportRule{Source: "connected", Destination: "bgp"}
	assert.True(t, accepts(&bgpOnly, route, "bgp"), "import under destination bgp must feed bgp")
	assert.False(t, accepts(&bgpOnly, route, "ospf"), "import under destination bgp must NOT feed ospf")

	ospfOnly := ImportRule{Source: "connected", Destination: "ospf"}
	assert.True(t, accepts(&ospfOnly, route, "ospf"), "import under destination ospf must feed ospf")
	assert.False(t, accepts(&ospfOnly, route, "bgp"), "import under destination ospf must NOT feed bgp")

	// Empty Destination stays agnostic (back-compat).
	agnostic := ImportRule{Source: "connected"}
	assert.True(t, accepts(&agnostic, route, "bgp"), "empty destination accepts any importer (bgp)")
	assert.True(t, accepts(&agnostic, route, "ospf"), "empty destination accepts any importer (ospf)")
}

// TestEvaluatorHasDestination verifies HasDestination reports whether any rule
// feeds a given destination protocol; an empty-Destination rule matches every
// destination.
//
// VALIDATES: orchestrator can cheaply skip firing a replay when no import feeds BGP.
// PREVENTS: a pointless replay broadcast on every peer-up when redistribute has
// no BGP destination.
func TestEvaluatorHasDestination(t *testing.T) {
	ev := NewEvaluator([]ImportRule{{Source: "connected", Destination: "ospf"}})
	assert.True(t, ev.HasDestination("ospf"))
	assert.False(t, ev.HasDestination("bgp"))

	// An empty-Destination (agnostic) rule matches every destination.
	evAgnostic := NewEvaluator([]ImportRule{{Source: "connected"}})
	assert.True(t, evAgnostic.HasDestination("bgp"))
	assert.True(t, evAgnostic.HasDestination("ospf"))
}

// TestRedistributeChainAccept is AC-14 at the rule: `import bgp { filters [ P ];
// final reject; }` under `destination ospf` moves only the prefixes P accepts,
// with the sets P applied, and a rule with no chain still moves every route.
//
// VALIDATES: Accept answers the chain's verdict and sets for each route.
// PREVENTS: a chain that is compiled and never consulted, and the chain's sets
// lost between the verdict and the dispatch.
func TestRedistributeChainAccept(t *testing.T) {
	accepted := netip.MustParsePrefix("203.0.113.0/24")
	rejected := netip.MustParsePrefix("198.51.100.0/24")
	rule := ImportRule{Source: "bgp", Destination: "ospf", Filters: []string{"P"}, Final: "reject",
		Chain: prefixSetChain{accept: []netip.Prefix{accepted}, tag: 7}}
	route := func(p netip.Prefix) *RedistRoute {
		return &RedistRoute{Origin: "bgp", Source: "ebgp", Route: ribpolicy.Route{Family: family.IPv4Unicast, Prefix: p}}
	}

	sets, ok := rule.Accept(route(accepted), "ospf")
	require.True(t, ok, "the prefix P accepts must move into OSPF")
	assert.Equal(t, ribpolicy.Sets{Tag: 7, HasTag: true}, sets, "the chain's sets reach the caller")

	_, ok = rule.Accept(route(rejected), "ospf")
	assert.False(t, ok, "a prefix P rejects must not move")

	sets, ok = NewEvaluator([]ImportRule{rule}).Accept(route(accepted), "ospf")
	require.True(t, ok, "the evaluator answers the accepting rule's verdict")
	assert.True(t, sets.HasTag, "the evaluator answers the accepting rule's sets")

	unfiltered := ImportRule{Source: "bgp", Destination: "ospf"}
	assert.True(t, accepts(&unfiltered, *route(rejected), "ospf"), "a rule with no chain moves every route")

	noChain := ImportRule{Source: "bgp", Destination: "ospf", Final: "accept"}
	assert.Panics(t, func() { noChain.Accept(route(accepted), "ospf") }, "a final with no compiled chain is a Ze defect")
}
