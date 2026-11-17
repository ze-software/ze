// Design: docs/architecture/api/process-protocol.md -- selected-output settlement.
// Related: rib_selected_dispatch.go -- one final writer request per destination.
package rib

import (
	"context"
	"errors"
	"net/netip"
)

// barrier joins the currently owed output of the specified destination set.
// An empty set means all currently admitted destinations. It never waits for
// another destination's worker while holding the publication mutex.
func (d *selectedDispatch) barrier(ctx context.Context, peers ...netip.Addr) error {
	if d == nil {
		return nil
	}
	d.mu.Lock()
	active := make([]*selectedDestination, 0, len(d.destinations))
	for _, destination := range d.destinations {
		if len(peers) > 0 {
			if !selectedPeerIn(destination.peer, peers) {
				continue
			}
		}
		active = append(active, destination)
	}
	d.mu.Unlock()
	return d.barrierDestinations(ctx, active)
}

func selectedPeerIn(peer netip.Addr, peers []netip.Addr) bool {
	for _, candidate := range peers {
		if peer == candidate {
			return true
		}
	}
	return false
}

// barrierSource includes output from a source that has already been replaced
// in current desired state. Its actual/uncertain acknowledged or in-flight
// source identity still names the affected destination until retirement ends.
func (d *selectedDispatch) barrierSource(ctx context.Context, receipts []uint64) error {
	if d == nil {
		return nil
	}
	d.mu.Lock()
	if d.fatal != nil {
		err := d.fatal
		d.mu.Unlock()
		return err
	}
	if len(receipts) == 0 {
		d.mu.Unlock()
		return nil
	}
	active := make([]*selectedDestination, 0, len(d.destinations))
	for _, destination := range d.destinations {
		if d.sourcePendingLocked(destination, receipts) {
			active = append(active, destination)
		}
	}
	d.mu.Unlock()
	return d.barrierDestinations(ctx, active)
}

func (d *selectedDispatch) sourcePendingLocked(destination *selectedDestination, receipts []uint64) bool {
	for _, current := range d.current {
		if selectedReceiptIn(current.wire.SourceReceipt, receipts) {
			return true
		}
	}
	for _, ack := range destination.acknowledged {
		if ack.uncertain {
			// The lost multi-key answer cannot prove which source is
			// currently on the wire at this affected destination.
			return true
		}
		if selectedReceiptIn(ack.source, receipts) {
			return true
		}
	}
	for _, item := range destination.inFlight {
		if selectedReceiptIn(item.source, receipts) {
			return true
		}
	}
	return false
}

func (d *selectedDispatch) barrierDestinations(ctx context.Context, active []*selectedDestination) error {
	for {
		d.mu.Lock()
		if d.fatal != nil {
			err := d.fatal
			d.mu.Unlock()
			return err
		}
		var unresolved []selectedUnsettled
		for _, destination := range active {
			id := selectedDestinationID{peer: destination.peer, session: destination.session}
			if d.destinations[id] != destination {
				continue // Its old Session's DOWN already joined the worker.
			}
			if d.settledLocked(destination) {
				continue
			}
			cause := destination.failure
			if cause == nil {
				cause = errors.New("selected output is queued or in flight")
			}
			unresolved = append(unresolved, selectedUnsettled{id: id, cause: cause})
		}
		changed := d.changed
		d.mu.Unlock()
		if len(unresolved) == 0 {
			return nil
		}
		select {
		case <-changed:
		case <-ctx.Done():
			return selectedBarrierError(ctx.Err(), unresolved)
		}
	}
}

func (d *selectedDispatch) settledLocked(destination *selectedDestination) bool {
	if len(destination.inFlight) > 0 {
		return false
	}
	if destination.failure != nil {
		return false
	}
	if !destination.ready {
		return false
	}
	return len(destination.pending) == 0
}
