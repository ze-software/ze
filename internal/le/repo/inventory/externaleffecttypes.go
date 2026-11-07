// Design: docs/architecture/plugin/feature-gates.md -- conservative retained type capabilities.
package repoinventory

import (
	"go/ast"
	"go/types"
)

// externalTypeWalk proves a capability union, never a certain method owner.
// Its zero path is empty; effects is supplied by the report's private reader.
type externalTypeWalk struct {
	effects    *externalEffects
	path       []externalTypeFrame
	references bool
	reflection bool
	// ambientRead records that the frame being computed read the ambient
	// pool, directly or through a settled value that did. A settled value
	// that never read it holds for every pool (externalTypeWalk.settle).
	ambientRead bool
	// lowest is the outermost active frame the frame being computed has
	// re-entered; at or above its own index, the frame read no outer estimate.
	lowest int
	// growth counts estimate growths, so a component's outermost frame sees
	// whether a round changed any estimate inside it.
	growth    int
	estimates map[int32]*externalValue
}

// externalTypeRounds bounds the rounds of one recursive frame. The estimate
// only grows, and the capabilities of a finite retained program form a finite
// set, so the rounds end. Three settle every component the walk models today:
// each arm joins its children into the type's own flattened value, so after
// the first round the outermost estimate is complete, after the second every
// inner one is, and the third confirms it; an inner component that settles
// inside a round grows its own estimates and may add one more. A frame still growing at the bound
// refuses with the cyclic unknown rather than trusting that argument.
const externalTypeRounds = 8

// externalTypeFrame is one named type the walk is computing. id is its exact
// frame (externalTypeTerms): definition, resolved arguments, the method flag
// and the walk's reflection setting. reentered records that the current round
// met a recursive reference to it, so only a recursive frame repeats or keeps
// an estimate.
type externalTypeFrame struct {
	id        int32
	reentered bool
}

// externalTypeFrameKind is the frame kind the type walk interns, distinct from
// every reflection-walk kind (externalReflectionFrameKind).
func externalTypeFrameKind(methods, reflection bool) uint8 {
	var kind uint8
	if methods {
		kind |= 1
	}
	if reflection {
		kind |= 2
	}
	return kind
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

// value walks a type for a caller at an incoming expression depth. A walk
// that starts at depth 128 or more refuses, as the value flow that asks it
// does; past that guard the walk treats the type graph by reachability: a
// cycle re-enters an open frame, and only an instantiation that grows without
// end is refused (externalTypeTermDepthMax), so the answer does not depend on
// the depth.
func (w *externalTypeWalk) value(source sourceExpr, scope *externalTypeScope, methods bool, depth int) externalValue {
	if depth >= 128 {
		result := externalValue{unknown: "type capability depth exceeds 128"}
		result.locate(source)
		return result
	}
	return w.walk(source, scope, methods)
}

// walk sites any unknown its shape produced at the type expression it walked.
func (w *externalTypeWalk) walk(source sourceExpr, scope *externalTypeScope, methods bool) externalValue {
	result := w.shape(source, scope, methods)
	result.locate(source)
	return result
}

// shape follows the retained expression and the parameter substitutions it
// mentions, both finite; a cycle passes through a named frame, which named
// closes.
func (w *externalTypeWalk) shape(source sourceExpr, scope *externalTypeScope, methods bool) externalValue {
	child := func(value ast.Expr, includeMethods bool) externalValue {
		return w.walk(sourceExpr{value: value, file: source.file, pkg: source.pkg}, scope, includeMethods)
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
				value.reflection.join(w.effects.reflectedValue(sourceExpr{value: field.Type, file: source.file, pkg: source.pkg}, scope, 0))
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
				w.ambientRead = true
				result := externalFlat(w.effects.amb, 0)
				result.references = true
				return result
			}
			return w.walk(argument.source, argument.scope, methods)
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
				w.ambientRead = true
				result := externalFlat(w.effects.amb, 0)
				result.references = true
				return result
			}
		}
		return w.named(source, nil, methods)
	case *ast.SelectorExpr:
		return w.named(source, nil, methods)
	case *ast.IndexExpr:
		arguments := []externalTypeArgument{{source: sourceExpr{value: datatype.Index, file: source.file, pkg: source.pkg}, scope: scope}}
		return w.named(sourceExpr{value: datatype.X, file: source.file, pkg: source.pkg}, arguments, methods)
	case *ast.IndexListExpr:
		arguments := make([]externalTypeArgument, 0, len(datatype.Indices))
		for _, argument := range datatype.Indices {
			arguments = append(arguments, externalTypeArgument{source: sourceExpr{value: argument, file: source.file, pkg: source.pkg}, scope: scope})
		}
		return w.named(sourceExpr{value: datatype.X, file: source.file, pkg: source.pkg}, arguments, methods)
	}
	return externalValue{unknown: "unsupported retained type capability shape"}
}

