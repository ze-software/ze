// Design: docs/architecture/plugin/feature-gates.md -- bounded ordinary reflection over retained type descriptors.
package repoinventory

import (
	"go/ast"
	"go/types"
)

// externalReflectionWalk traverses descriptor edges without constructing a
// field-value tree or granting existing backing storage. Active paths are local
// to one summary; independently owned field summaries never share a visitor.
type externalReflectionWalk struct {
	effects *externalEffects
	path    externalReflectionActivePath
	// reuse is the running query's table of completed visits, nil outside a
	// query or when a test disabled reuse.
	reuse *externalReflectionReuse
	// probe is set only by tests: it disables reuse, shrinks the table, or
	// counts expanded visits. Production walkers leave it nil.
	probe *externalReflectionReuseProbe
}

type externalReflectionFrame struct {
	definition *ast.TypeSpec
	arguments  []externalTypeArgument
	methods    bool
	valueRoot  bool
}

// value owns one fresh output for each query, including queries on a reused
// walker. Recursive descriptor edges collect into that output without copying
// intermediate summaries. Active path lifetime remains scoped to recursion.
// The query also owns a fresh reuse table, which is dropped when it returns,
// so no proof crosses from one output union to another.
func (w *externalReflectionWalk) value(source sourceExpr, scope *externalTypeScope, methods, valueRoot bool, depth int) externalReflection {
	var result externalReflection
	outer := w.reuse
	w.reuse = newExternalReflectionReuse(w.probe)
	if len(w.path.frames) == 0 {
		w.path.linear = w.probe != nil && w.probe.linearPath
	}
	defer func() { w.reuse = outer }()
	w.collect(source, scope, methods, valueRoot, depth, &result)
	return result
}

// collect sites any unknown its descriptor produced at the type expression it
// walked. The output union is shared, so the innermost visit sites a reason.
func (w *externalReflectionWalk) collect(source sourceExpr, scope *externalTypeScope, methods, valueRoot bool, depth int, result *externalReflection) {
	w.descriptor(source, scope, methods, valueRoot, depth, result)
	result.locate(source)
}

// descriptor bounds descriptor expansion at the original 128 source edges.
// valueRoot suppresses only an actual interface value's absent descriptor.
func (w *externalReflectionWalk) descriptor(source sourceExpr, scope *externalTypeScope, methods, valueRoot bool, depth int, result *externalReflection) {
	if depth >= 128 {
		result.noteUnknown("reflection type capability depth exceeds 128")
		return
	}
	child := func(value ast.Expr, includeMethods bool, root bool) {
		w.collect(sourceExpr{value: value, file: source.file, pkg: source.pkg}, scope, includeMethods, root, depth+1, result)
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
			result.noteUnknown("generic reflected callable signature is unsupported")
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
		if argument, found := scope.argument(datatype); found {
			if argument.source.value == nil {
				result.noteUnknown("reflected type parameter has no retained argument")
				return
			}
			w.collect(argument.source, argument.scope, methods, valueRoot, depth+1, result)
			return
		}
		binding := datatype.Obj
		reason := ""
		if binding == nil {
			binding, reason = w.effects.typeIdentities.packageBinding(source.pkg, datatype.Name)
		}
		if reason != "" {
			result.noteUnknown(reason)
			return
		}
		if binding == nil {
			if _, builtin := types.Universe.Lookup(datatype.Name).(*types.TypeName); builtin {
				return
			}
			result.noteUnknown("reflected type ownership is unresolved: " + datatype.Name)
			return
		}
		w.collectNamed(source, nil, methods, valueRoot, depth+1, result)
		return
	case *ast.SelectorExpr:
		w.collectNamed(source, nil, methods, valueRoot, depth+1, result)
		return
	case *ast.IndexExpr:
		arguments := []externalTypeArgument{{source: sourceExpr{value: datatype.Index, file: source.file, pkg: source.pkg}, scope: scope}}
		w.collectNamed(sourceExpr{value: datatype.X, file: source.file, pkg: source.pkg}, arguments, methods, valueRoot, depth+1, result)
		return
	case *ast.IndexListExpr:
		arguments := make([]externalTypeArgument, 0, len(datatype.Indices))
		for _, argument := range datatype.Indices {
			arguments = append(arguments, externalTypeArgument{source: sourceExpr{value: argument, file: source.file, pkg: source.pkg}, scope: scope})
		}
		w.collectNamed(sourceExpr{value: datatype.X, file: source.file, pkg: source.pkg}, arguments, methods, valueRoot, depth+1, result)
		return
	}
	result.noteUnknown("unsupported reflected type capability shape")
}

