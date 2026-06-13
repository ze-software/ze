// Design: plan/spec-filter-irr.md -- prefix matching for IRR-derived filter lists

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
