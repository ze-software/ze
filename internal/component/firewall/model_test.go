package firewall

import (
	"net/netip"
	"strings"
	"testing"
)

func repeatByte(b byte, n int) string { return strings.Repeat(string(b), n) }

// Phase 1: Enums and base types.

func TestTableFamily(t *testing.T) {
	tests := []struct {
		name   string
		family TableFamily
		str    string
		valid  bool
	}{
		{"inet", FamilyInet, "inet", true},
		{"ip", FamilyIP, "ip", true},
		{"ip6", FamilyIP6, "ip6", true},
		{"arp", FamilyARP, "arp", true},
		{"bridge", FamilyBridge, "bridge", true},
		{"netdev", FamilyNetdev, "netdev", true},
		{"unknown zero", TableFamily(0), "unknown", false},
		{"invalid above", TableFamily(7), "unknown", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.family.String(); got != tt.str {
				t.Errorf("String() = %q, want %q", got, tt.str)
			}
			if got := tt.family.Valid(); got != tt.valid {
				t.Errorf("Valid() = %v, want %v", got, tt.valid)
			}
		})
	}
}

func TestParseTableFamily(t *testing.T) {
	tests := []struct {
		input string
		want  TableFamily
		ok    bool
	}{
		{"inet", FamilyInet, true},
		{"ip", FamilyIP, true},
		{"ip6", FamilyIP6, true},
		{"arp", FamilyARP, true},
		{"bridge", FamilyBridge, true},
		{"netdev", FamilyNetdev, true},
		{"invalid", TableFamily(0), false},
		{"", TableFamily(0), false},
	}
	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got, ok := ParseTableFamily(tt.input)
			if got != tt.want || ok != tt.ok {
				t.Errorf("ParseTableFamily(%q) = (%v, %v), want (%v, %v)", tt.input, got, ok, tt.want, tt.ok)
			}
		})
	}
}

func TestChainHook(t *testing.T) {
	tests := []struct {
		name  string
		hook  ChainHook
		str   string
		valid bool
	}{
		{"input", HookInput, "input", true},
		{"output", HookOutput, "output", true},
		{"forward", HookForward, "forward", true},
		{"prerouting", HookPrerouting, "prerouting", true},
		{"postrouting", HookPostrouting, "postrouting", true},
		{"ingress", HookIngress, "ingress", true},
		{"egress", HookEgress, "egress", true},
		{"unknown zero", ChainHook(0), "unknown", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.hook.String(); got != tt.str {
				t.Errorf("String() = %q, want %q", got, tt.str)
			}
			if got := tt.hook.Valid(); got != tt.valid {
				t.Errorf("Valid() = %v, want %v", got, tt.valid)
			}
		})
	}
}

func TestChainType(t *testing.T) {
	tests := []struct {
		name  string
		ct    ChainType
		str   string
		valid bool
	}{
		{"filter", ChainFilter, "filter", true},
		{"nat", ChainNAT, "nat", true},
		{"route", ChainRoute, "route", true},
		{"unknown zero", ChainType(0), "unknown", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.ct.String(); got != tt.str {
				t.Errorf("String() = %q, want %q", got, tt.str)
			}
			if got := tt.ct.Valid(); got != tt.valid {
				t.Errorf("Valid() = %v, want %v", got, tt.valid)
			}
		})
	}
}

func TestPolicy(t *testing.T) {
	tests := []struct {
		name  string
		p     Policy
		str   string
		valid bool
	}{
		{"accept", PolicyAccept, "accept", true},
		{"drop", PolicyDrop, "drop", true},
		{"unknown zero", Policy(0), "unknown", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.p.String(); got != tt.str {
				t.Errorf("String() = %q, want %q", got, tt.str)
			}
			if got := tt.p.Valid(); got != tt.valid {
				t.Errorf("Valid() = %v, want %v", got, tt.valid)
			}
		})
	}
}

// Phase 2: Expression interface and concrete types.

