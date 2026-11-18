// Design: docs/architecture/plugin/rib-storage-design.md -- selected refresh settlement.
// Related: rib_selected_dispatch.go -- the same long-lived destination worker.
package rib

import (
	"context"
	"errors"
	"iter"
	"net/netip"

	"github.com/ze-software/ze/internal/core/diagnostic"
	"github.com/ze-software/ze/internal/core/family"
	"github.com/ze-software/ze/pkg/plugin/rpc"
)

type selectedRefreshSender func(context.Context, rpc.SelectRefreshInput) (*rpc.SelectRefreshOutput, error)

type selectedRefreshPhase uint8

const (
	selectedRefreshUnspecified selectedRefreshPhase = iota
	selectedRefreshBegin
	selectedRefreshRoutes
	selectedRefreshEnd
)

// selectedRefreshRun owns one family cut, never route payload or withdrawal
// history. The initial cut bounds keys and sources; current captures are read
// again when a batch is built. Only the destination's existing worker advances it.
type selectedRefreshRun struct {
	input   rpc.SelectRefreshInput
	phase   selectedRefreshPhase
	keys    map[advertiseKey]struct{}
	sources map[uint64]struct{}
	// sourceUncertain is one bounded conservative obligation, not a history
	// of every replacement source. It lasts until this replay's final End.
	sourceUncertain bool
}

// selectedRefreshFamilies seals the registered universe at this destination's
// UP. Runtime registration after this OPEN cannot grow its request inventory;
// the engine's final writer still checks the exact negotiated intersection.
func selectedRefreshFamilies() map[family.Family]struct{} {
	names := family.RegisteredFamilyNames()
	allowed := make(map[family.Family]struct{}, len(names))
	for _, name := range names {
		if fam, registered := family.LookupFamily(name); registered {
			allowed[fam] = struct{}{}
		}
	}
	return allowed
}

// setRefreshSender MUST precede destination admission. Its caller MUST bind
// the SDK's final-receipt operation, not an enqueue-only or bare-IP command.
func (d *selectedDispatch) setRefreshSender(send selectedRefreshSender) error {
	if send == nil {
		return errors.New("selected refresh has no final marker writer; bind the SDK refresh operation")
	}
	d.mu.Lock()
	defer d.mu.Unlock()
	if d.stopped || len(d.destinations) != 0 {
		return errors.New("selected refresh writer was bound after destination admission; restart the deciding process")
	}
	d.refresh = send
	return nil
}

// requestRefresh consumes authenticated engine facts. It MUST NOT derive
// authority from peer address. Callers MUST preserve the original receiving
// owner and Session together. Each already admitted Session holds at most one
// pending request per registered family plus its one active family cut.
func (d *selectedDispatch) requestRefresh(peer netip.Addr, owner, session uint64, fam family.Family) error {
	if owner == 0 || session == 0 {
		return errors.New("selected refresh has no captured owner or Session; preserve the receiving connection's event facts")
	}
	known, registered := family.LookupFamily(fam.String())
	if !registered || known != fam {
		return errors.New("selected refresh family is not registered; preserve the receiving connection's negotiated family")
	}
	d.mu.Lock()
	defer d.mu.Unlock()
	if d.stopped {
		return errors.New("selected refresh dispatcher has stopped; reconnect the deciding process")
	}
	destination := d.destinations[selectedDestinationID{peer: peer, session: session}]
	if destination == nil {
		return errors.New("selected refresh names no admitted destination Session; use its current ordinary UP receipt")
	}
	if destination.owner != owner {
		return errors.New("selected refresh owner differs from its captured destination; preserve the original Peer owner")
	}
	if _, admitted := destination.refreshFamilies[fam]; !admitted {
		return errors.New("selected refresh family was not registered for this captured Session; use a family from its OPEN")
	}
	if d.refresh == nil {
		return errors.New("selected refresh marker writer is unavailable; bind the SDK refresh operation before admitting destinations")
	}
	if active := destination.refreshActive; active != nil && active.input.PeerOwner != owner {
		return errors.New("selected refresh owner differs from the active captured Session; preserve the original peer owner")
	}
	if previous, queued := destination.refreshPending[fam]; queued {
		if previous != owner {
			return errors.New("selected refresh owner differs from the queued captured Session; preserve the original peer owner")
		}
		return nil
	}
	if destination.refreshPending == nil {
		destination.refreshPending = make(map[family.Family]uint64)
	}
	destination.refreshPending[fam] = owner
	destination.refreshOrder = append(destination.refreshOrder, fam)
	d.signalLocked()
	return nil
}

