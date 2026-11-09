// Design: docs/architecture/plugin/feature-gates.md -- ranged element summaries.
// Related: externalrangestores.go -- range header stores remain retained assignments.
// Related: externalrangeuses.go -- the per-package occurrence index the proof reads.
package repoinventory

import (
	"go/ast"
	"go/token"
	"slices"
)

// The builtins the element population proof admits by name. The callee is
// first proved to be the predeclared builtin, never a package shadow.
const (
	externalBuiltinAppend = "append"
	externalBuiltinCap    = "cap"
	externalBuiltinClear  = "clear"
	externalBuiltinDelete = "delete"
	externalBuiltinLen    = "len"
	externalBuiltinMake   = "make"
)

// externalElementSource is one expression the analysis proves is stored into
// a ranged collection's elements: a literal element, an element store, an
// append argument. A literal element keeps its enclosing collection layout,
// so an element that elides its type is evaluated under the type the
// enclosing literal states, as the capability walk evaluates it there.
type externalElementSource struct {
	source sourceExpr
	owner  ast.Node
	layout *externalLiteralLayout
}

// externalRangeVariable answers the defining range clause of a range
// variable: go/parser binds both iteration variables to a synthetic short
// assignment whose one right-hand side is the unary RANGE of the collection.
func externalRangeVariable(object *ast.Object) (*ast.AssignStmt, *ast.UnaryExpr, bool) {
	if object == nil {
		return nil, nil, false
	}
	assignment, assigned := object.Decl.(*ast.AssignStmt)
	if !assigned {
		return nil, nil, false
	}
	if len(assignment.Rhs) != 1 {
		return nil, nil, false
	}
	ranged, unary := assignment.Rhs[0].(*ast.UnaryExpr)
	if !unary {
		return nil, nil, false
	}
	if ranged.Op != token.RANGE {
		return nil, nil, false
	}
	return assignment, ranged, true
}

// rangeElements answers every source whose value the range variable's value
// slot can hold, or the reason the element population is not fully known.
//
// Go specification, "For statements with range clause": for an array, pointer
// to array, or slice value a, the iteration values are "index i int" and
// "a[i] E", and for a map m they are "key k K" and "m[k] V". The second
// iteration value is therefore always an element some store put into the
// collection, and nothing else: so the value slot holds exactly the join of
// the collection's stored elements. A channel range has one iteration value,
// the values sent on the channel, and a function iterator yields what its body
// chooses; neither is summarized here, and a channel keeps its received-element
// obligation. The population is proved from syntax and exact declaration
// identities only: a collection that escapes, comes from a call result, or is
// filled through an alias the proof cannot follow refuses. An empty proved
// population is a real answer: the range body never runs, so no element value
// is invented and none is needed.
func (e *externalEffects) rangeElements(object *ast.Object, variable sourceExpr, depth int) ([]externalElementSource, string) {
	assignment, ranged, synthetic := externalRangeVariable(object)
	if !synthetic {
		return nil, "range variable declaration is unresolved"
	}
	if assignment.Tok != token.DEFINE {
		return nil, "an assigned range target has no fresh element binding"
	}
	if len(assignment.Lhs) != 2 {
		return nil, "a range key is not an element"
	}
	slot, identifier := assignment.Lhs[1].(*ast.Ident)
	if !identifier {
		return nil, "range value slot is unresolved"
	}
	if slot.Obj != object {
		return nil, "a range key is not an element"
	}
	collection := sourceExpr{value: ranged.X, file: variable.file, pkg: variable.pkg}
	walk := externalElementWalk{effects: e, visited: make(map[any]bool)}
	return walk.collection(collection, nil, depth)
}