// VALIDATES: AC-3 "Every abstract match type has a concrete Match implementation (18 types)".
// PREVENTS: missing Match implementation for any abstract type.
func TestMatchTypes(t *testing.T) {
	prefix := netip.MustParsePrefix("10.0.0.0/24")
	matches := []struct {
		name  string
		match Match
	}{
		{"MatchSourceAddress", MatchSourceAddress{Prefix: prefix}},
		{"MatchDestinationAddress", MatchDestinationAddress{Prefix: prefix}},
		{"MatchSourcePort", MatchSourcePort{Port: 80}},
		{"MatchDestinationPort", MatchDestinationPort{Port: 443, PortEnd: 445}},
		{"MatchProtocol", MatchProtocol{Protocol: "tcp"}},
		{"MatchInputInterface", MatchInputInterface{Name: "eth0"}},
		{"MatchOutputInterface", MatchOutputInterface{Name: "eth1"}},
		{"MatchConnState", MatchConnState{States: ConnStateEstablished | ConnStateRelated}},
		{"MatchConnMark", MatchConnMark{Value: 0x10, Mask: 0xFF}},
		{"MatchMark", MatchMark{Value: 0x20, Mask: 0xFF}},
		{"MatchDSCP", MatchDSCP{Value: 46}},
		{"MatchConnBytes", MatchConnBytes{Bytes: 1024, Over: true}},
		{"MatchConnLimit", MatchConnLimit{Count: 100}},
		{"MatchFib", MatchFib{Result: FibResultOIF}},
		{"MatchSocket", MatchSocket{Key: SocketTransparent}},
		{"MatchRt", MatchRt{Key: RtClassID}},
		{"MatchExtHdr", MatchExtHdr{Type: 0}},
		{"MatchInSet", MatchInSet{SetName: "blocked", MatchField: SetFieldSourceAddr}},
	}
	if len(matches) != 18 {
		t.Fatalf("expected 18 match types, got %d", len(matches))
	}
	for _, tt := range matches {
		t.Run(tt.name, func(t *testing.T) {
			tt.match.matchMarker()
		})
	}
}

// VALIDATES: AC-4 "Every abstract action/modifier type has a concrete Action implementation (24 types)".
// PREVENTS: missing Action implementation for any abstract type.
func TestActionTypes(t *testing.T) {
	actions := []struct {
		name   string
		action Action
	}{
		// 16 action types
		{"Accept", Accept{}},
		{"Drop", Drop{}},
		{"Reject", Reject{Type: "icmp", Code: 3}},
		{"Jump", Jump{Target: "other-chain"}},
		{"Goto", Goto{Target: "other-chain"}},
		{"Return", Return{}},
		{"SNAT", SNAT{Address: netip.MustParseAddr("1.2.3.4"), Port: 1024}},
		{"DNAT", DNAT{Address: netip.MustParseAddr("10.0.0.1"), Port: 80}},
		{"Masquerade", Masquerade{}},
		{"Redirect", Redirect{Port: 8080}},
		{"Queue", Queue{Num: 0, Total: 4}},
		{"Notrack", Notrack{}},
		{"TProxy", TProxy{Address: netip.MustParseAddr("127.0.0.1"), Port: 9090}},
		{"Duplicate", Duplicate{Address: netip.MustParseAddr("10.0.0.2"), Device: "eth1"}},
		{"FlowOffload", FlowOffload{FlowtableName: "ft0"}},
		{"Synproxy", Synproxy{MSS: 1460, Wscale: 7}},
		// 8 modifier types
		{"SetMark", SetMark{Value: 0x10, Mask: 0xFFFFFFFF}},
		{"SetConnMark", SetConnMark{Value: 0x20, Mask: 0xFF}},
		{"SetDSCP", SetDSCP{Value: 46}},
		{"Counter", Counter{Name: "my-counter"}},
		{"Log", Log{Prefix: "INPUT-DROP: ", Level: 4}},
		{"Limit", Limit{Rate: 10, Unit: "second", Burst: 5}},
		{"Quota", Quota{Bytes: 1000000}},
		{"SecMark", SecMark{Name: "http_t"}},
	}
	if len(actions) != 24 {
		t.Fatalf("expected 24 action types, got %d", len(actions))
	}
	for _, tt := range actions {
		t.Run(tt.name, func(t *testing.T) {
			tt.action.actionMarker()
		})
	}
}

// Phase 3: Table/Chain/Term/Set/Flowtable structs.

