// Design: docs/architecture/plugin/feature-gates.md -- the reflection walk's active path, indexed so a backedge or an ancestor is found without scanning every frame.
// Related: externalreflectiontypes.go -- collectNamed pushes and pops this path and asks it for backedges.
// Related: externalreflectionreuse.go -- a cover asks this path where each recorded ancestor is active.
package repoinventory

import "go/ast"

// externalReflectionPathKey is the part of a frame that both frame
// comparisons require to be equal before they look at arguments:
// externalReflectionFrameEqual and externalReflectionFrameSame each refuse a
// different definition, method flag or value-root flag first. Two frames with
// different keys are therefore never equal under either, so a lookup that
// compares only the frames holding the same key answers what a scan of the
// whole path answers. The arguments stay out of the key on purpose: FrameEqual
// can prove two arguments with different scope objects equal by structure, so
// no pointer key over them is exact.
type externalReflectionPathKey struct {
	definition *ast.TypeSpec
	methods    bool
	valueRoot  bool
}

// externalReflectionPathEnds names the outermost and innermost path positions
// holding a key.
type externalReflectionPathEnds struct {
	first int
	last  int
}

// externalReflectionPathLink chains the positions of one key, outermost first.
// -1 ends the chain on either side.
type externalReflectionPathLink struct {
	previous int
	next     int
}

// externalReflectionActivePath is the stack of frames collectNamed has entered
// and not yet left, with an index from each key to the positions holding it.
// Positions of one key are linked in push order, so walking a chain from its
// first position visits them outermost first, the order a scan of the path
// takes. Every position is pushed and popped with its frame, and a key whose
// last position is popped leaves the index, so the index never holds more
// keys or links than the path holds frames. The path is bounded by the depth
// guard: every push raises the depth by at least one and collectNamed refuses
// at depth 128, so a path holds at most 128 frames.
//
// linear makes every lookup scan frames, as before the index existed; only a
// test sets it, through the walk's probe, and only while the path is empty.
// Not safe for concurrent use; a walker runs on one goroutine.
type externalReflectionActivePath struct {
	frames  []externalReflectionFrame
	links   []externalReflectionPathLink
	buckets map[externalReflectionPathKey]externalReflectionPathEnds
	linear  bool
}

func externalReflectionPathKeyOf(frame externalReflectionFrame) externalReflectionPathKey {
	return externalReflectionPathKey{definition: frame.definition, methods: frame.methods, valueRoot: frame.valueRoot}
}

// push enters frame as the innermost position.
func (p *externalReflectionActivePath) push(frame externalReflectionFrame) {
	position := len(p.frames)
	p.frames = append(p.frames, frame)
	if p.linear {
		return
	}
	key := externalReflectionPathKeyOf(frame)
	link := externalReflectionPathLink{previous: -1, next: -1}
	ends, found := p.buckets[key]
	if found {
		link.previous = ends.last
		p.links[ends.last].next = position
		ends.last = position
	} else {
		ends = externalReflectionPathEnds{first: position, last: position}
	}
	if p.buckets == nil {
		p.buckets = make(map[externalReflectionPathKey]externalReflectionPathEnds)
	}
	p.buckets[key] = ends
	p.links = append(p.links, link)
}

// pop leaves the innermost position. It is always the last position of its
// key, because positions only grow while a key's chain is built.
func (p *externalReflectionActivePath) pop() {
	last := len(p.frames) - 1
	frame := p.frames[last]
	p.frames = p.frames[:last]
	if p.linear {
		return
	}
	link := p.links[last]
	p.links = p.links[:last]
	key := externalReflectionPathKeyOf(frame)
	if link.previous < 0 {
		delete(p.buckets, key)
		return
	}
	p.links[link.previous].next = -1
	ends := p.buckets[key]
	ends.last = link.previous
	p.buckets[key] = ends
}

// chain answers the outermost position holding key, or -1 when none does.
func (p *externalReflectionActivePath) chain(key externalReflectionPathKey) int {
	ends, found := p.buckets[key]
	if !found {
		return -1
	}
	return ends.first
}

// backedge answers the outermost active position whose frame is
// externalReflectionFrameEqual to frame at depth, or -1 when none is.
func (p *externalReflectionActivePath) backedge(frame externalReflectionFrame, depth int) int {
	if p.linear {
		for index, active := range p.frames {
			if externalReflectionFrameEqual(active, frame, depth) {
				return index
			}
		}
		return -1
	}
	for position := p.chain(externalReflectionPathKeyOf(frame)); position >= 0; position = p.links[position].next {
		if externalReflectionFrameEqual(p.frames[position], frame, depth) {
			return position
		}
	}
	return -1
}

// externalReflectionActiveIndex answers the outermost path index holding a
// frame exactly the same as frame, or -1 when none does.
func externalReflectionActiveIndex(path *externalReflectionActivePath, frame externalReflectionFrame) int {
	if path.linear {
		for index, active := range path.frames {
			if externalReflectionFrameSame(active, frame) {
				return index
			}
		}
		return -1
	}
	for outer := path.chain(externalReflectionPathKeyOf(frame)); outer >= 0; outer = path.links[outer].next {
		if externalReflectionFrameSame(path.frames[outer], frame) {
			return outer
		}
	}
	return -1
}

func externalReflectionAncestorsActive(path *externalReflectionActivePath, ancestors []externalReflectionFrame) bool {
	for _, ancestor := range ancestors {
		if externalReflectionActiveIndex(path, ancestor) < 0 {
			return false
		}
	}
	return true
}
