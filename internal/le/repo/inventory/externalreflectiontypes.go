// Design: docs/architecture/plugin/feature-gates.md -- bounded ordinary reflection over retained type descriptors.
package repoinventory

import (
	"go/ast"
	"go/types"
)

// externalReflectionWalk walks descriptor edges by reachability, without
// constructing a field-value tree or granting existing backing storage.
//
// Each exact frame (definition, resolved arguments, method and value-root
// flags) contributes the union of what its own walk meets and what the frames
// it reaches contribute, so its contribution is the same from every query.
// The walk computes it once per analysis with Tarjan's algorithm over the
// frames: a frame met again while it is still open is a cycle, and closes no
// path; the frames of one strongly connected component share one union, kept
// for every member in the analysis-wide table (externalEffects.reflectionFrames)
// when the component's first frame completes; every later visit joins the
// table's entry. The walk recursion is bounded by the number of distinct
// frames, since an open frame is never entered again, and by the nesting of
// retained type expressions.
//
// The first unknown reason of a union is the first one the walk met in source
// order, frames joined at the moment it reached them. A component's union
// keeps the first reason met by the walk that computed it, so every member,
// and every later query reaching any of them, carries that reason; an unsited
// reason is sited at the type expression that joins it, as before.
//
// Not safe for concurrent use; one analysis runs on one goroutine.
type externalReflectionWalk struct {
	effects *externalEffects
	// order counts the reasons and joins the walk has met, so each part
	// records when its first reason arrived.
	order int
	// active maps an open frame's id to its position on stack; stack holds
	// the open frames in the order they were entered, and calls the frames
	// whose walk is running, innermost last.
	active map[int32]int
	stack  []*externalReflectionOpen
	calls  []*externalReflectionOpen
	// instantiating holds the receiver type parameters whose instantiations
	// are being collected (collectInstantiations).
	instantiating map[externalReceiverParameter]bool
}

// externalReflectionPart is the union one walk collects into: a query's
// answer or an open frame's own contribution. order is when its first
// unknown reason arrived (externalReflectionWalk.order).
type externalReflectionPart struct {
	reflection externalReflection
	order      int
}

// externalReflectionOpen is a frame entered and not yet stored. low is the
// lowest stack position its walk reached through frames still open.
type externalReflectionOpen struct {
	frame int32
	low   int
	part  externalReflectionPart
}

// externalReflectionFrameKind is the frame kind the reflection walk interns:
// distinct from every type-walk kind, with its two flags.
func externalReflectionFrameKind(methods, valueRoot bool) uint8 {
	kind := uint8(4)
	if methods {
		kind |= 2
	}
	if valueRoot {
		kind |= 1
	}
	return kind
}

// value owns one fresh output for each query. A query that starts at an
// incoming expression depth of 128 or more refuses, as the value flow that
// asks it does; past that guard the answer does not depend on the depth.
func (w *externalReflectionWalk) value(source sourceExpr, scope *externalTypeScope, methods, valueRoot bool, depth int) externalReflection {
	var result externalReflectionPart
	if counters := w.effects.counters; counters != nil {
		counters.reflectionQueries++
	}
	if depth >= 128 {
		w.noteUnknown(&result, "reflection type capability depth exceeds 128")
		result.reflection.locate(source)
		return result.reflection
	}
	w.collect(source, scope, methods, valueRoot, &result)
	return result.reflection
}

// collect sites any unknown its descriptor produced at the type expression it
// walked. The innermost collect over an unsited reason sites it.
func (w *externalReflectionWalk) collect(source sourceExpr, scope *externalTypeScope, methods, valueRoot bool, result *externalReflectionPart) {
	w.descriptor(source, scope, methods, valueRoot, result)
	result.reflection.locate(source)
}

// noteUnknown notes a reason in the part, which keeps its first one.
func (w *externalReflectionWalk) noteUnknown(result *externalReflectionPart, reason string) {
	w.order++
	if result.reflection.unknown != "" {
		return
	}
	result.reflection.noteUnknown(reason)
	result.order = w.order
}

// join adds a stored frame contribution to the part at this point of the walk.
func (w *externalReflectionWalk) join(result *externalReflectionPart, contribution externalReflection) {
	w.order++
	first := result.reflection.unknown == ""
	result.reflection.join(contribution)
	if first {
		if result.reflection.unknown != "" {
			result.order = w.order
		}
	}
}

