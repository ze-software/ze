// Design: docs/architecture/bgp/structural-forwarding.md -- receive validation and retained replay
// Related: server_forward.go -- per-source forwarding batches
package rs

import (
	"context"
	"encoding/hex"
	"strconv"
	"sync/atomic"

	"github.com/ze-software/ze/internal/core/bgp/ribevents"
	"github.com/ze-software/ze/internal/core/family"
	"github.com/ze-software/ze/pkg/plugin/rpc"
	"github.com/ze-software/ze/pkg/ze"
)

var validationBus atomic.Pointer[ze.EventBus]

// validationChanged only queues keys. The emitting Adj-RIB-In command handler
// cannot service a replay RPC until this callback returns, so forwarding MUST
// run on the existing source worker. That worker MUST flush its live batch first.
func (rs *routeServer) validationChanged(changes []ribevents.ValidationRoute) {
	if rs.stopping.Load() {
		return
	}
	for _, route := range changes {
		sourcePeer := route.Peer.String()
		item := workItem{sourcePeer: sourcePeer, validation: route}
		if !rs.workers.Dispatch(workerKey{sourcePeer: sourcePeer}, item) {
			logger().Debug("validation replay discarded after worker shutdown", "peer", sourcePeer)
		}
	}
}

// processValidation asks the receive store for the current path, including a
// retained ineligible path or a withdrawal for a removed path. The reactor gates
// its generation again before export. The event's position never supplies a
// verdict, and recovery uses the stored attributes without another UPDATE.
func (rs *routeServer) processValidation(key workerKey, route ribevents.ValidationRoute) {
	rs.flushWorkerBatch(key)
	rs.mu.RLock()
	peer := rs.peers[key.sourcePeer]
	if peer != nil && peer.StateSeen && !peer.Up {
		rs.mu.RUnlock()
		return
	}
	targets := rs.selectForwardTargets(nil, key.sourcePeer, 0, map[family.Family]bool{route.Family: true})
	rs.mu.RUnlock()
	for _, target := range targets {
		ctx, cancel := context.WithTimeout(context.Background(), forwardCachedTimeout)
		if ribevents.IsFlowSpec(route.Family) {
			stored := rpc.StoredRoute{
				SourcePeer: key.sourcePeer, Family: route.Family.String(), PathID: route.PathID,
				NLRIHex: hex.EncodeToString([]byte(route.NLRI)), NLRIFraming: rpc.NLRIFramingPrefixOnly, Withdraw: true,
			}
			if path, present := ribevents.LookupFlowSpecPath(route); present {
				stored.AttrHex = hex.EncodeToString(path.Attributes)
				stored.MsgID = path.MsgID
				stored.Withdraw = false
			}
			err := rs.plugin.RelayStoredRoute(ctx, target, []rpc.StoredRoute{stored})
			cancel()
			if err != nil {
				logger().Error("FlowSpec validation replay failed", "source", key.sourcePeer, "target", target, "error", err)
			}
			continue
		}
		status, _, err := rs.dispatchCommand(ctx, "request bgp adj-rib-in replay-path",
			target, key.sourcePeer, route.Family.String(), route.Prefix.String(), strconv.FormatUint(uint64(route.PathID), 10))
		cancel()
		if err != nil {
			logger().Error("validation replay failed", "source", key.sourcePeer, "target", target, "prefix", route.Prefix, "error", err)
			continue
		}
		if status == statusError {
			logger().Error("validation replay refused", "source", key.sourcePeer, "target", target, "prefix", route.Prefix)
		}
	}
}

// replayFlowSpecs supplies every peer-up snapshot from the mandatory RIB.
// The optional Adj-RIB-In excludes these families, preventing duplicate replay.
// Each reconstruction still crosses authorization and egress policies.
func (rs *routeServer) replayFlowSpecs(destination string, gen, cut uint64) {
	// A timed-out optional-store replay must not cancel this independent phase.
	ctx, cancel := context.WithTimeout(context.Background(), updateRouteTimeout)
	defer cancel()
	for _, key := range ribevents.FlowSpecRoutes() {
		if key.Peer.String() == destination {
			continue
		}
		rs.mu.RLock()
		target := rs.peers[destination]
		current := target != nil && target.Up && target.ReplayGen == gen
		supported := current && (target.Families == nil || target.SupportsFamily(key.Family))
		rs.mu.RUnlock()
		if !current {
			return
		}
		if !supported {
			continue
		}
		path, present := ribevents.LookupFlowSpecPath(key)
		if !present {
			continue
		}
		if path.MsgID != 0 && path.MsgID > cut {
			continue // The post-cut generation belongs to live forwarding.
		}
		route := rpc.StoredRoute{SourcePeer: key.Peer.String(), Family: key.Family.String(),
			PathID: key.PathID, MsgID: path.MsgID, NLRIFraming: rpc.NLRIFramingPrefixOnly,
			NLRIHex: hex.EncodeToString([]byte(key.NLRI)), AttrHex: hex.EncodeToString(path.Attributes)}
		if err := rs.plugin.RelayStoredRoute(ctx, destination, []rpc.StoredRoute{route}); err != nil {
			logger().Error("FlowSpec peer-up replay failed", "source", key.Peer, "target", destination, "error", err)
			return
		}
	}
}
