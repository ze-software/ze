// Design: docs/architecture/plugin/rib-storage-design.md -- bounded ordinary output.
// Related: rib_advertise_capture.go -- immutable elected received-route values.
package rib

import (
	"context"
	"encoding/hex"
	"errors"
	"fmt"
	"iter"
	"math"
	"net/netip"
	"slices"
	"sync"
	"time"

	"github.com/ze-software/ze/internal/component/bgp/plugins/rib/storage"
	"github.com/ze-software/ze/internal/core/bgp/nlri/nlrisplit"
	"github.com/ze-software/ze/internal/core/family"
	"github.com/ze-software/ze/pkg/plugin/rpc"
)

// scopedRevision orders published desired state, independently of received
// messages, source-policy epochs, capture revisions and socket generations.
// Zero denotes a fresh empty scope, never an advertised selection.
type scopedRevision uint64

type selectedScopedRoute struct {
	wire    rpc.SelectedRoute
	steady  selectedItemBudget
	initial selectedItemBudget
	absence selectedItemBudget
}

// selectedAcknowledgment contains no route payload. possible includes both a
// confirmed write and an admitted request whose final answer was lost. Such a
// key MUST retain its retirement obligation until an explicit final receipt.
type selectedAcknowledgment struct {
	revision  scopedRevision
	native    []byte
	source    uint64
	possible  bool
	uncertain bool
	family    string
	absence   selectedItemBudget
}

// selectedDestinationID includes the engine's captured Session token. Two
// separately configured TCP peers may have the same remote IP address.
type selectedDestinationID struct {
	peer    netip.Addr
	session uint64
}

type selectedDispatchItem struct {
	key     advertiseKey
	route   rpc.SelectedRoute
	source  uint64
	budget  selectedItemBudget
	absence selectedItemBudget
}

// selectedDestination is one explicitly admitted destination Session. Its zero
// is uninitialized. The dispatch mutex protects all fields except its channels
// and immutable lifetime/identity. One worker owns each RPC through settlement.
type selectedDestination struct {
	peer         netip.Addr
	address      string
	session      uint64
	ctx          context.Context
	cancel       context.CancelFunc
	done         chan struct{}
	wake         chan struct{}
	acknowledged map[advertiseKey]selectedAcknowledgment
	pending      map[advertiseKey]struct{}
	inFlight     []selectedDispatchItem
	ready        bool
	failure      error
	envelope     selectedItemBudget
}

// selectedDispatch holds only current desired selections and the output
// identities each current destination may still own. It never stores global
// withdrawn history. Nil means ordinary dispatch has not been started.
//
// Capture/election locks may precede mu. No operation holding mu may acquire
// peer/election/storage locks, call the engine, or wait for a worker. Current
// selection callbacks read this mutex alone, independently of event delivery.
type selectedDispatch struct {
	mu           sync.Mutex
	ctx          context.Context
	cancel       context.CancelFunc
	send         rpc.SelectRoutesHandler
	ready        func(context.Context, string, uint64) error
	reporter     *selectedReporter
	current      map[advertiseKey]*selectedScopedRoute
	destinations map[selectedDestinationID]*selectedDestination
	cut          scopedRevision
	changed      chan struct{}
	stopped      bool
	fatal        error
}

func newSelectedDispatch(parent context.Context, send rpc.SelectRoutesHandler, ready func(context.Context, string, uint64) error, report func(context.Context, error) error) (*selectedDispatch, error) {
	if parent == nil {
		return nil, errors.New("selected dispatcher needs its plugin lifetime; start it before admitting destinations")
	}
	if send == nil {
		return nil, errors.New("selected dispatcher has no final writer; install the selected SDK operation")
	}
	ctx, cancel := context.WithCancel(parent)
	d := &selectedDispatch{ctx: ctx, cancel: cancel, send: send, ready: ready,
		current:      make(map[advertiseKey]*selectedScopedRoute),
		destinations: make(map[selectedDestinationID]*selectedDestination), changed: make(chan struct{})}
	if report != nil {
		d.reporter = newSelectedReporter(d, report)
	}
	return d, nil
}

