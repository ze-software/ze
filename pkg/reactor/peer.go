package reactor

import (
	"context"
	"encoding/hex"
	"errors"
	"fmt"
	"net"
	"net/netip"
	"sort"
	"sync"
	"sync/atomic"
	"time"

	"codeberg.org/thomas-mangin/zebgp/pkg/api"
	"codeberg.org/thomas-mangin/zebgp/pkg/bgp/attribute"
	"codeberg.org/thomas-mangin/zebgp/pkg/bgp/capability"
	bgpctx "codeberg.org/thomas-mangin/zebgp/pkg/bgp/context"
	"codeberg.org/thomas-mangin/zebgp/pkg/bgp/fsm"
	"codeberg.org/thomas-mangin/zebgp/pkg/bgp/message"
	"codeberg.org/thomas-mangin/zebgp/pkg/bgp/nlri"
	"codeberg.org/thomas-mangin/zebgp/pkg/rib"
	"codeberg.org/thomas-mangin/zebgp/pkg/source"
	"codeberg.org/thomas-mangin/zebgp/pkg/trace"
)

// safiMUP is the SAFI for Mobile User Plane (draft-mpmz-bess-mup-safi).
// Not in capability package as it's not yet standardized (SAFI 85).
const safiMUP = 85

// PeerState represents the high-level state of a peer.
type PeerState int32

const (
	// PeerStateStopped means the peer is not running.
	PeerStateStopped PeerState = iota
	// PeerStateConnecting means the peer is attempting to connect.
	PeerStateConnecting
	// PeerStateActive means the peer is waiting for incoming connection.
	PeerStateActive
	// PeerStateEstablished means the BGP session is established.
	PeerStateEstablished
)

func (s PeerState) String() string {
	switch s {
	case PeerStateStopped:
		return "Stopped"
	case PeerStateConnecting:
		return "Connecting"
	case PeerStateActive:
		return "Active"
	case PeerStateEstablished:
		return "Established"
	default:
		return "Unknown"
	}
}

// Default reconnect delays.
const (
	DefaultReconnectMin = 5 * time.Second
	DefaultReconnectMax = 60 * time.Second
)

// Next-hop resolution errors.
var (
	// ErrNextHopUnset is returned when RouteNextHop has zero-value policy.
	ErrNextHopUnset = errors.New("next-hop policy not set")

	// ErrNextHopSelfNoLocal is returned when Self policy is used but
	// LocalAddress is not configured in peer settings.
	ErrNextHopSelfNoLocal = errors.New("next-hop self: no local address configured")

	// ErrNextHopIncompatible is returned when Self address is incompatible
	// with the NLRI family and Extended Next Hop is not negotiated.
	ErrNextHopIncompatible = errors.New("next-hop incompatible with family")
)

// PeerCallback is called when peer state changes.
type PeerCallback func(from, to PeerState)

// PeerOpType identifies the type of queued operation.
type PeerOpType int

const (
	PeerOpAnnounce PeerOpType = iota
	PeerOpWithdraw
	PeerOpTeardown
)

// PeerOp represents a queued operation (announce, withdraw, or teardown).
type PeerOp struct {
	Type    PeerOpType
	Route   *rib.Route // For PeerOpAnnounce
	NLRI    nlri.NLRI  // For PeerOpWithdraw
	Subcode uint8      // For PeerOpTeardown
}

// MaxOpQueueSize is the maximum number of operations that can be queued
// when the session is not established. Prevents unbounded memory growth.
const MaxOpQueueSize = 10000

// Peer wraps a Session with reconnection logic.
//
// It manages the connection lifecycle in its own goroutine,
// automatically reconnecting on failure with exponential backoff.
//
// # Route Queuing Architecture
//
// The peer uses opQueue for ordering when session is not established.
// Maintains strict ordering of announce/withdraw/teardown operations.
// Processed on session establishment, with teardowns acting as batch separators.
//
// When a route is announced:
//   - Session ESTABLISHED → sent immediately
//   - Session NOT ESTABLISHED → queued to opQueue
//
// On session establishment:
//  1. opQueue is processed in order until a teardown is encountered
//  2. Teardown sends EOR + NOTIFICATION, remaining opQueue items persist
//
// Note: Route persistence across reconnects is delegated to external API programs.
// See capability contract for route-refresh handling.
type Peer struct {
	settings *PeerSettings
	session  *Session

	// Negotiated capabilities: tracks which families are enabled.
	// Set when session transitions to Established, cleared on teardown.
	// Encoding details (AddPath, ExtNH, ASN4) live in sendCtx/recvCtx.
	// Uses atomic.Pointer for thread-safe access from multiple goroutines.
	negotiated atomic.Pointer[NegotiatedCapabilities]

	// Encoding contexts for this peer session.
	// Created at session establishment, cleared on teardown.
	// recvCtx is used when parsing routes FROM peer.
	// sendCtx is used when encoding routes TO peer.
	recvCtx   *bgpctx.EncodingContext
	recvCtxID bgpctx.ContextID
	sendCtx   *bgpctx.EncodingContext
	sendCtxID bgpctx.ContextID

	state           atomic.Int32
	callback        PeerCallback
	messageCallback MessageCallback // Called when any BGP message is received

	// Reconnect configuration
	reconnectMin time.Duration
	reconnectMax time.Duration

	// Ordered operation queue: Used when session is NOT established.
	// Maintains strict ordering of announce/withdraw/teardown operations.
	// Processed on session establishment; teardowns act as batch separators.
	opQueue []PeerOp

	// sendingInitialRoutes prevents concurrent sendInitialRoutes goroutines.
	// Set to 1 when sendInitialRoutes starts, 0 when it ends.
	sendingInitialRoutes atomic.Int32

	// API sync for EOR: wait for API processes to finish initial routes before EOR.
	// Reset on each session establishment, signaled by "session api ready" commands.
	apiSyncExpected  int32         // Number of ready signals expected (processes with SendUpdate)
	apiSyncReady     chan struct{} // Closed when all expected ready signals received
	apiSyncReadyOnce sync.Once     // Ensures channel is closed only once
	apiSyncCount     atomic.Int32  // Count of ready signals received since session start

	// Goroutine control
	ctx    context.Context
	cancel context.CancelFunc
	wg     sync.WaitGroup

	mu sync.RWMutex

	// Watchdog runtime state: tracks current announced/withdrawn state per route.
	// Key: watchdog name, Value: map of route key (prefix string) → isAnnounced.
	// Initialized from WatchdogGroups on session establishment.
	watchdogState map[string]map[string]bool

	// Global watchdog manager for API-created pools.
	// Set by reactor when peer is added. Used to re-send pool routes on reconnect.
	globalWatchdog *WatchdogManager

	// reactor is set when peer is added to reactor.
	// Used to notify reactor of state changes.
	reactor *Reactor

	// sourceID identifies this peer in the source registry.
	// Assigned at creation, never changes.
	sourceID source.SourceID

	// Collision detection (RFC 4271 §6.8):
	// When an incoming connection arrives while we're in OpenConfirm,
	// we queue it here and wait for its OPEN to resolve the collision.
	pendingConn net.Conn      // Pending incoming connection
	pendingOpen *message.Open // OPEN received on pending connection
}

// NewPeer creates a new peer for the given settings.
func NewPeer(settings *PeerSettings) *Peer {
	p := &Peer{
		settings:      settings,
		reconnectMin:  DefaultReconnectMin,
		reconnectMax:  DefaultReconnectMax,
		opQueue:       make([]PeerOp, 0, 16), // Pre-allocate small capacity
		watchdogState: make(map[string]map[string]bool),
		sourceID:      source.DefaultRegistry.RegisterPeer(settings.Address, settings.PeerAS),
	}

	// Initialize watchdog state from config
	for name, routes := range settings.WatchdogGroups {
		p.watchdogState[name] = make(map[string]bool)
		for i := range routes {
			wr := &routes[i]
			p.watchdogState[name][wr.RouteKey()] = !wr.InitiallyWithdrawn
		}
	}

	return p
}

// Settings returns the configured peer settings.
func (p *Peer) Settings() *PeerSettings {
	return p.settings
}

// SourceID returns the unique source ID for this peer.
func (p *Peer) SourceID() source.SourceID {
	return p.sourceID
}

// ResetAPISync resets the per-session API synchronization state.
// Called when session transitions to Established.
// expectedCount is the number of API processes with SendUpdate permission.
func (p *Peer) ResetAPISync(expectedCount int) {
	p.mu.Lock()
	p.apiSyncExpected = int32(expectedCount) //nolint:gosec // API process count will never overflow int32
	p.apiSyncReady = make(chan struct{})
	p.apiSyncReadyOnce = sync.Once{}
	p.apiSyncCount.Store(0)
	p.mu.Unlock()
}

// SignalAPIReady is called when "session api ready" is received for this peer.
// When all expected signals are received, unblocks waitForAPISync.
func (p *Peer) SignalAPIReady() {
	count := p.apiSyncCount.Add(1)
	p.mu.RLock()
	expected := p.apiSyncExpected
	ready := p.apiSyncReady
	p.mu.RUnlock()

	if count >= expected && ready != nil {
		p.mu.Lock()
		p.apiSyncReadyOnce.Do(func() {
			close(p.apiSyncReady)
		})
		p.mu.Unlock()
	}
}

// waitForAPISync blocks until all API processes signal ready or timeout.
// Returns immediately if no API sync is expected.
//
//nolint:unused // Reserved for future API sync implementation
func (p *Peer) waitForAPISync(timeout time.Duration) {
	p.mu.RLock()
	expected := p.apiSyncExpected
	ready := p.apiSyncReady
	p.mu.RUnlock()

	addr := p.settings.Address.String()
	trace.Log(trace.Routes, "peer %s: waiting for API sync (expected=%d)", addr, expected)

	if expected == 0 || ready == nil {
		trace.Log(trace.Routes, "peer %s: no API sync needed", addr)
		return
	}

	select {
	case <-ready:
		trace.Log(trace.Routes, "peer %s: API sync complete", addr)
		return
	case <-time.After(timeout):
		// Timeout - proceed anyway to avoid blocking forever
		trace.Log(trace.Routes, "peer %s: API sync timeout", addr)
		return
	}
}

// RecvContext returns the receive encoding context.
// Used when parsing routes received FROM this peer.
// Returns nil if session is not established.
func (p *Peer) RecvContext() *bgpctx.EncodingContext {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.recvCtx
}

// RecvContextID returns the receive context ID.
// Used for fast compatibility checks.
func (p *Peer) RecvContextID() bgpctx.ContextID {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.recvCtxID
}

// SendContext returns the send encoding context.
// Used when encoding routes TO this peer.
// Returns nil if session is not established.
func (p *Peer) SendContext() *bgpctx.EncodingContext {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.sendCtx
}

// SendContextID returns the send context ID.
// Used for fast compatibility checks.
func (p *Peer) SendContextID() bgpctx.ContextID {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.sendCtxID
}

