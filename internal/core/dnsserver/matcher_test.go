package dnsserver

import (
	"net/netip"
	"testing"
)

func entry(prefix, label string) Entry {
	return Entry{Prefix: netip.MustParsePrefix(prefix).Masked(), Label: label}
}

// VALIDATES: the most specific covering prefix wins; a v4 catch-all
// (0.0.0.0/0) and a v6 catch-all (::/0) act as family-scoped defaults;
// unmatched yields no label -- ported from a consumer plugin's
// TestLongestPrefixMatch (AC-7).
// PREVENTS: a broad prefix shadowing a more specific per-customer /32, and a
// v4 client matching a v6 catch-all (or vice versa).
func TestMatcher_LongestPrefix(t *testing.T) {
	m := BuildMatcher([]Entry{
		entry("82.219.0.0/16", "internal"),
		entry("82.219.4.10/32", "host"),
		entry("0.0.0.0/0", "external"),
		entry("::/0", "external6"),
	})
	cases := []struct {
		ip   string
		want string
		ok   bool
	}{
		{"82.219.4.10", "host", true},      // exact /32 beats /16
		{"82.219.99.99", "internal", true}, // /16 beats catch-all
		{"1.1.1.1", "external", true},      // v4 catch-all
		{"2a02:b80::1", "external6", true}, // v6 catch-all, not the v4 one
	}
	for _, tc := range cases {
		got, ok := m.Lookup(netip.MustParseAddr(tc.ip))
		if ok != tc.ok || got != tc.want {
			t.Errorf("Lookup(%s) = (%q,%v), want (%q,%v)", tc.ip, got, ok, tc.want, tc.ok)
		}
	}
}

// VALIDATES: with no covering prefix, Lookup reports no match.
// PREVENTS: answering from an arbitrary label when nothing matches.
func TestMatcher_NoMatch(t *testing.T) {
	m := BuildMatcher([]Entry{entry("10.0.0.0/8", "lan")})
	if _, ok := m.Lookup(netip.MustParseAddr("1.2.3.4")); ok {
		t.Error("expected no match for 1.2.3.4")
	}
	if _, ok := m.Lookup(netip.MustParseAddr("2a02:b80::1")); ok {
		t.Error("expected no match for v6 when only a v4 prefix exists")
	}
}
