package geodns

import (
	"net/netip"
	"testing"
)

func src(prefix, set string) sourceEntry {
	return sourceEntry{Prefix: netip.MustParsePrefix(prefix).Masked(), HostSet: set}
}

// VALIDATES: the most specific covering prefix wins; a v4 catch-all (0.0.0.0/0)
// and a v6 catch-all (::/0) act as defaults; unmatched yields no host-set.
// PREVENTS: a broad prefix shadowing a more specific per-customer /32, and a
// v4 client matching a v6 catch-all (or vice versa).
func TestLongestPrefixMatch(t *testing.T) {
	m := buildMatcher([]sourceEntry{
		src("82.219.0.0/16", "internal"),
		src("82.219.4.10/32", "host"),
		src("0.0.0.0/0", "external"),
		src("::/0", "external6"),
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
		got, ok := m.lookup(netip.MustParseAddr(tc.ip))
		if ok != tc.ok || got != tc.want {
			t.Errorf("lookup(%s) = (%q,%v), want (%q,%v)", tc.ip, got, ok, tc.want, tc.ok)
		}
	}
}

// VALIDATES: with no covering prefix, lookup reports no match.
// PREVENTS: answering from an arbitrary host-set when nothing matches.
func TestLookupNoMatch(t *testing.T) {
	m := buildMatcher([]sourceEntry{src("10.0.0.0/8", "lan")})
	if _, ok := m.lookup(netip.MustParseAddr("1.2.3.4")); ok {
		t.Error("expected no match for 1.2.3.4")
	}
	if _, ok := m.lookup(netip.MustParseAddr("2a02:b80::1")); ok {
		t.Error("expected no match for v6 when only a v4 prefix exists")
	}
}
