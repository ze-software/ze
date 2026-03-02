// Design: docs/architecture/rib-transition.md — watchdog plugin extraction
// Overview: watchdog.go — plugin main and SDK lifecycle
// Related: server.go — command dispatch and state management
// Related: config.go — config tree parser

package bgp_watchdog

import (
	"errors"
	"sync"
)

// PoolSet manages named watchdog route pools.
// Provides a centralized store for routes that can be
// announced/withdrawn in bulk via "watchdog announce/withdraw <name>".
//
// Thread-safe for concurrent access from command handlers and state events.
type PoolSet struct {
	pools map[string]*RoutePool
	mu    sync.RWMutex
}

// NewPoolSet creates a new PoolSet.
func NewPoolSet() *PoolSet {
	return &PoolSet{
		pools: make(map[string]*RoutePool),
	}
}

// ErrRouteExists is returned when attempting to add a route with a key that already exists.
var ErrRouteExists = errors.New("route already exists in pool")

// AddRoute adds a route to a named pool, creating the pool if needed.
// Returns ErrRouteExists if a route with the same key already exists.
func (s *PoolSet) AddRoute(poolName string, entry *PoolEntry) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	pool, exists := s.pools[poolName]
	if !exists {
		pool = newRoutePool(poolName)
		s.pools[poolName] = pool
	}

	return pool.addRoute(entry)
}

// RemoveRoute removes a route from a pool by key.
// Returns the removed entry and true, or nil and false if not found.
// Cleans up empty pools automatically.
func (s *PoolSet) RemoveRoute(poolName, routeKey string) (*PoolEntry, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()

	pool, exists := s.pools[poolName]
	if !exists {
		return nil, false
	}

	removed := pool.removeRoute(routeKey)
	if removed == nil {
		return nil, false
	}

	if pool.isEmpty() {
		delete(s.pools, poolName)
	}

	return removed, true
}

// GetPool returns a pool by name, or nil if not found.
func (s *PoolSet) GetPool(name string) *RoutePool {
	s.mu.RLock()
	defer s.mu.RUnlock()

	return s.pools[name]
}

// PoolNames returns all pool names.
func (s *PoolSet) PoolNames() []string {
	s.mu.RLock()
	defer s.mu.RUnlock()

	names := make([]string, 0, len(s.pools))
	for name := range s.pools {
		names = append(names, name)
	}
	return names
}

// AnnouncePool marks all withdrawn routes as announced for a peer.
// Returns entries that transitioned from withdrawn to announced.
// Returns nil if pool doesn't exist.
func (s *PoolSet) AnnouncePool(poolName, peerAddr string) []*PoolEntry {
	s.mu.Lock()
	defer s.mu.Unlock()

	pool, exists := s.pools[poolName]
	if !exists {
		return nil
	}

	return pool.announceForPeer(peerAddr)
}

// WithdrawPool marks all announced routes as withdrawn for a peer.
// Returns entries that transitioned from announced to withdrawn.
// Returns nil if pool doesn't exist.
func (s *PoolSet) WithdrawPool(poolName, peerAddr string) []*PoolEntry {
	s.mu.Lock()
	defer s.mu.Unlock()

	pool, exists := s.pools[poolName]
	if !exists {
		return nil
	}

	return pool.withdrawForPeer(peerAddr)
}

// AnnouncedForPeer returns all routes in a pool that are announced for a peer.
func (s *PoolSet) AnnouncedForPeer(poolName, peerAddr string) []*PoolEntry {
	s.mu.RLock()
	defer s.mu.RUnlock()

	pool, exists := s.pools[poolName]
	if !exists {
		return nil
	}

	return pool.announcedForPeer(peerAddr)
}

// RoutePool holds routes for a named watchdog group.
// Each route tracks per-peer announced/withdrawn state.
type RoutePool struct {
	name   string
	routes map[string]*PoolEntry
	mu     sync.RWMutex
}

func newRoutePool(name string) *RoutePool {
	return &RoutePool{
		name:   name,
		routes: make(map[string]*PoolEntry),
	}
}

