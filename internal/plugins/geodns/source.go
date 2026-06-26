// Design: plan/learned/992-geodns-1-config.md -- geodns source matcher (CIDR longest-prefix)

package geodns

import (
	"net/netip"
	"sort"
)

// matcher resolves a client IP to a host-set name by longest-prefix match over
// the configured source prefixes. Many prefixes may point at one host-set; the
// catch-all (0.0.0.0/0 for IPv4, ::/0 for IPv6) is the "external" default.
type matcher struct {
	entries []sourceEntry // sorted by prefix length descending (most specific first)
}

// buildMatcher orders the source entries most-specific-first so the first
// covering prefix found by lookup is the longest match.
func buildMatcher(sources []sourceEntry) *matcher {
	es := make([]sourceEntry, len(sources))
	copy(es, sources)
	sort.SliceStable(es, func(i, j int) bool {
		return es[i].Prefix.Bits() > es[j].Prefix.Bits()
	})
	return &matcher{entries: es}
}

// lookup returns the host-set name for the most specific source prefix that
// contains ip. netip.Prefix.Contains is family-aware, so a v4 client never
// matches a v6 prefix and vice versa.
func (m *matcher) lookup(ip netip.Addr) (string, bool) {
	for _, e := range m.entries {
		if e.Prefix.Contains(ip) {
			return e.HostSet, true
		}
	}
	return "", false
}
