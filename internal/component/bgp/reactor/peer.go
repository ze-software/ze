// Design: docs/architecture/core-design.md — BGP reactor event loop
// Detail: delivery.go — deliveryItem struct and batch drain
// Detail: peer_connection.go — peer TCP connection management
// Detail: peer_send.go — peer outbound message sending
// Detail: peer_initial_sync.go — initial route synchronization
// Detail: peer_rib_routes.go — RIB route extraction
// Detail: peer_static_routes.go — static route injection
// Detail: peer_stats.go — atomic message/route counters and uptime
// Detail: peer_run.go — peer run loop and session lifecycle
// Detail: routerid_unique.go — router-ID conflict detection
// Related: update_group.go — group join/leave keyed by sendCtxID

package reactor

import (
	"context"
	"errors"
	"net"
	"net/netip"
	"sync"
	"sync/atomic"
	"time"

	"codeberg.org/thomas-mangin/ze/internal/component/bgp/grmarker"
	bgptypes "codeberg.org/thomas-mangin/ze/internal/component/bgp/types"

	"codeberg.org/thomas-mangin/ze/internal/component/bgp/capability"
	bgpctx "codeberg.org/thomas-mangin/ze/internal/component/bgp/context"
	"codeberg.org/thomas-mangin/ze/internal/component/bgp/message"
	"codeberg.org/thomas-mangin/ze/internal/component/bgp/nlri"
	"codeberg.org/thomas-mangin/ze/internal/component/bgp/rib"
	"codeberg.org/thomas-mangin/ze/internal/component/plugin"
	"codeberg.org/thomas-mangin/ze/internal/core/clock"
	"codeberg.org/thomas-mangin/ze/internal/core/family"
	"codeberg.org/thomas-mangin/ze/internal/core/network"
	"codeberg.org/thomas-mangin/ze/internal/core/slogutil"
	"codeberg.org/thomas-mangin/ze/internal/core/source"
	"codeberg.org/thomas-mangin/ze/internal/core/syncutil"
)

// peerLogger is the peer subsystem logger (lazy initialization).
// Controlled by ze.log.bgp.reactor.peer environment variable.
var peerLogger = slogutil.LazyLogger("bgp.reactor.peer")

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
		return "stopped"
	case PeerStateConnecting:
		return "connecting"
	case PeerStateActive:
		return "active"
	case PeerStateEstablished:
		return "established"
	default:
		return "unknown"
	}
}

