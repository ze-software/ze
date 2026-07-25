// Design: docs/functional-tests.md -- fakeas112 current-set + replay-on-request
//
// The store tracks whether fakeas112 currently announces its covering prefixes
// (plus the origin ASN + community they carry) so it can re-emit them on a
// redistribute ReplayRequest, mirroring fakeredist/store.go and the real
// as112Producer's announced-state reconciliation. The covering prefixes are
// fixed and always added/withdrawn together, so the current set collapses to a
// single announced flag rather than fakeredist's per-prefix map -- but the
// apply / snapshot / reemitAll shape matches.

package fakeas112

import (
	"slices"
	"sync"

	"github.com/ze-software/ze/internal/core/family"
	"github.com/ze-software/ze/internal/core/redistevents"
)

// routeStore is the mutex-guarded current announced set. It tracks WHICH
// families are currently announced (not just a single flag) so a per-family
// `emit add family <af>` and a `del` that withdraws only what was actually
// announced both behave like the real producer -- a `del` after a single-family
// add must not withdraw the family that was never added.
type routeStore struct {
	mu        sync.Mutex
	families  []family.Family
	asn       uint32
	community []uint32
}

var store = &routeStore{}

// applyAdd records an add of the given families (union with the current set) and
// the attributes they carry. Only incremental emits mutate the store; a replay
// re-emits the existing set without touching it.
func (s *routeStore) applyAdd(families []family.Family, asn uint32, community []uint32) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, f := range families {
		if !slices.Contains(s.families, f) {
			s.families = append(s.families, f)
		}
	}
	s.asn = asn
	s.community = slices.Clone(community)
}

// applyDel removes the requested families from the announced set and returns the
// families that were ACTUALLY announced (so the caller withdraws only those),
// plus the attributes they carried. An empty request means "all announced". This
// is the fidelity fix: a del never emits a withdraw for a family that was not
// announced.
func (s *routeStore) applyDel(request []family.Family) (removed []family.Family, asn uint32, community []uint32) {
	s.mu.Lock()
	defer s.mu.Unlock()
	asn, community = s.asn, slices.Clone(s.community)
	target := request
	if len(target) == 0 {
		target = slices.Clone(s.families)
	}
	for _, f := range target {
		if i := slices.Index(s.families, f); i >= 0 {
			s.families = slices.Delete(s.families, i, i+1)
			removed = append(removed, f)
		}
	}
	if len(s.families) == 0 {
		s.asn = 0
		s.community = nil
	}
	return removed, asn, community
}

// snapshot returns the currently-announced families and the attributes they
// carry, for replay.
func (s *routeStore) snapshot() (families []family.Family, asn uint32, community []uint32) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return slices.Clone(s.families), s.asn, slices.Clone(s.community)
}

// resetStore clears the current set. Test-only.
func resetStore() {
	store.mu.Lock()
	defer store.mu.Unlock()
	store.families = nil
	store.asn = 0
	store.community = nil
}

// reemitAll re-emits the currently-announced covering prefixes as adds tagged
// with replayID, so the redistribute orchestrator can replay them to a peer
// that established after the original emit. A zero replayID is a no-op (the
// orchestrator only allocates nonzero tokens); nothing announced is a no-op.
func reemitAll(replayID uint64) {
	if replayID == 0 {
		return
	}
	families, asn, community := store.snapshot()
	if len(families) == 0 {
		return
	}
	if _, err := emitFamiliesList(redistevents.ActionAdd, families, asn, community, replayID); err != nil {
		logger().Warn("fakeas112: replay emit failed", "error", err)
	}
}

// _ resetStore referenced so the test-only helper is retained for unit tests
// that import the package; keeps the linter from flagging it unused.
var _ = resetStore
