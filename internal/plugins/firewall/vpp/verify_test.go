package firewallvpp

import (
	"strings"
	"testing"

	"codeberg.org/thomas-mangin/ze/internal/component/firewall"
)

func baseChain(terms ...firewall.Term) firewall.Chain {
	return firewall.Chain{
		Name:   "input",
		IsBase: true,
		Type:   firewall.ChainFilter,
		Hook:   firewall.HookInput,
		Policy: firewall.PolicyDrop,
		Terms:  terms,
	}
}

func simpleTable(chains ...firewall.Chain) []firewall.Table {
	return []firewall.Table{{Name: "wan", Family: firewall.FamilyInet, Chains: chains}}
}

func TestVerifyAcceptsBasicACLConfig(t *testing.T) {
	tables := simpleTable(baseChain(firewall.Term{
		Name:    "allow-ssh",
		Matches: []firewall.Match{firewall.MatchProtocol{Protocol: "tcp"}, firewall.MatchDestinationPort{Ranges: []firewall.PortRange{{Lo: 22, Hi: 22}}}},
		Actions: []firewall.Action{firewall.Accept{}},
	}))
	if err := Verify(tables); err != nil {
		t.Fatalf("basic ACL config should be accepted, got %v", err)
	}
}

func TestVerifyAcceptsConnStateEstablishedRelated(t *testing.T) {
	tables := simpleTable(baseChain(firewall.Term{
		Name:    "allow-established",
		Matches: []firewall.Match{firewall.MatchConnState{States: firewall.ConnStateEstablished | firewall.ConnStateRelated}},
		Actions: []firewall.Action{firewall.Accept{}},
	}))
	if err := Verify(tables); err != nil {
		t.Fatalf("established+related should be accepted, got %v", err)
	}
}

func TestVerifyAcceptsConnStateEstablishedOnly(t *testing.T) {
	tables := simpleTable(baseChain(firewall.Term{
		Name:    "established-only",
		Matches: []firewall.Match{firewall.MatchConnState{States: firewall.ConnStateEstablished}},
		Actions: []firewall.Action{firewall.Accept{}},
	}))
	if err := Verify(tables); err != nil {
		t.Fatalf("established-only should be accepted, got %v", err)
	}
}

func TestVerifyRejectsConnStateNew(t *testing.T) {
	tables := simpleTable(baseChain(firewall.Term{
		Name:    "new-only",
		Matches: []firewall.Match{firewall.MatchConnState{States: firewall.ConnStateNew}},
		Actions: []firewall.Action{firewall.Accept{}},
	}))
	err := Verify(tables)
	if err == nil {
		t.Fatal("ConnStateNew should be rejected")
	}
	if !strings.Contains(err.Error(), "new/invalid not supported") {
		t.Errorf("want new/invalid message, got %v", err)
	}
}

func TestVerifyRejectsConnStateInvalid(t *testing.T) {
	tables := simpleTable(baseChain(firewall.Term{
		Name:    "invalid",
		Matches: []firewall.Match{firewall.MatchConnState{States: firewall.ConnStateInvalid}},
		Actions: []firewall.Action{firewall.Accept{}},
	}))
	err := Verify(tables)
	if err == nil {
		t.Fatal("ConnStateInvalid should be rejected")
	}
	if !strings.Contains(err.Error(), "new/invalid not supported") {
		t.Errorf("want new/invalid message, got %v", err)
	}
}

func TestVerifyRejectsConnStateMixed(t *testing.T) {
	tables := simpleTable(baseChain(firewall.Term{
		Name:    "mixed",
		Matches: []firewall.Match{firewall.MatchConnState{States: firewall.ConnStateEstablished | firewall.ConnStateNew}},
		Actions: []firewall.Action{firewall.Accept{}},
	}))
	if err := Verify(tables); err == nil {
		t.Fatal("established+new should be rejected (new has no VPP equivalent)")
	}
}

func TestVerifyRejectsNonBaseChain(t *testing.T) {
	tables := []firewall.Table{{
		Name:   "wan",
		Family: firewall.FamilyInet,
		Chains: []firewall.Chain{{
			Name:   "helper",
			IsBase: false,
			Terms:  []firewall.Term{{Name: "t1", Actions: []firewall.Action{firewall.Accept{}}}},
		}},
	}}
	err := Verify(tables)
	if err == nil {
		t.Fatal("non-base chain should be rejected")
	}
	if !strings.Contains(err.Error(), "non-base chains") {
		t.Errorf("want non-base chain message, got %v", err)
	}
}

