// Design: docs/architecture/core-design.md -- withdrawal tracking for route reflector
// Overview: rr.go -- RouteReflector uses withdrawal map for peer-down cleanup
//
// RFC 4456 Section 8: when a client peer goes down, the route reflector
// must withdraw all routes previously advertised from that peer.
// This file tracks announced/withdrawn NLRIs per source peer so that
// handleStateDown can send explicit withdrawals to remaining peers.

package rr

import (
	"net/netip"
	"strings"

	bgpctx "codeberg.org/thomas-mangin/ze/internal/component/bgp/context"
	"codeberg.org/thomas-mangin/ze/internal/component/bgp/nlri"
	"codeberg.org/thomas-mangin/ze/internal/component/bgp/textparse"
	bgptypes "codeberg.org/thomas-mangin/ze/internal/component/bgp/types"
	"codeberg.org/thomas-mangin/ze/internal/core/family"
)

// withdrawalInfo stores the minimum information needed to send a withdrawal
// command when a source peer goes down.
type withdrawalInfo struct {
	Family string
	Prefix string // Full NLRI string including type keyword (e.g., "prefix 10.0.0.0/24").
}

// updateWithdrawalMapWire updates the withdrawal map from raw wire UPDATE data.
// Uses NLRIIterator for zero-allocation NLRI walking on IPv4/IPv6 unicast.
// Falls back to NLRIs() (allocating) for non-unicast families.
// Caller must hold rr.withdrawalMu.
func (rr *RouteReflector) updateWithdrawalMapWire(sourcePeer string, msg *bgptypes.RawMessage) {
	if msg.WireUpdate == nil {
		return
	}
	wu := msg.WireUpdate

	// Get encoding context for add-path detection.
	var encCtx *bgpctx.EncodingContext
	if msg.AttrsWire != nil {
		encCtx = bgpctx.Registry.Get(msg.AttrsWire.SourceContext())
	}

	// MP_REACH_NLRI -- announced routes (add).
	if mp, err := wu.MPReach(); err == nil && mp != nil {
		fam := mp.Family()
		addPath := encCtx != nil && encCtx.AddPath(fam)
		if isUnicast(fam) {
			if iter := mp.NLRIIterator(addPath); iter != nil {
				rr.walkUnicastNLRIs(sourcePeer, fam.String(), iter, actionAdd)
			}
		} else {
			nlris, nlriErr := mp.NLRIs(addPath)
			rr.walkNLRIsAllocating(sourcePeer, fam, nlris, nlriErr, actionAdd)
		}
	}

	// MP_UNREACH_NLRI -- withdrawn routes (del).
	if mp, err := wu.MPUnreach(); err == nil && mp != nil {
		fam := mp.Family()
		addPath := encCtx != nil && encCtx.AddPath(fam)
		if isUnicast(fam) {
			if iter := mp.NLRIIterator(addPath); iter != nil {
				rr.walkUnicastNLRIs(sourcePeer, fam.String(), iter, actionDel)
			}
		} else {
			nlris, nlriErr := mp.NLRIs(addPath)
			rr.walkNLRIsAllocating(sourcePeer, fam, nlris, nlriErr, actionDel)
		}
	}

	// IPv4 body NLRIs -- announced routes (add).
	addPathV4 := encCtx != nil && encCtx.AddPath(family.IPv4Unicast)
	if iter, err := wu.NLRIIterator(addPathV4); err == nil && iter != nil {
		rr.walkUnicastNLRIs(sourcePeer, "ipv4/unicast", iter, actionAdd)
	}

	// IPv4 body Withdrawn -- withdrawn routes (del).
	if iter, err := wu.WithdrawnIterator(addPathV4); err == nil && iter != nil {
		rr.walkUnicastNLRIs(sourcePeer, "ipv4/unicast", iter, actionDel)
	}
}

// isUnicast returns true for IPv4/IPv6 unicast families where NLRIIterator
// prefix bytes can be converted to netip.Prefix directly (zero-alloc path).
func isUnicast(f family.Family) bool {
	return f == family.IPv4Unicast || f == family.IPv6Unicast
}

