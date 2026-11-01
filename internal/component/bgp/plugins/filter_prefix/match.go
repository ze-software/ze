// Design: docs/architecture/core-design.md -- prefix-list filter matching
// Related: config.go -- prefix-list config parsing
// Related: condition.go -- the prefix-list condition of a match instance
// Related: rib_evaluator.go -- the Loc-RIB import gate's evaluator
//
// Package filter_prefix implements bgp-filter-prefix.
//
// The matching algorithm walks an ordered list of prefixEntry values.
// For each route prefix, the first entry whose CIDR contains the route
// AND whose ge/le range contains the route's prefix length is selected;
// its action is applied. No-match yields actionReject (implicit deny).
//
// evaluatePrefix is the one judgement of a prefix: the prefix-list condition
// of a match instance at a BGP chain (condition.go) and the Loc-RIB import
// gate (rib_evaluator.go) both call it.
package filter_prefix

import "net/netip"

// action is the per-entry decision applied when an entry matches.
type action int

const (
	actionAccept action = iota
	actionReject
)

func (a action) String() string {
	if a == actionAccept {
		return "accept"
	}
	return "reject"
}

// prefixEntry is one ordered match entry inside a prefix-list.
// Order matters: first match wins.
type prefixEntry struct {
	prefix netip.Prefix
	ge     uint8
	le     uint8
	action action
}

// prefixList is a named ordered list of match entries.
type prefixList struct {
	name    string
	entries []prefixEntry
}

// evaluatePrefix walks the entries in order and returns the action of the
// first entry that matches the route. Returns actionReject if no entry matches
// (implicit deny). The route prefix's address family must equal the entry's,
// and its prefix length must satisfy ge <= bits <= le.
func evaluatePrefix(entries []prefixEntry, route netip.Prefix) action {
	routeBits := uint8(route.Bits())
	routeIs4 := route.Addr().Is4()

	for i := range entries {
		e := &entries[i]
		// Cross-family entries cannot match.
		if e.prefix.Addr().Is4() != routeIs4 {
			continue
		}
		if routeBits < e.ge || routeBits > e.le {
			continue
		}
		// Subnet check: route must be contained in the entry prefix.
		if !e.prefix.Contains(route.Addr()) {
			continue
		}
		// Additionally, the route's prefix length must be at least as long as
		// the entry prefix length (Contains alone allows equal-or-more-specific
		// addresses but does not enforce that the route is itself a subnet).
		if routeBits < uint8(e.prefix.Bits()) {
			continue
		}
		return e.action
	}
	return actionReject
}