func TestVerifyRejectsSets(t *testing.T) {
	tables := []firewall.Table{{
		Name:   "wan",
		Family: firewall.FamilyInet,
		Sets:   []firewall.Set{{Name: "blocked", Type: firewall.SetTypeIPv4}},
	}}
	err := Verify(tables)
	if err == nil {
		t.Fatal("named sets should be rejected")
	}
	if !strings.Contains(err.Error(), "named sets not supported") {
		t.Errorf("want sets message, got %v", err)
	}
}

func TestVerifyRejectsFlowtables(t *testing.T) {
	tables := []firewall.Table{{
		Name:       "wan",
		Family:     firewall.FamilyInet,
		Flowtables: []firewall.Flowtable{{Name: "ft", Hook: firewall.HookIngress}},
	}}
	err := Verify(tables)
	if err == nil {
		t.Fatal("flowtables should be rejected")
	}
	if !strings.Contains(err.Error(), "flowtables not supported") {
		t.Errorf("want flowtables message, got %v", err)
	}
}

func TestVerifyRejectsMultiRangePort(t *testing.T) {
	tables := simpleTable(baseChain(firewall.Term{
		Name: "multi-port",
		Matches: []firewall.Match{firewall.MatchDestinationPort{
			Ranges: []firewall.PortRange{{Lo: 22, Hi: 22}, {Lo: 80, Hi: 80}},
		}},
		Actions: []firewall.Action{firewall.Accept{}},
	}))
	err := Verify(tables)
	if err == nil {
		t.Fatal("multi-range port should be rejected")
	}
	if !strings.Contains(err.Error(), "ranges not supported") {
		t.Errorf("want ranges message, got %v", err)
	}
}

func TestVerifyRejectsUnsupportedMatches(t *testing.T) {
	cases := []struct {
		name  string
		match firewall.Match
		want  string
	}{
		{"input-interface", firewall.MatchInputInterface{Name: "eth0"}, "input-interface"},
		{"output-interface", firewall.MatchOutputInterface{Name: "eth0"}, "output-interface"},
		{"conn-mark", firewall.MatchConnMark{Value: 1, Mask: 1}, "connection-mark"},
		{"mark", firewall.MatchMark{Value: 1, Mask: 1}, "mark match"},
		{"dscp", firewall.MatchDSCP{Value: 46}, "dscp match"},
		{"in-set", firewall.MatchInSet{SetName: "s", MatchField: firewall.SetFieldSourceAddr}, "set match"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			tables := simpleTable(baseChain(firewall.Term{
				Name:    "t1",
				Matches: []firewall.Match{tc.match},
				Actions: []firewall.Action{firewall.Accept{}},
			}))
			err := Verify(tables)
			if err == nil {
				t.Fatalf("%s should be rejected", tc.name)
			}
			if !strings.Contains(err.Error(), tc.want) {
				t.Errorf("want %q in message, got %v", tc.want, err)
			}
		})
	}
}

func TestVerifyRejectsUnsupportedActions(t *testing.T) {
	cases := []struct {
		name   string
		action firewall.Action
		want   string
	}{
		{"reject", firewall.Reject{Type: "icmp"}, "reject action"},
		{"jump", firewall.Jump{Target: "other"}, "jump action"},
		{"goto", firewall.Goto{Target: "other"}, "goto action"},
		{"return", firewall.Return{}, "return action"},
		{"snat", firewall.SNAT{}, "snat action"},
		{"dnat", firewall.DNAT{}, "dnat action"},
		{"masquerade", firewall.Masquerade{}, "masquerade action"},
		{"redirect", firewall.Redirect{}, "redirect action"},
		{"notrack", firewall.Notrack{}, "notrack action"},
		{"set-mark", firewall.SetMark{Value: 1}, "mark-set action"},
		{"set-conn-mark", firewall.SetConnMark{Value: 1}, "connection-mark-set"},
		{"set-dscp", firewall.SetDSCP{Value: 46}, "dscp-set action"},
		{"set-tcp-mss", firewall.SetTCPMSS{Size: 1400}, "tcp-mss-set"},
		{"counter", firewall.Counter{}, "counter action"},
		{"log", firewall.Log{Prefix: "test"}, "log action"},
		{"limit", firewall.Limit{Rate: 10, Unit: "second"}, "limit action"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			tables := simpleTable(baseChain(firewall.Term{
				Name:    "t1",
				Actions: []firewall.Action{tc.action},
			}))
			err := Verify(tables)
			if err == nil {
				t.Fatalf("%s should be rejected", tc.name)
			}
			if !strings.Contains(err.Error(), tc.want) {
				t.Errorf("want %q in message, got %v", tc.want, err)
			}
		})
	}
}

