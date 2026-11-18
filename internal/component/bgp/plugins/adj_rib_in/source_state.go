// Design: docs/architecture/plugin/rib-storage-design.md -- source-policy original owners.
// Related: source_policy.go -- owner-scoped ledgers and denial bounds.
// Related: rib.go -- captured typed and JSON state producers.
package adj_rib_in

import (
	"github.com/ze-software/ze/pkg/plugin/rpc"
	"net/netip"
)

// sourceLive is the captured current physical connection for one original
// owner. Its zero value is unrecorded and is never installed by a state event.
// AdjRIBInManager.mu protects every read and write.
type sourceLive struct {
	address        netip.Addr
	connectionID   uint64
	processedMsgID uint64 // Whole original UPDATE effects committed, never a global ingest maximum.
}

// recordSourceStateLocked applies captured physical state. Address-only legacy
// DOWN clears only unowned observations; it cannot clear an authenticated
// source, authorize policy, or identify a physical connection.
// A newer UP preserves received bytes with their original connection facts;
// delayed DOWN from the previous connection cannot retire the current owner.
// Caller MUST hold mu and finish with unlockValidation.
func (r *AdjRIBInManager) recordSourceStateLocked(address netip.Addr, up bool, owner, connectionID uint64) bool {
	if owner == 0 {
		if up {
			r.peerUp[address] = true
			return true
		}
		r.removeObservedRoutesLocked(address)
		r.peerUp[address] = false
		for _, live := range r.sourceLive {
			if live.address == address {
				r.peerUp[address] = true
				break
			}
		}
		return true
	}
	if connectionID == 0 {
		return true
	}
	if r.sourceLive == nil {
		r.sourceLive = make(map[uint64]sourceLive)
	}
	current, exists := r.sourceLive[owner]
	if up {
		if exists {
			if connectionID < current.connectionID {
				return false
			}
			if connectionID == current.connectionID {
				return current.address == address
			}
		}
		if err := r.retentionVerdicts.Begin(owner, connectionID); err != nil {
			return false
		}
		r.sourceLive[owner] = sourceLive{address: address, connectionID: connectionID}
		r.peerUp[address] = true
		return true
	}
	if !exists {
		return false
	}
	if current.address != address {
		return false
	}
	if current.connectionID != connectionID {
		return false
	}
	retained, err := r.retentionVerdicts.Take(owner, connectionID)
	if err != nil {
		return false
	}
	r.retireValidationConnectionLocked(receivedPeer{address: address, owner: owner}, connectionID, retained)
	delete(r.sourceLive, owner)
	r.pruneValidationMetadataLocked(receivedPeer{address: address, owner: owner})
	r.peerUp[address] = false
	for _, live := range r.sourceLive {
		if live.address == address {
			r.peerUp[address] = true
			break
		}
	}
	return true
}

// removeSourceInstalledLocked retires only the captured original owner. No
// address aggregate is used by the lifecycle path. Caller MUST hold mu and
// finish with unlockValidation, after any pending-removal notification.
func (r *AdjRIBInManager) removeSourceInstalledLocked(address netip.Addr, owner uint64) {
	source := receivedPeer{address: address, owner: owner}
	if owner != 0 {
		if err := r.retentionVerdicts.End(owner); err != nil {
			return
		}
	}
	r.removePeerInstalled(source)
	r.clearPeerPending(source)
}

// removeObservedRoutesLocked clears the separate observation store. Missing
// provenance cannot name an authenticated sibling. Caller MUST hold mu and
// finish with unlockValidation.
func (r *AdjRIBInManager) removeObservedRoutesLocked(address netip.Addr) {
	observed := receivedPeer{address: address}
	r.removePeerInstalled(observed)
	r.clearPeerPending(observed)
}

// noteSourceProcessed publishes only after this whole UPDATE committed. Both
// actual received callbacks run synchronously in ordered SDK delivery and hold
// mutationMu until this returns. It never creates an owner or advances another
// physical connection. Empty and withdrawal-only UPDATEs count as processed.
func (r *AdjRIBInManager) noteSourceProcessed(address netip.Addr, owner, connection, msgID uint64) {
	r.mu.Lock()
	defer r.mu.Unlock()
	current, present := r.sourceLive[owner]
	if !present {
		return
	}
	if current.address != address {
		return
	}
	if current.connectionID != connection {
		return
	}
	if msgID <= current.processedMsgID {
		return
	}
	current.processedMsgID = msgID
	r.sourceLive[owner] = current
}

// endSourceOwner handles only ReasonPeerRemoved's original nonzero owner.
// It joins escaped self replay, then releases every current or retained physical
// value belonging to that owner. Zero is refused, never an address wildcard.
// No completed-owner record remains; a duplicate terminal event is harmless.
func (r *AdjRIBInManager) endSourceOwner(owner uint64) {
	if owner == 0 {
		return
	}
	r.replayGuard.Lock()
	defer r.replayGuard.Unlock()
	if err := r.admitMutation(false); err != nil {
		return
	}
	defer r.mutationMu.Unlock()
	r.mu.Lock()
	defer r.unlockValidation()
	if err := r.retentionVerdicts.End(owner); err != nil {
		panic("BUG: nonzero source owner refused terminal cleanup")
	}
	for source := range r.ribIn {
		if source.owner != owner {
			continue
		}
		r.removePeerInstalled(source)
	}
	for key := range r.pending {
		if key.Peer.owner != owner {
			continue
		}
		r.removePending(key.Peer, key.Route)
	}
	for key := range r.earlyDecisions {
		if key.Peer.owner != owner {
			continue
		}
		delete(r.earlyDecisions, key)
	}
	for key := range r.validationPeers {
		if key.source.owner != owner {
			continue
		}
		delete(r.validationPeers, key)
	}
	r.dropSourcePolicyLocked(owner)
	current, live := r.sourceLive[owner]
	if !live {
		return
	}
	delete(r.sourceLive, owner)
	r.peerUp[current.address] = false
	for _, sibling := range r.sourceLive {
		if sibling.address == current.address {
			r.peerUp[current.address] = true
			break
		}
	}
}

// sourceDecisionAdmittedLocked permits a live source's pre-UPDATE decision or
// an exact generation of bytes this store actually still holds, including GR
// retention. Missing source state cannot recreate early debt after cleanup.
// Observational owner zero remains separate from authenticated owners.
// Caller MUST hold mu; this predicate creates no state and grants no sending.
func (r *AdjRIBInManager) sourceDecisionAdmittedLocked(source receivedPeer, key compactRouteKey, decision *rpc.ValidationDecision) bool {
	if source.owner == 0 {
		return true
	}
	if live, known := r.sourceLive[source.owner]; known {
		if live.address == source.address {
			if live.connectionID == decision.ConnectionID {
				return true
			}
		}
	}
	route := r.validationStoredRouteLocked(source, key)
	if route == nil {
		return false
	}
	if route.ConnectionID != decision.ConnectionID {
		return false
	}
	if decision.MsgID != 0 {
		if route.MsgID != decision.MsgID {
			return false
		}
	}
	return true
}
