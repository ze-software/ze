package copp

import (
	"net/netip"
	"slices"
	"testing"

	"github.com/ze-software/ze/internal/component/firewall"
)

// limitedPolicy is a policy with one IPv4 neighbor and a rate, the shape most
// translate tests start from.
func limitedPolicy(overPolicy string) coppPolicy {
	return coppPolicy{
		ProtectedPorts: []uint16{179},
		Neighbors:      []netip.Prefix{netip.MustParsePrefix("192.0.2.1/32")},
		RateLimit: &coppRateLimit{
			Rate:       100,
			RateUnit:   "second",
			Dimension:  firewall.RateDimensionPackets,
			Burst:      20,
			OverPolicy: overPolicy,
		},
	}
}

// chainNamed answers the chain called name, failing the test when the table
// holds none.
func chainNamed(t *testing.T, table firewall.Table, name string) firewall.Chain {
	t.Helper()
	for _, chain := range table.Chains {
		if chain.Name == name {
			return chain
		}
	}
	t.Fatalf("table %s has no chain %q", table.Name, name)
	return firewall.Chain{}
}

func termNames(chain firewall.Chain) []string {
	names := make([]string, len(chain.Terms))
	for i, term := range chain.Terms {
		names[i] = term.Name
	}
	return names
}

func TestCoppTranslateOrder(t *testing.T) {
	policy := limitedPolicy(overPolicyAccept)
	policy.TrustedSources = []netip.Prefix{netip.MustParsePrefix("192.0.2.0/24")}

	table := translatePolicy(policy)

	if table.Name != coppTableName {
		t.Errorf("table name = %q, want %q", table.Name, coppTableName)
	}
	if table.Family != firewall.FamilyInet {
		t.Errorf("family = %v, want inet", table.Family)
	}

	// Exactly the base input chain and the regular bgp-neighbor chain: a stray
	// chain, above all a second base input chain with a drop policy, would
	// lock the operator out.
	chainNames := make([]string, len(table.Chains))
	for i, c := range table.Chains {
		chainNames[i] = c.Name
	}
	if wantChains := []string{inputChainName, neighborChainName}; !slices.Equal(chainNames, wantChains) {
		t.Errorf("chains = %v, want %v", chainNames, wantChains)
	}

	chain := chainNamed(t, table, inputChainName)
	if !chain.IsBase {
		t.Error("expected base chain")
	}
	if chain.Type != firewall.ChainFilter {
		t.Errorf("chain type = %v, want filter", chain.Type)
	}
	if chain.Hook != firewall.HookInput {
		t.Errorf("chain hook = %v, want input", chain.Hook)
	}
	if chain.Policy != firewall.PolicyAccept {
		t.Errorf("chain policy = %v, want accept", chain.Policy)
	}

	want := []string{"established", "trusted-192.0.2.0-24", neighborSetIPv4, unknownSourceDrop}
	if got := termNames(chain); !slices.Equal(got, want) {
		t.Errorf("input terms = %v, want %v", got, want)
	}
}

func TestCoppTranslateNoTrusted(t *testing.T) {
	policy := limitedPolicy(overPolicyAccept)

	chain := chainNamed(t, translatePolicy(policy), inputChainName)
	want := []string{"established", neighborSetIPv4, unknownSourceDrop}
	if got := termNames(chain); !slices.Equal(got, want) {
		t.Fatalf("terms = %v, want %v (no trusted term without a trusted-source)", got, want)
	}
	if chain.Terms[0].Name != "established" {
		t.Errorf("term[0] name = %q, want established", chain.Terms[0].Name)
	}
}