func (s PeerState) PluginState() plugin.PeerState {
	return plugin.PeerState(s)
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
	Message string     // For PeerOpTeardown: RFC 8203 shutdown communication
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
	clock    clock.Clock
	dialer   network.Dialer
	session  *Session

	// remoteRouterID is the peer's BGP Identifier from their OPEN message.
	// Set in validateOpen when OPEN is received, cleared on teardown.
	// Used by route reflection to set ORIGINATOR_ID (RFC 4456 Section 8).
	remoteRouterID atomic.Uint32

	// Negotiated capabilities: tracks which families are enabled.
	// Set when session transitions to Established, cleared on teardown.
	// Encoding details (AddPath, ExtNH, ASN4) live in sendCtx/recvCtx.
	// Uses atomic.Pointer for thread-safe access from multiple goroutines.
	negotiated atomic.Pointer[NegotiatedCapabilities]

	// Encoding contexts for this peer session.
	// Created at session establishment, cleared on teardown.
	// recvCtx is used when parsing routes FROM peer.
	// sendCtx is used when encoding routes TO peer.
	// sendCtx uses atomic.Pointer for lock-free reads from plugin dispatch goroutines
	// (e.g., WithdrawNLRIBatch via DirectBridge) that race with FSM teardown writes.
	recvCtx   *bgpctx.EncodingContext
	recvCtxID bgpctx.ContextID
	sendCtx   atomic.Pointer[bgpctx.EncodingContext]
	sendCtxID bgpctx.ContextID

	// updateGroupKey is the key under which this peer was registered in
	// the update group index. Stored here so Remove can find the correct
	// group even if sendCtxID has been cleared by clearEncodingContexts.
	// Zero value means not in any group.
	updateGroupKey GroupKey

	// Per-peer message and route counters for operational statistics.
	counters peerCounters

	state           atomic.Int32
	callback        PeerCallback
	messageCallback MessageCallback // Called when any BGP message is received

	// Per-peer async delivery channel for received UPDATEs.
	// Created in runOnce() before session.Run(), closed after session exits.
	// nil means synchronous delivery (no channel configured).
	deliverChan chan deliveryItem

	// Reconnect configuration
	reconnectMin time.Duration
	reconnectMax time.Duration

	// prefixTeardownCount tracks consecutive prefix-limit teardowns for exponential backoff.
	// Reset when a session stays established (successful Run return).
	prefixTeardownCount uint32

	// Active prefix-threshold and prefix-stale warnings live on the report bus
	// (internal/core/report). Producer-side dedup uses Session.prefixCounts.warned;
	// the bus is the single source of truth for queries and the login banner.

	// notificationExchanged is set true by IncrNotificationSent / IncrNotificationReceived
	// when a NOTIFICATION is sent or received during the current session lifecycle.
	// Read by the FSM Established->Idle transition handler in peer_run.go to suppress
	// the session-dropped error report when a notification has already been raised.
	// Reset to false at the start of each runOnce iteration.
	notificationExchanged atomic.Bool

	// Ordered operation queue: Used when session is NOT established.
	// Maintains strict ordering of announce/withdraw/teardown operations.
	// Processed on session establishment; teardowns act as batch separators.
	opQueue []PeerOp

	// sendingInitialRoutes gates route sending during session establishment.
	// States: 0=idle, 1=flag set by FSM (queuing enabled), 2=goroutine running.
	// Set to 1 by FSM callback BEFORE notifying plugins of state=up, ensuring
	// routes from plugin commands are queued. Upgraded to 2 by sendInitialRoutes.
	sendingInitialRoutes atomic.Int32

	// sendingConfigStatic is true while sendInitialRoutes sends config-originated
	// static routes. notifyMessageReceiver tags sent events with config-static meta
	// so the RIB plugin skips ribOut storage (these routes are re-sent from config
	// on every reconnection, storing them would cause duplicates).
	sendingConfigStatic atomic.Bool

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

	// reactor is set when peer is added to reactor.
	// Used to notify reactor of state changes.
	reactor *Reactor

	// sourceID identifies this peer in the source registry.
	// Assigned at creation, never changes.
	sourceID source.SourceID

	// addrString caches settings.Address.String() to avoid per-message
	// string allocation on the hot path (Prometheus labels, bus notifications,
	// forward pool keys). Computed once at peer creation.
	addrString string

	// Collision detection (RFC 4271 §6.8):
	// When an incoming connection arrives while we're in OpenConfirm,
	// we queue it here and wait for its OPEN to resolve the collision.
	pendingConn net.Conn      // Pending incoming connection
	pendingOpen *message.Open // OPEN received on pending connection

	// Inbound connection buffering for passive peers:
	// When a connection arrives while the session is nil (between runOnce iterations),
	// store it here so the next runOnce() can accept it immediately.
	inboundConn   net.Conn
	inboundNotify chan struct{}

	// bfd is the per-peer BFD client state. Zero value means no BFD
	// session is currently open; startBFDClient populates it after
	// the FSM reaches Established and the peer opted in via config.
	// stopBFDClient clears it on session teardown. See peer_bfd.go.
	bfd bfdClient
}

