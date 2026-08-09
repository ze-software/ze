// Design: docs/architecture/bgp/filter-irr.md -- prefix matching for IRR-derived filter lists

package filter_irr

import (
	"net/netip"
	"strings"
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

func extractNLRIField(updateText string) string {
	_, after, ok := strings.Cut(updateText, "nlri ")
	if !ok {
		return ""
	}
	return after
}

// partitionResult carries the per-prefix classification used by the modify
// path: accepted holds prefix tokens that are in the IRR list, rejected holds
// the rest; family and op echo the NLRI block header so callers can rebuild the
// "nlri <family> <op> <accepted>..." block. hadParseError is set when any prefix
// token failed to parse, so callers fail closed (same contract as evaluateUpdate).
type partitionResult struct {
	family        string
	op            string
	accepted      []string
	rejected      []string
	hadParseError bool
}

// partitionUpdate walks every prefix in an nlri text field and classifies each
// as accepted (in the IRR list) or rejected. Unlike evaluateUpdate, which
// short-circuits on the first rejection, it consumes the whole list so the
// modify path can emit only the accepted subset and drop unauthorized prefixes
// without rejecting the legitimate ones in the same UPDATE.
func (l *irrPrefixList) partitionUpdate(nlriField string) partitionResult {
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
			continue
		}
		if evaluatePrefix(l.entries, route) {
			out.accepted = append(out.accepted, tok)
		} else {
			out.rejected = append(out.rejected, tok)
		}
	}
	return out
}

// buildModifyDelta renders the modify delta returned to the engine when some
// prefixes pass and some don't. It carries only the rewritten nlri block (the
// accepted subset) so the engine leaves every other attribute untouched.
func buildModifyDelta(partition partitionResult) string {
	parts := make([]string, 0, 3+len(partition.accepted))
	parts = append(parts, "nlri", partition.family, partition.op)
	parts = append(parts, partition.accepted...)
	return joinWords(parts)
}

func joinWords(parts []string) string {
	if len(parts) == 0 {
		return ""
	}
	n := len(parts) - 1
	for _, p := range parts {
		n += len(p)
	}
	b := make([]byte, 0, n)
	b = append(b, parts[0]...)
	for _, p := range parts[1:] {
		b = append(b, ' ')
		b = append(b, p...)
	}
	return string(b)
}