// publish borrows a capture's owned immutable bytes. The hex wire projection
// is made once for the changed winner and shared by all destination batches.
// The caller MUST supply a nonnil capture and keep its fields unchanged until
// return. publish MUST NOT retain the pointer or change the capture; its shared
// immutable byte slices retain their existing lifetime contract.
func (d *selectedDispatch) publish(capture *advertiseRoute) error {
	if d == nil {
		return nil
	}
	level := rpc.SelectedFresh
	switch capture.staleLevel {
	case 0:
	case 1:
		level = rpc.SelectedStale
	case 2:
		level = rpc.SelectedLLGR
	default:
		return errors.New("selected capture has unknown stale treatment; restart the deciding process")
	}
	fam := capture.key.family.String()
	stored := &rpc.StoredRoute{SourcePeer: capture.route.Source.String(), Family: fam,
		AttrHex: hex.EncodeToString(capture.route.Attributes), NextHopHex: hex.EncodeToString(capture.route.NextHop),
		NLRIHex: hex.EncodeToString(capture.route.NLRI), MsgID: capture.route.MessageID,
		SourcePolicyEpoch: capture.route.SourcePolicyEpoch, PathID: capture.route.PathID,
		NLRIFraming: rpc.NLRIFramingSourceWire}
	route := rpc.SelectedRoute{Kind: rpc.SelectedRouteReceived, Family: fam,
		KeyNLRI: capture.keyNLRI, Route: stored, SourceReceipt: capture.route.SourceReceipt,
		SourceAddPath: capture.route.AddPath, Retained: capture.staleLevel > 0,
		StaleLevel: level, FlowSpecRedirectRefused: capture.redirectRefused}
	current, err := newSelectedScopedRoute(route)
	if err != nil {
		return err
	}
	d.mu.Lock()
	defer d.mu.Unlock()
	if err := d.advanceLocked(); err != nil {
		return err
	}
	current.wire.Revision = uint64(d.cut)
	d.current[capture.key] = current
	for _, destination := range d.destinations {
		destination.pending[capture.key] = struct{}{}
	}
	d.signalLocked()
	return nil
}

func (d *selectedDispatch) advanceLocked() error {
	if d.stopped {
		return errors.New("selected dispatcher has stopped; restart the deciding process before replay")
	}
	if d.cut == scopedRevision(math.MaxUint64) {
		d.failLocked(errors.New("selected scope revision exhausted; restart the deciding process before replay"))
		return d.fatal
	}
	d.cut++
	return nil
}

func (d *selectedDispatch) remove(key advertiseKey) error {
	if d == nil {
		return nil
	}
	d.mu.Lock()
	defer d.mu.Unlock()
	if _, exists := d.current[key]; !exists {
		return nil
	}
	if err := d.advanceLocked(); err != nil {
		return err
	}
	delete(d.current, key)
	for _, destination := range d.destinations {
		ack, exists := destination.acknowledged[key]
		if !exists {
			delete(destination.pending, key)
			continue
		}
		if !ack.possible {
			delete(destination.acknowledged, key)
			delete(destination.pending, key)
			continue
		}
		destination.pending[key] = struct{}{}
	}
	d.signalLocked()
	return nil
}

// renewSource is the rollback publication after the engine restores fresh
// source leases. Even equal bytes MUST acquire a new scope revision: an old
// queued request has not necessarily acquired its now-revoked write guard.
func (d *selectedDispatch) renewSource(receipts []uint64) error {
	if d == nil {
		return nil
	}
	d.mu.Lock()
	defer d.mu.Unlock()
	var matched int
	for _, current := range d.current {
		if selectedReceiptIn(current.wire.SourceReceipt, receipts) {
			matched++
		}
	}
	if uint64(matched) > math.MaxUint64-uint64(d.cut) {
		d.failLocked(errors.New("selected scope revision exhausted during source restore; restart the deciding process"))
		return d.fatal
	}
	for key, current := range d.current {
		if !selectedReceiptIn(current.wire.SourceReceipt, receipts) {
			continue
		}
		if err := d.advanceLocked(); err != nil {
			return err
		}
		next := *current
		next.wire.Revision = uint64(d.cut)
		d.current[key] = &next
		for _, destination := range d.destinations {
			destination.pending[key] = struct{}{}
		}
	}
	d.signalLocked()
	return nil
}

