// Design: docs/architecture/plugin/feature-gates.md -- monotone external capabilities.
package repoinventory

import "go/ast"

// externalValue is an abstract union, not a concrete Go value. Its zero denotes
// no capability; an untyped nil contributes none, while a typed-nil function
// retains reflection authority. Unknown is a separate explicit fact. Reads
// record metadata dependencies without granting writes to scalar copies.
type externalValue struct {
	callables map[ast.Node]sourceExpr
	locations map[*ast.Object]bool
	reads     map[*ast.Object]bool
	origins   map[string]bool
	unknown   string
	// unknownAt is the expression that produced unknown. It travels with the
	// first reason, so a terminal refusal names the line that caused it.
	unknownAt  sourceExpr
	fields     map[string]externalValue
	reflection externalReflection
	// unresolvedFields is a root-selector obligation, not an absent capability.
	// Indexed/ranged whole values retain their aggregate effects, but a static
	// field type alone cannot certify the element's actual stored field value.
	unresolvedFields string
	// References preserves each field's backing shape without granting a
	// location until an owning retained binding supplies one.
	references bool
}

// join grows a value monotonically. It never shares mutable sets with its input;
// this makes distinct return slots independent while conservatively identical.
func (v *externalValue) join(other externalValue) bool {
	changed := v.reflection.join(other.reflection)
	if v.unresolvedFields == "" {
		if other.unresolvedFields != "" {
			v.unresolvedFields = other.unresolvedFields
			changed = true
		}
	}
	if other.references {
		if !v.references {
			v.references = true
			changed = true
		}
	}
	if v.fields == nil {
		v.fields = make(map[string]externalValue, len(other.fields))
	}
	for name, value := range other.fields {
		current, exists := v.fields[name]
		if current.join(value) {
			changed = true
		}
		if !exists {
			changed = true
		}
		v.fields[name] = current
	}
	if v.callables == nil {
		if len(other.callables) != 0 {
			v.callables = make(map[ast.Node]sourceExpr, len(other.callables))
		}
	}
	for node, source := range other.callables {
		if _, present := v.callables[node]; !present {
			v.callables[node] = source
			changed = true
		}
	}
	if v.locations == nil {
		if len(other.locations) != 0 {
			v.locations = make(map[*ast.Object]bool, len(other.locations))
		}
	}
	for object := range other.locations {
		if !v.references {
			v.references = true
			changed = true
		}
		if !v.locations[object] {
			v.locations[object] = true
			changed = true
		}
	}
	if v.reads == nil {
		if len(other.reads) != 0 {
			v.reads = make(map[*ast.Object]bool, len(other.reads))
		}
	}
	for object := range other.reads {
		if !v.reads[object] {
			v.reads[object] = true
			changed = true
		}
	}
	if v.origins == nil {
		if len(other.origins) != 0 {
			v.origins = make(map[string]bool, len(other.origins))
		}
	}
	for identity := range other.origins {
		if !v.origins[identity] {
			v.origins[identity] = true
			changed = true
		}
	}
	if v.unknown == "" {
		if other.unknown != "" {
			v.unknown = other.unknown
			v.unknownAt = other.unknownAt
			changed = true
		}
	}
	return changed
}

// locate gives an unsited unknown the expression a walk was evaluating when
// the reason appeared. Walks call it on their own result as they return, so
// the innermost walk sites the reason and every outer walk keeps that site.
func (v *externalValue) locate(source sourceExpr) {
	if v.unknown == "" {
		return
	}
	if v.unknownAt.file != nil {
		return
	}
	v.unknownAt = source
}

func externalCallable(node ast.Node, source sourceExpr) externalValue {
	return externalValue{callables: map[ast.Node]sourceExpr{node: source}}
}

func externalLocation(object *ast.Object) externalValue {
	return externalValue{locations: map[*ast.Object]bool{object: true}, references: true}
}

func externalOrigin(identity string) externalValue {
	return externalValue{origins: map[string]bool{identity: true}, references: true}
}

// externalFlat preserves all reachable capabilities while dropping root field
// names. Child aggregation and function capture use this union; fields belong
// only to the value whose exact root declares them. The 128-edge bound refuses
// recursive structural expansion rather than increasing the fixed-point budget.
func externalFlat(value externalValue, depth int) externalValue {
	var result externalValue
	result.joinFlat(value, depth)
	return result
}

// joinFlat copies each visited node into the same output union, avoiding a
// separate flattened union copied through every ancestor. Its 128-edge cutoff
// excludes that node's capabilities while preserving all sibling traversals.
func (v *externalValue) joinFlat(value externalValue, depth int) {
	if depth == 128 {
		if v.unknown == "" {
			v.unknown = "function capture structure exceeds 128 steps"
		}
		return
	}
	fields := value.fields
	value.fields = nil
	// A child's selector obligation belongs with its retained field value,
	// not with an enclosing holder's unrelated direct scalar field.
	value.unresolvedFields = ""
	v.join(value)
	for _, field := range fields {
		v.joinFlat(field, depth+1)
	}
}

// externalCallSite survives the private analysis FileSet without using global
// token.Pos integers as cross-snapshot identity. Its maps live in one reader.
type externalCallSite struct {
	file   string
	offset int
}

func externalSite(snapshot *Snapshot, call *ast.CallExpr) externalCallSite {
	// A chained call and its receiver call share Pos. Their invocation
	// parentheses are distinct, so an outer certificate cannot erase a factory.
	position := snapshot.fset.Position(call.Lparen)
	return externalCallSite{file: position.Filename, offset: position.Offset}
}