// rangedValue is the value a range variable's value slot holds: the join of
// every proved element value, the element type's own methods, and the
// collection's whole flattened capability union, so no callback or location
// the collection retains is lost. Only the proved elements contribute field
// names, so a key, a child or the holder never lends a field. A population
// that is not proved keeps the root-selector obligation, so a type-derived
// empty field can never certify an element's real field value.
func (e *externalEffects) rangedValue(object *ast.Object, source sourceExpr, collection, methods externalValue, depth int) externalValue {
	result := externalFlat(collection, 0)
	result.join(methods)
	if collection.unresolvedFields != "" {
		// The collection itself came through a selector the analysis could
		// not own, so its elements cannot be summarized either.
		result.unresolvedFields = "ranged element field ownership requires a retained element summary: " + collection.unresolvedFields
		return result
	}
	elements, reason := e.rangeElements(object, source, depth)
	if reason != "" {
		result.unresolvedFields = "ranged element field ownership requires a retained element summary: " + reason
		return result
	}
	if e.reading[object] {
		// An element source that reads this range variable again, such as a
		// store of it back into its own collection: the binding holds every
		// element value joined so far, and the next fixed-point round
		// revisits the equation, as readObject does for a recursive read.
		result.join(e.bindings[object])
		return result
	}
	e.reading[object] = true
	defer delete(e.reading, object)
	var stored externalValue
	for _, element := range elements {
		stored.join(e.elementValue(element, depth))
		if e.amb.unknown != "" {
			return externalValue{unknown: e.amb.unknown}
		}
	}
	e.bind(object, stored)
	result.join(stored)
	return result
}

// elementValue evaluates one proved element source in its producer's
// execution mode, never its consumer's, as readObject evaluates a definition.
func (e *externalEffects) elementValue(element externalElementSource, depth int) externalValue {
	possible := e.definitionPossible(externalDefinition{source: element.source, owner: element.owner})
	if element.layout != nil {
		return e.literalElement(element.source, element.source.value, *element.layout, false, possible, depth)
	}
	return e.value(element.source, possible, depth+1)
}

// rangedReceiverType answers the static element type of a range variable's
// value slot, once its element population is proved. The selector x.f on a
// pointer to a struct is shorthand for (*x).f (Go specification, "Selectors"),
// so a pointer element type selects the fields of the struct it points to.
func (e *externalEffects) rangedReceiverType(object *ast.Object, source sourceExpr, depth int) (sourceExpr, string) {
	if _, reason := e.rangeElements(object, source, depth); reason != "" {
		return sourceExpr{}, "ranged element field ownership requires a retained element summary: " + reason
	}
	datatype, known := e.resolver.valueType(source, depth)
	if !known {
		return sourceExpr{}, "ranged element type is unresolved"
	}
	if pointer, indirect := ast.Unparen(datatype.value).(*ast.StarExpr); indirect {
		datatype.value = pointer.X
	}
	return datatype, ""
}

// rangedProof grounds a range variable exactly when its collection's element
// sources are grounded: every proved source joins as a dependency, and each
// dependency must reach a real source on its own. A proved empty population
// grounds the variable itself, because no argument ever flows from it.
func (proofs *externalFormalProofs) rangedProof(object *ast.Object, variable sourceExpr, depth int) *externalFormalProof {
	elements, reason := proofs.effects.rangeElements(object, variable, 0)
	if reason != "" {
		return &externalFormalProof{unknown: "ranged element provenance is unresolved: " + reason}
	}
	proof := &externalFormalProof{grounded: true}
	for _, element := range elements {
		literal, composite := element.source.value.(*ast.CompositeLit)
		if element.layout != nil && composite && literal.Type == nil {
			stated, known := elidedLiteralType(*element.layout, false)
			if !known {
				proof.join(&externalFormalProof{unknown: "elided ranged element type is unresolved"})
				continue
			}
			proof.join(proofs.literal(element.source, literal, stated, depth+1))
			continue
		}
		proof.join(proofs.expression(element.source, depth+1))
	}
	return proof
}

// externalElementWalk follows the source equations of one collection. Each
// identity (a variable or an unexported struct field) is visited once: the
// population is the union of every visited identity's stores, so a cycle adds
// nothing it has not already added.
type externalElementWalk struct {
	effects *externalEffects
	visited map[any]bool
}

