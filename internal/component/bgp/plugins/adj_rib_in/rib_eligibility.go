// Design: docs/architecture/plugin/rib-storage-design.md -- receive validation gate
// RFC: rfc/short/draft-ietf-sidrops-aspa-verification.md -- Section 5.7 retained ineligible paths
package adj_rib_in

import (
	"context"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"time"
	"unique"

	"github.com/ze-software/ze/internal/core/bgp/nlri/nlrisplit"
	"github.com/ze-software/ze/internal/core/bgp/ribevents"
	"github.com/ze-software/ze/internal/core/textbuf"
	"github.com/ze-software/ze/pkg/plugin/rpc"
)

// validationEmitTimeout bounds one emitted validation frame: the engine
// enqueues it to each subscriber and answers, so only a dead engine connection
// reaches it. It is per frame, never per hand-off: a whole table is many
// frames, and a live engine acknowledging each one in turn is never cut off.
const validationEmitTimeout = 30 * time.Second

// validationRouteKey is the store key of the route a selection query names:
// the prefix for a family keyed by CIDR, and the family's route identity,
// decoded from route.NLRI the way routeKeyFromNLRI decodes a received NLRI,
// for every other. ok is false when the query names no route of its family
// (no NLRI, or one the family's key function refuses), so the caller answers
// for a route it could not find rather than for one it found.
func validationRouteKey(route ribevents.ValidationRoute) (key compactRouteKey, ok bool) {
	if isSimplePrefixFamily(route.Family) {
		return compactRouteKey{Fam: route.Family, Prefix: route.Prefix, PathID: route.PathID}, true
	}
	if nlrisplit.KeysByCIDR(route.Family) {
		return compactRouteKey{Fam: route.Family, Prefix: route.Prefix, PathID: route.PathID}, true
	}
	if route.NLRI == "" {
		return compactRouteKey{}, false
	}
	var scratch [nlrisplit.PrefixKeyScratchSize]byte
	identity, err := nlrisplit.GetPrefixKey(route.Family)([]byte(route.NLRI), scratch[:], false)
	if err != nil {
		return compactRouteKey{}, false
	}
	return compactRouteKey{Fam: route.Family, PathID: route.PathID, NLRI: unique.Make(string(identity))}, true
}

// routeEligible is the selection query, not a second copy of validation state.
// A route the store does not hold, or a query that names no route, is not
// eligible: the answer is never assumed for a family the store keys otherwise.
//
// FlowSpec is the one family this store has no verdict for: it never stores a
// FlowSpec rule (a rule carries no next hop, so installStructuredNLRIs skips
// it), and ribevents.RouteEligible asks the FlowSpec provider, bgp-rib, first.
// That provider applies the source-policy denial to FlowSpec authorization
// (rib-storage-design.md, "Source-policy lifecycle owners").
func (r *AdjRIBInManager) routeEligible(key ribevents.ValidationRoute, msgID uint64) bool {
	if key.PeerOwner == 0 {
		return false
	}
	if key.ConnectionID == 0 {
		return false
	}
	r.mu.RLock()
	defer r.mu.RUnlock()
	if r.terminal != nil {
		return false
	}
	// refuse-by-default: guards ribevents.RouteEligible -- the FlowSpec provider already refused every rule it did not authorize, and this store holds none
	if ribevents.IsFlowSpec(key.Family) {
		return true
	}
	rk, named := validationRouteKey(key)
	if !named {
		return false
	}
	if pending := r.pending[pendingKey(receivedPeerOf(key.Peer, key.PeerOwner, key.ConnectionID), rk)]; pending != nil {
		if pending.route.validationSourceMatches(key) {
			return false
		}
	}
	routes := r.ribIn[receivedPeerOf(key.Peer, key.PeerOwner, key.ConnectionID)]
	if routes == nil {
		return false
	}
	route, ok := routes.Get(rk)
	if !ok {
		return false
	}
	if route.SourceOwner != key.PeerOwner {
		return false
	}
	if route.ConnectionID != key.ConnectionID {
		return false
	}
	// RFC 7947 Section 2.1: an import-filtered entry is retained in Adj-RIB-In
	// only and is never eligible for selection or advertisement, regardless of
	// validation decisions.
	if route.ImportFiltered {
		return false
	}
	// RFC 8212 Section 3: an applied import denial excludes the route.
	if r.sourcePolicyDeniedLocked(route.SourceOwner, route.SourcePolicyEpoch) {
		return false
	}
	if newer := r.earlyDecisions[pendingKey(receivedPeerOf(key.Peer, key.PeerOwner, key.ConnectionID), rk)]; newer != nil && receivedBefore(route.ConnectionID, route.MsgID, newer.connectionID, newer.msgID) {
		return false
	}
	if r.validationEnabled && route.Ineligible {
		return false
	}
	if msgID != 0 {
		return route.MsgID == msgID
	}
	return true
}

