// Design: docs/architecture/plugin/rib-storage-design.md — Adj-RIB-In raw hex storage
// RFC: rfc/short/rfc4271.md -- Adj-RIBs-In stores unprocessed routing information
// Detail: rib_commands.go — command handlers (status, show, replay, validation)
// Detail: rib_validation.go — RPKI validation gate (pending routes, timeout, state constants)
//
// Package bgp_adj_rib_in implements an Adj-RIB-In plugin for ze.
// It stores all received routes per source peer as raw hex wire bytes
// (from format=full events) and replays them, with the source peer attached,
// through the engine's relay-stored-route call so a replayed route takes the
// same egress rail a forwarded one does.
//
// RFC 4271 Section 3.2: Adj-RIBs-In stores unprocessed routing information
// advertised to the local BGP speaker by its peers.
package adj_rib_in

import (
	"context"
	"encoding/hex"
	"fmt"
	"log/slog"
	"net"
	"net/netip"
	"sync"
	"sync/atomic"
	"time"

	"github.com/ze-software/ze/internal/core/bgp/routeaction"

	bgp "github.com/ze-software/ze/internal/component/bgp"
	adjyang "github.com/ze-software/ze/internal/component/bgp/plugins/adj_rib_in/yang"
	bgptypes "github.com/ze-software/ze/internal/component/bgp/types"
	"github.com/ze-software/ze/internal/core/bgp/attribute"
	bgpctx "github.com/ze-software/ze/internal/core/bgp/context"
	"github.com/ze-software/ze/internal/core/bgp/nlri"
	"github.com/ze-software/ze/internal/core/family"
	"github.com/ze-software/ze/internal/core/seqmap"
	"github.com/ze-software/ze/internal/core/slogutil"
	"github.com/ze-software/ze/pkg/plugin/rpc"
	sdk "github.com/ze-software/ze/pkg/plugin/sdk"
)

const (
	statusDone  = "done"
	statusError = "error"
	stateUp     = "up"
	stateDown   = "down"
)

// loggerPtr is the package-level logger, disabled by default.
var loggerPtr atomic.Pointer[slog.Logger]

func init() {
	d := slogutil.DiscardLogger()
	loggerPtr.Store(d)
}

func logger() *slog.Logger { return loggerPtr.Load() }

// setLogger sets the package-level logger.
// Called from register.go closures.
func setLogger(l *slog.Logger) {
	if l != nil {
		loggerPtr.Store(l)
	}
}

// RawRoute stores a route as raw hex wire bytes for efficient replay.
//
// AttrHex is the WHOLE path-attribute section as received, MP_REACH_NLRI and
// MP_UNREACH_NLRI INCLUDED -- it comes from RawMessage.AttrsWire, which
// reactor_notify.go sets to WireUpdate.Attrs() over the entire attribute block.
// For an MP family it therefore carries every NLRI of the originating UPDATE,
// not just this route's, so any consumer rebuilding a single-route UPDATE must
// strip types 14/15 and re-synthesize MP_REACH (see the reactor's relay_payload.go).
// This comment previously claimed the opposite; see assumption A-1 in
// plan/spec-fixit-bgp-egress-rail-divergence.md.
//
// NHopHex is the next-hop IP converted to wire hex.
// NLRIHex is the individual NLRI wire bytes in hex.
// Sequence numbers are tracked by the seqmap, not stored in RawRoute.
type RawRoute struct {
	Family          family.Family // Address family (e.g. family.IPv4Unicast)
	AttrHex         string        // Raw path attributes hex from format=full
	NHopHex         string        // Next-hop as wire hex (e.g. "0a000001" for 10.0.0.1)
	NLRIHex         string        // Individual NLRI wire bytes hex
	ValidationState uint8         // RPKI validation state (0=NotValidated, 1=Valid, 2=NotFound, 3=Invalid)
}

