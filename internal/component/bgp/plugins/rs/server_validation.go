// Design: docs/architecture/bgp/structural-forwarding.md -- receive validation and retained replay
// Related: server_forward.go -- per-source forwarding batches
package rs

import (
	"context"
	"encoding/hex"
	"slices"
	"strconv"
	"time"

	"github.com/ze-software/ze/internal/core/bgp/ribevents"
	"github.com/ze-software/ze/internal/core/family"
	"github.com/ze-software/ze/pkg/plugin/rpc"
)

// validationChanged only queues keys. The emitting Adj-RIB-In command handler
// cannot service a replay RPC until this callback returns, so forwarding MUST
// run on the existing source worker. That worker MUST flush its live batch first.
func (rs *routeServer) validationChanged(changes []ribevents.ValidationRoute) {
	if rs.stopping.Load() {
		return
	}
	for _, route := range changes {
		sourcePeer := route.Peer.String()
		key, err := newWorkerKey(sourcePeer, route.PeerOwner, route.ConnectionID)
		if err != nil {
			rs.warnLifecycle("validation has no physical source actor", "code", lifecycleDoctorCode, "fix", lifecycleDoctorFix, "error", err)
			continue
		}
		item := workItem{sourcePeer: sourcePeer, validation: route}
		if !rs.workers.Dispatch(key, item) {
			logger().Debug("validation replay discarded after worker shutdown", "peer", sourcePeer)
		}
	}
}

// processValidation asks the receive store for the current path, including a
// retained ineligible path or a withdrawal for a removed path. The reactor gates
// its generation again before export. The event's position never supplies a
// verdict, and recovery uses the stored attributes without another UPDATE.
//
// A FlowSpec relay can wait: see parkValidation.
func (rs *routeServer) processValidation(key workerKey, route ribevents.ValidationRoute) {
	rs.replayValidation(key, route, true)
}

// replayValidation is processValidation. mayPark false relays a FlowSpec rule
// even when its UPDATE has not reached this worker: releaseParkedValidations
// has already waited as long as it will.
func (rs *routeServer) replayValidation(key workerKey, route ribevents.ValidationRoute, mayPark bool) {
	rs.flushWorkerBatch(key)
	rs.mu.RLock()
	peer := rs.peers[key]
	if peer == nil {
		rs.mu.RUnlock()
		return
	}
	targets := rs.selectForwardTargets(nil, key, 0, map[family.Family]bool{route.Family: true})
	rs.mu.RUnlock()
	// A validation change is a live change. Its relay leaves
	// StoredRoute.InitialUpdate false, so the engine holds it behind the
	// peer-up replay of a destination still replaying, like a forwarded UPDATE
	// (flushBatch), and nothing here waits.
	if ribevents.IsFlowSpec(route.Family) {
		rs.relayFlowSpecValidation(key, route, targets, mayPark)
		return
	}
	// The store answers each path and this route server relays it, under the
	// `send [ update ]` grant the operator gave this route server and not the
	// store (ribevents.ReplayPathCommand).
	for _, target := range targets {
		ctx, cancel := context.WithTimeout(context.Background(), forwardCachedTimeout)
		_, err := ribevents.RelayReplayPath(ctx, rs.dispatchCommand, func(ctx context.Context, destination string, routes []rpc.StoredRoute) error {
			return rs.relayToTarget(ctx, target, routes)
		},
			target.Destination, key.address.String(), route.Family.String(), route.Prefix.String(), strconv.FormatUint(uint64(route.PathID), 10), route.PeerOwner, route.ConnectionID)
		cancel()
		if err != nil {
			logger().Error("validation replay failed", "source", key.address.String(), "target", target, "prefix", route.Prefix, "error", err)
		}
	}
}

