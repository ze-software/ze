// Design: docs/architecture/bgp/filter-path-asn.md -- the reject-asn filter plugin
// Detail: config.go -- the position vocabulary and the parse that expands it
// Related: filter_path_asn.go -- SDK entry point and handleFilterUpdate
// Related: internal/component/bgp/filtertext -- the reader of the filter text format
//
// The AS_PATH scan and the position judgement.
//
// The subject is the space-separated decimal string filtertext.ASPath reads out
// of the update text. (*attribute.ASPath).AppendText
// (internal/core/bgp/attribute/text_append.go) flattens AS_SEQUENCE, AS_SET,
// AS_CONFED_SEQUENCE and AS_CONFED_SET into it with no marker, so one flat scan
// sees a listed ASN wherever it sits.
//
// A token holds a SET of properties, not one label, and a list rejects the route
// when any property it lists holds.
//
// Three of them PARTITION the path, so each token holds exactly one:
//
//   - direct: inside the leading run of tokens equal to the SENDING peer's ASN,
//     prepends collapsed. Not index zero: a path [3356 65001] learned from
//     AS65001 carries 3356 at index zero and is a leak, which is the
//     route-server case RFC 7454 Section 9 names and the case this filter exists
//     to catch.
//   - origin: the last token, unless the leading run reaches it. A lone ASN sent
//     by the peer that owns it is direct rather than the origin.
//   - transit: every token between the two.
//
// The fourth CUTS ACROSS that partition, which is why a token cannot be reduced
// to one label:
//
//   - nth n: the token sits at collapsed position n, counted from us, 1-based.
//     In [P A O] the second ASN is both `nth 2` and transit; in [P A] it is both
//     `nth 2` and origin.
//
// `direct` is RELATIONAL and `nth 1` is POSITIONAL. Both are wanted: in
// [3356 65001] from AS65001, 3356 is at `nth 1` and is NOT direct.
//
// The sender is unknown on export, where FilterUpdateInput.PeerAS carries the
// DESTINATION peer rather than whoever sent the route. The leading run is then
// empty and `indirect` covers the whole path. That asymmetry is read off the
// seam once, in handleFilterUpdate, so this file never sees a direction.
//
// The scan allocates nothing. It walks the subject with strings.IndexByte and
// parses each decimal off a slice of it, so a list that names no pattern costs
// no garbage for each UPDATE (TestScanAllocatesNothing). A list that names one
// pays RE2's matching cost for it, which is linear in the subject.
package filter_path_asn

import (
	"regexp"
	"strconv"
	"strings"
)

// senderASN is the ASN whose leading run of an AS_PATH is the `direct`
// position.
//
// known is false on export, where the filter input carries the destination peer
// and nothing says who sent the route. The flag is explicit rather than a zero
// ASN, because AS0 is a value a peer can put in a path and a zero that behaves
// like "no sender" is the accidental guard ai/rules/principles.md forbids.
type senderASN struct {
	asn   uint32
	known bool
}

// hit is what a scan found: the listed ASN and the property that caught it. It
// is what a reject log line names, so an operator reading the log sees both the
// ASN to argue about and the rule that caught it.
//
// at is the primitive position the token occupies for a partition match, and
// positionNth for an nth match. index carries the collapsed position for an nth
// match and is zero otherwise, because 1 is the first position an nth rule can
// name and a zero therefore cannot be read as one.
type hit struct {
	asn   uint32
	at    position
	index uint8
}

