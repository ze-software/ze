package irr

// VALIDATES: AC-4 interval sets created with correct prefixes
// VALIDATES: set naming convention irr_v4_/irr_v6_ prefix
// PREVENTS: mismatched set names between config parser and plugin

import (
	"net/netip"
	"testing"

	"github.com/ze-software/ze/internal/component/firewall"
	"github.com/ze-software/ze/internal/component/resolve/irr/store"
)

func TestSetNaming(t *testing.T) {
	tests := []struct {
		name    string
		isASSet bool
		wantV4  string
		wantV6  string
	}{
		{"AS13335", false, "irr_v4_AS13335", "irr_v6_AS13335"},
		{"AS-CLOUDFLARE", true, "irr_v4_AS-CLOUDFLARE", "irr_v6_AS-CLOUDFLARE"},
	}
	for _, tt := range tests {
		v4, v6 := setNames(tt.name)
		if v4 != tt.wantV4 {
			t.Errorf("setNames(%q) v4 = %q, want %q", tt.name, v4, tt.wantV4)
		}
		if v6 != tt.wantV6 {
			t.Errorf("setNames(%q) v6 = %q, want %q", tt.name, v6, tt.wantV6)
		}
	}
}

func TestBuildIntervalSets(t *testing.T) {
	v4 := []netip.Prefix{
		netip.MustParsePrefix("192.168.0.0/24"),
		netip.MustParsePrefix("10.0.0.0/8"),
	}
	v6 := []netip.Prefix{
		netip.MustParsePrefix("2001:db8::/32"),
	}

	sets := buildSets("AS13335", v4, v6)
	if len(sets) != 2 {
		t.Fatalf("expected 2 sets (v4+v6), got %d", len(sets))
	}

	v4Set := sets[0]
	if v4Set.Name != "irr_v4_AS13335" {
		t.Errorf("v4 set name = %q, want irr_v4_AS13335", v4Set.Name)
	}
	if v4Set.Type != firewall.SetTypeIPv4 {
		t.Errorf("v4 set type = %v, want SetTypeIPv4", v4Set.Type)
	}
	if v4Set.Flags&firewall.SetFlagInterval == 0 {
		t.Error("v4 set must have SetFlagInterval")
	}
	// Each prefix becomes start + end elements
	if len(v4Set.Elements) != 4 {
		t.Errorf("v4 set elements = %d, want 4 (2 prefixes x 2 elements each)", len(v4Set.Elements))
	}

	v6Set := sets[1]
	if v6Set.Name != "irr_v6_AS13335" {
		t.Errorf("v6 set name = %q, want irr_v6_AS13335", v6Set.Name)
	}
	if v6Set.Type != firewall.SetTypeIPv6 {
		t.Errorf("v6 set type = %v, want SetTypeIPv6", v6Set.Type)
	}
	if len(v6Set.Elements) != 2 {
		t.Errorf("v6 set elements = %d, want 2 (1 prefix x 2 elements)", len(v6Set.Elements))
	}
}

func TestBuildSetsEmptyV6(t *testing.T) {
	v4 := []netip.Prefix{netip.MustParsePrefix("192.168.0.0/24")}
	sets := buildSets("AS13335", v4, nil)
	if len(sets) != 1 {
		t.Fatalf("expected 1 set (v4 only), got %d", len(sets))
	}
	if sets[0].Name != "irr_v4_AS13335" {
		t.Errorf("set name = %q, want irr_v4_AS13335", sets[0].Name)
	}
}

// VALIDATES: AC-3 -- an entry with no prefixes produces no set AND no table, so
// nothing empty reaches the kernel through the table-term consumer.
// PREVENTS: buildSets emitting an empty set for a family that answered nothing.
// An empty nftables set matches no packet, so an accept term naming one accepts
// nothing. The set would also RESOLVE, so dropTablesMissingAProvidedSet
// (internal/component/firewall/registry.go) would stop holding the operator's
// table back and would program a term that filters everything out. The zero-set
// answer is what keeps that table out of the kernel until prefixes arrive.
func TestBuildSetsEmptyBoth(t *testing.T) {
	sets := buildSets("AS13335", nil, nil)
	if len(sets) != 0 {
		t.Errorf("expected 0 sets for empty prefix lists, got %d", len(sets))
	}

	// The consumer, not just the builder: a term naming an entry with no
	// prefixes must yield no table at all.
	ps := store.New(nil, nil, "")
	ps.Put("AS13335", nil, nil)
	tables := buildIRRTables(ps, []irrRef{{Name: "AS13335", TableName: "ze_wan"}})
	if len(tables) != 0 {
		t.Fatalf("an entry with no prefixes produced %d table(s): %+v", len(tables), tables)
	}
}

