// Design: docs/functional-tests.md -- fakeredist current-set + replay-on-request
//
// routeStore tracks the current set of routes fakeredist has emitted so it can
// re-emit them on a redistribute ReplayRequest, mirroring the real producers'
// current-set maps (static routeManager.routes, connected routeObserver.prefixes,
// l2tp subscriberRouteObserver.records). This gives the late-join `.ci` tests an
// in-process producer that answers a peer-up replay.

package fakeredist

import (
	"net/netip"
	"sync"

	"github.com/ze-software/ze/internal/core/family"
	"github.com/ze-software/ze/internal/core/redistevents"
)

// storeKey identifies a tracked route by family and prefix. Both fields are
// comparable value types, so the key works directly as a map key.
type storeKey struct {
	fam    family.Family
	prefix netip.Prefix
}

// storeEntry is one tracked route (family, prefix, next-hop) returned by a
// snapshot for replay.
type storeEntry struct {
	fam    family.Family
	prefix netip.Prefix
	nh     netip.Addr
}

// routeStore is the mutex-guarded current set. An add records the route; a
// remove drops it, so a snapshot reflects the CURRENT live set (AC-4).
type routeStore struct {
	mu     sync.Mutex
	routes map[storeKey]netip.Addr
}

var store = &routeStore{routes: make(map[storeKey]netip.Addr)}

// apply records an add or drops a remove. Only incremental emits mutate the
// store; a replay re-emits the existing set without touching it.
func (s *routeStore) apply(action redistevents.RouteAction, fam family.Family, prefix netip.Prefix, nh netip.Addr) {
	s.mu.Lock()
	defer s.mu.Unlock()
	k := storeKey{fam: fam, prefix: prefix}
	if action == redistevents.ActionRemove {
		delete(s.routes, k)
		return
	}
	s.routes[k] = nh
}

// snapshot returns the current live set for replay.
func (s *routeStore) snapshot() []storeEntry {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make([]storeEntry, 0, len(s.routes))
	for k, nh := range s.routes {
		out = append(out, storeEntry{fam: k.fam, prefix: k.prefix, nh: nh})
	}
	return out
}

// resetStore clears the current set. Test-only.
func resetStore() {
	store.mu.Lock()
	defer store.mu.Unlock()
	store.routes = make(map[storeKey]netip.Addr)
}

// reemitAll re-emits every tracked route as an add tagged with replayID, so the
// redistribute orchestrator can replay them to a peer that established after the
// original emit. A zero replayID is a no-op (the orchestrator only allocates
// nonzero tokens).
func reemitAll(replayID uint64) {
	if replayID == 0 {
		return
	}
	for _, e := range store.snapshot() {
		if _, err := emitOnce(redistevents.ActionAdd, e.fam, e.prefix, e.nh, replayID); err != nil {
			logger().Warn("fakeredist: replay emit failed", "prefix", e.prefix, "error", err)
		}
	}
}
