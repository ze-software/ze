package dnsserver

import (
	"net/netip"
	"testing"
)

// VALIDATES: Matcher.Scope returns the shortest prefix length over which
// Lookup gives the client's address one answer: the matched entry's length,
// longer where another entry lies inside it, the shortest network that holds no
// entry for an unmatched address, and 0 when no entry is of the address's
// family. Method: a table over IPv4 and IPv6 entries, and for every row a check
// that each entry outside the scope network stays outside it.
// PREVENTS: a SCOPE PREFIX-LENGTH that hands one source prefix's answer to the
// clients of another.
func TestMatcherScope(t *testing.T) {
	m := BuildMatcher([]Entry{
		{Prefix: netip.MustParsePrefix("10.0.0.0/16"), Label: "wide"},
		{Prefix: netip.MustParsePrefix("10.0.5.0/24"), Label: "narrow"},
		{Prefix: netip.MustParsePrefix("2001:db8::/32"), Label: "six"},
	})
	cases := []struct {
		addr string
		want uint8
	}{
		{"10.0.1.0", 22},     // inside /16, kept off 10.0.5.0/24
		{"10.0.5.9", 24},     // the exception itself
		{"10.1.0.0", 16},     // no entry: 10.1.0.0/16 holds none
		{"192.0.2.1", 1},     // no entry: 128.0.0.0/1 holds none
		{"2001:db8:1::", 32}, // the IPv6 entry
		{"2001:db9::", 32},   // no entry: 2001:db9::/32 holds none
	}
	for _, tc := range cases {
		addr := netip.MustParseAddr(tc.addr)
		got := m.Scope(addr)
		if got != tc.want {
			t.Errorf("Scope(%s) = %d, want %d", addr, got, tc.want)
		}
		label, held := m.Lookup(addr)
		scope := netip.PrefixFrom(addr, int(got)).Masked()
		for _, e := range m.entries {
			if e.Prefix.Contains(addr) {
				continue
			}
			if scope.Overlaps(e.Prefix) && e.Prefix.Bits() > scope.Bits() {
				t.Errorf("Scope(%s) = %s holds %s, which answers otherwise than %q (held %v)", addr, scope, e.Prefix, label, held)
			}
		}
	}

	if got := BuildMatcher([]Entry{{Prefix: netip.MustParsePrefix("2001:db8::/32"), Label: "six"}}).Scope(netip.MustParseAddr("192.0.2.1")); got != 0 {
		t.Errorf("Scope of an IPv4 address with only IPv6 entries = %d, want 0", got)
	}
}