// setEncodingContexts creates and stores encoding contexts from negotiation.
// Called when session transitions to Established.
func (p *Peer) setEncodingContexts(neg *capability.Negotiated) {
	p.mu.Lock()
	defer p.mu.Unlock()

	p.recvCtx = bgpctx.FromNegotiatedRecv(neg)
	if p.recvCtx != nil {
		p.recvCtxID = bgpctx.Registry.Register(p.recvCtx)
	}

	p.sendCtx = bgpctx.FromNegotiatedSend(neg)
	if p.sendCtx != nil {
		p.sendCtxID = bgpctx.Registry.Register(p.sendCtx)
	}

	// Set recvCtxID on session for zero-copy WireUpdate creation
	if p.session != nil {
		p.session.SetRecvCtxID(p.recvCtxID)
	}
}

// clearEncodingContexts clears the encoding contexts.
// Called when session is torn down.
func (p *Peer) clearEncodingContexts() {
	p.mu.Lock()
	defer p.mu.Unlock()

	p.recvCtx = nil
	p.recvCtxID = 0
	p.sendCtx = nil
	p.sendCtxID = 0
}

// SetGlobalWatchdog sets the global watchdog manager for this peer.
// Called by reactor when peer is added.
func (p *Peer) SetGlobalWatchdog(wm *WatchdogManager) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.globalWatchdog = wm
}

// SetReactor sets the reactor reference.
// Called by Reactor.AddPeer().
func (p *Peer) SetReactor(r *Reactor) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.reactor = r
}

// packContext returns a PackContext for capability-aware encoding.
// RFC 7911: ADD-PATH requires 4-byte path identifier prefix on NLRI.
// RFC 6793: ASN4 determines 2-byte vs 4-byte AS numbers in AS_PATH.
//
// Returns nil only if session not established.
// Uses sendCtx for encoding parameters (ASN4, AddPath per family).
func (p *Peer) packContext(family nlri.Family) *nlri.PackContext {
	if p.sendCtx == nil {
		return nil
	}
	return p.sendCtx.ToPackContext(family)
}

// resolveNextHop returns the actual IP address for a RouteNextHop policy.
// Uses session's LocalAddress for Self, validates against Extended NH capability.
//
// RFC 4271 Section 5.1.3 - NEXT_HOP attribute.
// RFC 5549/8950 - Extended Next Hop Encoding.
func (p *Peer) resolveNextHop(nh api.RouteNextHop, family nlri.Family) (netip.Addr, error) {
	switch nh.Policy {
	case api.NextHopExplicit:
		// Explicit addresses bypass validation - user is responsible.
		// Returns invalid addr without error if that's what was configured.
		return nh.Addr, nil

	case api.NextHopSelf:
		local := p.settings.LocalAddress
		if !local.IsValid() {
			return netip.Addr{}, ErrNextHopSelfNoLocal
		}
		// Validate: can we use this address for this NLRI family?
		if !p.canUseNextHopFor(local, family) {
			return netip.Addr{}, ErrNextHopIncompatible
		}
		return local, nil

	case api.NextHopUnset:
		return netip.Addr{}, ErrNextHopUnset

	default:
		return netip.Addr{}, ErrNextHopUnset
	}
}

// canUseNextHopFor checks if addr is valid as next-hop for family.
// Natural match (IPv4 for IPv4, IPv6 for IPv6) always allowed.
// Cross-family allowed if Extended NH capability negotiated.
//
// RFC 5549/8950: Extended Next Hop Encoding for cross-family next-hops.
func (p *Peer) canUseNextHopFor(addr netip.Addr, family nlri.Family) bool {
	// Natural match - always allowed
	if addr.Is4() && family.AFI == nlri.AFIIPv4 {
		return true
	}
	if addr.Is6() && family.AFI == nlri.AFIIPv6 {
		return true
	}

	// Cross-family via Extended NH (RFC 5549/8950)
	if p.sendCtx != nil {
		nhAFI := p.sendCtx.ExtendedNextHopFor(family)
		if nhAFI != 0 {
			if addr.Is6() && nhAFI == nlri.AFIIPv6 {
				return true
			}
			if addr.Is4() && nhAFI == nlri.AFIIPv4 {
				return true
			}
		}
	}
	return false
}

func toVPLSParams(r VPLSRoute) message.VPLSParams {
	return message.VPLSParams{
		RD: r.RD, Endpoint: r.Endpoint, Base: r.Base, Offset: r.Offset,
		Size: r.Size, NextHop: r.NextHop, Origin: attribute.Origin(r.Origin),
		LocalPreference: r.LocalPreference, MED: r.MED, ASPath: r.ASPath,
		Communities: r.Communities, ExtCommunityBytes: r.ExtCommunityBytes,
		OriginatorID: r.OriginatorID, ClusterList: r.ClusterList,
	}
}

func toFlowSpecParams(r FlowSpecRoute) message.FlowSpecParams {
	return message.FlowSpecParams{
		IsIPv6: r.IsIPv6, RD: r.RD, NLRI: r.NLRI, NextHop: r.NextHop,
		CommunityBytes: r.CommunityBytes, ExtCommunityBytes: r.ExtCommunityBytes,
		IPv6ExtCommunityBytes: r.IPv6ExtCommunityBytes,
	}
}

func toMUPParams(r MUPRoute) message.MUPParams {
	return message.MUPParams{
		RouteType: r.RouteType, IsIPv6: r.IsIPv6, NLRI: r.NLRI,
		NextHop: r.NextHop, ExtCommunityBytes: r.ExtCommunityBytes,
		PrefixSID: r.PrefixSID,
	}
}

func toMVPNParams(routes []MVPNRoute) []message.MVPNParams {
	params := make([]message.MVPNParams, len(routes))
	for i, r := range routes {
		params[i] = message.MVPNParams{
			RouteType: r.RouteType, IsIPv6: r.IsIPv6, RD: r.RD,
			SourceAS: r.SourceAS, Source: r.Source, Group: r.Group,
			NextHop: r.NextHop, Origin: attribute.Origin(r.Origin),
			LocalPreference: r.LocalPreference, MED: r.MED,
			ExtCommunityBytes: r.ExtCommunityBytes,
			OriginatorID:      r.OriginatorID,
			ClusterList:       r.ClusterList,
		}
	}
	return params
}

// toStaticRouteUnicastParams converts a StaticRoute to UnicastParams.
// Used for IPv4/IPv6 unicast routes (not VPN).
// nextHop is the resolved next-hop address (from RouteNextHop policy).
func toStaticRouteUnicastParams(r StaticRoute, nextHop netip.Addr, sendCtx *bgpctx.EncodingContext) message.UnicastParams {
	// RFC 8950: Extended next-hop for cross-AFI next-hop
	var useExtNH bool
	if sendCtx != nil {
		if r.Prefix.Addr().Is4() && nextHop.Is6() {
			useExtNH = sendCtx.ExtendedNextHopFor(nlri.IPv4Unicast) != 0
		} else if r.Prefix.Addr().Is6() && nextHop.Is4() {
			useExtNH = sendCtx.ExtendedNextHopFor(nlri.IPv6Unicast) != 0
		}
	}

	// Pack raw attributes
	rawAttrs := make([][]byte, len(r.RawAttributes))
	for i, ra := range r.RawAttributes {
		rawAttrs[i] = packRawAttribute(ra)
	}

	return message.UnicastParams{
		Prefix:             r.Prefix,
		PathID:             r.PathID,
		NextHop:            nextHop,
		Origin:             attribute.Origin(r.Origin),
		ASPath:             r.ASPath,
		MED:                r.MED,
		LocalPreference:    r.LocalPreference,
		Communities:        r.Communities,
		ExtCommunityBytes:  r.ExtCommunityBytes,
		LargeCommunities:   r.LargeCommunities,
		AtomicAggregate:    r.AtomicAggregate,
		HasAggregator:      r.HasAggregator,
		AggregatorASN:      r.AggregatorASN,
		AggregatorIP:       r.AggregatorIP,
		UseExtendedNextHop: useExtNH,
		RawAttributeBytes:  rawAttrs,
		OriginatorID:       r.OriginatorID,
		ClusterList:        r.ClusterList,
	}
}

// toStaticRouteLabeledUnicastParams converts a StaticRoute to LabeledUnicastParams.
// Used for labeled unicast routes (SAFI 4).
// nextHop is the resolved next-hop address (from RouteNextHop policy).
func toStaticRouteLabeledUnicastParams(r StaticRoute, nextHop netip.Addr) message.LabeledUnicastParams {
	// Pack raw attributes
	rawAttrs := make([][]byte, len(r.RawAttributes))
	for i, ra := range r.RawAttributes {
		rawAttrs[i] = packRawAttribute(ra)
	}

	return message.LabeledUnicastParams{
		Prefix:            r.Prefix,
		PathID:            r.PathID,
		NextHop:           nextHop,
		Labels:            r.Labels,
		Origin:            attribute.Origin(r.Origin),
		ASPath:            r.ASPath,
		MED:               r.MED,
		LocalPreference:   r.LocalPreference,
		Communities:       r.Communities,
		ExtCommunityBytes: r.ExtCommunityBytes,
		LargeCommunities:  r.LargeCommunities,
		AtomicAggregate:   r.AtomicAggregate,
		HasAggregator:     r.HasAggregator,
		AggregatorASN:     r.AggregatorASN,
		AggregatorIP:      r.AggregatorIP,
		OriginatorID:      r.OriginatorID,
		ClusterList:       r.ClusterList,
		PrefixSID:         r.PrefixSIDBytes,
		RawAttributeBytes: rawAttrs,
	}
}

// toStaticRouteVPNParams converts a StaticRoute to VPNParams.
// Used for VPN routes (SAFI 128).
// nextHop is the resolved next-hop address (from RouteNextHop policy).
func toStaticRouteVPNParams(r StaticRoute, nextHop netip.Addr) message.VPNParams {
	return message.VPNParams{
		Prefix:            r.Prefix,
		PathID:            r.PathID,
		NextHop:           nextHop,
		Labels:            r.Labels,
		RDBytes:           r.RDBytes,
		Origin:            attribute.Origin(r.Origin),
		ASPath:            r.ASPath,
		MED:               r.MED,
		LocalPreference:   r.LocalPreference,
		Communities:       r.Communities,
		ExtCommunityBytes: r.ExtCommunityBytes,
		LargeCommunities:  r.LargeCommunities,
		AtomicAggregate:   r.AtomicAggregate,
		HasAggregator:     r.HasAggregator,
		AggregatorASN:     r.AggregatorASN,
		AggregatorIP:      r.AggregatorIP,
		OriginatorID:      r.OriginatorID,
		ClusterList:       r.ClusterList,
	}
}

