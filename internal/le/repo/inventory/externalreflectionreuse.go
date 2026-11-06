// Design: docs/architecture/plugin/feature-gates.md -- one reflection query skips a named visit only on proof that its output union already holds the contribution.
// Related: externalreflectiontypes.go -- the walk that consults this table before it expands a named type.
package repoinventory

import (
	"go/ast"
	"math/bits"
)

// The bounds of one query's reuse table. A visit that cannot be recorded
// inside them is simply not recorded, so a later equal visit traverses.
const (
	// externalReflectionReuseVisits caps the completed visits one query keeps.
	externalReflectionReuseVisits = 1 << 16
	// externalReflectionReuseVisitsPerKey caps the visits kept for one frame
	// shape, owner and incoming depth.
	externalReflectionReuseVisitsPerKey = 4
	// externalReflectionReuseFrames caps the ancestor frames all kept visits
	// hold together.
	externalReflectionReuseFrames = 1 << 20
	// externalReflectionReuseIndexes is the number of path indexes a pruned
	// set can name. collectNamed refuses at depth 128 and every push raises
	// the depth, so a path started at depth zero never reaches it.
	externalReflectionReuseIndexes = 128
)

// externalReflectionReuseProbe is set only by tests, on the walker. A nil
// probe is the production walk: reuse on, path index on, default capacity,
// nothing counted.
type externalReflectionReuseProbe struct {
	disabled     bool // traverse every visit, as before reuse existed
	visits       int  // completed visits the table may keep; 0 keeps the default
	visitsPerKey int  // completed visits one key may keep; 0 keeps the default
	frames       int  // ancestor frames all kept visits may hold; 0 keeps the default
	traversals   int  // named visits that pushed a frame and expanded
	linearPath   bool // scan the active path for every lookup, as before its index existed
}

// externalReflectionReuse belongs to one query: one value call and its one
// output union. It holds proof of coverage, never an answer: no method map,
// no origin, no reason and no scope it did not get from the frames it saw.
// Not safe for concurrent use; a walker runs on one goroutine.
type externalReflectionReuse struct {
	open            []externalReflectionOpenVisit
	completed       map[externalReflectionVisitKey][]externalReflectionCompletedVisit
	visits          int
	visitsMax       int
	visitsPerKeyMax int
	frames          int
	framesMax       int
}

// externalReflectionVisitKey groups the visits a skip may compare. Arguments
// are compared exactly inside the group, because a slice is not comparable.
type externalReflectionVisitKey struct {
	definition *ast.TypeSpec
	owner      sourceExpr
	depth      int
	methods    bool
	valueRoot  bool
	arguments  int
}

// externalReflectionOpenVisit is a visit whose frame is on the path at index.
// pruned names the path indexes below index that a backedge return inside the
// visit's subtree matched. wide records a match the set cannot name.
type externalReflectionOpenVisit struct {
	index  int
	pruned [2]uint64
	wide   bool
}

// externalReflectionCompletedVisit is one finished visit's proof material:
// its exact resolved arguments, and the frames outside it that its subtree
// pruned against. The argument slices are the frames' own: collectNamed
// resolves them before it builds the frame and never writes them after.
type externalReflectionCompletedVisit struct {
	arguments []externalTypeArgument
	ancestors []externalReflectionFrame
}

// newExternalReflectionReuse answers nil when a test disabled reuse, so the
// walk then takes the original traversal everywhere.
func newExternalReflectionReuse(probe *externalReflectionReuseProbe) *externalReflectionReuse {
	reuse := &externalReflectionReuse{
		visitsMax:       externalReflectionReuseVisits,
		visitsPerKeyMax: externalReflectionReuseVisitsPerKey,
		framesMax:       externalReflectionReuseFrames,
	}
	if probe == nil {
		return reuse
	}
	if probe.disabled {
		return nil
	}
	if probe.visits > 0 {
		reuse.visitsMax = probe.visits
	}
	if probe.visitsPerKey > 0 {
		reuse.visitsPerKeyMax = probe.visitsPerKey
	}
	if probe.frames > 0 {
		reuse.framesMax = probe.frames
	}
	return reuse
}

// pruned records a backedge return that matched the active frame at index.
// The innermost open visit keeps it when the frame lies outside that visit.
func (r *externalReflectionReuse) pruned(index int) {
	top := len(r.open) - 1
	if top < 0 {
		return
	}
	r.open[top].note(index)
}

func (v *externalReflectionOpenVisit) note(index int) {
	if index >= v.index {
		return
	}
	if index >= externalReflectionReuseIndexes {
		v.wide = true
		return
	}
	v.pruned[index/64] |= 1 << (index % 64)
}

