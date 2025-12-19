// Package reactor implements the BGP reactor - the main orchestrator
// that manages peer sessions, connections, and signal handling.
package reactor

import (
	"context"
	"errors"
	"net"
	"net/netip"
	"sync"
	"time"
)

// Reactor errors.
var (
	ErrAlreadyRunning   = errors.New("reactor already running")
	ErrNotRunning       = errors.New("reactor not running")
	ErrNeighborExists   = errors.New("neighbor already exists")
	ErrNeighborNotFound = errors.New("neighbor not found")
)

// Config holds reactor configuration.
type Config struct {
	// ListenAddr is the address to listen on (e.g., "0.0.0.0:179").
	ListenAddr string

	// RouterID is the local BGP router identifier.
	RouterID uint32

	// LocalAS is the local AS number.
	LocalAS uint32
}

// Stats holds reactor statistics.
type Stats struct {
	StartTime time.Time
	Uptime    time.Duration
	PeerCount int
}

// ConnectionCallback is called when a connection is matched to a neighbor.
type ConnectionCallback func(conn net.Conn, neighbor *Neighbor)

// Reactor is the main BGP orchestrator.
//
// It manages:
//   - Peer connections (outgoing)
//   - Listener for incoming connections
//   - Signal handling
//   - Graceful shutdown
type Reactor struct {
	config *Config

	peers    map[string]*Peer // keyed by neighbor address
	listener *Listener
	signals  *SignalHandler

	connCallback ConnectionCallback

	running   bool
	startTime time.Time

	ctx    context.Context
	cancel context.CancelFunc
	wg     sync.WaitGroup

	mu sync.RWMutex
}

// New creates a new reactor with the given configuration.
func New(config *Config) *Reactor {
	return &Reactor{
		config: config,
		peers:  make(map[string]*Peer),
	}
}

// Running returns true if the reactor is running.
func (r *Reactor) Running() bool {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.running
}

// Peers returns all configured peers.
func (r *Reactor) Peers() []*Peer {
	r.mu.RLock()
	defer r.mu.RUnlock()

	peers := make([]*Peer, 0, len(r.peers))
	for _, p := range r.peers {
		peers = append(peers, p)
	}
	return peers
}

// ListenAddr returns the listener's bound address.
func (r *Reactor) ListenAddr() net.Addr {
	r.mu.RLock()
	defer r.mu.RUnlock()

	if r.listener != nil {
		return r.listener.Addr()
	}
	return nil
}

// Stats returns current reactor statistics.
func (r *Reactor) Stats() *Stats {
	r.mu.RLock()
	defer r.mu.RUnlock()

	stats := &Stats{
		StartTime: r.startTime,
		PeerCount: len(r.peers),
	}
	if r.running {
		stats.Uptime = time.Since(r.startTime)
	}
	return stats
}

// SetConnectionCallback sets the callback for matched incoming connections.
func (r *Reactor) SetConnectionCallback(cb ConnectionCallback) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.connCallback = cb
}

// AddNeighbor adds a neighbor to the reactor.
func (r *Reactor) AddNeighbor(neighbor *Neighbor) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	key := neighbor.Address.String()
	if _, exists := r.peers[key]; exists {
		return ErrNeighborExists
	}

	peer := NewPeer(neighbor)
	r.peers[key] = peer

	// If reactor is running, start the peer
	if r.running && !neighbor.Passive {
		peer.StartWithContext(r.ctx)
	}

	return nil
}

// RemoveNeighbor removes a neighbor from the reactor.
func (r *Reactor) RemoveNeighbor(addr netip.Addr) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	key := addr.String()
	peer, exists := r.peers[key]
	if !exists {
		return ErrNeighborNotFound
	}

	// Stop peer if running
	peer.Stop()

	delete(r.peers, key)
	return nil
}

// Start begins the reactor with a background context.
func (r *Reactor) Start() error {
	return r.StartWithContext(context.Background())
}

// StartWithContext begins the reactor with the given context.
func (r *Reactor) StartWithContext(ctx context.Context) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if r.running {
		return ErrAlreadyRunning
	}

	r.ctx, r.cancel = context.WithCancel(ctx)
	r.startTime = time.Now()

	// Start listener
	if r.config.ListenAddr != "" {
		r.listener = NewListener(r.config.ListenAddr)
		r.listener.SetHandler(r.handleConnection)
		if err := r.listener.StartWithContext(r.ctx); err != nil {
			r.cancel()
			return err
		}
	}

	// Start signal handler
	r.signals = NewSignalHandler()
	r.signals.OnShutdown(func() {
		r.Stop()
	})
	r.signals.StartWithContext(r.ctx)

	// Start all non-passive peers
	for _, peer := range r.peers {
		if !peer.Neighbor().Passive {
			peer.StartWithContext(r.ctx)
		}
	}

	r.running = true

	// Monitor context for shutdown
	r.wg.Add(1)
	go r.monitor()

	return nil
}

// Stop signals the reactor to stop.
func (r *Reactor) Stop() {
	r.mu.Lock()
	cancel := r.cancel
	r.mu.Unlock()

	if cancel != nil {
		cancel()
	}
}

// Wait waits for the reactor to stop.
func (r *Reactor) Wait(ctx context.Context) error {
	done := make(chan struct{})
	go func() {
		r.wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

// monitor watches for shutdown and cleans up.
func (r *Reactor) monitor() {
	defer r.wg.Done()

	<-r.ctx.Done()

	r.cleanup()
}

// cleanup stops all components.
func (r *Reactor) cleanup() {
	r.mu.Lock()
	defer r.mu.Unlock()

	// Stop listener
	if r.listener != nil {
		r.listener.Stop()
		waitCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		_ = r.listener.Wait(waitCtx)
		cancel()
	}

	// Stop signal handler
	if r.signals != nil {
		r.signals.Stop()
		waitCtx, cancel := context.WithTimeout(context.Background(), time.Second)
		_ = r.signals.Wait(waitCtx)
		cancel()
	}

	// Stop all peers
	for _, peer := range r.peers {
		peer.Stop()
	}

	// Wait for all peers
	for _, peer := range r.peers {
		waitCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		_ = peer.Wait(waitCtx)
		cancel()
	}

	r.running = false
	r.cancel = nil
}

// handleConnection handles an incoming TCP connection.
func (r *Reactor) handleConnection(conn net.Conn) {
	remoteAddr, ok := conn.RemoteAddr().(*net.TCPAddr)
	if !ok {
		_ = conn.Close()
		return
	}
	peerIP, _ := netip.AddrFromSlice(remoteAddr.IP)
	peerIP = peerIP.Unmap() // Handle IPv4-mapped IPv6

	r.mu.RLock()
	peer, exists := r.peers[peerIP.String()]
	cb := r.connCallback
	r.mu.RUnlock()

	if !exists {
		// Unknown peer, close connection
		_ = conn.Close()
		return
	}

	neighbor := peer.Neighbor()

	// Call callback if set
	if cb != nil {
		cb(conn, neighbor)
		return
	}

	// Default: accept connection on peer's session
	// For passive peers, this triggers the session
	// TODO: integrate with peer/session for full handshake
	_ = conn.Close()
}
