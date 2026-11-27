package firewall

import (
	"errors"
	"net/netip"
	"strings"
	"testing"

	"github.com/ze-software/ze/internal/core/opererr"
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
			got, ok := parseTableFamily(tt.input)
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

// VALIDATES: every reachable Action type (parsed by config, lowered by the kernel
// backend, or both) implements the Action interface via actionMarker.
// PREVENTS: a new Action type added to the data model without the marker, which
// would compile but silently fail the a.(Action) assertion.
func TestActionTypes(t *testing.T) {
	actions := []struct {
		name   string
		action Action
	}{
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
		{"Notrack", Notrack{}},
		{"FlowOffload", FlowOffload{FlowtableName: "ft0"}},
		{"SetMark", SetMark{Value: 0x10, Mask: 0xFFFFFFFF}},
		{"SetConnMark", SetConnMark{Value: 0x20, Mask: 0xFF}},
		{"SetDSCP", SetDSCP{Value: 46}},
		{"SetTCPMSS", SetTCPMSS{Size: 1400}},
		{"Counter", Counter{Name: "my-counter"}},
		{"Log", Log{Prefix: "INPUT-DROP: ", Level: new(uint32(4))}},
		{"Limit", Limit{Rate: 10, Unit: "second", Burst: 5, Dimension: RateDimensionPackets}},
	}
	if len(actions) != 19 {
		t.Fatalf("expected 19 action types, got %d", len(actions))
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
						Matches: []Match{MatchDestinationPort{Ranges: []PortRange{{Lo: 22, Hi: 22}}}},
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

// VALIDATES: spec-firewall-dynamic-set-config-fidelity AC-2, AC-3 -- Set.Validate
// refuses contradictory set flags, element timeouts without flags-timeout,
// IntervalEnd markers without flags-interval, CIDR prefixes without
// flags-interval, non-canonical CIDR prefixes with host bits set, IP family
// mismatches, and overlapping intervals in unexpanded flags-interval sets,
// returning an *opererr.Error with problem, why, and fix while accepting valid
// static/timeout/dynamic/constant/interval/adjacent/top-of-space/RawIntervals sets.
// PREVENTS: invalid set flags or CIDR elements passing verify and silently
// dropping timeouts/prefix-lengths or failing only at kernel netlink flush.
func TestSetValidateFlagsAndElements(t *testing.T) {
	const (
		setBlocked   = "blocked"
		setBlocked6  = "blocked6"
		setNets      = "nets"
		setNets6     = "nets6"
		setBadFlags  = "bad-flags"
		setCoppV4    = "bgp-neighbor-ipv4"
		flagInterval = "flags-interval"
		flagTimeout  = "flags-timeout"
		flagConstant = "flags-constant"
		cidrV4       = "192.0.2.0/24"
		cidrV6       = "2001:db8::/32"
		addrV4       = "192.0.2.1"
		addrV6       = "2001:db8::1"
		addrMappedV6 = "::ffff:192.0.2.1"
		hostA        = "10.0.0.1"
		hostB        = "10.0.0.2"
		addrZeroV4   = "0.0.0.0"
		defaultV4    = "0.0.0.0/0"
		addrEndV4    = "10.0.1.0"
		topV4        = "255.255.255.0/24"
		overlapBase  = "10.0.0.0/24"
	)

	validCases := []struct {
		name string
		set  Set
	}{
		{
			name: "static ipv4 hosts",
			set: Set{
				Name:     setBlocked,
				Type:     SetTypeIPv4,
				Elements: []SetElement{{Value: hostA}, {Value: hostB}},
			},
		},
		{
			name: "static ipv6 host",
			set: Set{
				Name:     setBlocked,
				Type:     SetTypeIPv6,
				Elements: []SetElement{{Value: addrV6}},
			},
		},
		{
			name: "flags-timeout with element timeout and zero timeout",
			set: Set{
				Name:  "transient",
				Type:  SetTypeIPv4,
				Flags: SetFlagTimeout,
				Elements: []SetElement{
					{Value: hostA, Timeout: 3600},
					{Value: hostB, Timeout: 0},
				},
			},
		},
		{
			name: "flags-dynamic with flags-timeout",
			set: Set{
				Name:     "dyn",
				Type:     SetTypeIPv4,
				Flags:    SetFlagDynamic | SetFlagTimeout,
				Elements: []SetElement{{Value: hostA, Timeout: 60}},
			},
		},
		{
			name: "standalone flags-constant",
			set: Set{
				Name:     "frozen",
				Type:     SetTypeIPv4,
				Flags:    SetFlagConstant,
				Elements: []SetElement{{Value: addrV4}},
			},
		},
		{
			name: "flags-constant with flags-interval and canonical cidrs",
			set: Set{
				Name:  setNets,
				Type:  SetTypeIPv4,
				Flags: SetFlagConstant | SetFlagInterval,
				Elements: []SetElement{
					{Value: cidrV4},
					{Value: "198.51.100.1"},
				},
			},
		},
		{
			name: "flags-interval with adjacent non-overlapping ipv4 prefixes",
			set: Set{
				Name:  setNets,
				Type:  SetTypeIPv4,
				Flags: SetFlagInterval,
				Elements: []SetElement{
					{Value: "192.0.2.0/25"},
					{Value: "192.0.2.128/25"},
				},
			},
		},
		{
			name: "flags-interval with adjacent non-overlapping ipv6 prefixes",
			set: Set{
				Name:  setNets6,
				Type:  SetTypeIPv6,
				Flags: SetFlagInterval,
				Elements: []SetElement{
					{Value: "2001:db8::/33"},
					{Value: "2001:db8:8000::/33"},
				},
			},
		},
		{
			name: "flags-interval with top-of-space ipv4 prefix",
			set: Set{
				Name:     setNets,
				Type:     SetTypeIPv4,
				Flags:    SetFlagInterval,
				Elements: []SetElement{{Value: topV4}},
			},
		},
		{
			name: "flags-interval with default route ipv4 prefix",
			set: Set{
				Name:     setNets,
				Type:     SetTypeIPv4,
				Flags:    SetFlagInterval,
				Elements: []SetElement{{Value: defaultV4}},
			},
		},
		{
			name: "flags-interval with default route ipv6 prefix",
			set: Set{
				Name:     setNets6,
				Type:     SetTypeIPv6,
				Flags:    SetFlagInterval,
				Elements: []SetElement{{Value: "::/0"}},
			},
		},
		{
			name: "flags-interval with adjacent bare host addresses",
			set: Set{
				Name:  setNets,
				Type:  SetTypeIPv4,
				Flags: SetFlagInterval,
				Elements: []SetElement{
					{Value: hostA},
					{Value: hostB},
				},
			},
		},
		{
			name: "flags-interval with adjacent top-of-space ipv4 prefixes",
			set: Set{
				Name:  setNets,
				Type:  SetTypeIPv4,
				Flags: SetFlagInterval,
				Elements: []SetElement{
					{Value: "255.255.254.0/24"},
					{Value: topV4},
				},
			},
		},
		{
			name: "empty flags-dynamic set",
			set: Set{
				Name:  "dyn-empty",
				Type:  SetTypeIPv4,
				Flags: SetFlagDynamic,
			},
		},
		{
			name: "flags-interval with top-of-space ipv6 prefix",
			set: Set{
				Name:     setNets6,
				Type:     SetTypeIPv6,
				Flags:    SetFlagInterval,
				Elements: []SetElement{{Value: "ffff::/16"}},
			},
		},
		{
			name: "flags-interval with flags-timeout and cidr timeout",
			set: Set{
				Name:     setNets,
				Type:     SetTypeIPv4,
				Flags:    SetFlagInterval | SetFlagTimeout,
				Elements: []SetElement{{Value: cidrV4, Timeout: 300}},
			},
		},
		{
			name: "pre-expanded irr interval pair",
			set: Set{
				Name:  "irr_v4_AS13335",
				Type:  SetTypeIPv4,
				Flags: SetFlagInterval,
				Elements: []SetElement{
					{Value: "10.0.0.0"},
					{Value: addrEndV4, IntervalEnd: true},
				},
			},
		},
		{
			name: "raw intervals lone top-of-space copp element",
			set: Set{
				Name:         setCoppV4,
				Type:         SetTypeIPv4,
				Flags:        SetFlagInterval,
				RawIntervals: true,
				Elements:     []SetElement{{Value: addrZeroV4}},
			},
		},
		{
			name: "inet-service port set",
			set: Set{
				Name:     "ports",
				Type:     SetTypeInetService,
				Elements: []SetElement{{Value: "22"}, {Value: "443"}},
			},
		},
	}

	for _, tc := range validCases {
		t.Run("valid/"+tc.name, func(t *testing.T) {
			if err := tc.set.Validate(); err != nil {
				t.Errorf("Set.Validate() unexpected error: %v", err)
			}
		})
	}

	invalidCases := []struct {
		name     string
		set      Set
		wantText []string
	}{
		{
			name: "flags-constant with flags-dynamic",
			set: Set{
				Name:  setBadFlags,
				Type:  SetTypeIPv4,
				Flags: SetFlagConstant | SetFlagDynamic,
			},
			wantText: []string{setBadFlags, flagConstant, "flags-dynamic"},
		},
		{
			name: "flags-constant with flags-timeout",
			set: Set{
				Name:  setBadFlags,
				Type:  SetTypeIPv4,
				Flags: SetFlagConstant | SetFlagTimeout,
			},
			wantText: []string{setBadFlags, flagConstant, flagTimeout},
		},
		{
			name: "element timeout without flags-timeout",
			set: Set{
				Name:     setBlocked,
				Type:     SetTypeIPv4,
				Elements: []SetElement{{Value: hostA, Timeout: 3600}},
			},
			wantText: []string{setBlocked, hostA, flagTimeout},
		},
		{
			name: "interval end without flags-interval",
			set: Set{
				Name:     setNets,
				Type:     SetTypeIPv4,
				Elements: []SetElement{{Value: addrEndV4, IntervalEnd: true}},
			},
			wantText: []string{setNets, flagInterval},
		},
		{
			name: "ipv4 cidr without flags-interval",
			set: Set{
				Name:     setNets,
				Type:     SetTypeIPv4,
				Elements: []SetElement{{Value: cidrV4}},
			},
			wantText: []string{setNets, cidrV4, flagInterval},
		},
		{
			name: "ipv6 cidr without flags-interval",
			set: Set{
				Name:     setNets6,
				Type:     SetTypeIPv6,
				Elements: []SetElement{{Value: cidrV6}},
			},
			wantText: []string{setNets6, cidrV6, flagInterval},
		},
		{
			name: "non-canonical ipv4 cidr with host bits",
			set: Set{
				Name:     setNets,
				Type:     SetTypeIPv4,
				Flags:    SetFlagInterval,
				Elements: []SetElement{{Value: "192.0.2.5/24"}},
			},
			wantText: []string{setNets, "192.0.2.5/24", cidrV4},
		},
		{
			name: "non-canonical ipv6 cidr with host bits",
			set: Set{
				Name:     setNets6,
				Type:     SetTypeIPv6,
				Flags:    SetFlagInterval,
				Elements: []SetElement{{Value: "2001:db8::1/32"}},
			},
			wantText: []string{setNets6, "2001:db8::1/32", cidrV6},
		},
		{
			name: "ipv6 address in ipv4 set",
			set: Set{
				Name:     setBlocked,
				Type:     SetTypeIPv4,
				Elements: []SetElement{{Value: addrV6}},
			},
			wantText: []string{setBlocked, addrV6},
		},
		{
			name: "ipv4 address in ipv6 set",
			set: Set{
				Name:     setBlocked6,
				Type:     SetTypeIPv6,
				Elements: []SetElement{{Value: addrV4}},
			},
			wantText: []string{setBlocked6, addrV4},
		},
		{
			name: "ipv6 cidr in ipv4 interval set",
			set: Set{
				Name:     setNets,
				Type:     SetTypeIPv4,
				Flags:    SetFlagInterval,
				Elements: []SetElement{{Value: cidrV6}},
			},
			wantText: []string{setNets, cidrV6},
		},
		{
			name: "ipv4 cidr in ipv6 interval set",
			set: Set{
				Name:     setNets6,
				Type:     SetTypeIPv6,
				Flags:    SetFlagInterval,
				Elements: []SetElement{{Value: cidrV4}},
			},
			wantText: []string{setNets6, cidrV4},
		},
		{
			name: "overlapping ipv4 prefixes",
			set: Set{
				Name:  setNets,
				Type:  SetTypeIPv4,
				Flags: SetFlagInterval,
				Elements: []SetElement{
					{Value: overlapBase},
					{Value: "10.0.0.128/25"},
				},
			},
			wantText: []string{setNets, overlapBase, "10.0.0.128/25"},
		},
		{
			name: "overlapping ipv4 prefix and bare host",
			set: Set{
				Name:  setNets,
				Type:  SetTypeIPv4,
				Flags: SetFlagInterval,
				Elements: []SetElement{
					{Value: overlapBase},
					{Value: "10.0.0.5"},
				},
			},
			wantText: []string{setNets, overlapBase, "10.0.0.5"},
		},
		{
			name: "overlapping ipv6 prefixes",
			set: Set{
				Name:  setNets6,
				Type:  SetTypeIPv6,
				Flags: SetFlagInterval,
				Elements: []SetElement{
					{Value: cidrV6},
					{Value: "2001:db8:1::/48"},
				},
			},
			wantText: []string{setNets6, cidrV6, "2001:db8:1::/48"},
		},
		{
			name: "invalid ipv4 prefix length above 32",
			set: Set{
				Name:     setNets,
				Type:     SetTypeIPv4,
				Flags:    SetFlagInterval,
				Elements: []SetElement{{Value: "192.0.2.0/33"}},
			},
			wantText: []string{setNets, "192.0.2.0/33"},
		},
		{
			name: "invalid ipv6 prefix length above 128",
			set: Set{
				Name:     setNets6,
				Type:     SetTypeIPv6,
				Flags:    SetFlagInterval,
				Elements: []SetElement{{Value: "2001:db8::/129"}},
			},
			wantText: []string{setNets6, "2001:db8::/129"},
		},
		{
			name: "unparseable ip element",
			set: Set{
				Name:     setBlocked,
				Type:     SetTypeIPv4,
				Elements: []SetElement{{Value: "not-an-ip"}},
			},
			wantText: []string{setBlocked, "not-an-ip"},
		},
		{
			name: "ipv4-in-ipv6 address in ipv4 set",
			set: Set{
				Name:     setBlocked,
				Type:     SetTypeIPv4,
				Elements: []SetElement{{Value: addrMappedV6}},
			},
			wantText: []string{setBlocked, addrMappedV6},
		},
		{
			name: "ipv4-in-ipv6 address in ipv6 set",
			set: Set{
				Name:     setBlocked6,
				Type:     SetTypeIPv6,
				Elements: []SetElement{{Value: addrMappedV6}},
			},
			wantText: []string{setBlocked6, addrMappedV6},
		},
		{
			name: "ipv6 zone identifier rejected",
			set: Set{
				Name:     setBlocked6,
				Type:     SetTypeIPv6,
				Elements: []SetElement{{Value: "fe80::1%eth0"}},
			},
			wantText: []string{setBlocked6, "fe80::1%eth0"},
		},
		{
			name: "raw intervals family mismatch",
			set: Set{
				Name:         setCoppV4,
				Type:         SetTypeIPv4,
				Flags:        SetFlagInterval,
				RawIntervals: true,
				Elements:     []SetElement{{Value: addrV6}},
			},
			wantText: []string{setCoppV4, addrV6},
		},
		{
			name: "pre-expanded interval end family mismatch",
			set: Set{
				Name:  "irr_v6_AS13335",
				Type:  SetTypeIPv6,
				Flags: SetFlagInterval,
				Elements: []SetElement{
					{Value: "2001:db8::"},
					{Value: addrEndV4, IntervalEnd: true},
				},
			},
			wantText: []string{"irr_v6_AS13335", addrEndV4},
		},
		{
			name: "raw intervals timeout without flags-timeout",
			set: Set{
				Name:         setCoppV4,
				Type:         SetTypeIPv4,
				Flags:        SetFlagInterval,
				RawIntervals: true,
				Elements:     []SetElement{{Value: addrZeroV4, Timeout: 60}},
			},
			wantText: []string{setCoppV4, addrZeroV4, flagTimeout},
		},
		{
			name: "overlapping default route and narrower prefix",
			set: Set{
				Name:  setNets,
				Type:  SetTypeIPv4,
				Flags: SetFlagInterval,
				Elements: []SetElement{
					{Value: defaultV4},
					{Value: "10.0.0.0/8"},
				},
			},
			wantText: []string{setNets, defaultV4, "10.0.0.0/8"},
		},
		{
			name: "duplicate bare host and /32 in interval set",
			set: Set{
				Name:  setNets,
				Type:  SetTypeIPv4,
				Flags: SetFlagInterval,
				Elements: []SetElement{
					{Value: hostA},
					{Value: "10.0.0.1/32"},
				},
			},
			wantText: []string{setNets, hostA, "10.0.0.1/32"},
		},
	}

	for _, tc := range invalidCases {
		t.Run("invalid/"+tc.name, func(t *testing.T) {
			err := tc.set.Validate()
			if err == nil {
				t.Fatalf("Set.Validate() returned nil, want opererr refusal")
			}
			opErr, ok := errors.AsType[*opererr.Error](err)
			if !ok {
				t.Fatalf("Set.Validate() error type = %T, want *opererr.Error: %v", err, err)
			}
			if opErr.Problem() == "" {
				t.Errorf("opererr missing problem: %q", opErr.Error())
			}
			if opErr.Why() == "" {
				t.Errorf("opererr missing why: %q", opErr.Error())
			}
			if opErr.Fix() == "" {
				t.Errorf("opererr missing fix: %q", opErr.Error())
			}
			full := err.Error()
			for _, want := range tc.wantText {
				if !strings.Contains(full, want) {
					t.Errorf("Set.Validate() error %q does not contain %q", full, want)
				}
			}
		})
	}
}