// matchPositions reports the first listed ASN that asPath carries at a position
// the list rejects it at, walking the path left to right.
//
// asPath is the flattened space-separated decimal string. An empty path carries
// no token, so no position can match it and a locally originated route is
// accepted with no case written for it (AC-19).
//
// Each token is tested against both property families: the partition it occupies
// and, when the list names any nth rule, the collapsed position it sits at.
func matchPositions(asPath string, sender senderASN, positions map[uint32]positionSet, nth map[nthKey]struct{}) (hit, bool) {
	if len(positions) == 0 && len(nth) == 0 {
		return hit{}, false
	}

	tokens, direct := pathShape(asPath, sender)

	// collapsed counts RUNS rather than tokens, so a repeated ASN advances it
	// once. prevASN and prevKnown carry the previous token for that comparison.
	collapsed := 0
	var prevASN uint32
	prevKnown := false

	index := 0
	for off := 0; ; index++ {
		token, next, ok := nextToken(asPath, off)
		if !ok {
			return hit{}, false
		}
		off = next

		asn, ok := parseASN(token)
		if !ok || !prevKnown || asn != prevASN {
			collapsed++
		}
		prevASN, prevKnown = asn, ok

		if !ok {
			continue
		}

		at := positionAt(index, tokens, direct)
		if positions[asn].holds(at) {
			return hit{asn: asn, at: at}, true
		}
		if len(nth) == 0 || collapsed > nthIndexMax {
			continue
		}
		if _, listed := nth[nthKey{index: uint8(collapsed), asn: asn}]; listed {
			return hit{asn: asn, at: positionNth, index: uint8(collapsed)}, true
		}
	}
}

// matchPattern reports the first pattern of the list that matches the whole
// AS_PATH string.
//
// The subject is the same flattened string every other reader of the format
// sees, so a pattern reaches an AS_SET member and no bracket appears in it
// (AC-49).
func matchPattern(asPath string, patterns []*regexp.Regexp) (*regexp.Regexp, bool) {
	for _, pattern := range patterns {
		if pattern.MatchString(asPath) {
			return pattern, true
		}
	}
	return nil, false
}

// pathShape counts the tokens of asPath and the leading run of them that equal
// the sender's ASN. Prepends collapse: a run of four is four tokens at one
// direct position, not four positions.
//
// The run is empty when the sender is unknown, which is every export call.
func pathShape(asPath string, sender senderASN) (tokens, direct int) {
	inLead := sender.known

	for off := 0; ; {
		token, next, ok := nextToken(asPath, off)
		if !ok {
			return tokens, direct
		}
		off = next
		tokens++

		if !inLead {
			continue
		}
		asn, ok := parseASN(token)
		if !ok || asn != sender.asn {
			inLead = false
			continue
		}
		direct++
	}
}

// positionAt names the partition position of the token at index, in a path of
// tokens whose first direct tokens are the sender's leading run.
//
// Exactly one of direct, transit and origin holds for any token, which is what
// lets this answer one value. The `nth` property is judged separately, because
// it cuts across this partition rather than joining it.
//
// The leading run wins over the origin, which is what makes a lone ASN sent by
// the peer that owns it direct rather than the origin (AC-12).
func positionAt(index, tokens, direct int) position {
	if index < direct {
		return positionDirect
	}
	if index == tokens-1 {
		return positionOrigin
	}
	return positionTransit
}

// nextToken returns the token of asPath at or after off, and the offset just
// past it. ok is false once off has passed the last token.
//
// It skips a run of spaces, so a subject with a doubled or a trailing separator
// yields the tokens a single-spaced one yields. It allocates nothing: the token
// is a slice of asPath.
//
// next is strictly greater than off whenever ok is true, because the spaces are
// skipped before the separator is looked for, so a token is never empty. That is
// what bounds every loop driven by this function at the length of asPath.
func nextToken(asPath string, off int) (token string, next int, ok bool) {
	for off < len(asPath) && asPath[off] == ' ' {
		off++
	}
	if off >= len(asPath) {
		return "", off, false
	}

	end := strings.IndexByte(asPath[off:], ' ')
	if end < 0 {
		return asPath[off:], len(asPath), true
	}
	return asPath[off : off+end], off + end, true
}

// parseASN reads one decimal ASN off a slice of the path.
//
// A token that is not a decimal in uint32 range answers false rather than a
// value, so a malformed subject is a non-match rather than a panic or an
// accidental zero. The producer writes decimals only, so this guards a state
// only a Ze defect or a changed producer reaches, and the peer that chooses the
// ASNs cannot reach it.
//
// It allocates nothing on the path a well-formed subject takes: strconv.ParseUint
// needs no buffer and this never takes its error value.
func parseASN(token string) (uint32, bool) {
	asn, err := strconv.ParseUint(token, 10, 32)
	if err != nil {
		return 0, false
	}
	return uint32(asn), true
}
