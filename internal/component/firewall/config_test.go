package firewall

import (
	"net/netip"
	"testing"
)

// VALIDATES: AC-11 "Config table name wan produces Table with Name ze_wan".
// PREVENTS: missing ze_ prefix on table names.
func TestTableNamePrefix(t *testing.T) {
	data := `{"firewall":{"table":{"wan":{"family":"inet"}}}}`
	tables, err := ParseFirewallConfig(data)
	if err != nil {
		t.Fatalf("ParseFirewallConfig: %v", err)
	}
	if len(tables) != 1 {
		t.Fatalf("got %d tables, want 1", len(tables))
	}
	if tables[0].Name != "ze_wan" {
		t.Errorf("Table.Name = %q, want %q", tables[0].Name, "ze_wan")
	}
}

// VALIDATES: AC-1 "Config term with destination port 80 parsed to MatchDestinationPort(80)".
// PREVENTS: from-block keyword parsing broken.
func TestParseFromDestinationPort(t *testing.T) {
	data := `{"firewall":{"table":{"wan":{"family":"inet","chain":{"input":{"type":"filter","hook":"input","priority":"0","policy":"drop","term":{"allow-http":{"from":{"destination-port":"80"}}}}}}}}}`
	tables, err := ParseFirewallConfig(data)
	if err != nil {
		t.Fatalf("ParseFirewallConfig: %v", err)
	}
	chain := tables[0].Chains[0]
	if len(chain.Terms) != 1 {
		t.Fatalf("got %d terms, want 1", len(chain.Terms))
	}
	term := chain.Terms[0]
	if len(term.Matches) != 1 {
		t.Fatalf("got %d matches, want 1", len(term.Matches))
	}
	m, ok := term.Matches[0].(MatchDestinationPort)
	if !ok {
		t.Fatalf("match type = %T, want MatchDestinationPort", term.Matches[0])
	}
	if m.Port != 80 {
		t.Errorf("MatchDestinationPort.Port = %d, want 80", m.Port)
	}
}

// VALIDATES: AC-2 "Config term with source address 10.0.0.0/8 parsed to MatchSourceAddress".
// PREVENTS: source address parsing broken.
func TestParseFromSourceAddress(t *testing.T) {
	data := `{"firewall":{"table":{"wan":{"family":"inet","chain":{"input":{"term":{"block-rfc1918":{"from":{"source-address":"10.0.0.0/8"}}}}}}}}}`
	tables, err := ParseFirewallConfig(data)
	if err != nil {
		t.Fatalf("ParseFirewallConfig: %v", err)
	}
	term := tables[0].Chains[0].Terms[0]
	m, ok := term.Matches[0].(MatchSourceAddress)
	if !ok {
		t.Fatalf("match type = %T, want MatchSourceAddress", term.Matches[0])
	}
	want := netip.MustParsePrefix("10.0.0.0/8")
	if m.Prefix != want {
		t.Errorf("Prefix = %v, want %v", m.Prefix, want)
	}
}

// VALIDATES: AC-3 "Config term with connection state established,related".
// PREVENTS: connection state bitmask parsing broken.
func TestParseFromConnState(t *testing.T) {
	data := `{"firewall":{"table":{"wan":{"family":"inet","chain":{"input":{"term":{"allow-est":{"from":{"connection-state":"established,related"}}}}}}}}}`
	tables, err := ParseFirewallConfig(data)
	if err != nil {
		t.Fatalf("ParseFirewallConfig: %v", err)
	}
	term := tables[0].Chains[0].Terms[0]
	m, ok := term.Matches[0].(MatchConnState)
	if !ok {
		t.Fatalf("match type = %T, want MatchConnState", term.Matches[0])
	}
	want := ConnStateEstablished | ConnStateRelated
	if m.States != want {
		t.Errorf("States = %d, want %d", m.States, want)
	}
}

// VALIDATES: AC-7 "Config terms with accept, drop, reject".
// PREVENTS: verdict action parsing broken.
func TestParseThenVerdicts(t *testing.T) {
	data := `{"firewall":{"table":{"wan":{"family":"inet","chain":{"input":{"term":{"rule1":{"then":{"accept":""}}}}}}}}}`
	tables, err := ParseFirewallConfig(data)
	if err != nil {
		t.Fatalf("ParseFirewallConfig: %v", err)
	}
	term := tables[0].Chains[0].Terms[0]
	if len(term.Actions) != 1 {
		t.Fatalf("got %d actions, want 1", len(term.Actions))
	}
	if _, ok := term.Actions[0].(Accept); !ok {
		t.Errorf("action type = %T, want Accept", term.Actions[0])
	}
}