// relayFlowSpecValidation relays the RIB's current path for a FlowSpec rule,
// or its withdrawal, to every target. A path whose UPDATE this worker has not
// handled yet is parked even with no target: the UPDATE reaches this route
// server after any target up now in progress took its cut, so replayFlowSpecs
// leaves the rule to this relay, and the release selects the targets again.
func (rs *routeServer) relayFlowSpecValidation(key workerKey, route ribevents.ValidationRoute, targets []rpc.ForwardTarget, mayPark bool) {
	path, present := ribevents.LookupFlowSpecPath(route)
	if present && mayPark && rs.parkValidation(key, route, path.MsgID) {
		return
	}
	if len(targets) == 0 {
		return
	}
	stored := rpc.StoredRoute{
		SourcePeer: key.address.String(), SourceOwner: key.owner, SourceConnectionID: key.connectionID, Family: route.Family.String(), PathID: route.PathID,
		NLRIHex: hex.EncodeToString([]byte(route.NLRI)), NLRIFraming: rpc.NLRIFramingPrefixOnly, Withdraw: true,
	}
	if present {
		stored.AttrHex = hex.EncodeToString(path.Attributes)
		stored.MsgID = path.MsgID
		stored.SourceReceipt = path.SourceReceipt
		stored.SourcePolicyEpoch = path.SourcePolicyEpoch
		stored.Withdraw = false
	}
	routes := []rpc.StoredRoute{stored}
	for _, target := range targets {
		ctx, cancel := context.WithTimeout(context.Background(), forwardCachedTimeout)
		err := rs.relayToTarget(ctx, target, routes)
		cancel()
		if err != nil {
			logger().Error("FlowSpec validation replay failed", "source", key.address.String(), "target", target, "error", err)
		}
	}
}

// parkedValidation is a FlowSpec ValidationChange that reached its source
// worker before the UPDATE the judged generation arrived in (msgID).
type parkedValidation struct {
	route ribevents.ValidationRoute
	msgID uint64
}

// parkValidation holds a FlowSpec relay until this worker has handled the
// UPDATE msgID, and reports whether it did. The RIB and the route server take
// the source's UPDATEs on independent deliveries, so the RIB can judge a rule
// and its ValidationChange can reach this worker before the UPDATEs the source
// sent ahead of the rule, or beside it, have been batched. The flush in
// replayValidation orders the relay only after UPDATEs the worker already
// holds, so relaying now sent the rule ahead of a unicast route sent before it.
//
// Per-source delivery is in order and MessageIDs grow, so once the worker has
// handled any UPDATE at or above msgID, msgID itself has been handled or will
// never come: markProcessed then releases the relay. A source that sends
// nothing more releases it at the worker's idle timeout
// (releaseParkedValidations), and a source that goes down drops it
// (dropParkedValidations). One entry per rule: a newer change for the same rule
// waits for the newer UPDATE, so the list is bounded by the source's rules.
// Runs on the source worker only.
func (rs *routeServer) parkValidation(key workerKey, route ribevents.ValidationRoute, msgID uint64) bool {
	batch := rs.workerBatch(key)
	if msgID <= batch.processed {
		return false
	}
	for i := range batch.parked {
		if batch.parked[i].route == route {
			batch.parked[i].msgID = max(batch.parked[i].msgID, msgID)
			return true
		}
	}
	batch.parked = append(batch.parked, parkedValidation{route: route, msgID: msgID})
	return true
}

// markProcessed records that the source worker has handled the UPDATE msgID,
// forwarded or not, and releases every parked relay it was holding for: each
// one flushes the live batch, then relays. Runs on the source worker only.
func (rs *routeServer) markProcessed(key workerKey, msgID uint64) {
	batch := rs.workerBatch(key)
	if msgID > batch.processed {
		batch.processed = msgID
	}
	for i := 0; i < len(batch.parked); {
		parked := batch.parked[i]
		if parked.msgID > batch.processed {
			i++
			continue
		}
		batch.parked = slices.Delete(batch.parked, i, i+1)
		rs.processValidation(key, parked.route)
	}
}

// releaseParkedValidations relays every parked FlowSpec rule of an idle source
// worker. The source sent nothing for the idle timeout, so the UPDATE each one
// waits for is not on its way: a route server started after it was received, or
// an UPDATE the engine did not deliver here. Withholding the rule would lose it
// for every target. Called by the worker pool on the worker goroutine (onIdle).
func (rs *routeServer) releaseParkedValidations(key workerKey) {
	val, loaded := rs.batches.Load(key)
	if !loaded {
		return
	}
	batch, ok := val.(*forwardBatch)
	if !ok {
		return
	}
	for len(batch.parked) > 0 {
		parked := batch.parked[0]
		batch.parked = slices.Delete(batch.parked, 0, 1)
		logger().Warn("FlowSpec rule relayed before its UPDATE was forwarded: the source sent nothing more within the worker idle timeout",
			"source-peer", key.address.String(), "msg-id", parked.msgID, "handled-msg-id", batch.processed)
		rs.replayValidation(key, parked.route, false)
	}
}

