// Design: docs/architecture/plugin/feature-gates.md -- retained binding equations.
package repoinventory

import (
	"go/ast"
)

type externalStore struct {
	target sourceExpr
	values []ast.Expr
	// element marks a store into a container's contents (a channel send):
	// its values join flattened, as send does, so a sent value's root fields
	// and field obligation never become the channel holder's own.
	element bool
}

// externalDefinition keeps producing execution separate from later reads of
// its result. Exposing a result cannot invoke its certain initializer again.
type externalDefinition struct {
	source sourceExpr
	owner  ast.Node
}

// indexDefinitions discovers data-flow equations without executing their RHS.
// Only a reached or exposed binding evaluates these sources. All assignments to
// that identity contribute, including assignments in otherwise unselected code.
func (e *externalEffects) indexDefinitions(directories []string) {
	for _, directory := range directories {
		pkg := e.source.snapshot.packages[directory]
		for _, file := range pkg.files {
			for _, group := range file.File.Comments {
				for _, comment := range group.List {
					e.indexLinkname(comment.Text, sourceExpr{file: file.File, pkg: pkg})
				}
			}
			var owner ast.Node
			var owners []ast.Node
			ast.Inspect(file.File, func(node ast.Node) bool {
				if node == nil {
					owner = owners[len(owners)-1]
					owners = owners[:len(owners)-1]
					return false
				}
				owners = append(owners, owner)
				switch node.(type) {
				case *ast.FuncDecl, *ast.FuncLit:
					owner = node
				}
				switch declaration := node.(type) {
				case *ast.ValueSpec:
					for _, name := range declaration.Names {
						if declaration.Type != nil {
							e.types[name.Obj] = sourceExpr{value: declaration.Type, file: file.File, pkg: pkg}
						}
						for _, value := range declaration.Values {
							e.definitions[name.Obj] = append(e.definitions[name.Obj], externalDefinition{source: sourceExpr{value: value, file: file.File, pkg: pkg}, owner: owner})
						}
					}
				case *ast.AssignStmt:
					for _, target := range declaration.Lhs {
						object := e.object(sourceExpr{value: target, file: file.File, pkg: pkg})
						if object == nil {
							if name, identifier := target.(*ast.Ident); identifier {
								if name.Name == "_" {
									continue
								}
							}
							e.stores = append(e.stores, externalStore{target: sourceExpr{value: target, file: file.File, pkg: pkg}, values: declaration.Rhs})
							continue
						}
						for _, value := range declaration.Rhs {
							e.definitions[object] = append(e.definitions[object], externalDefinition{source: sourceExpr{value: value, file: file.File, pkg: pkg}, owner: owner})
						}
					}
				case *ast.SendStmt:
					// A send stores into the channel's contents, as an
					// indexed assignment stores into a slice element.
					e.stores = append(e.stores, externalStore{target: sourceExpr{value: declaration.Chan, file: file.File, pkg: pkg}, values: []ast.Expr{declaration.Value}, element: true})
				case *ast.RangeStmt:
					e.indexRangeAssignment(declaration, sourceExpr{file: file.File, pkg: pkg}, owner)
				case *ast.Field:
					for _, name := range declaration.Names {
						e.types[name.Obj] = sourceExpr{value: declaration.Type, file: file.File, pkg: pkg}
					}
				}
				return true
			})
		}
	}
}

// indirectStores closes assignments through retained aliases onto already
// exposed writable roots. An unresolvable location cannot prove disjointness.
// Only a permanent ambient unknown stops the remaining store expansion.
func (e *externalEffects) indirectStores() {
	if e.amb.unknown != "" {
		return
	}
	if len(e.amb.origins) == 0 {
		return
	}
	for _, store := range e.stores {
		if e.amb.unknown != "" {
			return
		}
		location := e.value(store.target, true, 0)
		if e.amb.unknown != "" {
			return
		}
		if location.unknown != "" {
			e.storeLocationProblem(store, ": "+location.unknown)
			continue
		}
		if len(location.locations) == 0 {
			e.storeLocationProblem(store, "")
			continue
		}
		for object := range location.locations {
			if !e.amb.locations[object] {
				continue
			}
			for _, expression := range store.values {
				value := e.value(sourceExpr{value: expression, file: store.target.file, pkg: store.target.pkg}, true, 0)
				if e.amb.unknown != "" {
					return
				}
				if store.element {
					value = externalFlat(value, 0)
				}
				e.bind(object, value)
			}
		}
	}
}

