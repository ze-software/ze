// Design: docs/architecture/core-design.md — route reflector plugin
//
// Package rr implements a BGP Route Server API plugin.
package bgp_rr

import "sync"

// Route represents a cached route for replay.
type Route struct {
	MsgID  uint64 // Message ID for zero-copy forwarding
	Family string // Address family (e.g., "ipv4/unicast")
	Prefix string // NLRI prefix (e.g., "10.0.0.0/24")
}

// routeKey returns the unique key for a route within a peer's RIB.
func (r *Route) routeKey() string {
	return r.Family + "|" + r.Prefix
}

// RIB is the Route Server's internal Adj-RIB-In.
// It stores routes per peer for replay on session establishment.
type RIB struct {
	mu     sync.RWMutex
	routes map[string]map[string]*Route // peer → routeKey → route
}

// NewRIB creates an empty RIB.
func NewRIB() *RIB {
	return &RIB{
		routes: make(map[string]map[string]*Route),
	}
}

// Insert adds or replaces a route from a peer.
// Returns the old route if one existed (for cleanup), nil otherwise.
func (r *RIB) Insert(peerID string, route *Route) *Route {
	r.mu.Lock()
	defer r.mu.Unlock()

	if r.routes[peerID] == nil {
		r.routes[peerID] = make(map[string]*Route)
	}

	key := route.routeKey()
	old := r.routes[peerID][key]
	r.routes[peerID][key] = route
	return old
}

// Remove deletes a route from a peer.
// Returns the removed route, or nil if not found.
func (r *RIB) Remove(peerID, family, prefix string) *Route {
	r.mu.Lock()
	defer r.mu.Unlock()

	peerRoutes := r.routes[peerID]
	if peerRoutes == nil {
		return nil
	}

	key := family + "|" + prefix
	old := peerRoutes[key]
	delete(peerRoutes, key)

	// Clean up empty peer entry
	if len(peerRoutes) == 0 {
		delete(r.routes, peerID)
	}

	return old
}

// GetPeerRoutes returns all routes from a specific peer.
// Returns a new slice (safe for iteration while RIB is modified).
func (r *RIB) GetPeerRoutes(peerID string) []*Route {
	r.mu.RLock()
	defer r.mu.RUnlock()

	peerRoutes := r.routes[peerID]
	if peerRoutes == nil {
		return nil
	}

	result := make([]*Route, 0, len(peerRoutes))
	for _, route := range peerRoutes {
		result = append(result, route)
	}
	return result
}

// ClearPeer removes all routes from a peer.
// Returns the removed routes for cleanup/withdrawal.
func (r *RIB) ClearPeer(peerID string) []*Route {
	r.mu.Lock()
	defer r.mu.Unlock()

	peerRoutes := r.routes[peerID]
	if peerRoutes == nil {
		return nil
	}

	result := make([]*Route, 0, len(peerRoutes))
	for _, route := range peerRoutes {
		result = append(result, route)
	}

	delete(r.routes, peerID)
	return result
}

// GetAllPeers returns a copy of all peers and their routes.
// Used for replay on peer session establishment.
func (r *RIB) GetAllPeers() map[string][]*Route {
	r.mu.RLock()
	defer r.mu.RUnlock()

	result := make(map[string][]*Route, len(r.routes))
	for peerID, peerRoutes := range r.routes {
		routes := make([]*Route, 0, len(peerRoutes))
		for _, route := range peerRoutes {
			routes = append(routes, route)
		}
		result[peerID] = routes
	}
	return result
}