// descriptor walks one type expression. Its recursion follows the retained
// expression and the parameter substitutions it mentions, both finite; a cycle
// passes through a named frame, which collectNamed closes. valueRoot
// suppresses only an actual interface value's absent descriptor.
func (w *externalReflectionWalk) descriptor(source sourceExpr, scope *externalTypeScope, methods, valueRoot bool, result *externalReflectionPart) {
	child := func(value ast.Expr, includeMethods bool, root bool) {
		w.collect(sourceExpr{value: value, file: source.file, pkg: source.pkg}, scope, includeMethods, root, result)
	}
	switch datatype := source.value.(type) {
	case *ast.ParenExpr:
		child(datatype.X, methods, valueRoot)
		return
	case *ast.StarExpr:
		child(datatype.X, true, false)
		return
	case *ast.ArrayType:
		child(datatype.Elt, true, false)
		return
	case *ast.Ellipsis:
		child(datatype.Elt, true, false)
		return
	case *ast.MapType:
		child(datatype.Key, true, false)
		child(datatype.Value, true, false)
		return
	case *ast.ChanType:
		child(datatype.Value, true, false)
		return
	case *ast.StructType:
		for _, field := range datatype.Fields.List {
			child(field.Type, true, false)
		}
		return
	case *ast.FuncType:
		if datatype.TypeParams.NumFields() != 0 {
			w.noteUnknown(result, "generic reflected callable signature is unsupported")
			return
		}
		for _, fields := range []*ast.FieldList{datatype.Params, datatype.Results} {
			if fields == nil {
				continue
			}
			for _, field := range fields.List {
				child(field.Type, true, false)
			}
		}
		return
	case *ast.InterfaceType:
		if valueRoot {
			return
		}
		for _, field := range datatype.Methods.List {
			child(field.Type, true, false)
		}
		return
	case *ast.Ident:
		if binding, found := scope.binding(datatype); found {
			argument := binding.argument
			if argument.source.value == nil && binding.receiver != nil {
				w.collectInstantiations(*binding.receiver, methods, valueRoot, result)
				return
			}
			if argument.source.value == nil {
				w.noteUnknown(result, "reflected type parameter has no retained argument")
				return
			}
			w.collect(argument.source, argument.scope, methods, valueRoot, result)
			return
		}
		binding := datatype.Obj
		reason := ""
		if binding == nil {
			binding, reason = w.effects.typeIdentities.packageBinding(source.pkg, datatype.Name)
		}
		if reason != "" {
			w.noteUnknown(result, reason)
			return
		}
		if binding == nil {
			if _, builtin := types.Universe.Lookup(datatype.Name).(*types.TypeName); builtin {
				return
			}
			w.noteUnknown(result, "reflected type ownership is unresolved: "+datatype.Name)
			return
		}
		w.collectNamed(source, nil, methods, valueRoot, result)
		return
	case *ast.SelectorExpr:
		w.collectNamed(source, nil, methods, valueRoot, result)
		return
	case *ast.IndexExpr:
		arguments := []externalTypeArgument{{source: sourceExpr{value: datatype.Index, file: source.file, pkg: source.pkg}, scope: scope}}
		w.collectNamed(sourceExpr{value: datatype.X, file: source.file, pkg: source.pkg}, arguments, methods, valueRoot, result)
		return
	case *ast.IndexListExpr:
		arguments := make([]externalTypeArgument, 0, len(datatype.Indices))
		for _, argument := range datatype.Indices {
			arguments = append(arguments, externalTypeArgument{source: sourceExpr{value: argument, file: source.file, pkg: source.pkg}, scope: scope})
		}
		w.collectNamed(sourceExpr{value: datatype.X, file: source.file, pkg: source.pkg}, arguments, methods, valueRoot, result)
		return
	}
	w.noteUnknown(result, "unsupported reflected type capability shape")
}

