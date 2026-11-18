// Design: docs/architecture/api/process-protocol.md -- selected-output settlement.
// Related: rib_selected_dispatch.go -- one final writer request per destination.
package rib

import (
	"context"
	"errors"
	"net/netip"
	"slices"
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
	return slices.Contains(peers, peer)
}

// barrierSource includes output from a source that has already been replaced
// in current desired state. Its actual/uncertain acknowledged or in-flight
// source identity still names the affected destination until retirement ends.
func (d *selectedDispatch) barrierSource(ctx context.Context, receipts []uint64) error {
	active, err := d.sourceDestinations(receipts)
	if err != nil {
		return err
	}
	return d.barrierDestinations(ctx, active)
}

// sourceDestinations MUST be captured before mutating a source as well as read
// afterwards. An unsent old winner can disappear without leaving an output
// receipt; its replacement still owes the destinations captured here. Callers
// MUST join these exact worker incarnations after publishing the replacement.
func (d *selectedDispatch) sourceDestinations(receipts []uint64) ([]*selectedDestination, error) {
	if d == nil {
		return nil, nil
	}
	d.mu.Lock()
	defer d.mu.Unlock()
	if d.fatal != nil {
		return nil, d.fatal
	}
	if len(receipts) == 0 {
		return nil, nil
	}
	active := make([]*selectedDestination, 0, len(d.destinations))
	for _, destination := range d.destinations {
		if d.sourcePendingLocked(destination, receipts) {
			active = append(active, destination)
		}
	}
	return active, nil
}

func (d *selectedDispatch) sourcePendingLocked(destination *selectedDestination, receipts []uint64) bool {
	for key := range d.current {
		current := d.currentForLocked(destination, key)
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
	if active := destination.refreshActive; active != nil {
		// Replacement writes outside the frozen cut cannot leave an
		// unbounded source history. As with a lost multi-key answer above,
		// conservatively join this destination's End for a nonempty source
		// request. Other destinations keep their independent workers.
		if active.sourceUncertain {
			if len(receipts) != 0 {
				return true
			}
		}
		for _, receipt := range receipts {
			if _, included := active.sources[receipt]; included {
				return true
			}
		}
	}
	for i := range destination.inFlight {
		if selectedReceiptIn(destination.inFlight[i].source, receipts) {
			return true
		}
	}
	return false
}

func (d *selectedDispatch) barrierDestinations(ctx context.Context, active []*selectedDestination) error {
	if d == nil {
		return nil
	}
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
	if destination.refreshActive != nil || len(destination.refreshOrder) != 0 {
		return false
	}
	if destination.reofferLearned || len(destination.inFlight) > 0 {
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

// sourceReleaseReady is the last-reference release guard, including retries
// from the running lease worker. It never waits: one blocked recipient cannot
// stop unrelated source releases. Existing retry admission repeats the guard.
// Operation holds cover mutation/publication; the captured workers then cover
// final removal or replacement, including a winner that was never written.
func (d *selectedDispatch) sourceReleaseReady(active []*selectedDestination) error {
	if d == nil {
		return nil
	}
	d.mu.Lock()
	defer d.mu.Unlock()
	if d.fatal != nil {
		return d.fatal
	}
	var pending []selectedUnsettled
	seen := make(map[*selectedDestination]struct{})
	inspect := func(destination *selectedDestination) {
		if _, duplicate := seen[destination]; duplicate {
			return
		}
		seen[destination] = struct{}{}
		id := selectedDestinationID{peer: destination.peer, session: destination.session}
		select {
		case <-destination.done:
			return
		default:
		}
		if destination.ctx.Err() != nil || d.destinations[id] != destination || !d.settledLocked(destination) {
			cause := destination.failure
			if cause == nil {
				cause = errors.New("selected output is queued or in flight")
			}
			pending = append(pending, selectedUnsettled{id: id, cause: cause})
		}
	}
	for _, destination := range active {
		inspect(destination)
	}
	if len(pending) != 0 {
		return selectedBarrierError(errors.New("selected output has not settled"), pending)
	}
	return nil
}
