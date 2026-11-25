// Design: docs/architecture/bgp/structural-forwarding.md -- validation-triggered reflection
// RFC: rfc/short/rfc8955.md -- Section 6, unicast-triggered revalidation
// Related: source_policy.go -- each source-policy phase waits for the worker (settleValidation)
package rr

import (
	"context"
	"encoding/hex"
	"errors"
	"strconv"
	"sync"
	"time"

	"github.com/ze-software/ze/internal/core/bgp/ribevents"
	"github.com/ze-software/ze/pkg/plugin/rpc"
)

// validationQueue is the reflector's revalidation worker state: notifications
// coalesce by received path in pending while the worker performs RPC, and a
// settle barrier queued in settles is closed once the batch it joined has
// been relayed. wake holds at most one signal; done is closed when the worker
// returns. mu guards pending and settles. Safe for concurrent use.
type validationQueue struct {
	failed  error // Current queued obligation, guarded by mu; cleared only by a successful batch.
	mu      sync.Mutex
	pending map[ribevents.ValidationRoute]struct{}
	settles []chan struct{}
	wake    chan struct{}
	done    chan struct{}
}

// signal wakes the worker, or leaves the wake already pending to collect
// what the caller queued.
func (q *validationQueue) signal() {
	select {
	case q.wake <- struct{}{}:
	default:
		// A pending wake will collect the queued work as well.
	}
}

// startValidation MUST be paired with the returned stop function before closing
// the SDK connection, and MUST be called before the source-policy worker runs
// and before Run: it publishes the queue settleValidation and
// validationChanged read. Notifications coalesce by received path while the
// worker performs RPC.
func (rr *routeReflector) startValidation(parent context.Context) func() {
	ctx, cancel := context.WithCancel(parent)
	q := &validationQueue{
		pending: make(map[ribevents.ValidationRoute]struct{}),
		wake:    make(chan struct{}, 1),
		done:    make(chan struct{}),
	}
	rr.validation = q
	go rr.runValidation(ctx, q)
	return func() {
		cancel()
		<-q.done
	}
}

// validationChanged queues a bgp-rib/validation-change batch for the worker
// and returns. It MUST NOT wait for that worker: the worker's RPC may need the
// receive-store command handler that published the notification.
func (rr *routeReflector) validationChanged(routes []ribevents.ValidationRoute) {
	q := rr.validation
	if q == nil {
		logger().Warn("bgp-rr: validation-change before the revalidation worker started, dropped", "routes", len(routes))
		return
	}
	q.mu.Lock()
	for _, route := range routes {
		q.pending[route] = struct{}{}
	}
	q.mu.Unlock()
	q.signal()
}

// runValidation is the revalidation worker startValidation starts: it relays
// each coalesced batch, then closes the settle barriers queued with it. It
// returns when ctx ends, closing q.done.
func (rr *routeReflector) runValidation(ctx context.Context, q *validationQueue) {
	defer close(q.done)
	for {
		select {
		case <-ctx.Done():
			return
		case <-q.wake:
			q.mu.Lock()
			routes := make([]ribevents.ValidationRoute, 0, len(q.pending))
			for route := range q.pending {
				routes = append(routes, route)
			}
			clear(q.pending)
			settles := q.settles
			q.settles = nil
			q.mu.Unlock()
			failed := rr.unsettledWork()
			if failed == nil {
				failed = rr.retryPeerReplays(ctx)
			}
			if failed == nil {
				failed = rr.replayValidation(ctx, routes)
			}
			q.mu.Lock()
			q.failed = failed
			if failed != nil {
				for _, route := range routes {
					q.pending[route] = struct{}{}
				}
			}
			q.mu.Unlock()
			for _, settle := range settles {
				close(settle)
			}
		}
	}
}

// settleValidation returns once every revalidation the worker held or was
// relaying when it was called has been relayed: a ValidationChange the
// receive store published is queued for the worker, so an event drain does
// not settle it. It refuses when the worker stopped or ctx ended first. One
// source-policy phase runs at a time, so at most one barrier waits.
func (rr *routeReflector) settleValidation(ctx context.Context) error {
	q := rr.validation
	// refuse-by-default: guards startValidation -- before it runs no worker exists, so no revalidation can be queued
	if q == nil {
		return nil
	}
	settle := make(chan struct{})
	q.mu.Lock()
	q.settles = append(q.settles, settle)
	q.mu.Unlock()
	q.signal()
	select {
	case <-settle:
		q.mu.Lock()
		failed := q.failed
		q.mu.Unlock()
		if failed != nil {
			return failed
		}
		return rr.unsettledWork()
	case <-q.done:
		return errors.New("the revalidation worker stopped before it relayed its queued paths")
	case <-ctx.Done():
		return errors.New("its revalidation relays did not reach the engine before the source-policy deadline: " +
			ctx.Err().Error())
	}
}

