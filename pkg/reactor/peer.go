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
		if to == fsm.StateEstablished {
			p.setState(PeerStateEstablished)
			// Send static routes from config.
			go p.sendInitialRoutes()
		} else if from == fsm.StateEstablished {
			p.setState(PeerStateConnecting)
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
	for _, route := range p.neighbor.StaticRoutes {
		update := buildStaticRouteUpdate(route, p.neighbor.LocalAS, p.neighbor.IsIBGP())
		if err := p.SendUpdate(update); err != nil {
			// Log error but continue - connection may have closed.
			break
		}
	}

	// Send End-of-RIB marker for IPv4 unicast.
	if len(p.neighbor.StaticRoutes) > 0 {
		eor := buildEORUpdate(1, 1) // IPv4 unicast
		_ = p.SendUpdate(eor)
	}
}

// buildStaticRouteUpdate builds an UPDATE message for a static route.
func buildStaticRouteUpdate(route StaticRoute, localAS uint32, isIBGP bool) *message.Update {
	var attrBytes []byte

	// 1. ORIGIN (IGP by default, use configured value if set)
	origin := attribute.Origin(route.Origin)
	attrBytes = append(attrBytes, attribute.PackAttribute(origin)...)

	// 2. AS_PATH (prepend local AS for eBGP, empty for iBGP routes we originate)
	asPath := &attribute.ASPath{
		Segments: []attribute.ASPathSegment{
			{Type: attribute.ASSequence, ASNs: []uint32{localAS}},
		},
	}
	attrBytes = append(attrBytes, attribute.PackAttribute(asPath)...)

	// 3. NEXT_HOP
	nextHop := &attribute.NextHop{Addr: route.NextHop}
	attrBytes = append(attrBytes, attribute.PackAttribute(nextHop)...)

	// 4. LOCAL_PREF (for iBGP)
	if isIBGP && route.LocalPreference > 0 {
		localPref := attribute.LocalPref(route.LocalPreference)
		attrBytes = append(attrBytes, attribute.PackAttribute(localPref)...)
	}

	// 5. MED (if set)
	if route.MED > 0 {
		med := attribute.MED(route.MED)
		attrBytes = append(attrBytes, attribute.PackAttribute(med)...)
	}

	// Build NLRI
	var nlriBytes []byte
	if route.Prefix.Addr().Is4() {
		inet := nlri.NewINET(nlri.Family{AFI: nlri.AFIIPv4, SAFI: nlri.SAFIUnicast}, route.Prefix, 0)
		nlriBytes = inet.Bytes()
	}

	return &message.Update{
		PathAttributes: attrBytes,
		NLRI:           nlriBytes,
	}
}
