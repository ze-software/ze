// Design: docs/architecture/plugin/feature-gates.md -- retained capability expressions.
package repoinventory

import (
	"go/ast"
	"go/token"
	"go/types"
)

func (e *externalEffects) object(source sourceExpr) *ast.Object {
	switch value := source.value.(type) {
	case *ast.Ident:
		if value.Obj != nil {
			return value.Obj
		}
		return callablePackageBinding(source.pkg, value.Name)
	case *ast.SelectorExpr:
		directory, imported := e.source.snapshot.evaluator().callableImport(source, value)
		if !imported {
			return nil
		}
		pkg, found := e.source.snapshot.packages[directory]
		if !found {
			return nil
		}
		return callablePackageBinding(pkg, value.Sel.Name)
	case *ast.ParenExpr:
		return e.object(sourceExpr{value: value.X, file: source.file, pkg: source.pkg})
	}
	return nil
}

// value bounds expression recursion at 128 edges. Unknown data and unknown
// callable/reference ownership are not converted into the empty capability set.
// Once ambient unknown proves refusal, new expression expansion retains that fact.
// indexArgument MUST preserve this entrance refusal order for type operands.
// Any unknown the expression produced is sited at it, innermost first.
func (e *externalEffects) value(source sourceExpr, possible bool, depth int) externalValue {
	result := e.expressionValue(source, possible, depth)
	result.locate(source)
	return result
}

