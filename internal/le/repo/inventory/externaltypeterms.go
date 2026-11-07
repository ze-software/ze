// Design: docs/architecture/plugin/feature-gates.md -- each exact type a walk reaches has one identity per analysis, and only an instantiation that grows without end is refused.
// Related: externalreflectiontypes.go -- the reflection walk keys its frames here.
// Related: externalreflectionroots.go -- the value-root chain finds its cycles here.
package repoinventory

import (
	"go/ast"
	"slices"
)

// externalTypeTermDepthMax bounds the nesting of a frame's resolved type
// arguments. A finite type graph reaches finitely many exact frames, and each
// of them has a fixed argument term, so the walks treat it by reachability
// and need no bound. Only a generic instantiation that grows without end, such
// as grow[T] holding *grow[[]T], reaches a new frame on every step; its
// argument term then nests one level deeper each time, and the walk refuses
// the first frame whose term nests deeper than this bound. Retained source
// nests type arguments a handful of levels, so the bound only meets growth.
const externalTypeTermDepthMax = 128

// externalTypeTermAbsent is the child of an identifier no type parameter
// binds, and the empty list.
const externalTypeTermAbsent int32 = -1

// externalTypeTermKey is one interned term: the retained argument expression,
// and the list of the terms its type-parameter identifiers are bound to, in
// ast.Inspect order. Two arguments intern to the same term exactly when they
// are the same expression and every parameter it mentions is bound to the
// same term, which is what a walk over either argument reads: an identifier
// no parameter binds resolves through the expression's own package, and a
// bound one walks its argument.
type externalTypeTermKey struct {
	source   sourceExpr
	children int32
}

// externalTypeTermCell is one cell of an interned list of term ids.
type externalTypeTermCell struct {
	head int32
	tail int32
}

// externalTypeFrameKey names one exact frame: its owner (the definition and
// its package, externalReflectionKeys.owner), the walk's flags for it, and
// its interned argument list.
type externalTypeFrameKey struct {
	owner     int32
	kind      uint8
	arguments int32
}

// externalTypeTerms interns type arguments and frames to dense ids for the
// whole analysis. Equal ids mean equal exact types. Every table holds at most
// one entry per distinct term, list or frame the analysis reached, and the
// depth bound makes that set finite. Not safe for concurrent use; one
// analysis runs on one goroutine.
type externalTypeTerms struct {
	resolved map[externalTypeArgument]int32
	terms    map[externalTypeTermKey]int32
	depths   []int
	lists    map[externalTypeTermCell]int32
	frames   map[externalTypeFrameKey]int32
}

// frame answers the id of the exact frame, or false when an argument term
// nests deeper than externalTypeTermDepthMax.
func (t *externalTypeTerms) frame(owner int32, kind uint8, arguments []externalTypeArgument) (int32, bool) {
	list := externalTypeTermAbsent
	for _, argument := range slices.Backward(arguments) {
		term, depth := t.term(argument, 1)
		if depth > externalTypeTermDepthMax {
			return 0, false
		}
		list = t.cell(term, list)
	}
	key := externalTypeFrameKey{owner: owner, kind: kind, arguments: list}
	if id, found := t.frames[key]; found {
		return id, true
	}
	if t.frames == nil {
		t.frames = make(map[externalTypeFrameKey]int32)
	}
	id := int32(len(t.frames))
	t.frames[key] = id
	return id, true
}

// term answers the id and the depth of argument's term. nesting counts the
// substitutions above this one: it only stops the recursion once the outer
// term is already deeper than the bound, so a depth past the bound answers
// no id. The recursion is bounded by externalTypeTermDepthMax levels.
func (t *externalTypeTerms) term(argument externalTypeArgument, nesting int) (int32, int) {
	if nesting > externalTypeTermDepthMax {
		return 0, nesting
	}
	argument = externalResolvedTypeArgument(argument, 0)
	if id, found := t.resolved[argument]; found {
		return id, t.depths[id]
	}
	key := externalTypeTermKey{children: externalTypeTermAbsent}
	depth := 1
	if argument.source.value != nil {
		key.source = argument.source
		var children []int32
		ast.Inspect(argument.source.value, func(node ast.Node) bool {
			if depth > externalTypeTermDepthMax {
				return false
			}
			name, identifier := node.(*ast.Ident)
			if !identifier {
				return true
			}
			bound, found := argument.scope.argument(name)
			if !found {
				children = append(children, externalTypeTermAbsent)
				return true
			}
			child, childDepth := t.term(bound, nesting+1)
			depth = max(depth, childDepth+1)
			children = append(children, child)
			return true
		})
		if depth > externalTypeTermDepthMax {
			return 0, depth
		}
		for _, child := range slices.Backward(children) {
			key.children = t.cell(child, key.children)
		}
	}
	id, found := t.terms[key]
	if !found {
		if t.terms == nil {
			t.terms = make(map[externalTypeTermKey]int32)
		}
		id = int32(len(t.depths))
		t.terms[key] = id
		t.depths = append(t.depths, depth)
	}
	if t.resolved == nil {
		t.resolved = make(map[externalTypeArgument]int32)
	}
	t.resolved[argument] = id
	return id, t.depths[id]
}

func (t *externalTypeTerms) cell(head, tail int32) int32 {
	key := externalTypeTermCell{head: head, tail: tail}
	if id, found := t.lists[key]; found {
		return id
	}
	if t.lists == nil {
		t.lists = make(map[externalTypeTermCell]int32)
	}
	id := int32(len(t.lists))
	t.lists[key] = id
	return id
}