// routePresent keeps regenerated path identities owned while validation only
// suppresses advertisement. Cache eviction may query it; no cache operation or
// external callback may run under this receive-store lock.
func (r *AdjRIBInManager) routePresent(key ribevents.ValidationRoute) bool {
	if key.PeerOwner == 0 {
		return false
	}
	if key.ConnectionID == 0 {
		return false
	}
	rk, named := validationRouteKey(key)
	if !named {
		return false
	}
	r.mu.RLock()
	defer r.mu.RUnlock()
	if pending := r.pending[pendingKey(receivedPeerOf(key.Peer, key.PeerOwner, key.ConnectionID), rk)]; pending != nil {
		if pending.route.validationSourceMatches(key) {
			return true
		}
	}
	if routes := r.ribIn[receivedPeerOf(key.Peer, key.PeerOwner, key.ConnectionID)]; routes != nil {
		route, present := routes.Get(rk)
		if present {
			return route.validationSourceMatches(key)
		}
	}
	return false
}

// validationSourceMatches compares immutable received provenance. A caller MUST
// already have refused an unrecorded query; this method grants no authority.
func (route *RawRoute) validationSourceMatches(key ribevents.ValidationRoute) bool {
	if route.SourceOwner != key.PeerOwner {
		return false
	}
	return route.ConnectionID == key.ConnectionID
}

// noteValidationChange MUST run under mu. The caller MUST finish with a
// validation unlock helper, or unlock mu then flushValidationChanges; simply
// unlocking would leave downstream selection unaware of its owed change.
//
// Nothing is noted until validation enables the eligibility gate (in memory
// in the daemon, through eligibilityCommand when forked). Before that,
// ribevents.RouteEligible answers true for every path, so the live forward
// already carried each route and no verdict changed. A ValidationChange then
// makes the route server and reflector replay the path to every target, which
// sends each UPDATE a second time and re-announces a route the next UPDATE
// withdraws.
//
// A route keyed by its NLRI identity is named by its native NLRI, the form
// validationRouteKey decodes, so a subscriber that asks the selection query
// with the notified key finds the route. The caller MUST note such a route
// while it is still stored: a removal notes before it deletes.
func (r *AdjRIBInManager) noteValidationChange(peer receivedPeer, key compactRouteKey) {
	if r.emitValidation == nil {
		return
	}
	if !r.validationGate {
		return
	}
	route := r.validationStoredRouteLocked(peer, key)
	if route == nil {
		return
	}
	change := ribevents.ValidationRoute{Peer: peer.address, PeerOwner: route.SourceOwner, ConnectionID: route.ConnectionID, Family: key.Fam, Prefix: key.Prefix, PathID: key.PathID}
	if key.keyedByNLRI() {
		nlri, named := r.validationNLRILocked(peer, key)
		if !named {
			return
		}
		change.NLRI = nlri
	}
	if r.validationOwed != nil {
		if _, owed := r.validationOwed[change]; owed {
			return
		}
		r.validationOwed[change] = struct{}{}
	}
	r.validationChanges = append(r.validationChanges, change)
}

// validationNLRILocked is the native NLRI, without ADD-PATH, of the route key
// names: the bytes the stored or pending route carries. named is false when
// this store holds no such route. Its eligibility was then false and stays
// false, since routeEligible refuses a key it cannot find, so nothing changed
// to announce. It is also false, and logged, when the stored bytes do not
// decode back to key: a ValidationRoute no subscriber could resolve is never
// emitted. MUST hold mu.
func (r *AdjRIBInManager) validationNLRILocked(peer receivedPeer, key compactRouteKey) (nlri string, named bool) {
	route := r.validationStoredRouteLocked(peer, key)
	if route == nil {
		return "", false
	}
	wire, err := hex.DecodeString(route.NLRIHex)
	if err != nil {
		logger().Error("validation change not announced: the stored route's NLRI is not hex, so no subscriber could find the route; this is a bgp-adj-rib-in defect, report it with this line",
			"peer", peer.address, "peer-owner", peer.owner, "family", key.Fam, "route", key.routeName(), "error", err)
		return "", false
	}
	change := ribevents.ValidationRoute{Peer: peer.address, PeerOwner: route.SourceOwner, ConnectionID: route.ConnectionID, Family: key.Fam, PathID: key.PathID, NLRI: string(wire)}
	resolved, ok := validationRouteKey(change)
	if !ok {
		logger().Error("validation change not announced: the stored route's NLRI does not parse as a store key, so no subscriber could find the route; this is a bgp-adj-rib-in defect, report it with this line",
			"peer", peer.address, "peer-owner", peer.owner, "family", key.Fam, "route", key.routeName(), "nlri", route.NLRIHex)
		return "", false
	}
	if resolved != key {
		logger().Error("validation change not announced: the stored route's NLRI does not name its own store key, so no subscriber could find the route; this is a bgp-adj-rib-in defect, report it with this line",
			"peer", peer.address, "peer-owner", peer.owner, "family", key.Fam, "route", key.routeName(), "nlri", route.NLRIHex)
		return "", false
	}
	return change.NLRI, true
}

