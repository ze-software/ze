package rib

import (
	"sync"
)

// IncomingRIB (Adj-RIB-In) stores routes received from peers.
//
// Structure: peer -> nlri_index -> route
// Each peer has its own namespace, allowing the same prefix from multiple peers.
type IncomingRIB struct {
	mu sync.RWMutex

	// routes maps peer ID -> route index -> route
	routes map[string]map[string]*Route
}

// NewIncomingRIB creates a new Adj-RIB-In.
func NewIncomingRIB() *IncomingRIB {
	return &IncomingRIB{
		routes: make(map[string]map[string]*Route),
	}
}

// Insert adds or replaces a route from a peer.
// Returns the old route if one existed (for cleanup), nil otherwise.
func (r *IncomingRIB) Insert(peerID string, route *Route) *Route {
	r.mu.Lock()
	defer r.mu.Unlock()

	// Get or create peer's route table
	peerRoutes, ok := r.routes[peerID]
	if !ok {
		peerRoutes = make(map[string]*Route)
		r.routes[peerID] = peerRoutes
	}

	// Route index as string key
	idx := string(route.Index())

	// Check for existing route
	old := peerRoutes[idx]

	// Store new route
	peerRoutes[idx] = route

	return old
}

// Get looks up a route by peer ID and route index.
// Returns nil if not found.
func (r *IncomingRIB) Get(peerID string, index []byte) *Route {
	r.mu.RLock()
	defer r.mu.RUnlock()

	peerRoutes, ok := r.routes[peerID]
	if !ok {
		return nil
	}

	return peerRoutes[string(index)]
}

// Remove removes a route by peer ID and route index.
// Returns the removed route, or nil if not found.
func (r *IncomingRIB) Remove(peerID string, index []byte) *Route {
	r.mu.Lock()
	defer r.mu.Unlock()

	peerRoutes, ok := r.routes[peerID]
	if !ok {
		return nil
	}

	idx := string(index)
	route := peerRoutes[idx]
	if route != nil {
		delete(peerRoutes, idx)
	}

	return route
}

// ClearPeer removes all routes from a peer (session teardown).
// Returns all removed routes for cleanup.
func (r *IncomingRIB) ClearPeer(peerID string) []*Route {
	r.mu.Lock()
	defer r.mu.Unlock()

	peerRoutes, ok := r.routes[peerID]
	if !ok {
		return nil
	}

	routes := make([]*Route, 0, len(peerRoutes))
	for _, route := range peerRoutes {
		routes = append(routes, route)
	}

	delete(r.routes, peerID)

	return routes
}

// ClearAll removes all routes from all peers.
// Returns count of routes removed.
func (r *IncomingRIB) ClearAll() int {
	r.mu.Lock()
	defer r.mu.Unlock()

	count := 0
	for _, peerRoutes := range r.routes {
		count += len(peerRoutes)
	}

	r.routes = make(map[string]map[string]*Route)

	return count
}

// GetPeerRoutes returns all routes from a peer.
// Returns a copy to avoid holding the lock.
func (r *IncomingRIB) GetPeerRoutes(peerID string) []*Route {
	r.mu.RLock()
	defer r.mu.RUnlock()

	peerRoutes, ok := r.routes[peerID]
	if !ok {
		return nil
	}

	routes := make([]*Route, 0, len(peerRoutes))
	for _, route := range peerRoutes {
		routes = append(routes, route)
	}

	return routes
}

// Stats returns statistics about the IncomingRIB.
func (r *IncomingRIB) Stats() IncomingRIBStats {
	r.mu.RLock()
	defer r.mu.RUnlock()

	stats := IncomingRIBStats{
		PeerCount: len(r.routes),
	}

	for _, peerRoutes := range r.routes {
		stats.RouteCount += len(peerRoutes)
	}

	return stats
}

// IncomingRIBStats holds statistics about the IncomingRIB.
type IncomingRIBStats struct {
	PeerCount  int
	RouteCount int
}
