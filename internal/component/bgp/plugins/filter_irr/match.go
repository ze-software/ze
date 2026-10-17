// Design: docs/architecture/bgp/filter-irr.md -- prefix matching for IRR-derived filter lists

package filter_irr

import (
	"net/netip"
	"slices"
	"strings"

	"github.com/ze-software/ze/internal/component/bgp/filtertext"
	"github.com/ze-software/ze/internal/core/family"
)

type prefixEntry struct {
	prefix netip.Prefix
	ge     uint8
	le     uint8
}

type irrPrefixList struct {
	entries []prefixEntry
}

func evaluatePrefix(entries []prefixEntry, route netip.Prefix) bool {
	routeBits := uint8(route.Bits())
	routeIs4 := route.Addr().Is4()

	for i := range entries {
		e := &entries[i]
		if e.prefix.Addr().Is4() != routeIs4 {
			continue
		}
		if routeBits < e.ge || routeBits > e.le {
			continue
		}
		if !e.prefix.Contains(route.Addr()) {
			continue
		}
		if routeBits < uint8(e.prefix.Bits()) {
			continue
		}
		return true
	}
	return false
}

func (l *irrPrefixList) evaluateUpdate(nlriField string) bool {
	if nlriField == "" {
		return true
	}
	tokens := strings.Fields(nlriField)
	if len(tokens) < 2 {
		return true
	}
	for _, tok := range tokens[2:] {
		route, err := netip.ParsePrefix(tok)
		if err != nil {
			return false
		}
		if !evaluatePrefix(l.entries, route) {
			return false
		}
	}
	return true
}

func findMatchingEntry(entries []prefixEntry, route netip.Prefix) string {
	routeBits := uint8(route.Bits())
	routeIs4 := route.Addr().Is4()

	for i := range entries {
		e := &entries[i]
		if e.prefix.Addr().Is4() != routeIs4 {
			continue
		}
		if routeBits < e.ge || routeBits > e.le {
			continue
		}
		if !e.prefix.Contains(route.Addr()) {
			continue
		}
		if routeBits < uint8(e.prefix.Bits()) {
			continue
		}
		return e.prefix.String()
	}
	return ""
}

// extractNLRIField returns the NLRI value of an update text: everything from
// its first "nlri" or "mp" keyword, keyword included, or "" when it has none.
// The reactor renders the NLRI blocks last (filter_format.go).
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

// nlriOpWithdraw is the operation token of a withdrawn NLRI block in the filter
// text (reactor filter_format.go). Every other operation announces.
const nlriOpWithdraw = "del"

// legacyFamily is the family of every "nlri" block: the legacy NLRI and
// Withdrawn Routes are IPv4 unicast (RFC 4271). nlriOpAnnounce on an "nlri"
// block names the legacy NLRI section. The engine rewrites that section and
// the mp block of a CIDR family from a modify answer (reactor
// filter_nlri_delta.go), and rewritable below admits exactly those.
const (
	legacyFamily   = "ipv4/unicast"
	nlriOpAnnounce = "add"
)

// nlriKeyword opens a legacy block, "nlri add|del <prefix>...", and mpKeyword
// a block rendered from an MP_REACH_NLRI or MP_UNREACH_NLRI attribute,
// "mp <family> add|del <prefix>...", so IPv4 unicast carried in MP_REACH_NLRI is
// told apart from the legacy NLRI field (reactor filter_format.go).
const (
	nlriKeyword = "nlri"
	mpKeyword   = "mp"
)

// isNLRIKeyword reports whether token opens an NLRI block.
func isNLRIKeyword(token string) bool {
	return token == nlriKeyword || token == mpKeyword
}

// nlriBlock is one "nlri <op> <prefix>..." or "mp <family> <op> <prefix>..."
// block of the filter text; family is legacyFamily on an "nlri" block.
// tokens and prefixes hold the same prefixes, as written and parsed. admitted
// marks, for an announce block, which prefixes the IRR list authorizes; a
// withdraw block is never judged, so its admitted is nil.
type nlriBlock struct {
	mp       bool // rendered from MP_REACH_NLRI / MP_UNREACH_NLRI ("mp ...")
	family   string
	op       string
	tokens   []string
	prefixes []netip.Prefix
	admitted []bool
}

func (b *nlriBlock) withdraws() bool {
	return b.op == nlriOpWithdraw
}

// partitionResult carries the per-prefix classification of one UPDATE, block by
// block. The reactor renders the legacy NLRI and Withdrawn Routes sections and
// each MP_REACH_NLRI and MP_UNREACH_NLRI attribute as a block of its own
// (filter_format.go), so one UPDATE can carry several.
//
// accepted and rejected hold the announced prefix tokens the list does and does
// not authorize, across every announce block: they decide the verdict. A
// withdrawn prefix is in neither, because withdrawing a route the list never
// authorized removes nothing it admitted. hadParseError is set when any prefix
// token of any block failed to parse, so callers fail closed.
type partitionResult struct {
	blocks        []nlriBlock
	accepted      []string
	rejected      []string
	hadParseError bool
}

