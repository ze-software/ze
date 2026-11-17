// Design: docs/architecture/api/process-protocol.md -- generation-specific intake barriers.
// Related: origin_write.go -- lifecycle revocation seals scope-level active intakes.
package reactor

import (
	"context"
	"errors"

	"github.com/ze-software/ze/internal/component/plugin"
)

// retireOriginReceipt revokes only a committed issued generation and joins its
// genuine outstanding writes. Pending issuance has no receipt yet, so it cannot
// belong to this generation. Lifecycle removal still uses scope-level revoke,
// which seals preflighted intake calls as well as committed generations.
func (store *originReceiptStore) retireOriginReceipt(ctx context.Context, id uint64) error {
	if store == nil {
		return errors.New("origin issuer is missing")
	}
	if id == 0 {
		return errors.New("origin receipt generation is missing")
	}
	store.mu.Lock()
	if store.lifetime == nil {
		store.mu.Unlock()
		return errors.New("origin issuer is uninitialized")
	}
	record := store.receipts[id]
	var idle <-chan struct{}
	if record != nil {
		store.invalidate(record)
		idle = record.idle
	}
	store.mu.Unlock()
	return waitSourceReceiptWrites(ctx, idle)
}

// joinOriginPredecessors settles only committed older generations of the exact
// source/key/Peer contribution. It neither seals a later pending replacement
// nor changes a newer committed contribution. No identifier history is retained.
func (store *originReceiptStore) joinOriginPredecessors(ctx context.Context, key originContributionKey, before uint64) error {
	if store == nil {
		return errors.New("origin issuer is missing")
	}
	store.mu.Lock()
	if store.lifetime == nil {
		store.mu.Unlock()
		return errors.New("origin issuer is uninitialized")
	}
	var pending []<-chan struct{}
	for _, record := range store.receipts {
		if record.key != key || record.id >= before {
			continue
		}
		store.invalidate(record)
		pending = append(pending, record.idle)
	}
	store.mu.Unlock()
	for _, idle := range pending {
		if err := waitSourceReceiptWrites(ctx, idle); err != nil {
			return err
		}
	}
	return nil
}

// validateOriginIntakeReceipt refuses a superseded or revoked callback receipt
// on both sides of the remote mutation. It does not derive new authority from
// an acknowledgment. The deciding plugin must also serialize candidate keys
// and refuse an older issued generation that arrives after its replacement.
func (store *originReceiptStore) validateOriginIntakeReceipt(id uint64, peer *Peer, owner plugin.SelectedOwner) error {
	if store == nil {
		return errors.New("origin issuer is missing")
	}
	if peer == nil {
		return errors.New("origin intake concrete peer is missing")
	}
	store.mu.Lock()
	defer store.mu.Unlock()
	if err := store.live(); err != nil {
		return err
	}
	record := store.receipts[id]
	if record == nil {
		return errors.New("origin intake receipt was superseded or released")
	}
	if record.revoked || store.current[record.key] != record {
		return errors.New("origin intake receipt is no longer current")
	}
	if record.peer != peer || !record.provider.SameIncarnation(owner) {
		return errors.New("origin intake receipt belongs to another scope or process")
	}
	if peer.stopping.Load() {
		return errors.New("origin intake original peer has stopped")
	}
	if err := record.key.producer.current(); err != nil {
		return err
	}
	return owner.Current()
}
