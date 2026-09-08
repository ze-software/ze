package policyroute

import (
	"net/netip"
	"testing"

	"github.com/ze-software/ze/internal/component/firewall"
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
	// CORRECTED: this asserted ChainRoute, which the kernel cannot load at
	// prerouting -- `type route` is OUTPUT-only. The green unit test is why the
	// defect shipped: translation matched its own expectation while producing a
	// chain nftables rejects with EOPNOTSUPP, taking the plugin down at startup.
	// This is not a relaxation; the previous expectation was unsatisfiable.
	if chain.Type != firewall.ChainFilter {
		t.Errorf("chain type = %v, want filter (route is output-only)", chain.Type)
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

// interfaceMatches returns every input-interface match a term carries, so a
// test can assert both how many there are and which interface each names.
func interfaceMatches(term firewall.Term) []firewall.MatchInputInterface {
	var out []firewall.MatchInputInterface
	for _, m := range term.Matches {
		if im, ok := m.(firewall.MatchInputInterface); ok {
			out = append(out, im)
		}
	}
	return out
}

// TestPolicyInterfaceListOneTermPerInterface pins the OR the leaf-list
// promises. A packet arrives on one interface and carries one name, so two
// interface matches inside one term match no packet at all: the alternatives
// have to be separate terms, which nftables programs as separate rules.
func TestPolicyInterfaceListOneTermPerInterface(t *testing.T) {
	policy := PolicyRoute{
		Name:       "wan",
		Interfaces: []InterfaceSpec{{Name: "eth0"}, {Name: "l2tp", Wildcard: true}},
		Rules: []PolicyRule{
			{
				Name:   "mark",
				Match:  PolicyMatch{Protocol: "tcp"},
				Action: PolicyAction{Type: ActionTable, Table: 100},
			},
		},
	}

	alloc := newAllocator()
	result, err := alloc.translate([]PolicyRoute{policy})
	if err != nil {
		t.Fatalf("translate: %v", err)
	}

	chain := result.Tables[0].Chains[0]
	if len(chain.Terms) != 2 {
		t.Fatalf("expected 2 terms (one per interface), got %d", len(chain.Terms))
	}

	wantNames := []string{"wan-mark-1", "wan-mark-2"}
	wantIfaces := []firewall.MatchInputInterface{
		{Name: "eth0"},
		{Name: "l2tp", Wildcard: true},
	}
	for i := range chain.Terms {
		term := chain.Terms[i]
		if term.Name != wantNames[i] {
			t.Errorf("term %d name = %q, want %q", i, term.Name, wantNames[i])
		}
		ifaces := interfaceMatches(term)
		if len(ifaces) != 1 {
			t.Fatalf("term %d carries %d interface matches, want 1 (a rule ANDs its matches)", i, len(ifaces))
		}
		if ifaces[0] != wantIfaces[i] {
			t.Errorf("term %d interface match = %+v, want %+v", i, ifaces[0], wantIfaces[i])
		}
		if _, ok := term.Matches[0].(firewall.MatchInputInterface); !ok {
			t.Errorf("term %d first match = %T, want the interface match prepended", i, term.Matches[0])
		}
		if _, ok := term.Matches[1].(firewall.MatchProtocol); !ok {
			t.Errorf("term %d second match = %T, want the rule's protocol match", i, term.Matches[1])
		}
	}

	if len(result.IPRules) != 1 {
		t.Fatalf("expected 1 ip rule for the one rule that selects a table, got %d", len(result.IPRules))
	}
	if result.IPRules[0].Table != 100 {
		t.Errorf("ip rule table = %d, want 100", result.IPRules[0].Table)
	}

	marks := make([]uint32, 0, 2)
	for i := range chain.Terms {
		for _, a := range chain.Terms[i].Actions {
			if sm, ok := a.(firewall.SetMark); ok {
				marks = append(marks, sm.Value)
			}
		}
	}
	if len(marks) != 2 {
		t.Fatalf("expected each term to set the mark, got %d SetMark actions", len(marks))
	}
	if marks[0] != marks[1] {
		t.Errorf("terms set marks %#x and %#x; both must carry the mark the one ip rule looks up", marks[0], marks[1])
	}
	if marks[0] != result.IPRules[0].Mark {
		t.Errorf("term mark %#x, ip rule mark %#x", marks[0], result.IPRules[0].Mark)
	}
}

// TestPolicyWithoutInterfaceMatchesEveryIngress covers the documented shape of
// a policy that names no interface: one term, and no interface match to narrow
// it.
func TestPolicyWithoutInterfaceMatchesEveryIngress(t *testing.T) {
	policy := PolicyRoute{
		Name: "any",
		Rules: []PolicyRule{
			{Name: "drop-udp", Match: PolicyMatch{Protocol: "udp"}, Action: PolicyAction{Type: ActionDrop}},
		},
	}

	alloc := newAllocator()
	result, err := alloc.translate([]PolicyRoute{policy})
	if err != nil {
		t.Fatalf("translate: %v", err)
	}

	chain := result.Tables[0].Chains[0]
	if len(chain.Terms) != 1 {
		t.Fatalf("expected 1 term, got %d", len(chain.Terms))
	}
	if chain.Terms[0].Name != "any-drop-udp" {
		t.Errorf("term name = %q, want any-drop-udp", chain.Terms[0].Name)
	}
	if got := len(interfaceMatches(chain.Terms[0])); got != 0 {
		t.Errorf("term carries %d interface matches, want 0", got)
	}
}

// TestPolicyInterfaceListKeepsRuleOrder proves the interface group does not
// disturb the sequence the order leaf establishes. The method is the whole
// config path, because the sort by order lives in parsePolicyRoute rather than
// in translate. The config gives the order 10 rule first. The four terms must
// still come out as the order 0 group, then the order 10 group, each group
// contiguous.
func TestPolicyInterfaceListKeepsRuleOrder(t *testing.T) {
	input := `{
		"policy": {
			"route": {
				"steer": {
					"interface": ["eth0", "l2tp*"],
					"rule": {
						"late": {
							"order": "10",
							"from": { "protocol": "udp" },
							"then": { "drop": "" }
						},
						"early": {
							"order": "0",
							"from": { "protocol": "tcp" },
							"then": { "accept": "" }
						}
					}
				}
			}
		}
	}`

	policies, err := parsePolicyConfig(input)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}

	alloc := newAllocator()
	result, err := alloc.translate(policies)
	if err != nil {
		t.Fatalf("translate: %v", err)
	}

	chain := result.Tables[0].Chains[0]
	want := []string{"steer-early-1", "steer-early-2", "steer-late-1", "steer-late-2"}
	if len(chain.Terms) != len(want) {
		t.Fatalf("expected %d terms (2 rules x 2 interfaces), got %d", len(want), len(chain.Terms))
	}
	for i := range want {
		if chain.Terms[i].Name != want[i] {
			t.Errorf("term %d name = %q, want %q", i, chain.Terms[i].Name, want[i])
		}
	}

	// Each group carries the two interfaces in leaf-list order, and each term
	// carries exactly one of them.
	wantIfaces := []firewall.MatchInputInterface{
		{Name: "eth0"},
		{Name: "l2tp", Wildcard: true},
		{Name: "eth0"},
		{Name: "l2tp", Wildcard: true},
	}
	for i := range chain.Terms {
		ifaces := interfaceMatches(chain.Terms[i])
		if len(ifaces) != 1 {
			t.Fatalf("term %d carries %d interface matches, want 1 (a rule ANDs its matches)", i, len(ifaces))
		}
		if ifaces[0] != wantIfaces[i] {
			t.Errorf("term %d interface match = %+v, want %+v", i, ifaces[0], wantIfaces[i])
		}
	}
}
