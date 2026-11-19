// Design: docs/architecture/plugin/rib-storage-design.md -- admitted scoped origins.
// Related: rib_selected_dispatch.go -- one common learned winner and bounded output.
// Related: internal/core/rib/locrib/preference.go -- shared distance/metric order.
package rib

import (
	"context"
	"encoding/hex"
	"errors"
	"fmt"
	"math"
	"net/netip"
	"slices"
	"time"

	"github.com/ze-software/ze/internal/core/redistevents"
	"github.com/ze-software/ze/internal/core/rib/locrib"
	"github.com/ze-software/ze/pkg/plugin/rpc"
)

// originScope is the original configured Peer, independent of physical Session.
// It names no send authority; the issuer receipt and final writer supply that.
type originScope struct {
	peer  netip.Addr
	owner uint64
}

// originIdentity is a contribution, not a preference or a writer revision.
// Legal path zero is distinguished from an absent path identifier.
type originIdentity struct {
	producer rpc.OriginProducer
	kind     rpc.OriginSourceKind
	protocol redistevents.ProtocolID
	instance uint64
	path     uint32
	addPath  bool
}

// originContribution owns one current engine-issued scope membership. Its
// input is immutable and retains opaque original build/context facts. The
// central key/identity table shares the candidate population across scopes;
// destinations never hold a copied candidate RIB or a losing-candidate ladder.
// Zero/nil is not an admitted contribution; newOriginContribution validates it.
type originContribution struct {
	input rpc.OriginIntakeInput
	wire  *selectedScopedRoute
	order uint64
}

type originScopedWinner struct{ candidate, current *selectedScopedRoute }

func newOriginContribution(input rpc.OriginIntakeInput) (*originContribution, advertiseKey, originIdentity, error) {
	if err := input.Validate(time.Now()); err != nil {
		return nil, advertiseKey{}, originIdentity{}, err
	}
	key, err := selectedDispatchKey(input.Route.Family, input.Route.KeyNLRI)
	if err != nil {
		return nil, key, originIdentity{}, err
	}
	id := originIdentity{producer: input.Producer, kind: input.Source.Kind,
		instance: input.Source.Instance, path: input.Route.SourcePathID, addPath: input.Route.SourceAddPath}
	switch input.Source.Kind {
	case rpc.OriginSourceLocalBGP, rpc.OriginSourceConfiguredBGP:
		// Explicit BGP origins use the existing static preference class only.
	case rpc.OriginSourceRedistributed:
		var known bool
		id.protocol, known = redistevents.ProtocolIDOf(input.Source.Protocol)
		if !known {
			return nil, key, id, fmt.Errorf("origin source protocol %q is not registered; retain its accepted producer provenance", input.Source.Protocol)
		}
		if redistevents.WouldLoop(input.Source.Protocol, redistevents.ProtocolName(bgpProtocolID)) {
			return nil, key, id, errors.New("origin source would redistribute BGP back into BGP; remove that redistribution rule")
		}
	case rpc.OriginSourceUnspecified:
		return nil, key, id, errors.New("origin source is unspecified; preserve engine provenance")
	default:
		return nil, key, id, errors.New("origin source kind is unknown; use a supported engine origin kind")
	}
	owned := rpc.CloneOriginIntake(input)
	wire, err := newSelectedScopedRoute(rpc.SelectedRoute{Kind: rpc.SelectedRouteOrigin,
		Family: owned.Route.Family, KeyNLRI: owned.Route.KeyNLRI, OriginReceipt: owned.OriginReceipt,
		StaleLevel: rpc.SelectedFresh, Route: &rpc.StoredRoute{Family: owned.Route.Family,
			NLRIHex: hex.EncodeToString(owned.Route.KeyNLRI), NLRIFraming: rpc.NLRIFramingPrefixOnly}})
	if err != nil {
		return nil, key, id, err
	}
	wire.preference = locrib.PreferenceSource{Protocol: owned.Source.Protocol, Metric: owned.Source.Metric,
		DistanceOverride: owned.Source.Distance, HasDistanceOverride: owned.Source.HasDistance}
	if id.kind != rpc.OriginSourceRedistributed {
		wire.preference.Protocol = "static"
	}
	return &originContribution{input: owned, wire: wire}, key, id, nil
}

func (d *selectedDispatch) currentForLocked(destination *selectedDestination, key advertiseKey) *selectedScopedRoute {
	return d.currentScopeLocked(originScope{peer: destination.peer, owner: destination.owner}, key)
}

