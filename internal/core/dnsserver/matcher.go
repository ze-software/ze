// Design: docs/architecture/dns/server-harness.md -- CIDR longest-prefix matcher
// (secondary extraction; the host-set/label semantics stay with the consumer).

package dnsserver

import (
	"math/bits"
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

// Scope returns the SCOPE PREFIX-LENGTH (RFC 7871 Section 6) of the answer
// Lookup gives ip: the shortest prefix length L for which every address in the
// network ip/L draws the same entry from Lookup, or no entry when ip draws none.
//
// L is at least the length of the entry that holds ip, and longer where an
// entry that does not hold ip lies inside that network, so the scope never
// covers an address another entry answers.
// RFC 7871 Section 7.2.1: "Because it can't be guaranteed that queries for all
// longer prefix lengths would arrive before one that would be answered by the
// shorter prefix length, an Authoritative Nameserver MUST NOT overlap
// prefixes."
// That section lets the server deaggregate the shorter prefix, and this is that
// deaggregation, done for the one network the query falls in: an entry that does
// not hold ip shares its first n bits with ip, and a scope of n+1 bits leaves it
// out. An entry shorter than the one that holds ip has n+1 at most its own length,
// so it never lengthens the scope. The result is at most the address width.
func (m *Matcher) Scope(ip netip.Addr) uint8 {
	scope, held := 0, false
	for _, e := range m.entries {
		if e.Prefix.Addr().Is4() != ip.Is4() {
			continue
		}
		if !e.Prefix.Contains(ip) {
			scope = max(scope, commonBits(ip, e.Prefix.Addr())+1)
			continue
		}
		// entries is sorted most specific first, so the first entry that holds
		// ip is the one Lookup answers with.
		if !held {
			held = true
			scope = max(scope, e.Prefix.Bits())
		}
	}
	return uint8(scope)
}

// commonBits returns how many leading bits a and b share. Both are of one
// family, and differ, because the caller passes an address and a prefix that
// does not hold it.
func commonBits(a, b netip.Addr) int {
	x, y := a.As16(), b.As16()
	n := len(x) * 8
	for i := range x {
		if d := x[i] ^ y[i]; d != 0 {
			n = i*8 + bits.LeadingZeros8(d)
			break
		}
	}
	if a.Is4() {
		// As16 puts an IPv4 address after the 96 bits of the ::ffff: prefix.
		n -= 96
	}
	return n
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
