package reactor

import (
	"context"
	"net"
	"sync"
	"sync/atomic"
	"time"

	"github.com/exa-networks/zebgp/pkg/bgp/attribute"
	"github.com/exa-networks/zebgp/pkg/bgp/fsm"
	"github.com/exa-networks/zebgp/pkg/bgp/message"
	"github.com/exa-networks/zebgp/pkg/bgp/nlri"
	"github.com/exa-networks/zebgp/pkg/trace"
)

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

// Peer wraps a Session with reconnection logic.
//
// It manages the connection lifecycle in its own goroutine,
// automatically reconnecting on failure with exponential backoff.
type Peer struct {
	neighbor *Neighbor
	session  *Session

	state    atomic.Int32
	callback PeerCallback

	// Reconnect configuration
	reconnectMin time.Duration
	reconnectMax time.Duration

	// Goroutine control
	ctx    context.Context
	cancel context.CancelFunc
	wg     sync.WaitGroup

	mu sync.RWMutex
}

// NewPeer creates a new peer for the given neighbor.
func NewPeer(neighbor *Neighbor) *Peer {
	return &Peer{
		neighbor:     neighbor,
		reconnectMin: DefaultReconnectMin,
		reconnectMax: DefaultReconnectMax,
	}
}

// Neighbor returns the configured neighbor.
func (p *Peer) Neighbor() *Neighbor {
	return p.neighbor
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
			// Backoff before retry
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
	session := NewSession(p.neighbor)

	p.mu.Lock()
	p.session = session
	p.mu.Unlock()

	defer func() {
		p.mu.Lock()
		p.session = nil
		p.mu.Unlock()
	}()

	// Update state based on FSM mode
	if p.neighbor.Passive {
		p.setState(PeerStateActive)
	} else {
		p.setState(PeerStateConnecting)
	}

	// Start FSM
	if err := session.Start(); err != nil {
		return err
	}

	// Connect (for active mode)
	if !p.neighbor.Passive {
		if err := session.Connect(p.ctx); err != nil {
			return err
		}
	}

	// Monitor FSM state
	session.fsm.SetCallback(func(from, to fsm.State) {
		addr := p.neighbor.Address.String()
		trace.FSMTransition(addr, from.String(), to.String())

		if to == fsm.StateEstablished {
			p.setState(PeerStateEstablished)
			trace.SessionEstablished(addr, p.neighbor.LocalAS, p.neighbor.PeerAS)
			// Send static routes from config.
			go p.sendInitialRoutes()
		} else if from == fsm.StateEstablished {
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

// cleanup runs when peer stops.
func (p *Peer) cleanup() {
	p.mu.Lock()
	if p.session != nil {
		_ = p.session.Close()
		p.session = nil
	}
	p.cancel = nil
	p.mu.Unlock()

	p.setState(PeerStateStopped)
}

// sendInitialRoutes sends static routes configured for this neighbor.
func (p *Peer) sendInitialRoutes() {
	addr := p.neighbor.Address.String()
	trace.Log(trace.Routes, "neighbor %s: sending %d static routes", addr, len(p.neighbor.StaticRoutes))

	for _, route := range p.neighbor.StaticRoutes {
		update := buildStaticRouteUpdate(route, p.neighbor.LocalAS, p.neighbor.IsIBGP())
		if err := p.SendUpdate(update); err != nil {
			trace.Log(trace.Routes, "neighbor %s: send error: %v", addr, err)
			// Log error but continue - connection may have closed.
			break
		}
		trace.RouteSent(addr, route.Prefix.String(), route.NextHop.String())
	}

	// Send End-of-RIB marker for IPv4 unicast.
	if len(p.neighbor.StaticRoutes) > 0 {
		eor := buildEORUpdate(1, 1) // IPv4 unicast
		_ = p.SendUpdate(eor)
		trace.Log(trace.Routes, "neighbor %s: sent EOR marker", addr)
	}
}

// buildStaticRouteUpdate builds an UPDATE message for a static route.
func buildStaticRouteUpdate(route StaticRoute, localAS uint32, isIBGP bool) *message.Update {
	var attrBytes []byte

	// 1. ORIGIN (IGP by default, use configured value if set)
	origin := attribute.Origin(route.Origin)
	attrBytes = append(attrBytes, attribute.PackAttribute(origin)...)

	// 2. AS_PATH
	// - For iBGP self-originated routes: empty AS_PATH
	// - For eBGP: prepend local AS
	var asPath *attribute.ASPath
	if isIBGP {
		// Empty AS_PATH for iBGP self-originated routes
		asPath = &attribute.ASPath{Segments: nil}
	} else {
		// Prepend local AS for eBGP
		asPath = &attribute.ASPath{
			Segments: []attribute.ASPathSegment{
				{Type: attribute.ASSequence, ASNs: []uint32{localAS}},
			},
		}
	}
	attrBytes = append(attrBytes, attribute.PackAttribute(asPath)...)

	// 3. NEXT_HOP
	nextHop := &attribute.NextHop{Addr: route.NextHop}
	attrBytes = append(attrBytes, attribute.PackAttribute(nextHop)...)

	// 4. LOCAL_PREF (for iBGP, default 100 if not set)
	if isIBGP {
		localPref := route.LocalPreference
		if localPref == 0 {
			localPref = 100 // Default LOCAL_PREF
		}
		attrBytes = append(attrBytes, attribute.PackAttribute(attribute.LocalPref(localPref))...)
	}

	// 5. MED (if set)
	if route.MED > 0 {
		med := attribute.MED(route.MED)
		attrBytes = append(attrBytes, attribute.PackAttribute(med)...)
	}

	// 6. EXTENDED_COMMUNITY (if set)
	if len(route.ExtCommunityBytes) > 0 {
		// Pack as attribute: flags=0xC0 (optional, transitive), type=16, len, value
		ecAttr := make([]byte, 0, 3+len(route.ExtCommunityBytes))
		ecAttr = append(ecAttr, 0xC0, 0x10, byte(len(route.ExtCommunityBytes)))
		ecAttr = append(ecAttr, route.ExtCommunityBytes...)
		attrBytes = append(attrBytes, ecAttr...)
	}

	// Build NLRI with optional path-id for ADD-PATH
	var nlriBytes []byte
	if route.Prefix.Addr().Is4() {
		inet := nlri.NewINET(nlri.Family{AFI: nlri.AFIIPv4, SAFI: nlri.SAFIUnicast}, route.Prefix, route.PathID)
		nlriBytes = inet.Bytes()
	}

	return &message.Update{
		PathAttributes: attrBytes,
		NLRI:           nlriBytes,
	}
}
