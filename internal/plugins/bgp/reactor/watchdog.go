// Package reactor implements the BGP reactor - the main orchestrator
// that manages peer sessions, connections, and signal handling.
package reactor

import (
	"errors"
	"sync"
)

// WatchdogManager manages global watchdog pools.
// Provides a centralized store for API-created routes that can be
// announced/withdrawn in bulk via "watchdog announce/withdraw <name>".
//
// Thread-safe for concurrent access from API handlers and peer goroutines.
type WatchdogManager struct {
	pools map[string]*WatchdogPool
	mu    sync.RWMutex
}

// NewWatchdogManager creates a new WatchdogManager.
func NewWatchdogManager() *WatchdogManager {
	return &WatchdogManager{
		pools: make(map[string]*WatchdogPool),
	}
}

// ErrRouteExists is returned when attempting to add a route with a key that already exists.
var ErrRouteExists = errors.New("route already exists in pool")

// AddRoute adds a route to a named pool, creating the pool if needed.
// Returns the PoolRoute for further state manipulation.
// Returns ErrRouteExists if a route with the same key already exists.
// To update a route, remove it first then add the new version.
func (m *WatchdogManager) AddRoute(poolName string, route StaticRoute) (*PoolRoute, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	pool, exists := m.pools[poolName]
	if !exists {
		pool = newWatchdogPool(poolName)
		m.pools[poolName] = pool
	}

	return pool.addRoute(route)
}

// RemoveRoute removes a route from a pool by its route key.
// Returns true if the route was found and removed.
// If this was the last route in the pool, the pool is also removed.
func (m *WatchdogManager) RemoveRoute(poolName, routeKey string) bool {
	_, removed := m.RemoveAndGetRoute(poolName, routeKey)
	return removed
}

// RemoveAndGetRoute atomically removes a route and returns its data.
// Returns (route data, true) if found and removed, (nil, false) otherwise.
// This is used to get route info (prefix, announced peers) for sending withdrawals.
// If this was the last route in the pool, the pool is also removed.
func (m *WatchdogManager) RemoveAndGetRoute(poolName, routeKey string) (*PoolRoute, bool) {
	m.mu.Lock()
	defer m.mu.Unlock()

	pool, exists := m.pools[poolName]
	if !exists {
		return nil, false
	}

	removed := pool.removeAndGetRoute(routeKey)
	if removed == nil {
		return nil, false
	}

	// Clean up empty pool
	if pool.isEmpty() {
		delete(m.pools, poolName)
	}

	return removed, true
}

// GetPool returns a pool by name, or nil if not found.
func (m *WatchdogManager) GetPool(name string) *WatchdogPool {
	m.mu.RLock()
	defer m.mu.RUnlock()

	return m.pools[name]
}

// PoolNames returns all pool names.
func (m *WatchdogManager) PoolNames() []string {
	m.mu.RLock()
	defer m.mu.RUnlock()

	names := make([]string, 0, len(m.pools))
	for name := range m.pools {
		names = append(names, name)
	}
	return names
}

// AnnouncePool marks all withdrawn routes as announced for a peer.
// Returns the routes that were transitioned from withdrawn → announced.
// Returns nil if pool doesn't exist.
func (m *WatchdogManager) AnnouncePool(poolName, peerAddr string) []*PoolRoute {
	m.mu.Lock()
	defer m.mu.Unlock()

	pool, exists := m.pools[poolName]
	if !exists {
		return nil
	}

	return pool.announceForPeer(peerAddr)
}

// WithdrawPool marks all announced routes as withdrawn for a peer.
// Returns the routes that were transitioned from announced → withdrawn.
// Returns nil if pool doesn't exist.
func (m *WatchdogManager) WithdrawPool(poolName, peerAddr string) []*PoolRoute {
	m.mu.Lock()
	defer m.mu.Unlock()

	pool, exists := m.pools[poolName]
	if !exists {
		return nil
	}

	return pool.withdrawForPeer(peerAddr)
}