func TestParseThenDrop(t *testing.T) {
	data := `{"firewall":{"table":{"wan":{"family":"inet","chain":{"input":{"term":{"rule1":{"then":{"drop":""}}}}}}}}}`
	tables, err := ParseFirewallConfig(data)
	if err != nil {
		t.Fatalf("ParseFirewallConfig: %v", err)
	}
	term := tables[0].Chains[0].Terms[0]
	if _, ok := term.Actions[0].(Drop); !ok {
		t.Errorf("action type = %T, want Drop", term.Actions[0])
	}
}

func TestParseThenReject(t *testing.T) {
	data := `{"firewall":{"table":{"wan":{"family":"inet","chain":{"input":{"term":{"rule1":{"then":{"reject":{"with":"icmp","code":"3"}}}}}}}}}}`
	tables, err := ParseFirewallConfig(data)
	if err != nil {
		t.Fatalf("ParseFirewallConfig: %v", err)
	}
	term := tables[0].Chains[0].Terms[0]
	r, ok := term.Actions[0].(Reject)
	if !ok {
		t.Fatalf("action type = %T, want Reject", term.Actions[0])
	}
	if r.Type != "icmp" || r.Code != 3 {
		t.Errorf("Reject = {%q %d}, want {icmp 3}", r.Type, r.Code)
	}
}

// VALIDATES: AC-6 "Config term with mark set 0x10".
// PREVENTS: modifier parsing broken.
func TestParseThenMarkSet(t *testing.T) {
	data := `{"firewall":{"table":{"wan":{"family":"inet","chain":{"input":{"term":{"rule1":{"then":{"mark-set":{"value":"0x10"}}}}}}}}}}`
	tables, err := ParseFirewallConfig(data)
	if err != nil {
		t.Fatalf("ParseFirewallConfig: %v", err)
	}
	term := tables[0].Chains[0].Terms[0]
	if len(term.Actions) != 1 {
		t.Fatalf("got %d actions, want 1", len(term.Actions))
	}
	sm, ok := term.Actions[0].(SetMark)
	if !ok {
		t.Fatalf("action type = %T, want SetMark", term.Actions[0])
	}
	if sm.Value != 0x10 {
		t.Errorf("SetMark.Value = 0x%x, want 0x10", sm.Value)
	}
}

// VALIDATES: AC-4 "Config term with limit rate 10/second".
// PREVENTS: rate limit parsing broken.
func TestParseThenLimitRate(t *testing.T) {
	data := `{"firewall":{"table":{"wan":{"family":"inet","chain":{"input":{"term":{"rule1":{"then":{"limit-rate":{"rate":"10/second","burst":"5"}}}}}}}}}}`
	tables, err := ParseFirewallConfig(data)
	if err != nil {
		t.Fatalf("ParseFirewallConfig: %v", err)
	}
	term := tables[0].Chains[0].Terms[0]
	lim, ok := term.Actions[0].(Limit)
	if !ok {
		t.Fatalf("action type = %T, want Limit", term.Actions[0])
	}
	if lim.Rate != 10 || lim.Unit != "second" || lim.Burst != 5 {
		t.Errorf("Limit = {Rate:%d Unit:%s Burst:%d}, want {10 second 5}", lim.Rate, lim.Unit, lim.Burst)
	}
}

// VALIDATES: AC-5 "Config term with log prefix".
// PREVENTS: log action parsing broken.
func TestParseThenLog(t *testing.T) {
	data := `{"firewall":{"table":{"wan":{"family":"inet","chain":{"input":{"term":{"rule1":{"then":{"log":{"prefix":"DROP: "}}}}}}}}}}`
	tables, err := ParseFirewallConfig(data)
	if err != nil {
		t.Fatalf("ParseFirewallConfig: %v", err)
	}
	term := tables[0].Chains[0].Terms[0]
	lg, ok := term.Actions[0].(Log)
	if !ok {
		t.Fatalf("action type = %T, want Log", term.Actions[0])
	}
	if lg.Prefix != "DROP: " {
		t.Errorf("Log.Prefix = %q, want %q", lg.Prefix, "DROP: ")
	}
}

