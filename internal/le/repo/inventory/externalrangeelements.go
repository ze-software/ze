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
// a collection's elements: a literal element, an element store, an append
// argument, or the value an in-place field write stores into one field of an
// element (field set). A literal element keeps its enclosing collection
// layout, so an element that elides its type is evaluated under the type the
// enclosing literal states, as the capability walk evaluates it there.
type externalElementSource struct {
	source sourceExpr
	owner  ast.Node
	layout *externalLiteralLayout
	// field names the element field an in-place write x[i].f = v, x[i].f op= v
	// or p.f = v through p := &x[i] sets; source is then v. Empty for a
	// whole-element source.
	field string
}

// externalIndexedObligation is the root-selector obligation an indexed read
// carries until its collection's element population is proved.
const externalIndexedObligation = "indexed element field ownership requires a retained element summary"

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
// filled through an alias the proof cannot follow refuses. An index
// expression reads one element of the same population (indexedValue). An empty proved
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
	result = e.withoutCollectionLocation(result, object, source)
	result.join(stored)
	return result
}

// withoutCollectionLocation drops the ranged collection variable's own
// location from a proved range value. Go specification, "For statements with
// range clause": the value slot holds "a[i] E", a copy of the element, so it
// never refers to the collection's backing storage; a reference into that
// storage an element does hold (a stored &x[j]) arrives through the proved
// element values, joined after this. Keeping the location would let a copy
// read from familySet make familySet writable wherever the copy escapes.
func (e *externalEffects) withoutCollectionLocation(value externalValue, object *ast.Object, source sourceExpr) externalValue {
	_, ranged, synthetic := externalRangeVariable(object)
	if !synthetic {
		return value
	}
	collection := e.object(sourceExpr{value: ast.Unparen(ranged.X), file: source.file, pkg: source.pkg})
	if collection == nil || !value.locations[collection] {
		return value
	}
	locations := make(map[*ast.Object]bool, len(value.locations))
	for location := range value.locations {
		if location != collection {
			locations[location] = true
		}
	}
	value.locations = locations
	return value
}