// NewPeer creates a new peer for the given settings.
func NewPeer(settings *PeerSettings) *Peer {
	reconnectMin := settings.ConnectRetry
	if reconnectMin == 0 {
		reconnectMin = DefaultReconnectMin
	}
	p := &Peer{
		settings:      settings,
		clock:         clock.RealClock{},
		dialer:        &network.RealDialer{},
		reconnectMin:  reconnectMin,
		reconnectMax:  DefaultReconnectMax,
		opQueue:       make([]PeerOp, 0, 16), // Pre-allocate small capacity
		sourceID:      source.DefaultRegistry.RegisterPeer(settings.Address, settings.PeerAS),
		inboundNotify: make(chan struct{}, 1),
		addrString:    settings.Address.String(),
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

// SetClock sets the clock used for delay and timeout operations.
// Propagated to sessions created by this peer. Must be called before Start.
func (p *Peer) SetClock(c clock.Clock) {
	p.clock = c
}

// SetDialer sets the dialer used for outbound connections.
// Propagated to sessions created by this peer. Must be called before Start.
func (p *Peer) SetDialer(d network.Dialer) {
	p.dialer = d
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
//
// Uses a single Lock (not RLock→WLock upgrade) to prevent a race where
// ResetAPISync replaces apiSyncReady between the read and close operations.
func (p *Peer) SignalAPIReady() {
	count := p.apiSyncCount.Add(1)
	p.mu.Lock()
	expected := p.apiSyncExpected
	if count >= expected && p.apiSyncReady != nil {
		p.apiSyncReadyOnce.Do(func() {
			close(p.apiSyncReady)
		})
	}
	p.mu.Unlock()
}

// waitForAPISync blocks until all API processes signal ready or timeout.
// Returns immediately if no API sync is expected.
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
	case <-p.clock.After(timeout):
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
	return p.sendCtx.Load()
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

	sctx := bgpctx.FromNegotiatedSend(neg)
	p.sendCtx.Store(sctx)
	if sctx != nil {
		p.sendCtxID = bgpctx.Registry.Register(sctx)
	}

	// Set context IDs on session for zero-copy WireUpdate and AttrsWire creation
	if p.session != nil {
		p.session.SetRecvCtxID(p.recvCtxID)
		p.session.SetSendCtxID(p.sendCtxID)
	}
}

// RemoteRouterID returns the peer's BGP Identifier from their OPEN message.
// Returns 0 if session has not been established or has been torn down.
// Used by route reflection to set ORIGINATOR_ID (RFC 4456 Section 8).
func (p *Peer) RemoteRouterID() uint32 {
	return p.remoteRouterID.Load()
}

// clearEncodingContexts clears the encoding contexts.
// Called when session is torn down.
func (p *Peer) clearEncodingContexts() {
	p.mu.Lock()
	defer p.mu.Unlock()

	p.recvCtx = nil
	p.recvCtxID = 0
	p.sendCtx.Store(nil)
	p.sendCtxID = 0
	p.remoteRouterID.Store(0)
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
//
// RFC 4724 Section 4.1: If within the restart window (RestartUntil), sets the
// Restart State bit (R=1) on code-64 capabilities so peers know we restarted.
func (p *Peer) getPluginCapabilities() []capability.Capability {
	p.mu.RLock()
	r := p.reactor
	settings := p.settings
	p.mu.RUnlock()

	if r == nil || r.api == nil {
		return nil
	}

	// Try peer name first, then IP address (plugins may key by either).
	injected := r.api.GetPluginCapabilitiesForPeer(settings.Name)
	if len(injected) == 0 {
		injected = r.api.GetPluginCapabilitiesForPeer(settings.Address.String())
	}
	if len(injected) == 0 {
		return nil
	}

	// RFC 4724 Section 4.1: Set R=1 on GR capabilities while within restart window.
	// After the deadline, new connections get R=0 (cold start behavior).
	if !r.config.RestartUntil.IsZero() && p.clock.Now().Before(r.config.RestartUntil) {
		injected = grmarker.SetRBit(injected)
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

// validateOpen checks router-ID uniqueness and delegates OPEN validation to plugins.
// Used as callback for Session.SetOpenValidator().
func (p *Peer) validateOpen(peerAddr string, local, remote *message.Open) error {
	p.mu.RLock()
	r := p.reactor
	p.mu.RUnlock()

	if r == nil {
		return nil
	}

	// Store the remote peer's BGP Identifier for route reflection (ORIGINATOR_ID).
	// RFC 4456 Section 8: ORIGINATOR_ID carries the BGP Identifier of the originator.
	p.remoteRouterID.Store(remote.BGPIdentifier)

	// RFC 4271 Section 4.2: BGP Identifier MUST be unique within an AS.
	// Reject if another ESTABLISHED peer in the same ASN has the same router-ID.
	r.mu.RLock()
	conflictAddr, conflict := checkRouterIDConflict(
		r.peers, p.settings.PeerKey(), p.settings.PeerAS, remote.BGPIdentifier)
	r.mu.RUnlock()
	if conflict {
		return &routerIDConflictError{
			conflictAddr: conflictAddr,
			peerAS:       p.settings.PeerAS,
			bgpID:        remote.BGPIdentifier,
		}
	}

	if r.eventDispatcher == nil {
		return nil
	}

	return r.eventDispatcher.BroadcastValidateOpen(peerAddr, local, remote)
}

// addPathFor returns whether ADD-PATH is negotiated for the given family.
// RFC 7911: ADD-PATH requires 4-byte path identifier prefix on NLRI.
// Returns false if session not established.
func (p *Peer) addPathFor(fam family.Family) bool {
	ctx := p.sendCtx.Load()
	if ctx == nil {
		return false
	}
	return ctx.AddPath(fam)
}

// asn4 returns whether 4-byte ASN is negotiated.
// RFC 6793: ASN4 determines 2-byte vs 4-byte AS numbers in AS_PATH.
// Returns true if session not established (default to modern).
func (p *Peer) asn4() bool {
	ctx := p.sendCtx.Load()
	if ctx == nil {
		return true
	}
	return ctx.ASN4()
}

// resolveNextHop returns the actual IP address for a RouteNextHop policy.
// Uses session's LocalAddress for Self, validates against Extended NH capability.
//
// RFC 4271 Section 5.1.3 - NEXT_HOP attribute.
// RFC 5549/8950 - Extended Next Hop Encoding.
func (p *Peer) resolveNextHop(nh bgptypes.RouteNextHop, fam family.Family) (netip.Addr, error) {
	switch nh.Policy {
	case bgptypes.NextHopExplicit:
		// Explicit addresses bypass validation - user is responsible.
		// Returns invalid addr without error if that's what was configured.
		return nh.Addr, nil

	case bgptypes.NextHopSelf:
		local := p.settings.LocalAddress
		if !local.IsValid() {
			return netip.Addr{}, ErrNextHopSelfNoLocal
		}
		// Validate: can we use this address for this NLRI family?
		if !p.canUseNextHopFor(local, fam) {
			return netip.Addr{}, ErrNextHopIncompatible
		}
		return local, nil

	case bgptypes.NextHopUnset:
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
func (p *Peer) canUseNextHopFor(addr netip.Addr, fam family.Family) bool {
	// Natural match - always allowed
	if addr.Is4() && fam.AFI == family.AFIIPv4 {
		return true
	}
	if addr.Is6() && fam.AFI == family.AFIIPv6 {
		return true
	}

	// Cross-family via Extended NH (RFC 5549/8950)
	ctx := p.sendCtx.Load()
	if ctx != nil {
		nhAFI := ctx.ExtendedNextHopFor(fam)
		if nhAFI != 0 {
			if addr.Is6() && nhAFI == family.AFIIPv6 {
				return true
			}
			if addr.Is4() && nhAFI == family.AFIIPv4 {
				return true
			}
		}
	}
	return false
}

// State returns the current peer state.
func (p *Peer) State() PeerState {
	return PeerState(p.state.Load())
}

// setState updates state and calls callback.
func (p *Peer) setState(s PeerState) {
	old := PeerState(p.state.Swap(int32(s)))
	if old != s {
		p.updatePeerStateMetric(old, s)
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

// ErrOpQueueFull is returned when the operation queue is full and the teardown
// cannot be queued. This prevents the API from reporting success when the
// teardown was silently dropped.
var ErrOpQueueFull = errors.New("operation queue full")

// Teardown sends a Cease NOTIFICATION with the given subcode and closes.
// The session will send NOTIFICATION before closing the connection.
// RFC 4486 defines Cease subcodes; RFC 8203 defines the shutdown message.
// If called when sendInitialRoutes is running, queues the teardown so that
// EOR can be sent before NOTIFICATION. If not connected, also queues.
// Returns ErrOpQueueFull if the teardown could not be queued.
func (p *Peer) Teardown(subcode uint8, shutdownMsg string) error {
	p.mu.Lock()
	session := p.session

	// If sendInitialRoutes is pending (1) or running (2), queue the teardown
	// so it can send EOR before executing the teardown. This ensures proper
	// BGP protocol sequencing: routes + EOR + NOTIFICATION.
	if p.sendingInitialRoutes.Load() != 0 {
		if len(p.opQueue) < MaxOpQueueSize {
			p.opQueue = append(p.opQueue, PeerOp{Type: PeerOpTeardown, Subcode: subcode, Message: shutdownMsg})
			p.mu.Unlock()
			return nil
		}
		p.mu.Unlock()
		routesLogger().Warn("opQueue full, dropping teardown", "peer", p.settings.Address)
		return ErrOpQueueFull
	}

	if session != nil {
		p.mu.Unlock()
		if err := session.Teardown(subcode, shutdownMsg); err != nil {
			peerLogger().Debug("teardown error", "peer", p.settings.Address, "error", err)
		}
		// Set state after teardown - there's a brief race window where
		// AnnounceRoute might see ESTABLISHED, but SendUpdate will fail
		// on the closed session (which is correct behavior)
		p.setState(PeerStateConnecting)
		return nil
	}

	// No active session - queue teardown to maintain operation order
	if len(p.opQueue) < MaxOpQueueSize {
		p.opQueue = append(p.opQueue, PeerOp{Type: PeerOpTeardown, Subcode: subcode, Message: shutdownMsg})
		p.mu.Unlock()
		return nil
	}
	p.mu.Unlock()
	routesLogger().Warn("opQueue full, dropping teardown", "peer", p.settings.Address)
	return ErrOpQueueFull
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
		routesLogger().Warn("opQueue full, dropping announce", "peer", p.settings.Address, "queueSize", len(p.opQueue), "nlri", route.NLRI())
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
		routesLogger().Warn("opQueue full, dropping withdraw", "peer", p.settings.Address, "queueSize", len(p.opQueue), "nlri", n)
		return
	}
	p.opQueue = append(p.opQueue, PeerOp{Type: PeerOpWithdraw, NLRI: n})
}

// Wait waits for the peer to stop.
func (p *Peer) Wait(ctx context.Context) error {
	return syncutil.WaitGroupWait(ctx, &p.wg)
}