// AdjRIBInManager implements the Adj-RIB-In plugin.
// Stores received routes as raw hex for fast replay via relay-stored-route.
type AdjRIBInManager struct {
	plugin *sdk.Plugin

	// ribIn stores routes received FROM peers.
	// sourcePeer → seqmap of compactRouteKey → RawRoute
	// Keyed by netip.Addr: peer strings are parsed once at the event /
	// command boundary (see parsePeerAddress).
	ribIn map[netip.Addr]*seqmap.Map[compactRouteKey, *RawRoute]

	// peerUp tracks which peers are currently up.
	peerUp map[netip.Addr]bool

	// seqCounter is the monotonic sequence counter for incremental replay.
	seqCounter uint64

	// pending stores routes awaiting RPKI validation.
	pending map[compactPendingKey]*PendingRoute

	// earlyDecisions buffers RPKI decisions that arrived before the route.
	earlyDecisions map[compactPendingKey]*EarlyDecision

	// validationEnabled is set by "request bgp adj-rib-in enable-validation".
	// When true, received routes are stored as pending instead of installed.
	validationEnabled bool

	// validationTimeout is the fail-open timeout for pending routes.
	// Zero means use defaultValidationTimeout (30s).
	validationTimeout time.Duration

	mu sync.RWMutex

	// routeRelayer, if set, overrides the engine relay call for replay delivery.
	// Used in tests to verify peer-up triggers replay without an engine.
	routeRelayer func(destination string, routes []rpc.StoredRoute)

	// replayOwned is set when another plugin has claimed peer-up replay (bgp-rs,
	// via "request bgp adj-rib-in claim-replay" at startup). While set, this
	// plugin does NOT self-replay on peer-up.
	//
	// Both used to fire. bgp-rs marks a peer Replaying and withholds it from
	// forward targets until its replay finishes, so the rs-driven replay never
	// races the live forward. This plugin's self-replay has no such gate, so with
	// both running a route learned just before a peer established went out twice:
	// once from the ungated self-replay and once from the forward path. Standing
	// down here leaves exactly one replay owner. With bgp-rs absent nothing claims
	// ownership, the flag stays false, and self-replay remains the only path.
	replayOwned atomic.Bool
}

// newSeqMap creates a new seqmap for route storage.
func newSeqMap() *seqmap.Map[compactRouteKey, *RawRoute] {
	return seqmap.New[compactRouteKey, *RawRoute]()
}

// RunAdjRIBInPlugin runs the Adj-RIB-In plugin using the SDK RPC protocol.
func RunAdjRIBInPlugin(conn net.Conn) int {
	logger().Debug("adj-rib-in plugin starting")

	p := sdk.NewWithConn("bgp-adj-rib-in", conn)
	defer func() { _ = p.Close() }()

	r := &AdjRIBInManager{
		plugin:         p,
		ribIn:          make(map[netip.Addr]*seqmap.Map[compactRouteKey, *RawRoute]),
		peerUp:         make(map[netip.Addr]bool),
		pending:        make(map[compactPendingKey]*PendingRoute),
		earlyDecisions: make(map[compactPendingKey]*EarlyDecision),
	}

	// Structured event handler for DirectBridge delivery.
	// State events use metadata fields directly. UPDATE events are dispatched
	// to the appropriate handler based on EventType.
	p.OnStructuredEvent(func(events []any) error {
		for _, event := range events {
			se, ok := event.(*rpc.StructuredEvent)
			if !ok || se.PeerAddress == "" {
				continue
			}
			switch se.EventType { //nolint:exhaustive // only state+update handled on structured path
			case rpc.EventKindState:
				r.handleStructuredState(se)
			case rpc.EventKindUpdate:
				r.handleReceivedStructured(se)
			}
		}
		return nil
	})

	// Fallback: JSON event handler for non-DirectBridge delivery.
	p.OnEvent(func(jsonStr string) error {
		event, err := bgp.ParseEvent([]byte(jsonStr))
		if err != nil {
			logger().Warn("parse error", "error", err, "line", jsonStr[:min(100, len(jsonStr))])
			return nil
		}
		r.dispatch(event)
		return nil
	})

	p.OnExecuteCommand(func(serial, command string, args []string, peer string) (string, any, error) {
		return r.handleCommand(command, args, peer)
	})

	// Start the timeout scanner for pending validation routes (fail-open).
	stopCh := make(chan struct{})
	r.startTimeoutScanner(stopCh)
	defer close(stopCh)

	// Register typed batch-validate handler for DirectBridge fast path.
	rpc.RegisterBatchValidator(r.handleBatchValidateTyped)

	// Subscribe to received events with format=full (includes raw hex bytes).
	p.SetStartupSubscriptions([]string{"update direction received", "state"}, nil, "full")

	ctx, cancel := sdk.SignalContext()
	defer cancel()
	err := p.Run(ctx, sdk.Registration{
		Commands: []sdk.CommandDecl{
			{Name: "show bgp adj-rib-in status"},
			{Name: "show bgp adj-rib-in"},
			{Name: "request bgp adj-rib-in replay"},
			// Plugin-to-plugin plumbing, not an operator verb: bgp-rs claims
			// peer-up replay ownership with this at startup.
			{Name: "request bgp adj-rib-in claim-replay", Hidden: true},
			{Name: "request bgp adj-rib-in enable-validation"},
			{Name: "request bgp adj-rib-in accept-routes"},
			{Name: "request bgp adj-rib-in reject-routes"},
			{Name: "request bgp adj-rib-in batch-validate"},
			{Name: "request bgp adj-rib-in revalidate"},
		},
	})
	if err != nil {
		logger().Error("adj-rib-in plugin failed", "error", err)
		return 1
	}

	return 0
}

