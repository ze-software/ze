package plugin

import (
	"net/netip"
	"testing"
)

// TestParseSelectorAll verifies "*" matches all peers.
//
// VALIDATES: Wildcard selector parses correctly.
// PREVENTS: All-peer operations failing.
func TestParseSelectorAll(t *testing.T) {
	sel, err := ParseSelector("*")
	if err != nil {
		t.Fatalf("ParseSelector(*) error: %v", err)
	}
	if !sel.All {
		t.Error("expected All=true for *")
	}
}

// TestParseSelectorIP verifies specific IP parsing.
//
// VALIDATES: IP selectors parse correctly.
// PREVENTS: Single-peer operations failing.
func TestParseSelectorIP(t *testing.T) {
	tests := []struct {
		input string
		want  netip.Addr
	}{
		{"10.0.0.1", netip.MustParseAddr("10.0.0.1")},
		{"192.168.1.1", netip.MustParseAddr("192.168.1.1")},
		{"2001:db8::1", netip.MustParseAddr("2001:db8::1")},
	}

	for _, tc := range tests {
		sel, err := ParseSelector(tc.input)
		if err != nil {
			t.Errorf("ParseSelector(%q) error: %v", tc.input, err)
			continue
		}
		if sel.IP != tc.want {
			t.Errorf("ParseSelector(%q).IP = %v, want %v", tc.input, sel.IP, tc.want)
		}
	}
}

// TestNegatedSelector verifies !<ip> parsing.
//
// VALIDATES: !<ip> matches all except specified.
// PREVENTS: Wrong peer selection, route loops.
func TestNegatedSelector(t *testing.T) {
	sel, err := ParseSelector("!10.0.0.1")
	if err != nil {
		t.Fatalf("ParseSelector(!10.0.0.1) error: %v", err)
	}
	if !sel.Exclude.IsValid() {
		t.Error("expected Exclude to be set")
	}
	if sel.Exclude != netip.MustParseAddr("10.0.0.1") {
		t.Errorf("Exclude = %v, want 10.0.0.1", sel.Exclude)
	}
}

// TestNegatedSelectorIPv6 verifies IPv6 negation.
//
// VALIDATES: IPv6 negation works correctly.
// PREVENTS: IPv6 exclusion bugs.
func TestNegatedSelectorIPv6(t *testing.T) {
	sel, err := ParseSelector("!2001:db8::1")
	if err != nil {
		t.Fatalf("ParseSelector(!2001:db8::1) error: %v", err)
	}
	if sel.Exclude != netip.MustParseAddr("2001:db8::1") {
		t.Errorf("Exclude = %v, want 2001:db8::1", sel.Exclude)
	}
}

// TestSelectorEdgeCases verifies invalid selectors rejected.
//
// VALIDATES: !*, contradictions return errors.
// PREVENTS: Undefined behavior on bad input.
func TestSelectorEdgeCases(t *testing.T) {
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
		_, err := ParseSelector(tc.input)
		if tc.wantErr && err == nil {
			t.Errorf("ParseSelector(%q) expected error", tc.input)
		}
		if !tc.wantErr && err != nil {
			t.Errorf("ParseSelector(%q) unexpected error: %v", tc.input, err)
		}
	}
}

// TestSelectorMatches verifies peer matching logic.
//
// VALIDATES: Selector.Matches() works correctly.
// PREVENTS: Wrong peer selection.
func TestSelectorMatches(t *testing.T) {
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
		sel, err := ParseSelector(tc.selector)
		if err != nil {
			t.Fatalf("ParseSelector(%q) error: %v", tc.selector, err)
		}
		got := sel.Matches(tc.peer)
		if got != tc.want {
			t.Errorf("Selector(%q).Matches(%v) = %v, want %v",
				tc.selector, tc.peer, got, tc.want)
		}
	}
}