// validationStoredRouteLocked borrows the original pending or installed value.
// A nil result means neither store holds that path. The caller MUST hold mu
// through every read of the returned route.
func (r *AdjRIBInManager) validationStoredRouteLocked(peer receivedPeer, key compactRouteKey) *RawRoute {
	if pending := r.pending[pendingKey(peer, key)]; pending != nil {
		return pending.route
	}
	if routes := r.ribIn[peer]; routes != nil {
		route, _ := routes.Get(key)
		return route
	}
	return nil
}

// unlockValidation follows ordinary receive mutations that note validation
// changes. Commands MUST use their error-returning validation unlock instead.
// Subscribers read current state; no store lock crosses the event bus.
func (r *AdjRIBInManager) unlockValidation() {
	r.mu.Unlock()
	if err := r.flushValidationChanges(false); err != nil {
		r.failValidationOwner(err)
		logger().Error("receive store stopped because validation publication failed; ze must stop its peers before restarting", "error", err)
	}
}

// unlockValidationCommand MUST replace unlockValidation in a command that
// mutates eligibility. The command MUST not report success before its owed
// notifications reach the engine. It may explicitly retry an earlier failure.
func (r *AdjRIBInManager) unlockValidationCommand(status *string, data *any, result *error) { //nolint:gocritic // ptrToRefParam: deferred publication failure must update the caller's named error result.
	r.mu.Unlock()
	if err := r.flushValidationChanges(true); err != nil {
		*status = statusError
		*data = ""
		*result = errors.Join(*result, err)
	}
}

// unlockValidationResult is the typed validation command's matching unlock.
// Its caller MUST return the error this writes rather than an OK receipt.
func (r *AdjRIBInManager) unlockValidationResult(result *error) { //nolint:gocritic // ptrToRefParam: deferred publication failure must update the caller's named error result.
	r.mu.Unlock()
	*result = errors.Join(*result, r.flushValidationChanges(true))
}

// flushValidationChanges never holds the receive-store lock across emission.
// One failed hand-off pauses automatic attempts; explicit commands may retry
// the unacknowledged keys, whose consumers always read current state. A timeout
// is ambiguous, so this does not promise exactly-once delivery after a retry.
func (r *AdjRIBInManager) flushValidationChanges(retry bool) error {
	r.validationEmitMu.Lock()
	defer r.validationEmitMu.Unlock()
	if !retry && r.validationEmitErr != nil {
		return r.validationEmitErr
	}
	r.mu.Lock()
	changes := r.validationChanges
	r.validationChanges = nil
	r.validationOwed = nil
	r.mu.Unlock()
	if len(changes) == 0 {
		r.validationEmitErr = nil
		return nil
	}
	err := r.emitValidation(changes)
	if err == nil {
		r.validationEmitErr = nil
		return nil
	}
	if partial, ok := errors.AsType[*validationEmissionError](err); ok {
		clear(changes[:partial.acknowledged])
		changes = changes[partial.acknowledged:]
	}
	r.mu.Lock()
	newer := r.validationChanges
	r.validationChanges = nil
	r.validationOwed = make(map[ribevents.ValidationRoute]struct{}, len(changes)+len(newer))
	for _, batch := range [][]ribevents.ValidationRoute{changes, newer} {
		for _, route := range batch {
			if _, held := r.validationOwed[route]; held {
				continue
			}
			r.validationOwed[route] = struct{}{}
			r.validationChanges = append(r.validationChanges, route)
		}
	}
	r.mu.Unlock()
	r.validationEmitErr = err
	return err
}

// validationEmissionError records the acknowledged prefix only. The failed
// chunk and the untouched suffix remain owed; no acknowledged chunk is retried.
type validationEmissionError struct {
	acknowledged int
	cause        error
}

