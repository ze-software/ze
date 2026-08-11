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

	"github.com/ze-software/ze/internal/component/bgp/filterapi"
	"github.com/ze-software/ze/internal/component/bgp/fsm"
	"github.com/ze-software/ze/internal/component/bgp/grmarker"
	bgptypes "github.com/ze-software/ze/internal/component/bgp/types"

	"github.com/ze-software/ze/internal/component/bgp/message"
	"github.com/ze-software/ze/internal/component/bgp/rib"
	"github.com/ze-software/ze/internal/component/plugin"
	"github.com/ze-software/ze/internal/core/bgp/capability"
	bgpctx "github.com/ze-software/ze/internal/core/bgp/context"
	"github.com/ze-software/ze/internal/core/bgp/nlri"
	"github.com/ze-software/ze/internal/core/clock"
	"github.com/ze-software/ze/internal/core/family"
	"github.com/ze-software/ze/internal/core/network"
	"github.com/ze-software/ze/internal/core/slogutil"
	"github.com/ze-software/ze/internal/core/source"
	"github.com/ze-software/ze/internal/core/syncutil"
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
	// PeerStateIdleHold means the peer is deliberately down and is trying
	// nothing. A prefix limit stopped the session and the offending family
	// asked for no reconnect (ze-bgp-conf.yang, prefix reconnect never), so the
	// peer waits for an operator. It is distinct from PeerStateStopped, which
	// means the peer is not running at all.
	//
	// New values go at the END: plugin.PeerState mirrors this list and
	// PluginState converts by value (types_bgp.go).
	PeerStateIdleHold
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
	case PeerStateIdleHold:
		return "idle-hold"
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

	// Automatic is set when the LOCAL SYSTEM chose this teardown rather than
	// the operator, so draining it raises RFC 4271 Event 8 (AutomaticStop)
	// instead of Event 2 (ManualStop). It has to survive the queue: the two
	// events differ on the ConnectRetryCounter (§8.2.2), and a teardown that
	// waited for End-of-RIB is not thereby an operator's decision.
	Automatic bool // For PeerOpTeardown
}

// sessionTeardown routes a teardown to the Session method whose RFC 4271 stop
// event matches its origin. One call site would otherwise have to repeat the
// if/else at every drain point (peer_initial_sync.go has two).
func sessionTeardown(session *Session, subcode uint8, shutdownMsg string, automatic bool) error {
	if automatic {
		return session.TeardownAutomatic(subcode, shutdownMsg)
	}
	return session.Teardown(subcode, shutdownMsg)
}