func (d *selectedDispatch) currentScopeLocked(scope originScope, key advertiseKey) *selectedScopedRoute {
	if scoped := d.scoped[scope]; scoped != nil {
		if value, found := scoped[key]; found {
			return value.current
		}
	}
	return d.current[key]
}

func (d *selectedDispatch) originLocked(key advertiseKey, id originIdentity, scope originScope) *originContribution {
	return d.origins[key][id][scope]
}

func (d *selectedDispatch) originPreferenceLocked(route *selectedScopedRoute) locrib.Preference {
	return locrib.PreferenceFor(route.preference, func(protocol string) (uint8, bool) {
		value, found := d.distances[protocol]
		return value, found
	})
}

// electOriginLocked sees exactly one learned BGP winner. It never reads received
// candidates and never evaluates export. Equal rank preserves admission order.
func (d *selectedDispatch) electOriginLocked(scope originScope, key advertiseKey) *selectedScopedRoute {
	best := d.current[key]
	for _, members := range d.origins[key] {
		origin := members[scope]
		if origin == nil {
			continue
		}
		candidate := origin.wire
		if best == nil {
			best = candidate
			continue
		}
		rank := d.originPreferenceLocked(candidate)
		previous := d.originPreferenceLocked(best)
		if rank.Before(previous) {
			best = candidate
			continue
		}
		if previous.Before(rank) {
			continue
		}
		if candidate.order < best.order {
			best = candidate
		}
	}
	return best
}

// reelectOriginLocked publishes a winner or preference-generation renewal. Its immutable
// scoped revision is read by dispatch, refresh and CurrentSelection together.
func (d *selectedDispatch) reelectOriginLocked(scope originScope, key advertiseKey, renew bool) error {
	previous, scoped := d.scoped[scope][key]
	if scoped && !d.scopeHasOriginLocked(scope, key) && !d.scopeHasDestinationLocked(scope) {
		d.clearOriginSelectionLocked(scope, key)
		return nil
	}
	winner := d.electOriginLocked(scope, key)
	if !renew {
		if scoped {
			if previous.candidate == winner {
				return nil
			}
		} else if winner == d.current[key] {
			return nil
		}
	}
	if err := d.advanceLocked(); err != nil {
		return err
	}
	if winner == nil {
		d.clearOriginSelectionLocked(scope, key)
	} else {
		next := *winner
		next.wire.Revision = uint64(d.cut)
		if d.scoped[scope] == nil {
			d.scoped[scope] = make(map[advertiseKey]originScopedWinner)
		}
		d.scoped[scope][key] = originScopedWinner{candidate: winner, current: &next}
	}
	d.queueOriginKeyLocked(scope, key)
	return nil
}

func (d *selectedDispatch) clearOriginSelectionLocked(scope originScope, key advertiseKey) {
	delete(d.scoped[scope], key)
	if len(d.scoped[scope]) == 0 {
		delete(d.scoped, scope)
	}
}

func (d *selectedDispatch) scopeHasOriginLocked(scope originScope, key advertiseKey) bool {
	for _, members := range d.origins[key] {
		if members[scope] != nil {
			return true
		}
	}
	return false
}

func (d *selectedDispatch) scopeHasDestinationLocked(scope originScope) bool {
	for id, destination := range d.destinations {
		if id.peer == scope.peer && destination.owner == scope.owner {
			return true
		}
	}
	return false
}

// An active Session may still need a scoped fallback's newer revision. After
// the last writer joins, later Sessions can use the common learned revision.
func (d *selectedDispatch) pruneDisconnectedOriginsLocked(scope originScope) {
	if len(d.scoped[scope]) == 0 || d.scopeHasDestinationLocked(scope) {
		return
	}
	for key := range d.scoped[scope] {
		if !d.scopeHasOriginLocked(scope, key) {
			d.clearOriginSelectionLocked(scope, key)
		}
	}
}

func (d *selectedDispatch) queueOriginKeyLocked(scope originScope, key advertiseKey) {
	for _, destination := range d.destinations {
		if destination.owner != scope.owner {
			continue
		}
		if destination.peer != scope.peer {
			continue
		}
		d.queueSelectedKeyLocked(destination, key)
	}
}

