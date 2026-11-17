// Design: docs/architecture/api/process-protocol.md -- source authority revocation.
// Related: source_receipt.go -- final guards own their completion until flush joins.
package reactor

import (
	"context"
	"fmt"

	"github.com/ze-software/ze/internal/component/plugin"
)

// revokePeers permanently ends the listed exact sources, including old retained
// incarnations. Callers MUST mark/remove those Peers before calling, MUST hold
// no reactor or peer lock, and MUST not report completion until this joins.
func (store *sourceReceiptStore) revokePeers(ctx context.Context, peers ...*Peer) error {
	targets := make(map[*Peer]struct{}, len(peers))
	for _, peer := range peers {
		targets[peer] = struct{}{}
	}
	return store.revokeSources(ctx, func(record *sourceReceiptRecord) bool {
		_, found := targets[record.facts.source.info.peer]
		return found
	}, func(plugin.SelectedOwner) bool { return true }, true)
}

// revokePeerOwner is the cold receive-grant removal barrier. It retires only
// this exact peer/process incarnation, keeping other processes' valid retention.
// The apply coordinator MUST first remove the grant, call outside its locks,
// and join selected dispatch before acknowledging the binding change.
func (store *sourceReceiptStore) revokePeerOwner(ctx context.Context, peer *Peer, owner plugin.SelectedOwner) error {
	return store.revokeSources(ctx, func(record *sourceReceiptRecord) bool {
		return record.facts.source.info.peer == peer
	}, func(candidate plugin.SelectedOwner) bool { return candidate.SameIncarnation(owner) }, false)
}

type sourceLeaseJoin struct {
	peer *Peer
	idle <-chan struct{}
}

// revokeSources scans only currently retained source/lease/write records. Its
// two pure predicates MUST not take locks, perform I/O or retain record aliases.
// All matched rights are revoked before waiting for any one destination write.
func (store *sourceReceiptStore) revokeSources(ctx context.Context, source func(*sourceReceiptRecord) bool, owner func(plugin.SelectedOwner) bool, closeDelivery bool) error {
	store.mu.Lock()
	var joins []sourceLeaseJoin
	for _, record := range store.entries {
		if !source(record) {
			continue
		}
		if closeDelivery {
			record.deliveryOpen = false
		}
		for identity, lease := range record.leases {
			if !owner(identity) {
				continue
			}
			lease.invalidate()
			joins = append(joins, sourceLeaseJoin{peer: record.facts.source.info.peer, idle: lease.idle})
		}
		store.dropUnused(record)
	}
	store.mu.Unlock()
	for _, join := range joins {
		if err := waitSourceReceiptWrites(ctx, join.idle); err != nil {
			return fmt.Errorf("source peer %q at %s port %d: final selected writes did not join: %w",
				join.peer.settings.Name, join.peer.addrString, join.peer.settings.Port, err)
		}
	}
	return nil
}
