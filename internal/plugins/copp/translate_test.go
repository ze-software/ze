package copp

import (
	"net/netip"
	"testing"

	"github.com/ze-software/ze/internal/component/firewall"
)

func TestCoppTranslateOrder(t *testing.T) {
	policy := coppPolicy{
		Rate:           100,
		RateUnit:       "second",
		Dimension:      firewall.RateDimensionPackets,
		Burst:          20,
		ProtectedPorts: []uint16{179},
		TrustedSources: []netip.Prefix{netip.MustParsePrefix("192.0.2.0/24")},
		OverPolicy:     "accept",
	}

	table := translatePolicy(policy)

	if table.Name != coppTableName {
		t.Errorf("table name = %q, want %q", table.Name, coppTableName)
	}
	if table.Family != firewall.FamilyInet {
		t.Errorf("family = %v, want inet", table.Family)
	}
	if len(table.Chains) != 1 {
		t.Fatalf("chains len = %d, want 1", len(table.Chains))
	}

	chain := table.Chains[0]
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

	if len(chain.Terms) != 3 {
		t.Fatalf("terms len = %d, want 3 (established, trusted, rate-limit)", len(chain.Terms))
	}

	if chain.Terms[0].Name != "established" {
		t.Errorf("term[0] name = %q, want established", chain.Terms[0].Name)
	}
	if chain.Terms[1].Name != "trusted-192.0.2.0/24" {
		t.Errorf("term[1] name = %q, want trusted-192.0.2.0/24", chain.Terms[1].Name)
	}
	if chain.Terms[2].Name != "rate-limit" {
		t.Errorf("term[2] name = %q, want rate-limit", chain.Terms[2].Name)
	}
}

func TestCoppTranslateNoTrusted(t *testing.T) {
	policy := coppPolicy{
		Rate:           50,
		RateUnit:       "second",
		Dimension:      firewall.RateDimensionPackets,
		ProtectedPorts: []uint16{179},
		OverPolicy:     "accept",
	}

	table := translatePolicy(policy)
	chain := table.Chains[0]

	if len(chain.Terms) != 2 {
		t.Fatalf("terms len = %d, want 2 (established, rate-limit)", len(chain.Terms))
	}
	if chain.Terms[0].Name != "established" {
		t.Errorf("term[0] name = %q, want established", chain.Terms[0].Name)
	}
	if chain.Terms[1].Name != "rate-limit" {
		t.Errorf("term[1] name = %q, want rate-limit", chain.Terms[1].Name)
	}
}

func TestCoppTranslateDropPolicy(t *testing.T) {
	policy := coppPolicy{
		Rate:           10,
		RateUnit:       "second",
		Dimension:      firewall.RateDimensionPackets,
		ProtectedPorts: []uint16{179},
		OverPolicy:     "drop",
	}

	table := translatePolicy(policy)
	chain := table.Chains[0]

	if chain.Policy != firewall.PolicyDrop {
		t.Errorf("chain policy = %v, want drop", chain.Policy)
	}
}

func TestCoppTranslateMultiplePorts(t *testing.T) {
	policy := coppPolicy{
		Rate:           10,
		RateUnit:       "second",
		Dimension:      firewall.RateDimensionPackets,
		ProtectedPorts: []uint16{179, 1790},
		OverPolicy:     "accept",
	}

	table := translatePolicy(policy)
	chain := table.Chains[0]

	rateLimitTerm := chain.Terms[len(chain.Terms)-1]
	for _, m := range rateLimitTerm.Matches {
		if dp, ok := m.(firewall.MatchDestinationPort); ok {
			if len(dp.Ranges) != 2 {
				t.Errorf("port ranges len = %d, want 2", len(dp.Ranges))
			}
			return
		}
	}
	t.Error("no MatchDestinationPort found in rate-limit term")
}

func TestCoppTranslateLimitValues(t *testing.T) {
	policy := coppPolicy{
		Rate:           100,
		RateUnit:       "second",
		Dimension:      firewall.RateDimensionPackets,
		Burst:          20,
		ProtectedPorts: []uint16{179},
		OverPolicy:     "accept",
	}

	table := translatePolicy(policy)
	chain := table.Chains[0]

	rateLimitTerm := chain.Terms[len(chain.Terms)-1]
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
	policy := coppPolicy{
		Rate:           100,
		RateUnit:       "second",
		Dimension:      firewall.RateDimensionPackets,
		ProtectedPorts: []uint16{179},
		TrustedSources: []netip.Prefix{
			netip.MustParsePrefix("192.0.2.0/24"),
			netip.MustParsePrefix("198.51.100.0/24"),
		},
		OverPolicy: "drop",
	}

	table := translatePolicy(policy)
	chain := table.Chains[0]

	if chain.Terms[0].Name != "established" {
		t.Errorf("first term must be 'established', got %q", chain.Terms[0].Name)
	}
	if _, ok := chain.Terms[0].Actions[0].(firewall.Accept); !ok {
		t.Error("established term must have Accept action")
	}
}

func TestCoppTableWithdraw(t *testing.T) {
	firewall.RegisterTables("copp", []firewall.Table{{
		Name:   coppTableName,
		Family: firewall.FamilyInet,
	}})
	firewall.RegisterTables("copp", nil)
}
