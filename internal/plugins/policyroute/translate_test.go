package policyroute

import (
	"net/netip"
	"testing"

	"codeberg.org/thomas-mangin/ze/internal/component/firewall"
)

func TestPolicyToFirewallTable(t *testing.T) {
	policy := PolicyRoute{
		Name:       "surfprotect",
		Interfaces: []InterfaceSpec{{Name: "l2tp", Wildcard: true}},
		Rules: []PolicyRule{
			{
				Name:   "bypass-dst",
				Match:  PolicyMatch{DestinationPort: "80,443", Protocol: "tcp"},
				Action: PolicyAction{Type: ActionAccept},
			},
			{
				Name:   "block-quic",
				Match:  PolicyMatch{DestinationPort: "80,443", Protocol: "udp"},
				Action: PolicyAction{Type: ActionDrop},
			},
			{
				Name:   "redirect",
				Match:  PolicyMatch{DestinationPort: "80,443", Protocol: "tcp"},
				Action: PolicyAction{Type: ActionTable, Table: 100, TCPMSS: 1436},
			},
		},
	}

	alloc := newAllocator()
	result, err := alloc.translate([]PolicyRoute{policy})
	if err != nil {
		t.Fatalf("translate: %v", err)
	}

	if len(result.Tables) != 1 {
		t.Fatalf("expected 1 table, got %d", len(result.Tables))
	}
	tbl := result.Tables[0]
	if tbl.Name != "ze_pr" {
		t.Errorf("table name = %q, want ze_pr", tbl.Name)
	}
	if tbl.Family != firewall.FamilyInet {
		t.Errorf("family = %v, want inet", tbl.Family)
	}
	if len(tbl.Chains) != 1 {
		t.Fatalf("expected 1 chain, got %d", len(tbl.Chains))
	}
	chain := tbl.Chains[0]
	if chain.Type != firewall.ChainRoute {
		t.Errorf("chain type = %v, want route", chain.Type)
	}
	if chain.Hook != firewall.HookPrerouting {
		t.Errorf("chain hook = %v, want prerouting", chain.Hook)
	}
	if len(chain.Terms) != 3 {
		t.Fatalf("expected 3 terms, got %d", len(chain.Terms))
	}

	// Term names are policy-rule
	if chain.Terms[0].Name != "surfprotect-bypass-dst" {
		t.Errorf("term 0 name = %q, want surfprotect-bypass-dst", chain.Terms[0].Name)
	}

	// First term should have interface match prepended
	term0 := chain.Terms[0]
	if len(term0.Matches) < 1 {
		t.Fatal("term 0 has no matches")
	}
	ifMatch, ok := term0.Matches[0].(firewall.MatchInputInterface)
	if !ok {
		t.Fatalf("first match should be MatchInputInterface, got %T", term0.Matches[0])
	}
	if ifMatch.Name != "l2tp" || !ifMatch.Wildcard {
		t.Errorf("interface match = %+v, want l2tp*", ifMatch)
	}

	// First term action: accept
	hasAccept := false
	for _, a := range term0.Actions {
		if _, ok := a.(firewall.Accept); ok {
			hasAccept = true
		}
	}
	if !hasAccept {
		t.Error("term 0 should have Accept action")
	}

	// Third term: table 100 with TCP MSS
	term2 := chain.Terms[2]
	hasMark := false
	hasMSS := false
	for _, a := range term2.Actions {
		if _, ok := a.(firewall.SetMark); ok {
			hasMark = true
		}
		if mss, ok := a.(firewall.SetTCPMSS); ok {
			hasMSS = true
			if mss.Size != 1436 {
				t.Errorf("tcp-mss = %d, want 1436", mss.Size)
			}
		}
	}
	if !hasMark {
		t.Error("term 2 should have SetMark action")
	}
	if !hasMSS {
		t.Error("term 2 should have SetTCPMSS action")
	}

	if len(result.IPRules) != 1 {
		t.Fatalf("expected 1 ip rule, got %d", len(result.IPRules))
	}
	if result.IPRules[0].Table != 100 {
		t.Errorf("ip rule table = %d, want 100", result.IPRules[0].Table)
	}
}

