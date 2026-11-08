// Design: docs/architecture/policyroute/policy-routing.md -- netlink ip rule and route management
// Related: iprule_linux.go -- the netlink encoding of Mark and Source
//
// Package iprule builds Linux fib rules (`ip rule`) for every Ze producer that
// steers traffic into a routing table of its own, and declares the table ranges
// those producers allocate from. It is a core leaf: a gated plugin (policyroute)
// and a gated component (the IKE engine) both use it, and neither may import the
// other.
package iprule

import "net/netip"

// Mark is an IPv4 fwmark rule: `fwmark <Mark>/<Mask> lookup <Table>` at Priority.
type Mark struct {
	Priority int
	Mark     uint32
	Mask     uint32
	Table    uint32
}

// Source is a source-prefix rule: `from <From> lookup <Table>` at Priority. The
// rule's family is the prefix's: an IPv4 prefix makes an IPv4 rule, an IPv6 prefix
// an IPv6 rule. A 4-in-6 prefix is refused rather than guessed at.
type Source struct {
	Priority int
	From     netip.Prefix
	Table    uint32
}

// TableRange is a run of routing table IDs one Ze producer allocates from, First
// and Last included.
type TableRange struct {
	First uint32
	Last  uint32
}

// Contains reports whether table is inside the range.
func (r TableRange) Contains(table uint32) bool {
	return table >= r.First && table <= r.Last
}

// Overlaps reports whether the two ranges share a table ID.
func (r TableRange) Overlaps(other TableRange) bool {
	return r.First <= other.Last && other.First <= r.Last
}

// The table ranges Ze producers allocate from. Each is declared once, here, so a
// new producer is checked against every other (TestTableRangesAreDisjoint).
// docs/architecture/policyroute/policy-routing.md names the operator side: VRF
// tables take 1000 to 1999, and an operator table ID in 1000 to 2999 is refused at
// parse time as Ze-reserved.
var (
	// PolicyRouteAutoTables is where the policy routing plugin places its auto
	// tables, one per next hop.
	PolicyRouteAutoTables = TableRange{First: 2000, Last: 2999}
	// IPsecSourceGuardTables is where the IKE engine places one table per bound
	// XFRM interface, for the IPv6 tunnel-source guard.
	IPsecSourceGuardTables = TableRange{First: 3000, Last: 3999}
)

// ZeTableRanges lists every range above, for the disjointness check.
func ZeTableRanges() []TableRange {
	return []TableRange{PolicyRouteAutoTables, IPsecSourceGuardTables}
}