// collectNamed joins a named type's frame contribution, computing it first
// when the analysis holds none (externalReflectionWalk).
func (w *externalReflectionWalk) collectNamed(source sourceExpr, arguments []externalTypeArgument, methods, valueRoot bool, result *externalReflectionPart) {
	named := w.effects.reflectionNamed(source)
	if named.external {
		w.collectExternal(named.origin, arguments, result)
		return
	}
	if named.reason != "" {
		w.noteUnknown(result, named.reason)
		return
	}
	definition := named.definition
	if len(arguments) != definition.TypeParams.NumFields() {
		w.noteUnknown(result, "reflected generic type argument count is unresolved")
		return
	}
	// The old invalid-argument branch returned a new unknown-only summary,
	// discarding earlier arguments. Prove ownership before collecting any of
	// this node's arguments; valid arguments retain their original visit order.
	for index, argument := range arguments {
		arguments[index] = externalResolvedTypeArgument(argument, 0)
		if arguments[index].source.value == nil {
			w.noteUnknown(result, "reflected generic type argument ownership is unresolved")
			return
		}
	}
	for _, argument := range arguments {
		w.collect(argument.source, argument.scope, true, false, result)
	}
	if counters := w.effects.counters; counters != nil {
		counters.namedVisits++
		counters.tick()
	}
	// An instantiation that grows without end reaches a new frame on every
	// step, and its argument term is what keeps nesting
	// (externalTypeTermDepthMax).
	id, bounded := w.effects.typeTerms.frame(named.ownerID, externalReflectionFrameKind(methods, valueRoot), arguments)
	if !bounded {
		w.noteUnknown(result, "reflection type capability depth exceeds 128")
		return
	}
	if contribution, stored := w.effects.reflectionFrames[id]; stored {
		w.join(result, contribution)
		return
	}
	if position, open := w.active[id]; open {
		// A cycle: the frame's contribution reaches this part through the
		// union its component shares.
		w.lower(position)
		return
	}
	open := w.open(id)
	w.expand(named, arguments, methods, valueRoot, &open.part)
	w.calls = w.calls[:len(w.calls)-1]
	position := w.active[id]
	if open.low < position {
		w.lower(open.low)
		return
	}
	w.store(position)
	w.join(result, w.effects.reflectionFrames[id])
}

// open enters frame id as the innermost open frame and the running walk.
func (w *externalReflectionWalk) open(id int32) *externalReflectionOpen {
	position := len(w.stack)
	open := &externalReflectionOpen{frame: id, low: position}
	w.stack = append(w.stack, open)
	w.calls = append(w.calls, open)
	if w.active == nil {
		w.active = make(map[int32]int)
	}
	w.active[id] = position
	if counters := w.effects.counters; counters != nil {
		counters.reflectionFrames++
	}
	return open
}

// lower records that the running frame's walk reached the open frame at
// position. Outside every frame (a query's own part) no frame is open, so
// nothing it reaches is open either.
func (w *externalReflectionWalk) lower(position int) {
	if len(w.calls) == 0 {
		return
	}
	running := w.calls[len(w.calls)-1]
	running.low = min(running.low, position)
}

// store closes the component whose first frame is at position: its frames
// share the union of their parts, which the table keeps for each of them.
// The union's first reason is the earliest any member met.
func (w *externalReflectionWalk) store(position int) {
	members := w.stack[position:]
	first := -1
	for index, member := range members {
		if member.part.reflection.unknown == "" {
			continue
		}
		if first < 0 {
			first = index
			continue
		}
		if member.part.order < members[first].part.order {
			first = index
		}
	}
	var union externalReflection
	if first >= 0 {
		union.join(members[first].part.reflection)
	}
	for _, member := range members {
		union.join(member.part.reflection)
	}
	if w.effects.reflectionFrames == nil {
		w.effects.reflectionFrames = make(map[int32]externalReflection)
	}
	for _, member := range members {
		w.effects.reflectionFrames[member.frame] = union
		delete(w.active, member.frame)
	}
	w.stack = w.stack[:position]
}

// expand runs a frame's own walk into its part: the methods its definition
// declares, and its representation in the definition's scope.
func (w *externalReflectionWalk) expand(named externalReflectionNamed, arguments []externalTypeArgument, methods, valueRoot bool, part *externalReflectionPart) {
	definition, owner := named.definition, named.owner
	scope := w.effects.typeIdentities.lexicalScope(owner)
	if len(arguments) != 0 {
		scope = externalBindTypeParameters(scope, definition.TypeParams, arguments)
	}
	if methods {
		w.collectMethods(definition, owner, arguments, part)
	}
	w.collect(owner, scope, methods && definition.Assign.IsValid(), valueRoot, part)
}