func (e *externalEffects) readObject(object *ast.Object) externalValue {
	if e.amb.unknown != "" {
		return externalValue{unknown: e.amb.unknown}
	}
	if _, formal := e.formals[object]; formal {
		e.formalDemand[object] = true
	}
	if e.linkedObject(object) {
		return externalValue{unknown: "linkname authority is unsupported", unknownAt: e.linknames[object]}
	}
	// A read answered earlier in this round is answered again from the round's
	// cache. A state change after that read sets e.changed, so the fixpoint
	// runs another round, and in the round that ends it nothing changes, so
	// every cached read equals a fresh one. The caller gets its own copy.
	if cached, found := e.readCache[object]; found {
		var answered externalValue
		answered.join(cached)
		return answered
	}
	var result externalValue
	result.join(e.bindings[object])
	if e.reading[object] {
		return result
	}
	e.reading[object] = true
	defer delete(e.reading, object)
	if e.counters != nil {
		e.counters.objectEvaluations++
	}
	if datatype, found := e.types[object]; found {
		result.join(e.typeMethods(datatype, 0))
		if e.referenceType(datatype, 0) {
			result.join(externalLocation(object))
		}
	}
	for _, binding := range e.definitions[object] {
		definition := binding.source
		value := e.value(definition, e.definitionPossible(binding), 0)
		if e.amb.unknown != "" {
			return externalValue{unknown: e.amb.unknown}
		}
		result.join(value)
		if literal, ok := definition.value.(*ast.CompositeLit); ok {
			if e.referenceType(sourceExpr{value: literal.Type, file: definition.file, pkg: definition.pkg}, 0) {
				result.join(externalLocation(object))
			}
		}
		if call, ok := definition.value.(*ast.CallExpr); ok {
			callee := sourceExpr{value: call.Fun, file: definition.file, pkg: definition.pkg}
			if e.builtin(callee) {
				name := callee.value.(*ast.Ident).Name
				switch name {
				case "append", "new":
					result.join(externalLocation(object))
				case "make":
					if len(call.Args) != 0 {
						if e.referenceType(sourceExpr{value: call.Args[0], file: definition.file, pkg: definition.pkg}, 0) {
							result.join(externalLocation(object))
						}
					}
				}
			}
			if function, retained := e.source.snapshot.evaluator().function(callee); retained {
				if function.decl.Type.Results != nil {
					for _, field := range function.decl.Type.Results.List {
						if e.referenceType(sourceExpr{value: field.Type, file: function.file, pkg: function.pkg}, 0) {
							result.join(externalLocation(object))
						}
					}
				}
			}
		}
	}
	e.bind(object, result)
	var cached externalValue
	cached.join(result)
	e.readCache[object] = cached
	return result
}

// definitionPossible uses the producer's execution mode, never its consumer's.
// Unscheduled writers stay possible; actual callback exposure can still upgrade
// a scheduled owner, and every fixed-point round revisits these equations.
func (e *externalEffects) definitionPossible(definition externalDefinition) bool {
	if definition.owner == nil {
		return !e.source.roots[definition.source.pkg.dir]
	}
	if _, scheduled := e.bodies[definition.owner]; !scheduled {
		return true
	}
	return e.possible[definition.owner]
}

func (e *externalEffects) referenceType(source sourceExpr, depth int) bool {
	walk := externalTypeWalk{effects: e}
	value := walk.value(source, e.typeIdentities.lexicalScope(source), true, depth)
	if value.unknown != "" {
		e.problem(source, "reference ownership is unresolved: "+value.unknown)
	}
	return walk.references
}

// externalFormalAmbient is the part of an ambient value a parameter of shape
// holds. Function values have no selectable fields: their captured
// authorities remain in the flattened union and lexical owners.
func externalFormalAmbient(value externalValue, shape externalFormalShape) externalValue {
	if shape == externalFormalCallable {
		return externalFlat(value, 0)
	}
	return value
}

// externalParameters admits arbitrary values reachable under the same ambient
// authority, including methods of the declared parameter/receiver types.
func (e *externalEffects) externalParameters(node ast.Node, source sourceExpr) {
	var signature *ast.FuncType
	var receiver *ast.FieldList
	switch function := node.(type) {
	case *ast.FuncDecl:
		signature = function.Type
		receiver = function.Recv
	case *ast.FuncLit:
		signature = function.Type
	}
	if signature == nil {
		e.problem(source, "callable signature is unavailable")
		return
	}
	for _, fields := range []*ast.FieldList{signature.Params, receiver} {
		if fields == nil {
			continue
		}
		for _, field := range fields.List {
			datatype := sourceExpr{value: field.Type, file: source.file, pkg: source.pkg}
			value := e.typeMethods(datatype, 0)
			shape, reason := e.formalShape(datatype, 0)
			if reason != "" {
				e.problem(datatype, "external parameter capability is unknown: "+reason)
				continue
			}
			for _, name := range field.Names {
				if !e.formalExternal[name.Obj] {
					e.formalExternal[name.Obj] = true
					e.changed = true
				}
				e.bind(name.Obj, value)
				if shape == externalFormalScalar {
					continue
				}
				// Binding the methods and then the pool equals binding their
				// union: join is monotone and its first-wins facts keep the
				// same order. A parameter that absorbed the pool at version v
				// already holds all of it, so it joins only the growths since.
				absorbed, seen := e.formalAmbient[name.Obj]
				if !seen {
					e.bind(name.Obj, externalFormalAmbient(e.amb, shape))
					e.formalAmbient[name.Obj] = e.ambient
					continue
				}
				for index := absorbed; index < len(e.ambientGrowths); index++ {
					e.bind(name.Obj, externalFormalAmbient(e.ambientGrowths[index], shape))
				}
				e.formalAmbient[name.Obj] = e.ambient
			}
		}
	}
}