// walkUnicastNLRIs walks NLRIs via iterator and updates the withdrawal map.
// Converts raw prefix bytes to netip.Prefix for route key -- zero allocation per NLRI.
func (rr *RouteReflector) walkUnicastNLRIs(sourcePeer, famName string, iter *nlri.NLRIIterator, action string) {
	isV6 := strings.HasPrefix(famName, "ipv6/")
	for {
		prefix, _, ok := iter.Next()
		if !ok {
			break
		}
		key := prefixBytesToKey(prefix, isV6)
		if key == "" {
			continue
		}
		routeKey := famName + "|" + key
		switch action {
		case actionAdd:
			if rr.withdrawals[sourcePeer] == nil {
				rr.withdrawals[sourcePeer] = make(map[string]withdrawalInfo)
			}
			rr.withdrawals[sourcePeer][routeKey] = withdrawalInfo{Family: famName, Prefix: "prefix " + key}
		case actionDel:
			if rr.withdrawals[sourcePeer] != nil {
				delete(rr.withdrawals[sourcePeer], routeKey)
			}
		}
	}
}

// prefixBytesToKey converts raw NLRI prefix bytes from NLRIIterator to a route key string.
// Input: [bitLen, addr_bytes...] from NLRIIterator.Next().
func prefixBytesToKey(prefix []byte, isV6 bool) string {
	if len(prefix) == 0 {
		return ""
	}
	bitLen := int(prefix[0])
	addrBytes := prefix[1:]
	if isV6 {
		var addr [16]byte
		copy(addr[:], addrBytes)
		p := netip.PrefixFrom(netip.AddrFrom16(addr), bitLen)
		return p.Masked().String()
	}
	var addr [4]byte
	copy(addr[:], addrBytes)
	p := netip.PrefixFrom(netip.AddrFrom4(addr), bitLen)
	return p.Masked().String()
}

// walkNLRIsAllocating updates the withdrawal map using parsed NLRI objects.
// Used for non-unicast families where raw prefix bytes need family-specific decoding.
func (rr *RouteReflector) walkNLRIsAllocating(sourcePeer string, fam family.Family, nlris []nlri.NLRI, err error, action string) {
	if err != nil || len(nlris) == 0 {
		return
	}
	familyStr := fam.String()
	switch action {
	case actionAdd:
		if rr.withdrawals[sourcePeer] == nil {
			rr.withdrawals[sourcePeer] = make(map[string]withdrawalInfo)
		}
		for _, n := range nlris {
			s := n.String()
			routeKey := familyStr + "|" + nlriKey(s)
			rr.withdrawals[sourcePeer][routeKey] = withdrawalInfo{Family: familyStr, Prefix: s}
		}
	case actionDel:
		if rr.withdrawals[sourcePeer] != nil {
			for _, n := range nlris {
				delete(rr.withdrawals[sourcePeer], familyStr+"|"+nlriKey(n.String()))
			}
		}
	}
}

// nlriKey extracts the compact routing key from an NLRI string.
// Strips the "prefix " type keyword since it is redundant within a family.
func nlriKey(s string) string {
	if after, ok := strings.CutPrefix(s, "prefix "); ok {
		return after
	}
	return s
}

// --- Text-path withdrawal tracking (fork-mode fallback) ---

// familyOperation represents a single add or del operation for a family.
type familyOperation struct {
	action string // "add" or "del"
	nlris  []string
}

// updateWithdrawalMapText updates the withdrawal map from text-parsed NLRI operations.
// Caller must hold rr.withdrawalMu. The incoming map is keyed by family.Family;
// withdrawalInfo.Family still carries the registered name text because it is
// re-emitted verbatim as a dispatched command argument.
func (rr *RouteReflector) updateWithdrawalMapText(sourcePeer string, ops map[family.Family][]familyOperation) {
	for fam, familyOps := range ops {
		famName := fam.String()
		for _, op := range familyOps {
			switch op.action {
			case actionAdd:
				if rr.withdrawals[sourcePeer] == nil {
					rr.withdrawals[sourcePeer] = make(map[string]withdrawalInfo)
				}
				for _, s := range op.nlris {
					if s != "" {
						routeKey := famName + "|" + nlriKey(s)
						rr.withdrawals[sourcePeer][routeKey] = withdrawalInfo{Family: famName, Prefix: s}
					}
				}
			case actionDel:
				if rr.withdrawals[sourcePeer] != nil {
					for _, s := range op.nlris {
						if s != "" {
							delete(rr.withdrawals[sourcePeer], famName+"|"+nlriKey(s))
						}
					}
				}
			}
		}
	}
}