// collection answers the element sources of a whole-collection expression.
// link is the occurrence through which the walk reached this expression, the
// one alias the identity it names is allowed to have. The walk follows at most
// 64 source steps.
func (w *externalElementWalk) collection(source sourceExpr, owner ast.Node, depth int) ([]externalElementSource, string) {
	if depth >= 64 {
		return nil, "ranged collection provenance exceeds 64 source steps"
	}
	e := w.effects
	switch value := source.value.(type) {
	case *ast.ParenExpr:
		return w.collection(sourceExpr{value: value.X, file: source.file, pkg: source.pkg}, owner, depth+1)
	case *ast.Ident:
		// The predeclared nil holds no element; nil is the only predeclared
		// identifier a collection can be assigned.
		if predeclaredIdentifier(source, value.Name) {
			return nil, ""
		}
		return w.identity(e.object(source), source, depth+1)
	case *ast.CompositeLit:
		return w.literal(source, value, owner, depth+1)
	case *ast.CallExpr:
		return w.builtin(source, value, owner, depth+1)
	case *ast.SelectorExpr:
		if _, imported := e.resolver.callableImport(source, value); imported {
			return nil, "ranged collection is another package's variable"
		}
		return w.field(source, value, depth+1)
	}
	return nil, "ranged collection source is not a retained variable, field, literal, make or append"
}

// literal answers a collection literal's elements. Its keys are not elements.
func (w *externalElementWalk) literal(source sourceExpr, literal *ast.CompositeLit, owner ast.Node, depth int) ([]externalElementSource, string) {
	if literal.Type == nil {
		return nil, "elided ranged collection literal type is unresolved"
	}
	layout, reason := w.effects.literalLayout(sourceExpr{value: literal.Type, file: source.file, pkg: source.pkg}, depth, false)
	if reason != "" {
		return nil, "ranged collection literal layout is unresolved: " + reason
	}
	if layout.kind != externalLiteralCollection {
		return nil, "ranged collection literal is not a slice, array or map"
	}
	var elements []externalElementSource
	for _, element := range literal.Elts {
		if keyed, ok := element.(*ast.KeyValueExpr); ok {
			element = keyed.Value
		}
		elements = append(elements, externalElementSource{
			source: sourceExpr{value: element, file: source.file, pkg: source.pkg},
			owner:  owner,
			layout: &layout,
		})
	}
	return elements, ""
}

// builtin admits make, which creates no stored element (a zero element
// carries only its type's capabilities, which the range value joins), and
// append, which keeps the first operand's elements and adds its arguments.
// Any other call result is a population the proof cannot see.
func (w *externalElementWalk) builtin(source sourceExpr, call *ast.CallExpr, owner ast.Node, depth int) ([]externalElementSource, string) {
	callee := sourceExpr{value: call.Fun, file: source.file, pkg: source.pkg}
	if !w.effects.builtin(callee) {
		return nil, "ranged collection is filled from a call result"
	}
	// builtin answers true only for an identifier callee.
	name, identifier := callee.value.(*ast.Ident)
	if !identifier {
		return nil, "ranged collection builtin is unresolved"
	}
	switch name.Name {
	case externalBuiltinMake:
		return nil, ""
	case externalBuiltinAppend:
		if len(call.Args) == 0 {
			return nil, "ranged collection append has no operand"
		}
		elements, reason := w.collection(sourceExpr{value: call.Args[0], file: source.file, pkg: source.pkg}, owner, depth+1)
		if reason != "" {
			return nil, reason
		}
		if call.Ellipsis.IsValid() {
			if len(call.Args) != 2 {
				return nil, "ranged collection append spread is unresolved"
			}
			spread, reason := w.collection(sourceExpr{value: call.Args[1], file: source.file, pkg: source.pkg}, owner, depth+1)
			if reason != "" {
				return nil, reason
			}
			return append(elements, spread...), ""
		}
		for _, argument := range call.Args[1:] {
			elements = append(elements, externalElementSource{source: sourceExpr{value: argument, file: source.file, pkg: source.pkg}, owner: owner})
		}
		return elements, ""
	}
	return nil, "ranged collection is filled from a builtin result"
}

