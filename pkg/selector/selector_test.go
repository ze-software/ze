package selector

import (
	"net/netip"
	"testing"
)

// TestParseAll verifies "*" matches all peers.
//
// VALIDATES: Wildcard selector parses correctly.
// PREVENTS: All-peer operations failing.
func TestParseAll(t *testing.T) {
	sel, err := Parse("*")
	if err != nil {
		t.Fatalf("Parse(*) error: %v", err)
	}
	if !sel.All {
		t.Error("expected All=true for *")
	}
}

// TestParseIP verifies specific IP parsing.
//
// VALIDATES: IP selectors parse correctly.
// PREVENTS: Single-peer operations failing.
func TestParseIP(t *testing.T) {
	tests := []struct {
		input string
		want  netip.Addr
	}{
		{"10.0.0.1", netip.MustParseAddr("10.0.0.1")},
		{"192.168.1.1", netip.MustParseAddr("192.168.1.1")},
		{"2001:db8::1", netip.MustParseAddr("2001:db8::1")},
	}

	for _, tc := range tests {
		sel, err := Parse(tc.input)
		if err != nil {
			t.Errorf("Parse(%q) error: %v", tc.input, err)
			continue
		}
		if sel.IP != tc.want {
			t.Errorf("Parse(%q).IP = %v, want %v", tc.input, sel.IP, tc.want)
		}
	}
}

// TestParseNegated verifies !<ip> parsing.
//
// VALIDATES: !<ip> matches all except specified.
// PREVENTS: Wrong peer selection, route loops.
func TestParseNegated(t *testing.T) {
	sel, err := Parse("!10.0.0.1")
	if err != nil {
		t.Fatalf("Parse(!10.0.0.1) error: %v", err)
	}
	if !sel.Exclude.IsValid() {
		t.Error("expected Exclude to be set")
	}
	if sel.Exclude != netip.MustParseAddr("10.0.0.1") {
		t.Errorf("Exclude = %v, want 10.0.0.1", sel.Exclude)
	}
}

// TestParseNegatedIPv6 verifies IPv6 negation.
//
// VALIDATES: IPv6 negation works correctly.
// PREVENTS: IPv6 exclusion bugs.
func TestParseNegatedIPv6(t *testing.T) {
	sel, err := Parse("!2001:db8::1")
	if err != nil {
		t.Fatalf("Parse(!2001:db8::1) error: %v", err)
	}
	if sel.Exclude != netip.MustParseAddr("2001:db8::1") {
		t.Errorf("Exclude = %v, want 2001:db8::1", sel.Exclude)
	}
}

// TestParseEdgeCases verifies invalid selectors rejected.
//
// VALIDATES: !*, contradictions return errors.
// PREVENTS: Undefined behavior on bad input.
func TestParseEdgeCases(t *testing.T) {
	tests := []struct {
		input   string
		wantErr bool
	}{
		{"!*", true},             // Cannot exclude all
		{"!", true},              // Empty exclude
		{"invalid", true},        // Not an IP
		{"!invalid", true},       // Invalid exclude IP
		{"", true},               // Empty
		{"  ", true},             // Whitespace only
		{"10.0.0.1", false},      // Valid IP
		{"!10.0.0.1", false},     // Valid exclude
		{"*", false},             // Valid all
		{"  10.0.0.1  ", false},  // Whitespace trimmed
		{"  !10.0.0.1  ", false}, // Whitespace with exclude
	}

	for _, tc := range tests {
		_, err := Parse(tc.input)
		if tc.wantErr && err == nil {
			t.Errorf("Parse(%q) expected error", tc.input)
		}
		if !tc.wantErr && err != nil {
			t.Errorf("Parse(%q) unexpected error: %v", tc.input, err)
		}
	}
}

// TestParseMultiIP verifies comma-separated IP parsing.
//
// VALIDATES: Multiple IPs parse correctly into IPs slice.
// PREVENTS: Multi-peer operations failing.
func TestParseMultiIP(t *testing.T) {
	tests := []struct {
		input string
		want  []netip.Addr
	}{
		{
			"10.0.0.1,10.0.0.2",
			[]netip.Addr{netip.MustParseAddr("10.0.0.1"), netip.MustParseAddr("10.0.0.2")},
		},
		{
			"10.0.0.1,10.0.0.2,10.0.0.3",
			[]netip.Addr{netip.MustParseAddr("10.0.0.1"), netip.MustParseAddr("10.0.0.2"), netip.MustParseAddr("10.0.0.3")},
		},
		{
			"2001:db8::1,2001:db8::2",
			[]netip.Addr{netip.MustParseAddr("2001:db8::1"), netip.MustParseAddr("2001:db8::2")},
		},
		{
			"10.0.0.1,2001:db8::1", // Mixed IPv4/IPv6
			[]netip.Addr{netip.MustParseAddr("10.0.0.1"), netip.MustParseAddr("2001:db8::1")},
		},
		{
			" 10.0.0.1 , 10.0.0.2 ", // Whitespace around IPs
			[]netip.Addr{netip.MustParseAddr("10.0.0.1"), netip.MustParseAddr("10.0.0.2")},
		},
	}

	for _, tc := range tests {
		sel, err := Parse(tc.input)
		if err != nil {
			t.Errorf("Parse(%q) error: %v", tc.input, err)
			continue
		}
		if len(sel.IPs) != len(tc.want) {
			t.Errorf("Parse(%q).IPs len = %d, want %d", tc.input, len(sel.IPs), len(tc.want))
			continue
		}
		for i, ip := range sel.IPs {
			if ip != tc.want[i] {
				t.Errorf("Parse(%q).IPs[%d] = %v, want %v", tc.input, i, ip, tc.want[i])
			}
		}
	}
}

