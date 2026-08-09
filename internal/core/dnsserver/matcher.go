// Design: docs/architecture/dns/server-harness.md -- CIDR longest-prefix matcher
// (secondary extraction; the host-set/label semantics stay with the consumer).

package dnsserver

import (
	"net/netip"
	"sort"
)

// Entry maps a client-IP prefix to an opaque label (longest prefix wins). The
// label's meaning (a host-set name, a pool ID, ...) is entirely the
// consumer's; the matcher only does longest-prefix selection.
type Entry struct {
	Prefix netip.Prefix
	Label  string
}

// Matcher resolves a client IP to a label by longest-prefix match over the
// configured entries. Many entries may share one label; a 0.0.0.0/0 or ::/0
// entry acts as a family-scoped catch-all default.
type Matcher struct {
	entries []Entry // sorted by prefix length descending (most specific first)
}

// BuildMatcher orders entries most-specific-first so the first covering
// prefix Lookup finds is the longest match.
func BuildMatcher(entries []Entry) *Matcher {
	es := make([]Entry, len(entries))
	copy(es, entries)
	sort.SliceStable(es, func(i, j int) bool {
		return es[i].Prefix.Bits() > es[j].Prefix.Bits()
	})
	return &Matcher{entries: es}
}

// Lookup returns the label for the most specific entry that contains ip.
// netip.Prefix.Contains is family-aware, so a v4 client never matches a v6
// prefix and vice versa.
func (m *Matcher) Lookup(ip netip.Addr) (string, bool) {
	for _, e := range m.entries {
		if e.Prefix.Contains(ip) {
			return e.Label, true
		}
	}
	return "", false
}
