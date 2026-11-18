// Design: docs/architecture/plugin/rib-storage-design.md -- permanent destination retirement.
// Related: rib_peer_name.go -- exact configured Peer removal, not physical DOWN.
package rib

// retireSentDestinationLocked ends only this configured destination's history.
// The caller MUST hold peerMu and invoke it only for permanent Peer removal.
// Physical DOWN MUST retain the same owner's inventory for reconnect replay.
// This owns no I/O: the removed peer's engine lifecycle already retires its
// connection. Its finite SENT maps and currently open transactions bound work.
func (r *RIBManager) retireSentDestinationLocked(owner uint64) {
	for _, routes := range r.ribOut[owner] {
		for _, entry := range routes {
			entry.release()
		}
	}
	delete(r.ribOut, owner)
	for _, transaction := range r.sourcePolicy {
		transaction.retireSentDestinationLocked(owner)
	}
}

// retireSentDestinationLocked releases this transaction's undo copies for one
// permanently removed destination. The manager MUST call it under peerMu
// before forgetting that destination's metadata. Abort MUST restore only the
// surviving pins; it must never recreate a removed owner's SENT inventory.
func (txn *sourcePolicyTxn) retireSentDestinationLocked(owner uint64) {
	kept := txn.sent[:0]
	for i := range txn.sent {
		sent := txn.sent[i]
		if sent.destination == owner {
			sent.entry.release()
			continue
		}
		kept = append(kept, sent)
	}
	clear(txn.sent[len(kept):])
	txn.sent = kept
}