func TestVerifyAcceptsFlowOffload(t *testing.T) {
	tables := simpleTable(baseChain(firewall.Term{
		Name:    "offload",
		Actions: []firewall.Action{firewall.FlowOffload{FlowtableName: "ft"}, firewall.Accept{}},
	}))
	if err := Verify(tables); err != nil {
		t.Fatalf("FlowOffload should be silently accepted, got %v", err)
	}
}

func TestVerifyRejectsLongACLTag(t *testing.T) {
	longName := strings.Repeat("x", 200)
	tables := simpleTable(firewall.Chain{
		Name:   longName,
		IsBase: true,
		Type:   firewall.ChainFilter,
		Hook:   firewall.HookInput,
		Policy: firewall.PolicyDrop,
	})
	err := Verify(tables)
	if err == nil {
		t.Fatal("expected rejection for over-long ACL tag")
	}
	if !strings.Contains(err.Error(), "exceeds") {
		t.Errorf("want 'exceeds' message, got %v", err)
	}
}

func TestVerifyReportsAllErrors(t *testing.T) {
	tables := simpleTable(baseChain(
		firewall.Term{
			Name:    "t1",
			Matches: []firewall.Match{firewall.MatchMark{Value: 1, Mask: 1}},
			Actions: []firewall.Action{firewall.Accept{}},
		},
		firewall.Term{
			Name:    "t2",
			Matches: []firewall.Match{firewall.MatchDSCP{Value: 46}},
			Actions: []firewall.Action{firewall.Drop{}},
		},
	))
	err := Verify(tables)
	if err == nil {
		t.Fatal("expected errors")
	}
	msg := err.Error()
	if !strings.Contains(msg, "t1") || !strings.Contains(msg, "t2") {
		t.Errorf("should report both terms, got %v", err)
	}
}

func TestVerifyRejectsConnStateEmpty(t *testing.T) {
	tables := simpleTable(baseChain(firewall.Term{
		Name:    "empty-state",
		Matches: []firewall.Match{firewall.MatchConnState{States: 0}},
		Actions: []firewall.Action{firewall.Accept{}},
	}))
	err := Verify(tables)
	if err == nil {
		t.Fatal("empty state mask should be rejected")
	}
	if !strings.Contains(err.Error(), "empty state mask") {
		t.Errorf("want 'empty state mask' message, got %v", err)
	}
}

func TestVerifyRejectsMultiRangeSourcePort(t *testing.T) {
	tables := simpleTable(baseChain(firewall.Term{
		Name: "multi-src-port",
		Matches: []firewall.Match{firewall.MatchSourcePort{
			Ranges: []firewall.PortRange{{Lo: 22, Hi: 22}, {Lo: 80, Hi: 80}},
		}},
		Actions: []firewall.Action{firewall.Accept{}},
	}))
	err := Verify(tables)
	if err == nil {
		t.Fatal("multi-range source port should be rejected")
	}
	if !strings.Contains(err.Error(), "ranges not supported") {
		t.Errorf("want ranges message, got %v", err)
	}
}

func TestVerifyRejectsMultipleTablesCollectsAll(t *testing.T) {
	tables := []firewall.Table{
		{Name: "t1", Family: firewall.FamilyInet, Sets: []firewall.Set{{Name: "s", Type: firewall.SetTypeIPv4}}},
		{Name: "t2", Family: firewall.FamilyInet, Flowtables: []firewall.Flowtable{{Name: "ft", Hook: firewall.HookIngress}}},
	}
	err := Verify(tables)
	if err == nil {
		t.Fatal("expected errors from both tables")
	}
	msg := err.Error()
	if !strings.Contains(msg, "t1") || !strings.Contains(msg, "t2") {
		t.Errorf("should report errors from both tables, got %v", err)
	}
}
