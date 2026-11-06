// Design: docs/architecture/plugin/feature-gates.md -- conservative retained type capabilities.
package repoinventory

import (
	"go/ast"
	"go/types"
	"slices"
)

// externalTypeWalk proves a capability union, never a certain method owner.
// Its zero path is empty; effects is supplied by the report's private reader.
type externalTypeWalk struct {
	effects    *externalEffects
	path       []externalTypeFrame
	references bool
	reflection bool
}

type externalTypeFrame struct {
	definition *ast.TypeSpec
	arguments  []externalTypeArgument
	methods    bool
}

// externalTypeArgument preserves the source and substitution scope of an
// actual argument. An absent source denotes an unconstrained ambient parameter.
type externalTypeArgument struct {
	source sourceExpr
	scope  *externalTypeScope
}

type externalTypeBinding struct {
	object   *ast.Object
	argument externalTypeArgument
}

// A nil scope contains no parameters. Bindings are local to one declaration;
// equal spellings in another file or generic declaration never share a key.
type externalTypeScope struct {
	parent     *externalTypeScope
	parameters map[string]externalTypeBinding
}

// value sites any unknown its shape produced at the type expression it walked.
func (w *externalTypeWalk) value(source sourceExpr, scope *externalTypeScope, methods bool, depth int) externalValue {
	result := w.shape(source, scope, methods, depth)
	result.locate(source)
	return result
}

func (w *externalTypeWalk) shape(source sourceExpr, scope *externalTypeScope, methods bool, depth int) externalValue {
	if depth >= 128 {
		return externalValue{unknown: "type capability depth exceeds 128"}
	}
	child := func(value ast.Expr, includeMethods bool) externalValue {
		return w.value(sourceExpr{value: value, file: source.file, pkg: source.pkg}, scope, includeMethods, depth+1)
	}
	switch datatype := source.value.(type) {
	case *ast.ParenExpr:
		return child(datatype.X, methods)
	case *ast.StarExpr:
		w.references = true
		result := child(datatype.X, true)
		result.references = true
		return result
	case *ast.ArrayType:
		result := externalFlat(child(datatype.Elt, true), 0)
		if datatype.Len == nil {
			w.references = true
			result.references = true
		}
		return result
	case *ast.MapType:
		w.references = true
		result := externalFlat(child(datatype.Key, true), 0)
		result.join(externalFlat(child(datatype.Value, true), 0))
		result.references = true
		return result
	case *ast.ChanType:
		// A channel is a reference container like a slice, whatever its
		// direction: what one holder sends any holder can receive, so its
		// possible contents are the flattened capability of its element type.
		w.references = true
		result := externalFlat(child(datatype.Value, true), 0)
		result.references = true
		return result
	case *ast.StructType:
		result := externalValue{fields: make(map[string]externalValue)}
		for _, field := range datatype.Fields.List {
			value := child(field.Type, true)
			if w.reflection {
				value.reflection.join(w.effects.reflectedValue(sourceExpr{value: field.Type, file: source.file, pkg: source.pkg}, scope, depth+1))
			}
			result.join(externalFlat(value, 0))
			if len(field.Names) == 0 {
				name := externalEmbeddedTypeName(field.Type)
				if name == "" {
					result.join(externalValue{unknown: "embedded type capability ownership is unresolved"})
					continue
				}
				current := result.fields[name]
				current.join(value)
				result.fields[name] = current
				continue
			}
			for _, name := range field.Names {
				current := result.fields[name.Name]
				current.join(value)
				result.fields[name.Name] = current
			}
		}
		return result
	case *ast.FuncType, *ast.InterfaceType:
		// Stored function/interface values are joined by the binding equations;
		// a type alone does not identify a retained implementation or closure.
		return externalValue{}
	case *ast.Ident:
		if argument, found := scope.argument(datatype); found {
			if argument.source.value == nil {
				w.references = true
				result := externalFlat(w.effects.amb, 0)
				result.references = true
				return result
			}
			return w.value(argument.source, argument.scope, methods, depth+1)
		}
		binding := datatype.Obj
		reason := ""
		if binding == nil {
			binding, reason = w.effects.typeIdentities.packageBinding(source.pkg, datatype.Name)
		}
		if reason != "" {
			return externalValue{unknown: reason}
		}
		if binding == nil {
			if _, builtin := types.Universe.Lookup(datatype.Name).(*types.TypeName); builtin {
				return externalValue{}
			}
			return externalValue{unknown: "retained type capability ownership is unresolved: " + datatype.Name}
		}
		if _, parameter := binding.Decl.(*ast.Field); parameter {
			if binding.Kind == ast.Typ {
				w.references = true
				result := externalFlat(w.effects.amb, 0)
				result.references = true
				return result
			}
		}
		return w.named(source, nil, methods, depth+1)
	case *ast.SelectorExpr:
		return w.named(source, nil, methods, depth+1)
	case *ast.IndexExpr:
		arguments := []externalTypeArgument{{source: sourceExpr{value: datatype.Index, file: source.file, pkg: source.pkg}, scope: scope}}
		return w.named(sourceExpr{value: datatype.X, file: source.file, pkg: source.pkg}, arguments, methods, depth+1)
	case *ast.IndexListExpr:
		arguments := make([]externalTypeArgument, 0, len(datatype.Indices))
		for _, argument := range datatype.Indices {
			arguments = append(arguments, externalTypeArgument{source: sourceExpr{value: argument, file: source.file, pkg: source.pkg}, scope: scope})
		}
		return w.named(sourceExpr{value: datatype.X, file: source.file, pkg: source.pkg}, arguments, methods, depth+1)
	}
	return externalValue{unknown: "unsupported retained type capability shape"}
}