// WatchdogPool holds routes for a named watchdog group.
// Each route tracks per-peer announced state.
type WatchdogPool struct {
	name   string
	routes map[string]*PoolRoute // routeKey → route
	mu     sync.RWMutex
}

// newWatchdogPool creates a new pool with the given name.
func newWatchdogPool(name string) *WatchdogPool {
	return &WatchdogPool{
		name:   name,
		routes: make(map[string]*PoolRoute),
	}
}

// Name returns the pool name.
func (p *WatchdogPool) Name() string {
	return p.name
}

// isEmpty returns true if the pool has no routes.
// Internal method, caller holds m.mu.
func (p *WatchdogPool) isEmpty() bool {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return len(p.routes) == 0
}

// Routes returns all routes in the pool.
func (p *WatchdogPool) Routes() []*PoolRoute {
	p.mu.RLock()
	defer p.mu.RUnlock()

	routes := make([]*PoolRoute, 0, len(p.routes))
	for _, r := range p.routes {
		routes = append(routes, r)
	}
	return routes
}

// addRoute adds a route to the pool. Internal method, caller holds m.mu.
// Returns ErrRouteExists if a route with the same key already exists.
func (p *WatchdogPool) addRoute(route StaticRoute) (*PoolRoute, error) {
	p.mu.Lock()
	defer p.mu.Unlock()

	key := route.RouteKey()
	if _, exists := p.routes[key]; exists {
		return nil, ErrRouteExists
	}

	pr := &PoolRoute{
		StaticRoute: route,
		announced:   make(map[string]bool),
		pool:        p,
	}
	p.routes[key] = pr
	return pr, nil
}

// removeAndGetRoute removes a route and returns it. Internal method, caller holds m.mu.
func (p *WatchdogPool) removeAndGetRoute(routeKey string) *PoolRoute {
	p.mu.Lock()
	defer p.mu.Unlock()

	route, exists := p.routes[routeKey]
	if !exists {
		return nil
	}
	delete(p.routes, routeKey)
	return route
}

// announceForPeer marks all withdrawn routes as announced for a peer.
// Returns routes that transitioned. Internal method, caller holds m.mu.
func (p *WatchdogPool) announceForPeer(peerAddr string) []*PoolRoute {
	p.mu.Lock()
	defer p.mu.Unlock()

	var announced []*PoolRoute
	for _, pr := range p.routes {
		if !pr.announced[peerAddr] {
			pr.announced[peerAddr] = true
			announced = append(announced, pr)
		}
	}
	return announced
}

// withdrawForPeer marks all announced routes as withdrawn for a peer.
// Returns routes that transitioned. Internal method, caller holds m.mu.
func (p *WatchdogPool) withdrawForPeer(peerAddr string) []*PoolRoute {
	p.mu.Lock()
	defer p.mu.Unlock()

	var withdrawn []*PoolRoute
	for _, pr := range p.routes {
		if pr.announced[peerAddr] {
			pr.announced[peerAddr] = false
			withdrawn = append(withdrawn, pr)
		}
	}
	return withdrawn
}

// PoolRoute is a route in a watchdog pool with per-peer state.
// The announced map is protected by the parent WatchdogPool's mutex.
// External callers must use IsAnnounced/SetAnnounced which acquire pool lock.
type PoolRoute struct {
	StaticRoute
	announced map[string]bool // peerAddr → isAnnounced (protected by pool.mu)
	pool      *WatchdogPool   // back-pointer for locking
}

// IsAnnounced returns true if the route is announced for the given peer.
// Thread-safe: acquires pool lock.
func (pr *PoolRoute) IsAnnounced(peerAddr string) bool {
	pr.pool.mu.RLock()
	defer pr.pool.mu.RUnlock()
	return pr.announced[peerAddr]
}

// SetAnnounced sets the announced state for a peer.
// Thread-safe: acquires pool lock.
func (pr *PoolRoute) SetAnnounced(peerAddr string, announced bool) {
	pr.pool.mu.Lock()
	defer pr.pool.mu.Unlock()
	pr.announced[peerAddr] = announced
}
