// Design: plan/design-rib-unified.md -- Phase 3 (unified Loc-RIB)
// Related: candidate.go -- Path value type stored in each PathGroup

package locrib

// PathGroup holds every Path contributed to one (family, prefix) by every
// source (protocol + instance), plus the index of the currently selected
// best Path. Best is stored as an index, not a pointer, so mutating the
// underlying slice does not leave a dangling reference.
//
// PathGroup is stored by VALUE in store.Store; Manager uses store.Modify to
// mutate in place. The Paths slice is heap-allocated, so in-place append
// works across Modify calls as long as the backing array is retained.
type PathGroup struct {
	// Paths are the per-source route contributions for this prefix.
	// Typical cardinality is 1-3 (one or two protocols per prefix).
	Paths []Path

	// Best is the index into Paths of the currently selected best path,
	// or -1 when no valid Path exists.
	Best int
}

// upsert inserts or replaces the Path at its (Source, Instance) key and
// recomputes Best.
func (g *PathGroup) upsert(p Path) {
	k := p.key()
	for i := range g.Paths {
		if g.Paths[i].key() == k {
			g.Paths[i] = p
			g.Best = selectBest(g.Paths)
			return
		}
	}
	g.Paths = append(g.Paths, p)
	g.Best = selectBest(g.Paths)
}

// remove deletes the Path matching (source, instance). Returns true if a
// removal happened; updates Best afterward.
func (g *PathGroup) remove(k pathKey) bool {
	for i := range g.Paths {
		if g.Paths[i].key() != k {
			continue
		}
		last := len(g.Paths) - 1
		g.Paths[i] = g.Paths[last]
		g.Paths = g.Paths[:last]
		g.Best = selectBest(g.Paths)
		return true
	}
	return false
}

// best returns the currently selected best Path plus a bool indicating
// whether a valid best exists.
func (g *PathGroup) best() (Path, bool) {
	if g.Best < 0 || g.Best >= len(g.Paths) {
		return Path{}, false
	}
	p := g.Paths[g.Best]
	if !p.Valid() {
		return Path{}, false
	}
	return p, true
}

// selectBest returns the index of the best Path in paths, or -1 when the
// slice is empty or every Path is invalid.
//
// Ordering: lower AdminDistance wins; on tie, lower Metric wins; on further
// tie, first seen wins (stable). Invalid Paths (Source == ProtocolUnspecified)
// are skipped.
func selectBest(paths []Path) int {
	best := -1
	for i := range paths {
		if !paths[i].Valid() {
			continue
		}
		if best < 0 {
			best = i
			continue
		}
		if paths[i].AdminDistance < paths[best].AdminDistance {
			best = i
			continue
		}
		if paths[i].AdminDistance > paths[best].AdminDistance {
			continue
		}
		if paths[i].Metric < paths[best].Metric {
			best = i
		}
	}
	return best
}