// TestTranslatePolicyTermOrder proves the order of the ACL, both families.
//
// VALIDATES: established accept, trusted-source accept, the neighbor sets
// sending to the bgp-neighbor chain, then the unknown-source drop; the
// limiter lives in the bgp-neighbor chain ahead of its accept, so only a
// neighbor reaches it; the chain policy stays accept and every input term but
// the first names TCP to the protected port; AC-12, AC-13, R-5.
// PREVENTS: an unknown-source flood spending the neighbors' budget, a trusted
// source meeting the limiter, and a term that reaches SSH or ICMP.
func TestTranslatePolicyTermOrder(t *testing.T) {
	policy := limitedPolicy(overPolicyDrop)
	policy.TrustedSources = []netip.Prefix{netip.MustParsePrefix("203.0.113.0/24")}
	policy.Neighbors = append(policy.Neighbors, netip.MustParsePrefix("2001:db8::1/128"))

	table := translatePolicy(policy)
	if err := firewall.ValidateTables([]firewall.Table{table}); err != nil {
		t.Fatalf("the firewall model refuses the table: %v", err)
	}

	input := chainNamed(t, table, inputChainName)
	if input.Policy != firewall.PolicyAccept {
		t.Errorf("input policy = %v, want accept: a drop policy locks the operator out", input.Policy)
	}
	want := []string{"established", "trusted-203.0.113.0-24", neighborSetIPv4, neighborSetIPv6, unknownSourceDrop}
	if got := termNames(input); !slices.Equal(got, want) {
		t.Fatalf("input terms = %v, want %v", got, want)
	}
	for _, term := range input.Terms[1:] {
		if !matchesTCPToPort(term, 179) {
			t.Errorf("term %q does not name TCP to port 179, so it reaches other traffic: %+v", term.Name, term.Matches)
		}
	}
	for i, setName := range []string{neighborSetIPv4, neighborSetIPv6} {
		term := input.Terms[2+i]
		if !slices.Contains(term.Matches, firewall.Match(firewall.MatchInSet{SetName: setName, MatchField: firewall.SetFieldSourceAddr})) {
			t.Errorf("term %q does not match the source against set %s: %+v", term.Name, setName, term.Matches)
		}
		if !slices.Equal(term.Actions, []firewall.Action{firewall.Jump{Target: neighborChainName}}) {
			t.Errorf("term %q actions = %+v, want a jump to %s", term.Name, term.Actions, neighborChainName)
		}
	}

	neighbor := chainNamed(t, table, neighborChainName)
	if neighbor.IsBase {
		t.Error("the bgp-neighbor chain is a base chain; only a jump may reach it")
	}
	if got := termNames(neighbor); !slices.Equal(got, []string{rateLimitTermName, neighborAcceptName}) {
		t.Fatalf("bgp-neighbor terms = %v, want [%s %s]", got, rateLimitTermName, neighborAcceptName)
	}
	if !slices.Equal(neighbor.Terms[1].Actions, []firewall.Action{firewall.Accept{}}) {
		t.Errorf("neighbor-accept actions = %+v, want accept", neighbor.Terms[1].Actions)
	}
}

// TestTranslatePolicyNoRate proves the ACL stands alone when no rate is set.
//
// VALIDATES: AC-11: the table carries the neighbor set, the accept and the
// drop, and no limiter anywhere.
func TestTranslatePolicyNoRate(t *testing.T) {
	policy := limitedPolicy(overPolicyDrop)
	policy.RateLimit = nil

	table := translatePolicy(policy)
	if err := firewall.ValidateTables([]firewall.Table{table}); err != nil {
		t.Fatalf("the firewall model refuses the table: %v", err)
	}
	for _, chain := range table.Chains {
		for _, term := range chain.Terms {
			for _, action := range term.Actions {
				if _, limited := action.(firewall.Limit); limited {
					t.Errorf("chain %s term %q carries a limiter with no rate configured", chain.Name, term.Name)
				}
			}
		}
	}
	neighbor := chainNamed(t, table, neighborChainName)
	if got := termNames(neighbor); !slices.Equal(got, []string{neighborAcceptName}) {
		t.Errorf("bgp-neighbor terms = %v, want [%s]", got, neighborAcceptName)
	}
	input := chainNamed(t, table, inputChainName)
	if last := input.Terms[len(input.Terms)-1]; !dropsTCPToPort(last, 179) {
		t.Errorf("last input term %q = %+v, want the unknown-source drop", last.Name, last)
	}
}