func (err *validationEmissionError) Error() string { return err.cause.Error() }
func (err *validationEmissionError) Unwrap() error { return err.cause }

// emitValidationEvent hands ordered canonical arrays to the engine through the
// SDK on both transports. Small batches keep one ordinary Marshal; large ones
// call each existing route MarshalJSON once and transfer each chunk's ownership
// to its event string. DirectBridge may queue that string beyond its ACK. A
// frame carries at most ribevents.FrameEntriesMax routes, and each frame gets
// its own validationEmitTimeout.
func (r *AdjRIBInManager) emitValidationEvent(changes []ribevents.ValidationRoute) error {
	return r.emitValidationFrames(validationEmitTimeout, changes)
}

// emitValidationFrames is emitValidationEvent with its per-frame deadline
// named, so a test proves the bound without waiting validationEmitTimeout.
func (r *AdjRIBInManager) emitValidationFrames(frameTimeout time.Duration, changes []ribevents.ValidationRoute) error {
	if len(changes) == 0 {
		return nil
	}
	input := rpc.EmitEventInput{Namespace: ribevents.Namespace, EventType: ribevents.EventValidationChange}
	budget := input.CanonicalPayloadBudget()
	singleFrame := len(changes) == 1
	if !singleFrame && len(changes) <= ribevents.FrameEntriesMax {
		singleFrame = validationArrayFits(changes, budget)
	}
	if singleFrame {
		payload, err := json.Marshal(changes)
		if err != nil {
			return &validationEmissionError{cause: fmt.Errorf("encode validation-change: %w", err)}
		}
		return r.emitValidationChunk(frameTimeout, string(payload), 0)
	}
	start, end := 0, 0
	var pending []byte
	var payload textbuf.Buffer
	for start < len(changes) {
		if pending == nil {
			var err error
			pending, err = changes[end].MarshalJSON()
			if err != nil {
				return &validationEmissionError{acknowledged: start, cause: fmt.Errorf("encode validation-change: %w", err)}
			}
		}
		if len(pending) > rpc.MaxMessageSize-2 {
			return &validationEmissionError{acknowledged: start,
				cause: fmt.Errorf("validation route has %d canonical JSON bytes before RPC framing, beyond the %d-byte frame ceiling", len(pending), rpc.MaxMessageSize)}
		}
		// String transfers this backing allocation to the event. It MUST NOT
		// be borrowed with Slice and then reused: engine delivery is queued.
		payload.Reset(max(budget, len(pending)+2)).Byte('[')
		for end < len(changes) {
			if pending == nil {
				var err error
				pending, err = changes[end].MarshalJSON()
				if err != nil {
					return &validationEmissionError{acknowledged: start, cause: fmt.Errorf("encode validation-change: %w", err)}
				}
			}
			if end > start && len(pending)+2 > budget-payload.Len() {
				break
			}
			if end > start {
				payload.Byte(',')
			}
			if _, err := payload.Write(pending); err != nil {
				return &validationEmissionError{acknowledged: start, cause: err}
			}
			pending = nil
			end++
			if payload.Len()+1 > budget {
				break // An indivisible row goes to the actual writer alone.
			}
			if end-start == ribevents.FrameEntriesMax {
				break // The subscriber's work per frame stays bounded.
			}
		}
		payload.Byte(']')
		if err := r.emitValidationChunk(frameTimeout, payload.String(), start); err != nil {
			return err
		}
		start = end
	}
	return nil
}

func validationArrayFits(changes []ribevents.ValidationRoute, budget int) bool {
	used := 2 // Array brackets.
	for i := range changes {
		remaining := budget - used
		if i > 0 {
			remaining-- // Comma before this route.
		}
		size := changes[i].JSONSizeBound()
		if size > remaining {
			return false
		}
		used += size
		if i > 0 {
			used++
		}
	}
	return true
}

// emitValidationChunk hands one frame to the engine under a deadline of its own.
func (r *AdjRIBInManager) emitValidationChunk(frameTimeout time.Duration, event string, acknowledged int) error {
	ctx, cancel := context.WithTimeout(context.Background(), frameTimeout)
	defer cancel()
	if _, err := r.plugin.EmitEvent(ctx, ribevents.Namespace, ribevents.EventValidationChange, "", "", event); err != nil {
		return &validationEmissionError{acknowledged: acknowledged, cause: fmt.Errorf("emit validation-change: %w", err)}
	}
	return nil
}

