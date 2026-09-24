// Design: docs/architecture/plugin/rib-storage-design.md -- RFC 6811 Section 4 origin re-validation
// Overview: rpki.go -- plugin tracking routes for origin re-validation on ROA/VRP cache change
// Related: roa_cache.go -- the ROA cache whose VRP changes trigger re-validation
// RFC: rfc/short/rfc6811.md -- Section 4: when a mapping is added or deleted the implementation
// MUST re-validate any affected prefixes and run the decision process.
package rpki

import "sync"

// originRoute retains the latest origin and ASPA verdicts for cache re-validation.
// Both cache callbacks update this record before producing a new decision.
type originRoute struct {
	originAS uint32
	// peerGroup is the group the source session belongs to, empty for a standalone
	// peer. Re-validation re-dispatches a decision with no UPDATE in hand, and
	// buildDecisions resolves a session created from a listen-range group by its
	// group's name, so the identity is stored rather than re-derived. Not part of
	// routeKey: it identifies the SESSION's config, never the route.
	peerGroup string
	peerName  string
	peerASN   uint32
	// unavailable is the cache availability reported with this route's last
	// event. First synchronization can change it without changing either verdict.
	unavailable bool
	state       uint8
	aspaState   uint8
	msgID       uint64
	// blackhole records that the received UPDATE carried the RFC 7999 BLACKHOLE
	// community. It is a property of the announcement, so it does not change
	// when the VRP set does, and re-validation carries it through unchanged. It
	// is stored because re-validation has no UPDATE in hand to re-read it from.
	blackhole bool
}

// originRevalidation describes a verdict, exemption, or availability transition.
type originRevalidation struct {
	key routeKey
	originRoute
	decisionRequired bool
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

// Track retains the received route's identity and the verdict last published for
// it. Cache callbacks have no UPDATE available to recover event metadata.
func (t *originTracker) Track(key routeKey, route originRoute) {
	t.mu.Lock()
	defer t.mu.Unlock()
	if current := t.routes[key]; current != nil && route.msgID != 0 && current.msgID > route.msgID {
		return
	}
	if _, ok := t.routes[key]; !ok && len(t.routes) >= maxTrackedRoutes {
		logger().Warn("rpki: origin tracker full, dropping route",
			"peer", key.peerAddr, "family", key.family, "prefix", key.prefix)
		return
	}
	t.routes[key] = &route
}

// Remove deletes a withdrawal's generation, never a newer config snapshot.
func (t *originTracker) Remove(key routeKey, msgID uint64) {
	t.mu.Lock()
	defer t.mu.Unlock()
	if current := t.routes[key]; current != nil && msgID != 0 && current.msgID > msgID {
		return
	}
	delete(t.routes, key)
}

func (t *originTracker) removePeer(peerAddr string) {
	t.mu.Lock()
	defer t.mu.Unlock()
	for key := range t.routes {
		if key.peerAddr == peerAddr {
			delete(t.routes, key)
		}
	}
}

func (t *originTracker) clear() {
	t.mu.Lock()
	defer t.mu.Unlock()
	clear(t.routes)
}

// revalidate updates origin states and returns routes whose eligibility must be
// reconsidered. A BLACKHOLE route can remain Invalid while its length-only
// exemption changes, so its current covering authorization must also be checked.
func (t *originTracker) revalidate(cache *ROACache) []originRevalidation {
	t.mu.Lock()
	defer t.mu.Unlock()
	var changed []originRevalidation
	v4, v6 := cache.Count()
	unavailable := v4+v6 == 0
	for key, rt := range t.routes {
		newState := cache.Validate(key.prefix, rt.originAS)
		reconsider := newState != rt.state
		if rt.blackhole {
			if newState == ValidationInvalid {
				reconsider = true
			}
		}
		if !reconsider && unavailable == rt.unavailable {
			continue
		}
		rt.state = newState
		rt.unavailable = unavailable
		changed = append(changed, originRevalidation{
			key: key, originRoute: *rt, decisionRequired: reconsider,
		})
	}
	return changed
}

// updateASPA joins a changed path verdict with the current origin verdict.
// A callback for a replaced UPDATE cannot overwrite its successor's state.
func (t *originTracker) updateASPA(cache *ROACache, key routeKey, msgID uint64, aspaState uint8) (originRevalidation, bool) {
	t.mu.Lock()
	defer t.mu.Unlock()
	rt, ok := t.routes[key]
	if !ok || rt.msgID != msgID {
		return originRevalidation{}, false
	}
	rt.aspaState = aspaState
	rt.state = cache.Validate(key.prefix, rt.originAS)
	v4, v6 := cache.Count()
	rt.unavailable = v4+v6 == 0
	return originRevalidation{key: key, originRoute: *rt}, true
}

// updateResults includes unchanged siblings of every affected UPDATE. Sending
// only changed prefixes would consume the decorator's primary with a partial
// verdict set and leave later sibling events orphaned.
func (t *originTracker) updateResults(affected map[rpkiUpdateKey]struct{}) map[rpkiUpdateKey]*rpkiUpdateResults {
	t.mu.Lock()
	defer t.mu.Unlock()
	updates := make(map[rpkiUpdateKey]*rpkiUpdateResults, len(affected))
	for key, route := range t.routes {
		if _, ok := affected[rpkiUpdateKey{peerAddr: key.peerAddr, msgID: route.msgID}]; ok {
			collectRPKIUpdate(updates, key, *route)
		}
	}
	return updates
}

// count returns the number of tracked routes.
func (t *originTracker) count() int {
	t.mu.Lock()
	defer t.mu.Unlock()
	return len(t.routes)
}