// named is the one entrance for a named type, bare or instantiated, so a
// standard-library type is external on every route that reaches it.
func (w *externalTypeWalk) named(source sourceExpr, arguments []externalTypeArgument, methods bool) externalValue {
	if selector, selected := source.value.(*ast.SelectorExpr); selected {
		if identity, external := w.effects.source.externalImport(source, selector); external {
			return w.external(identity, arguments)
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
		argument = externalResolvedTypeArgument(argument, 0)
		arguments[index] = argument
		if argument.source.value == nil {
			w.references = true
			result.references = true
			w.ambientRead = true
			result.join(externalFlat(w.effects.amb, 0))
			continue
		}
		result.join(externalFlat(w.walk(argument.source, argument.scope, true), 0))
	}
	if result.unknown != "" {
		return result
	}
	// An instantiation that grows without end reaches a new frame on every
	// step, and its argument term is what keeps nesting
	// (externalTypeTermDepthMax).
	owned := w.effects.reflectionKeys.owner(owner.pkg, definition)
	id, bounded := w.effects.typeTerms.frame(owned, externalTypeFrameKind(methods, w.reflection), arguments)
	if !bounded {
		result.join(externalValue{unknown: "type capability depth exceeds 128"})
		return result
	}
	frame := externalTypeFrame{id: id}
	for index := range w.path {
		active := &w.path[index]
		if active.id == frame.id {
			// A frame still being computed: the reference stands for its
			// current flattened estimate, and the frame's component repeats
			// until no estimate grows (see the rounds below).
			active.reentered = true
			w.lowest = min(w.lowest, index)
			result.join(externalFlat(*w.estimate(active), 0))
			return result
		}
	}
	if settled, found := w.settledValue(&frame); found {
		if counters := w.effects.counters; counters != nil {
			counters.typeSettledHits++
		}
		result.join(settled)
		return result
	}
	scope := w.effects.typeIdentities.lexicalScope(owner)
	if parameters != 0 {
		scope = externalBindTypeParameters(scope, definition.TypeParams, arguments)
	}
	own := len(w.path)
	w.path = append(w.path, frame)
	outerLowest, outerReferences, outerAmbient := w.lowest, w.references, w.ambientRead
	w.lowest, w.references, w.ambientRead = own, false, false
	defer func() {
		w.path = w.path[:own]
		w.lowest = min(outerLowest, w.lowest)
		w.references = outerReferences || w.references
		w.ambientRead = outerAmbient || w.ambientRead
	}()
	if probe := w.effects.typeProbe; probe != nil {
		probe.expansions++
	}
	if counters := w.effects.counters; counters != nil {
		counters.typeExpansions++
		counters.tick()
	}
	// A recursive type's capability is the least fixpoint of its own walk.
	// Answering the recursive reference with nothing would be unsound: a
	// field children []*T would claim no capability although every child
	// carries T's methods, origins and ambient authority. The reference is
	// answered instead with the frame's estimate, flattened, which keeps
	// every capability, unknown and site it holds. Estimates start empty,
	// live for the whole walk, and take each round's flattened value.
	//
	// The frames that re-enter one another form a component, and only its
	// outermost frame repeats: an inner frame that read an outer estimate
	// (lowest < own) returns after one round, and the outer frame repeats the
	// whole component until a round grows no estimate. Every estimate then
	// holds what its frame's round adds, so it over-approximates the least
	// fixpoint and the round over-approximates the true value. Inner frames
	// never repeat on their own, so nesting costs rounds once, not per level.
	for range externalTypeRounds {
		if counters := w.effects.counters; counters != nil {
			counters.typeRounds++
		}
		growth := w.growth
		w.path[own].reentered = false
		var round externalValue
		round.join(result)
		// A defined type has its own method set; only an alias inherits the target's
		// top-level methods. Fields within either representation retain their own.
		round.join(w.walk(owner, scope, methods && definition.Assign.IsValid()))
		if methods {
			round.join(w.methods(definition, owner))
		}
		if w.path[own].reentered {
			if w.estimate(&w.path[own]).join(externalFlat(round, 0)) {
				w.growth++
			}
		}
		if w.lowest < own {
			// Provisional: computed from an outer estimate that may still
			// grow, so it is never settled; the outer frame repeats it.
			if counters := w.effects.counters; counters != nil {
				counters.typeProvisional++
			}
			return round
		}
		if !w.path[own].reentered {
			w.settle(&w.path[own], round)
			return round
		}
		if w.growth == growth {
			w.settle(&w.path[own], round)
			return round
		}
	}
	result.join(externalValue{unknown: "cyclic retained type capability shape: " + definition.Name.Name})
	return result
}

// external models a standard-library named type the way an external call is
// modelled: its source is not an analyzer input, so the type contributes its
// import's origin and may reach every ambient capability. Its type arguments
// are still retained types in this scope: atomic.Pointer[hook] keeps hook's
// methods, so a registering argument is still tracked.
func (w *externalTypeWalk) external(identity string, arguments []externalTypeArgument) externalValue {
	if err := w.effects.source.witness(identity); err != nil {
		return externalValue{unknown: err.Error()}
	}
	result := externalOrigin(identity)
	w.references = true
	w.ambientRead = true
	result.join(externalFlat(w.effects.amb, 0))
	for _, argument := range arguments {
		argument = externalResolvedTypeArgument(argument, 0)
		if argument.source.value == nil {
			continue
		}
		retained := w.walk(argument.source, argument.scope, true)
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