// partitionUpdate walks every block of an nlri text field and classifies each
// announced prefix as accepted (in the IRR list) or rejected. Unlike
// evaluateUpdate, which short-circuits on the first rejection, it consumes the
// whole field so the modify path can emit only the accepted subset.
//
// The field is the NLRI value, keyword first: "nlri add|del <prefix>..." and
// "mp <family> add|del <prefix>..." blocks in any number. A block whose head is
// malformed is a parse error, so callers fail closed. The loop is bounded by
// the token count of the field.
func (l *irrPrefixList) partitionUpdate(nlriField string) partitionResult {
	var out partitionResult
	tokens := strings.Fields(nlriField)
	for start := 0; start < len(tokens); {
		end := start + 1
		for end < len(tokens) && !isNLRIKeyword(tokens[end]) {
			end++
		}
		mp := tokens[start] == mpKeyword
		head, family := start+1, legacyFamily
		if mp {
			head++
		}
		switch {
		case !isNLRIKeyword(tokens[start]):
			out.hadParseError = true // the field does not open with a block
		case end <= head:
			out.hadParseError = true // the block ends before its op token
		case mp:
			out.addBlock(l, true, tokens[start+1], tokens[head], tokens[head+1:end])
		default:
			out.addBlock(l, false, family, tokens[head], tokens[head+1:end])
		}
		start = end
	}
	return out
}

// addBlock parses one block's prefix tokens and classifies them.
func (out *partitionResult) addBlock(l *irrPrefixList, mp bool, family, op string, tokens []string) {
	block := nlriBlock{mp: mp, family: family, op: op, tokens: tokens, prefixes: make([]netip.Prefix, 0, len(tokens))}
	if !block.withdraws() {
		block.admitted = make([]bool, 0, len(tokens))
	}
	for _, tok := range tokens {
		route, err := netip.ParsePrefix(tok)
		if err != nil {
			out.hadParseError = true
			return
		}
		block.prefixes = append(block.prefixes, route)
		if block.withdraws() {
			continue
		}
		admitted := evaluatePrefix(l.entries, route)
		block.admitted = append(block.admitted, admitted)
		if admitted {
			out.accepted = append(out.accepted, tok)
		} else {
			out.rejected = append(out.rejected, tok)
		}
	}
	out.blocks = append(out.blocks, block)
}

// rewritable reports whether a modify delta can drop every rejected prefix of
// the UPDATE. The engine rewrites the legacy announce block and the mp block of
// a CIDR family (filtertext.CIDRFamily: unicast, multicast and mpls-label, whose
// label stacks it keeps), and refuses a changed block of any other family
// (reactor filter_nlri_delta.go, sectionNLRIDelta). An UPDATE with a rejected
// prefix in any other block, such as a FlowSpec destination, or in a block whose
// family this process does not know, is rejected whole: fail closed.
func (p *partitionResult) rewritable() bool {
	for i := range p.blocks {
		block := &p.blocks[i]
		if block.withdraws() {
			continue
		}
		if !slices.Contains(block.admitted, false) {
			continue
		}
		if !block.mp && block.op == nlriOpAnnounce {
			continue
		}
		if !block.mp {
			return false
		}
		fam, known := family.LookupFamily(block.family)
		if !known {
			return false
		}
		if !filtertext.CIDRFamily(fam) {
			return false
		}
	}
	return true
}

// buildModifyDelta renders the modify delta returned to the engine when some
// announced prefixes pass and some don't. It carries only the rewritten nlri
// field, so the engine leaves every other attribute untouched. Every block is
// rebuilt in order: an announce block keeps its accepted prefixes and is left
// out when it held prefixes and the list admits none, and a withdraw block is
// kept whole. A block with no prefix at all, the marker the engine renders for
// a non-CIDR family (reactor filter_format.go, appendMPBlock), is echoed
// unchanged: left out it reads as a changed block of a family the engine cannot
// rewrite, which drops the whole UPDATE, legitimate prefixes included.
func buildModifyDelta(partition partitionResult) string {
	size := 0
	for i := range partition.blocks {
		block := &partition.blocks[i]
		size += len(nlriKeyword) + len(block.family) + len(block.op) + 3
		for _, tok := range block.tokens {
			size += len(tok) + 1
		}
	}
	text := make([]byte, 0, size)
	for i := range partition.blocks {
		block := &partition.blocks[i]
		if !block.withdraws() && len(block.tokens) > 0 && !slices.Contains(block.admitted, true) {
			continue
		}
		if len(text) > 0 {
			text = append(text, ' ')
		}
		if block.mp {
			text = append(text, mpKeyword...)
			text = append(text, ' ')
			text = append(text, block.family...)
		} else {
			text = append(text, nlriKeyword...)
		}
		text = append(text, ' ')
		text = append(text, block.op...)
		for j, tok := range block.tokens {
			if !block.withdraws() && !block.admitted[j] {
				continue
			}
			text = append(text, ' ')
			text = append(text, tok...)
		}
	}
	return string(text)
}
