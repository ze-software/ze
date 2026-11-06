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
	path    []externalReflectionFrame
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
func (w *externalReflectionWalk) value(source sourceExpr, scope *externalTypeScope, methods, valueRoot bool, depth int) externalReflection {
	var result externalReflection
	w.collect(source, scope, methods, valueRoot, depth, &result)
	return result
}

// collect bounds descriptor expansion at the original 128 source edges.
// valueRoot suppresses only an actual interface value's absent descriptor.
func (w *externalReflectionWalk) collect(source sourceExpr, scope *externalTypeScope, methods, valueRoot bool, depth int, result *externalReflection) {
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
		if identity, external := w.effects.source.externalImport(source, datatype); external {
			if err := w.effects.source.witness(identity); err != nil {
				result.noteUnknown(err.Error())
				return
			}
			if result.origins == nil {
				result.origins = make(map[string]bool)
			}
			result.origins[identity] = true
			return
		}
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
	for _, active := range w.path {
		if externalReflectionFrameEqual(active, frame, depth) {
			return
		}
	}
	w.path = append(w.path, frame)
	defer func() { w.path = w.path[:len(w.path)-1] }()
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

func (r *externalReflection) noteUnknown(reason string) {
	if r.unknown == "" {
		r.unknown = reason
	}
}
