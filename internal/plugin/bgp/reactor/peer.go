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

	"codeberg.org/thomas-mangin/ze/internal/plugin"
	"codeberg.org/thomas-mangin/ze/internal/plugin/bgp/attribute"
	"codeberg.org/thomas-mangin/ze/internal/plugin/bgp/capability"
	bgpctx "codeberg.org/thomas-mangin/ze/internal/plugin/bgp/context"
	"codeberg.org/thomas-mangin/ze/internal/plugin/bgp/fsm"
	"codeberg.org/thomas-mangin/ze/internal/plugin/bgp/message"
	"codeberg.org/thomas-mangin/ze/internal/plugin/bgp/nlri"
	"codeberg.org/thomas-mangin/ze/internal/plugin/bgp/rib"
	"codeberg.org/thomas-mangin/ze/internal/slogutil"
	"codeberg.org/thomas-mangin/ze/internal/source"
)

// peerLogger is the peer subsystem logger (lazy initialization).
// Controlled by ze.log.bgp.reactor.peer environment variable.
var peerLogger = slogutil.LazyLogger("bgp.reactor.peer")

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
	// Reset on each session establishment, signaled by "plugin session ready" commands.
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

// SignalAPIReady is called when "plugin session ready" is received for this peer.
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
	routesLogger().Debug("waiting for API sync", "peer", addr, "expected", expected)

	if expected == 0 || ready == nil {
		routesLogger().Debug("no API sync needed", "peer", addr)
		return
	}

	select {
	case <-ready:
		routesLogger().Debug("API sync complete", "peer", addr)
		return
	case <-time.After(timeout):
		// Timeout - proceed anyway to avoid blocking forever
		routesLogger().Debug("API sync timeout", "peer", addr)
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

	// Set context IDs on session for zero-copy WireUpdate and AttrsWire creation
	if p.session != nil {
		p.session.SetRecvCtxID(p.recvCtxID)
		p.session.SetSendCtxID(p.sendCtxID)
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

// getPluginCapabilities returns capabilities declared by API plugins.
// Used as callback for Session.SetPluginCapabilityGetter().
// Converts plugin.InjectedCapability to capability.Capability for OPEN injection.
// Queries capabilities for this peer's specific address to support per-peer capabilities.
func (p *Peer) getPluginCapabilities() []capability.Capability {
	p.mu.RLock()
	r := p.reactor
	settings := p.settings
	p.mu.RUnlock()

	if r == nil || r.api == nil {
		return nil
	}

	// Use peer address to get per-peer capabilities
	peerAddr := settings.Address.String()
	injected := r.api.GetPluginCapabilitiesForPeer(peerAddr)
	if len(injected) == 0 {
		return nil
	}

	caps := make([]capability.Capability, len(injected))
	for i, ic := range injected {
		caps[i] = capability.NewPlugin(ic.Code, ic.Value)
	}
	return caps
}

// getPluginFamilies returns families from plugins that declared decode capability.
// Used as callback for Session.SetPluginFamiliesGetter().
// Plugins that can decode a family should advertise it in OPEN Multiprotocol capabilities.
func (p *Peer) getPluginFamilies() []string {
	p.mu.RLock()
	r := p.reactor
	p.mu.RUnlock()

	if r == nil || r.api == nil {
		return nil
	}

	return r.api.GetDecodeFamilies()
}

// addPathFor returns whether ADD-PATH is negotiated for the given family.
// RFC 7911: ADD-PATH requires 4-byte path identifier prefix on NLRI.
// Returns false if session not established.
func (p *Peer) addPathFor(family nlri.Family) bool {
	if p.sendCtx == nil {
		return false
	}
	return p.sendCtx.AddPath(family)
}

// asn4 returns whether 4-byte ASN is negotiated.
// RFC 6793: ASN4 determines 2-byte vs 4-byte AS numbers in AS_PATH.
// Returns true if session not established (default to modern).
func (p *Peer) asn4() bool {
	if p.sendCtx == nil {
		return true
	}
	return p.sendCtx.ASN4()
}

// resolveNextHop returns the actual IP address for a RouteNextHop policy.
// Uses session's LocalAddress for Self, validates against Extended NH capability.
//
// RFC 4271 Section 5.1.3 - NEXT_HOP attribute.
// RFC 5549/8950 - Extended Next Hop Encoding.
func (p *Peer) resolveNextHop(nh plugin.RouteNextHop, family nlri.Family) (netip.Addr, error) {
	switch nh.Policy {
	case plugin.NextHopExplicit:
		// Explicit addresses bypass validation - user is responsible.
		// Returns invalid addr without error if that's what was configured.
		return nh.Addr, nil

	case plugin.NextHopSelf:
		local := p.settings.LocalAddress
		if !local.IsValid() {
			return netip.Addr{}, ErrNextHopSelfNoLocal
		}
		// Validate: can we use this address for this NLRI family?
		if !p.canUseNextHopFor(local, family) {
			return netip.Addr{}, ErrNextHopIncompatible
		}
		return local, nil

	case plugin.NextHopUnset:
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
// linkLocal is the peer's IPv6 link-local address for 32-byte MP_REACH next-hop (RFC 2545 Section 3).
func toStaticRouteUnicastParams(r StaticRoute, nextHop netip.Addr, linkLocal netip.Addr, sendCtx *bgpctx.EncodingContext) message.UnicastParams {
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
		LinkLocalNextHop:   linkLocal,
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
		PrefixSID:         r.PrefixSIDBytes,
	}
}

// buildStaticRouteUpdateNew builds an UPDATE for a static route using UpdateBuilder.
// This is the new implementation that will replace buildStaticRouteUpdate.
// nextHop is the resolved next-hop address (from RouteNextHop policy).
// linkLocal is the peer's IPv6 link-local for 32-byte MP_REACH next-hop (RFC 2545 Section 3).
func buildStaticRouteUpdateNew(route StaticRoute, nextHop netip.Addr, linkLocal netip.Addr, localAS uint32, isIBGP bool, asn4, addPath bool, sendCtx *bgpctx.EncodingContext) *message.Update {
	ub := message.NewUpdateBuilder(localAS, isIBGP, asn4, addPath)
	if route.IsVPN() {
		return ub.BuildVPN(toStaticRouteVPNParams(route, nextHop))
	}
	if route.IsLabeledUnicast() {
		return ub.BuildLabeledUnicast(toStaticRouteLabeledUnicastParams(route, nextHop))
	}
	return ub.BuildUnicast(toStaticRouteUnicastParams(route, nextHop, linkLocal, sendCtx))
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
			routesLogger().Debug("opQueue full, dropping teardown", "peer", p.settings.Address)
		}
		p.mu.Unlock()
		return
	}

	if session != nil {
		p.mu.Unlock()
		if err := session.Teardown(subcode); err != nil {
			peerLogger().Debug("teardown error", "peer", p.settings.Address, "error", err)
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
			routesLogger().Debug("opQueue full, dropping teardown", "peer", p.settings.Address)
		}
		p.mu.Unlock()
	}
}

// ShouldQueue returns true if routes should be queued rather than sent directly.
// Routes must be queued when:
//   - Session is not established
//   - Initial route sending is in progress (sendInitialRoutes running)
//   - There are pending queued operations (preserves insertion order)
//
// This prevents a race where routes sent directly during sendInitialRoutes
// processing arrive at the peer before older queued routes.
func (p *Peer) ShouldQueue() bool {
	if p.State() != PeerStateEstablished {
		return true
	}
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.sendingInitialRoutes.Load() != 0 || len(p.opQueue) > 0
}

// QueueAnnounce queues a route announcement for when session establishes.
// Used when session is not established to maintain operation order.
// If queue is full (MaxOpQueueSize), the operation is dropped with a warning.
func (p *Peer) QueueAnnounce(route *rib.Route) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if len(p.opQueue) >= MaxOpQueueSize {
		routesLogger().Debug("opQueue full, dropping announce", "peer", p.settings.Address, "queueSize", len(p.opQueue), "nlri", route.NLRI())
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
		routesLogger().Debug("opQueue full, dropping withdraw", "peer", p.settings.Address, "queueSize", len(p.opQueue), "nlri", n)
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
	session.SetPluginCapabilityGetter(p.getPluginCapabilities)
	session.SetPluginFamiliesGetter(p.getPluginFamilies)

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
		peerLogger().Debug("FSM transition", "peer", addr, "from", from.String(), "to", to.String())

		if to == fsm.StateEstablished {
			// Pre-compute negotiated capabilities for O(1) access during route sending
			neg := session.Negotiated()
			p.negotiated.Store(NewNegotiatedCapabilities(neg))
			p.setEncodingContexts(neg)
			p.setState(PeerStateEstablished)
			peerLogger().Info("session established", "peer", addr, "localAS", p.settings.LocalAS, "peerAS", p.settings.PeerAS)

			// Reset per-session API sync: count plugins with SendUpdate permission.
			// They will signal "plugin session ready" after replaying routes.
			apiSendCount := 0
			for _, binding := range p.settings.ProcessBindings {
				if binding.SendUpdate {
					apiSendCount++
				}
			}
			p.ResetAPISync(apiSendCount)

			// Notify reactor of peer established and negotiated capabilities
			p.mu.RLock()
			reactor := p.reactor
			p.mu.RUnlock()
			if reactor != nil {
				reactor.notifyPeerEstablished(p)
				reactor.notifyPeerNegotiated(p, neg)
			}

			// Send static routes from config.
			peerLogger().Debug("spawning sendInitialRoutes", "peer", addr)
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
			peerLogger().Info("session closed", "peer", addr, "reason", reason)
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
//   - wait: channel to wait for existing session teardown (if acceptPending is true)
func (p *Peer) ResolvePendingCollision(pendingOpen *message.Open) (acceptPending bool, conn net.Conn, open *message.Open, wait <-chan struct{}) {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.pendingConn == nil {
		return false, nil, nil, nil
	}

	conn = p.pendingConn
	session := p.session

	if session == nil {
		// Session gone, reject pending
		p.pendingConn = nil
		p.pendingOpen = nil
		return false, conn, nil, nil
	}

	shouldAccept, shouldCloseExisting := session.DetectCollision(pendingOpen.BGPIdentifier)

	if shouldAccept && shouldCloseExisting {
		// Remote wins: close existing, accept pending
		// Store the OPEN so we can replay it
		p.pendingOpen = pendingOpen
		p.pendingConn = nil
		wait = session.Done()

		// Close existing session with NOTIFICATION
		// The session's Run loop will exit and we can accept pending
		go func() {
			_ = session.CloseWithNotification(message.NotifyCease, message.NotifyCeaseConnectionCollision)
		}()

		return true, conn, pendingOpen, wait
	}

	// Local wins: reject pending, keep existing
	p.pendingConn = nil
	p.pendingOpen = nil
	return false, conn, nil, nil
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

// SendAnnounce sends a BGP UPDATE message for announcing a route.
// Eliminates large buffer allocations by writing directly to session buffer.
// Returns ErrNotConnected if no session is active.
//
// RFC 4271 Section 4.3 - UPDATE Message Format.
// RFC 4760 Section 3 - MP_REACH_NLRI for IPv6 routes.
// RFC 7911 - ADD-PATH encoding based on negotiated capabilities.
func (p *Peer) SendAnnounce(route plugin.RouteSpec, localAS uint32) error {
	p.mu.RLock()
	session := p.session
	p.mu.RUnlock()

	if session == nil {
		return ErrNotConnected
	}

	isIBGP := p.settings.IsIBGP()
	family := nlri.Family{AFI: nlri.AFIIPv4, SAFI: nlri.SAFIUnicast}
	if route.Prefix.Addr().Is6() {
		family = nlri.Family{AFI: nlri.AFIIPv6, SAFI: nlri.SAFIUnicast}
	}
	asn4 := p.asn4()
	addPath := p.addPathFor(family)

	return session.SendAnnounce(route, localAS, isIBGP, asn4, addPath)
}

// SendWithdraw sends a BGP UPDATE message for withdrawing a route.
// Eliminates large buffer allocations by writing directly to session buffer.
// Returns ErrNotConnected if no session is active.
//
// RFC 4271 Section 4.3 - UPDATE Message Format (Withdrawn Routes for IPv4).
// RFC 4760 Section 4 - MP_UNREACH_NLRI for IPv6 withdrawals.
// RFC 7911 - ADD-PATH encoding based on negotiated capabilities.
func (p *Peer) SendWithdraw(prefix netip.Prefix) error {
	p.mu.RLock()
	session := p.session
	p.mu.RUnlock()

	if session == nil {
		return ErrNotConnected
	}

	family := nlri.Family{AFI: nlri.AFIIPv4, SAFI: nlri.SAFIUnicast}
	if prefix.Addr().Is6() {
		family = nlri.Family{AFI: nlri.AFIIPv6, SAFI: nlri.SAFIUnicast}
	}
	addPath := p.addPathFor(family)

	return session.SendWithdraw(prefix, addPath)
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
	peerLogger().Debug("sendInitialRoutes ENTER", "peer", addr)

	// Prevent concurrent sendInitialRoutes goroutines.
	// If another instance is running, skip this one - the running instance
	// will process any queued operations.
	if !p.sendingInitialRoutes.CompareAndSwap(0, 1) {
		peerLogger().Debug("sendInitialRoutes skipped (concurrent)", "peer", addr)
		return
	}
	// Flag is cleared inside the mutex after the opQueue drain loop completes,
	// NOT via defer. This ensures ShouldQueue() sees a consistent state:
	// either the flag is set (routes will be queued and drained by us),
	// or the flag is cleared and the queue is empty (routes can be sent directly).

	peerLogger().Debug("sendInitialRoutes started", "peer", addr)

	// Get negotiated capabilities for family checks.
	nc := p.negotiated.Load()
	if nc == nil {
		peerLogger().Debug("sendInitialRoutes aborted (no negotiated caps)", "peer", addr)
		return
	}

	peerLogger().Debug("sendInitialRoutes sending static routes", "peer", addr, "count", len(p.settings.StaticRoutes))

	// Calculate max message size for this peer
	maxMsgSize := int(message.MaxMessageLength(message.TypeUPDATE, nc.ExtendedMessage))

	// Send routes - either grouped or individually based on config.
	if p.settings.GroupUpdates {
		// Group routes by attributes (same attributes = same UPDATE).
		groups := groupRoutesByAttributes(p.settings.StaticRoutes)

		for _, routes := range groups {
			addPath := p.addPathFor(routeFamily(routes[0]))
			if len(routes) == 1 {
				// Single-route group (IPv6, VPN, LabeledUnicast, or solo IPv4)
				// Resolve next-hop from RouteNextHop policy
				nextHop, nhErr := p.resolveNextHop(routes[0].NextHop, routeFamily(routes[0]))
				if nhErr != nil {
					routesLogger().Debug("next-hop resolution failed", "peer", addr, "error", nhErr)
					continue
				}
				update := buildStaticRouteUpdateNew(routes[0], nextHop, p.settings.LinkLocal, p.settings.LocalAS, p.settings.IsIBGP(), p.asn4(), addPath, p.sendCtx)
				if err := p.sendUpdateWithSplit(update, maxMsgSize, routeFamily(routes[0])); err != nil {
					routesLogger().Debug("send error", "peer", addr, "error", err)
					break
				}
			} else {
				// Multi-route group - IPv4 unicast only (routeGroupKey ensures this)
				// Use size-aware builder to respect max message size
				ub := message.NewUpdateBuilder(p.settings.LocalAS, p.settings.IsIBGP(), p.asn4(), addPath)
				params := make([]message.UnicastParams, 0, len(routes))
				for _, r := range routes {
					nextHop, nhErr := p.resolveNextHop(r.NextHop, routeFamily(r))
					if nhErr != nil {
						routesLogger().Debug("next-hop resolution failed", "peer", addr, "prefix", r.Prefix, "error", nhErr)
						continue
					}
					params = append(params, toStaticRouteUnicastParams(r, nextHop, p.settings.LinkLocal, p.sendCtx))
				}
				if len(params) == 0 {
					continue
				}
				updates, err := ub.BuildGroupedUnicastWithLimit(params, maxMsgSize)
				if err != nil {
					routesLogger().Debug("build error", "peer", addr, "error", err)
					break
				}
				for _, update := range updates {
					if err := p.SendUpdate(update); err != nil {
						routesLogger().Debug("send error", "peer", addr, "error", err)
						break
					}
				}
			}
			for _, route := range routes {
				routesLogger().Debug("route sent", "peer", addr, "prefix", route.Prefix.String(), "nextHop", route.NextHop.String())
			}
		}
	} else {
		// Send each route in its own UPDATE.
		for _, route := range p.settings.StaticRoutes {
			// Resolve next-hop from RouteNextHop policy
			nextHop, nhErr := p.resolveNextHop(route.NextHop, routeFamily(route))
			if nhErr != nil {
				routesLogger().Debug("next-hop resolution failed", "peer", addr, "prefix", route.Prefix, "error", nhErr)
				continue
			}
			addPath := p.addPathFor(routeFamily(route))
			update := buildStaticRouteUpdateNew(route, nextHop, p.settings.LinkLocal, p.settings.LocalAS, p.settings.IsIBGP(), p.asn4(), addPath, p.sendCtx)
			if err := p.sendUpdateWithSplit(update, maxMsgSize, routeFamily(route)); err != nil {
				routesLogger().Debug("send error", "peer", addr, "error", err)
				break
			}
			routesLogger().Debug("route sent", "peer", addr, "prefix", route.Prefix.String(), "nextHop", route.NextHop.String())
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
					routesLogger().Debug("watchdog holding route", "peer", addr, "watchdog", name, "route", routeKey)
					continue
				}

				// Send the route - resolve next-hop from RouteNextHop policy
				nextHop, nhErr := p.resolveNextHop(wr.NextHop, routeFamily(wr.StaticRoute))
				if nhErr != nil {
					routesLogger().Debug("watchdog next-hop resolution failed", "peer", addr, "watchdog", name, "error", nhErr)
					continue
				}
				addPath := p.addPathFor(routeFamily(wr.StaticRoute))
				update := buildStaticRouteUpdateNew(wr.StaticRoute, nextHop, p.settings.LinkLocal, p.settings.LocalAS, p.settings.IsIBGP(), p.asn4(), addPath, p.sendCtx)
				if err := p.sendUpdateWithSplit(update, maxMsgSize, routeFamily(wr.StaticRoute)); err != nil {
					routesLogger().Debug("send error", "peer", addr, "error", err)
					break
				}
				routesLogger().Debug("route sent", "peer", addr, "prefix", routeKey, "nextHop", wr.NextHop.String())
			}
		}
		routesLogger().Debug("sent watchdog routes", "peer", addr, "groups", len(p.settings.WatchdogGroups))
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
					routesLogger().Debug("global pool next-hop resolution failed", "peer", addr, "pool", poolName, "error", nhErr)
					continue
				}
				addPath := p.addPathFor(routeFamily(route))
				update := buildStaticRouteUpdateNew(route, nextHop, p.settings.LinkLocal, p.settings.LocalAS, p.settings.IsIBGP(), p.asn4(), addPath, p.sendCtx)
				if err := p.sendUpdateWithSplit(update, maxMsgSize, routeFamily(route)); err != nil {
					routesLogger().Debug("send error", "peer", addr, "error", err)
					break
				}
				routesLogger().Debug("re-sent global pool route", "peer", addr, "route", pr.RouteKey(), "pool", poolName)
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
		routesLogger().Debug("sleeping for API routes", "peer", addr, "duration", "500ms")
		time.Sleep(500 * time.Millisecond)
		routesLogger().Debug("woke from sleep, processing queue", "peer", addr)
	}

	// Process operation queue in order (maintains announce/withdraw/teardown ordering).
	// Stop at first teardown - remaining items stay for next session.
	//
	// CONCURRENCY NOTE: Uses index-based loop (not range) so that items appended
	// by concurrent QueueAnnounce/QueueWithdraw calls during unlocked sends are
	// picked up by the next iteration's len(p.opQueue) check. This, combined with
	// ShouldQueue() in the announce/withdraw paths, ensures strict insertion order:
	// routes arriving while this loop runs are queued (not sent directly) and
	// processed here in FIFO order.
	var teardownSubcode uint8
	hasTeardown := false

	// Pre-compute max message size for size checking in PeerOpAnnounce
	opMaxMsgSize := int(message.MaxMessageLength(message.TypeUPDATE, nc.ExtendedMessage))

	p.mu.Lock()
	queueLen := len(p.opQueue)
	processed := 0
	connError := false
	for processed < len(p.opQueue) && !connError {
		op := p.opQueue[processed]
		switch op.Type {
		case PeerOpTeardown:
			teardownSubcode = op.Subcode
			hasTeardown = true
			processed++

		case PeerOpAnnounce:
			// Send route, splitting if needed.
			family := op.Route.NLRI().Family()
			addPath := p.addPathFor(family)
			attrBuf := getBuildBuf()
			update := buildRIBRouteUpdate(attrBuf, op.Route, p.settings.LocalAS, p.settings.IsIBGP(), p.asn4(), addPath)
			p.mu.Unlock()
			sendErr := p.sendUpdateWithSplit(update, opMaxMsgSize, family)
			putBuildBuf(attrBuf)
			if sendErr != nil {
				routesLogger().Debug("send error for queued route", "peer", addr, "nlri", op.Route.NLRI(), "error", sendErr)
				p.mu.Lock()
				processed++
				// Split errors: skip route. Connection errors: stop processing.
				if !errors.Is(sendErr, message.ErrAttributesTooLarge) && !errors.Is(sendErr, message.ErrNLRITooLarge) {
					connError = true
				}
				continue
			}
			p.mu.Lock()
			processed++
			continue

		case PeerOpWithdraw:
			// Send withdrawal using pooled buffer.
			family := op.NLRI.Family()
			addPath := p.addPathFor(family)
			wdBuf := getBuildBuf()
			update := buildWithdrawNLRI(wdBuf, op.NLRI, addPath)
			p.mu.Unlock()
			sendErr := p.sendUpdateWithSplit(update, opMaxMsgSize, family)
			putBuildBuf(wdBuf)
			if sendErr != nil {
				routesLogger().Debug("send error for withdrawal", "peer", addr, "nlri", op.NLRI, "error", sendErr)
				p.mu.Lock()
				processed++
				if !errors.Is(sendErr, message.ErrAttributesTooLarge) && !errors.Is(sendErr, message.ErrNLRITooLarge) {
					connError = true
				}
				continue
			}
			p.mu.Lock()
			processed++
			continue
		}

		// If we get here, it was a teardown - break out of loop
		break
	}
	// Remove processed items and clear the sendingInitialRoutes flag atomically.
	// This ensures ShouldQueue() sees a consistent state: either the flag is set
	// (new routes will be queued and drained by our loop), or the flag is cleared
	// and the queue is empty (new routes can be sent directly).
	if processed > 0 {
		p.opQueue = p.opQueue[processed:]
	}
	if !hasTeardown {
		p.sendingInitialRoutes.Store(0)
	}
	p.mu.Unlock()

	if queueLen > 0 {
		routesLogger().Debug("processed queue ops", "peer", addr, "processed", processed, "remaining", len(p.opQueue), "teardown", hasTeardown)
	}

	// If teardown was in queue, send EOR first, then execute teardown.
	// EOR must be sent BEFORE NOTIFICATION per RFC 4724 Section 4.
	if hasTeardown {
		// Send EOR for ALL negotiated families before teardown
		for _, family := range nc.Families() {
			_ = p.SendUpdate(message.BuildEOR(family))
			routesLogger().Debug("sent EOR (before teardown)", "peer", addr, "family", family)
		}

		routesLogger().Debug("executing queued teardown", "peer", addr, "subcode", teardownSubcode)
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
				routesLogger().Debug("teardown error", "peer", addr, "error", err)
			}
		}
		// Clear remaining opQueue - these routes were never sent, so shouldn't
		// be re-sent on reconnection. Persist plugin tracks actually-sent routes.
		p.mu.Lock()
		if len(p.opQueue) > 0 {
			routesLogger().Debug("clearing unsent queue items after teardown", "peer", addr, "count", len(p.opQueue))
			p.opQueue = p.opQueue[:0]
		}
		// Clear flag under mutex for teardown path too
		p.sendingInitialRoutes.Store(0)
		p.mu.Unlock()
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
		routesLogger().Debug("sent EOR", "peer", addr, "family", family)
	}
}