// identity answers the population of a variable: its definitions, plus every
// element store any occurrence makes. Only a variable whose every writer the
// retained source shows is admitted: a parameter takes its elements from
// callers, an exported package variable from importers, and a linked one from
// another package's declaration.
func (w *externalElementWalk) identity(object *ast.Object, link sourceExpr, depth int) ([]externalElementSource, string) {
	e := w.effects
	if object == nil {
		return nil, "ranged collection identity is unresolved"
	}
	if w.visited[object] {
		return nil, ""
	}
	w.visited[object] = true
	if object.Kind != ast.Var {
		return nil, "ranged collection is not a variable"
	}
	if e.linkedObject(object) {
		return nil, "ranged collection is linked to another declaration"
	}
	if _, formal := e.formals[object]; formal {
		return nil, "ranged collection is a parameter, whose elements come from its callers"
	}
	if _, _, ranged := externalRangeVariable(object); ranged {
		return nil, "ranged collection is itself a range variable"
	}
	if callablePackageBinding(link.pkg, object.Name) == object {
		if ast.IsExported(object.Name) {
			return nil, "ranged collection is an exported package variable another package can fill"
		}
	}
	// The ambient pool's locations are not consulted: the capability union
	// over-approximates them (len(x) joins x's location into a make result),
	// while every source route by which external code can reach the
	// collection is an occurrence the walk below already judges: an alias,
	// an address, or an argument to a call that can run.
	var elements []externalElementSource
	for _, use := range e.rangeUses(link.pkg).objects[object] {
		stored, reason := w.use(use, object, link.value, false, depth+1)
		if reason != "" {
			return nil, reason
		}
		elements = append(elements, stored...)
	}
	for _, definition := range e.definitions[object] {
		defined, reason := w.collection(definition.source, definition.owner, depth+1)
		if reason != "" {
			return nil, reason
		}
		elements = append(elements, defined...)
	}
	return elements, ""
}

// field answers the population of an unexported field of a retained struct
// type. Only the declaring package can name it, so the occurrences there are
// every writer: keyed literals of the type, assignments to the field and
// element stores through it. The population is one for the type, joined over
// every value of it, which only adds sources.
func (w *externalElementWalk) field(source sourceExpr, selector *ast.SelectorExpr, depth int) ([]externalElementSource, string) {
	e := w.effects
	identity, pkg, reason := e.rangeFieldIdentity(source, selector)
	if reason != "" {
		return nil, reason
	}
	if ast.IsExported(identity.name) {
		return nil, "ranged collection is an exported field another package can fill"
	}
	if w.visited[identity] {
		return nil, ""
	}
	w.visited[identity] = true
	uses := e.rangeUses(pkg)
	var elements []externalElementSource
	for _, use := range uses.fields[identity.name] {
		if use.selector == nil {
			continue
		}
		occurrence := sourceExpr{value: use.selector, file: use.file, pkg: pkg}
		if _, imported := e.resolver.callableImport(occurrence, use.selector); imported {
			continue
		}
		other, _, reason := e.rangeFieldIdentity(occurrence, use.selector)
		if reason != "" {
			return nil, "ranged collection field " + identity.name + " has an occurrence whose owner is unresolved: " + reason
		}
		if other != identity {
			continue
		}
		stored, reason := w.use(use, nil, source.value, true, depth+1)
		if reason != "" {
			return nil, reason
		}
		elements = append(elements, stored...)
	}
	if slices.ContainsFunc(uses.objects[identity.datatype], externalRangeConversion) {
		return nil, "ranged collection field " + identity.name + " belongs to a type a conversion can construct"
	}
	for _, key := range uses.keys[identity.name] {
		datatype, reason := e.rangeLiteralType(key, pkg, false)
		if reason != "" {
			return nil, "ranged collection field " + identity.name + " is set by a literal whose type is unresolved: " + reason
		}
		if datatype != identity.datatype {
			continue
		}
		defined, reason := w.collection(sourceExpr{value: key.node, file: key.file, pkg: pkg}, key.owner, depth+1)
		if reason != "" {
			return nil, reason
		}
		elements = append(elements, defined...)
	}
	for _, positional := range uses.positional {
		datatype, reason := e.rangeLiteralType(positional, pkg, true)
		if reason != "" {
			return nil, "ranged collection field " + identity.name + " may be set by a positional literal whose type is unresolved: " + reason
		}
		if datatype == identity.datatype {
			return nil, "ranged collection field " + identity.name + " is set by a positional literal"
		}
	}
	return elements, ""
}