// TestTranslatePolicyNoNeighbors proves a policy with no configured neighbor
// still drops: the ACL admits the trusted sources and nothing else, and no
// empty set or dangling jump is emitted.
func TestTranslatePolicyNoNeighbors(t *testing.T) {
	policy := limitedPolicy(overPolicyDrop)
	policy.Neighbors = nil

	table := translatePolicy(policy)
	if err := firewall.ValidateTables([]firewall.Table{table}); err != nil {
		t.Fatalf("the firewall model refuses the table: %v", err)
	}
	if len(table.Sets) != 0 || len(table.Chains) != 1 {
		t.Errorf("sets %d, chains %d; want no set and only the input chain", len(table.Sets), len(table.Chains))
	}
	input := chainNamed(t, table, inputChainName)
	if got := termNames(input); !slices.Equal(got, []string{"established", unknownSourceDrop}) {
		t.Errorf("input terms = %v, want [established %s]", got, unknownSourceDrop)
	}
}

// TestTranslatePolicyNeighborSetIntervals proves the neighbor sets hold each
// configured prefix exactly: a static peer inside a dynamic range is folded
// into the range (the kernel refuses overlapping intervals), and a prefix
// that reaches the top of the space gets no end element.
func TestTranslatePolicyNeighborSetIntervals(t *testing.T) {
	policy := limitedPolicy(overPolicyDrop)
	policy.Neighbors = []netip.Prefix{
		netip.MustParsePrefix("10.10.0.0/16"),
		netip.MustParsePrefix("10.10.3.4/32"),
		netip.MustParsePrefix("192.0.2.1/32"),
		netip.MustParsePrefix("255.255.255.0/24"),
		netip.MustParsePrefix("2001:db8::/32"),
		netip.MustParsePrefix("fe80::1/128"),
	}

	table := translatePolicy(policy)
	for _, set := range table.Sets {
		if set.Flags&firewall.SetFlagInterval == 0 {
			t.Errorf("set %s has no interval flag; a range cannot be held", set.Name)
		}
	}
	got := setElementPrefixes(t, table.Sets)
	want := []netip.Prefix{
		netip.MustParsePrefix("10.10.0.0/16"),
		netip.MustParsePrefix("192.0.2.1/32"),
		netip.MustParsePrefix("255.255.255.0/24"),
		netip.MustParsePrefix("2001:db8::/32"),
		netip.MustParsePrefix("fe80::1/128"),
	}
	if !slices.Equal(got, want) {
		t.Errorf("set prefixes = %v, want %v", got, want)
	}
}

// TestCoppTranslateDropPolicy checks that over-limit-policy drop reaches only
// the over-limit packets: the chain policy stays accept and the drop rides the
// limiter as an inverted limit.
func TestCoppTranslateDropPolicy(t *testing.T) {
	table := translatePolicy(limitedPolicy(overPolicyDrop))

	if input := chainNamed(t, table, inputChainName); input.Policy != firewall.PolicyAccept {
		t.Errorf("chain policy = %v, want accept: a drop policy locks the operator out", input.Policy)
	}

	rateLimit := chainNamed(t, table, neighborChainName).Terms[0]
	if rateLimit.Name != rateLimitTermName {
		t.Fatalf("first bgp-neighbor term = %q, want %s", rateLimit.Name, rateLimitTermName)
	}
	if len(rateLimit.Actions) != 2 {
		t.Fatalf("rate-limit actions = %d, want 2", len(rateLimit.Actions))
	}
	lim, ok := rateLimit.Actions[0].(firewall.Limit)
	if !ok {
		t.Fatalf("rate-limit action[0] = %T, want firewall.Limit", rateLimit.Actions[0])
	}
	if !lim.Over {
		t.Error("limit Over = false, want true: without the inversion the drop hits every under-limit packet")
	}
	if _, ok := rateLimit.Actions[1].(firewall.Drop); !ok {
		t.Errorf("rate-limit action[1] = %T, want firewall.Drop", rateLimit.Actions[1])
	}
	if !slices.Contains(rateLimit.Matches, firewall.Match(firewall.MatchConnState{States: firewall.ConnStateNew})) {
		t.Errorf("rate-limit matches = %+v, want ct state new", rateLimit.Matches)
	}
}

