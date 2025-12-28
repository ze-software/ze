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

	"github.com/exa-networks/zebgp/pkg/bgp/attribute"
	"github.com/exa-networks/zebgp/pkg/bgp/capability"
	"github.com/exa-networks/zebgp/pkg/bgp/fsm"
	"github.com/exa-networks/zebgp/pkg/bgp/message"
	"github.com/exa-networks/zebgp/pkg/bgp/nlri"
	"github.com/exa-networks/zebgp/pkg/rib"
	"github.com/exa-networks/zebgp/pkg/trace"
)

// safiMUP is the SAFI for Mobile User Plane (draft-mpmz-bess-mup-safi).
// Not in capability package as it's not yet standardized (SAFI 85).
const safiMUP = 85

// NegotiatedFamilies contains pre-computed flags from capability negotiation.
// Computed once when session is established, provides O(1) family checks.
// This avoids repeated iteration through capability lists when sending routes.
type NegotiatedFamilies struct {
	// Unicast (RFC 4760)
	IPv4Unicast bool
	IPv6Unicast bool

	// FlowSpec (RFC 8955)
	IPv4FlowSpec    bool
	IPv6FlowSpec    bool
	IPv4FlowSpecVPN bool
	IPv6FlowSpecVPN bool

	// L2VPN VPLS (RFC 4761)
	L2VPNVPLS bool

	// MVPN (RFC 6514)
	IPv4McastVPN bool
	IPv6McastVPN bool

	// MUP (draft-mpmz-bess-mup-safi, SAFI 85)
	IPv4MUP bool
	IPv6MUP bool

	// Encoding options
	ASN4            bool
	ExtendedMessage bool

	// RFC 8950: Extended Next Hop - allows IPv4 NLRI with IPv6 next-hop
	IPv4UnicastExtNH bool // IPv4 unicast can use IPv6 next-hop
	IPv4MPLSVPNExtNH bool // IPv4 mpls-vpn can use IPv6 next-hop
}

// computeNegotiatedFamilies extracts family flags from capability negotiation.
// Called once when session transitions to Established state.
func computeNegotiatedFamilies(neg *capability.Negotiated) *NegotiatedFamilies {
	if neg == nil {
		return nil
	}

	nf := &NegotiatedFamilies{
		ASN4:            neg.ASN4,
		ExtendedMessage: neg.ExtendedMessage,
	}

	for _, f := range neg.Families() {
		afi, safi := f.AFI, f.SAFI
		switch {
		// Unicast
		case afi == capability.AFIIPv4 && safi == capability.SAFIUnicast:
			nf.IPv4Unicast = true
		case afi == capability.AFIIPv6 && safi == capability.SAFIUnicast:
			nf.IPv6Unicast = true

		// FlowSpec
		case afi == capability.AFIIPv4 && safi == capability.SAFIFlowSpec:
			nf.IPv4FlowSpec = true
		case afi == capability.AFIIPv6 && safi == capability.SAFIFlowSpec:
			nf.IPv6FlowSpec = true
		case afi == capability.AFIIPv4 && safi == capability.SAFIFlowSpecVPN:
			nf.IPv4FlowSpecVPN = true
		case afi == capability.AFIIPv6 && safi == capability.SAFIFlowSpecVPN:
			nf.IPv6FlowSpecVPN = true

		// L2VPN VPLS
		case afi == capability.AFIL2VPN && safi == capability.SAFIVPLS:
			nf.L2VPNVPLS = true

		// MVPN
		case afi == capability.AFIIPv4 && safi == capability.SAFIMcastVPN:
			nf.IPv4McastVPN = true
		case afi == capability.AFIIPv6 && safi == capability.SAFIMcastVPN:
			nf.IPv6McastVPN = true

		// MUP
		case afi == capability.AFIIPv4 && safi == safiMUP:
			nf.IPv4MUP = true
		case afi == capability.AFIIPv6 && safi == safiMUP:
			nf.IPv6MUP = true
		}
	}

	// RFC 8950: Check extended next-hop for IPv4 families
	// If negotiated with IPv6 next-hop AFI, we can use MP_REACH_NLRI for IPv4 NLRI
	if neg.ExtendedNextHopAFI(capability.Family{AFI: capability.AFIIPv4, SAFI: capability.SAFIUnicast}) == capability.AFIIPv6 {
		nf.IPv4UnicastExtNH = true
	}
	if neg.ExtendedNextHopAFI(capability.Family{AFI: capability.AFIIPv4, SAFI: capability.SAFIMPLS}) == capability.AFIIPv6 {
		nf.IPv4MPLSVPNExtNH = true
	}

	return nf
}

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
// The peer uses two complementary queuing mechanisms:
//
//  1. adjRIBOut (Adj-RIB-Out): Manages the "sent cache" - routes that have been
//     successfully announced and should be re-sent on session re-establishment.
//     Also handles transaction-based batching via Begin/Commit.
//
//  2. opQueue: Ordered operation queue for when session is not established.
//     Maintains strict ordering of announce/withdraw/teardown operations.
//     Processed on session establishment, with teardowns acting as batch separators.
//
// When a route is announced:
//   - Session ESTABLISHED + no transaction → sent immediately, added to sent cache
//   - Session ESTABLISHED + in transaction → queued to adjRIBOut transaction queue
//   - Session NOT ESTABLISHED → queued to opQueue
//
// On session establishment:
//  1. Routes from sent cache are re-sent (previously announced routes)
//  2. opQueue is processed in order until a teardown is encountered
//  3. Teardown sends EOR + NOTIFICATION, remaining opQueue items persist
type Peer struct {
	settings *PeerSettings
	session  *Session

	// Pre-computed negotiated families for O(1) access.
	// Set when session transitions to Established, cleared on teardown.
	families atomic.Pointer[NegotiatedFamilies]

	state          atomic.Int32
	callback       PeerCallback
	updateCallback UpdateCallback // Called when UPDATE is received

	// Reconnect configuration
	reconnectMin time.Duration
	reconnectMax time.Duration

	// Adj-RIB-Out: Maintains the "sent cache" of routes that should persist
	// across session re-establishments. Also handles transaction batching.
	// Routes added here are re-sent automatically on reconnect.
	adjRIBOut *rib.OutgoingRIB

	// Ordered operation queue: Used when session is NOT established.
	// Maintains strict ordering of announce/withdraw/teardown operations.
	// Processed on session establishment; teardowns act as batch separators.
	opQueue []PeerOp

	// sendingInitialRoutes prevents concurrent sendInitialRoutes goroutines.
	// Set to 1 when sendInitialRoutes starts, 0 when it ends.
	sendingInitialRoutes atomic.Int32

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
		adjRIBOut:     rib.NewOutgoingRIB(),
		opQueue:       make([]PeerOp, 0, 16), // Pre-allocate small capacity
		watchdogState: make(map[string]map[string]bool),
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

// SetGlobalWatchdog sets the global watchdog manager for this peer.
// Called by reactor when peer is added.
func (p *Peer) SetGlobalWatchdog(wm *WatchdogManager) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.globalWatchdog = wm
}