// use admits one occurrence of a summarized collection, and answers the
// element sources it stores. An occurrence that can change the population in
// a way no source shows refuses: an alias other than link, an element whose
// address or method escapes, a call that receives the collection where that
// call can run. A call in a body no execution has reached cannot run, the
// same rule the capability walk applies to every call; the fixed point
// revisits this proof once that body is scheduled.
func (w *externalElementWalk) use(use externalRangeUse, object *ast.Object, link ast.Node, field bool, depth int) ([]externalElementSource, string) {
	occurrence := use.occurrence()
	scope := sourceExpr{file: use.file, pkg: use.pkg}
	switch parent := use.parents[0].(type) {
	case *ast.ValueSpec:
		for _, name := range parent.Names {
			if name == occurrence {
				return nil, ""
			}
		}
	case *ast.AssignStmt:
		for index, left := range parent.Lhs {
			if left != occurrence {
				continue
			}
			if !field {
				// The definitions index already holds this right-hand side.
				return nil, ""
			}
			if parent.Tok != token.ASSIGN {
				return nil, "ranged collection field assignment form is unresolved"
			}
			if len(parent.Lhs) != len(parent.Rhs) {
				return nil, "ranged collection field is assigned from a multi-value result"
			}
			scope.value = parent.Rhs[index]
			return w.collection(scope, use.owner, depth+1)
		}
	case *ast.RangeStmt:
		if parent.X == occurrence {
			return nil, ""
		}
	case *ast.IndexExpr:
		if parent.X == occurrence {
			return externalElementAccess(use, parent)
		}
	case *ast.BinaryExpr:
		// A comparison with nil reads no element.
		return nil, ""
	case *ast.CallExpr:
		return w.argument(use, parent, object, occurrence)
	}
	if occurrence == link {
		return nil, ""
	}
	return nil, "ranged collection is used in a form whose effect on its elements is not modeled"
}

// argument admits a collection passed to len, cap, delete or clear, which
// never add an element, and to append when the result is assigned back to the
// same variable, which the definitions index already holds. Any other callee
// can store into the collection, so the call refuses wherever it can run.
func (w *externalElementWalk) argument(use externalRangeUse, call *ast.CallExpr, object *ast.Object, occurrence ast.Expr) ([]externalElementSource, string) {
	e := w.effects
	if call.Fun == occurrence {
		return nil, "ranged collection is called"
	}
	callee := sourceExpr{value: call.Fun, file: use.file, pkg: use.pkg}
	if e.builtin(callee) {
		name, identifier := callee.value.(*ast.Ident)
		if !identifier {
			return nil, "ranged collection builtin is unresolved"
		}
		switch name.Name {
		case externalBuiltinLen, externalBuiltinCap, externalBuiltinDelete, externalBuiltinClear:
			return nil, ""
		case externalBuiltinAppend:
			if len(call.Args) != 0 {
				if call.Args[0] == occurrence {
					if externalAppendsInPlace(e, use, call, object) {
						return nil, ""
					}
				}
			}
		}
		return nil, "ranged collection is passed to a builtin that can alias its elements"
	}
	if use.owner != nil {
		if _, scheduled := e.bodies[use.owner]; !scheduled {
			return nil, ""
		}
	}
	return nil, "ranged collection is passed to a call that can store into it"
}

