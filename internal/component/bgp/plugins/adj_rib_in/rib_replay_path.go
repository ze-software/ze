// Design: docs/architecture/plugin/rib-storage-design.md -- validation-triggered replay
// RFC: rfc/short/draft-ietf-sidrops-aspa-verification.md -- Section 5.7 re-evaluation
package adj_rib_in

import (
	"encoding/hex"
	"fmt"
	"net/netip"
	"strconv"

	"github.com/ze-software/ze/internal/core/bgp/ribevents"
	"github.com/ze-software/ze/internal/core/family"
	"github.com/ze-software/ze/pkg/plugin/rpc"
)

// replayPathCommand answers ribevents.ReplayPathCommand: the current state of
// one source path for a destination after its validation changed, as a
// ribevents.ReplayPage of at most one route. Unlike peer-up replay it includes
// ineligible paths: the reactor gate converts them into withdrawals, and
// preserves eligible announcements. Missing paths carry an explicit withdrawal.
//
// The store answers and relays nothing. The caller (bgp-rr, bgp-rs) relays the
// route under its own name, because the destination's `send [ update ]` grant
// is the caller's and the operator gives this store none: a relay sent here
// would be judged on this store's grant and refused toward a peer that
// attached only the caller (ribevents.ReplayRoutesCommand).
func (r *AdjRIBInManager) replayPathCommand(args []string) (string, any, error) {
	if len(args) != 5 {
		return statusError, nil, fmt.Errorf("replay-path requires: destination source family prefix pathID")
	}
	destination, err := netip.ParseAddr(args[0])
	if err != nil {
		return statusError, nil, fmt.Errorf("replay-path destination: %w", err)
	}
	source, err := netip.ParseAddr(args[1])
	if err != nil {
		return statusError, nil, fmt.Errorf("replay-path source: %w", err)
	}
	fam, ok := family.LookupFamily(args[2])
	if !ok || !isSimplePrefixFamily(fam) {
		return statusError, nil, fmt.Errorf("replay-path requires a simple prefix family: %s", args[2])
	}
	prefix, err := netip.ParsePrefix(args[3])
	if err != nil {
		return statusError, nil, fmt.Errorf("replay-path prefix: %w", err)
	}
	pathID, err := strconv.ParseUint(args[4], 10, 32)
	if err != nil {
		return statusError, nil, fmt.Errorf("replay-path pathID: %w", err)
	}
	if destination == source {
		return statusDone, ribevents.ReplayPage{Routes: []rpc.StoredRoute{}}, nil
	}
	key := compactRouteKey{Fam: fam, Prefix: prefix, PathID: uint32(pathID)}
	route := rpc.StoredRoute{
		SourcePeer: args[1], Family: args[2], PathID: uint32(pathID),
		NLRIHex: prefixToWireHex(fam, args[3]), NLRIFraming: rpc.NLRIFramingPrefixOnly,
		Withdraw: true,
	}
	r.mu.RLock()
	var stored *RawRoute
	if pending := r.pending[pendingKey(source, key)]; pending != nil {
		stored = pending.route
	} else if routes := r.ribIn[source]; routes != nil {
		stored, _ = routes.Get(key)
	}
	if stored != nil {
		route.AttrHex = stored.AttrHex
		route.NextHopHex = stored.NHopHex
		route.NLRIHex = stored.NLRIHex
		route.NLRIFraming = stored.NLRIFraming
		route.MsgID = stored.MsgID
		route.SourcePolicyEpoch = stored.SourcePolicyEpoch
		route.Withdraw = false
	}
	r.mu.RUnlock()
	return statusDone, ribevents.ReplayPage{Routes: []rpc.StoredRoute{route}}, nil
}

// replayFlowSpecs supplies self-owned peer-up replay from the mandatory
// selecting RIB, never from this optional store's copies. The bounded relay
// preserves source generation and delegates destination family/session checks
// and export policy to the same reactor path used by RS and RR replay.
func (r *AdjRIBInManager) replayFlowSpecs(targetPeer netip.Addr) error {
	var routes []rpc.StoredRoute
	for _, key := range ribevents.FlowSpecRoutes() {
		if key.Peer == targetPeer {
			continue
		}
		path, present := ribevents.LookupFlowSpecPath(key)
		if !present {
			continue
		}
		routes = append(routes, rpc.StoredRoute{
			SourcePeer: key.Peer.String(), Family: key.Family.String(),
			PathID: key.PathID, MsgID: path.MsgID, NLRIFraming: rpc.NLRIFramingPrefixOnly,
			SourcePolicyEpoch: path.SourcePolicyEpoch,
			NLRIHex:           hex.EncodeToString([]byte(key.NLRI)), AttrHex: hex.EncodeToString(path.Attributes),
			InitialUpdate: true, // peer-up replay: passes the destination's replay fence
		})
	}
	r.mu.RLock()
	up := r.peerUp[targetPeer]
	r.mu.RUnlock()
	if !up {
		return nil
	}
	return r.relayRoutes(targetPeer.String(), routes)
}
