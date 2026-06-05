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
	if sel.SelectorKind() != KindAll {
		t.Errorf("expected KindAll, got %v", sel.SelectorKind())
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

	for _, tt := range tests {
		sel, err := Parse(tt.input)
		if err != nil {
			t.Errorf("Parse(%q) error: %v", tt.input, err)
			continue
		}
		if sel.SelectorKind() != KindAddr {
			t.Errorf("Parse(%q) kind = %v, want KindAddr", tt.input, sel.SelectorKind())
		}
		if sel.IP() != tt.want {
			t.Errorf("Parse(%q).IP() = %v, want %v", tt.input, sel.IP(), tt.want)
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
	if sel.SelectorKind() != KindAddr || !sel.IsExclude() {
		t.Errorf("expected KindAddr+exclude, got %v exclude=%v", sel.SelectorKind(), sel.IsExclude())
	}
	want := netip.MustParseAddr("10.0.0.1")
	if sel.IP() != want {
		t.Errorf("IP() = %v, want %v", sel.IP(), want)
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
	want := netip.MustParseAddr("2001:db8::1")
	if sel.IP() != want {
		t.Errorf("IP() = %v, want %v", sel.IP(), want)
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
		{"!invalid", false},      // Exclude peer name
		{"", true},               // Empty
		{"  ", true},             // Whitespace only
		{"10.0.0.1", false},      // Valid IP
		{"!10.0.0.1", false},     // Valid exclude
		{"*", false},             // Valid all
		{"  10.0.0.1  ", false},  // Whitespace trimmed
		{"  !10.0.0.1  ", false}, // Whitespace with exclude
	}

	for _, tt := range tests {
		_, err := Parse(tt.input)
		if tt.wantErr && err == nil {
			t.Errorf("Parse(%q) expected error", tt.input)
		}
		if !tt.wantErr && err != nil {
			t.Errorf("Parse(%q) unexpected error: %v", tt.input, err)
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

	for _, tt := range tests {
		sel, err := Parse(tt.input)
		if err != nil {
			t.Errorf("Parse(%q) error: %v", tt.input, err)
			continue
		}
		if sel.SelectorKind() != KindAddrs {
			t.Errorf("Parse(%q) kind = %v, want KindAddrs", tt.input, sel.SelectorKind())
			continue
		}
		ips := sel.IPs()
		if len(ips) != len(tt.want) {
			t.Errorf("Parse(%q).IPs() len = %d, want %d", tt.input, len(ips), len(tt.want))
			continue
		}
		for i, ip := range ips {
			if ip != tt.want[i] {
				t.Errorf("Parse(%q).IPs()[%d] = %v, want %v", tt.input, i, ip, tt.want[i])
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

	for _, tt := range tests {
		got := sel.Matches(tt.peer)
		if got != tt.want {
			t.Errorf("Selector(%q).Matches(%v) = %v, want %v",
				"10.0.0.1,10.0.0.3", tt.peer, got, tt.want)
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

	for _, tt := range tests {
		sel, err := Parse(tt.selector)
		if err != nil {
			t.Fatalf("Parse(%q) error: %v", tt.selector, err)
		}
		got := sel.Matches(tt.peer)
		if got != tt.want {
			t.Errorf("Selector(%q).Matches(%v) = %v, want %v",
				tt.selector, tt.peer, got, tt.want)
		}
	}
}

// TestParseASN verifies "as<N>" selector parsing.
//
// VALIDATES: ASN selectors parse correctly.
// PREVENTS: ASN-based peer selection failing.
func TestParseASN(t *testing.T) {
	tests := []struct {
		input   string
		wantASN uint32
	}{
		{"as65000", 65000},
		{"AS65000", 65000},
		{"As1", 1},
		{"as4294967295", 4294967295},
	}
	for _, tt := range tests {
		sel, err := Parse(tt.input)
		if err != nil {
			t.Errorf("Parse(%q) error: %v", tt.input, err)
			continue
		}
		if sel.SelectorKind() != KindASN {
			t.Errorf("Parse(%q) kind = %v, want KindASN", tt.input, sel.SelectorKind())
		}
		if sel.ASNValue() != tt.wantASN {
			t.Errorf("Parse(%q).ASNValue() = %d, want %d", tt.input, sel.ASNValue(), tt.wantASN)
		}
	}
}

// TestParseGlob verifies glob pattern selector parsing.
//
// VALIDATES: Glob selectors parse correctly.
// PREVENTS: Pattern-based peer selection failing.
func TestParseGlob(t *testing.T) {
	tests := []string{
		"192.168.*.*",
		"10.*.0.1",
		"*.*.*.1",
	}
	for _, input := range tests {
		sel, err := Parse(input)
		if err != nil {
			t.Errorf("Parse(%q) error: %v", input, err)
			continue
		}
		if sel.SelectorKind() != KindGlob {
			t.Errorf("Parse(%q) kind = %v, want KindGlob", input, sel.SelectorKind())
		}
		if sel.GlobPattern() != input {
			t.Errorf("Parse(%q).GlobPattern() = %q", input, sel.GlobPattern())
		}
	}
}

// TestParseName verifies peer name selector parsing.
//
// VALIDATES: Name selectors parse correctly.
// PREVENTS: Name-based peer selection failing.
func TestParseName(t *testing.T) {
	tests := []string{
		"peer1",
		"my-router",
		"core_rr_01",
	}
	for _, input := range tests {
		sel, err := Parse(input)
		if err != nil {
			t.Errorf("Parse(%q) error: %v", input, err)
			continue
		}
		if sel.SelectorKind() != KindName {
			t.Errorf("Parse(%q) kind = %v, want KindName", input, sel.SelectorKind())
		}
		if sel.NameValue() != input {
			t.Errorf("Parse(%q).NameValue() = %q", input, sel.NameValue())
		}
	}
}

// TestParseInvalidNowName verifies strings that were previously rejected as
// "not an IP" are now parsed as peer names.
//
// VALIDATES: Parse accepts peer names that look like invalid IPs.
// PREVENTS: Regression on name support.
func TestParseInvalidNowName(t *testing.T) {
	sel, err := Parse("invalid")
	if err != nil {
		t.Fatalf("Parse(%q) error: %v", "invalid", err)
	}
	if sel.SelectorKind() != KindName {
		t.Errorf("Parse(%q) kind = %v, want KindName", "invalid", sel.SelectorKind())
	}
}

// TestStringRoundTrip verifies that Parse(sel.String()) == sel for all kinds.
//
// VALIDATES: String representation round-trips through Parse.
// PREVENTS: Serialization bugs at RPC boundaries.
func TestStringRoundTrip(t *testing.T) {
	tests := []struct {
		name string
		sel  *Selector
		want string
	}{
		{"all", All(), "*"},
		{"addr", Addr(netip.MustParseAddr("10.0.0.1")), "10.0.0.1"},
		{"exclude-addr", ExcludeAddr(netip.MustParseAddr("10.0.0.1")), "!10.0.0.1"},
		{"exclude-name", excludeName("upstream"), "!upstream"},
		{"multi", MultiAddr([]netip.Addr{
			netip.MustParseAddr("10.0.0.1"),
			netip.MustParseAddr("10.0.0.2"),
		}), "10.0.0.1,10.0.0.2"},
		{"name", PeerName("peer1"), "peer1"},
		{"asn", ASN(65000), "as65000"},
		{"glob", Glob("192.168.*.*"), "192.168.*.*"},
	}

	for _, tt := range tests {
		got := tt.sel.String()
		if got != tt.want {
			t.Errorf("%s: String() = %q, want %q", tt.name, got, tt.want)
		}
		reparsed, err := Parse(got)
		if err != nil {
			t.Errorf("%s: Parse(%q) error: %v", tt.name, got, err)
			continue
		}
		if reparsed.SelectorKind() != tt.sel.SelectorKind() {
			t.Errorf("%s: round-trip kind = %v, want %v", tt.name, reparsed.SelectorKind(), tt.sel.SelectorKind())
		}
	}
}

// TestConstructorMatchesEquivalence verifies constructors produce the same
// matching behavior as Parse.
//
// VALIDATES: Constructors and Parse are interchangeable.
// PREVENTS: Constructor/Parse divergence.
func TestConstructorMatchesEquivalence(t *testing.T) {
	addr := netip.MustParseAddr("10.0.0.1")
	other := netip.MustParseAddr("10.0.0.2")

	pairs := []struct {
		name string
		a    *Selector
		b    *Selector
	}{
		{"all", All(), must(Parse("*"))},
		{"addr", Addr(addr), must(Parse("10.0.0.1"))},
		{"exclude", ExcludeAddr(addr), must(Parse("!10.0.0.1"))},
	}

	for _, p := range pairs {
		for _, peer := range []netip.Addr{addr, other} {
			a := p.a.Matches(peer)
			b := p.b.Matches(peer)
			if a != b {
				t.Errorf("%s: constructor.Matches(%v) = %v, Parse.Matches(%v) = %v",
					p.name, peer, a, peer, b)
			}
		}
	}
}

// TestNameASNGlobMatchesReturnsFalse verifies that Matches returns false
// for kinds that require more than an IP address.
//
// VALIDATES: Name/ASN/Glob selectors don't accidentally match by IP.
// PREVENTS: False positive peer selection.
func TestNameASNGlobMatchesReturnsFalse(t *testing.T) {
	addr := netip.MustParseAddr("10.0.0.1")
	sels := []*Selector{
		PeerName("peer1"),
		ASN(65000),
		Glob("10.*.*.*"),
	}
	for _, sel := range sels {
		if sel.Matches(addr) {
			t.Errorf("%v.Matches(%v) = true, want false", sel, addr)
		}
	}
}

// TestMatchesPeerKeyExcludeNonIP verifies that an IP-exclude selector
// correctly includes non-IP peer keys (e.g. BMP "router1:peer1").
//
// VALIDATES: MatchesPeerKey with exclude + non-IP peer returns true.
// PREVENTS: Regression where non-IP peers were wrongly excluded by IP selectors.
func TestMatchesPeerKeyExcludeNonIP(t *testing.T) {
	sel := ExcludeAddr(netip.MustParseAddr("10.0.0.1"))

	tests := []struct {
		peerKey string
		want    bool
	}{
		{"10.0.0.1", false},         // excluded IP
		{"10.0.0.2", true},          // different IP
		{"router1:peer1", true},     // non-IP key, not excluded
		{"bmp-collector:rr1", true}, // non-IP key, not excluded
	}

	for _, tt := range tests {
		got := sel.MatchesPeerKey(tt.peerKey)
		if got != tt.want {
			t.Errorf("ExcludeAddr(10.0.0.1).MatchesPeerKey(%q) = %v, want %v",
				tt.peerKey, got, tt.want)
		}
	}
}

// TestMatchesPeerKeyNameExclude verifies that name-exclude works on both
// IP and non-IP peer keys.
//
// VALIDATES: MatchesPeerKey with !name excludes named peer, includes others.
// PREVENTS: Name exclusion breaking for IP-keyed peers.
func TestMatchesPeerKeyNameExclude(t *testing.T) {
	sel := must(Parse("!upstream"))

	tests := []struct {
		peerKey string
		want    bool
	}{
		{"upstream", false},  // excluded name
		{"downstream", true}, // different name
		{"10.0.0.1", true},   // IP key, not the excluded name
	}

	for _, tt := range tests {
		got := sel.MatchesPeerKey(tt.peerKey)
		if got != tt.want {
			t.Errorf("Parse(\"!upstream\").MatchesPeerKey(%q) = %v, want %v",
				tt.peerKey, got, tt.want)
		}
	}
}

// TestParseDefaultMalformedMultiIP verifies that malformed multi-IP input
// falls back to PeerName (fail-closed) instead of All (fail-open).
//
// VALIDATES: ParseDefault on malformed input doesn't match everything.
// PREVENTS: Regression where parse errors caused match-all.
func TestParseDefaultMalformedMultiIP(t *testing.T) {
	sel := ParseDefault("10.0.0.1,,10.0.0.2")
	if sel.SelectorKind() != KindName {
		t.Errorf("ParseDefault malformed multi-IP: kind = %v, want KindName", sel.SelectorKind())
	}
	if sel.MatchesPeerKey("10.0.0.1") {
		t.Error("malformed multi-IP should not match any peer")
	}
}

// TestParseExclamationName verifies that "!<name>" is a valid exclude-by-name
// selector, not an error. A peer literally named "!router1" cannot be expressed
// in the string syntax (the "!" is always parsed as exclude prefix).
//
// VALIDATES: AC-7 -- "!" prefix is always exclusion, never part of a name.
// PREVENTS: Confusion between exclude syntax and peer names containing "!".
func TestParseExclamationName(t *testing.T) {
	sel, err := Parse("!router1")
	if err != nil {
		t.Fatalf("Parse(\"!router1\") error: %v", err)
	}
	if sel.SelectorKind() != KindName {
		t.Errorf("Parse(\"!router1\") kind = %v, want KindName", sel.SelectorKind())
	}
	if !sel.IsExclude() {
		t.Error("Parse(\"!router1\") should have exclude flag set")
	}
	if sel.NameValue() != "router1" {
		t.Errorf("Parse(\"!router1\").NameValue() = %q, want \"router1\"", sel.NameValue())
	}
	if sel.String() != "!router1" {
		t.Errorf("String() = %q, want \"!router1\"", sel.String())
	}
}

// TestParseDefaultEmpty verifies ParseDefault("") returns KindAll.
//
// VALIDATES: AC-8 -- empty string defaults to all peers.
// PREVENTS: Empty selector causing errors or matching nothing.
func TestParseDefaultEmpty(t *testing.T) {
	sel := ParseDefault("")
	if sel.SelectorKind() != KindAll {
		t.Errorf("ParseDefault(\"\") kind = %v, want KindAll", sel.SelectorKind())
	}
}

// TestParseDefaultStar verifies ParseDefault("*") returns KindAll.
//
// VALIDATES: AC-8 -- star defaults to all peers.
// PREVENTS: Wildcard handling inconsistency.
func TestParseDefaultStar(t *testing.T) {
	sel := ParseDefault("*")
	if sel.SelectorKind() != KindAll {
		t.Errorf("ParseDefault(\"*\") kind = %v, want KindAll", sel.SelectorKind())
	}
}

func must(sel *Selector, err error) *Selector {
	if err != nil {
		panic(err)
	}
	return sel
}