// externalRangeConversion answers whether a type occurrence is the callee of
// a conversion, T(x) or (*T)(p): a struct value of identical underlying type
// converts into T and carries its field values with it.
func externalRangeConversion(use externalRangeUse) bool {
	switch parent := use.parents[0].(type) {
	case *ast.CallExpr:
		return parent.Fun == use.node
	case *ast.ParenExpr:
		return true
	case *ast.StarExpr:
		_, parenthesized := use.parents[1].(*ast.ParenExpr)
		return parenthesized
	}
	return false
}

// externalAppendsInPlace answers whether append(x, ...) is assigned back to x.
func externalAppendsInPlace(e *externalEffects, use externalRangeUse, call *ast.CallExpr, object *ast.Object) bool {
	if object == nil {
		return false
	}
	assignment, assigned := use.parents[1].(*ast.AssignStmt)
	if !assigned {
		return false
	}
	if len(assignment.Lhs) != len(assignment.Rhs) {
		return false
	}
	for index, right := range assignment.Rhs {
		if right != call {
			continue
		}
		return e.object(sourceExpr{value: assignment.Lhs[index], file: use.file, pkg: use.pkg}) == object
	}
	return false
}

// externalElementAccess admits an indexed element. A read does not change
// the population; a pointer element read out of it is then judged by the
// ordinary location model wherever it goes. A store adds its right-hand side.
// An element whose address is taken, whose method is called, or which is
// itself written through a selector, index or dereference can change a stored
// element no source shows, so it refuses.
func externalElementAccess(use externalRangeUse, index *ast.IndexExpr) ([]externalElementSource, string) {
	switch holder := use.parents[1].(type) {
	case *ast.AssignStmt:
		for position, left := range holder.Lhs {
			if left != index {
				continue
			}
			if holder.Tok != token.ASSIGN {
				return nil, "ranged collection element store form is unresolved"
			}
			if len(holder.Lhs) != len(holder.Rhs) {
				return nil, "ranged collection element is stored from a multi-value result"
			}
			return []externalElementSource{{source: sourceExpr{value: holder.Rhs[position], file: use.file, pkg: use.pkg}, owner: use.owner}}, ""
		}
		return nil, ""
	case *ast.IncDecStmt:
		return nil, "ranged collection element is incremented in place"
	case *ast.UnaryExpr:
		if holder.Op == token.AND {
			return nil, "ranged collection element address escapes"
		}
		return nil, ""
	case *ast.SelectorExpr:
		switch selected := use.parents[2].(type) {
		case *ast.AssignStmt:
			for _, left := range selected.Lhs {
				if left == holder {
					return nil, "ranged collection element field is written in place"
				}
			}
		case *ast.IncDecStmt:
			return nil, "ranged collection element field is incremented in place"
		case *ast.UnaryExpr:
			if selected.Op == token.AND {
				return nil, "ranged collection element field address escapes"
			}
		case *ast.CallExpr:
			if selected.Fun == holder {
				return nil, "ranged collection element method can change the element"
			}
		}
		return nil, ""
	case *ast.IndexExpr:
		if holder.X == index {
			return nil, "ranged collection element is a nested collection"
		}
		return nil, ""
	case *ast.SliceExpr:
		if holder.X == index {
			return nil, "ranged collection element is a nested collection"
		}
		return nil, ""
	case *ast.StarExpr:
		return nil, "ranged collection element is dereferenced in place"
	case *ast.RangeStmt:
		if holder.X == index {
			return nil, ""
		}
		return nil, "ranged collection element is a range target"
	}
	return nil, ""
}
