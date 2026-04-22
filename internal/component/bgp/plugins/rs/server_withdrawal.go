// Design: docs/architecture/core-design.md — withdrawal tracking for route server
// Overview: server.go — route server plugin orchestration
// Related: server_forward.go — forward target selection and batching

package rs

import (
	"net/netip"
	"strings"

	bgpctx "codeberg.org/thomas-mangin/ze/internal/component/bgp/context"
	"codeberg.org/thomas-mangin/ze/internal/component/bgp/nlri"
	bgptypes "codeberg.org/thomas-mangin/ze/internal/component/bgp/types"
	"codeberg.org/thomas-mangin/ze/internal/core/family"
)

// nlriKey extracts the compact routing key from an NLRI string.
// Strips the "prefix " type keyword since it is redundant within a family.
// Other NLRI types (VPN, BGP-LS, EVPN) use the full string as key.
func nlriKey(nlri string) string {
	if after, ok := strings.CutPrefix(nlri, "prefix "); ok {
		return after
	}
	return nlri
}

// processForward handles a forwarding work item in a worker goroutine.
// Reads source peer and payload directly from the work item (no sync.Map lookup).
//
// Extract-then-forward design (buffer lifetime safety):
//  1. Extract families and compact NLRI records BEFORE forwarding (wire buffer
//     may be freed by cache eviction after ForwardCached).
//  2. Forward the UPDATE via batchForwardUpdate.
//  3. Update the withdrawal map AFTER forwarding using the pre-extracted records,
//     keeping per-prefix string-keyed map maintenance off the forward critical path.
func (rs *RouteServer) processForward(key workerKey, item workItem) {
	// Guard: release cache entry on any early return or panic.
	// forwardUpdate handles the entry when reached (forward or release),
	// so the flag prevents double-release on the normal path.
	forwarded := false
	defer func() {
		if !forwarded {
			rs.releaseCache(item.msgID)
		}
	}()

	// If the source peer is down, skip withdrawal map update and forward -- handleStateDown
	// will withdraw all routes. This prevents PeerDown from blocking while
	// workers process queued UPDATEs for a peer that is already gone.
	rs.mu.RLock()
	peer := rs.peers[item.sourcePeer]
	peerDown := peer == nil || !peer.Up
	rs.mu.RUnlock()
	if peerDown {
		return
	}

	// Extract families for forward target selection.
	// Structured path (DirectBridge): read directly from wire, no text parsing.
	// Text path (fork-mode): parse from text payload.
	var families map[family.Family]bool
	if item.msg != nil {
		families = extractWireFamilies(item.msg)
	} else {
		families = parseTextUpdateFamilies(item.textPayload)
	}
	if len(families) == 0 {
		return
	}

	// Extract compact NLRI records BEFORE forwarding. For wire UPDATEs, uses
	// netip.PrefixFrom (zero string allocation per prefix). String keys are
	// deferred to the withdrawal map update after forwarding.
	var wireRecords *[]nlriRecord
	if item.msg != nil {
		wireRecords = extractWireNLRIRecords(item.msg)
	}

	// Forward first -- minimizes UPDATE delivery latency.
	forwarded = true
	rs.batchForwardUpdate(key, item.sourcePeer, item.msgID, families)

	// Update withdrawal map AFTER forwarding using pre-extracted data.
	// String keys are produced here, off the forward critical path.
	rs.withdrawalMu.Lock()
	if wireRecords != nil {
		rs.applyNLRIRecords(item.sourcePeer, *wireRecords)
	} else {
		rs.updateWithdrawalMapText(item.sourcePeer, parseTextNLRIOps(item.textPayload))
	}
	rs.withdrawalMu.Unlock()

	returnNLRIRecords(wireRecords)
}

// extractWireFamilies extracts address families from a raw UPDATE message.
// Uses MPReachWire.Family() and MPUnreachWire.Family() (3-byte reads each),
// and checks for IPv4 body NLRIs. No NLRI parsing needed.
func extractWireFamilies(msg *bgptypes.RawMessage) map[family.Family]bool {
	families := make(map[family.Family]bool, 2)
	wu := msg.WireUpdate
	if wu == nil {
		return families
	}

	if mp, err := wu.MPReach(); err == nil && mp != nil {
		families[mp.Family()] = true
	}
	if mp, err := wu.MPUnreach(); err == nil && mp != nil {
		families[mp.Family()] = true
	}
	// Check IPv4 body NLRIs (only present for IPv4 unicast).
	if body, err := wu.NLRI(); err == nil && len(body) > 0 {
		families[family.IPv4Unicast] = true
	}
	if wd, err := wu.Withdrawn(); err == nil && len(wd) > 0 {
		families[family.IPv4Unicast] = true
	}

	return families
}