// DefaultOpQueueSize is the default maximum number of operations that can be
// queued when the session is not established. Scaled up when the peer has
// prefix-maximum configured (one queue slot per expected prefix).
const DefaultOpQueueSize = 10000

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

	// Negotiated timer values (set during OPEN exchange, cleared on teardown).
	negotiatedHoldTime      atomic.Uint32 // seconds
	negotiatedKeepaliveTime atomic.Uint32 // seconds

	state           atomic.Int32
	callback        PeerCallback
	messageCallback MessageCallback // Called when any BGP message is received
	history         *fsmHistory

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

	// connectRetryCounter is RFC 4271 §8.1.1 mandatory session attribute 2:
	// "the number of times a BGP peer has tried to establish a peer session".
	// The FSM §8.2.2 handlers own every mutation; the Peer owns the VALUE,
	// because runOnce builds a new Session and a new FSM per cycle and the
	// counter has to outlive them to count them (see fsm.ConnectRetryCounter).
	// runOnce hands it to each cycle's FSM.
	connectRetryCounter fsm.ConnectRetryCounter

	// operatorStarted is false until StartWithContext has handed the first
	// connection cycle its RFC 4271 Event 1 (ManualStart). Later cycles get
	// Event 6 (AutomaticStart_with_DampPeerOscillations) instead, so the
	// §8.2.2 "sets ConnectRetryCounter to zero" clause on Event 1 fires when
	// the operator starts the peer and not on every damped retry. Reset by
	// StartWithContext so stopping and restarting a peer is a fresh start.
	operatorStarted atomic.Bool

	// stopping is set by Reactor.stop (reactor.go) on every peer BEFORE it
	// notifies them, and it closes the only window in which p.ctx says "keep
	// going" while the engine is already leaving. Stop has to notify first --
	// the cancel closes the sockets the NOTIFICATION needs -- so for the whole
	// shutdown budget every peer context is still live, and a session torn down
	// by ShutdownNotify would send run() straight back round the loop to dial
	// again.
	//
	// It is read TWICE, and the second read is the load-bearing one. run()'s
	// loop top is the cheap early exit. runOnce reads it again inside the p.mu
	// hold that publishes p.session (peer_run.go), which is the field
	// ShutdownNotify reads under the same lock: that pairing is what makes "the
	// notify covered this session, or this session was never published" a
	// guarantee rather than a likelihood. The loop top alone can be passed
	// before Reactor.stop sets the mark, and the dial that follows takes a TCP
	// handshake.
	//
	// Cleared only by Reactor.StartWithContext, for every peer under the r.mu
	// that stop() sets it under. Peer.StartWithContext holds no such lock, so it
	// must not clear it: Reactor.StartPeers reaches that method with r.mu
	// released, and a SIGTERM during plugin startup used to re-open the outbound
	// rail on a peer the stop had already marked and notified.
	//
	// Nothing on the accept path reads it -- acceptOrReject runs on a Listener
	// goroutine (reactor_connection.go) and asks the peer, not this flag. What
	// this flag gives that path is narrower and is what the seal is built on:
	// after the mark, the peer publishes NO new session, so the accept rail can
	// only reach the one session that was already published. Reactor.stop seals
	// that one directly (sealSession below), and Session.connectionEstablished
	// reads the seal inside the s.mu hold that publishes s.conn.
	//
	// Shutting the listeners and the r.stopping reads in
	// startListenerForAddressPort and tryCreateDynamicPeer stop new SOCKETS and
	// new PEERS. Neither of them stops a conn that a Listener had already
	// accepted for a configured peer, which is why the seal is read at the two
	// places a conn becomes a session rather than on each rail that reaches them.
	stopping atomic.Bool

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
	opQueue    []PeerOp
	opQueueMax int

	// sendingInitialRoutes gates route sending during session establishment.
	// States: 0=idle, 1=gate closed (queuing enabled), 2=goroutine running.
	// Set to 1 by setState, in the same call that publishes PeerStateEstablished
	// and BEFORE it, so no goroutine can ever read an established peer whose
	// route ops still bypass opQueue. Upgraded to 2 by sendInitialRoutes.
	sendingInitialRoutes atomic.Int32

	// sendingConfigStatic is true while sendInitialRoutes sends config-originated
	// static routes. notifyMessageReceiver tags sent events with config-static meta
	// so the RIB plugin skips ribOut storage (these routes are re-sent from config
	// on every reconnection, storing them would cause duplicates).
	sendingConfigStatic atomic.Bool

	// initialSyncEOR records, per family, that THIS session's initial sync has
	// already put an End-of-RIB on the wire. RFC 4724 Section 2 allows exactly one
	// per family per session, and there are two producers: sendInitialRoutes'
	// own loop, and a route server announcing EoR when its replay finishes
	// (rs/server_handlers.go sendEOR -> AnnounceEOR).
	//
	// It replaces a TIME-WINDOW test as the de-duplicator. AnnounceEOR gated on
	// ShouldQueue(), i.e. on sendingInitialRoutes still being non-zero; when the
	// route-server replay finished after that flag cleared, the guard failed open
	// and the peer received the same family's EoR twice
	// (ai/rules/evidence.md). Whether the marker is already on the wire
	// is a FACT about this session, not a question about how long ago something
	// started, so it is recorded as one.
	//
	// Guarded by mu. Reset with the session in runOnce's teardown defer, so a
	// reconnect legitimately sends EoR again.
	initialSyncEOR map[family.Family]bool

	// API sync for EOR: wait for API processes to finish initial routes before EOR.
	// Reset on each session establishment, signaled by "plugin session ready" commands.
	apiSyncExpected  int32         // Number of ready signals expected (processes with SendUpdate)
	apiSyncReady     chan struct{} // Closed when all expected ready signals received
	apiSyncReadyOnce sync.Once     // Ensures channel is closed only once
	apiSyncCount     atomic.Int32  // Count of ready signals received since session start

	// Peer-up barrier: plugins that must have PROCESSED this session's peer-up
	// event before its initial-sync End-of-RIB goes out, so that "End-of-RIB
	// sent" implies "every such plugin has registered this peer". Distinct from
	// apiSync above, which counts plugins that SEND routes: a route sender
	// signaling early must not satisfy a registrar's obligation, so the two
	// barriers never share a counter (ai/rules/evidence.md).
	//
	// Guarded by mu, reset per session establishment before plugins are notified.
	//
	// peerUpArmed distinguishes "not armed yet" from "armed, expecting none".
	// Without it both read as expected==0 and a signal arriving before the count
	// is set would satisfy `count >= expected`, close the channel and spend the
	// sync.Once, leaving the session's barrier permanently open with nothing able
	// to reinstate it short of another reset.
	peerUpArmed     bool          // SetPeerUpBarrier has run for this session
	peerUpExpected  int32         // Barrier-declaring plugins the peer-up event is delivered to
	peerUpReady     chan struct{} // Closed once every one of them has taken delivery
	peerUpReadyOnce sync.Once     // Ensures the channel is closed only once
	peerUpCount     atomic.Int32  // Deliveries acknowledged since session start

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
	// localAddrString caches settings.LocalAddress.String() for PeerInfo snapshots.
	localAddrString string

	// dynImportFilters/dynExportFilters store the original unresolved filter
	// chains for dynamic peers. Captured on first Established transition so
	// reconnections re-resolve from the template, not from stale resolved values.
	dynImportFilters []filterapi.FilterRef
	dynExportFilters []filterapi.FilterRef

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

	health *sessionHealth

	fwdFacts atomic.Pointer[peerForwardFacts]

	// fwdOverflowPending is this peer's handle to the count its forward worker
	// keeps of the bytes it still owes this peer through overflow
	// (forward_pool.go, fwdWorker.overflowPending). DispatchOverflow publishes
	// it when it parks the first real item for this peer, and nothing clears
	// it: the counter it names reads zero once the worker has written what it
	// owed, and a later worker for the same peer republishes its own.
	//
	// The counter stays on the worker, mutated only under that worker's
	// overflowMu. This is a read handle, so the route-server rail asks with two
	// atomic loads (forwardOverflowPending) instead of a pool lock and a key
	// lookup. That rail exists to bypass the pool.
	fwdOverflowPending atomic.Pointer[atomic.Int64]

	// llScope holds the host facts RFC 2545 Section 3 needs to decide whether
	// the link-local address belongs in the MP_REACH_NLRI Next Hop field. It is
	// refreshed with fwdFacts; nil means the interface table has not been read
	// for this peer yet, which appends no link-local (link_scope.go).
	llScope atomic.Pointer[linkScope]
}