// fail publishes an unrecoverable capture failure before a final-writer query
// or settlement can authorize the previous snapshot. Nil is passive storage.
func (d *selectedDispatch) fail(err error) {
	if d == nil {
		return
	}
	d.mu.Lock()
	defer d.mu.Unlock()
	d.failLocked(err)
}

func (d *selectedDispatch) failLocked(err error) {
	if d.fatal != nil {
		return
	}
	d.fatal = err
	d.signalLocked()
}

func selectedReceiptIn(receipt uint64, receipts []uint64) bool {
	return slices.Contains(receipts, receipt)
}

func (d *selectedDispatch) signalLocked() {
	close(d.changed)
	d.changed = make(chan struct{})
	for _, destination := range d.destinations {
		select {
		case destination.wake <- struct{}{}:
		default:
		}
	}
}

// destinationUp is called only from the authenticated exact destination-owner
// projection. An UP received through source/state grants alone is insufficient.
func (d *selectedDispatch) destinationUp(peer netip.Addr, session uint64) error {
	if !peer.IsValid() {
		return errors.New("selected destination has no peer address; use its authenticated state event")
	}
	if session == 0 {
		return errors.New("selected destination has no Session token; use its authenticated UP receipt")
	}
	address := peer.String()
	envelope, err := selectedEnvelopeSize(rpc.SelectRoutesInput{Destination: address, DestinationSession: session})
	if err != nil {
		return err
	}
	d.mu.Lock()
	defer d.mu.Unlock()
	if d.stopped {
		return errors.New("selected dispatcher has stopped; start a new deciding process")
	}
	id := selectedDestinationID{peer: peer, session: session}
	if d.destinations[id] != nil {
		return nil
	}
	ctx, cancel := context.WithCancel(d.ctx)
	destination := &selectedDestination{peer: peer, address: address, session: session, envelope: envelope,
		ctx: ctx, cancel: cancel, done: make(chan struct{}), wake: make(chan struct{}, 1),
		acknowledged: make(map[advertiseKey]selectedAcknowledgment), pending: make(map[advertiseKey]struct{}, len(d.current))}
	for key := range d.current {
		destination.pending[key] = struct{}{}
	}
	d.destinations[id] = destination
	destination.wake <- struct{}{}
	// This is the destination Session's one long-lived component worker, not
	// a goroutine per route, message or retry. DOWN/stop cancel and join it.
	go d.run(destination)
	return nil
}

func (d *selectedDispatch) destinationDown(ctx context.Context, peer netip.Addr, session uint64) error {
	if d == nil {
		return nil
	}
	if session == 0 {
		return errors.New("selected destination DOWN has no captured Session; use the ended Session receipt")
	}
	id := selectedDestinationID{peer: peer, session: session}
	d.mu.Lock()
	destination := d.destinations[id]
	if destination == nil {
		d.mu.Unlock()
		return nil
	}
	destination.cancel()
	d.mu.Unlock()
	select {
	case <-destination.done:
		d.mu.Lock()
		if d.destinations[id] == destination {
			delete(d.destinations, id)
		}
		d.signalLocked()
		d.signalReportLocked()
		d.mu.Unlock()
		return nil
	case <-ctx.Done():
		return fmt.Errorf("selected output to %s Session %d did not retire: %w; inspect the destination and retry its removal", peer, session, ctx.Err())
	}
}

func (d *selectedDispatch) stop(ctx context.Context) error {
	if d == nil {
		return nil
	}
	d.mu.Lock()
	d.stopped = true
	d.cancel()
	active := make([]*selectedDestination, 0, len(d.destinations))
	for _, destination := range d.destinations {
		active = append(active, destination)
	}
	d.mu.Unlock()
	var refused error
	for _, destination := range active {
		refused = errors.Join(refused, d.destinationDown(ctx, destination.peer, destination.session))
	}
	if d.reporter != nil {
		select {
		case <-d.reporter.done:
		case <-ctx.Done():
			refused = errors.Join(refused, ctx.Err())
		}
	}
	return refused
}

