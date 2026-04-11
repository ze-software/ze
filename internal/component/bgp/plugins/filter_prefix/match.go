// Design: docs/architecture/core-design.md -- prefix-list filter matching
// Related: config.go -- prefix-list config parsing
// Related: filter_prefix.go -- SDK entry point and handleFilterUpdate
//
// Package filter_prefix implements bgp-filter-prefix.
//
// The matching algorithm walks an ordered list of prefixEntry values.
// For each route prefix, the first entry whose CIDR contains the route
// AND whose ge/le range contains the route's prefix length is selected;
// its action is applied. No-match yields actionReject (implicit deny).
//
// Two evaluation modes are exposed:
//   - evaluateUpdate: strict whole-update decision. Returns accept only if
//     every prefix in the NLRI passes; any denied prefix rejects the whole
//     UPDATE. Used by the accept/reject branches.
//   - partitionUpdate: per-prefix partition. Walks every prefix in the NLRI
//     and separates accepted from rejected, preserving order. The modify
//     path uses this to emit the accepted subset as a new NLRI block so
//     multi-prefix UPDATEs can have part of their NLRI denied without
//     rejecting the whole message.
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
// field is accepted (no prefixes to evaluate). Retained for callers that
// only need the whole-update accept/reject decision; partitionUpdate below
// is used by the modify path.
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

// partitionResult carries the outcome of the per-prefix partition walk used
// by the modify path. accepted holds the prefix text tokens that passed the
// prefix-list evaluation; rejected holds the ones that did not; family and op
// echo the NLRI block header so callers can rebuild "nlri <family> <op>
// <accepted>...". hadParseError is set when any prefix token failed
// ParsePrefix; the strict evaluator is fail-closed, so callers should reject
// the whole update in that case.
type partitionResult struct {
	family        string
	op            string
	accepted      []string
	rejected      []string
	hadParseError bool
}

// partitionUpdate walks every prefix in an nlri text field, classifies it as
// accepted or rejected by the prefix-list, and returns the partition. Unlike
// evaluateUpdate which short-circuits on the first rejection, partitionUpdate
// always consumes the whole list so the modify path can emit the accepted
// subset. An empty nlri or a nlri without prefix tokens produces an empty
// partition with family/op still populated when present.
//
// The family and op parsed here are purely echoed back -- the classifier does
// not gate on them. cmd-4 rewrites only the IPv4 unicast legacy NLRI section;
// that family filter happens at the engine boundary, not inside the match
// algorithm.
func (l *prefixList) partitionUpdate(nlriField string) partitionResult {
	var out partitionResult
	if nlriField == "" {
		return out
	}
	tokens := strings.Fields(nlriField)
	if len(tokens) < 2 {
		return out
	}
	out.family = tokens[0]
	out.op = tokens[1]
	for _, tok := range tokens[2:] {
		route, err := netip.ParsePrefix(tok)
		if err != nil {
			out.hadParseError = true
			// Keep walking: the caller decides whether to reject the whole
			// update on parse failure; we still want a complete picture for
			// telemetry and for callers that choose to treat parse errors as
			// non-fatal in non-strict mode.
			continue
		}
		if evaluatePrefix(l.entries, route) == actionAccept {
			out.accepted = append(out.accepted, tok)
		} else {
			out.rejected = append(out.rejected, tok)
		}
	}
	return out
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
