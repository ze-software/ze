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
