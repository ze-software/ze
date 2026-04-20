package redistribute

import (
	"testing"

	"codeberg.org/thomas-mangin/ze/internal/core/family"

	"github.com/stretchr/testify/assert"
)

// TestAcceptLoopPrevention verifies routes are rejected when origin matches importing protocol.
//
// VALIDATES: A route from protocol X is never redistributed back into protocol X.
// PREVENTS: Redistribution loops between protocols.
func TestAcceptLoopPrevention(t *testing.T) {
	rule := ImportRule{Source: "ospf"}
	route := RedistRoute{Origin: "ospf", Family: family.IPv4Unicast, Source: "ospf"}

	// OSPF route should not be imported back into OSPF
	assert.False(t, rule.Accept(route, "ospf"))

	// OSPF route can be imported into BGP
	assert.True(t, rule.Accept(route, "bgp"))
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
			route:             RedistRoute{Origin: "bgp", Family: family.IPv4Unicast, Source: "ibgp"},
			rule:              ImportRule{Source: "ibgp"},
			importingProtocol: "bgp",
			want:              false,
		},
		{
			name:              "ebgp route blocked from bgp import",
			route:             RedistRoute{Origin: "bgp", Family: family.IPv4Unicast, Source: "ebgp"},
			rule:              ImportRule{Source: "ebgp"},
			importingProtocol: "bgp",
			want:              false,
		},
		{
			name:              "ospf route accepted into bgp",
			route:             RedistRoute{Origin: "ospf", Family: family.IPv4Unicast, Source: "ospf"},
			rule:              ImportRule{Source: "ospf"},
			importingProtocol: "bgp",
			want:              true,
		},
		{
			name:              "connected route accepted into bgp",
			route:             RedistRoute{Origin: "connected", Family: family.IPv4Unicast, Source: "connected"},
			rule:              ImportRule{Source: "connected"},
			importingProtocol: "bgp",
			want:              true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, tt.rule.Accept(tt.route, tt.importingProtocol))
		})
	}
}

// TestAcceptSourceMismatch verifies routes are rejected when source doesn't match the rule.
//
// VALIDATES: Import rule only accepts routes from its configured source.
// PREVENTS: Routes from wrong source leaking through.
func TestAcceptSourceMismatch(t *testing.T) {
	rule := ImportRule{Source: "ebgp"}
	route := RedistRoute{Origin: "bgp", Family: family.IPv4Unicast, Source: "ibgp"}

	// ibgp route should not match ebgp rule (even though both have origin "bgp")
	// Loop prevention fires first here since origin matches importing protocol,
	// but test the source check by using a different importing protocol.
	assert.False(t, rule.Accept(route, "ospf"))
}

// TestAcceptFamilyFilter verifies the family restriction works.
//
// VALIDATES: Routes are filtered by address family when families list is non-empty.
// PREVENTS: Unwanted families being redistributed.
func TestAcceptFamilyFilter(t *testing.T) {
	ipv4VPN := family.Family{AFI: family.AFIIPv4, SAFI: family.SAFIVPN}
	l2vpnEVPN := family.Family{AFI: family.AFIL2VPN, SAFI: family.SAFIEVPN}

	rule := ImportRule{
		Source:   "ebgp",
		Families: []family.Family{family.IPv4Unicast, ipv4VPN},
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
			route := RedistRoute{Origin: "bgp", Family: tt.family, Source: "ebgp"}
			assert.Equal(t, tt.want, rule.Accept(route, "ospf"))
		})
	}
}

// TestAcceptEmptyFamiliesAllowsAll verifies that omitting families accepts all.
//
// VALIDATES: Empty families list means all families are accepted.
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
		route := RedistRoute{Origin: "ospf", Family: fam, Source: "ospf"}
		assert.True(t, rule.Accept(route, "bgp"), "family %s should be accepted", fam)
	}
}

// TestEvaluateMultipleRules verifies evaluation across multiple import rules.
//
// VALIDATES: Evaluate returns true if any rule accepts the route.
// PREVENTS: First-match-only or all-must-match logic errors.
func TestEvaluateMultipleRules(t *testing.T) {
	rules := []ImportRule{
		{Source: "ebgp", Families: []family.Family{family.IPv4Unicast}},
		{Source: "ospf"},
		{Source: "connected", Families: []family.Family{family.IPv6Unicast}},
	}

	tests := []struct {
		name  string
		route RedistRoute
		proto string
		want  bool
	}{
		{
			name:  "ebgp ipv4 accepted",
			route: RedistRoute{Origin: "bgp", Family: family.IPv4Unicast, Source: "ebgp"},
			proto: "ospf",
			want:  true,
		},
		{
			name:  "ebgp ipv6 rejected by family filter",
			route: RedistRoute{Origin: "bgp", Family: family.IPv6Unicast, Source: "ebgp"},
			proto: "ospf",
			want:  false,
		},
		{
			name:  "ospf any family accepted",
			route: RedistRoute{Origin: "ospf", Family: family.IPv6Unicast, Source: "ospf"},
			proto: "bgp",
			want:  true,
		},
		{
			name:  "connected ipv6 accepted",
			route: RedistRoute{Origin: "connected", Family: family.IPv6Unicast, Source: "connected"},
			proto: "bgp",
			want:  true,
		},
		{
			name:  "connected ipv4 rejected by family filter",
			route: RedistRoute{Origin: "connected", Family: family.IPv4Unicast, Source: "connected"},
			proto: "bgp",
			want:  false,
		},
		{
			name:  "loop prevention across all rules",
			route: RedistRoute{Origin: "bgp", Family: family.IPv4Unicast, Source: "ebgp"},
			proto: "bgp",
			want:  false,
		},
		{
			name:  "unknown source rejected",
			route: RedistRoute{Origin: "isis", Family: family.IPv4Unicast, Source: "isis"},
			proto: "bgp",
			want:  false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, Evaluate(tt.route, rules, tt.proto))
		})
	}
}

// TestEvaluateEmptyRules verifies that no rules means nothing accepted.
//
// VALIDATES: Empty rule set rejects all routes.
// PREVENTS: Nil/empty rules accidentally accepting everything.
func TestEvaluateEmptyRules(t *testing.T) {
	route := RedistRoute{Origin: "ospf", Family: family.IPv4Unicast, Source: "ospf"}
	assert.False(t, Evaluate(route, nil, "bgp"))
	assert.False(t, Evaluate(route, []ImportRule{}, "bgp"))
}
