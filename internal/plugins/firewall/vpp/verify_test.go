package firewallvpp

import (
	"net/netip"
	"strings"
	"testing"

	"github.com/ze-software/ze/internal/component/firewall"
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
		{"snat", firewall.SNAT{}, "requires a NAT chain"},
		{"dnat", firewall.DNAT{}, "requires a NAT chain"},
		{"masquerade", firewall.Masquerade{}, "requires a NAT chain"},
		{"redirect", firewall.Redirect{}, "redirect action"},
		{"notrack", firewall.Notrack{}, "notrack action"},
		{"set-conn-mark", firewall.SetConnMark{Value: 1}, "connection-mark-set"},
		{"set-dscp", firewall.SetDSCP{Value: 46}, "dscp-set action"},
		{"set-tcp-mss", firewall.SetTCPMSS{Size: 1400}, "tcp-mss-set"},
		{"counter", firewall.Counter{}, "counter action"},
		{"log", firewall.Log{Prefix: "test"}, "log action"},
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

func natChain(hook firewall.ChainHook, terms ...firewall.Term) firewall.Chain {
	return firewall.Chain{
		Name:   "natchain",
		IsBase: true,
		Type:   firewall.ChainNAT,
		Hook:   hook,
		Policy: firewall.PolicyAccept,
		Terms:  terms,
	}
}

func natTable(chains ...firewall.Chain) []firewall.Table {
	return []firewall.Table{{Name: "nat", Family: firewall.FamilyInet, Chains: chains}}
}

func TestVerifyAcceptsMasqueradeInNATChain(t *testing.T) {
	tables := natTable(natChain(firewall.HookPostrouting, firewall.Term{
		Name:    "masq",
		Actions: []firewall.Action{firewall.Masquerade{}},
	}))
	if err := Verify(tables); err != nil {
		t.Fatalf("masquerade in NAT chain should be accepted, got %v", err)
	}
}

func TestVerifyAcceptsSNATInNATChain(t *testing.T) {
	tables := natTable(natChain(firewall.HookPostrouting, firewall.Term{
		Name:    "snat",
		Actions: []firewall.Action{firewall.SNAT{Address: netip.MustParseAddr("1.2.3.4")}},
	}))
	if err := Verify(tables); err != nil {
		t.Fatalf("SNAT in NAT chain should be accepted, got %v", err)
	}
}

func TestVerifyAcceptsDNATInNATChain(t *testing.T) {
	tables := natTable(natChain(firewall.HookPrerouting, firewall.Term{
		Name: "dnat",
		Matches: []firewall.Match{
			firewall.MatchProtocol{Protocol: "tcp"},
			firewall.MatchDestinationPort{Ranges: []firewall.PortRange{{Lo: 80, Hi: 80}}},
		},
		Actions: []firewall.Action{firewall.DNAT{Address: netip.MustParseAddr("10.0.0.1"), Port: 8080}},
	}))
	if err := Verify(tables); err != nil {
		t.Fatalf("DNAT with protocol+port match in NAT chain should be accepted, got %v", err)
	}
}

func TestVerifyRejectsSNATWithDestMatch(t *testing.T) {
	tables := natTable(natChain(firewall.HookPostrouting, firewall.Term{
		Name:    "snat-filtered",
		Matches: []firewall.Match{firewall.MatchDestinationPort{Ranges: []firewall.PortRange{{Lo: 80, Hi: 80}}}},
		Actions: []firewall.Action{firewall.SNAT{Address: netip.MustParseAddr("1.2.3.4")}},
	}))
	err := Verify(tables)
	if err == nil {
		t.Fatal("SNAT with destination match should be rejected")
	}
	if !strings.Contains(err.Error(), "destination/protocol match not supported with snat") {
		t.Errorf("want destination match rejection message, got %v", err)
	}
}

func TestVerifyRejectsNATTermWithoutNATAction(t *testing.T) {
	tables := natTable(natChain(firewall.HookPostrouting, firewall.Term{
		Name:    "filter-in-nat",
		Actions: []firewall.Action{firewall.Accept{}},
	}))
	err := Verify(tables)
	if err == nil {
		t.Fatal("NAT chain term without NAT action should be rejected")
	}
	if !strings.Contains(err.Error(), "no NAT action") {
		t.Errorf("want 'no NAT action' message, got %v", err)
	}
}

func TestVerifyRejectsUnsupportedActionInNATChain(t *testing.T) {
	tables := natTable(natChain(firewall.HookPostrouting, firewall.Term{
		Name:    "limit-in-nat",
		Actions: []firewall.Action{firewall.Masquerade{}, firewall.Limit{Rate: 10, Unit: "second"}},
	}))
	err := Verify(tables)
	if err == nil {
		t.Fatal("limit in NAT chain should be rejected")
	}
	if !strings.Contains(err.Error(), "not supported in NAT chain") {
		t.Errorf("want 'not supported in NAT chain' message, got %v", err)
	}
}

func TestVerifyRejectsUnsupportedMatchWithDNAT(t *testing.T) {
	tables := natTable(natChain(firewall.HookPrerouting, firewall.Term{
		Name:    "dnat-mark",
		Matches: []firewall.Match{firewall.MatchMark{Value: 1, Mask: 1}},
		Actions: []firewall.Action{firewall.DNAT{Address: netip.MustParseAddr("10.0.0.1"), Port: 80}},
	}))
	err := Verify(tables)
	if err == nil {
		t.Fatal("DNAT with mark match should be rejected")
	}
	if !strings.Contains(err.Error(), "not supported with dnat") {
		t.Errorf("want 'not supported with dnat' message, got %v", err)
	}
}

func TestVerifyRejectsDNATWithoutProtocol(t *testing.T) {
	tables := natTable(natChain(firewall.HookPrerouting, firewall.Term{
		Name:    "dnat-no-proto",
		Matches: []firewall.Match{firewall.MatchDestinationPort{Ranges: []firewall.PortRange{{Lo: 80, Hi: 80}}}},
		Actions: []firewall.Action{firewall.DNAT{Address: netip.MustParseAddr("10.0.0.1"), Port: 8080}},
	}))
	err := Verify(tables)
	if err == nil {
		t.Fatal("DNAT without protocol should be rejected")
	}
	if !strings.Contains(err.Error(), "dnat requires a protocol match") {
		t.Errorf("want protocol requirement message, got %v", err)
	}
}

func TestVerifyRejectsDNATWithoutPort(t *testing.T) {
	tables := natTable(natChain(firewall.HookPrerouting, firewall.Term{
		Name:    "dnat-no-port",
		Matches: []firewall.Match{firewall.MatchProtocol{Protocol: "tcp"}},
		Actions: []firewall.Action{firewall.DNAT{Address: netip.MustParseAddr("10.0.0.1"), Port: 8080}},
	}))
	err := Verify(tables)
	if err == nil {
		t.Fatal("DNAT without destination port should be rejected")
	}
	if !strings.Contains(err.Error(), "dnat requires a destination port match") {
		t.Errorf("want port requirement message, got %v", err)
	}
}

func TestVerifyRejectsIPv6SNATAddress(t *testing.T) {
	tables := natTable(natChain(firewall.HookPostrouting, firewall.Term{
		Name:    "snat-v6",
		Actions: []firewall.Action{firewall.SNAT{Address: netip.MustParseAddr("2001:db8::1")}},
	}))
	err := Verify(tables)
	if err == nil {
		t.Fatal("IPv6 SNAT address should be rejected")
	}
	if !strings.Contains(err.Error(), "IPv6") {
		t.Errorf("want IPv6 rejection message, got %v", err)
	}
}

func TestVerifyRejectsIPv6DNATAddress(t *testing.T) {
	tables := natTable(natChain(firewall.HookPrerouting, firewall.Term{
		Name: "dnat-v6",
		Matches: []firewall.Match{
			firewall.MatchProtocol{Protocol: "tcp"},
			firewall.MatchDestinationPort{Ranges: []firewall.PortRange{{Lo: 80, Hi: 80}}},
		},
		Actions: []firewall.Action{firewall.DNAT{Address: netip.MustParseAddr("2001:db8::1"), Port: 8080}},
	}))
	err := Verify(tables)
	if err == nil {
		t.Fatal("IPv6 DNAT address should be rejected")
	}
	if !strings.Contains(err.Error(), "IPv6") {
		t.Errorf("want IPv6 rejection message, got %v", err)
	}
}

func TestVerifyAcceptsSetMarkInFilterChain(t *testing.T) {
	tables := simpleTable(baseChain(firewall.Term{
		Name:    "mark-voip",
		Matches: []firewall.Match{firewall.MatchProtocol{Protocol: "udp"}},
		Actions: []firewall.Action{firewall.SetMark{Value: 0x10, Mask: 0xffffffff}, firewall.Accept{}},
	}))
	if err := Verify(tables); err != nil {
		t.Fatalf("SetMark in filter chain should be accepted, got %v", err)
	}
}

func TestVerifyAcceptsLimitInFilterChain(t *testing.T) {
	tables := simpleTable(baseChain(firewall.Term{
		Name:    "rate-limit",
		Matches: []firewall.Match{firewall.MatchProtocol{Protocol: "tcp"}, firewall.MatchDestinationPort{Ranges: []firewall.PortRange{{Lo: 22, Hi: 22}}}},
		Actions: []firewall.Action{firewall.Limit{Rate: 100, Unit: "second"}, firewall.Accept{}},
	}))
	if err := Verify(tables); err != nil {
		t.Fatalf("Limit in filter chain should be accepted, got %v", err)
	}
}

func TestVerifyRejectsPortRangeWithSetMark(t *testing.T) {
	tables := simpleTable(baseChain(firewall.Term{
		Name:    "mark-range",
		Matches: []firewall.Match{firewall.MatchDestinationPort{Ranges: []firewall.PortRange{{Lo: 1024, Hi: 65535}}}},
		Actions: []firewall.Action{firewall.SetMark{Value: 0x10}, firewall.Accept{}},
	}))
	err := Verify(tables)
	if err == nil {
		t.Fatal("port range with set-mark should be rejected (classify needs exact port)")
	}
	if !strings.Contains(err.Error(), "port range") {
		t.Errorf("want port range message, got %v", err)
	}
}

func TestVerifyRejectsPortRangeWithLimit(t *testing.T) {
	tables := simpleTable(baseChain(firewall.Term{
		Name:    "limit-range",
		Matches: []firewall.Match{firewall.MatchDestinationPort{Ranges: []firewall.PortRange{{Lo: 80, Hi: 443}}}},
		Actions: []firewall.Action{firewall.Limit{Rate: 10, Unit: "second"}, firewall.Accept{}},
	}))
	err := Verify(tables)
	if err == nil {
		t.Fatal("port range with limit should be rejected")
	}
	if !strings.Contains(err.Error(), "port range") {
		t.Errorf("want port range message, got %v", err)
	}
}

func TestVerifyRejectsLongClassifyPolicerName(t *testing.T) {
	longName := strings.Repeat("x", 50)
	tables := simpleTable(firewall.Chain{
		Name:   "chain",
		IsBase: true,
		Type:   firewall.ChainFilter,
		Hook:   firewall.HookInput,
		Policy: firewall.PolicyDrop,
		Terms: []firewall.Term{{
			Name:    longName,
			Actions: []firewall.Action{firewall.Limit{Rate: 10, Unit: "second"}, firewall.Accept{}},
		}},
	})
	err := Verify(tables)
	if err == nil {
		t.Fatal("long classify policer name should be rejected")
	}
	if !strings.Contains(err.Error(), "exceeds") {
		t.Errorf("want exceeds message, got %v", err)
	}
}

func TestVerifyRejectsIPv6WithSetMark(t *testing.T) {
	tables := simpleTable(baseChain(firewall.Term{
		Name:    "mark-v6",
		Matches: []firewall.Match{firewall.MatchSourceAddress{Prefix: netip.MustParsePrefix("2001:db8::/32")}},
		Actions: []firewall.Action{firewall.SetMark{Value: 0x10}, firewall.Accept{}},
	}))
	err := Verify(tables)
	if err == nil {
		t.Fatal("IPv6 source address with set-mark should be rejected")
	}
	if !strings.Contains(err.Error(), "IPv6 source address") {
		t.Errorf("want IPv6 source address message, got %v", err)
	}
}

func TestVerifyRejectsIPv6DstWithLimit(t *testing.T) {
	tables := simpleTable(baseChain(firewall.Term{
		Name:    "limit-v6",
		Matches: []firewall.Match{firewall.MatchDestinationAddress{Prefix: netip.MustParsePrefix("2001:db8::1/128")}},
		Actions: []firewall.Action{firewall.Limit{Rate: 100, Unit: "second"}, firewall.Accept{}},
	}))
	err := Verify(tables)
	if err == nil {
		t.Fatal("IPv6 destination address with limit should be rejected")
	}
	if !strings.Contains(err.Error(), "IPv6 destination address") {
		t.Errorf("want IPv6 destination address message, got %v", err)
	}
}

func TestVerifyRejectsMasqueradePorts(t *testing.T) {
	tables := natTable(natChain(firewall.HookPostrouting, firewall.Term{
		Name:    "masq-ports",
		Actions: []firewall.Action{firewall.Masquerade{Port: 1024, PortEnd: 65535}},
	}))
	err := Verify(tables)
	if err == nil {
		t.Fatal("masquerade with port mapping should be rejected by VPP backend")
	}
	if !strings.Contains(err.Error(), "port mapping not supported") {
		t.Errorf("want 'port mapping not supported', got %v", err)
	}
}

func TestVerifyRejectsMasqueradeFlags(t *testing.T) {
	tables := natTable(natChain(firewall.HookPostrouting, firewall.Term{
		Name:    "masq-flags",
		Actions: []firewall.Action{firewall.Masquerade{Flags: firewall.MasqFlagRandom}},
	}))
	err := Verify(tables)
	if err == nil {
		t.Fatal("masquerade with flags should be rejected by VPP backend")
	}
	if !strings.Contains(err.Error(), "flags not supported") {
		t.Errorf("want 'flags not supported', got %v", err)
	}
}

func TestVerifyRejectsSetMarkAndLimitCombined(t *testing.T) {
	tables := simpleTable(baseChain(firewall.Term{
		Name:    "mark-and-limit",
		Matches: []firewall.Match{firewall.MatchProtocol{Protocol: "tcp"}},
		Actions: []firewall.Action{
			firewall.SetMark{Value: 0x10},
			firewall.Limit{Rate: 100, Unit: "second"},
			firewall.Accept{},
		},
	}))
	err := Verify(tables)
	if err == nil {
		t.Fatal("SetMark + Limit on same term should be rejected")
	}
	if !strings.Contains(err.Error(), "cannot be combined") {
		t.Errorf("want 'cannot be combined' message, got %v", err)
	}
}