// buildStaticRouteUpdateNew builds an UPDATE for a static route using UpdateBuilder.
// This is the new implementation that will replace buildStaticRouteUpdate.
// nextHop is the resolved next-hop address (from RouteNextHop policy).
func buildStaticRouteUpdateNew(route StaticRoute, nextHop netip.Addr, localAS uint32, isIBGP bool, ctx *nlri.PackContext, sendCtx *bgpctx.EncodingContext) *message.Update {
	ub := message.NewUpdateBuilder(localAS, isIBGP, ctx)
	if route.IsVPN() {
		return ub.BuildVPN(toStaticRouteVPNParams(route, nextHop))
	}
	if route.IsLabeledUnicast() {
		return ub.BuildLabeledUnicast(toStaticRouteLabeledUnicastParams(route, nextHop))
	}
	return ub.BuildUnicast(toStaticRouteUnicastParams(route, nextHop, sendCtx))
}

// State returns the current peer state.
func (p *Peer) State() PeerState {
	return PeerState(p.state.Load())
}

// setState updates state and calls callback.
func (p *Peer) setState(s PeerState) {
	old := PeerState(p.state.Swap(int32(s)))
	if old != s {
		p.mu.RLock()
		cb := p.callback
		p.mu.RUnlock()
		if cb != nil {
			cb(old, s)
		}
	}
}

// SetCallback sets the state change callback.
func (p *Peer) SetCallback(cb PeerCallback) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.callback = cb
}

// SetReconnectDelay configures reconnection delays.
func (p *Peer) SetReconnectDelay(min, max time.Duration) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.reconnectMin = min
	p.reconnectMax = max
}

// Start begins the peer goroutine with a background context.
func (p *Peer) Start() {
	p.StartWithContext(context.Background())
}

// StartWithContext begins the peer goroutine with the given context.
func (p *Peer) StartWithContext(ctx context.Context) {
	p.mu.Lock()
	if p.cancel != nil {
		p.mu.Unlock()
		return // Already running
	}
	p.ctx, p.cancel = context.WithCancel(ctx)
	p.mu.Unlock()

	p.wg.Add(1)
	go p.run()
}

// Stop signals the peer to stop.
func (p *Peer) Stop() {
	p.mu.Lock()
	cancel := p.cancel
	p.mu.Unlock()

	if cancel != nil {
		cancel()
	}
}

// Teardown sends a Cease NOTIFICATION with the given subcode and closes.
// The session will send NOTIFICATION before closing the connection.
// RFC 4486 defines Cease subcodes; RFC 9003 defines the message format.
// If called when sendInitialRoutes is running, queues the teardown so that
// EOR can be sent before NOTIFICATION. If not connected, also queues.
func (p *Peer) Teardown(subcode uint8) {
	p.mu.Lock()
	session := p.session

	// If sendInitialRoutes is running, queue the teardown so it can send
	// EOR before executing the teardown. This ensures proper BGP protocol
	// sequencing: routes + EOR + NOTIFICATION.
	if p.sendingInitialRoutes.Load() == 1 {
		if len(p.opQueue) < MaxOpQueueSize {
			p.opQueue = append(p.opQueue, PeerOp{Type: PeerOpTeardown, Subcode: subcode})
		} else {
			trace.Log(trace.Routes, "peer %s: opQueue full, dropping teardown", p.settings.Address)
		}
		p.mu.Unlock()
		return
	}

	if session != nil {
		p.mu.Unlock()
		if err := session.Teardown(subcode); err != nil {
			trace.Log(trace.Session, "peer %s: teardown error: %v", p.settings.Address, err)
		}
		// Set state after teardown - there's a brief race window where
		// AnnounceRoute might see ESTABLISHED, but SendUpdate will fail
		// on the closed session (which is correct behavior)
		p.setState(PeerStateConnecting)
	} else {
		// No active session - queue teardown to maintain operation order
		if len(p.opQueue) < MaxOpQueueSize {
			p.opQueue = append(p.opQueue, PeerOp{Type: PeerOpTeardown, Subcode: subcode})
		} else {
			trace.Log(trace.Routes, "peer %s: opQueue full, dropping teardown", p.settings.Address)
		}
		p.mu.Unlock()
	}
}

// QueueAnnounce queues a route announcement for when session establishes.
// Used when session is not established to maintain operation order.
// If queue is full (MaxOpQueueSize), the operation is dropped with a warning.
func (p *Peer) QueueAnnounce(route *rib.Route) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if len(p.opQueue) >= MaxOpQueueSize {
		trace.Log(trace.Routes, "peer %s: opQueue full (%d), dropping announce for %s",
			p.settings.Address, len(p.opQueue), route.NLRI())
		return
	}
	p.opQueue = append(p.opQueue, PeerOp{Type: PeerOpAnnounce, Route: route})
}

// QueueWithdraw queues a route withdrawal for when session establishes.
// Used when session is not established to maintain operation order.
// If queue is full (MaxOpQueueSize), the operation is dropped with a warning.
func (p *Peer) QueueWithdraw(n nlri.NLRI) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if len(p.opQueue) >= MaxOpQueueSize {
		trace.Log(trace.Routes, "peer %s: opQueue full (%d), dropping withdraw for %s",
			p.settings.Address, len(p.opQueue), n)
		return
	}
	p.opQueue = append(p.opQueue, PeerOp{Type: PeerOpWithdraw, NLRI: n})
}

