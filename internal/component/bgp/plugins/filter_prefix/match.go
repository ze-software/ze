// Design: docs/architecture/core-design.md -- prefix-list filter matching
//
// Package filter_prefix implements bgp-filter-prefix.
//
// The matching algorithm walks an ordered list of prefixEntry values.
// For each route prefix, the first entry whose CIDR contains the route
// AND whose ge/le range contains the route's prefix length is selected;
// its action is applied. No-match yields actionReject (implicit deny).
//
// Whole-update strict mode: an UPDATE is accepted only if every prefix in
// its NLRI list is accepted by the prefix-list. Any denied prefix rejects
// the entire UPDATE. This v1 semantics avoids text-protocol NLRI rewrites;
// per-prefix filtering can be added later via the modify action.
package filter_prefix

import (
	"net/netip"
	"strings"
)

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

// evaluateUpdate walks the prefixes in an nlri text field and returns true
// (accept the whole UPDATE) only if every prefix is accepted by the list.
// The nlri field is the text after the "nlri " keyword in the filter update
// text format, e.g. "ipv4/unicast add 10.0.0.0/24 10.0.1.0/24".
//
// Strict mode: any denied prefix rejects the whole UPDATE. An empty nlri
// field is accepted (no prefixes to evaluate).
func (l *prefixList) evaluateUpdate(nlriField string) bool {
	if nlriField == "" {
		return true
	}
	tokens := strings.Fields(nlriField)
	// First two tokens are <family> <op>. Remaining tokens are prefixes.
	if len(tokens) < 2 {
		return true
	}
	for _, tok := range tokens[2:] {
		route, err := netip.ParsePrefix(tok)
		if err != nil {
			// Malformed prefix in the text protocol -- fail-closed.
			return false
		}
		if evaluatePrefix(l.entries, route) != actionAccept {
			return false
		}
	}
	return true
}

// extractNLRIField pulls the text after "nlri " out of an update text string.
// Returns "" if no nlri field is present.
//
// The filter update text format places "nlri" followed by family, op, and
// prefixes; nlri is the LAST field, so everything from "nlri " to end of
// string is the value.
func extractNLRIField(updateText string) string {
	_, after, ok := strings.Cut(updateText, "nlri ")
	if !ok {
		return ""
	}
	return after
}