// dropParkedValidations discards the parked FlowSpec rules of a source worker
// that is closing, on the source's peer-down or on shutdown. The source's
// peer-down withdraws its routes, so a rule its targets never received is not
// relayed now. Called by the worker pool on the worker goroutine (onClose).
func (rs *routeServer) dropParkedValidations(key workerKey) {
	// The physical close callback runs once after the final cached flush. A
	// retained current path still owes its already-admitted validation replay;
	// currentStoredRouteLocked excludes unretained/removed predicates per route.
	rs.releaseParkedValidations(key)
	rs.batches.Delete(key)
}

// workerBatch returns the source worker's batch state, creating it on first use.
func (rs *routeServer) workerBatch(key workerKey) *forwardBatch {
	val, loaded := rs.batches.Load(key)
	if !loaded {
		val, _ = rs.batches.LoadOrStore(key, &forwardBatch{})
	}
	batch, ok := val.(*forwardBatch)
	if !ok {
		panic("BUG: rs.batches holds a value that is not a *forwardBatch")
	}
	return batch
}

// replayFlowSpecs supplies every peer-up snapshot from the mandatory RIB.
// The optional Adj-RIB-In excludes these families, preventing duplicate replay.
// Each reconstruction still crosses authorization and egress policies.
//
// It replays the paths at or below the destination's peer-up cut and leaves
// every later generation to the live rail, the same split Adj-RIB-In's replay
// makes for unicast (replay_cut.go). The live rail for a FlowSpec rule is the
// source worker's validation relay, and that relay runs only once the worker
// has handled the rule's UPDATE (parkValidation). A generation above the cut is
// one this route server took delivery of after the destination came up, so its
// worker handles it with the destination already a target, and the relay leaves
// behind the unicast UPDATEs the source sent before the rule. Replaying it here
// as well sent the rule twice and, worse, ahead of those unicast UPDATEs: the
// replay passes the destination's replay fence while the live unicast waits
// behind it. A path whose MessageID is unknown (0) is replayed: sent twice,
// never dropped.
func (rs *routeServer) replayFlowSpecs(parent context.Context, destinationKey workerKey, gen, cut uint64) error {
	destination := destinationKey.address.String()
	// A timed-out optional-store replay must not cancel this independent phase.
	ctx, cancel := context.WithTimeout(parent, updateRouteTimeout)
	defer cancel()
	for _, key := range ribevents.FlowSpecRoutes() {
		if key.PeerOwner == destinationKey.owner {
			continue
		}
		rs.mu.RLock()
		target := rs.peers[destinationKey]
		current := target != nil && target.Up && target.ReplayGen == gen
		supported := current && (target.Families == nil || target.SupportsFamily(key.Family))
		rs.mu.RUnlock()
		if !current {
			return nil
		}
		if !supported {
			continue
		}
		path, present := ribevents.LookupFlowSpecPath(key)
		if !present {
			continue
		}
		if path.MsgID != 0 && path.MsgID > cut {
			continue // the live rail's: relayed by the source worker (parkValidation)
		}
		route := rpc.StoredRoute{SourcePeer: key.Peer.String(), SourceOwner: key.PeerOwner, SourceConnectionID: key.ConnectionID, SourceReceipt: path.SourceReceipt, Family: key.Family.String(),
			PathID: key.PathID, MsgID: path.MsgID, NLRIFraming: rpc.NLRIFramingPrefixOnly,
			SourcePolicyEpoch: path.SourcePolicyEpoch,
			NLRIHex:           hex.EncodeToString([]byte(key.NLRI)), AttrHex: hex.EncodeToString(path.Attributes),
			InitialUpdate: true} // peer-up replay: passes the destination's replay fence
		if err := rs.relayReplayRoutes(ctx, destination, []rpc.StoredRoute{route}); err != nil {
			return err
		}
	}

	return nil
}

// askFlowSpecGate emits a gate-query, so the selecting RIB announces its
// flowspec-gate again for this plugin, whose subscription may have gone live
// after the RIB's first announcement (ribevents.EventGateQuery).
func (rs *routeServer) askFlowSpecGate() error {
	// refuse-by-default: guards ribevents.EventGateQuery -- a nil plugin is a test instance with no engine, and no RIB to ask
	if rs.plugin == nil {
		return nil
	}
	ctx, cancel := context.WithTimeout(context.Background(), flowSpecGateQueryTimeout)
	defer cancel()
	_, err := rs.plugin.EmitEvent(ctx, ribevents.Namespace, ribevents.EventGateQuery, "", "", "{}")
	return err
}

// flowSpecGateQueryTimeout bounds the gate-query askFlowSpecGate emits.
const flowSpecGateQueryTimeout = 5 * time.Second