// parsePeerAddress converts a peer address string to netip.Addr at the
// event / command boundary. The engine produces canonical netip.Addr.String()
// values, so a failure means a malformed producer or operator input; the
// caller must log or return the error and stop (fail closed, never a
// zero-Addr map key).
func parsePeerAddress(peerAddr string) (netip.Addr, error) {
	addr, err := netip.ParseAddr(peerAddr)
	if err != nil {
		return netip.Addr{}, fmt.Errorf("adj-rib-in: invalid peer address %q: %w (expected an IP address)", peerAddr, err)
	}
	return addr, nil
}

// handleReceivedStructured processes received UPDATE events from StructuredEvent wire types.
// Walks wire bytes directly using NLRIIterator, skipping the bgp.Event intermediary
// and wireNLRIsToAny boxing. The legacy handleReceived path is preserved for external
// text/JSON plugins.
func (r *AdjRIBInManager) handleReceivedStructured(se *rpc.StructuredEvent) {
	msg, ok := se.RawMessage.(*bgptypes.RawMessage)
	if !ok || msg == nil || msg.WireUpdate == nil {
		return
	}

	if se.PeerAddress == "" {
		return
	}
	peerAddr, err := parsePeerAddress(se.PeerAddress)
	if err != nil {
		logger().Warn("received structured event dropped", "error", err)
		return
	}

	wu := msg.WireUpdate
	ctx := bgpctx.Registry.Get(wu.SourceCtxID())

	// Hex-encode attributes once per UPDATE (shared across all NLRIs).
	var attrHex string
	if msg.AttrsWire != nil {
		if packed := msg.AttrsWire.Packed(); len(packed) > 0 {
			attrHex = hex.EncodeToString(packed)
		}
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	// IPv4 unicast announces (body NLRI section).
	if nlriData, err := wu.NLRI(); err == nil && len(nlriData) > 0 {
		fam := family.IPv4Unicast
		addPath := ctx != nil && ctx.AddPath(fam)
		nhopHex := nhopHexFromWireAttr(msg.AttrsWire)
		r.installStructuredNLRIs(peerAddr, fam, nlriData, addPath, attrHex, nhopHex)
	}

	// IPv4 unicast withdrawals (body Withdrawn section).
	if wdData, err := wu.Withdrawn(); err == nil && len(wdData) > 0 {
		fam := family.IPv4Unicast
		addPath := ctx != nil && ctx.AddPath(fam)
		r.removeStructuredNLRIs(peerAddr, fam, wdData, addPath)
	}

	// MP_REACH_NLRI announces.
	if mpReach, err := wu.MPReach(); err == nil && mpReach != nil {
		fam := mpReach.Family()
		nlriBytes := mpReach.NLRIBytes()
		if len(nlriBytes) > 0 {
			addPath := ctx != nil && ctx.AddPath(fam)
			if isSimplePrefixFamily(fam) {
				nhopHex := nhopHexFromAddr(mpReach.NextHop())
				r.installStructuredNLRIs(peerAddr, fam, nlriBytes, addPath, attrHex, nhopHex)
			} else {
				// Complex families: fall back to Event path for correct NLRI handling.
				r.installComplexNLRIs(peerAddr, fam, nlriBytes, addPath, attrHex, mpReach.NextHop().String())
			}
		}
	}

	// MP_UNREACH_NLRI withdrawals.
	if mpUnreach, err := wu.MPUnreach(); err == nil && mpUnreach != nil {
		fam := mpUnreach.Family()
		wdBytes := mpUnreach.WithdrawnBytes()
		if len(wdBytes) > 0 {
			addPath := ctx != nil && ctx.AddPath(fam)
			if isSimplePrefixFamily(fam) {
				r.removeStructuredNLRIs(peerAddr, fam, wdBytes, addPath)
			} else {
				r.removeComplexNLRIs(peerAddr, fam, wdBytes, addPath)
			}
		}
	}
}

// installStructuredNLRIs walks simple-prefix wire bytes and installs routes.
// Caller must hold r.mu.
func (r *AdjRIBInManager) installStructuredNLRIs(peerAddr netip.Addr, fam family.Family, data []byte, addPath bool, attrHex, nhopHex string) {
	if attrHex == "" || nhopHex == "" {
		return
	}
	iter := nlri.NewNLRIIterator(data, addPath)
	for {
		wirePrefix, pathID, ok := iter.Next()
		if !ok {
			break
		}
		pfx, valid := nlri.WirePrefixToKey(wirePrefix, fam)
		if !valid {
			continue
		}
		rk := routeKeyFromWire(fam, pfx, pathID)
		nlriHex := hex.EncodeToString(wirePrefix)

		route := &RawRoute{
			Family:  fam,
			AttrHex: attrHex,
			NHopHex: nhopHex,
			NLRIHex: nlriHex,
		}

		if r.validationEnabled {
			pr := &PendingRoute{
				peerAddr:   peerAddr,
				family:     fam,
				prefix:     pfx.String(),
				routeKey:   rk,
				route:      route,
				receivedAt: time.Now(),
				state:      ValidationPending,
			}
			if !r.applyEarlyDecision(peerAddr, rk, pr) {
				pKey := pendingKey(peerAddr, rk)
				r.pending[pKey] = pr
			}
		} else {
			if r.ribIn[peerAddr] == nil {
				r.ribIn[peerAddr] = newSeqMap()
			}
			r.seqCounter++
			r.ribIn[peerAddr].Put(rk, r.seqCounter, route)
		}
	}
}

// removeStructuredNLRIs walks simple-prefix wire bytes and removes routes.
// Caller must hold r.mu.
func (r *AdjRIBInManager) removeStructuredNLRIs(peerAddr netip.Addr, fam family.Family, data []byte, addPath bool) {
	iter := nlri.NewNLRIIterator(data, addPath)
	for {
		wirePrefix, pathID, ok := iter.Next()
		if !ok {
			break
		}
		pfx, valid := nlri.WirePrefixToKey(wirePrefix, fam)
		if !valid {
			continue
		}
		rk := routeKeyFromWire(fam, pfx, pathID)
		r.removePending(peerAddr, rk)
		if r.ribIn[peerAddr] != nil {
			r.ribIn[peerAddr].Delete(rk)
		}
	}
}

// installComplexNLRIs handles non-simple-prefix families (VPN, EVPN) via wireNLRIsToAny.
// These are rare in benchmarks and their wire format prevents direct prefix extraction.
// Caller must hold r.mu.
func (r *AdjRIBInManager) installComplexNLRIs(peerAddr netip.Addr, fam family.Family, data []byte, addPath bool, attrHex, nhopStr string) {
	if attrHex == "" {
		return
	}
	nhopHex := nhopToHex(nhopStr)
	if nhopHex == "" {
		return
	}
	nlris := wireNLRIsToAny(data, addPath, fam)
	rawNLRIHex := hex.EncodeToString(data)
	for i, nlriVal := range nlris {
		prefix, pathID := bgp.ParseNLRIValue(nlriVal)
		if prefix == "" {
			continue
		}
		rk := routeKeyFromStrings(fam, prefix, pathID)
		var nlriHex string
		if i == 0 {
			nlriHex = rawNLRIHex
		} else {
			continue
		}
		route := &RawRoute{
			Family:  fam,
			AttrHex: attrHex,
			NHopHex: nhopHex,
			NLRIHex: nlriHex,
		}
		if r.validationEnabled {
			pr := &PendingRoute{
				peerAddr:   peerAddr,
				family:     fam,
				prefix:     prefix,
				routeKey:   rk,
				route:      route,
				receivedAt: time.Now(),
				state:      ValidationPending,
			}
			if !r.applyEarlyDecision(peerAddr, rk, pr) {
				pKey := pendingKey(peerAddr, rk)
				r.pending[pKey] = pr
			}
		} else {
			if r.ribIn[peerAddr] == nil {
				r.ribIn[peerAddr] = newSeqMap()
			}
			r.seqCounter++
			r.ribIn[peerAddr].Put(rk, r.seqCounter, route)
		}
	}
}

// removeComplexNLRIs handles withdrawal of non-simple-prefix families.
// Caller must hold r.mu.
func (r *AdjRIBInManager) removeComplexNLRIs(peerAddr netip.Addr, fam family.Family, data []byte, addPath bool) {
	nlris := wireNLRIsToAny(data, addPath, fam)
	for _, nlriVal := range nlris {
		prefix, pathID := bgp.ParseNLRIValue(nlriVal)
		if prefix == "" {
			continue
		}
		rk := routeKeyFromStrings(fam, prefix, pathID)
		r.removePending(peerAddr, rk)
		if r.ribIn[peerAddr] != nil {
			r.ribIn[peerAddr].Delete(rk)
		}
	}
}

// nhopHexFromWireAttr extracts next-hop from wire NEXT_HOP attribute and hex-encodes it.
func nhopHexFromWireAttr(attrs *attribute.AttributesWire) string {
	if attrs == nil {
		return ""
	}
	attr, err := attrs.Get(attribute.AttrNextHop)
	if err != nil || attr == nil {
		return ""
	}
	nhop, ok := attr.(*attribute.NextHop)
	if !ok || !nhop.Addr.IsValid() {
		return ""
	}
	return nhopHexFromAddr(nhop.Addr)
}

// nhopHexFromAddr hex-encodes a netip.Addr for RawRoute storage.
func nhopHexFromAddr(addr netip.Addr) string {
	if !addr.IsValid() {
		return ""
	}
	if addr.Unmap().Is4() {
		b := addr.Unmap().As4()
		return hex.EncodeToString(b[:])
	}
	b := addr.As16()
	return hex.EncodeToString(b[:])
}

func legacyNextHop(attrs *attribute.AttributesWire) string {
	if attrs == nil {
		return ""
	}
	attr, err := attrs.Get(attribute.AttrNextHop)
	if err != nil || attr == nil {
		return ""
	}
	nhop, ok := attr.(*attribute.NextHop)
	if !ok || !nhop.Addr.IsValid() {
		return ""
	}
	return nhop.Addr.String()
}

// wireNLRIsToAny walks wire NLRI bytes and returns prefix strings as []any.
// Uses stack-allocated [16]byte buffer to avoid per-prefix heap allocation.
func wireNLRIsToAny(data []byte, addPath bool, fam family.Family) []any {
	isIPv6 := fam.AFI == family.AFIIPv6
	addrLen := 4
	if isIPv6 {
		addrLen = 16
	}

	var result []any
	var buf [16]byte // stack-allocated — large enough for IPv6
	offset := 0
	for offset < len(data) {
		if addPath {
			if offset+4 >= len(data) {
				break
			}
			offset += 4 // skip path-ID
		}
		if offset >= len(data) {
			break
		}
		prefixLen := int(data[offset])
		byteCount := (prefixLen + 7) / 8
		offset++ // skip prefix-len byte
		if offset+byteCount > len(data) {
			break
		}
		// Zero and fill from wire — reuse stack buffer each iteration.
		clear(buf[:])
		copy(buf[:], data[offset:offset+byteCount])
		offset += byteCount

		addr, ok := netip.AddrFromSlice(buf[:addrLen])
		if !ok {
			continue
		}
		result = append(result, netip.PrefixFrom(addr, prefixLen).String())
	}
	return result
}

// handleStructuredState processes a structured state event from DirectBridge.
func (r *AdjRIBInManager) handleStructuredState(se *rpc.StructuredEvent) {
	if se.PeerAddress == "" {
		return
	}
	peerAddr, err := parsePeerAddress(se.PeerAddress)
	if err != nil {
		logger().Warn("state structured event dropped", "error", err)
		return
	}

	state := se.State
	if state != rpc.SessionStateUp && state != rpc.SessionStateDown {
		logger().Debug("ignoring unknown peer state", "peer", peerAddr, "state", state)
		return
	}

	isUp := state == rpc.SessionStateUp

	r.mu.Lock()
	r.peerUp[peerAddr] = isUp

	if !isUp {
		delete(r.ribIn, peerAddr)
		r.clearPeerPending(peerAddr)
	}
	r.mu.Unlock()

	if isUp && !r.replayOwned.Load() {
		routes, _ := r.buildReplayRoutes(peerAddr, 0)
		if err := r.relayRoutes(se.PeerAddress, routes); err != nil {
			logger().Error("peer-up replay failed", "peer", se.PeerAddress, "routes", len(routes), "error", err)
		}
	}
}

// dispatch routes an event to the appropriate handler.
func (r *AdjRIBInManager) dispatch(event *bgp.Event) {
	eventType := event.GetEventType()

	switch eventType { //nolint:exhaustive // adj-rib-in only handles update+state
	case rpc.EventKindUpdate:
		r.handleReceived(event)
	case rpc.EventKindState:
		r.handleState(event)
	}
}

// handleReceived processes received UPDATE events from peers.
// Stores routes as raw hex from format=full events.
func (r *AdjRIBInManager) handleReceived(event *bgp.Event) {
	if event.GetPeerAddress() == "" {
		return
	}
	peerAddr, err := parsePeerAddress(event.GetPeerAddress())
	if err != nil {
		logger().Warn("received event dropped", "error", err)
		return
	}

	if len(event.FamilyOps) == 0 {
		return
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	for fam, ops := range event.FamilyOps {
		// Split raw NLRI hex into individual prefixes for simple families.
		// For complex families (VPN, EVPN), splitRawNLRIHex returns nil
		// and the raw blob is used directly (see switch below).
		rawNLRIHex := event.GetRawNLRIHex(fam)
		var splitHexEntries []string
		if rawNLRIHex != "" {
			splitHexEntries = splitRawNLRIHex(rawNLRIHex, fam)
		}

		for _, op := range ops {
			switch op.Action { //nolint:exhaustive // only Add/Del relevant for adj-rib-in
			case routeaction.Add:
				// Skip adds without essential fields -- routes missing attributes
				// or next-hop cannot be reconstructed for relay.
				if event.GetRawAttributesHex() == "" {
					continue
				}
				nhopHex := nhopToHex(op.NextHop)
				if nhopHex == "" {
					continue
				}

				for i, nlriVal := range op.NLRIs {
					prefix, pathID := bgp.ParseNLRIValue(nlriVal)
					if prefix == "" {
						continue
					}
					rk := routeKeyFromStrings(fam, prefix, pathID)

					var nlriHex string
					switch {
					case i < len(splitHexEntries):
						nlriHex = splitHexEntries[i]
					case rawNLRIHex != "" && !isSimplePrefixFamily(fam):
						if i > 0 {
							continue
						}
						nlriHex = rawNLRIHex
					default:
						nlriHex = prefixToWireHex(fam, prefix, pathID)
					}

					route := &RawRoute{
						Family:  fam,
						AttrHex: event.GetRawAttributesHex(),
						NHopHex: nhopHex,
						NLRIHex: nlriHex,
					}

					if r.validationEnabled {
						pr := &PendingRoute{
							peerAddr:   peerAddr,
							family:     fam,
							prefix:     prefix,
							routeKey:   rk,
							route:      route,
							receivedAt: time.Now(),
							state:      ValidationPending,
						}
						if !r.applyEarlyDecision(peerAddr, rk, pr) {
							pKey := pendingKey(peerAddr, rk)
							r.pending[pKey] = pr
						}
					} else {
						if r.ribIn[peerAddr] == nil {
							r.ribIn[peerAddr] = newSeqMap()
						}
						r.seqCounter++
						r.ribIn[peerAddr].Put(rk, r.seqCounter, route)
					}
				}

			case routeaction.Del:
				for _, nlriVal := range op.NLRIs {
					prefix, pathID := bgp.ParseNLRIValue(nlriVal)
					if prefix == "" {
						continue
					}
					rk := routeKeyFromStrings(fam, prefix, pathID)
					r.removePending(peerAddr, rk)
					if r.ribIn[peerAddr] != nil {
						r.ribIn[peerAddr].Delete(rk)
					}
				}
			}
		}
	}
}

// handleState processes peer state changes.
// On peer-up: marks peer as up, then replays all known routes from other
// source peers. Replay runs after lock release to avoid deadlock
// (buildReplayRoutes takes RLock, relayRoutes does I/O).
// Only processes "up" and "down" states; unknown/intermediate FSM states are ignored.
func (r *AdjRIBInManager) handleState(event *bgp.Event) {
	if event.GetPeerAddress() == "" {
		return
	}
	peerAddr, err := parsePeerAddress(event.GetPeerAddress())
	if err != nil {
		logger().Warn("state event dropped", "error", err)
		return
	}

	state := event.GetPeerState()

	// Only process known states. Ignore unknown/intermediate FSM states
	// to avoid accidentally clearing routes on transient transitions.
	if state != stateUp && state != stateDown {
		logger().Debug("ignoring unknown peer state", "peer", peerAddr, "state", state)
		return
	}

	isUp := state == stateUp

	r.mu.Lock()
	r.peerUp[peerAddr] = isUp

	if !isUp {
		// Peer went down -- clear installed and pending routes.
		delete(r.ribIn, peerAddr)
		r.clearPeerPending(peerAddr)
	}
	r.mu.Unlock()

	if isUp && !r.replayOwned.Load() {
		// Replay all known routes to the newly-up peer.
		// buildReplayRoutes takes RLock internally; relayRoutes does I/O.
		// Both must run outside the write lock to avoid deadlock.
		routes, _ := r.buildReplayRoutes(peerAddr, 0)
		if err := r.relayRoutes(event.GetPeerAddress(), routes); err != nil {
			logger().Error("peer-up replay failed", "peer", event.GetPeerAddress(), "routes", len(routes), "error", err)
		}
	}
}

// buildReplayRoutes collects the stored routes to replay to a target peer.
// Returns the routes and the maximum sequence index among them.
// Uses seqmap.Since for O(log N + K) delta replay instead of O(N) full scan.
//
// Each route carries the peer it was LEARNED from. That is the whole point of
// this shape: the engine reproduces the egress transform that source implies
// (AS_PATH prepend decision, RFC 4456 reflection, RFC 9234 role/OTC, export
// policy). The previous form built "update hex ... add" command strings, which
// dropped the source and sent the route down the ANNOUNCE rail -- prepending the
// local AS first and then running only the session's export filters, so a
// replayed route and a forwarded one diverged on the wire. See
// plan/spec-fixit-bgp-egress-rail-divergence.md.
func (r *AdjRIBInManager) buildReplayRoutes(targetPeer netip.Addr, fromIndex uint64) ([]rpc.StoredRoute, uint64) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	var out []rpc.StoredRoute
	var maxSeq uint64

	for sourcePeer, routes := range r.ribIn {
		if sourcePeer == targetPeer {
			continue // Don't replay a peer's own routes back to it.
		}
		sourceStr := sourcePeer.String()
		routes.Since(fromIndex, func(_ compactRouteKey, seq uint64, rt *RawRoute) bool {
			out = append(out, rpc.StoredRoute{
				SourcePeer: sourceStr,
				Family:     rt.Family.String(),
				AttrHex:    rt.AttrHex,
				NextHopHex: rt.NHopHex,
				NLRIHex:    rt.NLRIHex,
			})
			if seq > maxSeq {
				maxSeq = seq
			}
			return true
		})
	}

	return out, maxSeq
}

// relayChunkSize bounds how many stored routes travel in one relay call.
//
// A forked adj-rib-in reaches the engine over the plugin IPC framing, which
// rejects a frame above rpc.MaxMessageSize (16 MB) and does NOT split
// plugin->engine RPC payloads -- only the event path is chunked. A StoredRoute
// serializes to a couple of hundred bytes, so an unchunked replay of a large
// Adj-RIB-In would fail as a single oversized frame with every route lost at
// once. Chunking also bounds how many reconstruction buffers the engine holds in
// flight per call.
const relayChunkSize = 4096

// relayRoutes hands stored routes to the engine for relay through the forward
// rail, in bounded chunks.
//
// Returns an error when ANY chunk fails. The caller MUST propagate it. bgp-rs
// sends End-of-RIB on its failure path too (rs/server_handlers.go, "Always send
// EOR when replay terminates"), so propagating does not by itself stop a peer
// being told its table is complete -- what it does is make the failure visible
// at ERROR instead of WARN-and-continue, and skip the delta-convergence loop
// that would otherwise run against a replay that never happened.
func (r *AdjRIBInManager) relayRoutes(destination string, routes []rpc.StoredRoute) error {
	if len(routes) == 0 {
		return nil
	}
	if r.routeRelayer != nil {
		r.routeRelayer(destination, routes)
		return nil
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	for start := 0; start < len(routes); start += relayChunkSize {
		end := min(start+relayChunkSize, len(routes))
		if err := r.plugin.RelayStoredRoute(ctx, destination, routes[start:end]); err != nil {
			logger().Warn("relay-stored-route failed",
				"peer", destination, "routes", len(routes), "chunk-start", start, "error", err)
			return err
		}
	}
	return nil
}

// nhopToHex converts a next-hop IP address string to wire hex.
// IPv4: "10.0.0.1" -> "0a000001", IPv6: "::1" -> 32 hex chars.
func nhopToHex(ipStr string) string {
	addr, err := netip.ParseAddr(ipStr)
	if err != nil {
		return ""
	}
	if addr.Unmap().Is4() {
		b := addr.Unmap().As4()
		return hex.EncodeToString(b[:])
	}
	b := addr.As16()
	return hex.EncodeToString(b[:])
}

// splitRawNLRIHex splits concatenated raw NLRI hex into individual entries.
// Only works for simple prefix families (IPv4/IPv6 unicast/multicast).
// Returns nil for complex families (VPN, EVPN, FlowSpec).
func splitRawNLRIHex(rawHex string, fam family.Family) []string {
	data, err := hex.DecodeString(rawHex)
	if err != nil || len(data) == 0 {
		return nil
	}

	if !isSimplePrefixFamily(fam) {
		return nil
	}

	var result []string
	offset := 0
	for offset < len(data) {
		prefixLen := int(data[offset])
		nlriLen := 1 + (prefixLen+7)/8

		if offset+nlriLen > len(data) {
			break
		}
		result = append(result, hex.EncodeToString(data[offset:offset+nlriLen]))
		offset += nlriLen
	}

	return result
}

// isSimplePrefixFamily returns true for families with simple [prefix-len][prefix-bytes] format.
// Complex families (VPN, EVPN, FlowSpec, etc.) have different NLRI structures.
func isSimplePrefixFamily(fam family.Family) bool {
	switch fam {
	case family.IPv4Unicast, family.IPv4Multicast, family.IPv6Unicast, family.IPv6Multicast:
		return true
	}
	return false
}

// prefixToWireHex converts a text prefix to NLRI wire hex.
// Only correct for simple prefix families (IPv4/IPv6 unicast/multicast).
// Called as fallback when raw NLRI bytes are not available.
func prefixToWireHex(fam family.Family, prefix string, pathID uint32) string {
	_, ipnet, err := net.ParseCIDR(prefix)
	if err != nil {
		return ""
	}

	prefixLen, _ := ipnet.Mask.Size()
	prefixBytes := (prefixLen + 7) / 8

	var ipBytes net.IP
	switch fam.AFI {
	case family.AFIIPv4:
		ipBytes = ipnet.IP.To4()
	case family.AFIIPv6:
		ipBytes = ipnet.IP.To16()
	case family.AFIL2VPN, family.AFIBGPLS:
		// Complex AFIs handled via raw blob path; prefixToWireHex not called.
	}

	if ipBytes == nil {
		return ""
	}

	var wire []byte
	if pathID != 0 {
		wire = make([]byte, 4+1+prefixBytes)
		wire[0] = byte(pathID >> 24)
		wire[1] = byte(pathID >> 16)
		wire[2] = byte(pathID >> 8)
		wire[3] = byte(pathID)
		wire[4] = byte(prefixLen)
		copy(wire[5:], ipBytes[:prefixBytes])
	} else {
		wire = make([]byte, 1+prefixBytes)
		wire[0] = byte(prefixLen)
		copy(wire[1:], ipBytes[:prefixBytes])
	}

	return hex.EncodeToString(wire)
}

// getYANG returns the embedded YANG schema.
func getYANG() string {
	return adjyang.ZeAdjRibInAPIYANG
}