// collectMethods records only ordinary reflected callable methods. Signatures
// join this SAME traversal, so method-result chains cannot recursively launch
// independent walkers or borrow a different lexical owner.
func (w *externalReflectionWalk) collectMethods(definition *ast.TypeSpec, owner sourceExpr, arguments []externalTypeArgument, result *externalReflectionPart) {
	identity := externalTypeWalk{effects: w.effects}
	cursor := w.effects.methodCursor(owner.pkg, definition)
	for row, present := cursor.next(); present; row, present = cursor.next() {
		function := row.function
		if !ast.IsExported(function.decl.Name.Name) {
			continue
		}
		// A raw row MUST pass this shape guard before resolution.
		if len(function.decl.Recv.List) != 1 {
			w.noteUnknown(result, "reflected receiver shape is unresolved")
			continue
		}
		receiver := row.resolution(&identity, owner.pkg)
		if receiver.reason != "" {
			w.noteUnknown(result, receiver.reason)
			continue
		}
		if receiver.definition != definition {
			continue
		}
		if function.decl.Body == nil {
			w.noteUnknown(result, "bodyless reflected method execution is unsupported")
			continue
		}
		source := sourceExpr{value: function.decl.Name, file: function.file, pkg: owner.pkg}
		source.value = function.decl.Type
		scope := w.effects.typeIdentities.lexicalScope(source)
		if receiver.generic {
			// The method's signature names the receiver's type parameters,
			// which this instantiation's arguments bind in declaration order.
			if len(externalReceiverTypeParameters(function.decl)) != len(arguments) {
				w.noteUnknown(result, "reflected generic type argument count is unresolved")
				continue
			}
			scope = externalBindReceiverTypeParameters(scope, function.decl, function.file, owner.pkg, arguments)
		}
		if result.reflection.methods == nil {
			result.reflection.methods = make(map[ast.Node]sourceExpr)
		}
		if _, found := result.reflection.methods[function.decl]; !found {
			result.reflection.methods[function.decl] = sourceExpr{value: function.decl.Name, file: function.file, pkg: owner.pkg}
		}
		w.collect(source, scope, true, false, result)
	}
}

// collectInstantiations reflects a method's receiver type parameter that no
// instantiation binds, as the method runs when its function value is exposed:
// a value of that type has one of the types the program instantiates the
// receiver with, so each written argument is collected (receiverInstantiations).
// A parameter already being collected on this walk refuses, because its
// instantiations include itself through another generic type and the union
// would be read before it is complete.
func (w *externalReflectionWalk) collectInstantiations(parameter externalReceiverParameter, methods, valueRoot bool, result *externalReflectionPart) {
	if w.instantiating[parameter] {
		w.noteUnknown(result, "reflected receiver type parameter instantiates itself")
		return
	}
	arguments, reason := w.effects.receiverInstantiations(parameter)
	if reason != "" {
		w.noteUnknown(result, reason)
		return
	}
	if w.instantiating == nil {
		w.instantiating = make(map[externalReceiverParameter]bool)
	}
	w.instantiating[parameter] = true
	for _, argument := range arguments {
		w.collect(argument.source, argument.scope, methods, valueRoot, result)
	}
	delete(w.instantiating, parameter)
}

// collectExternal records a standard-library named type, bare or
// instantiated, as its import's origin: its descriptor is not an analyzer
// input. Its type arguments are retained descriptors and are still collected.
func (w *externalReflectionWalk) collectExternal(origin string, arguments []externalTypeArgument, result *externalReflectionPart) {
	if err := w.effects.source.witness(origin); err != nil {
		w.noteUnknown(result, err.Error())
		return
	}
	if result.reflection.origins == nil {
		result.reflection.origins = make(map[string]bool)
	}
	result.reflection.origins[origin] = true
	for _, argument := range arguments {
		argument = externalResolvedTypeArgument(argument, 0)
		if argument.source.value == nil {
			w.noteUnknown(result, "reflected generic type argument ownership is unresolved")
			return
		}
		w.collect(argument.source, argument.scope, true, false, result)
	}
}

// locate sites an unsited first reason; see externalValue.locate.
func (r *externalReflection) locate(source sourceExpr) {
	if r.unknown == "" {
		return
	}
	if r.unknownAt.file != nil {
		return
	}
	r.unknownAt = source
}

func (r *externalReflection) noteUnknown(reason string) {
	if r.unknown == "" {
		r.unknown = reason
	}
}