func (d *selectedDispatch) queueSelectedKeyLocked(destination *selectedDestination, key advertiseKey) {
	if current := d.currentForLocked(destination, key); current != nil {
		ack := destination.acknowledged[key]
		if ack.revision == scopedRevision(current.wire.Revision) {
			if !ack.uncertain {
				delete(destination.pending, key)
				return
			}
		}
		destination.pending[key] = struct{}{}
		return
	}
	if destination.acknowledged[key].possible {
		destination.pending[key] = struct{}{}
		return
	}
	delete(destination.acknowledged, key)
	delete(destination.pending, key)
}

func (d *selectedDispatch) originScopesLocked(key advertiseKey) map[originScope]struct{} {
	var scopes map[originScope]struct{}
	for scope, keys := range d.scoped {
		if _, found := keys[key]; found {
			if scopes == nil {
				scopes = make(map[originScope]struct{})
			}
			scopes[scope] = struct{}{}
		}
	}
	for _, members := range d.origins[key] {
		for scope := range members {
			if scopes == nil {
				scopes = make(map[originScope]struct{})
			}
			scopes[scope] = struct{}{}
		}
	}
	return scopes
}

// originIntake is registered by the real RIB startup on the independent SDK
// mutation worker. It holds d.mu only for publication, never engine I/O or joins.
// A cancellation removes only its own issued generation and cancels its accepted
// sender calls. Completion requires their join and current output confirmation;
// a bounded failure reports unresolved obligations without clearing the frontier.
func (d *selectedDispatch) originIntake(ctx context.Context, input rpc.OriginIntakeInput) (*rpc.UpdateRouteOutput, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	value, key, id, err := newOriginContribution(input)
	if err != nil {
		return nil, err
	}
	peer, err := netip.ParseAddr(input.Destination)
	if err != nil {
		return nil, err
	}
	scope := originScope{peer: peer, owner: input.PeerOwner}
	d.mu.Lock()
	if d.fatal != nil {
		err := d.fatal
		d.mu.Unlock()
		return nil, err
	}
	if d.stopped {
		d.mu.Unlock()
		return nil, errors.New("origin provider has stopped; reconnect its deciding process")
	}
	if err := ctx.Err(); err != nil {
		d.mu.Unlock()
		return nil, err
	}
	old := d.originLocked(key, id, scope)
	if input.Operation == rpc.OriginIntakeWithdraw {
		writes, joinError := d.cancelOriginWritesLocked(scope, input.OriginReceipt)
		if old != nil {
			if old.input.OriginReceipt == input.OriginReceipt {
				err = d.removeOriginLocked(key, id, scope, input.OriginReceipt)
			}
		}
		d.signalLocked()
		d.mu.Unlock()
		if joinError != nil {
			return nil, errors.Join(err, joinError)
		}
		if err != nil {
			return nil, err
		}
		if err := d.joinOriginWrites(ctx, writes); err != nil {
			return nil, err
		}
		if err := d.originBarrier(ctx, scope, key); err != nil {
			return nil, err
		}
		return &rpc.UpdateRouteOutput{Withdrawn: 1}, nil
	}
	if old != nil {
		if old.input.OriginReceipt > input.OriginReceipt {
			d.mu.Unlock()
			return nil, errors.New("origin generation was superseded; use the current issued contribution")
		}
		if old.input.OriginReceipt == input.OriginReceipt {
			// The issuer owns one immutable original value for this receipt.
			before := d.originAcknowledgmentLocked(scope, key)
			d.mu.Unlock()
			return d.originResult(scope, key, id, input, before)
		}
		value.order = old.order
	} else {
		if d.admission == math.MaxUint64 {
			d.mu.Unlock()
			return nil, errors.New("origin admission order exhausted; restart the deciding process")
		}
		d.admission++
		value.order = d.admission
	}
	value.wire.order = value.order
	before := d.originAcknowledgmentLocked(scope, key)
	if d.origins[key] == nil {
		d.origins[key] = make(map[originIdentity]map[originScope]*originContribution)
	}
	if d.origins[key][id] == nil {
		d.origins[key][id] = make(map[originScope]*originContribution)
	}
	d.origins[key][id][scope] = value
	if err = d.reelectOriginLocked(scope, key, false); err != nil {
		d.mu.Unlock()
		return nil, err
	}
	d.signalLocked()
	d.mu.Unlock()
	err = d.originBarrier(ctx, scope, key)
	if err == nil {
		err = ctx.Err()
	}
	if err != nil {
		d.mu.Lock()
		writes, joinError := d.cancelOriginWritesLocked(scope, input.OriginReceipt)
		cleanup := d.removeOriginLocked(key, id, scope, input.OriginReceipt)
		d.signalLocked()
		d.mu.Unlock()
		settlement, cancel := d.originCompensationContext(ctx)
		defer cancel()
		if joinError == nil {
			joinError = d.joinOriginWrites(settlement, writes)
		}
		if joinError != nil {
			return nil, errors.Join(err, cleanup, joinError)
		}
		joined := d.originBarrier(settlement, scope, key)
		return nil, errors.Join(err, cleanup, joined)
	}
	return d.originResult(scope, key, id, input, before)
}

