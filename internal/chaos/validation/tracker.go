// Design: docs/architecture/chaos-web-dashboard.md — property-based validation

package validation

import (
	"net/netip"
	"sync"
)

// Tracker records the actual routes each peer has received from the RR.
// It is safe for concurrent use from multiple peer reader goroutines.
type Tracker struct {
	mu    sync.Mutex
	peers []*PrefixSet
}

// NewTracker creates a new tracker for n peers.
func NewTracker(n int) *Tracker {
	peers := make([]*PrefixSet, n)
	for i := range n {
		peers[i] = NewPrefixSet()
	}
	return &Tracker{peers: peers}
}

// RecordReceive records that a peer received a route from the RR.
func (t *Tracker) RecordReceive(peer int, prefix netip.Prefix) {
	t.recordReceive(peer, makeRouteKey(prefix, ""))
}

func (t *Tracker) recordReceive(peer int, route routeKey) {
	t.mu.Lock()
	defer t.mu.Unlock()
	if peer >= 0 && peer < len(t.peers) {
		t.peers[peer].m[route] = struct{}{}
	}
}

// RecordWithdraw records that a peer received a withdrawal from the RR.
func (t *Tracker) RecordWithdraw(peer int, prefix netip.Prefix) {
	t.recordWithdraw(peer, makeRouteKey(prefix, ""))
}

func (t *Tracker) recordWithdraw(peer int, route routeKey) {
	t.mu.Lock()
	defer t.mu.Unlock()
	if peer >= 0 && peer < len(t.peers) {
		delete(t.peers[peer].m, route)
	}
}

// ClearPeer removes all received routes for a peer (e.g. on disconnect).
func (t *Tracker) ClearPeer(peer int) {
	t.mu.Lock()
	defer t.mu.Unlock()
	if peer >= 0 && peer < len(t.peers) {
		t.peers[peer] = NewPrefixSet()
	}
}

// ActualRoutes returns a snapshot of the routes peer has received.
// The returned PrefixSet is a copy — safe to read without holding the lock.
func (t *Tracker) ActualRoutes(peer int) *PrefixSet {
	t.mu.Lock()
	defer t.mu.Unlock()
	if peer < 0 || peer >= len(t.peers) {
		return NewPrefixSet()
	}
	// Return a copy to avoid holding the lock while caller iterates.
	snapshot := NewPrefixSet()
	snapshot.addAll(t.peers[peer])
	return snapshot
}