// Name returns the pool name.
func (p *RoutePool) Name() string {
	return p.name
}

// Routes returns all entries in the pool.
func (p *RoutePool) Routes() []*PoolEntry {
	p.mu.RLock()
	defer p.mu.RUnlock()

	entries := make([]*PoolEntry, 0, len(p.routes))
	for _, e := range p.routes {
		entries = append(entries, e)
	}
	return entries
}

func (p *RoutePool) isEmpty() bool {
	p.mu.RLock()
	defer p.mu.RUnlock()

	return len(p.routes) == 0
}

func (p *RoutePool) addRoute(entry *PoolEntry) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	if _, exists := p.routes[entry.Key]; exists {
		return ErrRouteExists
	}

	entry.pool = p
	p.routes[entry.Key] = entry
	return nil
}

func (p *RoutePool) removeRoute(routeKey string) *PoolEntry {
	p.mu.Lock()
	defer p.mu.Unlock()

	entry, exists := p.routes[routeKey]
	if !exists {
		return nil
	}
	delete(p.routes, routeKey)
	return entry
}

func (p *RoutePool) announceForPeer(peerAddr string) []*PoolEntry {
	p.mu.Lock()
	defer p.mu.Unlock()

	var announced []*PoolEntry
	for _, e := range p.routes {
		if !e.announced[peerAddr] {
			e.announced[peerAddr] = true
			announced = append(announced, e)
		}
	}
	return announced
}

func (p *RoutePool) withdrawForPeer(peerAddr string) []*PoolEntry {
	p.mu.Lock()
	defer p.mu.Unlock()

	var withdrawn []*PoolEntry
	for _, e := range p.routes {
		if e.announced[peerAddr] {
			e.announced[peerAddr] = false
			withdrawn = append(withdrawn, e)
		}
	}
	return withdrawn
}

func (p *RoutePool) announcedForPeer(peerAddr string) []*PoolEntry {
	p.mu.RLock()
	defer p.mu.RUnlock()

	var entries []*PoolEntry
	for _, e := range p.routes {
		if e.announced[peerAddr] {
			entries = append(entries, e)
		}
	}
	return entries
}

// PoolEntry is a route in a watchdog pool with per-peer state.
// Routes store pre-computed text commands for announce and withdraw.
// The announced map is protected by the parent RoutePool's mutex.
type PoolEntry struct {
	Key         string // Unique route identifier (prefix#pathid or rd:prefix#pathid)
	AnnounceCmd string // "update text ..." command for announcing
	WithdrawCmd string // "update text ..." command for withdrawing

	// initiallyAnnounced indicates whether this route should be sent on
	// first peer session establishment. Config routes without "withdraw true"
	// start as initially announced; withdrawn routes wait for explicit command.
	initiallyAnnounced bool

	announced map[string]bool // peerAddr → isAnnounced (protected by pool.mu)
	pool      *RoutePool      // back-pointer for locking
}

// NewPoolEntry creates a new pool entry with the given key and commands.
func NewPoolEntry(key, announceCmd, withdrawCmd string) *PoolEntry {
	return &PoolEntry{
		Key:         key,
		AnnounceCmd: announceCmd,
		WithdrawCmd: withdrawCmd,
		announced:   make(map[string]bool),
	}
}

// IsAnnounced returns true if the route is announced for the given peer.
// Thread-safe: acquires pool lock.
func (e *PoolEntry) IsAnnounced(peerAddr string) bool {
	e.pool.mu.RLock()
	defer e.pool.mu.RUnlock()

	return e.announced[peerAddr]
}

// AnnouncedPeers returns all peer addresses for which this route is announced.
// Thread-safe: acquires pool lock.
func (e *PoolEntry) AnnouncedPeers() []string {
	e.pool.mu.RLock()
	defer e.pool.mu.RUnlock()

	var peers []string
	for addr, ann := range e.announced {
		if ann {
			peers = append(peers, addr)
		}
	}
	return peers
}
