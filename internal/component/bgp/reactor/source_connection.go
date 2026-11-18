// Design: docs/architecture/api/process-protocol.md -- original received connection provenance.
package reactor

// originalConnectionID reads the immutable facts of a held source receipt.
// This leaf lock performs no I/O and never resolves a current Peer or Session.
// An unknown receipt/owner pair is explicitly unrecorded and grants nothing.
func (store *sourceReceiptStore) originalConnectionID(receipt, peerOwner uint64) uint64 {
	if receipt == 0 {
		return 0
	}
	if peerOwner == 0 {
		return 0
	}
	store.mu.Lock()
	defer store.mu.Unlock()
	record := store.entries[receipt]
	if record == nil {
		return 0
	}
	if record.facts.peerOwner != peerOwner {
		return 0
	}
	return record.facts.connectionID
}
