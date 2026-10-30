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
	"sync/atomic"
	"time"

	"github.com/ze-software/ze/internal/core/bgp/ribevents"
	"github.com/ze-software/ze/pkg/plugin/rpc"
	"github.com/ze-software/ze/pkg/ze"
)

var validationBus atomic.Pointer[ze.EventBus]

// validationQueue is the reflector's revalidation worker state: notifications
// coalesce by received path in pending while the worker performs RPC, and a
// settle barrier queued in settles is closed once the batch it joined has
// been relayed. wake holds at most one signal; done is closed when the worker
// returns. mu guards pending and settles. Safe for concurrent use.
type validationQueue struct {
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
// (before Run): it publishes the queue settleValidation reads. Notifications
// coalesce by received path while the worker performs RPC. The callback MUST
// NOT wait for that worker: its RPC may need the same receive-store command
// handler that is publishing the notification.
func (rr *routeReflector) startValidation(parent context.Context, bus ze.EventBus) func() {
	ctx, cancel := context.WithCancel(parent)
	q := &validationQueue{
		pending: make(map[ribevents.ValidationRoute]struct{}),
		wake:    make(chan struct{}, 1),
		done:    make(chan struct{}),
	}
	rr.validation = q
	unsubscribe := ribevents.ValidationChange.Subscribe(bus, func(routes []ribevents.ValidationRoute) {
		q.mu.Lock()
		for _, route := range routes {
			q.pending[route] = struct{}{}
		}
		q.mu.Unlock()
		q.signal()
	})
	go rr.runValidation(ctx, q)
	return func() {
		cancel()
		unsubscribe()
		<-q.done
	}
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
			rr.replayValidation(ctx, routes)
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
	// refuse-by-default: guards startValidation -- with no event bus no worker was started, so no revalidation can be queued
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
		return nil
	case <-q.done:
		return errors.New("the revalidation worker stopped before it relayed its queued paths")
	case <-ctx.Done():
		return errors.New("its revalidation relays did not reach the engine before the source-policy deadline: " +
			ctx.Err().Error())
	}
}

func (rr *routeReflector) replayValidation(ctx context.Context, routes []ribevents.ValidationRoute) {
	for _, key := range routes {
		if !key.Prefix.IsValid() && !ribevents.IsFlowSpec(key.Family) {
			continue
		}
		rr.mu.RLock()
		var targets []string
		for address, peer := range rr.peers {
			if peer.Up && address != key.Peer.String() && peer.Families[key.Family] {
				targets = append(targets, address)
			}
		}
		rr.mu.RUnlock()
		for _, target := range targets {
			if key.Prefix.IsValid() {
				// The store answers the path and this reflector relays it, under
				// the `send [ update ]` grant the operator gave this reflector
				// and not the store (ribevents.ReplayPathCommand).
				callCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
				_, err := ribevents.RelayReplayPath(callCtx, rr.dispatchCommand, rr.plugin.RelayStoredRoute,
					target, key.Peer.String(), key.Family.String(), key.Prefix.String(),
					strconv.FormatUint(uint64(key.PathID), 10))
				cancel()
				if err != nil {
					logger().Warn("reflection revalidation failed", "source", key.Peer, "target", target, "error", err)
				}
				continue
			}
			stored := rpc.StoredRoute{SourcePeer: key.Peer.String(), Family: key.Family.String(), PathID: key.PathID,
				NLRIHex: hex.EncodeToString([]byte(key.NLRI)), NLRIFraming: rpc.NLRIFramingPrefixOnly, Withdraw: true}
			if path, present := ribevents.LookupFlowSpecPath(key); present {
				stored.AttrHex = hex.EncodeToString(path.Attributes)
				stored.MsgID = path.MsgID
				stored.SourcePolicyEpoch = path.SourcePolicyEpoch
				stored.Withdraw = false
			}
			callCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
			err := rr.plugin.RelayStoredRoute(callCtx, target, []rpc.StoredRoute{stored})
			cancel()
			if err != nil && ctx.Err() == nil {
				logger().Warn("FlowSpec reflection revalidation failed", "source", key.Peer, "target", target, "error", err)
			}
		}
	}
}

// replayFlowSpecs owns the peer-up snapshot for these families. RelayStoredRoute
// preserves source authority and rechecks generation, reflection and export policy.
func (rr *routeReflector) replayFlowSpecs(destination string, gen uint64) int {
	// Keep mandatory replay independent of an expired optional-store request.
	ctx, cancel := context.WithTimeout(context.Background(), updateRouteTimeout)
	defer cancel()
	replayed := 0
	for _, key := range ribevents.FlowSpecRoutes() {
		if key.Peer.String() == destination {
			continue
		}
		rr.mu.RLock()
		target := rr.peers[destination]
		current := target != nil && target.Up && target.ReplayGen == gen
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
			SourcePolicyEpoch: path.SourcePolicyEpoch,
			NLRIHex:           hex.EncodeToString([]byte(key.NLRI)), AttrHex: hex.EncodeToString(path.Attributes),
			InitialUpdate: true} // peer-up replay: passes the destination's replay fence
		if err := rr.plugin.RelayStoredRoute(ctx, destination, []rpc.StoredRoute{stored}); err != nil {
			logger().Warn("FlowSpec peer-up reflection failed", "source", key.Peer, "target", destination, "error", err)
			break
		}
		replayed++
	}
	return replayed
}