// NewPeer creates a new peer for the given settings.
func NewPeer(settings *PeerSettings) *Peer {
	// Reconnect backoff uses the exponential range DefaultReconnectMin..
	// DefaultReconnectMax (see peer_run.go run()), NOT the RFC 4271
	// ConnectRetryTimer (settings.ConnectRetry, default 120s). Wiring ConnectRetry
	// in as the backoff floor made floor(120s) exceed ceiling(60s) and stranded a
	// peer 'connecting' for 2 minutes after a single failed attempt, so any
	// transient establishment hiccup read as "never established"
	// (spec-fixit-redistribute-establishment-stall). ConnectRetry remains the
	// connect timeout (reactor_dynamic.go).
	reconnectMin := DefaultReconnectMin
	addrStr := settings.Address.String()
	clk := clock.RealClock{}

	const maxOpQueueCap = 2_000_000
	queueMax := DefaultOpQueueSize
	for _, v := range settings.PrefixMaximum {
		if int(v) > queueMax {
			queueMax = int(v)
		}
	}
	if queueMax > maxOpQueueCap {
		queueMax = maxOpQueueCap
	}

	p := &Peer{
		settings:        settings,
		clock:           clk,
		dialer:          &network.RealDialer{},
		reconnectMin:    reconnectMin,
		reconnectMax:    DefaultReconnectMax,
		opQueue:         make([]PeerOp, 0, 16), // Pre-allocate small capacity
		opQueueMax:      queueMax,
		sourceID:        source.DefaultRegistry.RegisterPeer(settings.Address, settings.PeerAS),
		inboundNotify:   make(chan struct{}, 1),
		addrString:      addrStr,
		localAddrString: settings.LocalAddress.String(),
		history:         newFSMHistory(),
		health:          newSessionHealth(addrStr, clk),
	}

	return p
}

// Settings returns the configured peer settings.
//
// The returned pointer is shared, and five fields on the pointed-to struct are written
// after construction, always under p.mu. resolveDynamicPeerSettings (reactor_dynamic.go)
// writes PeerAS, ImportFilters and ExportFilters when a dynamic peer's session
// establishes; applyHotSwappableSettings (peer_settings_apply.go) writes ImportFilters,
// ExportFilters and PrefixUpdated when a config reload delivers a hot-swappable change,
// and Capabilities as well when the reload proved the negotiation unchanged
// (peer_settings_negotiation.go). A caller running on a different goroutine than those
// writes MUST read those five fields through PeerAS()/ImportFilters()/ExportFilters()/
// OldestPrefixUpdated()/ConfiguredCapabilities(), not off this pointer, or it races the
// write. Every other PeerSettings field is set at construction and never mutated, so
// reading it off this pointer is race-free.
//
// A caller that needs the WHOLE struct rather than one field uses SettingsSnapshot.
// There is no goroutine that owns every write to this struct, so no caller can be
// excused the lock by naming one: applyHotSwappableSettings runs on the reload
// goroutine and resolveDynamicPeerSettings runs on the establishment goroutine, and
// the two write overlapping fields.
func (p *Peer) Settings() *PeerSettings {
	return p.settings
}

// SettingsSnapshot returns a copy of the peer's settings taken under p.mu, for a
// caller that must read the struct as a whole.
//
// The reload path is that caller. reconcilePeersJournaled (reactor_api.go) compares
// the configured peers against the running ones and hands the result to
// peerSettingsSwapPlan (peer_settings_apply.go), which copies the struct and reads
// Capabilities off it. Both are whole-struct reads of the five mutable fields, so
// neither is served by the per-field accessors, and taking the lock here rather than
// inside the decision keeps that decision a pure function of two PeerSettings.
//
// The copy is shallow, and that is sufficient rather than a shortcut: every writer
// REPLACES a slice header or a map (resolveDynamicPeerSettings, reactor_dynamic.go,
// and applyHotSwappableSettings, peer_settings_apply.go), and no backing array or map
// is mutated in place. A shallow copy under the lock therefore observes one consistent
// set of headers, and what they point at never changes afterwards.
func (p *Peer) SettingsSnapshot() *PeerSettings {
	p.mu.RLock()
	defer p.mu.RUnlock()
	snapshot := *p.settings
	return &snapshot
}

// PeerAS returns the peer's ASN under p.mu. For a dynamic peer this is 0 until the OPEN is
// processed and resolveDynamicPeerSettings publishes the learned ASN; a static peer sets it
// at construction and never mutates it. Cross-goroutine readers MUST use this accessor
// rather than p.settings.PeerAS, which resolveDynamicPeerSettings writes under p.mu.
func (p *Peer) PeerAS() uint32 {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.settings.PeerAS
}

// ImportFilters returns the peer's import filter chain under p.mu. resolveDynamicPeerSettings
// replaces this slice (with $-variables resolved) on a dynamic peer's establishment, so
// cross-goroutine readers MUST use this accessor. The returned header is a snapshot; the
// backing array is never mutated in place (resolveFilterVars always allocates a new slice).
func (p *Peer) ImportFilters() []filterapi.FilterRef {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.settings.ImportFilters
}

// ExportFilters returns the peer's export filter chain under p.mu. See ImportFilters.
func (p *Peer) ExportFilters() []filterapi.FilterRef {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.settings.ExportFilters
}

// configuredCapabilities returns the capabilities the config asks this peer to
// advertise, under p.mu. A reload swap replaces the slice when the new set would
// negotiate to the same result (peer_settings_negotiation.go), so cross-goroutine
// readers MUST use this accessor rather than p.settings.Capabilities. The returned
// header is a snapshot; the backing array is never mutated in place.
func (p *Peer) configuredCapabilities() []capability.Capability {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.settings.Capabilities
}

// OldestPrefixUpdated returns the oldest per-family PeeringDB refresh date under p.mu.
// applyHotSwappableSettings (peer_settings_apply.go) replaces the whole PrefixUpdated map
// when a reload delivers new dates, so cross-goroutine readers MUST use this accessor:
// PeerSettings.OldestPrefixUpdated walks the map and reads the field more than once, and
// an unguarded walk can pair the old map's keys with the new map's values.
func (p *Peer) OldestPrefixUpdated() string {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.settings.OldestPrefixUpdated()
}

// IsIBGP reports whether this is an IBGP session (LocalAS == PeerAS) under p.mu.
// Cross-goroutine callers MUST use this rather than settings.IsIBGP(), which reads the
// mutable PeerAS off the shared settings pointer without synchronization.
func (p *Peer) IsIBGP() bool {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.settings.LocalAS == p.settings.PeerAS
}

