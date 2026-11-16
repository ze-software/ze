// Design: docs/architecture/api/process-protocol.md -- retained selected-source authority.
// Related: source_receipt.go -- existing lease and final guard lifetime.
package reactor

import (
	"context"
	"errors"

	"github.com/ze-software/ze/internal/component/plugin"
)

// admitWrite uses an existing authenticated lease; an issued identifier alone
// MUST NOT acquire one. The lease cannot be replaced between lookup and begin,
// because begin checks that same immutable lease's revocation under store.mu.
func (store *sourceReceiptStore) admitWrite(id uint64, owner plugin.SelectedOwner, epoch uint64) (sourceReceiptWrite, error) {
	store.mu.Lock()
	record := store.entries[id]
	if record == nil {
		store.mu.Unlock()
		return sourceReceiptWrite{}, errors.New("source receipt: source session is unknown or released")
	}
	lease := record.leases[owner]
	store.mu.Unlock()
	if lease == nil {
		return sourceReceiptWrite{}, errors.New("source receipt: selecting owner has no admitted lease")
	}
	return lease.begin(epoch)
}

// writeContext cancels a blocked final flush when this admitted lease is
// revoked. The caller MUST invoke cleanup after write/flush has joined, before
// completing the guard. The lease context is allocated once per lease.
func (write sourceReceiptWrite) writeContext(parent context.Context) (context.Context, func()) {
	ctx, cancel := context.WithCancel(parent)
	stop := context.AfterFunc(write.lease.lifetime, func() {
		cancel()
	})
	return ctx, func() {
		// This callback only cancels this private context. The actual socket
		// deadline callback is joined by requestWriteDeadline before unlocking;
		// a late cancel cannot touch a later request's context or socket.
		stop()
		cancel()
	}
}