// buildRIBRouteUpdate builds an UPDATE message from a RIB route.
// Used for re-announcing routes from Adj-RIB-Out on session re-establishment.
// Rebuilds the full set of required attributes since rib.Route may not store all.
// RFC 7911: addPath indicates ADD-PATH capability for NLRI encoding.
// RFC 6793: asn4 determines 2-byte vs 4-byte AS numbers in AS_PATH.
func buildRIBRouteUpdate(attrBuf []byte, route *rib.Route, localAS uint32, isIBGP bool, asn4, addPath bool) *message.Update {
	off := 0

	// Create encoding context for ASPath encoding
	dstCtx := bgpctx.EncodingContextForASN4(asn4)

	// 1. ORIGIN - use stored or default to IGP
	origin := attribute.OriginIGP
	for _, attr := range route.Attributes() {
		if o, ok := attr.(attribute.Origin); ok {
			origin = o
			break
		}
	}
	off += attribute.WriteAttrTo(origin, attrBuf, off)

	// 2. AS_PATH - use stored or build appropriate default
	storedASPath := route.ASPath()
	hasStoredASPath := storedASPath != nil && len(storedASPath.Segments) > 0

	var asPath *attribute.ASPath
	switch {
	case hasStoredASPath:
		asPath = storedASPath
	case isIBGP || localAS == 0:
		// iBGP or LocalAS not set: empty AS_PATH
		asPath = &attribute.ASPath{Segments: nil}
	default:
		// eBGP: prepend local AS
		asPath = &attribute.ASPath{
			Segments: []attribute.ASPathSegment{{
				Type: attribute.ASSequence,
				ASNs: []uint32{localAS},
			}},
		}
	}
	off += attribute.WriteAttrToWithContext(asPath, attrBuf, off, nil, dstCtx)

	// Determine NLRI handling based on address family
	routeNLRI := route.NLRI()
	family := routeNLRI.Family()
	var nlriBytes []byte

	switch {
	case family.AFI == nlri.AFIIPv4 && family.SAFI == nlri.SAFIUnicast:
		// 3. NEXT_HOP for IPv4 unicast
		nh := &attribute.NextHop{Addr: route.NextHop()}
		off += attribute.WriteAttrTo(nh, attrBuf, off)

		// 4. MED if present (before LOCAL_PREF per RFC order)
		for _, attr := range route.Attributes() {
			if med, ok := attr.(attribute.MED); ok {
				off += attribute.WriteAttrTo(med, attrBuf, off)
				break
			}
		}

		// 5. LOCAL_PREF for iBGP - use stored value or default to 100
		if isIBGP {
			var localPref attribute.LocalPref = 100
			for _, attr := range route.Attributes() {
				if lp, ok := attr.(attribute.LocalPref); ok {
					localPref = lp
					break
				}
			}
			off += attribute.WriteAttrTo(localPref, attrBuf, off)
		}

		// IPv4 unicast: use inline NLRI field
		// RFC 7911: WriteNLRI uses ADD-PATH encoding when negotiated
		// Write NLRI into tail of attrBuf (no overlap with attrs growing from offset 0)
		nlriLen := nlri.LenWithContext(routeNLRI, addPath)
		nlriOff := len(attrBuf) - nlriLen
		nlri.WriteNLRI(routeNLRI, attrBuf, nlriOff, addPath)
		nlriBytes = attrBuf[nlriOff : nlriOff+nlriLen]
	default: // non-IPv4-unicast families
		// Other families: MP_REACH_NLRI goes at end (after all other attributes)
		// Write NLRI into tail of attrBuf; WriteAttrTo copies it into attrs region
		nlriLen := nlri.LenWithContext(routeNLRI, addPath)
		nlriOff := len(attrBuf) - nlriLen
		nlri.WriteNLRI(routeNLRI, attrBuf, nlriOff, addPath)
		nlriData := attrBuf[nlriOff : nlriOff+nlriLen]

		mpReach := &attribute.MPReachNLRI{
			AFI:      attribute.AFI(family.AFI),
			SAFI:     attribute.SAFI(family.SAFI),
			NextHops: []netip.Addr{route.NextHop()},
			NLRI:     nlriData,
		}

		// MED if present (before LOCAL_PREF per RFC order)
		for _, attr := range route.Attributes() {
			if med, ok := attr.(attribute.MED); ok {
				off += attribute.WriteAttrTo(med, attrBuf, off)
				break
			}
		}

		// LOCAL_PREF for iBGP - use stored value or default to 100
		if isIBGP {
			var localPref attribute.LocalPref = 100
			for _, attr := range route.Attributes() {
				if lp, ok := attr.(attribute.LocalPref); ok {
					localPref = lp
					break
				}
			}
			off += attribute.WriteAttrTo(localPref, attrBuf, off)
		}

		// MP_REACH_NLRI at end (after all other path attributes)
		off += attribute.WriteAttrTo(mpReach, attrBuf, off)
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
			// Write optional attributes
			off += attribute.WriteAttrTo(attr, attrBuf, off)
		}
	}

	return &message.Update{
		PathAttributes: attrBuf[:off],
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
	addPath := p.sendCtx != nil && p.sendCtx.AddPath(family)

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
// buf is a caller-provided buffer (from buildBufPool).
// For IPv4 unicast, NLRI is written at buf[0:]. For MP families, NLRI is
// written at a high offset to avoid overlap with the MP_UNREACH_NLRI header.
// RFC 4760: IPv4 unicast uses WithdrawnRoutes, others use MP_UNREACH_NLRI.
// RFC 7911: addPath indicates ADD-PATH capability for NLRI encoding.
func buildWithdrawNLRI(buf []byte, n nlri.NLRI, addPath bool) *message.Update {
	family := n.Family()
	nlriLen := nlri.LenWithContext(n, addPath)

	if family.AFI == nlri.AFIIPv4 && family.SAFI == nlri.SAFIUnicast {
		// IPv4 unicast: write NLRI at start, use WithdrawnRoutes field
		nlri.WriteNLRI(n, buf, 0, addPath)
		return &message.Update{
			WithdrawnRoutes: buf[:nlriLen],
		}
	}

	// MP families: write NLRI at high offset so WriteAttrTo can build
	// the MP_UNREACH_NLRI attribute from buf[0:] without overlapping.
	const nlriRegion = 2048
	nlri.WriteNLRI(n, buf, nlriRegion, addPath)
	nlriData := buf[nlriRegion : nlriRegion+nlriLen]

	mpUnreach := &attribute.MPUnreachNLRI{
		AFI:  attribute.AFI(family.AFI),
		SAFI: attribute.SAFI(family.SAFI),
		NLRI: nlriData,
	}
	attrLen := attribute.WriteAttrTo(mpUnreach, buf, 0)

	return &message.Update{
		PathAttributes: buf[:attrLen],
	}
}

// buildStaticRouteWithdraw builds a withdrawal UPDATE for a static route.
// Handles VPN, labeled-unicast, IPv4 unicast, and IPv6 unicast correctly.
// buf is a caller-provided buffer (from buildBufPool) for zero-allocation encoding.
// RFC 7911: addPath indicates ADD-PATH capability for NLRI encoding.
func buildStaticRouteWithdraw(buf []byte, route StaticRoute, addPath bool) *message.Update {
	switch {
	case route.IsVPN():
		// VPN route: use MP_UNREACH_NLRI with RD + prefix
		return buildMPUnreachVPN(buf, route)
	case route.IsLabeledUnicast():
		// Labeled unicast: use MP_UNREACH_NLRI with label + prefix
		return buildMPUnreachLabeledUnicast(buf, route, addPath)
	case route.Prefix.Addr().Is4():
		// IPv4 unicast: use WithdrawnRoutes field
		// RFC 7911: WriteNLRI uses ADD-PATH encoding when negotiated
		inet := nlri.NewINET(nlri.Family{AFI: nlri.AFIIPv4, SAFI: nlri.SAFIUnicast}, route.Prefix, route.PathID)
		nlriLen := nlri.LenWithContext(inet, addPath)
		nlri.WriteNLRI(inet, buf, 0, addPath)
		return &message.Update{
			WithdrawnRoutes: buf[:nlriLen],
		}
	default: // IPv6 unicast
		// IPv6 unicast: use MP_UNREACH_NLRI
		// Write NLRI into tail of buf; WriteAttrTo copies it into attr region
		inet := nlri.NewINET(nlri.Family{AFI: nlri.AFIIPv6, SAFI: nlri.SAFIUnicast}, route.Prefix, route.PathID)
		nlriLen := nlri.LenWithContext(inet, addPath)
		nlriOff := len(buf) / 2
		nlri.WriteNLRI(inet, buf, nlriOff, addPath)

		mpUnreach := &attribute.MPUnreachNLRI{
			AFI:  attribute.AFI(nlri.AFIIPv6),
			SAFI: attribute.SAFI(nlri.SAFIUnicast),
			NLRI: buf[nlriOff : nlriOff+nlriLen],
		}

		attrLen := attribute.WriteAttrTo(mpUnreach, buf, 0)
		return &message.Update{
			PathAttributes: buf[:attrLen],
		}
	}
}

// buildMPUnreachVPN builds MP_UNREACH_NLRI for VPN route withdrawal.
// buf is a caller-provided buffer for zero-allocation encoding.
// NLRI is written into buf[nlriRegion:] (second half), then WriteAttrTo copies
// it into buf[0:] as part of the MP_UNREACH_NLRI attribute.
func buildMPUnreachVPN(buf []byte, route StaticRoute) *message.Update {
	// Determine AFI from prefix
	var afi nlri.AFI
	if route.Prefix.Addr().Is4() {
		afi = nlri.AFIIPv4
	} else {
		afi = nlri.AFIIPv6
	}

	// Write labeled VPN NLRI into second half of buf: length(1) + label(3) + RD(8) + prefix
	nlriRegion := len(buf) / 2
	off := nlriRegion

	// Length byte (filled after computing total)
	lengthPos := off
	off++

	// Label: use route.SingleLabel() or withdraw label (0x800000)
	// RFC 8277: Withdrawal uses single label regardless of original stack
	label := route.SingleLabel()
	if label == 0 {
		label = 0x800000 // Withdraw label
	}
	buf[off] = byte(label >> 16)
	buf[off+1] = byte(label >> 8)
	buf[off+2] = byte(label) | 0x01 // Bottom of stack
	off += 3

	// RD (8 bytes)
	copy(buf[off:], route.RDBytes[:])
	off += 8

	// Prefix
	prefixBits := route.Prefix.Bits()
	prefixBytes := (prefixBits + 7) / 8
	addr := route.Prefix.Addr()
	if addr.Is4() {
		a4 := addr.As4()
		copy(buf[off:], a4[:prefixBytes])
	} else {
		a16 := addr.As16()
		copy(buf[off:], a16[:prefixBytes])
	}
	off += prefixBytes

	// Fill length byte: label(24) + RD(64) + prefix bits
	buf[lengthPos] = byte(24 + 64 + prefixBits)

	mpUnreach := &attribute.MPUnreachNLRI{
		AFI:  attribute.AFI(afi),
		SAFI: attribute.SAFI(nlri.SAFIVPN), // RFC 4364: SAFI 128
		NLRI: buf[nlriRegion:off],
	}

	attrLen := attribute.WriteAttrTo(mpUnreach, buf, 0)
	return &message.Update{
		PathAttributes: buf[:attrLen],
	}
}

// buildMPUnreachLabeledUnicast builds MP_UNREACH_NLRI for labeled unicast withdrawal.
// buf is a caller-provided buffer for zero-allocation encoding.
// RFC 8277: Labeled unicast uses SAFI 4 with label + prefix.
func buildMPUnreachLabeledUnicast(buf []byte, route StaticRoute, addPath bool) *message.Update {
	// Determine AFI from prefix
	var afi nlri.AFI
	if route.Prefix.Addr().Is4() {
		afi = nlri.AFIIPv4
	} else {
		afi = nlri.AFIIPv6
	}

	// Write labeled unicast NLRI into second half of buf
	nlriRegion := len(buf) / 2
	off := nlriRegion

	// Handle ADD-PATH: path-id (4 bytes) before length byte
	if addPath && route.PathID != 0 {
		buf[off] = byte(route.PathID >> 24)
		buf[off+1] = byte(route.PathID >> 16)
		buf[off+2] = byte(route.PathID >> 8)
		buf[off+3] = byte(route.PathID)
		off += 4
	}

	// Length byte (label + prefix bits)
	prefixBits := route.Prefix.Bits()
	totalBits := 24 + prefixBits // 3 bytes label + prefix
	buf[off] = byte(totalBits)
	off++

	// Label: use route.SingleLabel() or withdraw label (0x800000)
	// RFC 8277: Withdrawal uses single label regardless of original stack
	label := route.SingleLabel()
	if label == 0 {
		label = 0x800000 // Withdraw label
	}
	buf[off] = byte(label >> 12)
	buf[off+1] = byte(label >> 4)
	buf[off+2] = byte(label<<4) | 0x01 // BOS=1
	off += 3

	// Prefix
	prefixBytes := (prefixBits + 7) / 8
	addr := route.Prefix.Addr()
	if addr.Is4() {
		a4 := addr.As4()
		copy(buf[off:], a4[:prefixBytes])
	} else {
		a16 := addr.As16()
		copy(buf[off:], a16[:prefixBytes])
	}
	off += prefixBytes

	mpUnreach := &attribute.MPUnreachNLRI{
		AFI:  attribute.AFI(afi),
		SAFI: 4, // RFC 8277: Labeled Unicast
		NLRI: buf[nlriRegion:off],
	}

	attrLen := attribute.WriteAttrTo(mpUnreach, buf, 0)
	return &message.Update{
		PathAttributes: buf[:attrLen],
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
		routesLogger().Debug("skipping IPv4 MVPN routes (not negotiated)", "peer", addr, "count", skippedIPv4)
	}
	if skippedIPv6 > 0 {
		routesLogger().Debug("skipping IPv6 MVPN routes (not negotiated)", "peer", addr, "count", skippedIPv6)
	}

	// RFC 8654: Respect peer's max message size (4096 or 65535)
	maxMsgSize := int(message.MaxMessageLength(message.TypeUPDATE, nc.ExtendedMessage))

	// Send IPv4 MVPN routes grouped by attributes (sorted for deterministic order)
	if len(ipv4Routes) > 0 {
		ipv4MVPNFamily := nlri.Family{AFI: 1, SAFI: 5} // IPv4 MVPN
		addPath := p.addPathFor(ipv4MVPNFamily)
		ub := message.NewUpdateBuilder(p.settings.LocalAS, p.settings.IsIBGP(), p.asn4(), addPath)
		ipv4Groups := groupMVPNRoutesByKey(ipv4Routes)
		for _, key := range sortedKeys(ipv4Groups) {
			routes := ipv4Groups[key]
			// Use size-aware builder to respect max message size
			updates, err := ub.BuildMVPNWithLimit(toMVPNParams(routes), maxMsgSize)
			if err != nil {
				routesLogger().Debug("MVPN build error", "peer", addr, "error", err)
				continue
			}
			for _, update := range updates {
				if err := p.SendUpdate(update); err != nil {
					routesLogger().Debug("MVPN send error", "peer", addr, "error", err)
					break
				}
			}
			routesLogger().Debug("sent IPv4 MVPN routes", "peer", addr, "routes", len(routes), "updates", len(updates))
		}
	}

	// Send IPv6 MVPN routes grouped by attributes (sorted for deterministic order)
	if len(ipv6Routes) > 0 {
		ipv6MVPNFamily := nlri.Family{AFI: 2, SAFI: 5} // IPv6 MVPN
		addPath := p.addPathFor(ipv6MVPNFamily)
		ub := message.NewUpdateBuilder(p.settings.LocalAS, p.settings.IsIBGP(), p.asn4(), addPath)
		ipv6Groups := groupMVPNRoutesByKey(ipv6Routes)
		for _, key := range sortedKeys(ipv6Groups) {
			routes := ipv6Groups[key]
			// Use size-aware builder to respect max message size
			updates, err := ub.BuildMVPNWithLimit(toMVPNParams(routes), maxMsgSize)
			if err != nil {
				routesLogger().Debug("MVPN build error", "peer", addr, "error", err)
				continue
			}
			for _, update := range updates {
				if err := p.SendUpdate(update); err != nil {
					routesLogger().Debug("MVPN send error", "peer", addr, "error", err)
					break
				}
			}
			routesLogger().Debug("sent IPv6 MVPN routes", "peer", addr, "routes", len(routes), "updates", len(updates))
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
			routesLogger().Debug("skipping VPLS routes (L2VPN VPLS not negotiated)", "peer", addr, "count", len(p.settings.VPLSRoutes))
		}
		return
	}

	addr := p.settings.Address.String()

	if len(p.settings.VPLSRoutes) > 0 {
		routesLogger().Debug("sending VPLS routes", "peer", addr, "count", len(p.settings.VPLSRoutes))
		// VPLS family: AFI=25 (L2VPN), SAFI=65 (VPLS)
		// Note: VPLS doesn't support ADD-PATH
		vplsFamily := nlri.Family{AFI: 25, SAFI: 65}
		addPath := p.addPathFor(vplsFamily)
		ub := message.NewUpdateBuilder(p.settings.LocalAS, p.settings.IsIBGP(), p.asn4(), addPath)
		for _, route := range p.settings.VPLSRoutes {
			update := ub.BuildVPLS(toVPLSParams(route))
			if err := p.SendUpdate(update); err != nil {
				routesLogger().Debug("VPLS send error", "peer", addr, "error", err)
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

	// RFC 8654: Respect peer's max message size (4096 or 65535)
	maxMsgSize := int(message.MaxMessageLength(message.TypeUPDATE, nc.ExtendedMessage))

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
			routesLogger().Debug("skipping FlowSpec route (family not negotiated)", "peer", addr)
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
		addPath := p.addPathFor(nlri.Family{AFI: nlri.AFI(afi), SAFI: nlri.SAFI(safi)})
		ub := message.NewUpdateBuilder(p.settings.LocalAS, p.settings.IsIBGP(), p.asn4(), addPath)
		// RFC 5575 Section 4: FlowSpec NLRI max 4095 bytes.
		// Single FlowSpec rule is atomic - cannot be split across UPDATEs.
		update, err := ub.BuildFlowSpecWithMaxSize(toFlowSpecParams(route), maxMsgSize)
		if err != nil {
			routesLogger().Debug("FlowSpec build error (too large?)", "peer", addr, "error", err)
			continue
		}
		if err := p.SendUpdate(update); err != nil {
			routesLogger().Debug("FlowSpec send error", "peer", addr, "error", err)
			continue
		}
		sentCount++
	}
	if sentCount > 0 {
		routesLogger().Debug("sent FlowSpec routes", "peer", addr, "count", sentCount)
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
		routesLogger().Debug("skipping IPv4 MUP routes (not negotiated)", "peer", addr, "count", skippedIPv4)
	}
	if skippedIPv6 > 0 {
		routesLogger().Debug("skipping IPv6 MUP routes (not negotiated)", "peer", addr, "count", skippedIPv6)
	}

	// Send IPv4 MUP routes
	if len(ipv4Routes) > 0 {
		routesLogger().Debug("sending IPv4 MUP routes", "peer", addr, "count", len(ipv4Routes))
		ipv4MUPFamily := nlri.Family{AFI: 1, SAFI: 85}
		addPath := p.addPathFor(ipv4MUPFamily)
		ub := message.NewUpdateBuilder(p.settings.LocalAS, p.settings.IsIBGP(), p.asn4(), addPath)
		for _, route := range ipv4Routes {
			update := ub.BuildMUP(toMUPParams(route))
			if err := p.SendUpdate(update); err != nil {
				routesLogger().Debug("MUP send error", "peer", addr, "error", err)
			}
		}
	}

	// Send IPv6 MUP routes
	if len(ipv6Routes) > 0 {
		routesLogger().Debug("sending IPv6 MUP routes", "peer", addr, "count", len(ipv6Routes))
		ipv6MUPFamily := nlri.Family{AFI: 2, SAFI: 85}
		addPath := p.addPathFor(ipv6MUPFamily)
		ub := message.NewUpdateBuilder(p.settings.LocalAS, p.settings.IsIBGP(), p.asn4(), addPath)
		for _, route := range ipv6Routes {
			update := ub.BuildMUP(toMUPParams(route))
			if err := p.SendUpdate(update); err != nil {
				routesLogger().Debug("MUP send error", "peer", addr, "error", err)
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
		routesLogger().Debug("watchdog marked routes for announce (disconnected)", "peer", p.settings.Address, "watchdog", name, "count", len(routes))
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
		nextHop, nhErr := p.resolveNextHop(wr.NextHop, routeFamily(wr.StaticRoute))
		if nhErr != nil {
			routesLogger().Debug("watchdog next-hop resolution failed", "peer", addr, "watchdog", name, "error", nhErr)
			continue
		}
		// RFC 7911: Get ADD-PATH encoding state
		addPath := p.addPathFor(routeFamily(wr.StaticRoute))
		update := buildStaticRouteUpdateNew(wr.StaticRoute, nextHop, p.settings.LinkLocal, p.settings.LocalAS, p.settings.IsIBGP(), p.asn4(), addPath, p.sendCtx)
		if err := p.SendUpdate(update); err != nil {
			return err
		}

		// Update state
		p.mu.Lock()
		p.watchdogState[name][routeKey] = true
		p.mu.Unlock()

		routesLogger().Debug("route sent", "peer", addr, "prefix", routeKey, "nextHop", wr.NextHop.String())
		announced++
	}

	if announced > 0 {
		routesLogger().Debug("watchdog announced routes", "peer", addr, "watchdog", name, "count", announced)
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
		routesLogger().Debug("watchdog marked routes for withdraw (disconnected)", "peer", p.settings.Address, "watchdog", name, "count", len(routes))
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
		// RFC 7911: Get ADD-PATH encoding state
		addPath := p.addPathFor(routeFamily(wr.StaticRoute))
		attrBuf := getBuildBuf()
		update := buildStaticRouteWithdraw(attrBuf, wr.StaticRoute, addPath)
		sendErr := p.SendUpdate(update)
		putBuildBuf(attrBuf)
		if sendErr != nil {
			return sendErr
		}

		// Update state
		p.mu.Lock()
		p.watchdogState[name][routeKey] = false
		p.mu.Unlock()

		routesLogger().Debug("watchdog withdrew route", "peer", addr, "watchdog", name, "route", routeKey)
		withdrawn++
	}

	if withdrawn > 0 {
		routesLogger().Debug("watchdog withdrew routes", "peer", addr, "watchdog", name, "count", withdrawn)
	}
	return nil
}