// IsEBGP reports whether this is an EBGP session under p.mu. See IsIBGP.
func (p *Peer) IsEBGP() bool {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.settings.LocalAS != p.settings.PeerAS
}

// NegotiatedHoldTime returns the negotiated hold time in seconds.
func (p *Peer) NegotiatedHoldTime() uint32 { return p.negotiatedHoldTime.Load() }

// NegotiatedKeepaliveTime returns the negotiated keepalive time in seconds.
func (p *Peer) NegotiatedKeepaliveTime() uint32 { return p.negotiatedKeepaliveTime.Load() }

// tCPPorts returns the local and remote TCP ports of the active session.
// Returns 0, 0 if no session or connection is active.
func (p *Peer) tCPPorts() (localPort, remotePort uint16) {
	p.mu.RLock()
	sess := p.session
	p.mu.RUnlock()
	if sess == nil {
		return 0, 0
	}
	sess.mu.RLock()
	conn := sess.conn
	sess.mu.RUnlock()
	if conn == nil {
		return 0, 0
	}
	if addr := conn.LocalAddr(); addr != nil {
		if la, ok := addr.(*net.TCPAddr); ok {
			localPort = uint16(la.Port)
		}
	}
	if addr := conn.RemoteAddr(); addr != nil {
		if ra, ok := addr.(*net.TCPAddr); ok {
			remotePort = uint16(ra.Port)
		}
	}
	return localPort, remotePort
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

// waitForAPISync blocks until all API processes signal ready, or until
// apiSyncTimeout.
// Returns immediately if no API sync is expected.
func (p *Peer) waitForAPISync() {
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
	case <-p.clock.After(apiSyncTimeout):
		// Timeout - proceed anyway to avoid blocking forever
		routesLogger().Debug("API sync timeout", "peer", addr)
		return
	}
}

// ResetPeerUpBarrier clears the peer-up barrier for a new session.
// Called at Established BEFORE plugins are notified, so the barrier state a
// signal lands on always belongs to the session that is establishing.
//
// Expected starts at zero: a session whose peer-up event reaches no
// barrier-declaring plugin must not wait at all. SetPeerUpBarrier raises it.
func (p *Peer) ResetPeerUpBarrier() {
	p.mu.Lock()
	p.peerUpArmed = false
	p.peerUpExpected = 0
	p.peerUpReady = make(chan struct{})
	p.peerUpReadyOnce = sync.Once{}
	p.peerUpCount.Store(0)
	p.mu.Unlock()
}

// SetPeerUpBarrier declares how many barrier plugins this session's peer-up
// event is being delivered to. Called by the event dispatcher before the first
// delivery, so it is always ordered before any SignalPeerUpBarrier and before
// sendInitialRoutes is spawned.
//
// Also the release valve: the dispatcher lowers the count to the number of
// acknowledgements it actually obtained when a delivery is skipped or fails.
// Lowering it opens the barrier at once, because no further acknowledgement can
// arrive and waiting out the timeout would only delay the End-of-RIB without
// making the guarantee any truer.
//
// expected <= 0 opens the barrier immediately: there is nothing to wait for.
func (p *Peer) SetPeerUpBarrier(expected int) {
	p.mu.Lock()
	p.peerUpArmed = true
	p.peerUpExpected = int32(expected)                                                      //nolint:gosec // plugin count will never overflow int32
	if p.peerUpReady != nil && (expected <= 0 || p.peerUpCount.Load() >= int32(expected)) { //nolint:gosec // plugin count will never overflow int32
		p.peerUpReadyOnce.Do(func() { close(p.peerUpReady) })
	}
	p.mu.Unlock()
}

// SignalPeerUpBarrier records that one barrier plugin has taken delivery of
// this session's peer-up event. Opens the barrier once all expected plugins
// have.
//
// Ignores the close while unarmed. A signal that arrives before the count is
// set cannot satisfy a count nobody has stated yet, and letting it close the
// channel would spend the sync.Once and leave the session's barrier open for
// good. The count is still recorded, so SetPeerUpBarrier settles it.
//
// Single Lock (not RLock then Lock) for the same reason SignalAPIReady uses
// one: ResetPeerUpBarrier must not be able to replace the channel between the
// read and the close.
func (p *Peer) SignalPeerUpBarrier() {
	count := p.peerUpCount.Add(1)
	p.mu.Lock()
	if p.peerUpArmed && count >= p.peerUpExpected && p.peerUpReady != nil {
		p.peerUpReadyOnce.Do(func() { close(p.peerUpReady) })
	}
	p.mu.Unlock()
}

// waitPeerUpBarrier blocks until every barrier plugin has taken delivery of the
// peer-up event, or until peerUpBarrierTimeout. Returns true when the barrier
// opened.
//
// Returns immediately when nothing is expected, which is the common case: with
// no barrier plugin loaded the peer pays nothing, and with an in-process one it
// pays a closed-channel receive, because the delivery completes on the FSM
// callback goroutine before this goroutine is spawned.
//
// Bounded on purpose. A plugin that never takes delivery must not wedge
// establishment, so the timeout releases the End-of-RIB -- and says so, because
// past that point a peer treating the End-of-RIB as "I am a registered forward
// target" is being told something the engine could not confirm.
func (p *Peer) waitPeerUpBarrier() bool {
	p.mu.RLock()
	expected := p.peerUpExpected
	ready := p.peerUpReady
	p.mu.RUnlock()

	if expected == 0 || ready == nil {
		return true
	}

	select {
	case <-ready:
		return true
	case <-p.clock.After(peerUpBarrierTimeout):
		routesLogger().Warn("peer-up barrier timeout; end-of-rib no longer implies every plugin registered this peer",
			"peer", p.settings.Address,
			"acknowledged", p.peerUpCount.Load(),
			"expected", expected,
			"timeout", peerUpBarrierTimeout)
		return false
	}
}