// AdjRIBOut returns the peer's Adj-RIB-Out.
func (p *Peer) AdjRIBOut() *rib.OutgoingRIB {
	return p.adjRIBOut
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
// If called when not connected, queues the teardown to maintain operation order.
func (p *Peer) Teardown(subcode uint8) {
	p.mu.Lock()
	session := p.session
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
// Also removes from sent cache immediately to prevent re-announcement on reconnect.
// If queue is full (MaxOpQueueSize), the operation is dropped with a warning,
// but the route is still removed from sent cache.
func (p *Peer) QueueWithdraw(n nlri.NLRI) {
	p.mu.Lock()
	defer p.mu.Unlock()
	// Always remove from sent cache, even if queue is full
	p.adjRIBOut.RemoveFromSent(n)
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
	session.onUpdateReceived = p.updateCallback

	p.mu.Lock()
	p.session = session
	p.mu.Unlock()

	defer func() {
		p.families.Store(nil) // Clear pre-computed families
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
			// Pre-compute negotiated families for O(1) access during route sending
			p.families.Store(computeNegotiatedFamilies(session.Negotiated()))
			p.setState(PeerStateEstablished)
			trace.SessionEstablished(addr, p.settings.LocalAS, p.settings.PeerAS)
			// Send static routes from config.
			go p.sendInitialRoutes()
		} else if from == fsm.StateEstablished {
			// Clear negotiated families on session teardown
			p.families.Store(nil)
			p.setState(PeerStateConnecting)
			trace.SessionClosed(addr, "FSM left Established state")
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

	return &message.Negotiated{
		ASN4:    neg.ASN4,
		LocalAS: neg.LocalASN,
		PeerAS:  neg.PeerASN,
	}
}

// cleanup runs when peer stops.
func (p *Peer) cleanup() {
	p.families.Store(nil) // Clear pre-computed families
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
	// Prevent concurrent sendInitialRoutes goroutines.
	// If another instance is running, skip this one - the running instance
	// will process any queued operations.
	if !p.sendingInitialRoutes.CompareAndSwap(0, 1) {
		return
	}
	defer p.sendingInitialRoutes.Store(0)

	// Get pre-computed negotiated families for ASN4 and family checks.
	nf := p.families.Load()
	if nf == nil {
		return
	}

	addr := p.settings.Address.String()
	trace.Log(trace.Routes, "peer %s: sending %d static routes", addr, len(p.settings.StaticRoutes))

	// Send routes - either grouped or individually based on config.
	if p.settings.GroupUpdates {
		// Group routes by attributes (same attributes = same UPDATE).
		groups := groupRoutesByAttributes(p.settings.StaticRoutes)

		for _, routes := range groups {
			update := buildGroupedUpdate(routes, p.settings.LocalAS, p.settings.IsIBGP(), nf.ASN4)
			if err := p.SendUpdate(update); err != nil {
				trace.Log(trace.Routes, "peer %s: send error: %v", addr, err)
				break
			}
			for _, route := range routes {
				trace.RouteSent(addr, route.Prefix.String(), route.NextHop.String())
			}
		}
	} else {
		// Send each route in its own UPDATE.
		for _, route := range p.settings.StaticRoutes {
			update := buildStaticRouteUpdate(route, p.settings.LocalAS, p.settings.IsIBGP(), nf.ASN4, nf)
			if err := p.SendUpdate(update); err != nil {
				trace.Log(trace.Routes, "peer %s: send error: %v", addr, err)
				break
			}
			trace.RouteSent(addr, route.Prefix.String(), route.NextHop.String())
		}
	}

	// Handle watchdog routes (routes controlled via "announce/withdraw watchdog" API).
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

				// Send the route
				update := buildStaticRouteUpdate(wr.StaticRoute, p.settings.LocalAS, p.settings.IsIBGP(), nf.ASN4, nf)
				if err := p.SendUpdate(update); err != nil {
					trace.Log(trace.Routes, "peer %s: send error: %v", addr, err)
					break
				}
				trace.RouteSent(addr, routeKey, wr.NextHop.String())
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
		localAddr := p.settings.LocalAddress
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
				// Resolve next-hop self if needed
				nextHop := pr.NextHop
				if pr.NextHopSelf && localAddr.IsValid() {
					nextHop = localAddr
				}
				// Build route with resolved next-hop
				route := pr.StaticRoute
				route.NextHop = nextHop
				update := buildStaticRouteUpdate(route, p.settings.LocalAS, p.settings.IsIBGP(), nf.ASN4, nf)
				if err := p.SendUpdate(update); err != nil {
					trace.Log(trace.Routes, "peer %s: send error: %v", addr, err)
					break
				}
				trace.Log(trace.Routes, "peer %s: re-sent global pool route %s from pool %s", addr, pr.RouteKey(), poolName)
			}
		}
	}

	// Re-send Adj-RIB-Out routes (routes announced via API that persist across reconnects).
	sentRoutes := p.adjRIBOut.GetSentRoutes()
	if len(sentRoutes) > 0 {
		trace.Log(trace.Routes, "peer %s: re-sending %d Adj-RIB-Out routes", addr, len(sentRoutes))
		for _, route := range sentRoutes {
			update := buildRIBRouteUpdate(route, p.settings.LocalAS, p.settings.IsIBGP(), nf.ASN4)
			if err := p.SendUpdate(update); err != nil {
				trace.Log(trace.Routes, "peer %s: send error: %v", addr, err)
				break
			}
		}
	}

	// Send pending routes from transactions (committed while not connected).
	pendingRoutes := p.adjRIBOut.FlushAllPending()
	if len(pendingRoutes) > 0 {
		trace.Log(trace.Routes, "peer %s: sending %d pending routes from transactions", addr, len(pendingRoutes))
		for _, route := range pendingRoutes {
			update := buildRIBRouteUpdate(route, p.settings.LocalAS, p.settings.IsIBGP(), nf.ASN4)
			if err := p.SendUpdate(update); err != nil {
				trace.Log(trace.Routes, "peer %s: send error: %v", addr, err)
				break
			}
		}
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
			// Send route and add to sent cache
			update := buildRIBRouteUpdate(op.Route, p.settings.LocalAS, p.settings.IsIBGP(), nf.ASN4)
			p.mu.Unlock()
			if err := p.SendUpdate(update); err != nil {
				trace.Log(trace.Routes, "peer %s: send error: %v", addr, err)
				p.mu.Lock()
				break
			}
			p.adjRIBOut.MarkSent(op.Route)
			p.mu.Lock()
			processed = i + 1
			continue

		case PeerOpWithdraw:
			// Send withdrawal (already removed from sent cache when queued)
			update := buildWithdrawNLRI(op.NLRI)
			p.mu.Unlock()
			if err := p.SendUpdate(update); err != nil {
				trace.Log(trace.Routes, "peer %s: send error: %v", addr, err)
				p.mu.Lock()
				break
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

	// Send End-of-RIB marker for each negotiated address family.
	if nf.IPv4Unicast {
		_ = p.SendUpdate(message.BuildEOR(nlri.IPv4Unicast))
		trace.Log(trace.Routes, "peer %s: sent IPv4 unicast EOR marker", addr)
	}
	if nf.IPv6Unicast {
		_ = p.SendUpdate(message.BuildEOR(nlri.IPv6Unicast))
		trace.Log(trace.Routes, "peer %s: sent IPv6 unicast EOR marker", addr)
	}

	// If teardown was in queue, execute it now (after EOR)
	if hasTeardown {
		trace.Log(trace.Routes, "peer %s: executing queued teardown (subcode=%d)", addr, teardownSubcode)
		p.mu.RLock()
		session := p.session
		p.mu.RUnlock()
		if session != nil {
			if err := session.Teardown(teardownSubcode); err != nil {
				trace.Log(trace.Routes, "peer %s: teardown error: %v", addr, err)
			}
			// Immediately mark as not established to prevent race conditions
			// where subsequent API commands see stale ESTABLISHED state
			p.setState(PeerStateConnecting)
		}
		return // Don't send other routes after teardown
	}

	// Send MVPN routes
	p.sendMVPNRoutes()

	// Send VPLS routes
	p.sendVPLSRoutes()

	// Send FlowSpec routes
	p.sendFlowSpecRoutes()

	// Send MUP routes
	p.sendMUPRoutes()
}

// buildRIBRouteUpdate builds an UPDATE message from a RIB route.
// Used for re-announcing routes from Adj-RIB-Out on session re-establishment.
// Rebuilds the full set of required attributes since rib.Route may not store all.
func buildRIBRouteUpdate(route *rib.Route, localAS uint32, isIBGP, asn4 bool) *message.Update {
	var attrBytes []byte

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

		// 4. LOCAL_PREF for iBGP
		if isIBGP {
			attrBytes = append(attrBytes, attribute.PackAttribute(attribute.LocalPref(100))...)
		}

		// IPv4 unicast: use inline NLRI field
		nlriBytes = routeNLRI.Bytes()
	default:
		// Other families: use MP_REACH_NLRI attribute
		// This includes IPv6, VPN, etc.
		mpReach := &attribute.MPReachNLRI{
			AFI:      attribute.AFI(family.AFI),
			SAFI:     attribute.SAFI(family.SAFI),
			NextHops: []netip.Addr{route.NextHop()},
			NLRI:     routeNLRI.Bytes(),
		}
		attrBytes = append(attrBytes, attribute.PackAttribute(mpReach)...)

		// LOCAL_PREF for iBGP
		if isIBGP {
			attrBytes = append(attrBytes, attribute.PackAttribute(attribute.LocalPref(100))...)
		}
	}

	// Copy optional attributes from stored route (MED, communities, etc.)
	for _, attr := range route.Attributes() {
		switch attr.(type) {
		case attribute.Origin, *attribute.ASPath, *attribute.NextHop, attribute.LocalPref:
			// Already handled above
			continue
		case attribute.MED, attribute.Communities,
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

// buildWithdrawNLRI builds an UPDATE message to withdraw an NLRI.
// RFC 4760: IPv4 unicast uses WithdrawnRoutes, others use MP_UNREACH_NLRI.
func buildWithdrawNLRI(n nlri.NLRI) *message.Update {
	family := n.Family()

	if family.AFI == nlri.AFIIPv4 && family.SAFI == nlri.SAFIUnicast {
		// IPv4 unicast: use WithdrawnRoutes field
		return &message.Update{
			WithdrawnRoutes: n.Bytes(),
		}
	}

	// Other families: use MP_UNREACH_NLRI attribute
	mpUnreach := &attribute.MPUnreachNLRI{
		AFI:  attribute.AFI(family.AFI),
		SAFI: attribute.SAFI(family.SAFI),
		NLRI: n.Bytes(),
	}

	return &message.Update{
		PathAttributes: attribute.PackAttribute(mpUnreach),
	}
}

// buildStaticRouteWithdraw builds a withdrawal UPDATE for a static route.
// Handles VPN, IPv4 unicast, and IPv6 unicast correctly.
func buildStaticRouteWithdraw(route StaticRoute) *message.Update {
	switch {
	case route.IsVPN():
		// VPN route: use MP_UNREACH_NLRI with RD + prefix
		return buildMPUnreachVPN(route)
	case route.Prefix.Addr().Is4():
		// IPv4 unicast: use WithdrawnRoutes field
		inet := nlri.NewINET(nlri.Family{AFI: nlri.AFIIPv4, SAFI: nlri.SAFIUnicast}, route.Prefix, route.PathID)
		return &message.Update{
			WithdrawnRoutes: inet.Bytes(),
		}
	default:
		// IPv6 unicast: use MP_UNREACH_NLRI
		inet := nlri.NewINET(nlri.Family{AFI: nlri.AFIIPv6, SAFI: nlri.SAFIUnicast}, route.Prefix, route.PathID)
		mpUnreach := &attribute.MPUnreachNLRI{
			AFI:  attribute.AFI(nlri.AFIIPv6),
			SAFI: attribute.SAFI(nlri.SAFIUnicast),
			NLRI: inet.Bytes(),
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

	// Label: use route.Label or withdraw label (0x800000)
	label := route.Label
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

// buildStaticRouteUpdate builds an UPDATE message for a static route.
// asn4 indicates whether to use 4-byte AS numbers in AS_PATH.
// nf contains negotiated family flags including ExtNH support.
func buildStaticRouteUpdate(route StaticRoute, localAS uint32, isIBGP, asn4 bool, nf *NegotiatedFamilies) *message.Update {
	var attrBytes []byte

	// 1. ORIGIN (IGP by default, use configured value if set)
	origin := attribute.Origin(route.Origin)
	attrBytes = append(attrBytes, attribute.PackAttribute(origin)...)

	// 2. AS_PATH
	// - For iBGP: use configured AS_PATH or empty
	// - For eBGP: prepend local AS to configured AS_PATH
	var asPath *attribute.ASPath
	switch {
	case len(route.ASPath) > 0:
		// Use configured AS_PATH, prepend local AS for eBGP
		asns := make([]uint32, 0, len(route.ASPath)+1)
		if !isIBGP {
			asns = append(asns, localAS)
		}
		asns = append(asns, route.ASPath...)
		asPath = &attribute.ASPath{
			Segments: []attribute.ASPathSegment{
				{Type: attribute.ASSequence, ASNs: asns},
			},
		}
	case isIBGP:
		// Empty AS_PATH for iBGP self-originated routes
		asPath = &attribute.ASPath{Segments: nil}
	default:
		// Prepend local AS for eBGP
		asPath = &attribute.ASPath{
			Segments: []attribute.ASPathSegment{
				{Type: attribute.ASSequence, ASNs: []uint32{localAS}},
			},
		}
	}
	attrBytes = append(attrBytes, attribute.PackASPathAttribute(asPath, asn4)...)

	// 3. NEXT_HOP (for IPv4 routes with IPv4 next-hop)
	// RFC 8950: When using extended next-hop, next-hop goes in MP_REACH_NLRI, not here.
	// Check for IPv4 unicast or VPN with IPv6 next-hop.
	useExtNH := route.Prefix.Addr().Is4() && route.NextHop.Is6() && nf != nil && nf.IPv4UnicastExtNH
	useExtNHVPN := route.IsVPN() && route.Prefix.Addr().Is4() && route.NextHop.Is6() && nf != nil && nf.IPv4MPLSVPNExtNH
	if route.NextHop.Is4() && !route.IsVPN() {
		nextHop := &attribute.NextHop{Addr: route.NextHop}
		attrBytes = append(attrBytes, attribute.PackAttribute(nextHop)...)
	}
	// VPN routes: next-hop goes in MP_REACH_NLRI (handled in buildMPReachNLRI)

	// 4. MED (if set).
	if route.MED > 0 {
		med := attribute.MED(route.MED)
		attrBytes = append(attrBytes, attribute.PackAttribute(med)...)
	}

	// 5. LOCAL_PREF (for iBGP, default 100 if not set)
	if isIBGP {
		localPref := route.LocalPreference
		if localPref == 0 {
			localPref = 100 // Default LOCAL_PREF
		}
		attrBytes = append(attrBytes, attribute.PackAttribute(attribute.LocalPref(localPref))...)
	}

	// 6. ATOMIC_AGGREGATE (if set)
	if route.AtomicAggregate {
		attrBytes = append(attrBytes, attribute.PackAttribute(attribute.AtomicAggregate{})...)
	}

	// 7. AGGREGATOR (if set)
	if route.HasAggregator {
		agg := &attribute.Aggregator{
			ASN:     route.AggregatorASN,
			Address: netip.AddrFrom4(route.AggregatorIP),
		}
		attrBytes = append(attrBytes, attribute.PackAttribute(agg)...)
	}

	// 8. COMMUNITIES (RFC 1997) - sorted per RFC.
	if len(route.Communities) > 0 {
		sorted := make([]uint32, len(route.Communities))
		copy(sorted, route.Communities)
		sort.Slice(sorted, func(i, j int) bool { return sorted[i] < sorted[j] })

		comms := make(attribute.Communities, len(sorted))
		for i, c := range sorted {
			comms[i] = attribute.Community(c)
		}
		attrBytes = append(attrBytes, attribute.PackAttribute(comms)...)
	}

	// 7. EXTENDED_COMMUNITY (if set)
	if len(route.ExtCommunityBytes) > 0 {
		// Pack as attribute: flags=0xC0 (optional, transitive), type=16, len, value
		ecAttr := make([]byte, 0, 3+len(route.ExtCommunityBytes))
		ecAttr = append(ecAttr, 0xC0, 0x10, byte(len(route.ExtCommunityBytes)))
		ecAttr = append(ecAttr, route.ExtCommunityBytes...)
		attrBytes = append(attrBytes, ecAttr...)
	}

	// 8. LARGE_COMMUNITIES (RFC 8092) - if set
	if len(route.LargeCommunities) > 0 {
		lcs := make(attribute.LargeCommunities, len(route.LargeCommunities))
		for i, lc := range route.LargeCommunities {
			lcs[i] = attribute.LargeCommunity{
				GlobalAdmin: lc[0],
				LocalData1:  lc[1],
				LocalData2:  lc[2],
			}
		}
		attrBytes = append(attrBytes, attribute.PackAttribute(lcs)...)
	}

	// 9. RAW ATTRIBUTES - pass through as-is from config
	for _, ra := range route.RawAttributes {
		attrBytes = append(attrBytes, packRawAttribute(ra)...)
	}

	// Build NLRI - use MP_REACH_NLRI for VPN/IPv6, inline NLRI for IPv4 unicast
	// RFC 8950: Use MP_REACH_NLRI for IPv4 with IPv6 next-hop when extended NH negotiated
	var nlriBytes []byte
	switch {
	case route.IsVPN():
		// VPN route: use MP_REACH_NLRI attribute (returns raw bytes)
		// RFC 8950: IPv6 next-hop requires extended next-hop negotiation
		if route.Prefix.Addr().Is4() && route.NextHop.Is6() && !useExtNHVPN {
			// Extended next-hop not negotiated for VPN - peer may reject
			trace.Log(trace.Routes, "VPN route %s with IPv6 next-hop but extended-nexthop not negotiated",
				route.Prefix)
		}
		attrBytes = append(attrBytes, buildMPReachNLRI(route)...)
	case useExtNH:
		// RFC 8950: IPv4 unicast with IPv6 next-hop via MP_REACH_NLRI
		attrBytes = append(attrBytes, buildMPReachNLRIExtNHUnicast(route)...)
	case route.Prefix.Addr().Is4():
		// IPv4 unicast: inline NLRI with optional path-id
		inet := nlri.NewINET(nlri.Family{AFI: nlri.AFIIPv4, SAFI: nlri.SAFIUnicast}, route.Prefix, route.PathID)
		nlriBytes = inet.Bytes()
	default:
		// IPv6 unicast: use MP_REACH_NLRI attribute (RFC 4760)
		attrBytes = append(attrBytes, buildMPReachNLRIUnicast(route)...)
	}

	return &message.Update{
		PathAttributes: attrBytes,
		NLRI:           nlriBytes,
	}
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
	// as-path, atomic-aggregate, aggregator.
	// For IPv6 routes, include prefix in key to prevent grouping (each needs separate MP_REACH_NLRI UPDATE).
	// IPv4 routes can be grouped since multiple NLRIs can be in one UPDATE.
	prefixKey := ""
	if !r.Prefix.Addr().Is4() {
		prefixKey = r.Prefix.String()
	}
	return fmt.Sprintf("%s|%d|%d|%d|%v|%v|%s|%s|%v|%s|%v|%v|%d|%v",
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

// buildGroupedUpdate builds an UPDATE message for multiple routes with same attributes.
// asn4 indicates whether to use 4-byte AS numbers in AS_PATH.
func buildGroupedUpdate(routes []StaticRoute, localAS uint32, isIBGP, asn4 bool) *message.Update {
	if len(routes) == 0 {
		return &message.Update{}
	}

	// Use first route as representative for attributes.
	route := routes[0]
	var attrBytes []byte

	// 1. ORIGIN.
	origin := attribute.Origin(route.Origin)
	attrBytes = append(attrBytes, attribute.PackAttribute(origin)...)

	// 2. AS_PATH.
	var asPath *attribute.ASPath
	switch {
	case len(route.ASPath) > 0:
		asns := make([]uint32, 0, len(route.ASPath)+1)
		if !isIBGP {
			asns = append(asns, localAS)
		}
		asns = append(asns, route.ASPath...)
		asPath = &attribute.ASPath{
			Segments: []attribute.ASPathSegment{
				{Type: attribute.ASSequence, ASNs: asns},
			},
		}
	case isIBGP:
		asPath = &attribute.ASPath{Segments: nil}
	default:
		asPath = &attribute.ASPath{
			Segments: []attribute.ASPathSegment{
				{Type: attribute.ASSequence, ASNs: []uint32{localAS}},
			},
		}
	}
	attrBytes = append(attrBytes, attribute.PackASPathAttribute(asPath, asn4)...)

	// 3. NEXT_HOP (for IPv4 routes).
	if route.NextHop.Is4() {
		nextHop := &attribute.NextHop{Addr: route.NextHop}
		attrBytes = append(attrBytes, attribute.PackAttribute(nextHop)...)
	}

	// 4. MED (if set).
	if route.MED > 0 {
		med := attribute.MED(route.MED)
		attrBytes = append(attrBytes, attribute.PackAttribute(med)...)
	}

	// 5. LOCAL_PREF (for iBGP).
	if isIBGP {
		localPref := route.LocalPreference
		if localPref == 0 {
			localPref = 100
		}
		attrBytes = append(attrBytes, attribute.PackAttribute(attribute.LocalPref(localPref))...)
	}

	// 6. ATOMIC_AGGREGATE (if set).
	if route.AtomicAggregate {
		attrBytes = append(attrBytes, attribute.PackAttribute(attribute.AtomicAggregate{})...)
	}

	// 7. AGGREGATOR (if set).
	if route.HasAggregator {
		agg := &attribute.Aggregator{
			ASN:     route.AggregatorASN,
			Address: netip.AddrFrom4(route.AggregatorIP),
		}
		attrBytes = append(attrBytes, attribute.PackAttribute(agg)...)
	}

	// 8. COMMUNITIES (RFC 1997) - sorted per RFC 1997.
	if len(route.Communities) > 0 {
		// Copy and sort communities.
		sorted := make([]uint32, len(route.Communities))
		copy(sorted, route.Communities)
		sort.Slice(sorted, func(i, j int) bool { return sorted[i] < sorted[j] })

		comms := make(attribute.Communities, len(sorted))
		for i, c := range sorted {
			comms[i] = attribute.Community(c)
		}
		attrBytes = append(attrBytes, attribute.PackAttribute(comms)...)
	}

	// 9. EXTENDED_COMMUNITY.
	if len(route.ExtCommunityBytes) > 0 {
		ecAttr := make([]byte, 0, 3+len(route.ExtCommunityBytes))
		ecAttr = append(ecAttr, 0xC0, 0x10, byte(len(route.ExtCommunityBytes)))
		ecAttr = append(ecAttr, route.ExtCommunityBytes...)
		attrBytes = append(attrBytes, ecAttr...)
	}

	// 10. LARGE_COMMUNITIES (RFC 8092).
	if len(route.LargeCommunities) > 0 {
		lcs := make(attribute.LargeCommunities, len(route.LargeCommunities))
		for i, lc := range route.LargeCommunities {
			lcs[i] = attribute.LargeCommunity{
				GlobalAdmin: lc[0],
				LocalData1:  lc[1],
				LocalData2:  lc[2],
			}
		}
		attrBytes = append(attrBytes, attribute.PackAttribute(lcs)...)
	}

	// Build NLRI for all routes in group.
	var nlriBytes []byte
	for _, r := range routes {
		switch {
		case r.IsVPN():
			// VPN routes need separate handling - for now, just use first.
			attrBytes = append(attrBytes, buildMPReachNLRI(r)...)
		case r.Prefix.Addr().Is4():
			// IPv4 unicast: append to NLRI.
			inet := nlri.NewINET(nlri.Family{AFI: nlri.AFIIPv4, SAFI: nlri.SAFIUnicast}, r.Prefix, r.PathID)
			nlriBytes = append(nlriBytes, inet.Bytes()...)
		default:
			// IPv6 unicast: use MP_REACH_NLRI (handle separately).
			attrBytes = append(attrBytes, buildMPReachNLRIUnicast(r)...)
		}
	}

	return &message.Update{
		PathAttributes: attrBytes,
		NLRI:           nlriBytes,
	}
}

// buildMPReachNLRI builds MP_REACH_NLRI for VPN routes.
// Returns raw attribute bytes (flags + type + length + value).
func buildMPReachNLRI(route StaticRoute) []byte {
	// Determine AFI/SAFI
	afi := uint16(1) // IPv4
	if route.Prefix.Addr().Is6() {
		afi = 2 // IPv6
	}
	safi := byte(128) // MPLS VPN

	// Build next-hop: RD (all zeros) + IP address
	var nhBytes []byte
	if route.NextHop.Is4() {
		nhBytes = make([]byte, 12) // 8-byte RD + 4-byte IPv4
		copy(nhBytes[8:], route.NextHop.AsSlice())
	} else {
		nhBytes = make([]byte, 24) // 8-byte RD + 16-byte IPv6
		copy(nhBytes[8:], route.NextHop.AsSlice())
	}

	// Build VPN NLRI
	vpnNLRI := buildVPNNLRIBytes(route)

	// MP_REACH_NLRI value: AFI(2) + SAFI(1) + NH_Len(1) + NextHop + Reserved(1) + NLRI
	valueLen := 2 + 1 + 1 + len(nhBytes) + 1 + len(vpnNLRI)
	value := make([]byte, valueLen)
	value[0] = byte(afi >> 8)
	value[1] = byte(afi)
	value[2] = safi
	value[3] = byte(len(nhBytes))
	copy(value[4:], nhBytes)
	value[4+len(nhBytes)] = 0 // Reserved
	copy(value[4+len(nhBytes)+1:], vpnNLRI)

	// Build attribute header: flags + type + length + value
	// Flags: 0x80 = optional, 0x90 = optional + extended length
	var attr []byte
	if valueLen > 255 {
		attr = make([]byte, 4+valueLen)
		attr[0] = 0x90 // optional + extended length
		attr[1] = 14   // MP_REACH_NLRI
		attr[2] = byte(valueLen >> 8)
		attr[3] = byte(valueLen)
		copy(attr[4:], value)
	} else {
		attr = make([]byte, 3+valueLen)
		attr[0] = 0x80 // optional
		attr[1] = 14   // MP_REACH_NLRI
		attr[2] = byte(valueLen)
		copy(attr[3:], value)
	}

	return attr
}

// buildVPNNLRIBytes builds the raw VPN NLRI bytes (for MP_REACH_NLRI).
func buildVPNNLRIBytes(route StaticRoute) []byte {
	// Label encoding: 20-bit label, 3-bit TC, 1-bit BOS (bottom of stack)
	// Label is in upper 20 bits: label << 4, BOS in bit 0
	label := route.Label
	labelBytes := []byte{
		byte(label >> 12),
		byte(label >> 4),
		byte(label<<4) | 0x01, // BOS = 1
	}

	// Prefix bytes
	prefixBits := route.Prefix.Bits()
	prefixBytes := (prefixBits + 7) / 8
	prefixData := route.Prefix.Addr().AsSlice()[:prefixBytes]

	// Total bits: 24 (label) + 64 (RD) + prefix bits
	totalBits := 24 + 64 + prefixBits

	// Build: [path-id] + length + label + RD + prefix
	var buf []byte
	if route.PathID != 0 {
		buf = make([]byte, 4+1+3+8+prefixBytes)
		buf[0] = byte(route.PathID >> 24)
		buf[1] = byte(route.PathID >> 16)
		buf[2] = byte(route.PathID >> 8)
		buf[3] = byte(route.PathID)
		buf[4] = byte(totalBits)
		copy(buf[5:8], labelBytes)
		copy(buf[8:16], route.RDBytes[:])
		copy(buf[16:], prefixData)
	} else {
		buf = make([]byte, 1+3+8+prefixBytes)
		buf[0] = byte(totalBits)
		copy(buf[1:4], labelBytes)
		copy(buf[4:12], route.RDBytes[:])
		copy(buf[12:], prefixData)
	}

	return buf
}

// buildMPReachNLRIExtNHUnicast builds MP_REACH_NLRI for IPv4 unicast NLRI with IPv6 next-hop.
// RFC 8950 Section 3: AFI=1, SAFI=1, NH Length=16, IPv6 next-hop, IPv4 NLRI.
func buildMPReachNLRIExtNHUnicast(route StaticRoute) []byte {
	// Build NLRI bytes for the IPv4 prefix
	prefixBits := route.Prefix.Bits()
	prefixBytes := (prefixBits + 7) / 8
	nlriData := make([]byte, 1+prefixBytes)
	nlriData[0] = byte(prefixBits)
	copy(nlriData[1:], route.Prefix.Addr().AsSlice()[:prefixBytes])

	// RFC 8950: AFI=1 (IPv4), IPv6 next-hop (16 bytes)
	mpReach := &attribute.MPReachNLRI{
		AFI:      attribute.AFIIPv4, // NLRI is IPv4
		SAFI:     attribute.SAFIUnicast,
		NextHops: []netip.Addr{route.NextHop}, // IPv6 next-hop
		NLRI:     nlriData,
	}

	// Pack the attribute with header
	value := mpReach.Pack()
	valueLen := len(value)

	// Build attribute: flags + type + length + value
	// Flags: 0x80 = optional, 0x90 = optional + extended length
	var attr []byte
	if valueLen > 255 {
		attr = make([]byte, 4+valueLen)
		attr[0] = 0x90 // optional + extended length
		attr[1] = byte(attribute.AttrMPReachNLRI)
		attr[2] = byte(valueLen >> 8)
		attr[3] = byte(valueLen)
		copy(attr[4:], value)
	} else {
		attr = make([]byte, 3+valueLen)
		attr[0] = 0x80 // optional
		attr[1] = byte(attribute.AttrMPReachNLRI)
		attr[2] = byte(valueLen)
		copy(attr[3:], value)
	}

	return attr
}

// buildMPReachNLRIUnicast builds MP_REACH_NLRI for IPv6 unicast routes.
// Returns raw attribute bytes (flags + type + length + value).
func buildMPReachNLRIUnicast(route StaticRoute) []byte {
	// Build NLRI bytes for the prefix
	prefixBits := route.Prefix.Bits()
	prefixBytes := (prefixBits + 7) / 8
	nlriData := make([]byte, 1+prefixBytes)
	nlriData[0] = byte(prefixBits)
	copy(nlriData[1:], route.Prefix.Addr().AsSlice()[:prefixBytes])

	// Determine AFI based on address family
	var afi attribute.AFI
	var nhLen int
	if route.Prefix.Addr().Is6() {
		afi = attribute.AFIIPv6
		nhLen = 16
	} else {
		afi = attribute.AFIIPv4
		nhLen = 4
	}

	mpReach := &attribute.MPReachNLRI{
		AFI:      afi,
		SAFI:     attribute.SAFIUnicast,
		NextHops: []netip.Addr{route.NextHop},
		NLRI:     nlriData,
	}

	// Pack the attribute with header
	value := mpReach.Pack()
	valueLen := len(value)

	// Build attribute: flags + type + length + value
	// Flags: 0x80 = optional, 0x90 = optional + extended length
	var attr []byte
	if valueLen > 255 {
		attr = make([]byte, 4+valueLen)
		attr[0] = 0x90 // optional + extended length
		attr[1] = byte(attribute.AttrMPReachNLRI)
		attr[2] = byte(valueLen >> 8)
		attr[3] = byte(valueLen)
		copy(attr[4:], value)
	} else {
		attr = make([]byte, 3+valueLen)
		attr[0] = 0x80 // optional
		attr[1] = byte(attribute.AttrMPReachNLRI)
		attr[2] = byte(valueLen)
		copy(attr[3:], value)
	}

	// Silence unused variable
	_ = nhLen

	return attr
}

// sendMVPNRoutes sends MVPN routes configured for this peer.
func (p *Peer) sendMVPNRoutes() {
	nf := p.families.Load()
	if nf == nil {
		return
	}

	addr := p.settings.Address.String()

	// Group MVPN routes by AFI, filtering by negotiated families
	var ipv4Routes, ipv6Routes []MVPNRoute
	var skippedIPv4, skippedIPv6 int

	for _, route := range p.settings.MVPNRoutes {
		if route.IsIPv6 {
			if nf.IPv6McastVPN {
				ipv6Routes = append(ipv6Routes, route)
			} else {
				skippedIPv6++
			}
		} else {
			if nf.IPv4McastVPN {
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

	// Send IPv4 MVPN routes grouped by next-hop
	if len(ipv4Routes) > 0 {
		ipv4Groups := groupMVPNRoutesByNextHop(ipv4Routes)
		for nh, routes := range ipv4Groups {
			update := buildMVPNUpdate(routes, p.settings.LocalAS, p.settings.IsIBGP(), false, nf.ASN4)
			if err := p.SendUpdate(update); err != nil {
				trace.Log(trace.Routes, "peer %s: MVPN send error: %v", addr, err)
			} else {
				trace.Log(trace.Routes, "peer %s: sent %d IPv4 MVPN routes (NH=%s)", addr, len(routes), nh)
			}
		}
	}

	// Send IPv6 MVPN routes grouped by next-hop
	if len(ipv6Routes) > 0 {
		ipv6Groups := groupMVPNRoutesByNextHop(ipv6Routes)
		for nh, routes := range ipv6Groups {
			update := buildMVPNUpdate(routes, p.settings.LocalAS, p.settings.IsIBGP(), true, nf.ASN4)
			if err := p.SendUpdate(update); err != nil {
				trace.Log(trace.Routes, "peer %s: MVPN send error: %v", addr, err)
			} else {
				trace.Log(trace.Routes, "peer %s: sent %d IPv6 MVPN routes (NH=%s)", addr, len(routes), nh)
			}
		}
	}

	// Send EORs for negotiated MVPN families
	if nf.IPv4McastVPN {
		_ = p.SendUpdate(message.BuildEOR(nlri.Family{AFI: 1, SAFI: 5}))
	}
	if nf.IPv6McastVPN {
		_ = p.SendUpdate(message.BuildEOR(nlri.Family{AFI: 2, SAFI: 5}))
	}
}

// groupMVPNRoutesByNextHop groups MVPN routes by next-hop address.
func groupMVPNRoutesByNextHop(routes []MVPNRoute) map[string][]MVPNRoute {
	groups := make(map[string][]MVPNRoute)
	for _, route := range routes {
		key := route.NextHop.String()
		groups[key] = append(groups[key], route)
	}
	return groups
}

// buildMVPNUpdate builds an UPDATE message for MVPN routes.
func buildMVPNUpdate(routes []MVPNRoute, localAS uint32, isIBGP, isIPv6, asn4 bool) *message.Update {
	if len(routes) == 0 {
		return &message.Update{}
	}

	// Use first route for common attributes
	first := routes[0]

	var attrBytes []byte

	// 1. ORIGIN (default to IGP if not set; Origin 0 is IGP, which is correct default)
	origin := attribute.Origin(first.Origin)
	attrBytes = append(attrBytes, attribute.PackAttribute(origin)...)

	// 2. AS_PATH (empty for iBGP)
	var asPath *attribute.ASPath
	if isIBGP {
		asPath = &attribute.ASPath{Segments: nil}
	} else {
		asPath = &attribute.ASPath{
			Segments: []attribute.ASPathSegment{
				{Type: attribute.ASSequence, ASNs: []uint32{localAS}},
			},
		}
	}
	attrBytes = append(attrBytes, attribute.PackASPathAttribute(asPath, asn4)...)

	// 3. NEXT_HOP (via MP_REACH_NLRI for MVPN)
	// Build as traditional NEXT_HOP for test compatibility
	if first.NextHop.Is4() {
		nextHop := &attribute.NextHop{Addr: first.NextHop}
		attrBytes = append(attrBytes, attribute.PackAttribute(nextHop)...)
	}

	// 4. LOCAL_PREF (for iBGP, default to 100 if not set)
	if isIBGP {
		lp := first.LocalPreference
		if lp == 0 {
			lp = 100 // Default LOCAL_PREF for iBGP
		}
		attrBytes = append(attrBytes, attribute.PackAttribute(attribute.LocalPref(lp))...)
	}

	// 5. Extended Communities (manually build attribute)
	if len(first.ExtCommunityBytes) > 0 {
		ecAttr := make([]byte, 0, 3+len(first.ExtCommunityBytes))
		ecAttr = append(ecAttr, 0xC0, 0x10, byte(len(first.ExtCommunityBytes)))
		ecAttr = append(ecAttr, first.ExtCommunityBytes...)
		attrBytes = append(attrBytes, ecAttr...)
	}

	// 6. MP_REACH_NLRI with MVPN NLRIs
	mpReach := buildMVPNMPReachNLRI(routes, isIPv6)
	attrBytes = append(attrBytes, mpReach...)

	return &message.Update{
		PathAttributes: attrBytes,
	}
}

// buildMVPNMPReachNLRI builds MP_REACH_NLRI for MVPN routes.
func buildMVPNMPReachNLRI(routes []MVPNRoute, isIPv6 bool) []byte {
	if len(routes) == 0 {
		return nil
	}

	first := routes[0]

	// Build NLRI data for all routes
	var nlriData []byte
	for _, route := range routes {
		nlri := buildMVPNNLRI(route)
		nlriData = append(nlriData, nlri...)
	}

	// AFI/SAFI
	var afi uint16 = 1 // IPv4
	if isIPv6 {
		afi = 2 // IPv6
	}
	const safiMVPN uint8 = 5

	// Next-hop
	nhBytes := first.NextHop.AsSlice()
	nhLen := len(nhBytes)

	// Build MP_REACH_NLRI value
	// AFI (2) + SAFI (1) + NH Len (1) + NH + Reserved (1) + NLRI
	valueLen := 2 + 1 + 1 + nhLen + 1 + len(nlriData)
	value := make([]byte, valueLen)
	value[0] = byte(afi >> 8)
	value[1] = byte(afi)
	value[2] = safiMVPN
	value[3] = byte(nhLen)
	copy(value[4:4+nhLen], nhBytes)
	value[4+nhLen] = 0 // reserved
	copy(value[5+nhLen:], nlriData)

	// Build attribute header
	var attr []byte
	if valueLen > 255 {
		attr = make([]byte, 4+valueLen)
		attr[0] = 0x90 // optional + extended length
		attr[1] = byte(attribute.AttrMPReachNLRI)
		attr[2] = byte(valueLen >> 8)
		attr[3] = byte(valueLen)
		copy(attr[4:], value)
	} else {
		attr = make([]byte, 3+valueLen)
		attr[0] = 0x80 // optional
		attr[1] = byte(attribute.AttrMPReachNLRI)
		attr[2] = byte(valueLen)
		copy(attr[3:], value)
	}

	return attr
}

// buildMVPNNLRI builds a single MVPN NLRI.
func buildMVPNNLRI(route MVPNRoute) []byte {
	// MVPN NLRI format:
	// Route Type (1) + Length (1) + Route Type Specific Data

	var data []byte

	switch route.RouteType {
	case 5: // Source Active A-D
		// RD (8) + Source (4/16) + Group (4/16)
		data = append(data, route.RD[:]...)
		if route.Source.Is4() {
			// Encoded as /32 prefix
			data = append(data, 32) // prefix len
			src4 := route.Source.As4()
			data = append(data, src4[:]...)
		} else {
			data = append(data, 128) // prefix len
			src16 := route.Source.As16()
			data = append(data, src16[:]...)
		}
		if route.Group.Is4() {
			data = append(data, 32) // prefix len
			grp4 := route.Group.As4()
			data = append(data, grp4[:]...)
		} else {
			data = append(data, 128) // prefix len
			grp16 := route.Group.As16()
			data = append(data, grp16[:]...)
		}

	case 6, 7: // Shared Tree Join (6) or Source Tree Join (7)
		// RD (8) + Source-AS (4) + Source/RP (4/16) + Group (4/16)
		data = append(data, route.RD[:]...)
		// Source-AS as 4 bytes
		data = append(data, byte(route.SourceAS>>24), byte(route.SourceAS>>16),
			byte(route.SourceAS>>8), byte(route.SourceAS))
		if route.Source.Is4() {
			data = append(data, 32) // prefix len
			src4 := route.Source.As4()
			data = append(data, src4[:]...)
		} else {
			data = append(data, 128) // prefix len
			src16 := route.Source.As16()
			data = append(data, src16[:]...)
		}
		if route.Group.Is4() {
			data = append(data, 32) // prefix len
			grp4 := route.Group.As4()
			data = append(data, grp4[:]...)
		} else {
			data = append(data, 128) // prefix len
			grp16 := route.Group.As16()
			data = append(data, grp16[:]...)
		}
	}

	// Build NLRI: Route Type (1) + Length (1) + Data
	nlri := make([]byte, 2+len(data))
	nlri[0] = route.RouteType
	nlri[1] = byte(len(data))
	copy(nlri[2:], data)

	return nlri
}

// sendVPLSRoutes sends VPLS routes configured for this peer.
func (p *Peer) sendVPLSRoutes() {
	nf := p.families.Load()
	if nf == nil || !nf.L2VPNVPLS {
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
		for _, route := range p.settings.VPLSRoutes {
			update := buildVPLSUpdate(route, p.settings.LocalAS, p.settings.IsIBGP(), nf.ASN4)
			if err := p.SendUpdate(update); err != nil {
				trace.Log(trace.Routes, "peer %s: VPLS send error: %v", addr, err)
			}
		}
	}

	// Send EOR for L2VPN VPLS
	_ = p.SendUpdate(message.BuildEOR(nlri.Family{AFI: 25, SAFI: 65}))
}

// buildVPLSUpdate builds an UPDATE message for a VPLS route.
// asn4 indicates whether to use 4-byte AS numbers in AS_PATH.
func buildVPLSUpdate(route VPLSRoute, localAS uint32, isIBGP, asn4 bool) *message.Update {
	var attrBytes []byte

	// 1. ORIGIN
	origin := attribute.Origin(route.Origin)
	attrBytes = append(attrBytes, attribute.PackAttribute(origin)...)

	// 2. AS_PATH
	var asPath *attribute.ASPath
	switch {
	case isIBGP && len(route.ASPath) == 0:
		asPath = &attribute.ASPath{Segments: nil}
	case len(route.ASPath) > 0:
		asPath = &attribute.ASPath{
			Segments: []attribute.ASPathSegment{
				{Type: attribute.ASSequence, ASNs: route.ASPath},
			},
		}
	default:
		asPath = &attribute.ASPath{
			Segments: []attribute.ASPathSegment{
				{Type: attribute.ASSequence, ASNs: []uint32{localAS}},
			},
		}
	}
	attrBytes = append(attrBytes, attribute.PackASPathAttribute(asPath, asn4)...)

	// Note: NEXT_HOP is in MP_REACH_NLRI for VPLS, not as separate attribute

	// 3. MED (type 4, before LOCAL_PREF for RFC ordering)
	if route.MED > 0 {
		attrBytes = append(attrBytes, attribute.PackAttribute(attribute.MED(route.MED))...)
	}

	// 4. LOCAL_PREF (type 5)
	if isIBGP {
		lp := route.LocalPreference
		if lp == 0 {
			lp = 100
		}
		attrBytes = append(attrBytes, attribute.PackAttribute(attribute.LocalPref(lp))...)
	}

	// 6. COMMUNITIES
	if len(route.Communities) > 0 {
		sorted := make([]uint32, len(route.Communities))
		copy(sorted, route.Communities)
		sort.Slice(sorted, func(i, j int) bool { return sorted[i] < sorted[j] })
		commAttr := make([]byte, 0, 3+len(sorted)*4)
		commAttr = append(commAttr, 0xC0, 0x08, byte(len(sorted)*4))
		for _, c := range sorted {
			commAttr = append(commAttr, byte(c>>24), byte(c>>16), byte(c>>8), byte(c))
		}
		attrBytes = append(attrBytes, commAttr...)
	}

	// 7. ORIGINATOR_ID (type 9)
	if route.OriginatorID != 0 {
		origAttr := []byte{0x80, 0x09, 0x04,
			byte(route.OriginatorID >> 24), byte(route.OriginatorID >> 16),
			byte(route.OriginatorID >> 8), byte(route.OriginatorID)}
		attrBytes = append(attrBytes, origAttr...)
	}

	// 8. CLUSTER_LIST (type 10)
	if len(route.ClusterList) > 0 {
		clAttr := make([]byte, 0, 3+len(route.ClusterList)*4)
		clAttr = append(clAttr, 0x80, 0x0A, byte(len(route.ClusterList)*4))
		for _, c := range route.ClusterList {
			clAttr = append(clAttr, byte(c>>24), byte(c>>16), byte(c>>8), byte(c))
		}
		attrBytes = append(attrBytes, clAttr...)
	}

	// 9. EXTENDED_COMMUNITY (type 16)
	if len(route.ExtCommunityBytes) > 0 {
		ecAttr := make([]byte, 0, 3+len(route.ExtCommunityBytes))
		ecAttr = append(ecAttr, 0xC0, 0x10, byte(len(route.ExtCommunityBytes)))
		ecAttr = append(ecAttr, route.ExtCommunityBytes...)
		attrBytes = append(attrBytes, ecAttr...)
	}

	// 10. MP_REACH_NLRI for VPLS (type 14)
	mpReach := buildVPLSMPReachNLRI(route)
	attrBytes = append(attrBytes, mpReach...)

	return &message.Update{
		PathAttributes: attrBytes,
	}
}

// buildVPLSMPReachNLRI builds MP_REACH_NLRI for a VPLS route.
func buildVPLSMPReachNLRI(route VPLSRoute) []byte {
	// Build VPLS NLRI
	var rd nlri.RouteDistinguisher
	copy(rd.Value[:], route.RD[2:8])
	rd.Type = nlri.RDType(uint16(route.RD[0])<<8 | uint16(route.RD[1]))

	vpls := nlri.NewVPLSFull(rd, route.Endpoint, route.Offset, route.Size, route.Base)
	nlriData := vpls.Bytes()

	// Next-hop
	nhBytes := route.NextHop.AsSlice()

	// Build MP_REACH_NLRI
	// AFI=25 (L2VPN), SAFI=65 (VPLS)
	valueLen := 2 + 1 + 1 + len(nhBytes) + 1 + len(nlriData)
	value := make([]byte, valueLen)
	value[0], value[1] = 0, 25 // AFI
	value[2] = 65              // SAFI
	value[3] = byte(len(nhBytes))
	copy(value[4:4+len(nhBytes)], nhBytes)
	value[4+len(nhBytes)] = 0 // Reserved
	copy(value[5+len(nhBytes):], nlriData)

	// Build attribute header - use extended length only if needed
	var attr []byte
	if valueLen > 255 {
		attr = make([]byte, 4+valueLen)
		attr[0] = 0x90                // Optional, Extended Length
		attr[1] = 14                  // MP_REACH_NLRI
		attr[2] = byte(valueLen >> 8) // Length high
		attr[3] = byte(valueLen)      // Length low
		copy(attr[4:], value)
	} else {
		attr = make([]byte, 3+valueLen)
		attr[0] = 0x80 // Optional
		attr[1] = 14   // MP_REACH_NLRI
		attr[2] = byte(valueLen)
		copy(attr[3:], value)
	}

	return attr
}

// sendFlowSpecRoutes sends FlowSpec routes configured for this peer.
// Only sends routes for families that were successfully negotiated.
func (p *Peer) sendFlowSpecRoutes() {
	nf := p.families.Load()
	if nf == nil {
		return
	}

	addr := p.settings.Address.String()

	// Send routes only for negotiated families
	var sentCount int
	for _, route := range p.settings.FlowSpecRoutes {
		// Check if this route's family is negotiated
		isIPv6 := route.IsIPv6
		isVPN := route.RD != [8]byte{}

		var negotiated bool
		switch {
		case !isIPv6 && !isVPN:
			negotiated = nf.IPv4FlowSpec
		case !isIPv6 && isVPN:
			negotiated = nf.IPv4FlowSpecVPN
		case isIPv6 && !isVPN:
			negotiated = nf.IPv6FlowSpec
		case isIPv6 && isVPN:
			negotiated = nf.IPv6FlowSpecVPN
		}

		if !negotiated {
			trace.Log(trace.Routes, "peer %s: skipping FlowSpec route (family not negotiated)", addr)
			continue
		}

		update := buildFlowSpecUpdate(route, p.settings.LocalAS, p.settings.IsIBGP(), nf.ASN4)
		if err := p.SendUpdate(update); err != nil {
			trace.Log(trace.Routes, "peer %s: FlowSpec send error: %v", addr, err)
		}
		sentCount++
	}
	if sentCount > 0 {
		trace.Log(trace.Routes, "peer %s: sent %d FlowSpec routes", addr, sentCount)
	}

	// Send EORs for all negotiated FlowSpec families (even if no routes)
	if nf.IPv4FlowSpec {
		_ = p.SendUpdate(message.BuildEOR(nlri.Family{AFI: 1, SAFI: 133}))
	}
	if nf.IPv6FlowSpec {
		_ = p.SendUpdate(message.BuildEOR(nlri.Family{AFI: 2, SAFI: 133}))
	}
	if nf.IPv4FlowSpecVPN {
		_ = p.SendUpdate(message.BuildEOR(nlri.Family{AFI: 1, SAFI: 134}))
	}
	if nf.IPv6FlowSpecVPN {
		_ = p.SendUpdate(message.BuildEOR(nlri.Family{AFI: 2, SAFI: 134}))
	}
}

// buildFlowSpecUpdate builds an UPDATE message for a FlowSpec route.
// asn4 indicates whether to use 4-byte AS numbers in AS_PATH.
func buildFlowSpecUpdate(route FlowSpecRoute, localAS uint32, isIBGP, asn4 bool) *message.Update {
	var attrBytes []byte

	// 1. ORIGIN (IGP)
	origin := attribute.Origin(0)
	attrBytes = append(attrBytes, attribute.PackAttribute(origin)...)

	// 2. AS_PATH
	var asPath *attribute.ASPath
	if isIBGP {
		asPath = &attribute.ASPath{Segments: nil}
	} else {
		asPath = &attribute.ASPath{
			Segments: []attribute.ASPathSegment{
				{Type: attribute.ASSequence, ASNs: []uint32{localAS}},
			},
		}
	}
	attrBytes = append(attrBytes, attribute.PackASPathAttribute(asPath, asn4)...)

	// 3. LOCAL_PREF
	if isIBGP {
		attrBytes = append(attrBytes, attribute.PackAttribute(attribute.LocalPref(100))...)
	}

	// 4. COMMUNITY (if set)
	if len(route.CommunityBytes) > 0 {
		commAttr := make([]byte, 0, 3+len(route.CommunityBytes))
		commAttr = append(commAttr, 0xC0, 0x08, byte(len(route.CommunityBytes)))
		commAttr = append(commAttr, route.CommunityBytes...)
		attrBytes = append(attrBytes, commAttr...)
	}

	// 5. EXTENDED_COMMUNITY (for actions like rate-limit, redirect)
	if len(route.ExtCommunityBytes) > 0 {
		ecAttr := make([]byte, 0, 3+len(route.ExtCommunityBytes))
		ecAttr = append(ecAttr, 0xC0, 0x10, byte(len(route.ExtCommunityBytes)))
		ecAttr = append(ecAttr, route.ExtCommunityBytes...)
		attrBytes = append(attrBytes, ecAttr...)
	}

	// 5b. IPv6 EXTENDED_COMMUNITY (attribute 25, RFC 5701)
	// Used for IPv6-specific actions like redirect-to-nexthop-ietf with IPv6 address
	if len(route.IPv6ExtCommunityBytes) > 0 {
		ec6Attr := make([]byte, 0, 3+len(route.IPv6ExtCommunityBytes))
		ec6Attr = append(ec6Attr, 0xC0, 0x19, byte(len(route.IPv6ExtCommunityBytes)))
		ec6Attr = append(ec6Attr, route.IPv6ExtCommunityBytes...)
		attrBytes = append(attrBytes, ec6Attr...)
	}

	// 6. MP_REACH_NLRI for FlowSpec
	mpReach := buildFlowSpecMPReachNLRI(route)
	attrBytes = append(attrBytes, mpReach...)

	return &message.Update{
		PathAttributes: attrBytes,
	}
}

// buildFlowSpecMPReachNLRI builds MP_REACH_NLRI for a FlowSpec route.
func buildFlowSpecMPReachNLRI(route FlowSpecRoute) []byte {
	nlriData := route.NLRI
	if len(nlriData) == 0 {
		return nil
	}

	// Determine AFI/SAFI
	var afi uint16 = 1  // IPv4
	var safi byte = 133 // FlowSpec
	if route.IsIPv6 {
		afi = 2 // IPv6
	}
	isVPN := route.RD != [8]byte{}
	if isVPN {
		safi = 134 // FlowSpec VPN
	}

	// Next-hop (usually 0 length for FlowSpec, or actual for VPN)
	var nhBytes []byte
	if route.NextHop.IsValid() {
		nhBytes = route.NextHop.AsSlice()
	}

	// Build the full NLRI with length prefix.
	// For FlowSpec VPN, format is: length + RD (8 bytes) + FlowSpec NLRI
	var fullNLRI []byte
	if isVPN {
		// VPN format: length (1 or 2 bytes) + RD (8 bytes) + FlowSpec NLRI
		nlriLen := 8 + len(nlriData)
		if nlriLen < 240 {
			fullNLRI = make([]byte, 1+nlriLen)
			fullNLRI[0] = byte(nlriLen)
			copy(fullNLRI[1:9], route.RD[:])
			copy(fullNLRI[9:], nlriData)
		} else {
			fullNLRI = make([]byte, 2+nlriLen)
			fullNLRI[0] = 0xF0 | byte(nlriLen>>8)
			fullNLRI[1] = byte(nlriLen)
			copy(fullNLRI[2:10], route.RD[:])
			copy(fullNLRI[10:], nlriData)
		}
	} else {
		// Non-VPN format: FlowSpec NLRI already includes length prefix
		fullNLRI = nlriData
	}

	// Build MP_REACH_NLRI value
	valueLen := 2 + 1 + 1 + len(nhBytes) + 1 + len(fullNLRI)
	value := make([]byte, valueLen)
	value[0] = byte(afi >> 8)
	value[1] = byte(afi)
	value[2] = safi
	value[3] = byte(len(nhBytes))
	copy(value[4:4+len(nhBytes)], nhBytes)
	value[4+len(nhBytes)] = 0 // Reserved
	copy(value[5+len(nhBytes):], fullNLRI)

	// Build attribute header (use short form when possible)
	var attr []byte
	if valueLen > 255 {
		attr = make([]byte, 4+valueLen)
		attr[0] = 0x90 // Optional + Extended Length
		attr[1] = 14   // MP_REACH_NLRI
		attr[2] = byte(valueLen >> 8)
		attr[3] = byte(valueLen)
		copy(attr[4:], value)
	} else {
		attr = make([]byte, 3+valueLen)
		attr[0] = 0x80 // Optional
		attr[1] = 14   // MP_REACH_NLRI
		attr[2] = byte(valueLen)
		copy(attr[3:], value)
	}

	return attr
}

// sendMUPRoutes sends MUP routes configured for this peer.
func (p *Peer) sendMUPRoutes() {
	nf := p.families.Load()
	if nf == nil {
		return
	}

	addr := p.settings.Address.String()

	// Separate routes by AFI, filtering by negotiated families
	var ipv4Routes, ipv6Routes []MUPRoute
	var skippedIPv4, skippedIPv6 int

	for _, route := range p.settings.MUPRoutes {
		if route.IsIPv6 {
			if nf.IPv6MUP {
				ipv6Routes = append(ipv6Routes, route)
			} else {
				skippedIPv6++
			}
		} else {
			if nf.IPv4MUP {
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
		for _, route := range ipv4Routes {
			update := buildMUPUpdate(route, p.settings.LocalAS, p.settings.IsIBGP(), nf.ASN4)
			if err := p.SendUpdate(update); err != nil {
				trace.Log(trace.Routes, "peer %s: MUP send error: %v", addr, err)
			}
		}
	}

	// Send IPv6 MUP routes
	if len(ipv6Routes) > 0 {
		trace.Log(trace.Routes, "peer %s: sending %d IPv6 MUP routes", addr, len(ipv6Routes))
		for _, route := range ipv6Routes {
			update := buildMUPUpdate(route, p.settings.LocalAS, p.settings.IsIBGP(), nf.ASN4)
			if err := p.SendUpdate(update); err != nil {
				trace.Log(trace.Routes, "peer %s: MUP send error: %v", addr, err)
			}
		}
	}

	// Send EORs for negotiated MUP families
	if nf.IPv4MUP {
		_ = p.SendUpdate(message.BuildEOR(nlri.Family{AFI: 1, SAFI: safiMUP}))
	}
	if nf.IPv6MUP {
		_ = p.SendUpdate(message.BuildEOR(nlri.Family{AFI: 2, SAFI: safiMUP}))
	}
}

// buildMUPUpdate builds an UPDATE message for a MUP route.
// asn4 indicates whether to use 4-byte AS numbers in AS_PATH.
func buildMUPUpdate(route MUPRoute, localAS uint32, isIBGP, asn4 bool) *message.Update {
	var attrBytes []byte

	// 1. ORIGIN (IGP)
	origin := attribute.Origin(0)
	attrBytes = append(attrBytes, attribute.PackAttribute(origin)...)

	// 2. AS_PATH
	var asPath *attribute.ASPath
	if isIBGP {
		asPath = &attribute.ASPath{Segments: nil}
	} else {
		asPath = &attribute.ASPath{
			Segments: []attribute.ASPathSegment{
				{Type: attribute.ASSequence, ASNs: []uint32{localAS}},
			},
		}
	}
	attrBytes = append(attrBytes, attribute.PackASPathAttribute(asPath, asn4)...)

	// 3. LOCAL_PREF
	if isIBGP {
		attrBytes = append(attrBytes, attribute.PackAttribute(attribute.LocalPref(100))...)
	}

	// 4. EXTENDED_COMMUNITY
	if len(route.ExtCommunityBytes) > 0 {
		ecAttr := make([]byte, 0, 3+len(route.ExtCommunityBytes))
		ecAttr = append(ecAttr, 0xC0, 0x10, byte(len(route.ExtCommunityBytes)))
		ecAttr = append(ecAttr, route.ExtCommunityBytes...)
		attrBytes = append(attrBytes, ecAttr...)
	}

	// 5. PREFIX_SID (if present)
	if len(route.PrefixSID) > 0 {
		// Type 40 (Prefix SID)
		if len(route.PrefixSID) <= 255 {
			sidAttr := make([]byte, 0, 3+len(route.PrefixSID))
			sidAttr = append(sidAttr, 0xC0, 0x28, byte(len(route.PrefixSID)))
			sidAttr = append(sidAttr, route.PrefixSID...)
			attrBytes = append(attrBytes, sidAttr...)
		}
	}

	// 6. MP_REACH_NLRI for MUP
	mpReach := buildMUPMPReachNLRI(route)
	attrBytes = append(attrBytes, mpReach...)

	return &message.Update{
		PathAttributes: attrBytes,
	}
}

// buildMUPMPReachNLRI builds MP_REACH_NLRI for a MUP route.
func buildMUPMPReachNLRI(route MUPRoute) []byte {
	nlriData := route.NLRI
	if len(nlriData) == 0 {
		return nil
	}

	// Determine AFI
	var afi uint16 = 1 // IPv4
	if route.IsIPv6 {
		afi = 2 // IPv6
	}
	var safi byte = safiMUP

	// Next-hop
	nhBytes := route.NextHop.AsSlice()

	// Build MP_REACH_NLRI value
	valueLen := 2 + 1 + 1 + len(nhBytes) + 1 + len(nlriData)
	value := make([]byte, valueLen)
	value[0] = byte(afi >> 8)
	value[1] = byte(afi)
	value[2] = safi
	value[3] = byte(len(nhBytes))
	copy(value[4:4+len(nhBytes)], nhBytes)
	value[4+len(nhBytes)] = 0 // Reserved
	copy(value[5+len(nhBytes):], nlriData)

	// Build attribute header
	attr := make([]byte, 4+valueLen)
	attr[0] = 0x90
	attr[1] = 14
	attr[2] = byte(valueLen >> 8)
	attr[3] = byte(valueLen)
	copy(attr[4:], value)

	return attr
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

	asn4 := true
	if neg := session.Negotiated(); neg != nil {
		asn4 = neg.ASN4
	}

	// Get negotiated families for ExtNH support
	nf := p.families.Load()

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

		// Send the route
		update := buildStaticRouteUpdate(wr.StaticRoute, p.settings.LocalAS, p.settings.IsIBGP(), asn4, nf)
		if err := p.SendUpdate(update); err != nil {
			return err
		}

		// Update state
		p.mu.Lock()
		p.watchdogState[name][routeKey] = true
		p.mu.Unlock()

		trace.RouteSent(addr, routeKey, wr.NextHop.String())
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
		update := buildStaticRouteWithdraw(wr.StaticRoute)
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