// eligibilityCommand answers ribevents.EligibilityRequest from routeEligible
// and routePresent, for a process this store's lookup cannot reach in memory.
// The validation-gate announcement names it, so no reader names this plugin.
const eligibilityCommand = "request bgp adj-rib-in eligibility"

// eligibilityAnswerCommand serves eligibilityCommand: one argument, the
// compact JSON EligibilityRequest, bounded by ribevents.EligibilityBatchMax.
func (r *AdjRIBInManager) eligibilityAnswerCommand(args []string) (string, any, error) {
	if len(args) != 1 {
		return statusError, "", errors.New(eligibilityCommand + " requires one argument: the JSON eligibility request")
	}
	answer, err := ribevents.AnswerEligibility(args[0], r.routeEligible, r.routePresent, r.eligibilityProgress)
	if err != nil {
		return statusError, "", fmt.Errorf("%s: %w", eligibilityCommand, err)
	}
	return statusDone, answer, nil
}

// emitValidationGateEvent hands the gate's state to the engine as
// bgp-rib/validation-gate through the SDK, the path emitValidationEvent takes,
// so it reaches the daemon and every forked subscriber alike.
func (r *AdjRIBInManager) emitValidationGateEvent(state ribevents.ValidationGateState) error {
	payload, err := json.Marshal(state)
	if err != nil {
		return fmt.Errorf("encode validation-gate: %w", err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), validationEmitTimeout)
	defer cancel()
	if _, err := r.plugin.EmitEvent(ctx, ribevents.Namespace, ribevents.EventValidationGate, "", "", string(payload)); err != nil {
		return fmt.Errorf("emit validation-gate: %w", err)
	}
	return nil
}

// announcedGateCommand announces the state an enable-validation or
// disable-validation command left, once the command returned and so released
// the store lock. A command that failed announces nothing. An announcement
// that fails fails the command: the caller (bgp-rpki) then reports that
// validation is not in force, where a silent success would leave a process
// that cannot reach this store's lookup forwarding every route.
func (r *AdjRIBInManager) announcedGateCommand(status string, data any, err error) (string, any, error) {
	if err != nil {
		return status, data, err
	}
	if err := r.admitMutation(true); err != nil {
		return statusError, "", err
	}
	defer r.mutationMu.Unlock()
	// refuse-by-default: guards emitValidationGateEvent -- a nil announcer is a test manager with no engine, and no other process exists to tell
	if r.announceGate == nil {
		return status, data, nil
	}
	r.mu.RLock()
	state := ribevents.ValidationGateState{Enabled: r.validationEnabled, Command: eligibilityCommand}
	r.mu.RUnlock()
	if announceErr := r.announceGate(state); announceErr != nil {
		return statusError, "", announceErr
	}
	return status, data, nil
}

// removeInstalled also invalidates a replaced path before its replacement has
// a validation result. The old path must not remain selectable or replayable.
//
// The change is noted before the delete, while the route still supplies the
// NLRI that names it (noteValidationChange).
func (r *AdjRIBInManager) removeInstalled(peer receivedPeer, key compactRouteKey) {
	r.noteValidationChange(peer, key)
	if routes := r.ribIn[peer]; routes != nil {
		routes.Delete(key)
	}
}

func (r *AdjRIBInManager) removePeerInstalled(peer receivedPeer) {
	if routes := r.ribIn[peer]; routes != nil {
		routes.Range(func(key compactRouteKey, _ uint64, _ *RawRoute) bool {
			r.noteValidationChange(peer, key)
			return true
		})
	}
	delete(r.ribIn, peer)
	for key := range r.validationPeers {
		if key.source == peer {
			delete(r.validationPeers, key)
		}
	}
	if peer.owner != 0 {
		r.dropSourcePolicyLocked(peer.owner)
	}
}

// eligibilityProgress is a causal processing fact, never a route verdict or a
// source lease. The caller MUST still ask exact current membership and retake
// its own local actor revision before admitting any output.
func (r *AdjRIBInManager) eligibilityProgress(route ribevents.ValidationRoute, minimum uint64) (bool, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	if r.terminal != nil {
		return false, r.terminal
	}
	source, live := r.sourceLive[route.PeerOwner]
	if !live {
		return false, errors.New("eligibility original source is no longer live; reconcile its retained membership before cold adoption")
	}
	if source.address != route.Peer {
		return false, errors.New("eligibility minimum names another source address; preserve the original source")
	}
	if source.connectionID != route.ConnectionID {
		return false, errors.New("eligibility minimum names another physical connection; preserve the original source")
	}
	return source.processedMsgID >= minimum, nil
}
