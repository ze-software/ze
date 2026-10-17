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

// evaluateUpdate returns true (accept the whole UPDATE) only if every prefix of
// every NLRI block is accepted by the list. The nlri field starts at the first
// "nlri" or "mp" keyword of the filter update text, keyword included, e.g.
// "nlri add 10.0.0.0/24 mp ipv6/unicast add 2001:db8::/32".
//
// Strict mode: any denied prefix, or any token that is not a prefix, rejects
// the whole UPDATE. An empty nlri field is accepted (no prefixes to evaluate).
// partitionUpdate below is the per-prefix form the modify path uses.
func (l *prefixList) evaluateUpdate(nlriField string) bool {
	p := l.partitionUpdate(nlriField)
	if p.hadParseError {
		return false
	}
	return len(p.rejected) == 0
}

// The keywords that open an NLRI block of the filter text (reactor
// filter_format.go): "nlri add|del <prefix>..." for the legacy NLRI and
// Withdrawn Routes, always IPv4 unicast, and "mp <family> add|del <prefix>..."
// for an MP_REACH_NLRI or MP_UNREACH_NLRI attribute.
const (
	nlriKeyword = "nlri"
	mpKeyword   = "mp"
	// legacyFamily is the family of every "nlri" block.
	legacyFamily = "ipv4/unicast"
)

// prefixBlock is one NLRI block of the filter text, split into its head
// ("nlri <op>" or "mp <family> <op>") and its prefixes classified by the list.
type prefixBlock struct {
	head     []string
	family   string
	op       string
	accepted []string
	rejected []string
}

// partitionResult carries the outcome of the per-prefix partition walk used
// by the modify path, block by block. One UPDATE renders a block per section:
// the legacy NLRI and Withdrawn Routes, and each MP_REACH_NLRI and
// MP_UNREACH_NLRI attribute. accepted and rejected hold the prefix tokens
// across every block. hadParseError is set when any prefix token failed
// ParsePrefix; the strict evaluator is fail-closed, so callers reject the
// whole update in that case.
type partitionResult struct {
	blocks        []prefixBlock
	accepted      []string
	rejected      []string
	hadParseError bool
}

// partitionUpdate walks every prefix of every NLRI block, classifies it as
// accepted or rejected by the prefix-list, and returns the partition. Unlike
// a strict walk it consumes the whole field, so the modify path can emit the
// accepted subset of each block, in the order it was given.
//
// The engine rewrites each section from its own block and refuses an answer
// that changes a block of a family it cannot rewrite, so the classifier does
// not gate on the family. The loop is bounded by the token count of the field.
func (l *prefixList) partitionUpdate(nlriField string) partitionResult {
	var out partitionResult
	tokens := strings.Fields(nlriField)
	for start := 0; start < len(tokens); {
		end := start + 1
		for end < len(tokens) && !isNLRIKeyword(tokens[end]) {
			end++
		}
		out.addBlock(l, tokens[start:end])
		start = end
	}
	return out
}

// isNLRIKeyword reports whether token opens an NLRI block.
func isNLRIKeyword(token string) bool {
	return token == nlriKeyword || token == mpKeyword
}

// addBlock classifies the prefixes of one block's tokens, keyword first. A
// block with a malformed head is a parse error: the caller fails closed.
func (out *partitionResult) addBlock(l *prefixList, tokens []string) {
	headLen, fam := 2, legacyFamily
	if tokens[0] == mpKeyword {
		headLen = 3
	}
	if len(tokens) < headLen {
		out.hadParseError = true
		return
	}
	if !isNLRIKeyword(tokens[0]) {
		out.hadParseError = true
		return
	}
	if headLen == 3 {
		fam = tokens[1]
	}
	block := prefixBlock{head: tokens[:headLen], family: fam, op: tokens[headLen-1]}
	for _, tok := range tokens[headLen:] {
		route, err := netip.ParsePrefix(tok)
		if err != nil {
			out.hadParseError = true
			// Keep walking: the caller rejects the whole update on a parse
			// failure, and the complete picture still serves telemetry.
			continue
		}
		if evaluatePrefix(l.entries, route) == actionAccept {
			block.accepted = append(block.accepted, tok)
			out.accepted = append(out.accepted, tok)
		} else {
			block.rejected = append(block.rejected, tok)
			out.rejected = append(out.rejected, tok)
		}
	}
	out.blocks = append(out.blocks, block)
}

// extractNLRIField returns the NLRI value of an update text: everything from
// its first "nlri" or "mp" keyword, keyword included, or "" when it has none.
//
// The filter update text format places the NLRI blocks LAST, so everything
// from the first keyword to the end of the string is the value.
func extractNLRIField(updateText string) string {
	for i := 0; i < len(updateText); {
		for i < len(updateText) && updateText[i] == ' ' {
			i++
		}
		end := strings.IndexByte(updateText[i:], ' ')
		if end < 0 {
			end = len(updateText) - i
		}
		if isNLRIKeyword(updateText[i : i+end]) {
			return updateText[i:]
		}
		i += end
	}
	return ""
}