func (w *externalReflectionWalk) collectNamed(source sourceExpr, arguments []externalTypeArgument, methods, valueRoot bool, depth int, result *externalReflection) {
	if depth >= 128 {
		result.noteUnknown("reflection type capability depth exceeds 128")
		return
	}
	if selector, selected := source.value.(*ast.SelectorExpr); selected {
		if origin, external := w.effects.source.externalImport(source, selector); external {
			w.collectExternal(origin, arguments, depth, result)
			return
		}
	}
	identity := externalTypeWalk{effects: w.effects}
	definition, owner, reason := identity.definition(source)
	if reason != "" {
		result.noteUnknown(reason)
		return
	}
	if len(arguments) != definition.TypeParams.NumFields() {
		result.noteUnknown("reflected generic type argument count is unresolved")
		return
	}
	// The old invalid-argument branch returned a new unknown-only summary,
	// discarding earlier arguments. Prove ownership before collecting any of
	// this node's arguments; valid arguments retain their original visit order.
	for index, argument := range arguments {
		arguments[index] = externalResolvedTypeArgument(argument, depth)
		if arguments[index].source.value == nil {
			result.noteUnknown("reflected generic type argument ownership is unresolved")
			return
		}
	}
	for _, argument := range arguments {
		w.collect(argument.source, argument.scope, true, false, depth+1, result)
	}
	frame := externalReflectionFrame{definition: definition, arguments: arguments, methods: methods, valueRoot: valueRoot}
	// The first active frame equal to this one is the backedge, as a scan of
	// the path from the outside in would find it; the path's index compares
	// only the frames that share this frame's definition and flags.
	if index := w.path.backedge(frame, depth); index >= 0 {
		if w.reuse != nil {
			w.reuse.pruned(index)
		}
		return
	}
	// Skip this visit when the output union already holds everything it could
	// add. An earlier visit of the same exact frame, owner and incoming depth
	// has completed in this query, and every active frame its subtree pruned
	// against is on the path now. Inside its subtree that visit read only the
	// frame, the depth, the scopes its arguments carry, and the active path,
	// and it read the path only in the backedge check above. So this visit
	// would make every prune the earlier one made, and perhaps more, and a
	// prune only cuts a subtree out: this walk would be the earlier walk with
	// parts removed. The union only accumulates (methods and origins are sets,
	// and the first unknown reason wins), so what this visit could add is
	// already there. Without that proof, or when the table is full, the visit
	// traverses as before.
	if w.reuse != nil {
		if w.reuse.covered(&w.path, frame, owner, depth) {
			return
		}
	}
	w.path.push(frame)
	defer w.path.pop()
	if w.probe != nil {
		w.probe.traversals++
	}
	if w.reuse != nil {
		w.reuse.enter(len(w.path.frames) - 1)
		// Runs before the path pop above, so the path still holds this frame.
		defer func() { w.reuse.complete(w.path.frames, frame, owner, depth) }()
	}
	scope := w.effects.typeIdentities.lexicalScope(owner)
	if len(arguments) != 0 {
		scope = externalBindTypeParameters(scope, definition.TypeParams, arguments)
	}
	if methods {
		w.collectMethods(definition, owner, depth+1, result)
	}
	w.collect(owner, scope, methods && definition.Assign.IsValid(), valueRoot, depth+1, result)
}

// collectMethods records only ordinary reflected callable methods. Signatures
// join this SAME traversal, so method-result chains cannot recursively launch
// independent walkers or borrow a different lexical owner.
func (w *externalReflectionWalk) collectMethods(definition *ast.TypeSpec, owner sourceExpr, depth int, result *externalReflection) {
	identity := externalTypeWalk{effects: w.effects}
	cursor := w.effects.methodCursor(owner.pkg, definition, depth)
	for row, present := cursor.next(); present; row, present = cursor.next() {
		function := row.function
		if !ast.IsExported(function.decl.Name.Name) {
			continue
		}
		// A raw row MUST pass this shape guard before resolution.
		if len(function.decl.Recv.List) != 1 {
			result.noteUnknown("reflected receiver shape is unresolved")
			continue
		}
		receiver := row.resolution(&identity, owner.pkg, depth)
		if receiver.reason != "" {
			result.noteUnknown(receiver.reason)
			continue
		}
		if receiver.definition != definition {
			continue
		}
		if receiver.generic {
			result.noteUnknown("generic reflected receiver execution is unsupported")
			continue
		}
		if function.decl.Body == nil {
			result.noteUnknown("bodyless reflected method execution is unsupported")
			continue
		}
		source := sourceExpr{value: function.decl.Name, file: function.file, pkg: owner.pkg}
		if result.methods == nil {
			result.methods = make(map[ast.Node]sourceExpr)
		}
		if _, found := result.methods[function.decl]; !found {
			result.methods[function.decl] = source
		}
		source.value = function.decl.Type
		w.collect(source, w.effects.typeIdentities.lexicalScope(source), true, false, depth+1, result)
	}
}

// collectExternal records a standard-library named type, bare or
// instantiated, as its import's origin: its descriptor is not an analyzer
// input. Its type arguments are retained descriptors and are still collected.
func (w *externalReflectionWalk) collectExternal(origin string, arguments []externalTypeArgument, depth int, result *externalReflection) {
	if err := w.effects.source.witness(origin); err != nil {
		result.noteUnknown(err.Error())
		return
	}
	if result.origins == nil {
		result.origins = make(map[string]bool)
	}
	result.origins[origin] = true
	for _, argument := range arguments {
		argument = externalResolvedTypeArgument(argument, depth)
		if argument.source.value == nil {
			result.noteUnknown("reflected generic type argument ownership is unresolved")
			return
		}
		w.collect(argument.source, argument.scope, true, false, depth+1, result)
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
