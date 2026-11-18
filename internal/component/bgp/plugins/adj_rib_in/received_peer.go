// Design: docs/architecture/plugin/rib-storage-design.md -- original-owner received stores.
// Related: rib_validation.go -- current physical and original ingress ordering.
package adj_rib_in

import "net/netip"

// receivedPeer qualifies an outer store with its original stable owner. The
// native per-route key stays 48 bytes. owner zero names observations only:
// it never matches an owned store or supplies policy/eligibility authority.
// Its zero value has no address and is not installed by a parsed event.
type receivedPeer struct {
	address netip.Addr
	owner   uint64
}

// receivedPeerOf keeps missing provenance explicitly observational. It never
// fills the owner or connection from a current address lookup. Physical facts
// remain on current RawRoute values, not an unbounded connection-history map.
func receivedPeerOf(address netip.Addr, owner, connection uint64) receivedPeer {
	if owner == 0 {
		return receivedPeer{address: address}
	}
	if connection == 0 {
		return receivedPeer{address: address}
	}
	return receivedPeer{address: address, owner: owner}
}

// receivedBefore orders original physical birth before original ingress ID.
// Zero ingress keeps its explicitly unversioned meaning within one connection.
func receivedBefore(connection, message, nextConnection, nextMessage uint64) bool {
	if connection != nextConnection {
		return connection < nextConnection
	}
	if message == 0 {
		return false
	}
	if nextMessage == 0 {
		return false
	}
	return message < nextMessage
}

// receiveCurrentLocked refuses an old physical/ingress replacement while
// preserving useful current bytes. Caller MUST hold mu through publication.
func (r *AdjRIBInManager) receiveCurrentLocked(peer receivedPeer, key compactRouteKey, connection, message uint64) bool {
	current := r.validationStoredRouteLocked(peer, key)
	if current == nil {
		return true
	}
	return !receivedBefore(connection, message, current.ConnectionID, current.MsgID)
}

// removeReceivedLocked applies the original physical connection's withdrawal.
// Caller MUST hold mu and finish with unlockValidation, publishing before return.
func (r *AdjRIBInManager) removeReceivedLocked(peer receivedPeer, key compactRouteKey, connection, message uint64) {
	current := r.validationStoredRouteLocked(peer, key)
	if current != nil {
		if current.ConnectionID != connection {
			return
		}
		if receivedBefore(connection, message, current.ConnectionID, current.MsgID) {
			return
		}
	}
	if early := r.earlyDecisions[pendingKey(peer, key)]; early != nil {
		if early.connectionID != connection {
			return
		}
		if receivedBefore(connection, message, early.connectionID, early.msgID) {
			return
		}
	}
	r.removePending(peer, key)
	r.removeInstalled(peer, key)
}