// VALIDATES: /0 prefixes skipped to avoid overflow producing empty intervals.
// PREVENTS: uint32 overflow wrapping exclusive end to 0.0.0.0 for /0.
func TestPrefixRangeSkipsSlashZero(t *testing.T) {
	v4 := []netip.Prefix{
		netip.MustParsePrefix("0.0.0.0/0"),
		netip.MustParsePrefix("10.0.0.0/8"),
	}
	elements := prefixesToIntervalElements(v4, 100)
	if len(elements) != 2 {
		t.Fatalf("expected 2 elements (1 prefix, /0 skipped), got %d", len(elements))
	}
	if elements[0].Value != "10.0.0.0" {
		t.Errorf("first element = %q, want 10.0.0.0", elements[0].Value)
	}
}

func TestPrefixRangeHostRoute(t *testing.T) {
	start, end := prefixRange(netip.MustParsePrefix("1.2.3.4/32"))
	if start.String() != "1.2.3.4" {
		t.Errorf("start = %s, want 1.2.3.4", start)
	}
	if end.String() != "1.2.3.5" {
		t.Errorf("end = %s, want 1.2.3.5", end)
	}
}

func TestPrefixRangeIPv6(t *testing.T) {
	start, end := prefixRange(netip.MustParsePrefix("2001:db8::/32"))
	if start.String() != "2001:db8::" {
		t.Errorf("start = %s, want 2001:db8::", start)
	}
	if end.String() != "2001:db9::" {
		t.Errorf("end = %s, want 2001:db9::", end)
	}
}

// VALIDATES: AC-1 per-interface table with ingress chain and sets.
// PREVENTS: missing chain or sets in per-interface filter table.
func TestBuildIfaceTables(t *testing.T) {
	ps := store.New(nil, nil, "")
	ps.Put("AS-FOO", []netip.Prefix{netip.MustParsePrefix("10.0.0.0/8")},
		[]netip.Prefix{netip.MustParsePrefix("2001:db8::/32")})

	bindings := []ifaceBinding{{Interface: "eth1", ASSet: "AS-FOO"}}
	tables := buildIfaceTables(ps, bindings)

	if len(tables) != 1 {
		t.Fatalf("expected 1 table, got %d", len(tables))
	}
	tbl := tables[0]
	if tbl.Name != ifaceTableName {
		t.Errorf("table name = %q, want %q", tbl.Name, ifaceTableName)
	}
	if tbl.Family != firewall.FamilyInet {
		t.Errorf("table family = %v, want inet", tbl.Family)
	}
	if len(tbl.Sets) != 2 {
		t.Errorf("expected 2 sets (v4+v6), got %d", len(tbl.Sets))
	}
	if len(tbl.Chains) != 1 {
		t.Fatalf("expected 1 chain, got %d", len(tbl.Chains))
	}
	chain := tbl.Chains[0]
	if !chain.IsBase {
		t.Error("chain must be a base chain")
	}
	if chain.Hook != firewall.HookPrerouting {
		t.Errorf("chain hook = %v, want prerouting", chain.Hook)
	}
	if chain.Policy != firewall.PolicyAccept {
		t.Errorf("chain policy = %v, want accept", chain.Policy)
	}
	if len(chain.Terms) != 3 {
		t.Fatalf("expected 3 terms (v4 accept, v6 accept, drop), got %d", len(chain.Terms))
	}
}

// VALIDATES: AC-3 multiple interfaces get independent terms.
// PREVENTS: second interface binding overwriting the first.
func TestBuildIfaceTablesMultiple(t *testing.T) {
	ps := store.New(nil, nil, "")
	ps.Put("AS-FOO", []netip.Prefix{netip.MustParsePrefix("10.0.0.0/8")}, nil)
	ps.Put("AS-BAR", []netip.Prefix{netip.MustParsePrefix("172.16.0.0/12")}, nil)

	bindings := []ifaceBinding{
		{Interface: "eth1", ASSet: "AS-FOO"},
		{Interface: "eth2", ASSet: "AS-BAR"},
	}
	tables := buildIfaceTables(ps, bindings)

	if len(tables) != 1 {
		t.Fatalf("expected 1 table, got %d", len(tables))
	}
	chain := tables[0].Chains[0]
	// 2 interfaces x (v4 accept + drop) = 4 terms (no v6 data).
	if len(chain.Terms) != 4 {
		t.Errorf("expected 4 terms for 2 v4-only interfaces, got %d", len(chain.Terms))
	}
}