func (d *selectedDispatch) refreshQueuedLocked(destination *selectedDestination) bool {
	if d.fatal != nil {
		return false
	}
	initialSettled := destination.ready || len(destination.pending) == 0
	return destination.refreshActive != nil || (initialSettled && len(destination.refreshOrder) != 0)
}

func (d *selectedDispatch) startRefreshLocked(destination *selectedDestination) *selectedRefreshRun {
	if destination.refreshActive != nil {
		return destination.refreshActive
	}
	fam := destination.refreshOrder[0]
	owner := destination.refreshPending[fam]
	delete(destination.refreshPending, fam)
	destination.refreshOrder = destination.refreshOrder[1:]
	refresh := &selectedRefreshRun{phase: selectedRefreshBegin, input: rpc.SelectRefreshInput{
		Destination: destination.address, PeerOwner: owner,
		DestinationSession: destination.session, Family: fam.String(),
	}}
	destination.refreshActive = refresh
	return refresh
}

// captureRefreshLocked takes the family cut only after Begin's final receipt.
// The source set is frozen and bounded by that cut, so later source churn
// cannot turn a held marker into an unbounded history of source identities.
func (d *selectedDispatch) captureRefreshLocked(destination *selectedDestination, refresh *selectedRefreshRun) {
	refresh.keys = make(map[advertiseKey]struct{})
	refresh.sources = make(map[uint64]struct{})
	keys := make(map[advertiseKey]struct{}, len(d.current))
	for key := range d.current {
		keys[key] = struct{}{}
	}
	for key := range d.scoped[originScope{peer: destination.peer, owner: destination.owner}] {
		keys[key] = struct{}{}
	}
	for key := range keys {
		current := d.currentForLocked(destination, key)
		if current.wire.Family != refresh.input.Family {
			continue
		}
		refresh.keys[key] = struct{}{}
		refresh.sources[current.wire.SourceReceipt] = struct{}{}
	}
	for key, ack := range destination.acknowledged {
		if ack.family != refresh.input.Family || !ack.possible {
			continue
		}
		refresh.keys[key] = struct{}{}
		refresh.sources[ack.source] = struct{}{}
	}
	refresh.phase = selectedRefreshRoutes
}

// refreshStep runs on the destination's existing worker. No event handler
// waits here and no new goroutine or per-request timer is created. The worker
// MUST await Begin and every replay receipt before End; End MUST NOT settle
// route work that has not received its own final receipt.
func (d *selectedDispatch) refreshStep(destination *selectedDestination, refresh *selectedRefreshRun) error {
	switch refresh.phase {
	case selectedRefreshUnspecified:
		panic("BUG: selected refresh has an unspecified worker phase")
	case selectedRefreshBegin:
		if err := d.refreshMarker(destination, refresh, rpc.SelectedRefreshBegin); err != nil {
			return err
		}
		d.mu.Lock()
		if destination.ctx.Err() == nil {
			d.captureRefreshLocked(destination, refresh)
			d.signalChangeLocked()
		}
		d.mu.Unlock()
		return destination.ctx.Err()
	case selectedRefreshRoutes:
		d.mu.Lock()
		batch := rpc.SelectRoutesInput{Destination: destination.address, DestinationSession: destination.session}
		batch, items, err := selectedDispatchSizedBatch(batch, destination.envelope,
			d.itemsForKeysLocked(destination, refresh.keys))
		if len(items) == 0 {
			if err == nil {
				refresh.phase = selectedRefreshEnd
				d.signalChangeLocked()
			}
			d.mu.Unlock()
			return err
		}
		d.admitBatchLocked(destination, items)
		d.mu.Unlock()
		if err := d.sendBatch(destination, batch, items); err != nil {
			return err
		}
		d.mu.Lock()
		for _, item := range items {
			if d.keySettledLocked(destination, item.key) {
				delete(refresh.keys, item.key)
			}
		}
		d.mu.Unlock()
		return nil
	case selectedRefreshEnd:
		if err := d.refreshMarker(destination, refresh, rpc.SelectedRefreshEnd); err != nil {
			return err
		}
		d.mu.Lock()
		if destination.ctx.Err() == nil {
			destination.refreshActive = nil
			destination.failure = nil
			d.signalChangeLocked()
			d.signalReportLocked()
		}
		d.mu.Unlock()
		return destination.ctx.Err()
	default:
		panic("BUG: selected refresh has an unknown worker phase")
	}
}

func (d *selectedDispatch) refreshMarker(destination *selectedDestination, refresh *selectedRefreshRun, action rpc.SelectedRefreshAction) error {
	input := refresh.input
	input.Action = action
	output, err := d.refresh(destination.ctx, input)
	if err == nil {
		err = output.Validate(input)
	}
	return err
}