func (d *selectedDispatch) originAcknowledgmentLocked(scope originScope, key advertiseKey) scopedRevision {
	var revision scopedRevision
	for _, destination := range d.destinations {
		if destination.owner != scope.owner {
			continue
		}
		if destination.peer != scope.peer {
			continue
		}
		revision = max(revision, destination.acknowledged[key].revision)
	}
	return revision
}

func (d *selectedDispatch) originResult(scope originScope, key advertiseKey, id originIdentity, input rpc.OriginIntakeInput, before scopedRevision) (*rpc.UpdateRouteOutput, error) {
	d.mu.Lock()
	defer d.mu.Unlock()
	retained := d.originLocked(key, id, scope)
	if retained == nil {
		return nil, errors.New("origin was removed before completion; retry from its current producer state")
	}
	if retained.input.OriginReceipt != input.OriginReceipt {
		return nil, errors.New("origin was replaced before completion; use the current issued generation")
	}
	disposition := rpc.RouteAdmissionSuppressedSelection
	var count uint32
	selected := d.currentScopeLocked(scope, key)
	if selected != nil {
		if selected.wire.OriginReceipt == input.OriginReceipt {
			disposition = rpc.RouteAdmissionSelected
			written, suppressed := false, false
			for _, destination := range d.destinations {
				if destination.owner != scope.owner {
					continue
				}
				if destination.peer != scope.peer {
					continue
				}
				ack := destination.acknowledged[key]
				if ack.revision != scopedRevision(selected.wire.Revision) {
					continue
				}
				if ack.possible {
					written = true
					if ack.revision > before {
						count = 1
					}
				} else {
					suppressed = true
				}
			}
			if !written {
				if suppressed {
					disposition = rpc.RouteAdmissionSuppressedExport
				}
			}
		}
	}
	return &rpc.UpdateRouteOutput{Announced: count, Admissions: []rpc.RouteAdmission{{Family: input.Route.Family,
		KeyNLRI: slices.Clone(retained.input.Route.KeyNLRI), Destination: input.Destination, PeerOwner: input.PeerOwner,
		Generation: input.OriginReceipt, Outcome: rpc.RouteAdmissionRetained, Disposition: disposition}}}, nil
}

func (d *selectedDispatch) removeOriginLocked(key advertiseKey, id originIdentity, scope originScope, receipt uint64) error {
	current := d.originLocked(key, id, scope)
	if current == nil {
		return nil
	}
	if current.input.OriginReceipt != receipt {
		return nil
	}
	delete(d.origins[key][id], scope)
	if len(d.origins[key][id]) == 0 {
		delete(d.origins[key], id)
	}
	if len(d.origins[key]) == 0 {
		delete(d.origins, key)
	}
	return d.reelectOriginLocked(scope, key, false)
}