// TestCoppTranslateAcceptPolicyDoesNotInvertTheLimiter checks the explicit
// accept spelling: the limiter matches while it has credit and the action
// pair ends in accept, so an over-limit packet falls through to the
// neighbor-accept term.
func TestCoppTranslateAcceptPolicyDoesNotInvertTheLimiter(t *testing.T) {
	rateLimit := chainNamed(t, translatePolicy(limitedPolicy(overPolicyAccept)), neighborChainName).Terms[0]
	lim, ok := rateLimit.Actions[0].(firewall.Limit)
	if !ok {
		t.Fatalf("rate-limit action[0] = %T, want firewall.Limit", rateLimit.Actions[0])
	}
	if lim.Over {
		t.Error("limit Over = true, want false: accept must not invert the limiter")
	}
	if _, ok := rateLimit.Actions[1].(firewall.Accept); !ok {
		t.Errorf("rate-limit action[1] = %T, want firewall.Accept", rateLimit.Actions[1])
	}
}

func TestCoppTranslateMultiplePorts(t *testing.T) {
	policy := limitedPolicy(overPolicyAccept)
	policy.ProtectedPorts = []uint16{179, 1790}

	// Exactly one single-port range per protected port, as portRanges emits:
	// a merged 179-1790 range, or 0-65535, would cover both ports and drop
	// every port between them.
	wantRanges := []firewall.PortRange{{Lo: 179, Hi: 179}, {Lo: 1790, Hi: 1790}}
	input := chainNamed(t, translatePolicy(policy), inputChainName)
	for _, term := range input.Terms[1:] {
		for _, port := range policy.ProtectedPorts {
			if !matchesTCPToPort(term, port) {
				t.Errorf("term %q does not name TCP to port %d", term.Name, port)
			}
		}
		found := false
		for _, match := range term.Matches {
			dport, ok := match.(firewall.MatchDestinationPort)
			if !ok {
				continue
			}
			found = true
			if !slices.Equal(dport.Ranges, wantRanges) {
				t.Errorf("term %q dport ranges = %v, want %v", term.Name, dport.Ranges, wantRanges)
			}
		}
		if !found {
			t.Errorf("term %q has no destination port match", term.Name)
		}
	}
}

func TestCoppTranslateLimitValues(t *testing.T) {
	rateLimitTerm := chainNamed(t, translatePolicy(limitedPolicy(overPolicyAccept)), neighborChainName).Terms[0]
	for _, a := range rateLimitTerm.Actions {
		lim, ok := a.(firewall.Limit)
		if !ok {
			continue
		}
		if lim.Rate != 100 {
			t.Errorf("limit rate = %d, want 100", lim.Rate)
		}
		if lim.Unit != "second" {
			t.Errorf("limit unit = %q, want second", lim.Unit)
		}
		if lim.Burst != 20 {
			t.Errorf("limit burst = %d, want 20", lim.Burst)
		}
		if lim.Dimension != firewall.RateDimensionPackets {
			t.Errorf("limit dimension = %d, want RateDimensionPackets", lim.Dimension)
		}
		return
	}
	t.Error("no Limit action found in rate-limit term")
}

func TestCoppTranslateEstablishedAcceptsFirst(t *testing.T) {
	policy := limitedPolicy(overPolicyDrop)
	policy.TrustedSources = []netip.Prefix{
		netip.MustParsePrefix("192.0.2.0/24"),
		netip.MustParsePrefix("198.51.100.0/24"),
	}

	chain := chainNamed(t, translatePolicy(policy), inputChainName)
	if chain.Terms[0].Name != "established" {
		t.Errorf("first term must be 'established', got %q", chain.Terms[0].Name)
	}
	if _, ok := chain.Terms[0].Actions[0].(firewall.Accept); !ok {
		t.Error("established term must have Accept action")
	}
}

func TestCoppTableWithdraw(t *testing.T) {
	_ = firewall.RegisterTables("copp", []firewall.Table{{
		Name:   coppTableName,
		Family: firewall.FamilyInet,
	}})
	_ = firewall.RegisterTables("copp", nil)
}

// matchesTCPToPort reports whether a term matches protocol tcp and a
// destination port range holding port.
func matchesTCPToPort(term firewall.Term, port uint16) bool {
	tcp, toPort := false, false
	for _, match := range term.Matches {
		switch m := match.(type) {
		case firewall.MatchProtocol:
			tcp = m.Protocol == "tcp"
		case firewall.MatchDestinationPort:
			for _, r := range m.Ranges {
				if r.Lo <= port && port <= r.Hi {
					toPort = true
				}
			}
		}
	}
	return tcp && toPort
}
