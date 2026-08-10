// Design: docs/architecture/core-design.md — withdrawal tracking for route server
// Overview: server.go — route server plugin orchestration
// Related: server_forward.go — forward target selection and batching

package rs

import (
	bgptypes "github.com/ze-software/ze/internal/component/bgp/types"
	"github.com/ze-software/ze/internal/core/family"
)

// processForward handles a forwarding work item in a worker goroutine.
// Reads source peer and payload directly from the work item (no sync.Map lookup).
//
// Extract-then-forward design (buffer lifetime safety):
//  1. Extract families and compact NLRI records BEFORE forwarding (wire buffer
//     may be freed by cache eviction after ForwardCached).
//  2. Forward the UPDATE via batchForwardUpdate.
//  3. Update the withdrawal map AFTER forwarding using the pre-extracted records,
//     keeping per-prefix string-keyed map maintenance off the forward critical path.
func (rs *routeServer) processForward(key workerKey, item workItem) {
	// Guard: release cache entry on any early return or panic.
	// forwardUpdate handles the entry when reached (forward or release),
	// so the flag prevents double-release on the normal path.
	forwarded := false
	defer func() {
		if !forwarded {
			rs.releaseCache(item.msgID)
		}
	}()

	// If the source peer is DOWN, skip withdrawal map update and forward --
	// handleStateDown will withdraw all its routes anyway. This prevents PeerDown
	// from blocking while workers process queued UPDATEs for a peer that is
	// already gone.
	//
	// "Down" is StateSeen && !Up, not the bare !Up this used to test. A peer whose
	// state event has not arrived yet is also !Up, and discarding its UPDATE lost
	// the route for good: no rail replays it (dispatchStructured has already
	// advanced seenMsgID past the message, so handleStateUp's cut excludes it
	// too). The engine relays a message only for a session it considers
	// established, so an UPDATE from a not-yet-up source is a real route on a
	// healthy session and is forwarded. Target selection is unaffected either
	// way: selectForwardTargets requires Up of the TARGETS, never of the source.
	rs.mu.RLock()
	peer := rs.peers[item.sourcePeer]
	peerDown := peer != nil && peer.StateSeen && !peer.Up
	known := peer != nil
	rs.mu.RUnlock()
	if peerDown {
		// A guard that neither denies nor speaks does not exist
		// (ai/rules/evidence.md). This arm drops the whole UPDATE --
		// every withdrawal and every announcement in it -- and no rail replays
		// it later, so the silence here hid permanent route loss.
		logger().Warn("forward dropped: source peer is down",
			"source-peer", item.sourcePeer, "msg-id", item.msgID, "peer-known", known)
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

	// Reactor fast path: when ReactorForwarded is set, the reactor already
	// forwarded this UPDATE to eligible peers. Skip ForwardCached but still
	// update the withdrawal map for peer-down correctness.
	// If FastPathSkipped is non-empty, forward to only those peers.
	forwarded = true
	reactorHandled := item.msg != nil && item.msg.ReactorForwarded
	switch {
	case !reactorHandled:
		rs.batchForwardUpdate(key, item.sourcePeer, item.msgID, families)
	case len(item.msg.FastPathSkipped) > 0:
		rs.batchForwardUpdateSkipped(key, item.sourcePeer, item.msgID, families, item.msg.FastPathSkipped)
	default:
		rs.releaseCache(item.msgID)
	}

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

// isUnicast returns true for IPv4/IPv6 unicast families where NLRIIterator
// prefix bytes can be converted to netip.Prefix directly (zero-alloc path).
func isUnicast(f family.Family) bool {
	return f == (family.IPv4Unicast) || f == (family.IPv6Unicast)
}

// updateWithdrawalMapText updates the withdrawal map from text-parsed NLRI operations.
// Caller must hold rs.withdrawalMu.
func (rs *routeServer) updateWithdrawalMapText(sourcePeer string, ops map[string][]FamilyOperation) {
	for famName, familyOps := range ops {
		fam, _ := family.LookupFamily(famName)
		for _, op := range familyOps {
			switch op.Action {
			case actionAdd:
				if rs.withdrawals[sourcePeer] == nil {
					rs.withdrawals[sourcePeer] = make(map[withdrawalKey]struct{})
				}
				for _, n := range op.NLRIs {
					if s, ok := n.(string); ok && s != "" {
						wk := withdrawalKey{fam: fam, nlriStr: s}
						rs.withdrawals[sourcePeer][wk] = struct{}{}
					}
				}
			case actionDel:
				if rs.withdrawals[sourcePeer] != nil {
					for _, n := range op.NLRIs {
						if s, ok := n.(string); ok && s != "" {
							delete(rs.withdrawals[sourcePeer], withdrawalKey{fam: fam, nlriStr: s})
						}
					}
				}
			}
		}
	}
}