func (d *selectedDispatch) originBarrier(ctx context.Context, scope originScope, key advertiseKey) error {
	for {
		d.mu.Lock()
		if d.fatal != nil {
			err := d.fatal
			d.mu.Unlock()
			return err
		}
		if err := ctx.Err(); err != nil {
			d.mu.Unlock()
			return err
		}
		var unresolved []selectedUnsettled
		refused := false
		for id, destination := range d.destinations {
			if destination.owner != scope.owner {
				continue
			}
			if destination.peer != scope.peer {
				continue
			}
			waiting := false
			for _, item := range destination.inFlight {
				if item.key == key {
					waiting = true
					break
				}
			}
			inFlight := waiting
			current := d.currentForLocked(destination, key)
			if !waiting {
				if current != nil {
					ack := destination.acknowledged[key]
					waiting = ack.revision != scopedRevision(current.wire.Revision)
					if ack.uncertain {
						waiting = true
					}
				} else {
					waiting = destination.acknowledged[key].possible
				}
			}
			if !waiting {
				continue
			}
			cause := destination.failure
			ack := destination.acknowledged[key]
			if !inFlight {
				if ack.failure != nil {
					failedCurrent := false
					if current != nil {
						failedCurrent = ack.failure.revision == scopedRevision(current.wire.Revision)
					}
					if current == nil {
						failedCurrent = ack.failure.kind == rpc.SelectedRouteAbsent
					}
					if failedCurrent {
						refused = true
						cause = ack.failure.cause
					}
				}
			}
			if cause == nil {
				cause = errors.New("origin selection is queued or in flight")
			}
			unresolved = append(unresolved, selectedUnsettled{id: id, cause: cause})
		}
		changed := d.changed
		d.mu.Unlock()
		if len(unresolved) == 0 {
			return nil
		}
		if refused {
			return selectedBarrierError(errors.New("origin current output was refused; reconciliation remains owed"), unresolved)
		}
		select {
		case <-changed:
		case <-ctx.Done():
			return selectedBarrierError(ctx.Err(), unresolved)
		}
	}
}

// retireOriginOwner runs after the exact Session worker joined permanent DOWN.
// It owns no I/O and is safe under peerMu. Physical DOWN MUST NOT call it.
// No historical owner/key remains after its last current membership is removed.
func (d *selectedDispatch) retireOriginOwner(owner uint64) {
	if d == nil {
		return
	}
	d.mu.Lock()
	defer d.mu.Unlock()
	for key, origins := range d.origins {
		for id, members := range origins {
			for scope := range members {
				if scope.owner != owner {
					continue
				}
				delete(members, scope)
			}
			if len(members) == 0 {
				delete(origins, id)
			}
		}
		if len(origins) == 0 {
			delete(d.origins, key)
		}
	}
	for scope := range d.scoped {
		if scope.owner == owner {
			delete(d.scoped, scope)
		}
	}
	d.signalLocked()
}

// originWriteJoin holds one exact admitted sender call, never a replacement or
// route history. The existing destination worker owns and closes done only after
// its send/receipt call returns. Physical engine writes remain issuer-owned;
// an unsettled transport result keeps the acknowledgment uncertain until C2's
// final-write revocation join or a later current writer receipt resolves it.
type originWriteJoin struct {
	id   selectedDestinationID
	done <-chan struct{}
}

func (d *selectedDispatch) cancelOriginWritesLocked(scope originScope, receipt uint64) ([]originWriteJoin, error) {
	var writes []originWriteJoin
	for id, destination := range d.destinations {
		if destination.owner != scope.owner {
			continue
		}
		if destination.peer != scope.peer {
			continue
		}
		for _, item := range destination.inFlight {
			if item.route.OriginReceipt != receipt {
				continue
			}
			if destination.batchDone == nil {
				return writes, errors.New("origin accepted call has no completion receipt; restart its selecting process")
			}
			writes = append(writes, originWriteJoin{id: id, done: destination.batchDone})
			destination.cancelBatch()
			break
		}
	}
	return writes, nil
}

func (d *selectedDispatch) joinOriginWrites(ctx context.Context, writes []originWriteJoin) error {
	for i, write := range writes {
		select {
		case <-write.done:
		case <-ctx.Done():
			unresolved := make([]selectedUnsettled, 0, len(writes)-i)
			for _, pending := range writes[i:] {
				select {
				case <-pending.done:
					continue
				default:
				}
				unresolved = append(unresolved, selectedUnsettled{id: pending.id, cause: errors.New("accepted origin sender call did not join")})
			}
			if len(unresolved) == 0 {
				return nil
			}
			return selectedBarrierError(ctx.Err(), unresolved)
		}
	}
	return nil
}

// originCompensationContext is only for failed/canceled ANNOUNCE rollback.
// A still-live caller retains its deadline and cancellation. An already canceled
// announce gets a bounded cancellation-join budget under the component lifetime.
// WITHDRAW never uses this helper: B/C2's fresh cleanup context stays authoritative.
func (d *selectedDispatch) originCompensationContext(ctx context.Context) (context.Context, context.CancelFunc) {
	if ctx.Err() == nil {
		return context.WithTimeout(ctx, rpc.RequestCancelTimeout)
	}
	return context.WithTimeout(d.ctx, rpc.RequestCancelTimeout)
}
