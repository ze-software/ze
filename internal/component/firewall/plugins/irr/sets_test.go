package irr

// VALIDATES: AC-4 interval sets created with correct prefixes
// VALIDATES: set naming convention irr_v4_/irr_v6_ prefix
// PREVENTS: mismatched set names between config parser and plugin

import (
	"net/netip"
	"testing"

	"codeberg.org/thomas-mangin/ze/internal/component/firewall"
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

func TestBuildSetsEmptyBoth(t *testing.T) {
	sets := buildSets("AS13335", nil, nil)
	if len(sets) != 0 {
		t.Errorf("expected 0 sets for empty prefix lists, got %d", len(sets))
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