// VALIDATES: shared AS-SET across interfaces produces one set, not duplicates.
// PREVENTS: nftables EEXIST rejection from duplicate set names in same table.
func TestBuildIfaceTablesSharedASSet(t *testing.T) {
	ps := store.New(nil, nil, "")
	ps.Put("AS-FOO", []netip.Prefix{netip.MustParsePrefix("10.0.0.0/8")}, nil)

	bindings := []ifaceBinding{
		{Interface: "eth1", ASSet: "AS-FOO"},
		{Interface: "eth2", ASSet: "AS-FOO"},
	}
	tables := buildIfaceTables(ps, bindings)

	if len(tables) != 1 {
		t.Fatalf("expected 1 table, got %d", len(tables))
	}
	setNames := make(map[string]int)
	for _, s := range tables[0].Sets {
		setNames[s.Name]++
	}
	for name, count := range setNames {
		if count > 1 {
			t.Errorf("duplicate set %q (count %d)", name, count)
		}
	}
	chain := tables[0].Chains[0]
	if len(chain.Terms) != 4 {
		t.Errorf("expected 4 terms (2 interfaces x (v4 accept + drop)), got %d", len(chain.Terms))
	}
}

// VALIDATES: AC-4 no table generated when bindings removed.
// PREVENTS: stale chain remaining after config removal.
func TestBuildIfaceTablesEmpty(t *testing.T) {
	ps := store.New(nil, nil, "")
	tables := buildIfaceTables(ps, nil)
	if len(tables) != 0 {
		t.Errorf("expected 0 tables for empty bindings, got %d", len(tables))
	}
}

// VALIDATES: AC-3 -- an interface bound to an AS-SET with no prefixes never
// produces a table whose only term drops everything on that interface.
// PREVENTS: an IRR answer that learned nothing blackholing a customer port.
func TestBuildIfaceTablesNeverBlackholes(t *testing.T) {
	ps := store.New(nil, nil, "")
	ps.Put("AS-EMPTY", nil, nil)

	bindings := []ifaceBinding{{Interface: "eth1", ASSet: "AS-EMPTY"}}
	tables := buildIfaceTables(ps, bindings)

	for _, tbl := range tables {
		for _, chain := range tbl.Chains {
			for _, term := range chain.Terms {
				if !termDropsInterface(term, "eth1") {
					continue
				}
				t.Fatalf("term %q drops every packet on eth1 with no accept term to precede it", term.Name)
			}
		}
	}
}

// termDropsInterface reports whether term drops every packet arriving on iface:
// its only match is the input interface and its only action is a drop.
func termDropsInterface(term firewall.Term, iface string) bool {
	if len(term.Matches) != 1 || len(term.Actions) != 1 {
		return false
	}
	in, ok := term.Matches[0].(firewall.MatchInputInterface)
	if !ok || in.Name != iface {
		return false
	}
	_, drops := term.Actions[0].(firewall.Drop)
	return drops
}

// VALIDATES: AC-3 -- an interface whose AS-SET has prefixes keeps its drop term,
// so the accept terms still act as a whitelist.
// PREVENTS: the no-blackhole guard disabling ingress filtering outright.
func TestBuildIfaceTablesKeepsDropWhenPopulated(t *testing.T) {
	ps := store.New(nil, nil, "")
	ps.Put("AS-FULL", []netip.Prefix{netip.MustParsePrefix("10.0.0.0/8")}, nil)

	tables := buildIfaceTables(ps, []ifaceBinding{{Interface: "eth1", ASSet: "AS-FULL"}})
	if len(tables) != 1 {
		t.Fatalf("expected 1 table, got %d", len(tables))
	}
	var dropped bool
	for _, term := range tables[0].Chains[0].Terms {
		if termDropsInterface(term, "eth1") {
			dropped = true
		}
	}
	if !dropped {
		t.Fatal("a populated binding must still drop what its accept terms did not match")
	}
}
