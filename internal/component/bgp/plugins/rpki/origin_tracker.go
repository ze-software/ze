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
	originAS uint32
	// peerGroup is the group the source session belongs to, empty for a standalone
	// peer. Re-validation re-dispatches a decision with no UPDATE in hand, and
	// buildDecisions resolves a session created from a listen-range group by its
	// group's name, so the identity is stored rather than re-derived. Not part of
	// routeKey: it identifies the SESSION's config, never the route.
	peerGroup string
	state     uint8
	aspaState uint8
	// blackhole records that the received UPDATE carried the RFC 7999 BLACKHOLE
	// community. It is a property of the announcement, so it does not change
	// when the VRP set does, and re-validation carries it through unchanged. It
	// is stored because re-validation has no UPDATE in hand to re-read it from.
	blackhole bool
}

// originRevalidation is a route whose origin-validation state changed on a VRP update.
type originRevalidation struct {
	key       routeKey
	peerGroup string
	state     uint8
	aspaState uint8
	originAS  uint32
	blackhole bool
}

// originTracker records active routes and their last origin-validation state so they can be
// re-validated when the ROA cache (VRP set) changes (RFC 6811 Section 4). Unlike ASPATracker it
// keeps no reverse index: a VRP change can affect any covering prefix, so re-validation re-runs
// Validate over every tracked route (a correct superset of "affected prefixes").
type originTracker struct {
	routes map[routeKey]*originRoute
	mu     sync.Mutex
}

// newOriginTracker creates an empty origin route tracker.
func newOriginTracker() *originTracker {
	return &originTracker{routes: make(map[routeKey]*originRoute)}
}

// Track records (or updates) a route's origin AS, the group of the session it came
// from, the current validation state, the ASPA snapshot, and whether the
// announcement carried the RFC 7999 BLACKHOLE community.
func (t *originTracker) Track(key routeKey, peerGroup string, originAS uint32, state, aspaState uint8, blackhole bool) {
	t.mu.Lock()
	defer t.mu.Unlock()
	if _, ok := t.routes[key]; !ok && len(t.routes) >= maxTrackedRoutes {
		logger().Warn("rpki: origin tracker full, dropping route",
			"peer", key.peerAddr, "family", key.family, "prefix", key.prefix)
		return
	}
	t.routes[key] = &originRoute{
		originAS: originAS, peerGroup: peerGroup,
		state: state, aspaState: aspaState, blackhole: blackhole,
	}
}

// Remove deletes a tracked route.
func (t *originTracker) Remove(key routeKey) {
	t.mu.Lock()
	defer t.mu.Unlock()
	delete(t.routes, key)
}

// revalidate re-runs origin validation over every tracked route against the current ROA cache and
// returns the routes whose validation state changed (with their updated state and ASPA snapshot),
// updating the stored state in place.
func (t *originTracker) revalidate(cache *ROACache) []originRevalidation {
	t.mu.Lock()
	defer t.mu.Unlock()
	var changed []originRevalidation
	for key, rt := range t.routes {
		newState := cache.Validate(key.prefix, rt.originAS)
		if newState != rt.state {
			rt.state = newState
			changed = append(changed, originRevalidation{
				key: key, peerGroup: rt.peerGroup, state: newState, aspaState: rt.aspaState,
				originAS: rt.originAS, blackhole: rt.blackhole,
			})
		}
	}
	return changed
}

// count returns the number of tracked routes.
func (t *originTracker) count() int {
	t.mu.Lock()
	defer t.mu.Unlock()
	return len(t.routes)
}
