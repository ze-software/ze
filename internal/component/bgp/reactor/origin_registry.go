// Design: docs/architecture/api/process-protocol.md -- exact accepted-entry retirement.
// Related: origin_runtime.go -- reoffers preserve lineage; origin_receipt.go -- issuer ownership.
package reactor

import (
	"context"
	"errors"

	bgptypes "github.com/ze-software/ze/internal/component/bgp/types"
	"github.com/ze-software/ze/internal/component/plugin"
	"github.com/ze-software/ze/pkg/plugin/rpc"
)

func (record *originReceiptRecord) matchesRetirement(retirement *plugin.OriginRetirement) bool {
	return retirement.MatchesOrigin(record.key.producer.sender(), record.key.producer.owner,
		record.build.family.String(), record.build.native, record.key.peerOwner, record.lineage)
}

// retireTrackedOriginBatch precedes any provider lookup, optional-capability
// fallback or direct write. The original accepted Peer scopes are authoritative;
// an absent/superseded lineage completes without touching a newer contribution.
// Legacy direct scopes retain their existing wire path.
func (a *reactorAPIAdapter) retireTrackedOriginBatch(ctx context.Context, batch bgptypes.NLRIBatch, sender plugin.Sender, peers []*Peer, retirement *plugin.OriginRetirement, output *rpc.UpdateRouteOutput) ([]*Peer, error) {
	var direct []*Peer
	handled := false
	for _, peer := range peers {
		if !retirement.HasPeer(sender, peer.sourceOwner) {
			if retirement.HasDestination(sender, peer.addrString) {
				handled = true
				continue
			}
			direct = append(direct, peer)
			continue
		}
		handled = true
		store := a.r.originReceipts
		if store == nil {
			return nil, errors.New("origin registry retirement has no initialized issuer")
		}
		records, err := store.retireTrackedLineage(ctx, peer, retirement)
		if err != nil {
			return nil, err
		}
		if len(records) == 0 {
			// Absence never sends a guessed withdrawal to a same-name current
			// provider. Preserve the ordinary unavailable-owner refusal, while
			// the newer unrelated contribution remains entirely untouched.
			peer.mu.RLock()
			configured := peer.settings.DisseminationOwner
			peer.mu.RUnlock()
			if configured.IsSet() && !configured.Explicit() {
				if a.r.api == nil {
					return nil, errors.New("origin registry retirement has no deciding-process connection")
				}
				if _, _, err := a.r.api.OriginIntakeOwner(configured.Name()); err != nil {
					return nil, err
				}
			}
		}
		var failures []error
		for _, record := range records {
			// The captured original connection is used even after its name has
			// a successor. A stopped connection is an explicit remote failure;
			// local current data has already gone and cannot replay later.
			if err := a.retireOriginCandidate(ctx, record); err != nil {
				failures = append(failures, err)
			}
		}
		if err := errors.Join(failures...); err != nil {
			return nil, err
		}
	}
	if handled {
		output.Withdrawn = uint32(len(batch.NLRIs)) //nolint:gosec // command-bounded native input
	}
	return direct, nil
}

// retireTrackedLineage MUST seal only the accepted original, including actual
// reoffers/callbacks/writes of that lineage. A fresh same-key intake has a new
// lineage, or has not issued one yet, and is never sealed by this old entry.
// All waits and provider I/O run after releasing the issuer leaf mutex.
func (store *originReceiptStore) retireTrackedLineage(ctx context.Context, peer *Peer, retirement *plugin.OriginRetirement) ([]*originReceiptRecord, error) {
	store.mu.Lock()
	if err := store.live(); err != nil {
		store.mu.Unlock()
		return nil, err
	}
	var records []*originReceiptRecord
	var pending []<-chan struct{}
	for intake := range store.intakes {
		if intake.peer != peer {
			continue
		}
		if intake.original == nil {
			continue
		}
		if intake.original.matchesRetirement(retirement) {
			intake.revoked = true
		}
	}
	for _, record := range store.current {
		if record.peer != peer {
			continue
		}
		if !record.matchesRetirement(retirement) {
			continue
		}
		records = append(records, record)
		store.invalidate(record)
		pending = append(pending, record.idle)
	}
	for _, record := range store.receipts {
		if record.peer != peer {
			continue
		}
		if !record.matchesRetirement(retirement) {
			continue
		}
		store.invalidate(record)
		pending = append(pending, record.idle)
	}
	for call := range store.calls {
		if call.scope.peer != peer {
			continue
		}
		// Call facts are captured from the same immutable record at enrollment.
		// The record may already be parked outside the receipt index.
		if !call.original.matchesRetirement(retirement) {
			continue
		}
		call.cancel()
		pending = append(pending, call.done)
	}
	store.mu.Unlock()
	for _, done := range pending {
		if err := waitSourceReceiptWrites(ctx, done); err != nil {
			return records, err
		}
	}
	return records, nil
}
