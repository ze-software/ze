// Design: docs/architecture/plugin/rib-storage-design.md -- ASPA route tracker for re-validation
// Overview: rpki.go -- plugin tracking routes for ASPA re-validation on cache change
// Related: aspa_cache.go -- cache whose changes trigger re-validation
package rpki

import "sync"

// routeKey uniquely identifies a tracked route.
type routeKey struct {
	peerAddr string
	family   string
	prefix   string
	pathID   uint32
}

// trackedRoute holds the data needed for ASPA re-validation.
type trackedRoute struct {
	key       routeKey
	peerName  string
	peerASN   uint32
	msgID     uint64
	path      []uint32 // owned copy of normalized AS_PATH
	aspaState uint8
}

// maxTrackedRoutes bounds tracker memory.
const maxTrackedRoutes = 1_000_000

// aSPATracker tracks active routes with their normalized AS_PATH for ASPA re-validation.
// Maintains a reverse index (customer-AS -> route keys) for efficient re-validation
// when ASPA cache data changes.
type aSPATracker struct {
	routes       map[routeKey]*trackedRoute
	reverseIndex map[uint32]map[routeKey]struct{}
	mu           sync.Mutex
}

// newASPATracker creates an empty route tracker.
func newASPATracker() *aSPATracker {
	return &aSPATracker{
		routes:       make(map[routeKey]*trackedRoute),
		reverseIndex: make(map[uint32]map[routeKey]struct{}),
	}
}

// Track adds or updates a tracked route. Stores an owned copy of the path.
// The path slice MUST NOT be retained from WireUpdate buffers.
func (t *aSPATracker) Track(rt trackedRoute) {
	t.mu.Lock()
	defer t.mu.Unlock()

	if existing, ok := t.routes[rt.key]; ok {
		t.removeFromIndexLocked(existing)
	} else if len(t.routes) >= maxTrackedRoutes {
		logger().Warn("aspa: tracker full, dropping route",
			"peer", rt.key.peerAddr, "family", rt.key.family, "prefix", rt.key.prefix)
		return
	}

	t.routes[rt.key] = &rt
	t.addToIndexLocked(&rt)
}

// Remove deletes a tracked route and its reverse index entries.
func (t *aSPATracker) Remove(key routeKey) {
	t.mu.Lock()
	defer t.mu.Unlock()

	existing, ok := t.routes[key]
	if !ok {
		return
	}
	t.removeFromIndexLocked(existing)
	delete(t.routes, key)
}

// revalidate re-verifies routes affected by ASPA cache changes.
// Returns routes whose ASPA state changed (for event emission).
func (t *aSPATracker) revalidate(cache *aSPACache, changedCustomers []uint32) []trackedRoute {
	t.mu.Lock()
	defer t.mu.Unlock()

	affected := make(map[routeKey]struct{})
	for _, customerAS := range changedCustomers {
		if keys, ok := t.reverseIndex[customerAS]; ok {
			for k := range keys {
				affected[k] = struct{}{}
			}
		}
	}

	var changed []trackedRoute
	for key := range affected {
		rt, ok := t.routes[key]
		if !ok {
			continue
		}
		newState := verifyASPA(cache, rt.path)
		if newState != rt.aspaState {
			rt.aspaState = newState
			changed = append(changed, *rt)
		}
	}

	return changed
}

// Count returns the number of tracked routes.
func (t *aSPATracker) count() int {
	t.mu.Lock()
	defer t.mu.Unlock()
	return len(t.routes)
}

// addToIndexLocked adds a route to the reverse index for all ASNs in its path.
func (t *aSPATracker) addToIndexLocked(rt *trackedRoute) {
	for _, asn := range rt.path {
		if t.reverseIndex[asn] == nil {
			t.reverseIndex[asn] = make(map[routeKey]struct{})
		}
		t.reverseIndex[asn][rt.key] = struct{}{}
	}
}

// removeFromIndexLocked removes a route from the reverse index.
func (t *aSPATracker) removeFromIndexLocked(rt *trackedRoute) {
	for _, asn := range rt.path {
		if keys, ok := t.reverseIndex[asn]; ok {
			delete(keys, rt.key)
			if len(keys) == 0 {
				delete(t.reverseIndex, asn)
			}
		}
	}
}