// recvContext returns the receive encoding context.
// Used when parsing routes received FROM this peer.
// Returns nil if session is not established.
func (p *Peer) recvContext() *bgpctx.EncodingContext {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.recvCtx
}

// recvContextID returns the receive context ID.
// Used for fast compatibility checks.
func (p *Peer) recvContextID() bgpctx.ContextID {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.recvCtxID
}

// sendContext returns the send encoding context.
// Used when encoding routes TO this peer.
// Returns nil if session is not established.
func (p *Peer) sendContext() *bgpctx.EncodingContext {
	return p.sendCtx.Load()
}

// sendContextID returns the send context ID.
// Used for fast compatibility checks.
func (p *Peer) sendContextID() bgpctx.ContextID {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.sendCtxID
}

// setEncodingContexts creates and stores encoding contexts from negotiation.
// Called when session transitions to Established.
func (p *Peer) setEncodingContexts(neg *capability.Negotiated) {
	p.mu.Lock()

	p.recvCtx = bgpctx.FromNegotiatedRecv(neg)
	if p.recvCtx != nil {
		id, err := bgpctx.Registry.Register(p.recvCtx)
		if err != nil {
			reactorLogger().Error("context ID exhausted for recv context", "peer", p.addrString, "error", err)
		} else {
			p.recvCtxID = id
		}
	}

	sctx := bgpctx.FromNegotiatedSend(neg)
	p.sendCtx.Store(sctx)
	if sctx != nil {
		id, err := bgpctx.Registry.Register(sctx)
		if err != nil {
			reactorLogger().Error("context ID exhausted for send context", "peer", p.addrString, "error", err)
		} else {
			p.sendCtxID = id
		}
	}

	if p.session != nil {
		p.session.SetRecvCtxID(p.recvCtxID)
		p.session.setSendCtxID(p.sendCtxID)
	}

	p.mu.Unlock()

	p.refreshForwardFacts()
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
	p.fwdFacts.Store(nil)

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

	// RFC 6286 Section 2.1: the BGP Identifier SHOULD be unique within an AS. Ze enforces
	// that by default by CLAIMING the identifier for this peer here, while its OPEN is
	// validated -- so two peers of one AS presenting one identifier at the same instant race
	// for a single registry entry and exactly one wins, whatever the scheduler does. An
	// operator opts out with bgp/session/allow-shared-router-id when the duplication is
	// intentional, e.g. one anycast speaker (AS112) peering over both IPv4 and IPv6 with the
	// same router-id; because uniqueness is only a SHOULD, ze then performs no AS-wide
	// identifier check at all and both behaviors are conformant.
	//
	// This is independent of RFC 6286 Section 2.2 (a zero identifier, or this speaker's OWN
	// identifier from an internal peer), which Session.validateOpenIdentifier has already
	// rejected on both OPEN rails and which the opt-out does NOT relax.
	if !r.config.AllowSharedRouterID {
		peerAS := p.claimPeerAS(remote)
		conflictAddr, claimed := r.routerIDs.claim(p, p.settings.Address, peerAS, remote.BGPIdentifier)
		if !claimed {
			return &routerIDConflictError{
				conflictAddr: conflictAddr,
				peerAS:       peerAS,
				bgpID:        remote.BGPIdentifier,
			}
		}
	}

	if r.eventDispatcher == nil {
		return nil
	}

	return r.eventDispatcher.BroadcastValidateOpen(peerAddr, local, remote)
}

// claimPeerAS returns the AS that scopes this peer's BGP Identifier claim.
//
// RFC 6286 Section 2.1 scopes identifier uniqueness to an AS, so the claim key needs the
// peer's AS at OPEN time. A configured peer has it in its settings. A DYNAMIC peer does not:
// resolveDynamicPeerSettings publishes the learned ASN at establishment, which is after this
// runs, so its configured PeerAS is still the template value. Fall back to the AS the peer
// advertises in its OPEN, computed the same way resolveDynamicPeerSettings computes it
// (RFC 6793: the 4-byte ASN when present, else the two-octet My AS).
//
// Reads PeerAS through the p.mu-guarded accessor because resolveDynamicPeerSettings writes it
// from the establishment goroutine.
func (p *Peer) claimPeerAS(remote *message.Open) uint32 {
	if as := p.PeerAS(); as != 0 {
		return as
	}
	return openAdvertisedAS(remote)
}

// openAdvertisedAS returns the AS a peer advertises in its OPEN, per RFC 6793:
// the 4-byte ASN from the AS4 capability when present, else the two-octet My AS.
//
// It parses the capability out of OptionalParams rather than reading
// message.Open.ASN4, which is NEVER set on a received OPEN -- UnpackOpen does not
// populate it and no other code assigns it, so a `remote.ASN4 > 0` test is dead.
// The consequence of that dead branch was not cosmetic: every 4-byte-AS peer
// whose settings carry no AS fell through to My AS, which RFC 6793 requires such
// a speaker to send as AS_TRANS (23456). All of them then claimed their BGP
// Identifier in one shared 23456 bucket, so two peers in genuinely DIFFERENT
// 4-byte ASes that legitimately share an identifier collided and the second was
// refused with Bad BGP Identifier -- a rejection RFC 6286 Section 2.1 does not
// license, since its uniqueness requirement is scoped per-AS.
//
// Capability parse errors are swallowed deliberately: this runs before
// negotiation, where a malformed capability gets its own OPEN error with the
// right subcode (rejectOpenCapabilityError). Falling back to My AS here keeps
// that the single place a capability problem is reported.
func openAdvertisedAS(remote *message.Open) uint32 {
	caps, err := capability.ParseFromOptionalParams(remote.OptionalParams)
	if err == nil {
		for _, c := range caps {
			if as4, ok := c.(*capability.ASN4); ok && as4.ASN > 0 {
				return as4.ASN
			}
		}
	}
	return uint32(remote.MyAS)
}