// VALIDATES: AC-10 "Config term with source address @blocked".
// PREVENTS: set reference parsing broken.
func TestParseFromSetReference(t *testing.T) {
	data := `{"firewall":{"table":{"wan":{"family":"inet","chain":{"input":{"term":{"rule1":{"from":{"source-address":"@blocked"}}}}}}}}}`
	tables, err := ParseFirewallConfig(data)
	if err != nil {
		t.Fatalf("ParseFirewallConfig: %v", err)
	}
	term := tables[0].Chains[0].Terms[0]
	m, ok := term.Matches[0].(MatchInSet)
	if !ok {
		t.Fatalf("match type = %T, want MatchInSet", term.Matches[0])
	}
	if m.SetName != "blocked" || m.MatchField != SetFieldSourceAddr {
		t.Errorf("MatchInSet = {%q %v}, want {blocked SetFieldSourceAddr}", m.SetName, m.MatchField)
	}
}

// VALIDATES: AC-9 "Config term with flow offload @flowtable-name".
// PREVENTS: flow offload parsing broken.
func TestParseThenFlowOffload(t *testing.T) {
	data := `{"firewall":{"table":{"wan":{"family":"inet","chain":{"fwd":{"term":{"offload":{"then":{"flow-offload":{"flowtable":"ft0"}}}}}}}}}}`
	tables, err := ParseFirewallConfig(data)
	if err != nil {
		t.Fatalf("ParseFirewallConfig: %v", err)
	}
	term := tables[0].Chains[0].Terms[0]
	fo, ok := term.Actions[0].(FlowOffload)
	if !ok {
		t.Fatalf("action type = %T, want FlowOffload", term.Actions[0])
	}
	if fo.FlowtableName != "ft0" {
		t.Errorf("FlowtableName = %q, want %q", fo.FlowtableName, "ft0")
	}
}

// VALIDATES: AC-16 "Base chain parsed with type, hook, priority, policy".
// PREVENTS: base chain fields lost during parsing.
func TestParseBaseChain(t *testing.T) {
	data := `{"firewall":{"table":{"wan":{"family":"inet","chain":{"input":{"type":"filter","hook":"input","priority":"0","policy":"drop"}}}}}}`
	tables, err := ParseFirewallConfig(data)
	if err != nil {
		t.Fatalf("ParseFirewallConfig: %v", err)
	}
	chain := tables[0].Chains[0]
	if !chain.IsBase {
		t.Error("expected base chain")
	}
	if chain.Type != ChainFilter {
		t.Errorf("Type = %v, want filter", chain.Type)
	}
	if chain.Hook != HookInput {
		t.Errorf("Hook = %v, want input", chain.Hook)
	}
	if chain.Policy != PolicyDrop {
		t.Errorf("Policy = %v, want drop", chain.Policy)
	}
}

// VALIDATES: AC-13 "Invalid config rejected".
// PREVENTS: invalid families accepted silently.
func TestParseInvalidFamily(t *testing.T) {
	data := `{"firewall":{"table":{"wan":{"family":"invalid"}}}}`
	_, err := ParseFirewallConfig(data)
	if err == nil {
		t.Fatal("expected error for invalid family")
	}
}

// VALIDATES: AC-17 "Empty table list returns nil slice".
// PREVENTS: crash on empty firewall section.
func TestParseEmptyFirewall(t *testing.T) {
	data := `{"firewall":{}}`
	tables, err := ParseFirewallConfig(data)
	if err != nil {
		t.Fatalf("ParseFirewallConfig: %v", err)
	}
	if len(tables) != 0 {
		t.Errorf("got %d tables, want 0", len(tables))
	}
}

// VALIDATES: AC-18 "No firewall section returns nil".
// PREVENTS: crash when firewall section absent.
func TestParseNoFirewallSection(t *testing.T) {
	data := `{}`
	tables, err := ParseFirewallConfig(data)
	if err != nil {
		t.Fatalf("ParseFirewallConfig: %v", err)
	}
	if tables != nil {
		t.Errorf("got %v, want nil", tables)
	}
}

// VALIDATES: AC-8 "Config terms with snat/dnat/masquerade".
// PREVENTS: NAT action parsing broken.
func TestParseThenSNAT(t *testing.T) {
	data := `{"firewall":{"table":{"nat":{"family":"inet","chain":{"post":{"term":{"masq":{"then":{"masquerade":""}}}}}}}}}`
	tables, err := ParseFirewallConfig(data)
	if err != nil {
		t.Fatalf("ParseFirewallConfig: %v", err)
	}
	term := tables[0].Chains[0].Terms[0]
	if _, ok := term.Actions[0].(Masquerade); !ok {
		t.Errorf("action type = %T, want Masquerade", term.Actions[0])
	}
}