// parseTextNLRIOps extracts family operations (add/del + NLRIs) from a text UPDATE.
//
// Format: "peer <addr> remote as <n> <dir> update <id> <attrs> [next <nh>] nlri <fam> add|del <nlris> ..."
//
// Key-dispatch loop processes keywords sequentially, resolving aliases via textparse.ResolveAlias:
// - Attribute keywords (origin, path, pref, etc.): skip value(s)
// - "nlri": consume family, extract action (add/del) and collect NLRI tokens until next keyword.
// The returned map is keyed by family.Family; unregistered family names are dropped.
func parseTextNLRIOps(text string) map[family.Family][]familyOperation {
	result := make(map[family.Family][]familyOperation)
	s := textparse.NewScanner(strings.TrimRight(text, "\n"))

	// Skip header: peer <addr> remote as <n> <dir> update <id>
	for i := 0; i < 8 && !s.Done(); i++ {
		s.Next()
	}

	// Key-dispatch loop.
	for !s.Done() {
		raw, ok := s.Next()
		if !ok {
			break
		}
		tok := textparse.ResolveAlias(raw)

		switch tok {
		case textparse.KWNextHop:
			s.Next() // consume the address

		case textparse.KWNLRI:
			famTok, ok := s.Next()
			if !ok || !strings.Contains(famTok, "/") {
				continue
			}
			fam, famOK := family.LookupFamily(famTok)
			if !famOK {
				continue
			}

			// Optional path-id modifier.
			next, ok := s.Peek()
			if !ok {
				continue
			}
			if textparse.ResolveAlias(next) == textparse.KWPathInformation {
				s.Next() // consume "info"/"path-information"
				s.Next() // consume the ID value
				if _, ok = s.Peek(); !ok {
					continue
				}
			}

			// Action: add or del.
			action, ok := s.Next()
			if !ok || (action != actionAdd && action != actionDel) {
				continue
			}

			// Collect NLRI tokens until next top-level keyword or end.
			var nlriTokens []string
			for !s.Done() {
				next, ok := s.Peek()
				if !ok || textparse.IsTopLevelKeyword(next) {
					break
				}
				tok, _ := s.Next()
				nlriTokens = append(nlriTokens, tok)
			}

			nlris := buildNLRIEntries(nlriTokens)
			if len(nlris) > 0 {
				result[fam] = append(result[fam], familyOperation{action: action, nlris: nlris})
			}

		// Attribute keywords: consume their values.
		case textparse.KWOrigin, textparse.KWMED, textparse.KWLocalPreference,
			textparse.KWAggregator, textparse.KWOriginatorID:
			s.Next()
		case textparse.KWASPath, textparse.KWCommunity, textparse.KWLargeCommunity,
			textparse.KWExtendedCommunity, textparse.KWClusterList:
			s.Next()
		case textparse.KWAtomicAggregate:
			// flag, no value
		}
	}

	return result
}

// buildNLRIEntries splits collected tokens into individual NLRI strings.
// Accepts two formats:
//   - Comma: "prefix 10.0.0.0/24,10.0.1.0/24" -- type keyword + comma-separated values.
//   - Keyword boundary: "prefix 10.0.0.0/24 prefix 10.0.1.0/24" -- repeated type keyword.
func buildNLRIEntries(tokens []string) []string {
	if len(tokens) == 0 {
		return nil
	}

	// Check for comma in any token.
	for i, tok := range tokens {
		if !strings.Contains(tok, ",") {
			continue
		}
		typePrefix := strings.Join(tokens[:i], " ")
		var nlris []string
		for part := range strings.SplitSeq(tok, ",") {
			part = strings.TrimSpace(part)
			if part != "" {
				if typePrefix != "" {
					nlris = append(nlris, typePrefix+" "+part)
				} else {
					nlris = append(nlris, part)
				}
			}
		}
		return nlris
	}

	// No commas -- check for keyword boundary (repeated type keywords).
	if textparse.NLRITypeKeywords[tokens[0]] {
		var nlris []string
		var current []string
		for _, tok := range tokens {
			if tok == tokens[0] && len(current) > 0 {
				nlris = append(nlris, strings.Join(current, " "))
				current = nil
			}
			current = append(current, tok)
		}
		if len(current) > 0 {
			nlris = append(nlris, strings.Join(current, " "))
		}
		return nlris
	}

	// Single complex NLRI: join all tokens.
	return []string{strings.Join(tokens, " ")}
}
