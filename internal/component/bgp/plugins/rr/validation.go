// Design: docs/architecture/bgp/structural-forwarding.md -- validation-triggered reflection
// RFC: rfc/short/rfc8955.md -- Section 6, unicast-triggered revalidation
package rr

import (
	"context"
	"encoding/hex"
	"strconv"
	"sync"
	"sync/atomic"
	"time"

	"github.com/ze-software/ze/internal/core/bgp/ribevents"
	"github.com/ze-software/ze/pkg/plugin/rpc"
	"github.com/ze-software/ze/pkg/ze"
)

var validationBus atomic.Pointer[ze.EventBus]

// startValidation MUST be paired with the returned stop function before closing
// the SDK connection. Notifications coalesce by received path while the worker
// performs RPC. The callback MUST NOT wait for that worker: its RPC may need the
// same receive-store command handler that is publishing the notification.
func (rr *routeReflector) startValidation(parent context.Context, bus ze.EventBus) func() {
	ctx, cancel := context.WithCancel(parent)
	wake := make(chan struct{}, 1)
	var mu sync.Mutex
	pending := make(map[ribevents.ValidationRoute]struct{})
	done := make(chan struct{})
	unsubscribe := ribevents.ValidationChange.Subscribe(bus, func(routes []ribevents.ValidationRoute) {
		mu.Lock()
		for _, route := range routes {
			pending[route] = struct{}{}
		}
		mu.Unlock()
		select {
		case wake <- struct{}{}:
		default:
			// A pending wake will collect these keys as well.
		}
	})
	go func() {
		defer close(done)
		for {
			select {
			case <-ctx.Done():
				return
			case <-wake:
				mu.Lock()
				routes := make([]ribevents.ValidationRoute, 0, len(pending))
				for route := range pending {
					routes = append(routes, route)
				}
				clear(pending)
				mu.Unlock()
				rr.replayValidation(ctx, routes)
			}
		}
	}()
	return func() {
		cancel()
		unsubscribe()
		<-done
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
				callCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
				status, _, err := rr.dispatchCommand(callCtx, "request bgp adj-rib-in replay-path",
					target, key.Peer.String(), key.Family.String(), key.Prefix.String(),
					strconv.FormatUint(uint64(key.PathID), 10))
				cancel()
				if err != nil {
					logger().Warn("reflection revalidation failed", "source", key.Peer, "target", target, "error", err)
				} else if status == statusError {
					logger().Warn("reflection revalidation refused", "source", key.Peer, "target", target)
				}
				continue
			}
			stored := rpc.StoredRoute{SourcePeer: key.Peer.String(), Family: key.Family.String(), PathID: key.PathID,
				NLRIHex: hex.EncodeToString([]byte(key.NLRI)), NLRIFraming: rpc.NLRIFramingPrefixOnly, Withdraw: true}
			if path, present := ribevents.LookupFlowSpecPath(key); present {
				stored.AttrHex = hex.EncodeToString(path.Attributes)
				stored.MsgID = path.MsgID
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
			NLRIHex: hex.EncodeToString([]byte(key.NLRI)), AttrHex: hex.EncodeToString(path.Attributes),
			InitialUpdate: true} // peer-up replay: passes the destination's replay fence
		if err := rr.plugin.RelayStoredRoute(ctx, destination, []rpc.StoredRoute{stored}); err != nil {
			logger().Warn("FlowSpec peer-up reflection failed", "source", key.Peer, "target", destination, "error", err)
			break
		}
		replayed++
	}
	return replayed
}
