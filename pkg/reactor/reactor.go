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

	"github.com/exa-networks/zebgp/pkg/api"
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

	// APISocketPath is the path to the Unix socket for API communication.
	// If empty, API server is not started.
	APISocketPath string

	// APIProcesses defines external processes for API communication.
	APIProcesses []APIProcessConfig
}

// APIProcessConfig holds external process configuration for the API.
type APIProcessConfig struct {
	Name    string
	Run     string
	Encoder string
	Respawn bool
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
//   - API server for external communication
type Reactor struct {
	config *Config

	peers    map[string]*Peer // keyed by neighbor address
	listener *Listener
	signals  *SignalHandler
	api      *api.Server // API server for CLI and external processes

	connCallback ConnectionCallback

	running   bool
	startTime time.Time

	ctx    context.Context
	cancel context.CancelFunc
	wg     sync.WaitGroup

	mu sync.RWMutex
}

// reactorAPIAdapter implements api.ReactorInterface for the Reactor.
type reactorAPIAdapter struct {
	r *Reactor
}

// Peers returns peer information for the API.
func (a *reactorAPIAdapter) Peers() []api.PeerInfo {
	a.r.mu.RLock()
	defer a.r.mu.RUnlock()

	result := make([]api.PeerInfo, 0, len(a.r.peers))
	for _, p := range a.r.peers {
		n := p.Neighbor()
		info := api.PeerInfo{
			Address:      n.Address,
			LocalAddress: netip.Addr{}, // TODO: get from session
			LocalAS:      n.LocalAS,
			PeerAS:       n.PeerAS,
			RouterID:     n.RouterID,
			State:        p.State().String(),
		}
		if p.State() == PeerStateEstablished {
			info.Uptime = time.Since(a.r.startTime) // TODO: track per-peer
		}
		result = append(result, info)
	}
	return result
}

// Stats returns reactor statistics for the API.
func (a *reactorAPIAdapter) Stats() api.ReactorStats {
	stats := a.r.Stats()
	return api.ReactorStats{
		StartTime: stats.StartTime,
		Uptime:    stats.Uptime,
		PeerCount: stats.PeerCount,
	}
}

// Stop signals the reactor to stop.
func (a *reactorAPIAdapter) Stop() {
	a.r.Stop()
}

// AnnounceRoute announces a route to matching peers.
// TODO: Implement when RIB integration is complete.
func (a *reactorAPIAdapter) AnnounceRoute(_ string, _ api.RouteSpec) error {
	return errors.New("not implemented")
}

// WithdrawRoute withdraws a route from matching peers.
// TODO: Implement when RIB integration is complete.
func (a *reactorAPIAdapter) WithdrawRoute(_ string, _ netip.Prefix) error {
	return errors.New("not implemented")
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
	if r.running {
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

	// Start API server if configured
	if r.config.APISocketPath != "" {
		apiConfig := &api.ServerConfig{
			SocketPath: r.config.APISocketPath,
		}
		// Convert process configs
		for _, pc := range r.config.APIProcesses {
			apiConfig.Processes = append(apiConfig.Processes, api.ProcessConfig{
				Name:    pc.Name,
				Run:     pc.Run,
				Encoder: pc.Encoder,
				Respawn: pc.Respawn,
			})
		}
		r.api = api.NewServer(apiConfig, &reactorAPIAdapter{r})
		if err := r.api.StartWithContext(r.ctx); err != nil {
			if r.listener != nil {
				r.listener.Stop()
			}
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

	// Start all peers (passive peers wait for incoming connections).
	for _, peer := range r.peers {
		peer.StartWithContext(r.ctx)
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

	// Stop API server
	if r.api != nil {
		r.api.Stop()
		waitCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		_ = r.api.Wait(waitCtx)
		cancel()
	}

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

	// Accept connection on peer's session.
	// For passive peers, this triggers the BGP handshake.
	if err := peer.AcceptConnection(conn); err != nil {
		_ = conn.Close()
	}
}