// VALIDATES: from-block protocol keyword.
// PREVENTS: protocol match parsing broken.
func TestParseFromProtocol(t *testing.T) {
	data := `{"firewall":{"table":{"wan":{"family":"inet","chain":{"input":{"term":{"rule1":{"from":{"protocol":"tcp"}}}}}}}}}`
	tables, err := ParseFirewallConfig(data)
	if err != nil {
		t.Fatalf("ParseFirewallConfig: %v", err)
	}
	term := tables[0].Chains[0].Terms[0]
	m, ok := term.Matches[0].(MatchProtocol)
	if !ok {
		t.Fatalf("match type = %T, want MatchProtocol", term.Matches[0])
	}
	if m.Protocol != "tcp" {
		t.Errorf("Protocol = %q, want %q", m.Protocol, "tcp")
	}
}

// VALIDATES: then-block counter keyword.
// PREVENTS: counter action parsing broken.
func TestParseThenCounter(t *testing.T) {
	data := `{"firewall":{"table":{"wan":{"family":"inet","chain":{"input":{"term":{"rule1":{"then":{"counter":"my-counter"}}}}}}}}}`
	tables, err := ParseFirewallConfig(data)
	if err != nil {
		t.Fatalf("ParseFirewallConfig: %v", err)
	}
	term := tables[0].Chains[0].Terms[0]
	c, ok := term.Actions[0].(Counter)
	if !ok {
		t.Fatalf("action type = %T, want Counter", term.Actions[0])
	}
	if c.Name != "my-counter" {
		t.Errorf("Counter.Name = %q, want %q", c.Name, "my-counter")
	}
}

// Boundary: port range parsing.
func TestParsePortRange(t *testing.T) {
	data := `{"firewall":{"table":{"wan":{"family":"inet","chain":{"input":{"term":{"rule1":{"from":{"destination-port":"5060-5061"}}}}}}}}}`
	tables, err := ParseFirewallConfig(data)
	if err != nil {
		t.Fatalf("ParseFirewallConfig: %v", err)
	}
	term := tables[0].Chains[0].Terms[0]
	m, ok := term.Matches[0].(MatchDestinationPort)
	if !ok {
		t.Fatalf("match type = %T, want MatchDestinationPort", term.Matches[0])
	}
	if m.Port != 5060 || m.PortEnd != 5061 {
		t.Errorf("Port = %d-%d, want 5060-5061", m.Port, m.PortEnd)
	}
}

func TestParseNamedSet(t *testing.T) {
	data := `{"firewall":{"table":{"wan":{"family":"inet","set":{"blocked":{"type":"ipv4","flags-interval":""}}}}}}`
	tables, err := ParseFirewallConfig(data)
	if err != nil {
		t.Fatalf("ParseFirewallConfig: %v", err)
	}
	if len(tables[0].Sets) != 1 {
		t.Fatalf("got %d sets, want 1", len(tables[0].Sets))
	}
	s := tables[0].Sets[0]
	if s.Name != "blocked" {
		t.Errorf("Set.Name = %q, want %q", s.Name, "blocked")
	}
	if s.Type != SetTypeIPv4 {
		t.Errorf("Set.Type = %v, want SetTypeIPv4", s.Type)
	}
	if s.Flags&SetFlagInterval == 0 {
		t.Error("expected SetFlagInterval set")
	}
}

func TestParseFlowtable(t *testing.T) {
	data := `{"firewall":{"table":{"wan":{"family":"inet","flowtable":{"ft0":{"hook":"ingress","priority":"-100","device":["eth0","eth1"]}}}}}}`
	tables, err := ParseFirewallConfig(data)
	if err != nil {
		t.Fatalf("ParseFirewallConfig: %v", err)
	}
	if len(tables[0].Flowtables) != 1 {
		t.Fatalf("got %d flowtables, want 1", len(tables[0].Flowtables))
	}
	ft := tables[0].Flowtables[0]
	if ft.Name != "ft0" {
		t.Errorf("Name = %q, want %q", ft.Name, "ft0")
	}
	if ft.Hook != HookIngress {
		t.Errorf("Hook = %v, want ingress", ft.Hook)
	}
	if ft.Priority != -100 {
		t.Errorf("Priority = %d, want -100", ft.Priority)
	}
	if len(ft.Devices) != 2 {
		t.Errorf("Devices len = %d, want 2", len(ft.Devices))
	}
}