// covered answers whether a completed visit of the same exact frame, owner and
// incoming depth pruned only against frames that are active now. On a yes, the
// skipped visit's pruned set is charged to the innermost open visit, exactly
// as its traversal would have charged it, so that visit's own record stays a
// proof. Both ancestor lookups go through the path's index.
func (r *externalReflectionReuse) covered(path *externalReflectionActivePath, frame externalReflectionFrame, owner sourceExpr, depth int) bool {
	for _, visit := range r.completed[externalReflectionVisitKeyOf(frame, owner, depth)] {
		if !externalReflectionArgumentsSame(visit.arguments, frame.arguments) {
			continue
		}
		if !externalReflectionAncestorsActive(path, visit.ancestors) {
			continue
		}
		for _, ancestor := range visit.ancestors {
			r.pruned(externalReflectionActiveIndex(path, ancestor))
		}
		return true
	}
	return false
}

// enter opens the visit whose frame was just pushed at index.
func (r *externalReflectionReuse) enter(index int) {
	r.open = append(r.open, externalReflectionOpenVisit{index: index})
}

// complete closes the innermost open visit, hands its pruned set to the visit
// that contains it (minus that visit's own frame and anything inside it), and
// records it while the table has room. path still holds the visit's frame.
func (r *externalReflectionReuse) complete(path []externalReflectionFrame, frame externalReflectionFrame, owner sourceExpr, depth int) {
	top := len(r.open) - 1
	visit := r.open[top]
	r.open = r.open[:top]
	if top > 0 {
		parent := &r.open[top-1]
		parent.wide = parent.wide || visit.wide
		externalReflectionEachIndex(visit.pruned, parent.note)
	}
	if visit.wide {
		return
	}
	r.store(path, frame, owner, depth, visit.pruned)
}

// store keeps one completed visit, or nothing when any bound is reached.
func (r *externalReflectionReuse) store(path []externalReflectionFrame, frame externalReflectionFrame, owner sourceExpr, depth int, pruned [2]uint64) {
	if r.visits >= r.visitsMax {
		return
	}
	count := bits.OnesCount64(pruned[0]) + bits.OnesCount64(pruned[1])
	if r.frames+count > r.framesMax {
		return
	}
	key := externalReflectionVisitKeyOf(frame, owner, depth)
	group := r.completed[key]
	if len(group) >= r.visitsPerKeyMax {
		return
	}
	ancestors := make([]externalReflectionFrame, 0, count)
	externalReflectionEachIndex(pruned, func(index int) {
		ancestors = append(ancestors, path[index])
	})
	if r.completed == nil {
		r.completed = make(map[externalReflectionVisitKey][]externalReflectionCompletedVisit)
	}
	r.completed[key] = append(group, externalReflectionCompletedVisit{arguments: frame.arguments, ancestors: ancestors})
	r.visits++
	r.frames += count
}

func externalReflectionVisitKeyOf(frame externalReflectionFrame, owner sourceExpr, depth int) externalReflectionVisitKey {
	return externalReflectionVisitKey{
		definition: frame.definition,
		owner:      owner,
		depth:      depth,
		methods:    frame.methods,
		valueRoot:  frame.valueRoot,
		arguments:  len(frame.arguments),
	}
}

// externalReflectionEachIndex calls visit for every set index, lowest first.
func externalReflectionEachIndex(set [2]uint64, visit func(index int)) {
	for word, remaining := range set {
		for remaining != 0 {
			visit(word*64 + bits.TrailingZeros64(remaining))
			remaining &= remaining - 1
		}
	}
}

// externalReflectionArgumentsSame is exact identity: the same retained source
// and the same scope object for every argument. Equal scope pointers carry
// equal bindings, because a scope is never written after it is built.
func externalReflectionArgumentsSame(left, right []externalTypeArgument) bool {
	if len(left) != len(right) {
		return false
	}
	for index, argument := range left {
		if argument.source != right[index].source {
			return false
		}
		if argument.scope != right[index].scope {
			return false
		}
	}
	return true
}

// externalReflectionFrameSame is exact frame identity, the form a cover is
// checked in. It is stricter than externalReflectionFrameEqual, which may
// prove two fresh scope chains equal by structure.
func externalReflectionFrameSame(left, right externalReflectionFrame) bool {
	if left.definition != right.definition {
		return false
	}
	if left.methods != right.methods {
		return false
	}
	if left.valueRoot != right.valueRoot {
		return false
	}
	return externalReflectionArgumentsSame(left.arguments, right.arguments)
}