func TestMultiplePoliciesMergedIntoOneTable(t *testing.T) {
	policyA := PolicyRoute{
		Name:       "alpha",
		Interfaces: []InterfaceSpec{{Name: "eth0"}},
		Rules: []PolicyRule{
			{Name: "r1", Match: PolicyMatch{Protocol: "tcp"}, Action: PolicyAction{Type: ActionAccept}},
		},
	}
	policyB := PolicyRoute{
		Name:       "beta",
		Interfaces: []InterfaceSpec{{Name: "eth1"}},
		Rules: []PolicyRule{
			{Name: "r1", Match: PolicyMatch{Protocol: "udp"}, Action: PolicyAction{Type: ActionDrop}},
		},
	}

	alloc := newAllocator()
	result, err := alloc.translate([]PolicyRoute{policyA, policyB})
	if err != nil {
		t.Fatalf("translate: %v", err)
	}

	if len(result.Tables) != 1 {
		t.Fatalf("expected 1 unified table, got %d", len(result.Tables))
	}
	chain := result.Tables[0].Chains[0]
	if len(chain.Terms) != 2 {
		t.Fatalf("expected 2 terms (one per policy rule), got %d", len(chain.Terms))
	}
	if chain.Terms[0].Name != "alpha-r1" {
		t.Errorf("term 0 name = %q, want alpha-r1", chain.Terms[0].Name)
	}
	if chain.Terms[1].Name != "beta-r1" {
		t.Errorf("term 1 name = %q, want beta-r1", chain.Terms[1].Name)
	}
}

func TestEmptyPoliciesProduceNoTable(t *testing.T) {
	alloc := newAllocator()
	result, err := alloc.translate(nil)
	if err != nil {
		t.Fatalf("translate: %v", err)
	}
	if len(result.Tables) != 0 {
		t.Errorf("expected 0 tables for empty input, got %d", len(result.Tables))
	}
}

func TestPolicyNextHopToFirewallTable(t *testing.T) {
	nh := netip.MustParseAddr("10.0.0.1")
	policy := PolicyRoute{
		Name:       "redirect",
		Interfaces: []InterfaceSpec{{Name: "eth1", Wildcard: false}},
		Rules: []PolicyRule{
			{
				Name:   "web",
				Match:  PolicyMatch{DestinationPort: "80,443", Protocol: "tcp"},
				Action: PolicyAction{Type: ActionNextHop, NextHop: nh},
			},
		},
	}

	alloc := newAllocator()
	result, err := alloc.translate([]PolicyRoute{policy})
	if err != nil {
		t.Fatalf("translate: %v", err)
	}

	if len(result.IPRules) != 1 {
		t.Fatalf("expected 1 ip rule, got %d", len(result.IPRules))
	}
	rule := result.IPRules[0]
	if rule.Table < autoTableBase || rule.Table > autoTableMax {
		t.Errorf("auto table %d outside range [%d, %d]", rule.Table, autoTableBase, autoTableMax)
	}

	if len(result.AutoRoutes) != 1 {
		t.Fatalf("expected 1 auto route, got %d", len(result.AutoRoutes))
	}
	ar := result.AutoRoutes[0]
	if ar.NextHop != nh {
		t.Errorf("auto route next-hop = %s, want %s", ar.NextHop, nh)
	}
	if ar.Table != rule.Table {
		t.Errorf("auto route table = %d, ip rule table = %d, should match", ar.Table, rule.Table)
	}
}

func TestPolicyNextHopDedup(t *testing.T) {
	nh := netip.MustParseAddr("10.0.0.1")
	policy := PolicyRoute{
		Name:       "test",
		Interfaces: []InterfaceSpec{{Name: "eth0"}},
		Rules: []PolicyRule{
			{
				Name:   "r1",
				Match:  PolicyMatch{Protocol: "tcp"},
				Action: PolicyAction{Type: ActionNextHop, NextHop: nh},
			},
			{
				Name:   "r2",
				Match:  PolicyMatch{Protocol: "udp"},
				Action: PolicyAction{Type: ActionNextHop, NextHop: nh},
			},
		},
	}

	alloc := newAllocator()
	result, err := alloc.translate([]PolicyRoute{policy})
	if err != nil {
		t.Fatalf("translate: %v", err)
	}

	if len(result.IPRules) != 2 {
		t.Fatalf("expected 2 ip rules, got %d", len(result.IPRules))
	}
	if len(result.AutoRoutes) != 1 {
		t.Fatalf("expected 1 auto route (shared), got %d", len(result.AutoRoutes))
	}
	if result.IPRules[0].Table != result.IPRules[1].Table {
		t.Errorf("rules should share table: %d vs %d", result.IPRules[0].Table, result.IPRules[1].Table)
	}
}