func (d *selectedDispatch) keySettledLocked(destination *selectedDestination, key advertiseKey) bool {
	if current := d.currentForLocked(destination, key); current != nil {
		return destination.acknowledged[key].revision == scopedRevision(current.wire.Revision)
	}
	_, remains := destination.acknowledged[key]
	return !remains
}

// itemsForKeysLocked borrows only current immutable captures and final receipt
// identities. It MUST run under d.mu and MUST NOT escape its iterator.
func (d *selectedDispatch) itemsForKeysLocked(destination *selectedDestination, keys map[advertiseKey]struct{}) iter.Seq[selectedDispatchItem] {
	return func(yield func(selectedDispatchItem) bool) {
		for key := range keys {
			if current := d.currentForLocked(destination, key); current != nil {
				route := current.wire
				stored := *route.Route
				stored.InitialUpdate = !destination.ready
				route.Route = &stored
				budget := current.steady
				if !destination.ready {
					budget = current.initial
				}
				if !yield(selectedDispatchItem{key: key, route: route, source: route.SourceReceipt,
					budget: budget, absence: current.absence}) {
					return
				}
				continue
			}
			ack := destination.acknowledged[key]
			if !ack.possible {
				delete(destination.acknowledged, key)
				delete(keys, key)
				continue
			}
			if !yield(selectedDispatchItem{key: key, source: ack.source, budget: ack.absence, absence: ack.absence,
				route: rpc.SelectedRoute{Kind: rpc.SelectedRouteAbsent, Family: ack.family,
					KeyNLRI: ack.native, Revision: uint64(d.cut)}}) {
				return
			}
		}
	}
}

// admitBatchLocked creates the batch cancellation and completion state.
// The caller MUST hold d.mu and then call sendBatch exactly once with the same
// destination and items after releasing d.mu.
func (d *selectedDispatch) admitBatchLocked(destination *selectedDestination, items []selectedDispatchItem) {
	destination.inFlight = items
	destination.batchContext, destination.cancelBatch = context.WithCancel(destination.ctx)
	// Only local-origin batches need the separate receipt-removal join.
	// Learned-only throughput retains the existing worker completion path.
	for _, item := range items {
		if item.route.Kind == rpc.SelectedRouteOrigin {
			destination.batchDone = make(chan struct{})
			break
		}
	}
	for i := range items {
		item := &items[i]
		if item.route.Kind == rpc.SelectedRouteAbsent {
			continue
		}
		previous := destination.acknowledged[item.key]
		if !previous.possible {
			previous.native, previous.source, previous.possible = item.route.KeyNLRI, item.source, true
			previous.family, previous.absence = item.route.Family, item.absence
		}
		destination.acknowledged[item.key] = previous
	}
}

// sendBatch MUST follow admitBatchLocked with the same destination and items,
// with d.mu released. It cancels the admitted context and completes its join.
func (d *selectedDispatch) sendBatch(destination *selectedDestination, batch rpc.SelectRoutesInput, items []selectedDispatchItem) error {
	output, err := d.send(destination.batchContext, batch)
	destination.cancelBatch()
	if err == nil {
		err = output.Validate(batch)
	}
	recordSelectedBatch(err)
	d.mu.Lock()
	defer d.mu.Unlock()
	destination.inFlight = nil
	destination.cancelBatch = nil
	destination.batchContext = nil
	completed := destination.batchDone
	destination.batchDone = nil
	if completed != nil {
		defer close(completed)
	}
	if destination.ctx.Err() != nil {
		return destination.ctx.Err()
	}
	if active := destination.refreshActive; active != nil {
		for i, item := range items {
			if _, captured := active.sources[item.source]; captured {
				continue
			}
			// A failed answer can hide a completed prefix of the batch.
			// A successful answer names actual writes. Either can add a
			// source outside the frozen cut; retain one uncertainty bit
			// instead of accumulating replacement-source history.
			if err != nil {
				active.sourceUncertain = true
				continue
			}
			if output.Results[i].Outcome == rpc.SelectedWritten {
				active.sourceUncertain = true
			}
		}
	}
	if err == nil {
		err = d.completeLocked(destination, items, output)
	} else {
		for i := range items {
			ack := destination.acknowledged[items[i].key]
			ack.uncertain = true
			ack.failure = &selectedWriteFailure{revision: scopedRevision(items[i].route.Revision), kind: items[i].route.Kind, cause: err}
			destination.acknowledged[items[i].key] = ack
		}
	}
	destination.failure = err
	d.signalChangeLocked()
	d.signalReportLocked()
	return err
}