// updateWithdrawalMapWire updates the withdrawal map from raw wire UPDATE data.
// Uses NLRIIterator for zero-allocation NLRI walking on IPv4/IPv6 unicast.
// Falls back to NLRIs() (allocating) for non-unicast families to produce correct text keys.
// Caller must hold rs.withdrawalMu.
func (rs *RouteServer) updateWithdrawalMapWire(sourcePeer string, msg *bgptypes.RawMessage) {
	if msg.WireUpdate == nil {
		return
	}
	wu := msg.WireUpdate

	// Get encoding context for add-path detection.
	var encCtx *bgpctx.EncodingContext
	if msg.AttrsWire != nil {
		encCtx = bgpctx.Registry.Get(msg.AttrsWire.SourceContext())
	}

	// MP_REACH_NLRI — announced routes (add).
	if mp, err := wu.MPReach(); err == nil && mp != nil {
		fam := mp.Family()
		addPath := encCtx != nil && encCtx.AddPath(fam)
		if isUnicast(fam) {
			if iter := mp.NLRIIterator(addPath); iter != nil {
				rs.walkUnicastNLRIs(sourcePeer, fam.String(), iter, actionAdd)
			}
		} else {
			nlris, nlriErr := mp.NLRIs(addPath)
			rs.walkNLRIsAllocating(sourcePeer, fam, nlris, nlriErr)
		}
	}

	// MP_UNREACH_NLRI — withdrawn routes (del).
	if mp, err := wu.MPUnreach(); err == nil && mp != nil {
		fam := mp.Family()
		addPath := encCtx != nil && encCtx.AddPath(fam)
		if isUnicast(fam) {
			if iter := mp.NLRIIterator(addPath); iter != nil {
				rs.walkUnicastNLRIs(sourcePeer, fam.String(), iter, actionDel)
			}
		} else {
			nlris, nlriErr := mp.NLRIs(addPath)
			rs.walkUnreachNLRIsAllocating(sourcePeer, fam, nlris, nlriErr)
		}
	}

	// IPv4 body NLRIs — announced routes (add).
	addPathV4 := encCtx != nil && encCtx.AddPath(family.IPv4Unicast)
	if iter, err := wu.NLRIIterator(addPathV4); err == nil && iter != nil {
		rs.walkUnicastNLRIs(sourcePeer, "ipv4/unicast", iter, actionAdd)
	}

	// IPv4 body Withdrawn — withdrawn routes (del).
	if iter, err := wu.WithdrawnIterator(addPathV4); err == nil && iter != nil {
		rs.walkUnicastNLRIs(sourcePeer, "ipv4/unicast", iter, actionDel)
	}
}

// isUnicast returns true for IPv4/IPv6 unicast families where NLRIIterator
// prefix bytes can be converted to netip.Prefix directly (zero-alloc path).
func isUnicast(f family.Family) bool {
	return f == (family.IPv4Unicast) || f == (family.IPv6Unicast)
}

// walkUnicastNLRIs walks NLRIs via iterator and updates the withdrawal map.
// Converts raw prefix bytes to netip.Prefix for route key — zero allocation per NLRI.
// Only valid for IPv4/IPv6 unicast families.
func (rs *RouteServer) walkUnicastNLRIs(sourcePeer, famName string, iter *nlri.NLRIIterator, action string) {
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
			if rs.withdrawals[sourcePeer] == nil {
				rs.withdrawals[sourcePeer] = make(map[string]withdrawalInfo)
			}
			rs.withdrawals[sourcePeer][routeKey] = withdrawalInfo{Family: famName, Prefix: "prefix " + key}
		case actionDel:
			if rs.withdrawals[sourcePeer] != nil {
				delete(rs.withdrawals[sourcePeer], routeKey)
			}
		}
	}
}

// prefixBytesToKey converts raw NLRI prefix bytes from NLRIIterator to a route key string.
// Input: [bitLen, addr_bytes...] from NLRIIterator.Next().
// Returns netip.Prefix.String() (e.g., "10.0.0.0/24", "2001:db8::/32").
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

// walkNLRIsAllocating updates the withdrawal map using parsed NLRI objects (add action).
// Used for non-unicast families where raw prefix bytes need family-specific decoding.
// Allocates via NLRIs() — acceptable for rare non-unicast route server traffic.
func (rs *RouteServer) walkNLRIsAllocating(sourcePeer string, fam family.Family, nlris []nlri.NLRI, err error) {
	if err != nil || len(nlris) == 0 {
		return
	}
	familyStr := fam.String()
	if rs.withdrawals[sourcePeer] == nil {
		rs.withdrawals[sourcePeer] = make(map[string]withdrawalInfo)
	}
	for _, n := range nlris {
		s := n.String()
		routeKey := familyStr + "|" + nlriKey(s)
		rs.withdrawals[sourcePeer][routeKey] = withdrawalInfo{Family: familyStr, Prefix: s}
	}
}

// walkUnreachNLRIsAllocating updates the withdrawal map using parsed NLRI objects (del action).
// Used for non-unicast MP_UNREACH_NLRI families.
func (rs *RouteServer) walkUnreachNLRIsAllocating(sourcePeer string, fam family.Family, nlris []nlri.NLRI, err error) {
	if err != nil || len(nlris) == 0 {
		return
	}
	familyStr := fam.String()
	if rs.withdrawals[sourcePeer] != nil {
		for _, n := range nlris {
			delete(rs.withdrawals[sourcePeer], familyStr+"|"+nlriKey(n.String()))
		}
	}
}

// updateWithdrawalMapText updates the withdrawal map from text-parsed NLRI operations.
// Caller must hold rs.withdrawalMu.
func (rs *RouteServer) updateWithdrawalMapText(sourcePeer string, ops map[string][]FamilyOperation) {
	for famName, familyOps := range ops {
		for _, op := range familyOps {
			switch op.Action {
			case actionAdd:
				if rs.withdrawals[sourcePeer] == nil {
					rs.withdrawals[sourcePeer] = make(map[string]withdrawalInfo)
				}
				for _, n := range op.NLRIs {
					if s, ok := n.(string); ok && s != "" {
						routeKey := famName + "|" + nlriKey(s)
						rs.withdrawals[sourcePeer][routeKey] = withdrawalInfo{Family: famName, Prefix: s}
					}
				}
			case actionDel:
				if rs.withdrawals[sourcePeer] != nil {
					for _, n := range op.NLRIs {
						if s, ok := n.(string); ok && s != "" {
							delete(rs.withdrawals[sourcePeer], famName+"|"+nlriKey(s))
						}
					}
				}
			}
		}
	}
}
