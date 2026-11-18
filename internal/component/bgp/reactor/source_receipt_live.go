// Design: docs/architecture/api/process-protocol.md -- live replay uses existing receipt joins.
// Related: source_receipt.go -- lease admission, per-write guard and completion.
package reactor

import (
	"errors"
	"github.com/ze-software/ze/internal/component/plugin"
	"github.com/ze-software/ze/internal/core/events"
)

// beginLive enrolls one live-only legacy write. It preserves that rail's actual
// send authority without granting retention. The existing lease map lets grant,
// process and source cleanup revoke and join it. A transient lease leaves when
// its last actual write settles; explicit Acquire upgrades it while delivery
// remains open. No per-route or completed-session history is created.
func (store *sourceReceiptStore) beginLive(id uint64, owner plugin.SelectedOwner, epoch uint64) (sourceReceiptWrite, error) {
	// Resolve once without retaining this raw pointer across the lock change.
	store.mu.Lock()
	original := store.entries[id]
	if original == nil {
		store.mu.Unlock()
		return sourceReceiptWrite{}, errors.New("source receipt: live original is unknown or released")
	}
	peer := original.facts.source.info.peer
	store.mu.Unlock()
	// Lock order is Peer then receipt store, as source settings publication does.
	// Neither lock survives a final write or join.
	peer.mu.RLock()
	defer peer.mu.RUnlock()
	if !liveSourceReceiveGranted(peer, owner.Sender()) {
		return sourceReceiptWrite{}, errors.New("live source no longer grants this process received UPDATEs; reconcile its original attachment")
	}
	store.mu.Lock()
	defer store.mu.Unlock()
	if err := owner.Current(); err != nil {
		return sourceReceiptWrite{}, err
	}
	record := store.entries[id]
	if record != original {
		return sourceReceiptWrite{}, errors.New("source receipt: original delivery changed before admission; reconcile the received snapshot")
	}
	source := &record.facts.source
	if store.grants[sourceGrantKey{peer: source.info.peer, owner: owner}] != nil {
		return sourceReceiptWrite{}, errors.New("source receipt: original source grant transaction is still open")
	}
	if !record.deliveryOpen {
		return sourceReceiptWrite{}, errForwardNoSource
	}
	if !forwardSourceCurrent(source.info.peer, source.generation) {
		return sourceReceiptWrite{}, errForwardNoSource
	}
	if !sourcePolicyCurrent(source.info.peer, epoch) {
		return sourceReceiptWrite{}, errAdjOutPolicy
	}
	lease := record.leases[owner]
	created := false
	if lease == nil {
		lease = newSourceReceiptLease(store, record, owner)
		lease.transient = true
		record.leases[owner] = lease
		created = true
	}
	write, err := lease.beginLocked(epoch)
	if err != nil {
		if created {
			delete(record.leases, owner)
			store.dropUnused(record)
		}
		return sourceReceiptWrite{}, err
	}
	write.live = true
	return write, nil
}

// liveSourceReceiveGranted reads the same exact configured UPDATE receive word
// as event delivery. The caller MUST hold peer.mu; the authenticated Sender was
// captured from the actual Process before this call. No alias grants authority.
func liveSourceReceiveGranted(peer *Peer, sender plugin.Sender) bool {
	name, named := sender.Process()
	if !named {
		return false
	}
	for _, binding := range peer.settings.ProcessBindings {
		if binding.PluginName != name {
			continue
		}
		if binding.ReceiveAll {
			return true
		}
		direction := binding.Receive["update"]
		if direction == events.DirReceived {
			return true
		}
		if direction == events.DirBoth {
			return true
		}
	}
	return false
}

// invalidateLivePeer cancels only currently referenced original sessions whose
// captured generation has ended. Retained leases remain independent. The scan
// is bounded by current delivery/lease/write references; no history is added.
func (store *sourceReceiptStore) invalidateLivePeer(peer *Peer) {
	store.mu.Lock()
	defer store.mu.Unlock()
	for _, record := range store.entries {
		source := &record.facts.source
		if source.info.peer != peer {
			continue
		}
		if forwardSourceCurrent(peer, source.generation) {
			continue
		}
		record.endLive()
	}
}