func (rr *routeReflector) replayValidation(ctx context.Context, routes []ribevents.ValidationRoute) error {
	var failed error
	for _, key := range routes {
		if !key.Prefix.IsValid() && !ribevents.IsFlowSpec(key.Family) {
			continue
		}
		source, sourceErr := newPeerIdentity(key.Peer.String(), key.PeerOwner, key.ConnectionID)
		if sourceErr != nil {
			return sourceErr
		}
		targets := rr.targets(source, key.Family)
		for _, target := range targets {
			if key.Prefix.IsValid() {
				// The store answers the path and this reflector relays it, under
				// the `send [ update ]` grant the operator gave this reflector
				// and not the store (ribevents.ReplayPathCommand).
				callCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
				_, err := ribevents.RelayReplayPath(callCtx, rr.dispatchCommand, func(ctx context.Context, destination string, routes []rpc.StoredRoute) error {
					return rr.relayToPeer(ctx, target, destination, routes)
				},
					target.identity.address.String(), key.Peer.String(), key.Family.String(), key.Prefix.String(),
					strconv.FormatUint(uint64(key.PathID), 10), key.PeerOwner, key.ConnectionID)
				cancel()
				if err != nil {
					if failed == nil {
						failed = err
					}
				}
				continue
			}
			stored := rpc.StoredRoute{SourcePeer: key.Peer.String(), Family: key.Family.String(), PathID: key.PathID,
				SourceOwner: key.PeerOwner, SourceConnectionID: key.ConnectionID,
				NLRIHex: hex.EncodeToString([]byte(key.NLRI)), NLRIFraming: rpc.NLRIFramingPrefixOnly, Withdraw: true}
			if path, present := ribevents.LookupFlowSpecPath(key); present {
				stored.AttrHex = hex.EncodeToString(path.Attributes)
				stored.MsgID = path.MsgID
				stored.SourcePolicyEpoch = path.SourcePolicyEpoch
				stored.SourceReceipt = path.SourceReceipt
				stored.Withdraw = false
			}
			rr.fillValidationReceipt(&stored)
			callCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
			err := rr.relayToPeer(callCtx, target, target.identity.address.String(), []rpc.StoredRoute{stored})
			cancel()
			if err != nil && ctx.Err() == nil {
				if failed == nil {
					failed = err
				}
			}
		}
	}
	return failed
}

// replayFlowSpecs owns the peer-up snapshot for these families. RelayStoredRoute
// preserves source authority and rechecks generation, reflection and export policy.
// fillValidationReceipt reads only the exact original inventory row. A
// withdrawal whose receive-store row just disappeared still carries its old
// receipt and ingress, without borrowing a current source at the same address.
func (rr *routeReflector) fillValidationReceipt(route *rpc.StoredRoute) {
	if route.SourceReceipt != 0 {
		return
	}
	rr.withdrawalMu.Lock()
	defer rr.withdrawalMu.Unlock()
	info, found := rr.lookupNativeLocked(route.SourcePeer, route.SourceOwner, route.SourceConnectionID, route.Family, route.PathID, route.NLRIHex, true)
	if !found {
		return
	}
	route.SourceReceipt = info.SourceReceipt
	route.MsgID = info.MsgID
	route.SourcePolicyEpoch = info.SourcePolicyEpoch
}

func (rr *routeReflector) replayFlowSpecs(parent context.Context, captured peerTarget) (int, error) {
	destination := captured.identity.address.String()
	// Keep mandatory replay independent of an expired optional-store request.
	ctx, cancel := context.WithTimeout(parent, updateRouteTimeout)
	defer cancel()
	replayed := 0
	for _, key := range ribevents.FlowSpecRoutes() {
		if key.PeerOwner == captured.identity.owner {
			continue
		}
		rr.mu.RLock()
		target := rr.peers[captured.identity]
		current := target != nil && target.Up && target.ReplayGen == captured.generation && target.DestinationSession == captured.session
		supported := current && target.Families[key.Family]
		rr.mu.RUnlock()
		if !current {
			break
		}
		if !supported {
			continue
		}
		path, present := ribevents.LookupFlowSpecPath(key)
		if !present {
			continue
		}
		stored := rpc.StoredRoute{SourcePeer: key.Peer.String(), Family: key.Family.String(),
			PathID: key.PathID, MsgID: path.MsgID, NLRIFraming: rpc.NLRIFramingPrefixOnly,
			SourcePolicyEpoch: path.SourcePolicyEpoch, SourceOwner: key.PeerOwner, SourceConnectionID: key.ConnectionID, SourceReceipt: path.SourceReceipt,
			NLRIHex: hex.EncodeToString([]byte(key.NLRI)), AttrHex: hex.EncodeToString(path.Attributes),
			InitialUpdate: true} // peer-up replay: passes the destination's replay fence
		if err := rr.relayToPeer(ctx, captured, destination, []rpc.StoredRoute{stored}); err != nil {
			return replayed, err
		}
		replayed++
	}
	return replayed, nil
}

// askFlowSpecGate emits a gate-query, so the selecting RIB announces its
// flowspec-gate again for this plugin, whose subscription may have gone live
// after the RIB's first announcement (ribevents.EventGateQuery).
func (rr *routeReflector) askFlowSpecGate() error {
	// refuse-by-default: guards ribevents.EventGateQuery -- a nil plugin is a test instance with no engine, and no RIB to ask
	if rr.plugin == nil {
		return nil
	}
	ctx, cancel := context.WithTimeout(context.Background(), flowSpecGateQueryTimeout)
	defer cancel()
	_, err := rr.plugin.EmitEvent(ctx, ribevents.Namespace, ribevents.EventGateQuery, "", "", "{}")
	return err
}

// flowSpecGateQueryTimeout bounds the gate-query askFlowSpecGate emits.
const flowSpecGateQueryTimeout = 5 * time.Second