// releaseRouterIDClaim drops the AS-wide BGP Identifier this peer holds (RFC 6286
// Section 2.1). Called on every session teardown so the identifier becomes available to
// another peer as soon as this one's session ends; a no-op when nothing is held.
func (p *Peer) releaseRouterIDClaim() {
	p.mu.RLock()
	r := p.reactor
	p.mu.RUnlock()

	if r == nil {
		return
	}
	r.routerIDs.release(p)
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
//
// Publishing PeerStateEstablished CLOSES the initial-sync gate first, and the
// order is the whole point: p.state is what every other goroutine reads, so the
// instant Established becomes visible the peer must already look busy to both
// gate readers. ShouldQueue would otherwise send a plugin's route DIRECT to the
// session, ahead of the End-of-RIB sendInitialRoutes has not started emitting
// (RFC 4724 Section 2), and PendingSync would tell the bgp-peer-sync quiescer
// (DrainPeerSync, reactor_api.go) that a peer whose initial sync has not begun
// is settled -- so `request quiesce` returns and the caller sends its next route
// into the same window.
//
// It lives HERE rather than at the one call site because the store used to sit
// 39 lines after the publication in peer_run.go's FSM callback, and every line
// between them (SetEstablishedNow, the GR EoR timer, resolveDynamicPeerSettings,
// a synchronous Info log, ResetAPISync, ResetPeerUpBarrier) held the window open
// -- wide enough that an oversubscribed CI host reordered test/plugin/mup4.ci's
// wire to announce, withdraw, EoR. Binding the store to the publication makes
// the window unreachable rather than short, and a future publication site
// inherits the guarantee instead of having to remember it.
//
// Store, not CompareAndSwap: a rapid reconnect can find a previous
// sendInitialRoutes still running (flag 2), and forcing 1 is what lets the new
// session's goroutine take the CAS(1,2) and run its own sync. That is the
// pre-existing contract of the store this replaces.
func (p *Peer) setState(s PeerState) {
	if s == PeerStateEstablished {
		p.sendingInitialRoutes.Store(1)
	}
	old := PeerState(p.state.Swap(int32(s)))
	if old != s {
		if old == PeerStateEstablished && s != PeerStateEstablished {
			p.incrConnectionsDropped()
		}
		p.updatePeerStateMetric(old, s)
		if p.health != nil {
			p.health.onStateChange(old, s)
		}
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

	// The next connection cycle is the operator's start, so runOnce fires
	// RFC 4271 Event 1 (ManualStart) for it and Event 6 for every cycle after.
	// Event 1's §8.2.2 clause zeroes the ConnectRetryCounter, which is what an
	// operator starting (or restarting) a peer means.
	p.operatorStarted.Store(false)

	// p.stopping is deliberately NOT cleared here. Clearing it is the act of
	// beginning a new engine generation, and that authority belongs to
	// Reactor.StartWithContext, which clears it for every peer under the same
	// r.mu that Reactor.stop sets it under (reactor.go). This method has no such
	// lock and cannot order itself against a stop: Reactor.StartPeers reaches it
	// through coord.OnPostStartup with r.mu released, so a SIGTERM landing
	// during plugin startup used to be answered by re-opening the outbound rail
	// on a peer the stop had already marked and notified.

	p.wg.Add(1)
	go p.run()
}

// ConnectRetryCounter returns RFC 4271 §8.1.1 mandatory session attribute 2:
// "the number of times a BGP peer has tried to establish a peer session".
//
// It counts across connection cycles, not within one: the FSM §8.2.2 handlers
// raise it on every teardown the RFC gives a "increments the
// ConnectRetryCounter" clause and zero it on an operator start or stop. Read
// by Stats (peer_stats.go), which carries it to `show bgp peer <ip> detail`
// and to the ze_bgp_connect_retry_counter gauge.
func (p *Peer) ConnectRetryCounter() uint32 {
	return p.connectRetryCounter.Load()
}

// ShutdownNotify tells this peer's live session that the local system is being
// taken out of service, with Cease / Administrative Shutdown (RFC 4486 subcode
// 2). It is the ManualStop action of RFC 4271 Section 8.2.2, which OpenSent,
// OpenConfirm and Established all list, and Teardown is state-blind for the
// same reason: what it needs is an open socket, not a particular FSM state.
//
// It does NOT go through Peer.teardown, and the difference is the point. That
// path QUEUES the teardown when no session is up or when the initial routes are
// still going out, so the peer receives it after the queue drains -- and on the
// way to exit nothing drains it, so the queued NOTIFICATION would simply never
// be sent. A peer with no session here has nothing to say goodbye on and is
// skipped. Nor does it set PeerStateConnecting: this peer is not reconnecting.
func (p *Peer) ShutdownNotify() {
	p.mu.Lock()
	session := p.session
	p.mu.Unlock()

	if session == nil {
		return
	}
	// Session.teardown returns nil on every path (session_connection.go): a
	// NOTIFICATION that cannot be written is reported where the write error
	// exists, by logNotifyErr (session.go, "notification send failed"). There
	// is nothing to branch on here, and a branch that cannot run reads as
	// coverage of a path nothing takes -- which is the defect this whole spec
	// was written about.
	_ = session.Teardown(message.NotifyCeaseAdminShutdown, "")
}

// sealSession is the peer's half of Reactor.stop's seal: it carries the seal to
// the one session that can still publish a connection.
//
// Marking p.stopping stops a NEW session being published (runOnce reads it inside
// the p.mu hold that publishes p.session), which leaves exactly one session per
// peer -- the one already published when the stop landed. That one is reached by
// the accept rail through AcceptConnection, so the seal has to land ON it, and
// Session.connectionEstablished is where it is read (session_connection.go).
//
// It reads p.session under the same lock ShutdownNotify does, and after the same
// mark, so the pairing is the one runOnce is written against: either the publish
// got in first and is sealed here, or it never happens.
//
// This is not ShutdownNotify with the message removed. ShutdownNotify runs on
// the notify path only, so without this a StopForRestart would seal no session
// at all, and it also spends up to shutdownNotifyBudget on the wire -- the seal
// must be in place before any of that starts.
func (p *Peer) sealSession() {
	p.mu.Lock()
	session := p.session
	p.mu.Unlock()

	if session != nil {
		session.seal()
	}
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
	return p.teardown(subcode, shutdownMsg, false)
}

// TeardownAutomatic is Teardown for a stop the LOCAL SYSTEM chose rather than
// the operator, so it raises RFC 4271 Event 8 (AutomaticStop) instead of Event
// 2 (ManualStop). §8.2.2 has Event 8 increment the ConnectRetryCounter where
// Event 2 zeroes it, and a peer torn down because BFD went down or the forward
// pool ran out of room has just failed an attempt, not been told to forget the
// ones before it. Everything else matches Teardown, queuing included.
func (p *Peer) TeardownAutomatic(subcode uint8, shutdownMsg string) error {
	return p.teardown(subcode, shutdownMsg, true)
}

func (p *Peer) teardown(subcode uint8, shutdownMsg string, automatic bool) error {
	p.mu.Lock()
	session := p.session

	// If sendInitialRoutes is pending (1) or running (2), queue the teardown
	// so it can send EOR before executing the teardown. This ensures proper
	// BGP protocol sequencing: routes + EOR + NOTIFICATION.
	if p.sendingInitialRoutes.Load() != 0 {
		if len(p.opQueue) < p.opQueueMax {
			p.opQueue = append(p.opQueue, PeerOp{Type: PeerOpTeardown, Subcode: subcode, Message: shutdownMsg, Automatic: automatic})
			p.mu.Unlock()
			return nil
		}
		p.mu.Unlock()
		routesLogger().Warn("opQueue full, dropping teardown", "peer", p.settings.Address)
		return ErrOpQueueFull
	}

	if session != nil {
		p.mu.Unlock()
		if err := sessionTeardown(session, subcode, shutdownMsg, automatic); err != nil {
			peerLogger().Debug("teardown error", "peer", p.settings.Address, "error", err)
		}
		// Set state after teardown - there's a brief race window where
		// AnnounceRoute might see ESTABLISHED, but SendUpdate will fail
		// on the closed session (which is correct behavior)
		p.setState(PeerStateConnecting)
		return nil
	}

	// No active session - queue teardown to maintain operation order
	if len(p.opQueue) < p.opQueueMax {
		p.opQueue = append(p.opQueue, PeerOp{Type: PeerOpTeardown, Subcode: subcode, Message: shutdownMsg, Automatic: automatic})
		p.mu.Unlock()
		return nil
	}
	p.mu.Unlock()
	routesLogger().Warn("opQueue full, dropping teardown", "peer", p.settings.Address)
	return ErrOpQueueFull
}

// ClaimInitialSyncEOR records that an End-of-RIB for fam is about to go on the
// wire for THIS session, and reports whether the caller is the one that may send
// it. It returns false when the marker has already been sent, so the second
// producer stands down instead of duplicating it (RFC 4724 Section 2: one
// End-of-RIB per family per session).
//
// Claim-then-send, not send-then-mark: the two producers run on different
// goroutines (sendInitialRoutes, and the route server's replay goroutine via
// AnnounceEOR), so the check and the record must be one atomic step or both can
// pass the check before either marks.
//
// A caller whose send then FAILS should release the claim with
// ReleaseInitialSyncEOR, otherwise the family is left marked and the peer never
// receives the marker at all.
func (p *Peer) ClaimInitialSyncEOR(fam family.Family) bool {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.initialSyncEOR == nil {
		p.initialSyncEOR = make(map[family.Family]bool, 2)
	}
	if p.initialSyncEOR[fam] {
		return false
	}
	p.initialSyncEOR[fam] = true
	return true
}

// ReleaseInitialSyncEOR undoes a claim whose send failed, so the other producer
// may still deliver the marker. Pairs with ClaimInitialSyncEOR.
func (p *Peer) ReleaseInitialSyncEOR(fam family.Family) {
	p.mu.Lock()
	defer p.mu.Unlock()
	delete(p.initialSyncEOR, fam)
}

// resetInitialSyncEOR clears every claim, so the next session sends End-of-RIB
// again. Called from the session teardown defer alongside sendingInitialRoutes.
func (p *Peer) resetInitialSyncEOR() {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.initialSyncEOR = nil
}

// ShouldQueue returns true if routes should be queued rather than sent directly.
// Routes must be queued when:
//   - Session is not established
//   - The initial sync is pending or running (sendingInitialRoutes non-zero)
//   - There are pending queued operations (preserves insertion order)
//
// This prevents a race where routes sent directly during sendInitialRoutes
// processing arrive at the peer before older queued routes.
//
// The second condition covers the whole of establishment, not just the running
// goroutine: setState closes the gate before it publishes PeerStateEstablished,
// so an observer that sees Established already sees a non-zero flag. Reading the
// state first and the flag second is therefore safe in either order.
func (p *Peer) ShouldQueue() bool {
	if p.State() != PeerStateEstablished {
		return true
	}
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.sendingInitialRoutes.Load() != 0 || len(p.opQueue) > 0
}

// initialSyncInProgress reports whether sendInitialRoutes is currently running for
// this peer. It distinguishes the three conditions ShouldQueue folds together, so
// a caller that suppresses work can say WHICH one applied (logEORSuppressed,
// reactor_api_forward.go). Reads the atomic directly: sendingInitialRoutes is set
// and cleared without p.mu, and ShouldQueue reads it the same way.
func (p *Peer) initialSyncInProgress() bool {
	return p.sendingInitialRoutes.Load() != 0
}

// forwardOrderHold reports whether a forwarded UPDATE for this peer must be
// parked behind the peer's own route operations instead of going to the wire.
// ONE predicate for both halves of that decision: the dispatch gates on the two
// forwarding rails (reactor_api_forward.go, forward_rs.go) and the hold in
// drainOverflow (forward_pool.go, overflowHeld). A gate wider than its hold
// parks items nothing will release, and a hold wider than its gate releases
// items the gate never parked.
//
// It is ShouldQueue() narrowed twice, and each narrowing is load-bearing.
//
// Not-Established is excluded: that peer will never drain an opQueue, so holding
// for it parks the items until its REPLACEMENT session has finished its own
// sync, and then delivers a dead session's UPDATE after a full RIB dump -- the
// duplicate delivery test/plugin/role-otc-rs-withdraw-eor.ci refuses. Not
// gating instead lets fwdBatchHandler discard them, which is what the forwarding
// rail already does for a destination that is not Established.
//
// A non-empty opQueue is excluded for the same reason by a different route.
// setState stores the sync flag in the same call that publishes Established and
// BEFORE it, so from establishment until sendInitialRoutes clears the flag,
// every queued operation is covered by the flag alone. An opQueue that is still
// non-empty with the flag CLEAR is one nothing will drain -- sendInitialRoutes
// is the only drainer and it has returned -- so ordering a forwarded UPDATE
// behind it would park that UPDATE for the life of the session.
func (p *Peer) forwardOrderHold() bool {
	return p.State() == PeerStateEstablished && p.initialSyncInProgress()
}

// forwardOverflowPending reports whether the forward pool still owes this peer
// bytes it took through overflow: items queued in its worker's overflow buffer,
// snapshotted by an in-flight drain, sitting in that worker's channel, or in the
// batch being written right now.
//
// TryDispatch reads the same count inside the pool and refuses a channel send
// while it is nonzero, so the pool's own rails preserve FIFO without asking. The
// route-server fast path does not go through the pool at all --
// tryDirectWriteNoFlush writes into the destination's bufWriter under the same
// session.writeMu the worker has not taken yet -- so it asks here instead
// (forward_rs.go). Reaching the wire first is exactly the inversion the ordering
// hold exists to prevent, which is why the count covers the in-flight items and
// not only the queued ones (forward_pool.go, fwdWorker.overflowPending).
//
// Two atomic loads, no lock and no map hash: the destination peer is already in
// hand at the call site, and this runs per destination per UPDATE.
//
// A nil handle means no real item was ever parked for this peer, so this peer is
// owed nothing through overflow. A barrier sentinel can raise the count on its
// own (forward_pool_barrier.go) without naming a peer, and it carries no bytes:
// a destination owed nothing but sentinels is owed nothing.
func (p *Peer) forwardOverflowPending() bool {
	c := p.fwdOverflowPending.Load()
	return c != nil && c.Load() > 0
}

// wakeForwardOverflow releases the forwarded UPDATEs parked behind this peer's
// initial route sync. Call it after every store that clears
// sendingInitialRoutes.
//
// The forwarding rails park an UPDATE for a peer that forwardOrderHold()s in the
// destination worker's overflow buffer, and drainOverflow keeps it there while
// that stays true (forward_pool.go, overflowHeld). The worker re-evaluates the
// hold only after something reaches its channel, and a held worker's channel is
// empty by construction, so the store alone would leave the items parked until
// the next forward to this peer.
//
// p.reactor is read WITHOUT p.mu, like every other reader of it (peer_run.go).
// One of the call sites is the recover defer in sendInitialRoutes, and the drain
// loop it guards holds p.mu.Lock across buildRIBRouteUpdate: a panic there
// reaches this function write-locked, and an RLock would deadlock the peer's
// goroutine for good.
func (p *Peer) wakeForwardOverflow() {
	r := p.reactor
	if r == nil || r.fwdPool == nil {
		return
	}
	// p.settings.PeerKey() is the key both forwarding rails dispatch under
	// (peerForwardFacts.peerKey, peer_forward_facts.go). Read without p.mu like
	// buildForwardFacts reads the same fields: address and port are set at
	// construction (the contract on Peer.Settings).
	r.fwdPool.wakeOverflow(fwdKey{peerAddr: p.settings.PeerKey()})
}

// PendingSync reports whether the peer still has route work that has not reached
// the wire: routes queued while not-yet-established, or an in-flight initial-route
// sync. Unlike ShouldQueue it does NOT gate on state -- a not-yet-established peer
// with queued routes IS pending (those routes drain when it establishes), while a
// down/idle peer with an empty queue is not. Used by the DrainPeerSync barrier so
// a test can wait for send()-during-establishment routes to reach the wire.
func (p *Peer) PendingSync() bool {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.sendingInitialRoutes.Load() != 0 || len(p.opQueue) > 0
}

// QueueAnnounce queues a route announcement for when session establishes.
// Used when session is not established to maintain operation order.
// If queue is full, the operation is dropped with a warning.
func (p *Peer) QueueAnnounce(route *rib.Route) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if len(p.opQueue) >= p.opQueueMax {
		routesLogger().Warn("opQueue full, dropping announce", "peer", p.settings.Address, "queueSize", len(p.opQueue), "nlri", route.NLRI())
		return
	}
	p.opQueue = append(p.opQueue, PeerOp{Type: PeerOpAnnounce, Route: route})
}

// QueueWithdraw queues a route withdrawal for when session establishes.
// Used when session is not established to maintain operation order.
// If queue is full, the operation is dropped with a warning.
func (p *Peer) QueueWithdraw(n nlri.NLRI) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if len(p.opQueue) >= p.opQueueMax {
		routesLogger().Warn("opQueue full, dropping withdraw", "peer", p.settings.Address, "queueSize", len(p.opQueue), "nlri", n)
		return
	}
	p.opQueue = append(p.opQueue, PeerOp{Type: PeerOpWithdraw, NLRI: n})
}

// Wait waits for the peer to stop.
func (p *Peer) Wait(ctx context.Context) error {
	return syncutil.WaitGroupWait(ctx, &p.wg)
}