// Wait waits for the peer to stop.
func (p *Peer) Wait(ctx context.Context) error {
	done := make(chan struct{})
	go func() {
		p.wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

// run is the main peer loop.
func (p *Peer) run() {
	defer p.wg.Done()
	defer p.cleanup()

	delay := p.reconnectMin

	for {
		select {
		case <-p.ctx.Done():
			return
		default:
		}

		// Attempt connection
		err := p.runOnce()

		select {
		case <-p.ctx.Done():
			return
		default:
		}

		if err != nil {
			// Check if this was a teardown - reconnect immediately
			if errors.Is(err, ErrTeardown) {
				// Teardown means intentional disconnect, reconnect immediately
				// Reset delay and continue without waiting
				delay = p.reconnectMin
				p.setState(PeerStateConnecting)
				continue
			}

			// Normal error: Backoff before retry
			p.setState(PeerStateConnecting)

			select {
			case <-p.ctx.Done():
				return
			case <-time.After(delay):
			}

			// Exponential backoff
			delay *= 2
			p.mu.RLock()
			maxDelay := p.reconnectMax
			p.mu.RUnlock()
			if delay > maxDelay {
				delay = maxDelay
			}
		} else {
			// Reset delay on successful session
			delay = p.reconnectMin
		}
	}
}

// runOnce attempts a single connection cycle.
func (p *Peer) runOnce() error {
	// Create session
	session := NewSession(p.settings)
	session.onMessageReceived = p.messageCallback
	session.SetSourceID(p.sourceID)

	p.mu.Lock()
	p.session = session
	p.mu.Unlock()

	defer func() {
		p.negotiated.Store(nil) // Clear negotiated capabilities
		p.clearEncodingContexts()
		// Reset sendingInitialRoutes flag so next session can run sendInitialRoutes().
		// This is needed because session.Teardown() may return before the old
		// sendInitialRoutes() goroutine finishes its 500ms sleep.
		p.sendingInitialRoutes.Store(0)
		p.mu.Lock()
		p.session = nil
		p.mu.Unlock()
	}()

	// Update state based on FSM mode
	if p.settings.Passive {
		p.setState(PeerStateActive)
	} else {
		p.setState(PeerStateConnecting)
	}

	// Start FSM
	if err := session.Start(); err != nil {
		return err
	}

	// Connect (for active mode)
	if !p.settings.Passive {
		if err := session.Connect(p.ctx); err != nil {
			return err
		}
	}

	// Monitor FSM state
	session.fsm.SetCallback(func(from, to fsm.State) {
		addr := p.settings.Address.String()
		trace.FSMTransition(addr, from.String(), to.String())

		if to == fsm.StateEstablished {
			// Pre-compute negotiated capabilities for O(1) access during route sending
			neg := session.Negotiated()
			p.negotiated.Store(NewNegotiatedCapabilities(neg))
			p.setEncodingContexts(neg)
			p.setState(PeerStateEstablished)
			trace.SessionEstablished(addr, p.settings.LocalAS, p.settings.PeerAS)

			// Reset per-session API sync: count processes with SendUpdate permission.
			// They will signal "session api ready" after replaying routes.
			apiSendCount := 0
			for _, binding := range p.settings.APIBindings {
				if binding.SendUpdate {
					apiSendCount++
				}
			}
			p.ResetAPISync(apiSendCount)

			// Notify reactor of peer established
			p.mu.RLock()
			reactor := p.reactor
			p.mu.RUnlock()
			if reactor != nil {
				reactor.notifyPeerEstablished(p)
			}

			// Send static routes from config.
			go p.sendInitialRoutes()
		} else if from == fsm.StateEstablished {
			// Determine reason based on target state
			reason := "session closed"
			if to == fsm.StateIdle {
				reason = "connection lost"
			}

			// Notify reactor of peer closed
			p.mu.RLock()
			reactor := p.reactor
			p.mu.RUnlock()
			if reactor != nil {
				reactor.notifyPeerClosed(p, reason)
			}

			// Clear negotiated capabilities and encoding contexts on session teardown
			p.negotiated.Store(nil)
			p.clearEncodingContexts()
			p.setState(PeerStateConnecting)
			trace.SessionClosed(addr, reason)
		}
	})

	// Run session loop
	return session.Run(p.ctx)
}

// AcceptConnection accepts an incoming TCP connection for this peer.
// Used by the reactor to hand incoming connections to passive peers.
func (p *Peer) AcceptConnection(conn net.Conn) error {
	p.mu.RLock()
	session := p.session
	p.mu.RUnlock()

	if session == nil {
		return ErrNotConnected
	}

	return session.Accept(conn)
}

// SessionState returns the current FSM state of the session.
// Returns StateIdle if no session exists.
func (p *Peer) SessionState() fsm.State {
	p.mu.RLock()
	session := p.session
	p.mu.RUnlock()

	if session == nil {
		return fsm.StateIdle
	}
	return session.State()
}

// SetPendingConnection queues an incoming connection for collision resolution.
// RFC 4271 §6.8: Used when we're in OpenConfirm and an incoming connection arrives.
// Returns error if there's already a pending connection.
func (p *Peer) SetPendingConnection(conn net.Conn) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.pendingConn != nil {
		return errors.New("pending connection already exists")
	}
	p.pendingConn = conn
	p.pendingOpen = nil
	return nil
}

// ClearPendingConnection clears any pending connection.
func (p *Peer) ClearPendingConnection() {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.pendingConn = nil
	p.pendingOpen = nil
}

// HasPendingConnection returns true if there's a pending incoming connection.
func (p *Peer) HasPendingConnection() bool {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.pendingConn != nil
}

// ResolvePendingCollision resolves a collision after receiving OPEN on pending connection.
// RFC 4271 §6.8: Compare BGP IDs and close the loser.
//
// Returns:
//   - acceptPending: true if pending connection should be accepted
//   - the pending connection (caller must handle it)
//   - the pending OPEN message (if acceptPending is true)
func (p *Peer) ResolvePendingCollision(pendingOpen *message.Open) (acceptPending bool, conn net.Conn, open *message.Open) {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.pendingConn == nil {
		return false, nil, nil
	}

	conn = p.pendingConn
	session := p.session

	if session == nil {
		// Session gone, reject pending
		p.pendingConn = nil
		p.pendingOpen = nil
		return false, conn, nil
	}

	shouldAccept, shouldCloseExisting := session.DetectCollision(pendingOpen.BGPIdentifier)

	if shouldAccept && shouldCloseExisting {
		// Remote wins: close existing, accept pending
		// Store the OPEN so we can replay it
		p.pendingOpen = pendingOpen
		p.pendingConn = nil

		// Close existing session with NOTIFICATION
		// The session's Run loop will exit and we can accept pending
		go func() {
			_ = session.CloseWithNotification(message.NotifyCease, message.NotifyCeaseConnectionCollision)
		}()

		return true, conn, pendingOpen
	}

	// Local wins: reject pending, keep existing
	p.pendingConn = nil
	p.pendingOpen = nil
	return false, conn, nil
}

// AcceptConnectionWithOpen accepts an incoming connection with a pre-received OPEN.
// RFC 4271 §6.8: Used after collision resolution when the pending connection wins.
func (p *Peer) AcceptConnectionWithOpen(conn net.Conn, peerOpen *message.Open) error {
	p.mu.RLock()
	session := p.session
	p.mu.RUnlock()

	if session == nil {
		return ErrNotConnected
	}

	return session.AcceptWithOpen(conn, peerOpen)
}

// SendUpdate sends a BGP UPDATE message to this peer.
// Returns ErrNotConnected if no session is active.
// Returns an error if the session is not in ESTABLISHED state.
func (p *Peer) SendUpdate(update *message.Update) error {
	p.mu.RLock()
	session := p.session
	p.mu.RUnlock()

	if session == nil {
		return ErrNotConnected
	}

	return session.SendUpdate(update)
}

// SendRawUpdateBody sends a pre-encoded UPDATE message body (without BGP header).
// Used for zero-copy forwarding when encoding contexts match.
// Returns ErrNotConnected if no session is active.
func (p *Peer) SendRawUpdateBody(body []byte) error {
	p.mu.RLock()
	session := p.session
	p.mu.RUnlock()

	if session == nil {
		return ErrNotConnected
	}

	return session.SendRawUpdateBody(body)
}

// SendRawMessage sends raw bytes to the peer.
// If msgType is 0, payload is a full BGP packet.
// If msgType is non-zero, payload is message body only.
func (p *Peer) SendRawMessage(msgType uint8, payload []byte) error {
	p.mu.RLock()
	session := p.session
	p.mu.RUnlock()

	if session == nil {
		return ErrNotConnected
	}

	return session.SendRawMessage(msgType, payload)
}

// messageNegotiated returns message.Negotiated for use with CommitService.
// Returns nil if session is not established.
func (p *Peer) messageNegotiated() *message.Negotiated {
	p.mu.RLock()
	session := p.session
	p.mu.RUnlock()

	if session == nil {
		return nil
	}

	neg := session.Negotiated()
	if neg == nil {
		return nil
	}

	msgNeg := &message.Negotiated{
		ASN4:    neg.ASN4,
		LocalAS: neg.LocalASN,
		PeerAS:  neg.PeerASN,
	}

	// Populate ADD-PATH send capability per family (RFC 7911)
	// We can send with ADD-PATH if mode is Send or Both
	for _, f := range neg.Families() {
		mode := neg.AddPathMode(f)
		if mode == capability.AddPathSend || mode == capability.AddPathBoth {
			if msgNeg.AddPath == nil {
				msgNeg.AddPath = make(map[message.Family]bool)
			}
			msgNeg.AddPath[message.Family{AFI: uint16(f.AFI), SAFI: uint8(f.SAFI)}] = true
		}
	}

	return msgNeg
}

// cleanup runs when peer stops.
func (p *Peer) cleanup() {
	p.negotiated.Store(nil) // Clear negotiated capabilities
	p.clearEncodingContexts()
	p.mu.Lock()
	if p.session != nil {
		_ = p.session.Close()
		p.session = nil
	}
	p.cancel = nil
	p.mu.Unlock()

	p.setState(PeerStateStopped)
}

// sendInitialRoutes sends static routes configured for this peer.
// Routes with identical attributes are grouped into a single UPDATE message.
// Uses atomic flag to prevent concurrent execution if session reconnects quickly.
func (p *Peer) sendInitialRoutes() {
	addr := p.settings.Address.String()

	// Prevent concurrent sendInitialRoutes goroutines.
	// If another instance is running, skip this one - the running instance
	// will process any queued operations.
	if !p.sendingInitialRoutes.CompareAndSwap(0, 1) {
		trace.Log(trace.Routes, "peer %s: sendInitialRoutes skipped (concurrent instance)", addr)
		return
	}
	defer p.sendingInitialRoutes.Store(0)

	trace.Log(trace.Routes, "peer %s: sendInitialRoutes started", addr)

	// Get negotiated capabilities for family checks.
	nc := p.negotiated.Load()
	if nc == nil {
		trace.Log(trace.Routes, "peer %s: sendInitialRoutes aborted (no negotiated caps)", addr)
		return
	}

	trace.Log(trace.Routes, "peer %s: sending %d static routes", addr, len(p.settings.StaticRoutes))

	// Calculate max message size for this peer
	maxMsgSize := int(message.MaxMessageLength(message.TypeUPDATE, nc.ExtendedMessage))

	// Send routes - either grouped or individually based on config.
	if p.settings.GroupUpdates {
		// Group routes by attributes (same attributes = same UPDATE).
		groups := groupRoutesByAttributes(p.settings.StaticRoutes)

		for _, routes := range groups {
			ctx := p.packContext(routeFamily(routes[0]))
			if len(routes) == 1 {
				// Single-route group (IPv6, VPN, LabeledUnicast, or solo IPv4)
				// Resolve next-hop from RouteNextHop policy
				nextHop, nhErr := p.resolveNextHop(routes[0].NextHop, routeFamily(routes[0]))
				if nhErr != nil {
					trace.Log(trace.Routes, "peer %s: next-hop resolution failed: %v", addr, nhErr)
					continue
				}
				update := buildStaticRouteUpdateNew(routes[0], nextHop, p.settings.LocalAS, p.settings.IsIBGP(), ctx, p.sendCtx)
				if err := p.sendUpdateWithSplit(update, maxMsgSize, routeFamily(routes[0])); err != nil {
					trace.Log(trace.Routes, "peer %s: send error: %v", addr, err)
					break
				}
			} else {
				// Multi-route group - IPv4 unicast only (routeGroupKey ensures this)
				// Use size-aware builder to respect max message size
				ub := message.NewUpdateBuilder(p.settings.LocalAS, p.settings.IsIBGP(), ctx)
				params := make([]message.UnicastParams, 0, len(routes))
				for _, r := range routes {
					nextHop, nhErr := p.resolveNextHop(r.NextHop, routeFamily(r))
					if nhErr != nil {
						trace.Log(trace.Routes, "peer %s: next-hop resolution failed for %s: %v", addr, r.Prefix, nhErr)
						continue
					}
					params = append(params, toStaticRouteUnicastParams(r, nextHop, p.sendCtx))
				}
				if len(params) == 0 {
					continue
				}
				updates, err := ub.BuildGroupedUnicastWithLimit(params, maxMsgSize)
				if err != nil {
					trace.Log(trace.Routes, "peer %s: build error: %v", addr, err)
					break
				}
				for _, update := range updates {
					if err := p.SendUpdate(update); err != nil {
						trace.Log(trace.Routes, "peer %s: send error: %v", addr, err)
						break
					}
				}
			}
			for _, route := range routes {
				trace.RouteSent(addr, route.Prefix.String(), route.NextHop.String())
			}
		}
	} else {
		// Send each route in its own UPDATE.
		for _, route := range p.settings.StaticRoutes {
			// Resolve next-hop from RouteNextHop policy
			nextHop, nhErr := p.resolveNextHop(route.NextHop, routeFamily(route))
			if nhErr != nil {
				trace.Log(trace.Routes, "peer %s: next-hop resolution failed for %s: %v", addr, route.Prefix, nhErr)
				continue
			}
			ctx := p.packContext(routeFamily(route))
			update := buildStaticRouteUpdateNew(route, nextHop, p.settings.LocalAS, p.settings.IsIBGP(), ctx, p.sendCtx)
			if err := p.sendUpdateWithSplit(update, maxMsgSize, routeFamily(route)); err != nil {
				trace.Log(trace.Routes, "peer %s: send error: %v", addr, err)
				break
			}
			trace.RouteSent(addr, route.Prefix.String(), route.NextHop.String())
		}
	}

	// Handle watchdog routes (routes controlled via "watchdog announce/withdraw" API).
	// State is eagerly initialized in NewPeer() and persists across reconnects.
	// Send routes based on current state (which may have been modified by API while disconnected).
	if len(p.settings.WatchdogGroups) > 0 {
		for name, routes := range p.settings.WatchdogGroups {
			for i := range routes {
				wr := &routes[i]
				routeKey := wr.RouteKey()

				p.mu.RLock()
				isAnnounced := p.watchdogState[name][routeKey]
				p.mu.RUnlock()

				if !isAnnounced {
					trace.Log(trace.Routes, "peer %s: watchdog %s: holding route %s", addr, name, routeKey)
					continue
				}

				// Send the route - resolve next-hop from RouteNextHop policy
				nextHop, nhErr := p.resolveNextHop(wr.StaticRoute.NextHop, routeFamily(wr.StaticRoute))
				if nhErr != nil {
					trace.Log(trace.Routes, "peer %s: watchdog %s: next-hop resolution failed: %v", addr, name, nhErr)
					continue
				}
				ctx := p.packContext(routeFamily(wr.StaticRoute))
				update := buildStaticRouteUpdateNew(wr.StaticRoute, nextHop, p.settings.LocalAS, p.settings.IsIBGP(), ctx, p.sendCtx)
				if err := p.sendUpdateWithSplit(update, maxMsgSize, routeFamily(wr.StaticRoute)); err != nil {
					trace.Log(trace.Routes, "peer %s: send error: %v", addr, err)
					break
				}
				trace.RouteSent(addr, routeKey, wr.StaticRoute.NextHop.String())
			}
		}
		trace.Log(trace.Routes, "peer %s: sent watchdog routes from %d groups", addr, len(p.settings.WatchdogGroups))
	}

	// Re-send global watchdog pool routes that were announced for this peer.
	// These are API-created routes that persist across reconnects.
	p.mu.RLock()
	globalWatchdog := p.globalWatchdog
	p.mu.RUnlock()

	if globalWatchdog != nil {
		poolNames := globalWatchdog.PoolNames()
		for _, poolName := range poolNames {
			pool := globalWatchdog.GetPool(poolName)
			if pool == nil {
				continue
			}
			for _, pr := range pool.Routes() {
				if !pr.IsAnnounced(addr) {
					continue
				}
				// Resolve next-hop from RouteNextHop policy
				route := pr.StaticRoute
				nextHop, nhErr := p.resolveNextHop(route.NextHop, routeFamily(route))
				if nhErr != nil {
					trace.Log(trace.Routes, "peer %s: global pool %s: next-hop resolution failed: %v", addr, poolName, nhErr)
					continue
				}
				ctx := p.packContext(routeFamily(route))
				update := buildStaticRouteUpdateNew(route, nextHop, p.settings.LocalAS, p.settings.IsIBGP(), ctx, p.sendCtx)
				if err := p.sendUpdateWithSplit(update, maxMsgSize, routeFamily(route)); err != nil {
					trace.Log(trace.Routes, "peer %s: send error: %v", addr, err)
					break
				}
				trace.Log(trace.Routes, "peer %s: re-sent global pool route %s from pool %s", addr, pr.RouteKey(), poolName)
			}
		}
	}

	// Wait for API processes to send initial routes before processing queue.
	// Only delay if there are API processes that may send routes (SendUpdate permission).
	// This prevents unnecessary delay for tests without persist/route-injection APIs.
	p.mu.RLock()
	needsAPIWait := p.apiSyncExpected > 0
	p.mu.RUnlock()
	if needsAPIWait {
		trace.Log(trace.Routes, "peer %s: sleeping 500ms for API routes", addr)
		time.Sleep(500 * time.Millisecond)
		trace.Log(trace.Routes, "peer %s: woke from sleep, processing queue", addr)
	}

	// Process operation queue in order (maintains announce/withdraw/teardown ordering).
	// Stop at first teardown - remaining items stay for next session.
	//
	// CONCURRENCY NOTE: opQueue is append-only from other goroutines (QueueAnnounce,
	// QueueWithdraw, Teardown). We capture the slice at loop start via range; new items
	// appended while unlocked go to the end. The processed count remains valid because
	// the first N items in current opQueue match what we processed from the captured slice.
	var teardownSubcode uint8
	hasTeardown := false

	// Pre-compute max message size for size checking in PeerOpAnnounce
	opMaxMsgSize := int(message.MaxMessageLength(message.TypeUPDATE, nc.ExtendedMessage))

	p.mu.Lock()
	queueLen := len(p.opQueue)
	processed := 0
	for i, op := range p.opQueue {
		switch op.Type {
		case PeerOpTeardown:
			teardownSubcode = op.Subcode
			hasTeardown = true
			processed = i + 1 // Include the teardown in processed count

		case PeerOpAnnounce:
			// Send route, splitting if needed.
			family := op.Route.NLRI().Family()
			ctx := p.packContext(family)
			update := buildRIBRouteUpdate(op.Route, p.settings.LocalAS, p.settings.IsIBGP(), ctx)
			p.mu.Unlock()
			if err := p.sendUpdateWithSplit(update, opMaxMsgSize, family); err != nil {
				trace.Log(trace.Routes, "peer %s: send error for queued route %s: %v", addr, op.Route.NLRI(), err)
				p.mu.Lock()
				// Split errors: skip route
				// Connection errors: stop processing
				if errors.Is(err, message.ErrAttributesTooLarge) || errors.Is(err, message.ErrNLRITooLarge) {
					processed = i + 1
					continue
				}
				break
			}
			p.mu.Lock()
			processed = i + 1
			continue

		case PeerOpWithdraw:
			// Send withdrawal.
			// Use sendUpdateWithSplit for consistency, though single withdrawals rarely need splitting
			family := op.NLRI.Family()
			ctx := p.packContext(family)
			update := buildWithdrawNLRI(op.NLRI, ctx)
			p.mu.Unlock()
			if err := p.sendUpdateWithSplit(update, opMaxMsgSize, family); err != nil {
				trace.Log(trace.Routes, "peer %s: send error for withdrawal %s: %v", addr, op.NLRI, err)
				p.mu.Lock()
				// Split errors are unlikely for single withdrawals, but handle consistently
				if errors.Is(err, message.ErrAttributesTooLarge) || errors.Is(err, message.ErrNLRITooLarge) {
					processed = i + 1
					continue // Skip this withdrawal, try next
				}
				break // Connection error, stop processing
			}
			p.mu.Lock()
			processed = i + 1
			continue
		}

		// If we get here, it was a teardown - break out of loop
		break
	}
	// Remove processed items from queue
	if processed > 0 {
		p.opQueue = p.opQueue[processed:]
	}
	p.mu.Unlock()

	if queueLen > 0 {
		trace.Log(trace.Routes, "peer %s: processed %d queue ops, %d remaining, teardown=%v",
			addr, processed, queueLen-processed, hasTeardown)
	}

	// If teardown was in queue, send EOR first, then execute teardown.
	// EOR must be sent BEFORE NOTIFICATION per RFC 4724 Section 4.
	if hasTeardown {
		// Send EOR for ALL negotiated families before teardown
		for _, family := range nc.Families() {
			_ = p.SendUpdate(message.BuildEOR(family))
			trace.Log(trace.Routes, "peer %s: sent %s EOR (before teardown)", addr, family)
		}

		trace.Log(trace.Routes, "peer %s: executing queued teardown (subcode=%d)", addr, teardownSubcode)
		p.mu.RLock()
		session := p.session
		p.mu.RUnlock()
		if session != nil {
			// Set state to Connecting BEFORE Teardown to avoid race condition:
			// Teardown closes TCP, peer immediately reconnects, but if peer.State()
			// still shows Established, the new connection is rejected by collision check.
			// The FSM callback will also set this, but may fire too late.
			p.setState(PeerStateConnecting)
			if err := session.Teardown(teardownSubcode); err != nil {
				trace.Log(trace.Routes, "peer %s: teardown error: %v", addr, err)
			}
		}
		// Clear remaining opQueue - these routes were never sent, so shouldn't
		// be re-sent on reconnection. Persist plugin tracks actually-sent routes.
		p.mu.Lock()
		if len(p.opQueue) > 0 {
			trace.Log(trace.Routes, "peer %s: clearing %d unsent queue items after teardown", addr, len(p.opQueue))
			p.opQueue = p.opQueue[:0]
		}
		p.mu.Unlock()
		// Reset flag BEFORE return so new session's sendInitialRoutes can run.
		// Teardown triggers reconnection; new session may call sendInitialRoutes
		// before this defer runs, causing the new call to be skipped.
		p.sendingInitialRoutes.Store(0)
		return // Don't send family-specific routes after teardown
	}

	// Send family-specific routes (config-originated)
	p.sendMVPNRoutes()
	p.sendVPLSRoutes()
	p.sendFlowSpecRoutes()
	p.sendMUPRoutes()

	// Send EOR for ALL negotiated families per RFC 4724 Section 4.
	// RFC 4724: "including the case when there is no update to send"
	// IMPORTANT: EORs must be sent AFTER all routes for each family.
	// Families() returns families in deterministic order (sorted by AFI, then SAFI).
	for _, family := range nc.Families() {
		_ = p.SendUpdate(message.BuildEOR(family))
		trace.Log(trace.Routes, "peer %s: sent %s EOR", addr, family)
	}
}

// buildRIBRouteUpdate builds an UPDATE message from a RIB route.
// Used for re-announcing routes from Adj-RIB-Out on session re-establishment.
// Rebuilds the full set of required attributes since rib.Route may not store all.
// RFC 7911: ctx provides ADD-PATH capability state for NLRI encoding.
// RFC 6793: ctx.ASN4 determines 2-byte vs 4-byte AS numbers in AS_PATH.
func buildRIBRouteUpdate(route *rib.Route, localAS uint32, isIBGP bool, ctx *nlri.PackContext) *message.Update {
	var attrBytes []byte

	// Extract ASN4 from context (default to true if nil)
	asn4 := ctx == nil || ctx.ASN4

	// 1. ORIGIN - use stored or default to IGP
	foundOrigin := false
	for _, attr := range route.Attributes() {
		if _, ok := attr.(attribute.Origin); ok {
			attrBytes = append(attrBytes, attribute.PackAttribute(attr)...)
			foundOrigin = true
			break
		}
	}
	if !foundOrigin {
		attrBytes = append(attrBytes, attribute.PackAttribute(attribute.OriginIGP)...)
	}

	// 2. AS_PATH - use stored or build appropriate default
	storedASPath := route.ASPath()
	hasStoredASPath := storedASPath != nil && len(storedASPath.Segments) > 0

	switch {
	case hasStoredASPath:
		attrBytes = append(attrBytes, attribute.PackASPathAttribute(storedASPath, asn4)...)
	case isIBGP || localAS == 0:
		// iBGP or LocalAS not set: empty AS_PATH
		emptyASPath := &attribute.ASPath{Segments: nil}
		attrBytes = append(attrBytes, attribute.PackASPathAttribute(emptyASPath, asn4)...)
	default:
		// eBGP: prepend local AS
		ebgpASPath := &attribute.ASPath{
			Segments: []attribute.ASPathSegment{{
				Type: attribute.ASSequence,
				ASNs: []uint32{localAS},
			}},
		}
		attrBytes = append(attrBytes, attribute.PackASPathAttribute(ebgpASPath, asn4)...)
	}

	// Determine NLRI handling based on address family
	routeNLRI := route.NLRI()
	family := routeNLRI.Family()
	var nlriBytes []byte

	switch {
	case family.AFI == nlri.AFIIPv4 && family.SAFI == nlri.SAFIUnicast:
		// 3. NEXT_HOP for IPv4 unicast
		nh := &attribute.NextHop{Addr: route.NextHop()}
		attrBytes = append(attrBytes, attribute.PackAttribute(nh)...)

		// 4. MED if present (before LOCAL_PREF per RFC order)
		for _, attr := range route.Attributes() {
			if med, ok := attr.(attribute.MED); ok {
				attrBytes = append(attrBytes, attribute.PackAttribute(med)...)
				break
			}
		}

		// 5. LOCAL_PREF for iBGP - use stored value or default to 100
		if isIBGP {
			foundLocalPref := false
			for _, attr := range route.Attributes() {
				if lp, ok := attr.(attribute.LocalPref); ok {
					attrBytes = append(attrBytes, attribute.PackAttribute(lp)...)
					foundLocalPref = true
					break
				}
			}
			if !foundLocalPref {
				attrBytes = append(attrBytes, attribute.PackAttribute(attribute.LocalPref(100))...)
			}
		}

		// IPv4 unicast: use inline NLRI field
		// RFC 7911: Pack uses ADD-PATH encoding when negotiated
		nlriBytes = routeNLRI.Pack(ctx)
	default:
		// Other families: MP_REACH_NLRI goes at end (after all other attributes)
		// Build it now but append after LOCAL_PREF
		mpReach := &attribute.MPReachNLRI{
			AFI:      attribute.AFI(family.AFI),
			SAFI:     attribute.SAFI(family.SAFI),
			NextHops: []netip.Addr{route.NextHop()},
			NLRI:     routeNLRI.Pack(ctx),
		}

		// MED if present (before LOCAL_PREF per RFC order)
		for _, attr := range route.Attributes() {
			if med, ok := attr.(attribute.MED); ok {
				attrBytes = append(attrBytes, attribute.PackAttribute(med)...)
				break
			}
		}

		// LOCAL_PREF for iBGP - use stored value or default to 100
		if isIBGP {
			foundLocalPref := false
			for _, attr := range route.Attributes() {
				if lp, ok := attr.(attribute.LocalPref); ok {
					attrBytes = append(attrBytes, attribute.PackAttribute(lp)...)
					foundLocalPref = true
					break
				}
			}
			if !foundLocalPref {
				attrBytes = append(attrBytes, attribute.PackAttribute(attribute.LocalPref(100))...)
			}
		}

		// MP_REACH_NLRI at end (after all other path attributes)
		// RFC 7911: Pack uses ADD-PATH encoding when negotiated
		attrBytes = append(attrBytes, attribute.PackAttribute(mpReach)...)
	}

	// Copy optional attributes from stored route (communities, etc.)
	for _, attr := range route.Attributes() {
		switch attr.(type) {
		case attribute.Origin, *attribute.ASPath, *attribute.NextHop, attribute.LocalPref, attribute.MED:
			// Already handled above
			continue
		case attribute.Communities,
			attribute.ExtendedCommunities, attribute.LargeCommunities,
			attribute.IPv6ExtendedCommunities,
			attribute.AtomicAggregate, *attribute.Aggregator,
			attribute.OriginatorID, attribute.ClusterList:
			// Pack optional attributes
			attrBytes = append(attrBytes, attribute.PackAttribute(attr)...)
		}
	}

	return &message.Update{
		PathAttributes: attrBytes,
		NLRI:           nlriBytes,
	}
}

// sendUpdateWithSplit sends an UPDATE, splitting if it exceeds maxSize.
// Uses SplitUpdateWithAddPath to chunk oversized messages into multiple UPDATEs.
// The family parameter is used to determine Add-Path state for correct NLRI parsing.
// Returns nil on success, first error encountered on failure.
//
// RFC 4271 Section 4.3: Each split UPDATE is self-contained with full attributes.
// RFC 7911: Add-Path requires 4-byte path identifier before each NLRI.
// RFC 8654: Respects peer's max message size (4096 or 65535).
func (p *Peer) sendUpdateWithSplit(update *message.Update, maxSize int, family nlri.Family) error {
	// Determine Add-Path state for this family using sendCtx
	// RFC 7911: Add-Path is negotiated per AFI/SAFI
	addPath := p.sendCtx != nil && p.sendCtx.AddPath[family]

	chunks, err := message.SplitUpdateWithAddPath(update, maxSize, addPath)
	if err != nil {
		// Attributes too large or single NLRI too large - cannot send
		return fmt.Errorf("splitting update: %w", err)
	}

	for _, chunk := range chunks {
		if err := p.SendUpdate(chunk); err != nil {
			return err
		}
	}
	return nil
}

// buildWithdrawNLRI builds an UPDATE message to withdraw an NLRI.
// RFC 4760: IPv4 unicast uses WithdrawnRoutes, others use MP_UNREACH_NLRI.
// RFC 7911: ctx provides ADD-PATH capability state for NLRI encoding.
func buildWithdrawNLRI(n nlri.NLRI, ctx *nlri.PackContext) *message.Update {
	family := n.Family()

	if family.AFI == nlri.AFIIPv4 && family.SAFI == nlri.SAFIUnicast {
		// IPv4 unicast: use WithdrawnRoutes field
		// RFC 7911: Pack uses ADD-PATH encoding when negotiated
		return &message.Update{
			WithdrawnRoutes: n.Pack(ctx),
		}
	}

	// Other families: use MP_UNREACH_NLRI attribute
	// RFC 7911: Pack uses ADD-PATH encoding when negotiated
	mpUnreach := &attribute.MPUnreachNLRI{
		AFI:  attribute.AFI(family.AFI),
		SAFI: attribute.SAFI(family.SAFI),
		NLRI: n.Pack(ctx),
	}

	return &message.Update{
		PathAttributes: attribute.PackAttribute(mpUnreach),
	}
}

// buildStaticRouteWithdraw builds a withdrawal UPDATE for a static route.
// Handles VPN, labeled-unicast, IPv4 unicast, and IPv6 unicast correctly.
// RFC 7911: ctx provides ADD-PATH capability state for NLRI encoding.
func buildStaticRouteWithdraw(route StaticRoute, ctx *nlri.PackContext) *message.Update {
	switch {
	case route.IsVPN():
		// VPN route: use MP_UNREACH_NLRI with RD + prefix
		// VPN doesn't use Pack() - manual NLRI construction
		return buildMPUnreachVPN(route)
	case route.IsLabeledUnicast():
		// Labeled unicast: use MP_UNREACH_NLRI with label + prefix
		return buildMPUnreachLabeledUnicast(route, ctx)
	case route.Prefix.Addr().Is4():
		// IPv4 unicast: use WithdrawnRoutes field
		// RFC 7911: Pack uses ADD-PATH encoding when negotiated
		inet := nlri.NewINET(nlri.Family{AFI: nlri.AFIIPv4, SAFI: nlri.SAFIUnicast}, route.Prefix, route.PathID)
		return &message.Update{
			WithdrawnRoutes: inet.Pack(ctx),
		}
	default:
		// IPv6 unicast: use MP_UNREACH_NLRI
		// RFC 7911: Pack uses ADD-PATH encoding when negotiated
		inet := nlri.NewINET(nlri.Family{AFI: nlri.AFIIPv6, SAFI: nlri.SAFIUnicast}, route.Prefix, route.PathID)
		mpUnreach := &attribute.MPUnreachNLRI{
			AFI:  attribute.AFI(nlri.AFIIPv6),
			SAFI: attribute.SAFI(nlri.SAFIUnicast),
			NLRI: inet.Pack(ctx),
		}
		return &message.Update{
			PathAttributes: attribute.PackAttribute(mpUnreach),
		}
	}
}

// buildMPUnreachVPN builds MP_UNREACH_NLRI for VPN route withdrawal.
func buildMPUnreachVPN(route StaticRoute) *message.Update {
	// Determine AFI from prefix
	var afi nlri.AFI
	if route.Prefix.Addr().Is4() {
		afi = nlri.AFIIPv4
	} else {
		afi = nlri.AFIIPv6
	}

	// Build labeled VPN NLRI: label (3 bytes) + RD (8 bytes) + prefix
	var nlriBytes []byte

	// Label: use route.SingleLabel() or withdraw label (0x800000)
	// RFC 8277: Withdrawal uses single label regardless of original stack
	label := route.SingleLabel()
	if label == 0 {
		label = 0x800000 // Withdraw label
	}
	nlriBytes = append(nlriBytes, byte(label>>16), byte(label>>8), byte(label)|0x01) // Bottom of stack

	// RD (convert [8]byte to slice)
	nlriBytes = append(nlriBytes, route.RDBytes[:]...)

	// Prefix
	prefixBits := route.Prefix.Bits()
	prefixBytes := (prefixBits + 7) / 8
	addr := route.Prefix.Addr()
	if addr.Is4() {
		a4 := addr.As4()
		nlriBytes = append(nlriBytes, a4[:prefixBytes]...)
	} else {
		a16 := addr.As16()
		nlriBytes = append(nlriBytes, a16[:prefixBytes]...)
	}

	// Prepend length (label + RD + prefix bits)
	totalBits := 24 + 64 + prefixBits // 3 bytes label + 8 bytes RD + prefix
	nlriWithLen := append([]byte{byte(totalBits)}, nlriBytes...)

	mpUnreach := &attribute.MPUnreachNLRI{
		AFI:  attribute.AFI(afi),
		SAFI: attribute.SAFI(nlri.SAFIVPN), // RFC 4364: SAFI 128
		NLRI: nlriWithLen,
	}

	return &message.Update{
		PathAttributes: attribute.PackAttribute(mpUnreach),
	}
}

// buildMPUnreachLabeledUnicast builds MP_UNREACH_NLRI for labeled unicast withdrawal.
// RFC 8277: Labeled unicast uses SAFI 4 with label + prefix.
func buildMPUnreachLabeledUnicast(route StaticRoute, ctx *nlri.PackContext) *message.Update {
	// Determine AFI from prefix
	var afi nlri.AFI
	if route.Prefix.Addr().Is4() {
		afi = nlri.AFIIPv4
	} else {
		afi = nlri.AFIIPv6
	}

	// Build labeled unicast NLRI: label (3 bytes) + prefix
	var nlriBytes []byte

	// Label: use route.SingleLabel() or withdraw label (0x800000)
	// RFC 8277: Withdrawal uses single label regardless of original stack
	label := route.SingleLabel()
	if label == 0 {
		label = 0x800000 // Withdraw label
	}
	nlriBytes = append(nlriBytes, byte(label>>12), byte(label>>4), byte(label<<4)|0x01) // BOS=1

	// Prefix
	prefixBits := route.Prefix.Bits()
	prefixBytes := (prefixBits + 7) / 8
	addr := route.Prefix.Addr()
	if addr.Is4() {
		a4 := addr.As4()
		nlriBytes = append(nlriBytes, a4[:prefixBytes]...)
	} else {
		a16 := addr.As16()
		nlriBytes = append(nlriBytes, a16[:prefixBytes]...)
	}

	// Prepend length (label + prefix bits)
	totalBits := 24 + prefixBits // 3 bytes label + prefix
	var nlriWithLen []byte

	// Handle ADD-PATH if enabled
	hasAddPath := ctx != nil && ctx.AddPath
	if hasAddPath && route.PathID != 0 {
		nlriWithLen = make([]byte, 4+1+len(nlriBytes))
		nlriWithLen[0] = byte(route.PathID >> 24)
		nlriWithLen[1] = byte(route.PathID >> 16)
		nlriWithLen[2] = byte(route.PathID >> 8)
		nlriWithLen[3] = byte(route.PathID)
		nlriWithLen[4] = byte(totalBits)
		copy(nlriWithLen[5:], nlriBytes)
	} else {
		nlriWithLen = append([]byte{byte(totalBits)}, nlriBytes...)
	}

	mpUnreach := &attribute.MPUnreachNLRI{
		AFI:  attribute.AFI(afi),
		SAFI: 4, // RFC 8277: Labeled Unicast
		NLRI: nlriWithLen,
	}

	return &message.Update{
		PathAttributes: attribute.PackAttribute(mpUnreach),
	}
}

// routeFamily returns the NLRI family for a StaticRoute.
// Used to track which families had routes sent for EOR purposes.
func routeFamily(route StaticRoute) nlri.Family {
	if route.IsVPN() {
		if route.Prefix.Addr().Is6() {
			return nlri.Family{AFI: nlri.AFIIPv6, SAFI: 128} // VPNv6
		}
		return nlri.Family{AFI: nlri.AFIIPv4, SAFI: 128} // VPNv4
	}
	if route.IsLabeledUnicast() {
		if route.Prefix.Addr().Is6() {
			return nlri.Family{AFI: nlri.AFIIPv6, SAFI: 4} // IPv6 Labeled Unicast
		}
		return nlri.Family{AFI: nlri.AFIIPv4, SAFI: 4} // IPv4 Labeled Unicast
	}
	if route.Prefix.Addr().Is6() {
		return nlri.IPv6Unicast
	}
	return nlri.IPv4Unicast
}

// packRawAttribute packs a raw attribute into wire format.
// Format: flags (1 byte) + code (1 byte) + length (1 or 2 bytes) + value.
func packRawAttribute(ra RawAttribute) []byte {
	flags := ra.Flags
	valueLen := len(ra.Value)

	// Use extended length if value > 255 bytes OR if extended length flag is set
	if valueLen > 255 || (flags&0x10) != 0 {
		flags |= 0x10 // Ensure extended length flag is set
		buf := make([]byte, 4+valueLen)
		buf[0] = flags
		buf[1] = ra.Code
		buf[2] = byte(valueLen >> 8)
		buf[3] = byte(valueLen)
		copy(buf[4:], ra.Value)
		return buf
	}

	buf := make([]byte, 3+valueLen)
	buf[0] = flags
	buf[1] = ra.Code
	buf[2] = byte(valueLen)
	copy(buf[3:], ra.Value)
	return buf
}

// routeGroupKey generates a string key for grouping routes by attributes.
// Routes with the same key can be combined into a single UPDATE.
func routeGroupKey(r StaticRoute) string {
	// Sort communities for consistent key.
	comms := make([]uint32, len(r.Communities))
	copy(comms, r.Communities)
	sort.Slice(comms, func(i, j int) bool { return comms[i] < comms[j] })

	// Sort large communities.
	lcs := make([][3]uint32, len(r.LargeCommunities))
	copy(lcs, r.LargeCommunities)
	sort.Slice(lcs, func(i, j int) bool {
		if lcs[i][0] != lcs[j][0] {
			return lcs[i][0] < lcs[j][0]
		}
		if lcs[i][1] != lcs[j][1] {
			return lcs[i][1] < lcs[j][1]
		}
		return lcs[i][2] < lcs[j][2]
	})

	// Key includes: nexthop, origin, localpref, med, communities, large-communities, ext-communities, vpn, ipv4/ipv6,
	// as-path, atomic-aggregate, aggregator, originator-id, cluster-list.
	// For IPv6 routes, include prefix in key to prevent grouping (each needs separate MP_REACH_NLRI UPDATE).
	// IPv4 routes can be grouped since multiple NLRIs can be in one UPDATE.
	prefixKey := ""
	if !r.Prefix.Addr().Is4() {
		prefixKey = r.Prefix.String()
	}
	return fmt.Sprintf("%s|%d|%d|%d|%v|%v|%s|%s|%v|%s|%v|%v|%d|%v|%d|%v",
		r.NextHop.String(),
		r.Origin,
		r.LocalPreference,
		r.MED,
		comms,
		lcs,
		hex.EncodeToString(r.ExtCommunityBytes),
		r.RD,
		r.Prefix.Addr().Is4(),
		prefixKey,
		r.ASPath,
		r.AtomicAggregate,
		r.AggregatorASN,
		r.AggregatorIP,
		r.OriginatorID,
		r.ClusterList,
	)
}

// groupRoutesByAttributes groups routes by their attribute key.
// Returns groups sorted: multi-route groups first (by first prefix), then singletons (by prefix).
// This matches ExaBGP's behavior for UPDATE grouping.
func groupRoutesByAttributes(routes []StaticRoute) [][]StaticRoute {
	groups := make(map[string][]StaticRoute)

	for _, r := range routes {
		key := routeGroupKey(r)
		groups[key] = append(groups[key], r)
	}

	// Collect groups into slice.
	result := make([][]StaticRoute, 0, len(groups))
	for _, g := range groups {
		// Sort routes within group by prefix.
		sort.Slice(g, func(i, j int) bool {
			return g[i].Prefix.Addr().Compare(g[j].Prefix.Addr()) < 0
		})
		result = append(result, g)
	}

	// Sort groups: multi-route first, then singletons, each ordered by first prefix.
	sort.Slice(result, func(i, j int) bool {
		// Multi-route groups come before singletons.
		if len(result[i]) > 1 && len(result[j]) == 1 {
			return true
		}
		if len(result[i]) == 1 && len(result[j]) > 1 {
			return false
		}
		// Same category: sort by first prefix.
		return result[i][0].Prefix.Addr().Compare(result[j][0].Prefix.Addr()) < 0
	})

	return result
}

// sendMVPNRoutes sends MVPN routes configured for this peer.
func (p *Peer) sendMVPNRoutes() {
	nc := p.negotiated.Load()
	if nc == nil {
		return
	}

	addr := p.settings.Address.String()

	// Group MVPN routes by AFI, filtering by negotiated families
	var ipv4Routes, ipv6Routes []MVPNRoute
	var skippedIPv4, skippedIPv6 int

	for _, route := range p.settings.MVPNRoutes {
		if route.IsIPv6 {
			if nc.Has(nlri.IPv6MVPN) {
				ipv6Routes = append(ipv6Routes, route)
			} else {
				skippedIPv6++
			}
		} else {
			if nc.Has(nlri.IPv4MVPN) {
				ipv4Routes = append(ipv4Routes, route)
			} else {
				skippedIPv4++
			}
		}
	}

	if skippedIPv4 > 0 {
		trace.Log(trace.Routes, "peer %s: skipping %d IPv4 MVPN routes (not negotiated)", addr, skippedIPv4)
	}
	if skippedIPv6 > 0 {
		trace.Log(trace.Routes, "peer %s: skipping %d IPv6 MVPN routes (not negotiated)", addr, skippedIPv6)
	}

	// Send IPv4 MVPN routes grouped by attributes (sorted for deterministic order)
	if len(ipv4Routes) > 0 {
		ipv4MVPNFamily := nlri.Family{AFI: 1, SAFI: 5} // IPv4 MVPN
		ctx := p.packContext(ipv4MVPNFamily)
		ub := message.NewUpdateBuilder(p.settings.LocalAS, p.settings.IsIBGP(), ctx)
		ipv4Groups := groupMVPNRoutesByKey(ipv4Routes)
		for _, key := range sortedKeys(ipv4Groups) {
			routes := ipv4Groups[key]
			update := ub.BuildMVPN(toMVPNParams(routes))
			if err := p.SendUpdate(update); err != nil {
				trace.Log(trace.Routes, "peer %s: MVPN send error: %v", addr, err)
			} else {
				trace.Log(trace.Routes, "peer %s: sent %d IPv4 MVPN routes", addr, len(routes))
			}
		}
	}

	// Send IPv6 MVPN routes grouped by attributes (sorted for deterministic order)
	if len(ipv6Routes) > 0 {
		ipv6MVPNFamily := nlri.Family{AFI: 2, SAFI: 5} // IPv6 MVPN
		ctx := p.packContext(ipv6MVPNFamily)
		ub := message.NewUpdateBuilder(p.settings.LocalAS, p.settings.IsIBGP(), ctx)
		ipv6Groups := groupMVPNRoutesByKey(ipv6Routes)
		for _, key := range sortedKeys(ipv6Groups) {
			routes := ipv6Groups[key]
			update := ub.BuildMVPN(toMVPNParams(routes))
			if err := p.SendUpdate(update); err != nil {
				trace.Log(trace.Routes, "peer %s: MVPN send error: %v", addr, err)
			} else {
				trace.Log(trace.Routes, "peer %s: sent %d IPv6 MVPN routes", addr, len(routes))
			}
		}
	}
	// Note: EORs are sent by the generic loop in sendInitialRoutes() for ALL
	// negotiated families, so we don't send family-specific EORs here.
}

// mvpnRouteGroupKey generates a grouping key for MVPN routes.
// Routes with identical keys can share path attributes in one UPDATE.
//
// Fields in key (shared UPDATE attributes per RFC 4271 Section 4.3):
// - NextHop, Origin, LocalPreference, MED: Standard path attributes.
// - ExtCommunityBytes: Route Targets for VPN isolation (RFC 4360).
// - OriginatorID, ClusterList: Route reflector attributes (RFC 4456).
//
// Fields NOT in key (per-NLRI, not per-UPDATE):
// - IsIPv6: Routes pre-separated by AFI before grouping.
// - RouteType: Multiple types allowed in same UPDATE.
// - RD: Per-NLRI field in MP_REACH_NLRI.
// - SourceAS, Source, Group: Per-NLRI fields.
//
// RFC 4456 Section 8: ClusterList is ordered (RRs prepend their CLUSTER_ID).
// Routes with same cluster IDs in different order traversed different paths
// and MUST NOT be grouped together. ClusterList is intentionally not sorted.
func mvpnRouteGroupKey(r MVPNRoute) string {
	return fmt.Sprintf("%s|%d|%d|%d|%s|%d|%v",
		r.NextHop.String(),
		r.Origin,
		r.LocalPreference,
		r.MED,
		hex.EncodeToString(r.ExtCommunityBytes),
		r.OriginatorID,
		r.ClusterList,
	)
}

// groupMVPNRoutesByKey groups MVPN routes by attribute key.
// Routes with same key can share path attributes in a single UPDATE message.
func groupMVPNRoutesByKey(routes []MVPNRoute) map[string][]MVPNRoute {
	groups := make(map[string][]MVPNRoute)
	for _, route := range routes {
		key := mvpnRouteGroupKey(route)
		groups[key] = append(groups[key], route)
	}
	return groups
}

// sortedKeys returns map keys in sorted order for deterministic iteration.
func sortedKeys[V any](m map[string]V) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

// sendVPLSRoutes sends VPLS routes configured for this peer.
func (p *Peer) sendVPLSRoutes() {
	nc := p.negotiated.Load()
	if nc == nil || !nc.Has(nlri.L2VPNVPLS) {
		if len(p.settings.VPLSRoutes) > 0 {
			addr := p.settings.Address.String()
			trace.Log(trace.Routes, "peer %s: skipping %d VPLS routes (L2VPN VPLS not negotiated)",
				addr, len(p.settings.VPLSRoutes))
		}
		return
	}

	addr := p.settings.Address.String()

	if len(p.settings.VPLSRoutes) > 0 {
		trace.Log(trace.Routes, "peer %s: sending %d VPLS routes", addr, len(p.settings.VPLSRoutes))
		// VPLS family: AFI=25 (L2VPN), SAFI=65 (VPLS)
		// Note: VPLS doesn't support ADD-PATH, but we use Pack(ctx) for consistency
		vplsFamily := nlri.Family{AFI: 25, SAFI: 65}
		ctx := p.packContext(vplsFamily)
		ub := message.NewUpdateBuilder(p.settings.LocalAS, p.settings.IsIBGP(), ctx)
		for _, route := range p.settings.VPLSRoutes {
			update := ub.BuildVPLS(toVPLSParams(route))
			if err := p.SendUpdate(update); err != nil {
				trace.Log(trace.Routes, "peer %s: VPLS send error: %v", addr, err)
			}
		}
	}
	// Note: EORs are sent by the generic loop in sendInitialRoutes() for ALL
	// negotiated families, so we don't send family-specific EORs here.
}

// sendFlowSpecRoutes sends FlowSpec routes configured for this peer.
// Only sends routes for families that were successfully negotiated.
// Per RFC 4724 Section 4, EOR is sent for all negotiated families,
// "including the case when there is no update to send".
func (p *Peer) sendFlowSpecRoutes() {
	nc := p.negotiated.Load()
	if nc == nil {
		return
	}

	addr := p.settings.Address.String()

	// Send routes only for negotiated families
	var sentCount int
	for _, route := range p.settings.FlowSpecRoutes {
		// Check if this route's family is negotiated
		isIPv6 := route.IsIPv6
		isVPN := route.RD != [8]byte{}

		var family nlri.Family
		switch {
		case !isIPv6 && !isVPN:
			family = nlri.IPv4FlowSpec
		case !isIPv6 && isVPN:
			family = nlri.IPv4FlowSpecVPN
		case isIPv6 && !isVPN:
			family = nlri.IPv6FlowSpec
		case isIPv6 && isVPN:
			family = nlri.IPv6FlowSpecVPN
		}

		if !nc.Has(family) {
			trace.Log(trace.Routes, "peer %s: skipping FlowSpec route (family not negotiated)", addr)
			continue
		}

		// Determine FlowSpec family: AFI 1/2, SAFI 133 (unicast) or 134 (VPN)
		afi := uint16(1)
		if isIPv6 {
			afi = 2
		}
		safi := uint8(133)
		if isVPN {
			safi = 134
		}
		ctx := p.packContext(nlri.Family{AFI: nlri.AFI(afi), SAFI: nlri.SAFI(safi)})
		ub := message.NewUpdateBuilder(p.settings.LocalAS, p.settings.IsIBGP(), ctx)
		update := ub.BuildFlowSpec(toFlowSpecParams(route))
		if err := p.SendUpdate(update); err != nil {
			trace.Log(trace.Routes, "peer %s: FlowSpec send error: %v", addr, err)
			continue
		}
		sentCount++
	}
	if sentCount > 0 {
		trace.Log(trace.Routes, "peer %s: sent %d FlowSpec routes", addr, sentCount)
	}

	// Note: EOR for FlowSpec families is now sent by the main sendInitialRoutes loop
	// which iterates over all negotiated families using nc.Families().
}

// sendMUPRoutes sends MUP routes configured for this peer.
func (p *Peer) sendMUPRoutes() {
	nc := p.negotiated.Load()
	if nc == nil {
		return
	}

	addr := p.settings.Address.String()

	// Separate routes by AFI, filtering by negotiated families
	var ipv4Routes, ipv6Routes []MUPRoute
	var skippedIPv4, skippedIPv6 int

	for _, route := range p.settings.MUPRoutes {
		if route.IsIPv6 {
			if nc.Has(nlri.IPv6MUP) {
				ipv6Routes = append(ipv6Routes, route)
			} else {
				skippedIPv6++
			}
		} else {
			if nc.Has(nlri.IPv4MUP) {
				ipv4Routes = append(ipv4Routes, route)
			} else {
				skippedIPv4++
			}
		}
	}

	if skippedIPv4 > 0 {
		trace.Log(trace.Routes, "peer %s: skipping %d IPv4 MUP routes (not negotiated)", addr, skippedIPv4)
	}
	if skippedIPv6 > 0 {
		trace.Log(trace.Routes, "peer %s: skipping %d IPv6 MUP routes (not negotiated)", addr, skippedIPv6)
	}

	// Send IPv4 MUP routes
	if len(ipv4Routes) > 0 {
		trace.Log(trace.Routes, "peer %s: sending %d IPv4 MUP routes", addr, len(ipv4Routes))
		ipv4MUPFamily := nlri.Family{AFI: 1, SAFI: 85}
		ctx := p.packContext(ipv4MUPFamily)
		ub := message.NewUpdateBuilder(p.settings.LocalAS, p.settings.IsIBGP(), ctx)
		for _, route := range ipv4Routes {
			update := ub.BuildMUP(toMUPParams(route))
			if err := p.SendUpdate(update); err != nil {
				trace.Log(trace.Routes, "peer %s: MUP send error: %v", addr, err)
			}
		}
	}

	// Send IPv6 MUP routes
	if len(ipv6Routes) > 0 {
		trace.Log(trace.Routes, "peer %s: sending %d IPv6 MUP routes", addr, len(ipv6Routes))
		ipv6MUPFamily := nlri.Family{AFI: 2, SAFI: 85}
		ctx := p.packContext(ipv6MUPFamily)
		ub := message.NewUpdateBuilder(p.settings.LocalAS, p.settings.IsIBGP(), ctx)
		for _, route := range ipv6Routes {
			update := ub.BuildMUP(toMUPParams(route))
			if err := p.SendUpdate(update); err != nil {
				trace.Log(trace.Routes, "peer %s: MUP send error: %v", addr, err)
			}
		}
	}
	// Note: EORs are sent by the generic loop in sendInitialRoutes() for ALL
	// negotiated families, so we don't send family-specific EORs here.
}

// ErrWatchdogNotFound is returned when a watchdog group doesn't exist.
var ErrWatchdogNotFound = errors.New("watchdog group not found")

// AnnounceWatchdog sends all routes in the named watchdog group that are currently withdrawn.
// Routes are moved from withdrawn (-) to announced (+) state.
// State is updated even when disconnected, so routes will be in correct state on reconnect.
// Returns ErrWatchdogNotFound if the watchdog group doesn't exist.
func (p *Peer) AnnounceWatchdog(name string) error {
	p.mu.Lock()
	session := p.session
	routes, exists := p.settings.WatchdogGroups[name]

	if !exists {
		p.mu.Unlock()
		return fmt.Errorf("%w: %s", ErrWatchdogNotFound, name)
	}

	// Ensure watchdogState is initialized for this group
	if p.watchdogState == nil {
		p.watchdogState = make(map[string]map[string]bool)
	}
	if p.watchdogState[name] == nil {
		p.watchdogState[name] = make(map[string]bool)
		// Initialize state for all routes in group
		for i := range routes {
			wr := &routes[i]
			p.watchdogState[name][wr.RouteKey()] = !wr.InitiallyWithdrawn
		}
	}
	p.mu.Unlock()

	// If not connected, just update state (will be sent on reconnect)
	if session == nil {
		p.mu.Lock()
		for i := range routes {
			wr := &routes[i]
			p.watchdogState[name][wr.RouteKey()] = true
		}
		p.mu.Unlock()
		trace.Log(trace.Routes, "peer %s: watchdog %s: marked %d routes for announce (disconnected)",
			p.settings.Address, name, len(routes))
		return nil
	}

	addr := p.settings.Address.String()
	announced := 0

	for i := range routes {
		wr := &routes[i]
		routeKey := wr.RouteKey()

		// Read state inside lock to avoid race with stale captured reference
		p.mu.RLock()
		isAnnounced := p.watchdogState[name][routeKey]
		p.mu.RUnlock()

		if isAnnounced {
			continue // Already announced
		}

		// Send the route - resolve next-hop from RouteNextHop policy
		nextHop, nhErr := p.resolveNextHop(wr.StaticRoute.NextHop, routeFamily(wr.StaticRoute))
		if nhErr != nil {
			trace.Log(trace.Routes, "peer %s: watchdog %s: next-hop resolution failed: %v", addr, name, nhErr)
			continue
		}
		// RFC 7911: Get PackContext for ADD-PATH encoding
		ctx := p.packContext(routeFamily(wr.StaticRoute))
		update := buildStaticRouteUpdateNew(wr.StaticRoute, nextHop, p.settings.LocalAS, p.settings.IsIBGP(), ctx, p.sendCtx)
		if err := p.SendUpdate(update); err != nil {
			return err
		}

		// Update state
		p.mu.Lock()
		p.watchdogState[name][routeKey] = true
		p.mu.Unlock()

		trace.RouteSent(addr, routeKey, wr.StaticRoute.NextHop.String())
		announced++
	}

	if announced > 0 {
		trace.Log(trace.Routes, "peer %s: watchdog %s: announced %d routes", addr, name, announced)
	}
	return nil
}

// WithdrawWatchdog withdraws all routes in the named watchdog group that are currently announced.
// Routes are moved from announced (+) to withdrawn (-) state.
// State is updated even when disconnected, so routes will be in correct state on reconnect.
// Returns ErrWatchdogNotFound if the watchdog group doesn't exist.
func (p *Peer) WithdrawWatchdog(name string) error {
	p.mu.Lock()
	session := p.session
	routes, exists := p.settings.WatchdogGroups[name]

	if !exists {
		p.mu.Unlock()
		return fmt.Errorf("%w: %s", ErrWatchdogNotFound, name)
	}

	// Ensure watchdogState is initialized for this group
	if p.watchdogState == nil {
		p.watchdogState = make(map[string]map[string]bool)
	}
	if p.watchdogState[name] == nil {
		p.watchdogState[name] = make(map[string]bool)
		// Initialize state for all routes in group
		for i := range routes {
			wr := &routes[i]
			p.watchdogState[name][wr.RouteKey()] = !wr.InitiallyWithdrawn
		}
	}
	p.mu.Unlock()

	// If not connected, just update state (will NOT be sent on reconnect)
	if session == nil {
		p.mu.Lock()
		for i := range routes {
			wr := &routes[i]
			p.watchdogState[name][wr.RouteKey()] = false
		}
		p.mu.Unlock()
		trace.Log(trace.Routes, "peer %s: watchdog %s: marked %d routes for withdraw (disconnected)",
			p.settings.Address, name, len(routes))
		return nil
	}

	addr := p.settings.Address.String()
	withdrawn := 0

	for i := range routes {
		wr := &routes[i]
		routeKey := wr.RouteKey()

		// Read state inside lock to avoid race with stale captured reference
		p.mu.RLock()
		isAnnounced := p.watchdogState[name][routeKey]
		p.mu.RUnlock()

		if !isAnnounced {
			continue // Already withdrawn
		}

		// Build withdrawal - handles VPN, IPv4 unicast, IPv6 unicast correctly
		// RFC 7911: Get PackContext for ADD-PATH encoding
		ctx := p.packContext(routeFamily(wr.StaticRoute))
		update := buildStaticRouteWithdraw(wr.StaticRoute, ctx)
		if err := p.SendUpdate(update); err != nil {
			return err
		}

		// Update state
		p.mu.Lock()
		p.watchdogState[name][routeKey] = false
		p.mu.Unlock()

		trace.Log(trace.Routes, "peer %s: watchdog %s: withdrew %s", addr, name, routeKey)
		withdrawn++
	}

	if withdrawn > 0 {
		trace.Log(trace.Routes, "peer %s: watchdog %s: withdrew %d routes", addr, name, withdrawn)
	}
	return nil
}