// VALIDATES: AC-1 "Table, Chain, Set, Flowtable, Term structs hold all firewall concepts".
// PREVENTS: missing fields or wrong struct composition.
func TestTableConstruction(t *testing.T) {
	tbl := Table{
		Name:   "wan",
		Family: FamilyInet,
		Chains: []Chain{
			{
				Name:     "input",
				Type:     ChainFilter,
				Hook:     HookInput,
				Priority: 0,
				Policy:   PolicyDrop,
				IsBase:   true,
				Terms: []Term{
					{
						Name:    "allow-ssh",
						Matches: []Match{MatchDestinationPort{Port: 22}},
						Actions: []Action{Accept{}},
					},
				},
			},
		},
		Sets: []Set{
			{
				Name:     "blocked",
				Type:     SetTypeIPv4,
				Flags:    SetFlagInterval,
				Elements: []SetElement{{Value: "10.0.0.0/24"}},
			},
		},
		Flowtables: []Flowtable{
			{
				Name:     "ft0",
				Hook:     HookIngress,
				Priority: -100,
				Devices:  []string{"eth0", "eth1"},
			},
		},
	}
	if tbl.Name != "wan" {
		t.Errorf("Table.Name = %q, want %q", tbl.Name, "wan")
	}
	if tbl.Family != FamilyInet {
		t.Errorf("Table.Family = %v, want %v", tbl.Family, FamilyInet)
	}
	if len(tbl.Chains) != 1 {
		t.Fatalf("Table.Chains len = %d, want 1", len(tbl.Chains))
	}
	if len(tbl.Sets) != 1 {
		t.Fatalf("Table.Sets len = %d, want 1", len(tbl.Sets))
	}
	if len(tbl.Flowtables) != 1 {
		t.Fatalf("Table.Flowtables len = %d, want 1", len(tbl.Flowtables))
	}
}

// VALIDATES: AC-2 "Term has Name, Matches []Match, Actions []Action".
// PREVENTS: missing Match/Action in Term.
func TestTermConstruction(t *testing.T) {
	term := Term{
		Name: "allow-established",
		Matches: []Match{
			MatchConnState{States: ConnStateEstablished | ConnStateRelated},
		},
		Actions: []Action{
			Counter{Name: "allow-established"},
			Accept{},
		},
	}
	if term.Name != "allow-established" {
		t.Errorf("Term.Name = %q, want %q", term.Name, "allow-established")
	}
	if len(term.Matches) != 1 {
		t.Errorf("Term.Matches len = %d, want 1", len(term.Matches))
	}
	if len(term.Actions) != 2 {
		t.Errorf("Term.Actions len = %d, want 2", len(term.Actions))
	}
}

// VALIDATES: AC-12 "Term name validation: Names must be non-empty, valid identifiers".
// PREVENTS: empty or invalid term names accepted.
func TestTermNameValidation(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		wantErr bool
	}{
		{"valid", "allow-ssh", false},
		{"valid underscore", "allow_ssh", false},
		{"valid alphanumeric", "rule1", false},
		{"at kernel limit", repeatByte('a', 255), false},
		{"over kernel limit", repeatByte('a', 256), true},
		{"empty", "", true},
		{"space", "has space", true},
		{"slash", "has/slash", true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateName(tt.input)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateName(%q) error = %v, wantErr %v", tt.input, err, tt.wantErr)
			}
		})
	}
}