// named is the one entrance for a named type, bare or instantiated, so a
// standard-library type is external on every route that reaches it.
func (w *externalTypeWalk) named(source sourceExpr, arguments []externalTypeArgument, methods bool, depth int) externalValue {
	if depth >= 128 {
		return externalValue{unknown: "type capability depth exceeds 128"}
	}
	if selector, selected := source.value.(*ast.SelectorExpr); selected {
		if identity, external := w.effects.source.externalImport(source, selector); external {
			return w.external(identity, arguments, depth)
		}
	}
	definition, owner, reason := w.definition(source)
	if reason != "" {
		return externalValue{unknown: reason}
	}
	parameters := definition.TypeParams.NumFields()
	if arguments != nil {
		if len(arguments) != parameters {
			return externalValue{unknown: "generic type capability argument count is unresolved"}
		}
	}
	var result externalValue
	for index, argument := range arguments {
		argument = externalResolvedTypeArgument(argument, depth)
		arguments[index] = argument
		if argument.source.value == nil {
			w.references = true
			result.references = true
			result.join(externalFlat(w.effects.amb, 0))
			continue
		}
		result.join(externalFlat(w.value(argument.source, argument.scope, true, depth+1), 0))
	}
	if result.unknown != "" {
		return result
	}
	frame := externalTypeFrame{definition: definition, arguments: arguments, methods: methods}
	for _, active := range w.path {
		if active.definition != frame.definition {
			continue
		}
		if active.methods != frame.methods {
			continue
		}
		if slices.Equal(active.arguments, frame.arguments) {
			result.join(externalValue{unknown: "cyclic retained type capability shape: " + definition.Name.Name})
			return result
		}
	}
	w.path = append(w.path, frame)
	defer func() { w.path = w.path[:len(w.path)-1] }()
	scope := w.effects.typeIdentities.lexicalScope(owner)
	if parameters != 0 {
		scope = externalBindTypeParameters(scope, definition.TypeParams, arguments)
	}
	// A defined type has its own method set; only an alias inherits the target's
	// top-level methods. Fields within either representation retain their own.
	result.join(w.value(owner, scope, methods && definition.Assign.IsValid(), depth+1))
	if methods {
		result.join(w.methods(definition, owner, depth+1))
	}
	return result
}

// external models a standard-library named type the way an external call is
// modelled: its source is not an analyzer input, so the type contributes its
// import's origin and may reach every ambient capability. Its type arguments
// are still retained types in this scope: atomic.Pointer[hook] keeps hook's
// methods, so a registering argument is still tracked.
func (w *externalTypeWalk) external(identity string, arguments []externalTypeArgument, depth int) externalValue {
	if err := w.effects.source.witness(identity); err != nil {
		return externalValue{unknown: err.Error()}
	}
	result := externalOrigin(identity)
	w.references = true
	result.join(externalFlat(w.effects.amb, 0))
	for _, argument := range arguments {
		argument = externalResolvedTypeArgument(argument, depth)
		if argument.source.value == nil {
			continue
		}
		retained := w.value(argument.source, argument.scope, true, depth+1)
		result.join(externalFlat(retained, 0))
	}
	return result
}

func (scope *externalTypeScope) argument(name *ast.Ident) (externalTypeArgument, bool) {
	for current := scope; current != nil; current = current.parent {
		binding, found := current.parameters[name.Name]
		if !found {
			continue
		}
		if name.Obj != nil {
			if binding.object != name.Obj {
				return externalTypeArgument{}, false
			}
		}
		return binding.argument, true
	}
	return externalTypeArgument{}, false
}

func externalResolvedTypeArgument(argument externalTypeArgument, depth int) externalTypeArgument {
	for depth < 128 {
		name, identifier := argument.source.value.(*ast.Ident)
		if !identifier {
			return argument
		}
		next, found := argument.scope.argument(name)
		if !found {
			return argument
		}
		argument = next
		depth++
	}
	return argument
}

func externalEmbeddedTypeName(value ast.Expr) string {
	for range 128 {
		switch datatype := value.(type) {
		case *ast.Ident:
			return datatype.Name
		case *ast.SelectorExpr:
			return datatype.Sel.Name
		case *ast.StarExpr:
			value = datatype.X
		case *ast.IndexExpr:
			value = datatype.X
		case *ast.IndexListExpr:
			value = datatype.X
		default:
			return ""
		}
	}
	return ""
}
