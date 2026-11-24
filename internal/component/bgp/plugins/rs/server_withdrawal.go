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
	if item.barrier != nil {
		rs.flushWorkerBatch(key)
		rs.releaseParkedValidations(key)
		close(item.barrier)
		return
	}
	if item.validation.Prefix.IsValid() || item.validation.NLRI != "" {
		rs.processValidation(key, item.validation)
		return
	}
	// Deferred first, so it runs last: a FlowSpec relay parked for this UPDATE
	// leaves after the UPDATE is batched (parkValidation).
	defer rs.finishSourceUpdate(key)
	defer rs.markProcessed(key, item.msgID)
	// Guard: release cache entry on any early return or panic.
	// forwardUpdate handles the entry when reached (forward or release),
	// so the flag prevents double-release on the normal path.
	forwarded := false
	defer func() {
		if !forwarded {
			rs.releaseCache(item.msgID)
		}
	}()

	rs.mu.RLock()
	nonClient := rs.peers[key].nonRSClient()
	rs.mu.RUnlock()
	if nonClient {
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
	var wireRecords *nlriRecords
	var recordsErr error
	noLLGR := false
	if item.msg != nil {
		wireRecords, recordsErr = extractWireNLRIRecords(item.msg)
		noLLGR = wireNoLLGR(item.msg.AttrsWire)
	}

	// Both transports carry the same owned delivery receipt. The reactor
	// already served the decided targets; this rail carries only handed-back
	// and later-established targets under the ordinary peer-up replay cut.
	// The withdrawal map is still updated for peer-down correctness.
	forwarded = true
	if item.reactorForwarded {
		rs.batchForwardUpdateSkipped(key, item.sourcePeer, item.msgID, families, item.fastPathSkipped)
	} else {
		rs.batchForwardUpdate(key, item.sourcePeer, item.msgID, families)
	}

	// Update withdrawal map AFTER forwarding using pre-extracted data.
	// String keys are produced here, off the forward critical path.
	// A wire UPDATE whose records could not be read has no text to fall back
	// on: the inventory misses its routes, so the refusal is logged with the
	// message it lost, and the text path is never run over an empty payload.
	if recordsErr != nil {
		logger().Error("route server inventory not updated: NLRI records not extracted",
			"error", recordsErr, "source-peer", item.sourcePeer, "message-id", item.msgID)
		return
	}
	rs.withdrawalMu.Lock()
	if wireRecords != nil {
		rs.applyNLRIRecords(item.sourcePeer, wireRecords)
	} else {
		noLLGR = textNoLLGR(item.textPayload)
		rs.updateWithdrawalMapText(item.sourcePeer, parseTextNLRIOps(item.textPayload), item.sourcePolicyEpoch, item.sourceReceipt, item.sourceOwner, key.connectionID)
	}
	rs.stampCurrentRowsLocked(key, item.msgID, item.sourceReceipt, noLLGR)
	// Current source progress is recorded only after every parsed route mutation,
	// including withdrawals. Another source's larger MessageID proves nothing.
	rs.mu.RLock()
	if actor := rs.peers[key]; actor != nil {
		actor.inventoryRevision++
		actor.processedMsgID = max(actor.processedMsgID, item.msgID)
	}
	rs.mu.RUnlock()
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
func (rs *routeServer) updateWithdrawalMapText(sourcePeer string, ops map[string][]FamilyOperation, epoch, receipt, owner, connection uint64) {
	for famName, operations := range ops {
		fam, known := family.LookupFamily(famName)
		if !known {
			continue
		}
		for _, operation := range operations {
			for _, value := range operation.NLRIs {
				text, ok := value.(string)
				if !ok {
					continue
				}
				native, entry, err := textInventoryEntry(fam, text, operation.PathID, operation.AddPath, operation.Action == actionDel, epoch, receipt, owner, connection)
				if err != nil {
					rs.warnLifecycle("original text NLRI inventory refused", "code", lifecycleDoctorCode, "fix", lifecycleDoctorFix, "error", err)
					continue
				}
				if operation.Action == actionAdd {
					if rs.withdrawals[sourcePeer] == nil {
						rs.withdrawals[sourcePeer] = make(map[withdrawalKey]withdrawalEntry)
					}
					rs.withdrawals[sourcePeer][native] = entry
				} else if old, exists := rs.withdrawals[sourcePeer][native]; exists && old.connectionID == connection {
					delete(rs.withdrawals[sourcePeer], native)
				}
			}
		}
	}
}