// VALIDATES: AC-8 "Table name validation: Names must be non-empty, valid identifiers".
// PREVENTS: invalid table names accepted.
func TestTableValidation(t *testing.T) {
	tests := []struct {
		name    string
		table   Table
		wantErr bool
	}{
		{
			"valid",
			Table{Name: "wan", Family: FamilyInet},
			false,
		},
		{
			"empty name",
			Table{Name: "", Family: FamilyInet},
			true,
		},
		{
			"invalid family",
			Table{Name: "wan", Family: TableFamily(0)},
			true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.table.Validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("Table.Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

// VALIDATES: AC-9 "Base chain has type, hook, priority, policy; regular chain has none".
// PREVENTS: base chain without hook or regular chain with hook.
func TestChainHookValidation(t *testing.T) {
	tests := []struct {
		name    string
		chain   Chain
		wantErr bool
	}{
		{
			"valid base chain",
			Chain{
				Name:     "input",
				IsBase:   true,
				Type:     ChainFilter,
				Hook:     HookInput,
				Priority: 0,
				Policy:   PolicyDrop,
			},
			false,
		},
		{
			"valid regular chain",
			Chain{
				Name:   "helper",
				IsBase: false,
			},
			false,
		},
		{
			"base chain missing type",
			Chain{
				Name:   "input",
				IsBase: true,
				Hook:   HookInput,
				Policy: PolicyDrop,
			},
			true,
		},
		{
			"base chain missing hook",
			Chain{
				Name:   "input",
				IsBase: true,
				Type:   ChainFilter,
				Policy: PolicyDrop,
			},
			true,
		},
		{
			"base chain missing policy",
			Chain{
				Name:   "input",
				IsBase: true,
				Type:   ChainFilter,
				Hook:   HookInput,
			},
			true,
		},
		{
			"empty chain name",
			Chain{Name: ""},
			true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.chain.Validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("Chain.Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

// VALIDATES: AC-10 "Named set with type, flags, optional elements".
// PREVENTS: set construction missing fields.
func TestSetConstruction(t *testing.T) {
	s := Set{
		Name:  "blocked-hosts",
		Type:  SetTypeIPv4,
		Flags: SetFlagInterval,
		Elements: []SetElement{
			{Value: "10.0.0.0/24"},
			{Value: "192.168.1.0/24"},
		},
	}
	if s.Name != "blocked-hosts" {
		t.Errorf("Set.Name = %q, want %q", s.Name, "blocked-hosts")
	}
	if s.Type != SetTypeIPv4 {
		t.Errorf("Set.Type = %v, want %v", s.Type, SetTypeIPv4)
	}
	if len(s.Elements) != 2 {
		t.Errorf("Set.Elements len = %d, want 2", len(s.Elements))
	}
}

// VALIDATES: AC-11 "Flowtable with hook, priority, devices list".
// PREVENTS: flowtable missing required fields.
func TestFlowtableConstruction(t *testing.T) {
	ft := Flowtable{
		Name:     "ft0",
		Hook:     HookIngress,
		Priority: -100,
		Devices:  []string{"eth0", "eth1"},
	}
	if ft.Name != "ft0" {
		t.Errorf("Flowtable.Name = %q, want %q", ft.Name, "ft0")
	}
	if ft.Hook != HookIngress {
		t.Errorf("Flowtable.Hook = %v, want %v", ft.Hook, HookIngress)
	}
	if ft.Priority != -100 {
		t.Errorf("Flowtable.Priority = %d, want %d", ft.Priority, -100)
	}
	if len(ft.Devices) != 2 {
		t.Errorf("Flowtable.Devices len = %d, want 2", len(ft.Devices))
	}
}

// Boundary tests.

func TestTableFamilyBoundary(t *testing.T) {
	// Last valid: FamilyNetdev (6)
	if !FamilyNetdev.Valid() {
		t.Error("FamilyNetdev should be valid")
	}
	// First invalid above: 7
	if TableFamily(7).Valid() {
		t.Error("TableFamily(7) should be invalid")
	}
}

func TestPortBoundary(t *testing.T) {
	tests := []struct {
		name    string
		port    uint16
		wantErr bool
	}{
		{"valid min", 1, false},
		{"valid max", 65535, false},
		{"invalid zero", 0, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidatePort(tt.port)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidatePort(%d) error = %v, wantErr %v", tt.port, err, tt.wantErr)
			}
		})
	}
}

func TestRateBoundary(t *testing.T) {
	tests := []struct {
		name    string
		rate    uint64
		wantErr bool
	}{
		{"valid min", 1, false},
		{"invalid zero", 0, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateRate(tt.rate)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateRate(%d) error = %v, wantErr %v", tt.rate, err, tt.wantErr)
			}
		})
	}
}

func TestMarkBoundary(t *testing.T) {
	// Mark value is uint32; max value 0xFFFFFFFF is inherently valid.
	m := MatchMark{Value: 0xFFFFFFFF, Mask: 0xFFFFFFFF}
	m.matchMarker()
}