// queueSelectedRefresh chooses the existing replay authority. Ordinary
// destinations use only the selected worker. Other recipients retain their
// SENT replay under the existing grants; observing a selected owner's state
// does not give that observer the selected owner's route or marker authority.
// The caller preserves the original receiving owner and Session.
func (r *RIBManager) queueSelectedRefresh(peer string, owner, session uint64, eligible bool, fam family.Family) {
	address, err := parsePeerAddr(peer)
	if err == nil && !eligible && !r.currentRefreshSession(address, owner, session, eligible) {
		logger().Debug("refresh observation belongs to an ended or unowned Session", "peer", peer, "session", session)
		return
	}
	if err == nil && !eligible {
		r.replayLegacyRefresh(peer, owner, fam)
		return
	}
	if err == nil {
		if r.selectedDispatch == nil {
			err = errors.New("ordinary refresh has no deciding dispatcher; restart the configured RIB process")
		} else {
			err = r.selectedDispatch.requestRefresh(address, owner, session, fam)
		}
	}
	if err != nil {
		logger().Warn("ordinary refresh was refused", "peer", peer, "session", session, "error", err,
			"code", diagnostic.CodeDoctorBGPSelectedOutput, "fix", "ze show bgp rib status")
	}
}

// replayLegacyRefresh preserves the existing non-selected refresh path. It
// reads only the destination's existing SENT inventory, never the selected RIB or a
// losing candidate. The engine's existing command admission still decides
// whether this process may write the markers and replayed routes.
// RFC 7313 Section 4: "After the speaker completes the re-advertisement
// of the entire Adj-RIB-Out to the peer, it MUST send an EoRR message."
func (r *RIBManager) replayLegacyRefresh(peer string, owner uint64, fam family.Family) {
	_, err := parsePeerAddr(peer)
	if err != nil {
		logger().Warn("refresh event dropped", "error", err,
			"code", diagnostic.CodeDoctorBGPSelectedOutput, "fix", "ze show bgp rib status")
		return
	}
	r.peerMu.RLock()
	if !r.peerUp[owner] {
		r.peerMu.RUnlock()
		return
	}
	routes := r.collectRibOutRoutes(owner, fam)
	r.peerMu.RUnlock()

	// RFC 7313 Section 4: marker ordering brackets the existing SENT replay.
	r.dispatchPeerAction(peer, "borr "+fam.String())
	r.sendRoutes(peer, routes)
	r.dispatchPeerAction(peer, "eorr "+fam.String())
}

// rememberRefreshSession stores only the latest live state observation in the
// existing peer metadata. It grants nothing and keeps no ended-Session ledger.
// Callers MUST invoke it before taking peerMu, on both ordinary state rails.
func (r *RIBManager) rememberRefreshSession(peer netip.Addr, owner, session uint64, up, ordinary bool) {
	if session == 0 {
		return
	}
	r.peerMu.Lock()
	defer r.peerMu.Unlock()
	metadata := r.peerMeta[owner]
	if up {
		if metadata == nil {
			metadata = &peerMetadata{Address: peer}
			r.peerMeta[owner] = metadata
		}
		metadata.refreshOwner, metadata.refreshSession, metadata.refreshOrdinary = owner, session, ordinary
	} else if metadata != nil && metadata.refreshSession == session {
		metadata.refreshOwner, metadata.refreshSession, metadata.refreshOrdinary = 0, 0, false
	}
}

// copyRefreshSessionLocked MUST run under peerMu when UPDATE metadata replaces
// a peer value. Metadata changes MUST NOT erase the current control tuple.
func (r *RIBManager) copyRefreshSessionLocked(peer uint64, metadata *peerMetadata) {
	if previous := r.peerMeta[peer]; previous != nil {
		metadata.refreshOwner = previous.refreshOwner
		metadata.refreshSession = previous.refreshSession
		metadata.refreshOrdinary = previous.refreshOrdinary
	}
}

// currentRefreshSession refuses a captured stale observation before any legacy
// bare-address command. A current ordinary observation without eligibility
// never borrows legacy replay. The unrecorded legacy fixture/event form remains
// only when this peer has no captured current control Session.
func (r *RIBManager) currentRefreshSession(peer netip.Addr, owner, session uint64, eligible bool) bool {
	r.peerMu.RLock()
	defer r.peerMu.RUnlock()
	metadata := r.peerMeta[owner]
	if session == 0 {
		return !eligible && (metadata == nil || metadata.refreshSession == 0)
	}
	if owner == 0 || !r.peerUp[owner] || metadata == nil {
		return false
	}
	if metadata.Address != peer {
		return false
	}
	if metadata.refreshSession != session || metadata.refreshOwner != owner {
		return false
	}
	return !metadata.refreshOrdinary || eligible
}