func (e *externalEffects) expressionValue(source sourceExpr, possible bool, depth int) externalValue {
	if e.amb.unknown != "" {
		return externalValue{unknown: e.amb.unknown}
	}
	if source.value == nil {
		return externalValue{}
	}
	if e.linkedObject(e.object(source)) {
		return externalValue{unknown: "linkname authority is unsupported"}
	}
	if depth == 128 {
		return externalValue{unknown: "expression capability depth exceeds 128"}
	}
	child := func(value ast.Expr) externalValue {
		return e.value(sourceExpr{value: value, file: source.file, pkg: source.pkg}, possible, depth+1)
	}
	switch value := source.value.(type) {
	case *ast.BasicLit:
		return externalValue{}
	case *ast.Ident:
		switch value.Name {
		case "nil", "true", "false":
			if predeclaredIdentifier(source, value.Name) {
				return externalValue{}
			}
		case "_":
			return externalValue{}
		}
		object := e.object(source)
		if object == nil {
			if types.Universe.Lookup(value.Name) != nil {
				return externalValue{}
			}
			return externalValue{unknown: "identifier ownership is unresolved: " + value.Name}
		}
		if object.Kind == ast.Fun {
			function, found := source.pkg.functions[value.Name]
			if !found {
				return externalValue{unknown: "function ownership is unresolved"}
			}
			return externalCallable(function.decl, sourceExpr{value: value, file: function.file, pkg: function.pkg})
		}
		if assignment, synthetic := object.Decl.(*ast.AssignStmt); synthetic {
			if len(assignment.Rhs) == 1 {
				if ranged, ok := assignment.Rhs[0].(*ast.UnaryExpr); ok {
					if ranged.Op == token.RANGE {
						datatype, known := e.source.snapshot.evaluator().valueType(source, 0)
						if !known {
							return externalValue{unknown: "range capability ownership is unsupported"}
						}
						if _, callback := datatype.value.(*ast.FuncType); callback {
							return externalValue{unknown: "range callback capability transfer is unsupported"}
						}
						collection := child(ranged.X)
						return e.rangedValue(object, source, collection, e.typeMethods(datatype, depth+1), depth+1)
					}
				}
			}
		}
		var result externalValue
		result.join(e.readObject(object))
		result.join(externalValue{reads: map[*ast.Object]bool{object: true}})
		return result
	case *ast.FuncLit:
		return externalCallable(value, source)
	case *ast.ParenExpr:
		return child(value.X)
	case *ast.SelectorExpr:
		if identity, external := e.source.externalImport(source, value); external {
			return externalOrigin(identity)
		}
		directory, imported := e.source.snapshot.evaluator().callableImport(source, value)
		if imported {
			pkg, found := e.source.snapshot.packages[directory]
			if !found {
				return externalValue{unknown: "imported source identity is unresolved"}
			}
			if function, found := pkg.functions[value.Sel.Name]; found {
				return externalCallable(function.decl, sourceExpr{value: function.decl.Name, file: function.file, pkg: pkg})
			}
			object := callablePackageBinding(pkg, value.Sel.Name)
			if object == nil {
				return externalValue{unknown: "imported declaration ownership is unresolved"}
			}
			var result externalValue
			result.join(e.readObject(object))
			result.join(externalValue{reads: map[*ast.Object]bool{object: true}})
			return result
		}
		if function, addressed, found := e.source.snapshot.evaluator().methodReceiver(source, 0); found {
			operand := sourceExpr{value: value.X, file: source.file, pkg: source.pkg}
			var receiver externalValue
			if addressed {
				// Go calls the pointer method on &x, so the receiver is that
				// address and a write through it reaches x's storage.
				receiver = e.addressValue(operand, source, possible, depth)
			} else {
				receiver = child(value.X)
			}
			if e.amb.unknown != "" {
				return externalValue{unknown: e.amb.unknown}
			}
			for _, field := range function.decl.Recv.List {
				for _, name := range field.Names {
					e.bind(name.Obj, receiver)
				}
			}
			return externalCallable(function.decl, sourceExpr{value: function.decl.Name, file: function.file, pkg: function.pkg})
		}
		receiver := child(value.X)
		if e.amb.unknown != "" {
			return externalValue{unknown: e.amb.unknown}
		}
		if len(receiver.origins) != 0 {
			return receiver
		}
		return e.projectField(source, value, receiver, depth)
	case *ast.CallExpr:
		return e.call(source, value, possible, depth+1)
	case *ast.CompositeLit:
		return e.literalValue(source, value, possible, depth)
	case *ast.UnaryExpr:
		if value.Op == token.RANGE {
			const reason = "range assignment capability ownership is unsupported"
			e.problem(source, reason)
			return externalValue{unknown: reason}
		}
		if value.Op == token.ARROW {
			// A receive reads the channel's flattened contents, as an index
			// reads a slice's. A received whole value cannot borrow a field
			// from an earlier binding, so its fields stay an obligation.
			result := child(value.X)
			result.fields = nil
			result.unresolvedFields = "received channel element field ownership requires a retained element summary"
			return result
		}
		if value.Op == token.AND {
			return e.addressValue(sourceExpr{value: value.X, file: source.file, pkg: source.pkg}, source, possible, depth)
		}
		return child(value.X)
	case *ast.BinaryExpr:
		result := child(value.X)
		result.join(child(value.Y))
		return result
	case *ast.IndexExpr:
		result := child(value.X)
		result.join(e.indexArgument(sourceExpr{value: value.Index, file: source.file, pkg: source.pkg}, possible, depth+1))
		result.fields = nil
		result.unresolvedFields = "indexed element field ownership requires a retained element summary"
		return result
	case *ast.SliceExpr:
		result := child(value.X)
		result.join(child(value.Low))
		result.join(child(value.High))
		result.join(child(value.Max))
		return result
	case *ast.StarExpr:
		result := child(value.X)
		for object := range result.locations {
			result.join(e.bindings[object])
			result.join(externalValue{reads: map[*ast.Object]bool{object: true}})
		}
		return result
	case *ast.TypeAssertExpr:
		return child(value.X)
	case *ast.ArrayType, *ast.MapType, *ast.ChanType, *ast.FuncType, *ast.InterfaceType, *ast.StructType:
		return e.typeMethods(source, depth+1)
	}
	return externalValue{unknown: "unsupported capability expression"}
}

// typeMethods computes possible capabilities without certifying a concrete
// method owner. Separate reflected type authority preserves concrete value
// descriptors without adding ordinary callee owners. Each walk keeps its own
// lexical substitutions and cycle path.
// A permanent ambient unknown prevents starting another type tree.
func (e *externalEffects) typeMethods(source sourceExpr, depth int) externalValue {
	if e.amb.unknown != "" {
		return externalValue{unknown: e.amb.unknown}
	}
	if source.value == nil {
		return externalValue{}
	}
	walk := externalTypeWalk{effects: e, reflection: true}
	scope := e.typeIdentities.lexicalScope(source)
	result := walk.value(source, scope, true, depth)
	result.reflection.join(e.reflectedValue(source, scope, depth))
	return result
}
