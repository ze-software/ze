// Design: docs/architecture/plugin/rib-storage-design.md -- RFC 6811 Section 4 origin re-validation
// Overview: rpki.go -- plugin tracking routes for origin re-validation on ROA/VRP cache change
// Related: roa_cache.go -- the ROA cache whose VRP changes trigger re-validation
// RFC: rfc/short/rfc6811.md -- Section 4: when a mapping is added or deleted the implementation
// MUST re-validate any affected prefixes and run the decision process.
package rpki

import "sync"

// originRoute holds the minimum needed to re-validate a route's RFC 6811 origin-validation state
// against the ROA cache. aspaState is the receive-time ASPA snapshot, carried through so a
// re-dispatched decision preserves the ASPA component (ASPA re-validation is handled separately
// on ASPA cache changes; see ASPATracker).
type originRoute struct {
	originAS  uint32
	state     uint8
	aspaState uint8
}

// originRevalidation is a route whose origin-validation state changed on a VRP update.
type originRevalidation struct {
	key       routeKey
	state     uint8
	aspaState uint8
}

// OriginTracker records active routes and their last origin-validation state so they can be
// re-validated when the ROA cache (VRP set) changes (RFC 6811 Section 4). Unlike ASPATracker it
// keeps no reverse index: a VRP change can affect any covering prefix, so re-validation re-runs
// Validate over every tracked route (a correct superset of "affected prefixes").
type OriginTracker struct {
	routes map[routeKey]*originRoute
	mu     sync.Mutex
}

// newOriginTracker creates an empty origin route tracker.
func newOriginTracker() *OriginTracker {
	return &OriginTracker{routes: make(map[routeKey]*originRoute)}
}

// Track records (or updates) a route's origin AS, current validation state, and ASPA snapshot.
func (t *OriginTracker) Track(key routeKey, originAS uint32, state, aspaState uint8) {
	t.mu.Lock()
	defer t.mu.Unlock()
	if _, ok := t.routes[key]; !ok && len(t.routes) >= maxTrackedRoutes {
		logger().Warn("rpki: origin tracker full, dropping route",
			"peer", key.peerAddr, "family", key.family, "prefix", key.prefix)
		return
	}
	t.routes[key] = &originRoute{originAS: originAS, state: state, aspaState: aspaState}
}

// Remove deletes a tracked route.
func (t *OriginTracker) Remove(key routeKey) {
	t.mu.Lock()
	defer t.mu.Unlock()
	delete(t.routes, key)
}

// revalidate re-runs origin validation over every tracked route against the current ROA cache and
// returns the routes whose validation state changed (with their updated state and ASPA snapshot),
// updating the stored state in place.
func (t *OriginTracker) revalidate(cache *ROACache) []originRevalidation {
	t.mu.Lock()
	defer t.mu.Unlock()
	var changed []originRevalidation
	for key, rt := range t.routes {
		newState := cache.Validate(key.prefix, rt.originAS)
		if newState != rt.state {
			rt.state = newState
			changed = append(changed, originRevalidation{key: key, state: newState, aspaState: rt.aspaState})
		}
	}
	return changed
}

// count returns the number of tracked routes.
func (t *OriginTracker) count() int {
	t.mu.Lock()
	defer t.mu.Unlock()
	return len(t.routes)
}