func (d *selectedDispatch) run(destination *selectedDestination) {
	defer close(destination.done)
	for {
		select {
		case <-destination.ctx.Done():
			return
		case <-destination.wake:
		}
		for destination.ctx.Err() == nil {
			d.mu.Lock()
			if destination.ctx.Err() != nil {
				d.mu.Unlock()
				return
			}
			batch, items, buildError := d.batchLocked(destination)
			if len(items) == 0 {
				destination.failure = buildError
				ready := !destination.ready
				d.signalChangeLocked()
				d.signalReportLocked()
				d.mu.Unlock()
				if buildError != nil {
					if !selectedDispatchRetry(destination) {
						return
					}
					continue
				}
				if ready {
					if !d.completeReady(destination) {
						if !selectedDispatchRetry(destination) {
							return
						}
						continue
					}
				}
				break
			}
			destination.inFlight = items
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
			d.mu.Unlock()

			output, err := d.send(destination.ctx, batch)
			if err == nil {
				err = output.Validate(batch)
			}
			recordSelectedBatch(err)
			d.mu.Lock()
			destination.inFlight = nil
			if destination.ctx.Err() != nil {
				d.mu.Unlock()
				return
			}
			if err == nil {
				err = d.completeLocked(destination, items, output)
			} else {
				// A multi-key request may have written a prefix before its
				// final answer failed. Retain one uncertainty bit, not a
				// history of all attempted source receipts or route bodies.
				for i := range items {
					ack := destination.acknowledged[items[i].key]
					ack.uncertain = true
					destination.acknowledged[items[i].key] = ack
				}
			}
			destination.failure = err
			d.signalChangeLocked()
			d.signalReportLocked()
			d.mu.Unlock()
			if err == nil {
				continue
			}
			// One bounded retry timer belongs to this worker. Publication can
			// wake it sooner; healthy recipients have independent workers.
			if !selectedDispatchRetry(destination) {
				return
			}
		}
	}
}

func selectedDispatchRetry(destination *selectedDestination) bool {
	timer := time.NewTimer(time.Second)
	defer timer.Stop()
	select {
	case <-destination.ctx.Done():
		return false
	case <-destination.wake:
		return true
	case <-timer.C:
		return true
	}
}

func (d *selectedDispatch) signalChangeLocked() {
	close(d.changed)
	d.changed = make(chan struct{})
}

func (d *selectedDispatch) completeReady(destination *selectedDestination) bool {
	var err error
	if d.ready != nil {
		err = d.ready(destination.ctx, destination.address, destination.session)
	}
	d.mu.Lock()
	defer d.mu.Unlock()
	if destination.ctx.Err() != nil {
		return false
	}
	if err != nil {
		destination.failure = err
		d.signalChangeLocked()
		d.signalReportLocked()
		return false
	}
	destination.ready = true
	d.signalChangeLocked()
	return true
}

func (d *selectedDispatch) completeLocked(destination *selectedDestination, items []selectedDispatchItem, output *rpc.SelectRoutesOutput) error {
	var refused error
	for i := range items {
		item := &items[i]
		result := output.Results[i]
		// These are open final wire outcomes; Validate has refused unknowns.
		switch result.Outcome {
		case rpc.SelectedWritten:
			destination.acknowledged[item.key] = selectedAcknowledgment{revision: scopedRevision(item.route.Revision),
				native: item.route.KeyNLRI, source: item.source, possible: true, family: item.route.Family, absence: item.absence}
		case rpc.SelectedSuppressed, rpc.SelectedAbsent:
			if _, current := d.current[item.key]; current {
				destination.acknowledged[item.key] = selectedAcknowledgment{revision: scopedRevision(item.route.Revision),
					native: item.route.KeyNLRI, source: item.source, family: item.route.Family, absence: item.absence}
			} else {
				delete(destination.acknowledged, item.key)
			}
		case rpc.SelectedStaleRevision:
			// No success projection: reconcile the current desired revision.
			refused = errors.Join(refused, errors.New("selected writer reports a newer revision; reconcile the current destination selection"))
		case rpc.SelectedOutcomeUnspecified:
			panic("BUG: validated selected outcome is unspecified")
		default:
			panic("BUG: validated selected outcome is unknown")
		}
		if current := d.current[item.key]; current != nil {
			if destination.acknowledged[item.key].revision == scopedRevision(current.wire.Revision) {
				delete(destination.pending, item.key)
			}
			continue
		}
		if _, remains := destination.acknowledged[item.key]; !remains {
			delete(destination.pending, item.key)
		}
	}
	return refused
}

