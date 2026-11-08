// Design: docs/architecture/policyroute/policy-routing.md -- netlink ip rule and route management
// Related: iprule_linux.go -- the netlink encoding of Mark and Source
//
// Package iprule builds Linux fib rules (`ip rule`) for every Ze producer that
// steers traffic into a routing table of its own, and declares the table ranges
// those producers allocate from. It is a core leaf: a gated plugin (policyroute)
// and a gated component (the IKE engine) both use it, and neither may import the
// other.
package iprule

import (
	"net/netip"

	"github.com/ze-software/ze/internal/core/rtproto"
)

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

// The table ranges Ze reserves. Each is declared once, here, so a new producer is
// checked against every other (TestTableRangesAreDisjoint), and every operator table
// ID is refused inside any of them (Reserved).
var (
	// VRFTables is held for VRF tables, one per VRF. No producer allocates from it
	// yet; it is reserved so an operator table placed there today does not collide
	// with the VRF tables later.
	VRFTables = TableRange{First: 1000, Last: 1999}
	// PolicyRouteAutoTables is where the policy routing plugin places its auto
	// tables, one per next hop.
	PolicyRouteAutoTables = TableRange{First: 2000, Last: 2999}
	// IPsecSourceGuardTables is where the IKE engine places one table per bound
	// XFRM interface, for the IPv6 tunnel-source guard.
	IPsecSourceGuardTables = TableRange{First: 3000, Last: 3999}
)

// ZeTableRanges lists every range above.
func ZeTableRanges() []TableRange {
	return []TableRange{VRFTables, PolicyRouteAutoTables, IPsecSourceGuardTables}
}

// Reserved answers the Ze range that holds table, and false when no range does. An
// operator table inside one is refused: Ze would overwrite or flush it.
func Reserved(table uint32) (TableRange, bool) {
	for _, r := range ZeTableRanges() {
		if r.Contains(table) {
			return r, true
		}
	}
	return TableRange{}, false
}

// IPsecSourceGuardPriority is the `ip rule` priority of the IKE engine's source
// rules. It sits after the local table's rule (priority 0), so a packet for one of
// the box's own addresses is still delivered locally, and before main (32766). The
// policy routing plugin places its fwmark rules from priority 100 upward, one per
// rule, so a guard rule never shares a priority with one of them.
const IPsecSourceGuardPriority = 30000

// Route is one route in a table a Ze producer owns: a device route to Dst through
// the link at LinkIndex, or, with Unreachable, an unreachable route that answers a
// sender with an error instead of falling through to another table. Protocol names
// the producer, so a delete matches only Ze's own route.
type Route struct {
	Table       uint32
	Dst         netip.Prefix
	LinkIndex   int
	Unreachable bool
	Protocol    rtproto.Proto
}