// TestParseMultiIPErrors verifies invalid multi-IP selectors rejected.
//
// VALIDATES: Invalid multi-IP formats return errors.
// PREVENTS: Bad input causing undefined behavior.
func TestParseMultiIPErrors(t *testing.T) {
	tests := []string{
		"10.0.0.1,",          // Trailing comma
		",10.0.0.1",          // Leading comma
		"10.0.0.1,,10.0.0.2", // Empty item
		"10.0.0.1,invalid",   // Invalid IP in list
		"invalid,10.0.0.1",   // Invalid IP first
		"!10.0.0.1,10.0.0.2", // Negation with multi-IP not supported
		"*,10.0.0.1",         // Wildcard mixed with IP (invalid)
		"10.0.0.1,*",         // IP mixed with wildcard (invalid)
	}

	for _, input := range tests {
		_, err := Parse(input)
		if err == nil {
			t.Errorf("Parse(%q) expected error", input)
		}
	}
}

// TestMatchesMultiIP verifies multi-IP matching logic.
//
// VALIDATES: Selector.Matches() works with IPs slice.
// PREVENTS: Multi-peer selection bugs.
func TestMatchesMultiIP(t *testing.T) {
	sel, err := Parse("10.0.0.1,10.0.0.3")
	if err != nil {
		t.Fatalf("Parse error: %v", err)
	}

	tests := []struct {
		peer netip.Addr
		want bool
	}{
		{netip.MustParseAddr("10.0.0.1"), true},  // In list
		{netip.MustParseAddr("10.0.0.2"), false}, // Not in list
		{netip.MustParseAddr("10.0.0.3"), true},  // In list
		{netip.MustParseAddr("10.0.0.4"), false}, // Not in list
	}

	for _, tc := range tests {
		got := sel.Matches(tc.peer)
		if got != tc.want {
			t.Errorf("Selector(%q).Matches(%v) = %v, want %v",
				"10.0.0.1,10.0.0.3", tc.peer, got, tc.want)
		}
	}
}

// TestStringMultiIP verifies String() for multi-IP selector.
//
// VALIDATES: Multi-IP selector serializes correctly.
// PREVENTS: Display/logging bugs.
func TestStringMultiIP(t *testing.T) {
	sel, err := Parse("10.0.0.1,10.0.0.2")
	if err != nil {
		t.Fatalf("Parse error: %v", err)
	}

	got := sel.String()
	if got != "10.0.0.1,10.0.0.2" {
		t.Errorf("String() = %q, want %q", got, "10.0.0.1,10.0.0.2")
	}
}

// TestMatches verifies peer matching logic.
//
// VALIDATES: Selector.Matches() works correctly.
// PREVENTS: Wrong peer selection.
func TestMatches(t *testing.T) {
	peers := []netip.Addr{
		netip.MustParseAddr("10.0.0.1"),
		netip.MustParseAddr("10.0.0.2"),
		netip.MustParseAddr("10.0.0.3"),
	}

	tests := []struct {
		selector string
		peer     netip.Addr
		want     bool
	}{
		// All selector
		{"*", peers[0], true},
		{"*", peers[1], true},
		{"*", peers[2], true},
		// Specific IP
		{"10.0.0.1", peers[0], true},
		{"10.0.0.1", peers[1], false},
		{"10.0.0.2", peers[1], true},
		// Exclude
		{"!10.0.0.1", peers[0], false},
		{"!10.0.0.1", peers[1], true},
		{"!10.0.0.1", peers[2], true},
	}

	for _, tc := range tests {
		sel, err := Parse(tc.selector)
		if err != nil {
			t.Fatalf("Parse(%q) error: %v", tc.selector, err)
		}
		got := sel.Matches(tc.peer)
		if got != tc.want {
			t.Errorf("Selector(%q).Matches(%v) = %v, want %v",
				tc.selector, tc.peer, got, tc.want)
		}
	}
}