// batchLocked derives absence only from possible output. Suppressed current
// keys are retained only to avoid re-sending unchanged policy exclusions.
func (d *selectedDispatch) batchLocked(destination *selectedDestination) (rpc.SelectRoutesInput, []selectedDispatchItem, error) {
	batch := rpc.SelectRoutesInput{Destination: destination.address, DestinationSession: destination.session}
	if d.fatal != nil {
		return batch, nil, d.fatal
	}
	return selectedDispatchSizedBatch(batch, destination.envelope, d.pendingItemsLocked(destination))
}

// pendingItemsLocked borrows current captures only during batch construction.
// The receiver MUST hold d.mu through iteration and MUST NOT retain the iterator.
func (d *selectedDispatch) pendingItemsLocked(destination *selectedDestination) iter.Seq[selectedDispatchItem] {
	return func(yield func(selectedDispatchItem) bool) {
		for key := range destination.pending {
			if current := d.current[key]; current != nil {
				route := current.wire
				stored := *route.Route
				stored.InitialUpdate = !destination.ready
				route.Route = &stored
				budget := current.steady
				if !destination.ready {
					budget = current.initial
				}
				if !yield(selectedDispatchItem{key: key, route: route, source: route.SourceReceipt, budget: budget, absence: current.absence}) {
					return
				}
				continue
			}
			ack := destination.acknowledged[key]
			if !ack.possible {
				delete(destination.acknowledged, key)
				delete(destination.pending, key)
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

func selectedDispatchKey(name string, raw []byte) (advertiseKey, error) {
	fam, found := family.LookupFamily(name)
	if !found {
		return advertiseKey{}, errors.New("current selection names an unknown family; use the registered selected family")
	}
	key := advertiseKey{family: fam}
	if storage.IsCIDRFamily(fam) {
		_, prefix, valid := parsePrevKey(fam, raw, false)
		if !valid {
			return key, errors.New("current selection has an invalid CIDR key; use native advertisement framing")
		}
		key.prefix = prefix
		return key, nil
	}
	var scratch [nlrisplit.PrefixKeyScratchSize]byte
	native, valid := routeIdentity(fam, raw, false, false, scratch[:])
	if !valid {
		return key, errors.New("current selection has an invalid native key; use native advertisement framing")
	}
	key.native = string(native)
	return key, nil
}

func (d *selectedDispatch) currentSelection(ctx context.Context, input rpc.CurrentSelectionInput) (*rpc.CurrentSelectionOutput, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	peer, err := parsePeerAddr(input.Destination)
	if err != nil {
		return nil, err
	}
	keys := make([]advertiseKey, len(input.Keys))
	for i, key := range input.Keys {
		keys[i], err = selectedDispatchKey(key.Family, key.KeyNLRI)
		if err != nil {
			return nil, err
		}
	}
	d.mu.Lock()
	defer d.mu.Unlock()
	if d.fatal != nil {
		return nil, d.fatal
	}
	destination := d.destinations[selectedDestinationID{peer: peer, session: input.DestinationSession}]
	if destination == nil {
		return nil, errors.New("current selection destination is not admitted; use its authenticated owner and Session")
	}
	if destination.session != input.DestinationSession {
		return nil, errors.New("current selection destination Session ended; use the current UP receipt")
	}
	if err := destination.ctx.Err(); err != nil {
		return nil, err
	}
	output := &rpc.CurrentSelectionOutput{Destination: input.Destination, DestinationSession: input.DestinationSession,
		Cut: uint64(d.cut), Results: make([]rpc.CurrentSelectionResult, len(keys))}
	for i, key := range keys {
		requested := input.Keys[i]
		result := rpc.CurrentSelectionResult{Family: requested.Family, KeyNLRI: append([]byte(nil), requested.KeyNLRI...),
			Revision: requested.Revision, Presence: rpc.CurrentSelectionAbsent}
		if current := d.current[key]; current != nil {
			result.Presence, result.CurrentRevision = rpc.CurrentSelectionPresent, current.wire.Revision
		}
		output.Results[i] = result
	}
	return output, nil
}