// elementValue evaluates one proved element source in its producer's
// execution mode, never its consumer's, as readObject evaluates a definition.
// A field write's value lands in that one field: its fields table holds only
// that field, and the element's other fields come from the sources that
// stored the whole element, so the join holds every value the field can have.
func (e *externalEffects) elementValue(element externalElementSource, depth int) externalValue {
	possible := e.definitionPossible(externalDefinition{source: element.source, owner: element.owner})
	if element.field != "" {
		stored := e.value(element.source, possible, depth+1)
		return externalValue{fields: map[string]externalValue{element.field: stored}}
	}
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

// indexedValue is the value an index expression x[k] reads. Go specification,
// "Index expressions": for an array "a[x] is the array element at index x and
// the type of a[x] is the element type of A", for a slice "a[x] is the slice
// element at index x and the type of a[x] is the element type of S", and for
// a map "a[x] is the map element with key x and the type of a[x] is the
// element type of M" (or the zero value when no entry exists). The read
// therefore holds exactly one stored element, as a range variable's value
// slot does, so it carries the same summary: the collection's and the key's
// whole flattened capability union, so no retained callback is lost, joined
// with every proved element value, which alone lends field names. Only a
// collection of retained structs (or pointers to them) is summarized: no
// other element has a field to read. A population that is not proved keeps
// the root-selector obligation with its reason.
//
// All reads of one collection variable share one population, so they share
// one per-round answer (indexedRound, which beginRound clears) and one
// monotone element value (indexedElements). A read the element sources make
// of the same collection, such as the swap a[i], a[j] = a[j], a[i], finds the
// round's answer already started and takes the element value joined so far;
// the next fixed-point round revisits it, as rangedValue does through
// reading.
func (e *externalEffects) indexedValue(source sourceExpr, index *ast.IndexExpr, collection, key externalValue, depth int) externalValue {
	result := externalFlat(collection, 0)
	result.join(externalFlat(key, 0))
	if collection.unresolvedFields != "" {
		// The collection came through a selector the analysis could not own,
		// so its elements cannot be summarized either.
		result.unresolvedFields = externalIndexedObligation + ": " + collection.unresolvedFields
		return result
	}
	identity := e.indexedIdentity(source, index)
	if reason, started := e.indexedRound[identity]; started {
		if reason != "" {
			result.unresolvedFields = externalIndexedObligation + ": " + reason
			return result
		}
		result.join(e.indexedElements[identity])
		return result
	}
	elements, reason := e.indexedElementSources(source, index, depth)
	e.indexedRound[identity] = reason
	if reason != "" {
		result.unresolvedFields = externalIndexedObligation + ": " + reason
		return result
	}
	var held externalValue
	for _, element := range elements {
		held.join(e.elementValue(element, depth))
		if e.amb.unknown != "" {
			return externalValue{unknown: e.amb.unknown}
		}
	}
	current := e.indexedElements[identity]
	if current.join(held) {
		e.indexedElements[identity] = current
		e.changed = true
	}
	result.join(held)
	return result
}

// indexedIdentity names the population an index expression reads: the
// collection variable, which every read of it shares, or the index expression
// itself when the collection is no variable.
func (e *externalEffects) indexedIdentity(source sourceExpr, index *ast.IndexExpr) any {
	if object := e.object(sourceExpr{value: ast.Unparen(index.X), file: source.file, pkg: source.pkg}); object != nil {
		return object
	}
	return index
}

// indexedElementSources answers the proved element population an index
// expression reads from, or the reason it is not proved. The element must be
// a retained struct, or a pointer to one, for any field to be read from it.
func (e *externalEffects) indexedElementSources(source sourceExpr, index *ast.IndexExpr, depth int) ([]externalElementSource, string) {
	element, reason := e.indexedElementType(source, index, depth+1)
	if reason != "" {
		return nil, reason
	}
	if pointer, indirect := ast.Unparen(element.value).(*ast.StarExpr); indirect {
		element.value = pointer.X
	}
	layout, reason := e.literalLayout(element, depth+1, false)
	if reason != "" {
		return nil, "indexed element layout is unresolved: " + reason
	}
	if layout.kind != externalLiteralStruct {
		return nil, "indexed element is not a retained struct"
	}
	walk := externalElementWalk{effects: e, visited: make(map[any]bool)}
	return walk.collection(sourceExpr{value: index.X, file: source.file, pkg: source.pkg}, nil, depth+1)
}

// indexedReceiverType answers the static element type of an index
// expression, once its collection's element population is proved, as
// rangedReceiverType answers a range variable's. A pointer element type
// selects the fields of the struct it points to. Go specification,
// "Selectors": "For a value x of type T or *T where T is not a pointer or
// interface type, x.f denotes the field or method at the shallowest depth in
// T where there is such an f." The round's population answer is read when
// indexedValue already gave it; otherwise the proof runs without recording
// one, because only indexedValue evaluates and records the element values.
func (e *externalEffects) indexedReceiverType(source sourceExpr, index *ast.IndexExpr, depth int) (sourceExpr, string) {
	reason, answered := e.indexedRound[e.indexedIdentity(source, index)]
	if !answered {
		_, reason = e.indexedElementSources(source, index, depth)
	}
	if reason != "" {
		return sourceExpr{}, externalIndexedObligation + ": " + reason
	}
	datatype, reason := e.indexedElementType(source, index, depth+1)
	if reason != "" {
		return sourceExpr{}, externalIndexedObligation + ": " + reason
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
	case *ast.SliceExpr:
		// Go specification, "Slice expressions": "Otherwise, if the result is
		// a slice, it shares its underlying array with the operand." Every
		// element of x[lo:hi] is an element of x, so its population is x's.
		operand := sourceExpr{value: value.X, file: source.file, pkg: source.pkg}
		return w.collection(operand, owner, depth+1)
	}
	return nil, "ranged collection source is not a retained variable, field, literal, make, append or reslice"
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
		stored, reason := w.use(use, link.value, false, depth+1)
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
		stored, reason := w.use(use, source.value, true, depth+1)
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
// element sources it stores. A reslice or an append assigned back to the same
// variable keeps the population, and an element address bound to one fresh
// pointer adds that pointer's field writes. An occurrence that can change the
// population in a way no source shows refuses: an alias other than link, an
// element whose address or method escapes, a call that receives the
// collection where that call can run. A call in a body no execution has
// reached cannot run, the same rule the capability walk applies to every
// call; the fixed point revisits this proof once that body is scheduled.
func (w *externalElementWalk) use(use externalRangeUse, link ast.Node, field bool, depth int) ([]externalElementSource, string) {
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
			if pointer, bound := externalElementPointer(use, parent); bound {
				return w.pointer(use, parent, pointer, depth+1)
			}
			if reason := w.belowElementField(use, parent, depth+1); reason != "" {
				return nil, reason
			}
			return externalElementAccess(use, parent)
		}
	case *ast.SliceExpr:
		// x = x[lo:hi] only drops elements, and the definitions index already
		// holds the right-hand side, as for append in place.
		if parent.X == occurrence {
			if externalAssignedInPlace(w.effects, use, parent) {
				return nil, ""
			}
		}
	case *ast.BinaryExpr:
		// A comparison with nil reads no element.
		return nil, ""
	case *ast.CallExpr:
		return w.argument(use, parent, occurrence)
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
func (w *externalElementWalk) argument(use externalRangeUse, call *ast.CallExpr, occurrence ast.Expr) ([]externalElementSource, string) {
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
					if externalAssignedInPlace(e, use, call) {
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

// externalAssignedInPlace answers whether an operation on the variable x the
// occurrence names, append(x, ...) or the reslice x[lo:hi], is assigned back
// to x itself. A field occurrence is never such a variable.
func externalAssignedInPlace(e *externalEffects, use externalRangeUse, operation ast.Expr) bool {
	if use.selector != nil {
		return false
	}
	target := e.object(sourceExpr{value: use.node, file: use.file, pkg: use.pkg})
	if target == nil {
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
		if right != operation {
			continue
		}
		return e.object(sourceExpr{value: assignment.Lhs[index], file: use.file, pkg: use.pkg}) == target
	}
	return false
}

// externalElementAccess admits an indexed element. A read does not change
// the population; a pointer element read out of it is then judged by the
// ordinary location model wherever it goes. A store adds its right-hand side,
// and an in-place field write x[i].f = v or x[i].f op= v adds v as the value
// of field f (externalFieldStore); x[i].f++ adds nothing. A chain below an
// inline field, x[i].f.g, is judged first by belowElementField. An element whose
// address escapes (other than into the one fresh pointer w.pointer judges),
// whose method is called, or which is itself written through an index or a
// dereference can change a stored element no source shows, so it refuses.
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
			for position, left := range selected.Lhs {
				if left == holder {
					return externalFieldStore(use, selected, position, holder.Sel.Name)
				}
			}
		case *ast.IncDecStmt:
			// Go specification, "IncDec statements": "The "++" and "--"
			// statements increment or decrement their operands by the untyped
			// constant 1." The field keeps a value of its own type and gains
			// no capability.
			return nil, ""
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

// externalFieldStore answers the element source an in-place field write
// stores. Go specification, "Assignment statements": "An assignment operation
// x op= y where op is a binary arithmetic operator is equivalent to x = x op
// (y) but evaluates x only once." The field then holds a value built from its
// old value, which another source already holds, and y, so y is the source in
// both forms; an assignment operation has exactly one operand on each side.
func externalFieldStore(use externalRangeUse, assignment *ast.AssignStmt, position int, field string) ([]externalElementSource, string) {
	if assignment.Tok == token.DEFINE {
		return nil, "collection element field store form is unresolved"
	}
	if len(assignment.Lhs) != len(assignment.Rhs) {
		return nil, "collection element field is written from a multi-value result"
	}
	return []externalElementSource{{
		source: sourceExpr{value: assignment.Rhs[position], file: use.file, pkg: use.pkg},
		owner:  use.owner,
		field:  field,
	}}, ""
}

// externalElementPointer answers the one fresh identifier p of p := &x[i],
// the only form in which an element's address is admitted. Go specification,
// "Address operators": "For an operand x of type T, the address operation &x
// generates a pointer of type *T to x." Every write through p is then a write
// into that element, which w.pointer judges at each occurrence of p.
func externalElementPointer(use externalRangeUse, index *ast.IndexExpr) (*ast.Ident, bool) {
	address, unary := use.parents[1].(*ast.UnaryExpr)
	if !unary {
		return nil, false
	}
	if address.Op != token.AND {
		return nil, false
	}
	if address.X != index {
		return nil, false
	}
	definition, assigned := use.parents[2].(*ast.AssignStmt)
	if !assigned {
		return nil, false
	}
	if definition.Tok != token.DEFINE {
		return nil, false
	}
	if len(definition.Lhs) != 1 {
		return nil, false
	}
	if len(definition.Rhs) != 1 {
		return nil, false
	}
	if definition.Rhs[0] != address {
		return nil, false
	}
	name, identifier := definition.Lhs[0].(*ast.Ident)
	if !identifier {
		return nil, false
	}
	if name.Name == "_" {
		return nil, false
	}
	return name, true
}

// pointer admits p := &x[i] when every occurrence of p reads or writes a
// direct field of the element, increments one, or compares p, and answers the
// field writes as element sources. Each occurrence is judged wherever it sits,
// a closure body included, so a write a closure makes later is still a source.
// Any other use can change the element through a path no source shows, and
// refuses with the use it met: p passed to a call, stored, returned,
// reassigned, dereferenced, its address taken, or a method called on it.
func (w *externalElementWalk) pointer(use externalRangeUse, index *ast.IndexExpr, name *ast.Ident, depth int) ([]externalElementSource, string) {
	e := w.effects
	object := e.object(sourceExpr{value: name, file: use.file, pkg: use.pkg})
	if object == nil {
		return nil, "collection element pointer " + name.Name + " binding is unresolved"
	}
	element, reason := e.indexedElementType(sourceExpr{value: index, file: use.file, pkg: use.pkg}, index, depth+1)
	if reason != "" {
		return nil, "collection element pointer " + name.Name + ": " + reason
	}
	var sources []externalElementSource
	for _, occurrence := range e.rangeUses(use.pkg).objects[object] {
		if occurrence.node == name {
			continue
		}
		stored, reason := e.pointerUse(occurrence, name.Name, element, depth+1)
		if reason != "" {
			return nil, reason
		}
		sources = append(sources, stored...)
	}
	return sources, ""
}

// pointerUse judges one occurrence of an element pointer p. A selector p.f
// must name a direct field of the element type, since a method could change
// the element through its receiver; the field occurrence is then judged by
// externalPointerField. A comparison reads only the pointer.
func (e *externalEffects) pointerUse(occurrence externalRangeUse, name string, element sourceExpr, depth int) ([]externalElementSource, string) {
	switch parent := occurrence.parents[0].(type) {
	case *ast.SelectorExpr:
		declared, reason := e.declaredFieldType(element, parent.Sel.Name, depth)
		if reason != "" {
			return nil, "collection element pointer " + name + " selects " + parent.Sel.Name + ", which is not a direct field of its element: " + reason
		}
		if below := e.belowField(declared, true, parent, occurrence.parents[1:], depth); below != "" {
			return nil, below
		}
		return externalPointerField(occurrence, parent)
	case *ast.BinaryExpr:
		return nil, ""
	case *ast.CallExpr:
		return nil, "collection element pointer " + name + " is passed to a call"
	case *ast.AssignStmt:
		if slices.Contains(parent.Lhs, occurrence.node) {
			return nil, "collection element pointer " + name + " is reassigned"
		}
		return nil, "collection element pointer " + name + " is stored"
	case *ast.ReturnStmt:
		return nil, "collection element pointer " + name + " is returned"
	case *ast.UnaryExpr:
		if parent.Op == token.AND {
			return nil, "collection element pointer " + name + " has its address taken"
		}
	case *ast.StarExpr:
		return nil, "collection element pointer " + name + " is dereferenced"
	}
	return nil, "collection element pointer " + name + " is used in a form whose effect on its element is not modeled"
}

// externalPointerField judges a field occurrence p.f of an element pointer.
// A write p.f = v or p.f op= v adds v as the value of field f; p.f++ adds
// nothing; &p.f lets another writer change the field, so it refuses. Every
// other occurrence reads p.f, a selector or method call on p.f included.
func externalPointerField(occurrence externalRangeUse, selector *ast.SelectorExpr) ([]externalElementSource, string) {
	switch holder := occurrence.parents[1].(type) {
	case *ast.AssignStmt:
		for position, left := range holder.Lhs {
			if left == selector {
				return externalFieldStore(occurrence, holder, position, selector.Sel.Name)
			}
		}
	case *ast.UnaryExpr:
		if holder.Op == token.AND {
			return nil, "collection element pointer field " + selector.Sel.Name + " address escapes"
		}
	}
	return nil, ""
}

// belowElementField judges a chain below a field of an indexed element,
// x[i].f.g or x[i].f[j], by the declared type of f in the element struct
// (externalEffects.belowField). An element type the proof cannot name leaves
// f's type unresolved, which belowField judges as an inline value.
func (w *externalElementWalk) belowElementField(use externalRangeUse, index *ast.IndexExpr, depth int) string {
	e := w.effects
	selector, selected := use.parents[1].(*ast.SelectorExpr)
	if !selected {
		return ""
	}
	if selector.X != index {
		return ""
	}
	element, reason := e.indexedElementType(sourceExpr{value: index, file: use.file, pkg: use.pkg}, index, depth+1)
	if reason != "" {
		return e.belowField(sourceExpr{}, false, selector, use.parents[2:], depth)
	}
	if pointer, indirect := ast.Unparen(element.value).(*ast.StarExpr); indirect {
		element.value = pointer.X
	}
	declared, reason := e.declaredFieldType(element, selector.Sel.Name, depth+1)
	return e.belowField(declared, reason == "", selector, use.parents[2:], depth)
}

// belowField judges the chain below an element field f, given f's declared
// type when known. A field of a reference type (pointer, map, slice, channel,
// function, interface) names a referent: a write below it lands there, and
// the ordinary location model judges it, so the chain is not judged here; a
// basic type has nothing below it to write. A
// struct or array value, or a type the proof cannot resolve, is stored inline
// in the element, so a write anywhere below f changes the element in place:
// externalBelowField refuses it.
func (e *externalEffects) belowField(declared sourceExpr, known bool, field *ast.SelectorExpr, above []ast.Node, depth int) string {
	if known {
		if e.fieldHoldsReference(declared, depth+1) {
			return ""
		}
	}
	return externalBelowField(field, above)
}

// fieldHoldsReference answers whether nothing below a field's value is
// stored inline in the element: its declared type refers to its value (a
// pointer, map, slice, channel, function or interface type), or is a
// predeclared basic type, which has no part to write (a string's bytes are
// immutable). An unresolved type answers false, so it is judged inline.
func (e *externalEffects) fieldHoldsReference(declared sourceExpr, depth int) bool {
	shape, known := e.resolver.underlyingType(declared, depth+1)
	if !known {
		return false
	}
	switch kind := shape.value.(type) {
	case *ast.StarExpr, *ast.MapType, *ast.ChanType, *ast.FuncType, *ast.InterfaceType:
		return true
	case *ast.ArrayType:
		return kind.Len == nil
	case *ast.Ident:
		_, basic := basicType(shape)
		return basic
	}
	return false
}

// externalBelowField judges the chain below an inline element field f: the
// selectors, indexes and parentheses that start at field, within the nodes
// above it the occurrence index retained (above, nearest first). Directly
// above f the field itself is judged elsewhere, so no chain answers "". A
// chain that is the left side of an assignment or an assignment operation, an
// IncDec operand, an address operand, a sliced array, or the callee of a call
// (a method on an inline value can change it through its receiver) refuses,
// and so does a chain whose context lies past the retained nodes, since its
// effect cannot be seen. Any other context reads the chain.
func externalBelowField(field *ast.SelectorExpr, above []ast.Node) string {
	name := field.Sel.Name
	var current ast.Expr = field
	for position, node := range above {
		if step := externalChainStep(node, current); step != nil {
			current = step
			continue
		}
		// Go specification, "Slice expressions": "Otherwise, if the result is
		// a slice, it shares its underlying array with the operand." A slice
		// of an inline array field lets a later write reach the element.
		if sliced, slicing := node.(*ast.SliceExpr); slicing {
			if sliced.X == current {
				return "collection element field " + name + " is sliced below the field"
			}
		}
		if position == 0 {
			return ""
		}
		switch context := node.(type) {
		case *ast.AssignStmt:
			if slices.Contains(context.Lhs, current) {
				return "collection element field " + name + " is written below the field"
			}
		case *ast.IncDecStmt:
			return "collection element field " + name + " is incremented below the field"
		case *ast.UnaryExpr:
			if context.Op == token.AND {
				return "collection element field " + name + " has an address taken below the field"
			}
		case *ast.CallExpr:
			if context.Fun == current {
				return "collection element field " + name + " has a method called below the field"
			}
		}
		return ""
	}
	if current == field {
		return ""
	}
	return "collection element field " + name + " is used below the field past the retained context"
}

// externalChainStep answers node when it continues a selector chain at
// current, x.g, x[j] or (x), and nil otherwise.
func externalChainStep(node ast.Node, current ast.Expr) ast.Expr {
	switch step := node.(type) {
	case *ast.SelectorExpr:
		if step.X == current {
			return step
		}
	case *ast.IndexExpr:
		if step.X == current {
			return step
		}
	case *ast.ParenExpr:
		if step.X == current {
			return step
		}
	}
	return nil
}
