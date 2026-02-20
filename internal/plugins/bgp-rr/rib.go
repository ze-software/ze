// Design: docs/architecture/core-design.md — route reflector plugin
//
// Package rr implements a BGP Route Server API plugin.
package bgp_rr

import (
	"maps"
	"sync"
)

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

// peerRIB holds routes for a single peer with its own lock.
// Operations on different peers proceed concurrently.
type peerRIB struct {
	mu     sync.Mutex
	routes map[string]*Route // routeKey → route
}

// RIB is the Route Server's internal Adj-RIB-In.
// It stores routes per peer for replay on session establishment.
// Top-level lock protects the peer map; per-peer locks protect route maps.
type RIB struct {
	mu    sync.Mutex
	peers map[string]*peerRIB
}

// NewRIB creates an empty RIB.
func NewRIB() *RIB {
	return &RIB{
		peers: make(map[string]*peerRIB),
	}
}

// getOrCreatePeerRIB returns the peerRIB for the given peer, creating one if needed.
func (r *RIB) getOrCreatePeerRIB(peerID string) *peerRIB {
	r.mu.Lock()
	defer r.mu.Unlock()
	pr := r.peers[peerID]
	if pr == nil {
		pr = &peerRIB{routes: make(map[string]*Route)}
		r.peers[peerID] = pr
	}
	return pr
}

// getPeerRIB returns the peerRIB for the given peer, or nil if not found.
func (r *RIB) getPeerRIB(peerID string) *peerRIB {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.peers[peerID]
}

// Insert adds or replaces a route from a peer.
// Returns the old route if one existed (for cleanup), nil otherwise.
func (r *RIB) Insert(peerID string, route *Route) *Route {
	pr := r.getOrCreatePeerRIB(peerID)

	pr.mu.Lock()
	defer pr.mu.Unlock()

	key := route.routeKey()
	old := pr.routes[key]
	pr.routes[key] = route
	return old
}

// Remove deletes a route from a peer.
// Returns the removed route, or nil if not found.
func (r *RIB) Remove(peerID, family, prefix string) *Route {
	pr := r.getPeerRIB(peerID)
	if pr == nil {
		return nil
	}

	pr.mu.Lock()
	defer pr.mu.Unlock()

	key := family + "|" + prefix
	old := pr.routes[key]
	delete(pr.routes, key)
	return old
}

// GetPeerRoutes returns all routes from a specific peer.
// Returns a new slice (safe for iteration while RIB is modified).
func (r *RIB) GetPeerRoutes(peerID string) []*Route {
	pr := r.getPeerRIB(peerID)
	if pr == nil {
		return nil
	}

	pr.mu.Lock()
	defer pr.mu.Unlock()

	if len(pr.routes) == 0 {
		return nil
	}

	result := make([]*Route, 0, len(pr.routes))
	for _, route := range pr.routes {
		result = append(result, route)
	}
	return result
}

// ClearPeer removes all routes from a peer.
// Returns the removed routes for cleanup/withdrawal.
// Locks the peerRIB to wait for any in-flight Insert to complete.
func (r *RIB) ClearPeer(peerID string) []*Route {
	r.mu.Lock()
	pr := r.peers[peerID]
	delete(r.peers, peerID)
	r.mu.Unlock()

	if pr == nil {
		return nil
	}

	pr.mu.Lock()
	defer pr.mu.Unlock()

	result := make([]*Route, 0, len(pr.routes))
	for _, route := range pr.routes {
		result = append(result, route)
	}
	return result
}

// GetAllPeers returns a copy of all peers and their routes.
// Used for replay on peer session establishment.
func (r *RIB) GetAllPeers() map[string][]*Route {
	// Snapshot peer references under top-level lock.
	r.mu.Lock()
	snapshot := make(map[string]*peerRIB, len(r.peers))
	maps.Copy(snapshot, r.peers)
	r.mu.Unlock()

	result := make(map[string][]*Route, len(snapshot))
	for id, pr := range snapshot {
		pr.mu.Lock()
		if len(pr.routes) > 0 {
			routes := make([]*Route, 0, len(pr.routes))
			for _, route := range pr.routes {
				routes = append(routes, route)
			}
			result[id] = routes
		}
		pr.mu.Unlock()
	}
	return result
}
