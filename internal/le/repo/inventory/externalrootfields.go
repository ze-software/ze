// Design: docs/architecture/plugin/feature-gates.md -- retained field projections require root ownership.
package repoinventory

import (
	"go/ast"
	"go/token"
)

// fieldReceiverType proves the receiver expression's static root, independently
// of the capability union carried by interface alternatives or child values.
// Indexed element fields need a separate value summary; they cannot borrow a
// type-only empty callback field after collection flattening. A range
// variable's value slot, and an index expression, own their element type only
// once the collection's whole element population is proved, whose stored
// values then carry the actual field values (rangedValue, indexedValue).
func (e *externalEffects) fieldReceiverType(source sourceExpr, depth int) (sourceExpr, string) {
	if depth >= 128 {
		return sourceExpr{}, "root field ownership exceeds 128 source steps"
	}
	child := func(value ast.Expr) (sourceExpr, string) {
		return e.fieldReceiverType(sourceExpr{value: value, file: source.file, pkg: source.pkg}, depth+1)
	}
	switch value := source.value.(type) {
	case *ast.ParenExpr:
		return child(value.X)
	case *ast.CompositeLit:
		if value.Type == nil {
			return sourceExpr{}, "elided root field ownership is unresolved"
		}
		return sourceExpr{value: value.Type, file: source.file, pkg: source.pkg}, ""
	case *ast.TypeAssertExpr:
		if value.Type == nil {
			return sourceExpr{}, "asserted root field ownership is unresolved"
		}
		return sourceExpr{value: value.Type, file: source.file, pkg: source.pkg}, ""
	case *ast.UnaryExpr:
		if value.Op == token.AND {
			return child(value.X)
		}
		return sourceExpr{}, "unary root field ownership is unresolved"
	case *ast.StarExpr:
		return child(value.X)
	case *ast.IndexExpr:
		return e.indexedReceiverType(source, value, depth+1)
	case *ast.CallExpr:
		callee := sourceExpr{value: value.Fun, file: source.file, pkg: source.pkg}
		if e.conversion(callee) {
			return callee, ""
		}
		if datatype, known := e.resolver.valueType(source, depth+1); known {
			return datatype, ""
		}
		return sourceExpr{}, "returned root field ownership is unresolved"
	case *ast.SelectorExpr:
		if _, imported := e.resolver.callableImport(source, value); !imported {
			datatype, reason := child(value.X)
			if reason != "" {
				return sourceExpr{}, reason
			}
			return e.declaredFieldType(datatype, value.Sel.Name, depth+1)
		}
	case *ast.Ident:
	default:
		return sourceExpr{}, "root field expression ownership is unresolved"
	}
	object := e.object(source)
	if object == nil {
		return sourceExpr{}, "root field binding ownership is unresolved"
	}
	if _, _, ranged := externalRangeVariable(object); ranged {
		return e.rangedReceiverType(object, source, depth+1)
	}
	if datatype, declared := e.types[object]; declared {
		return datatype, ""
	}
	if externalTupleOperand(object) {
		// o, valid := mi.skipField(...): the initializer is the whole call,
		// which has no single type, so o's root is its position's declared
		// result (tupleType).
		if datatype, known := e.resolver.valueType(source, depth+1); known {
			return datatype, ""
		}
		return sourceExpr{}, "returned root field ownership is unresolved"
	}
	initializer, err := e.resolver.target(source)
	if err != nil {
		return sourceExpr{}, "root field initializer ownership is unresolved"
	}
	return e.fieldReceiverType(initializer, depth+1)
}

// indexedElementType answers the declared element type of an index
// expression from its operand's static type. Go specification, "Index
// expressions": for an array "a[x] is the array element at index x and the
// type of a[x] is the element type of A", for a pointer to an array "a[x] is
// shorthand for (*a)[x]", for a slice "a[x] is the slice element at index x
// and the type of a[x] is the element type of S", and for a map the type of
// a[x] "is the element type of M". A string element is a byte, never a
// struct, and a generic instantiation or a type parameter holds no element
// this proof names, so each of those refuses.
func (e *externalEffects) indexedElementType(source sourceExpr, index *ast.IndexExpr, depth int) (sourceExpr, string) {
	operand, known := e.resolver.valueType(sourceExpr{value: index.X, file: source.file, pkg: source.pkg}, depth+1)
	if !known {
		return sourceExpr{}, "indexed collection type is unresolved"
	}
	shape, known := e.resolver.underlyingType(operand, depth+1)
	if !known {
		return sourceExpr{}, "indexed collection type is unresolved"
	}
	if pointer, indirect := shape.value.(*ast.StarExpr); indirect {
		shape, known = e.resolver.underlyingType(sourceExpr{value: pointer.X, file: shape.file, pkg: shape.pkg}, depth+1)
		if !known {
			return sourceExpr{}, "indexed array type is unresolved"
		}
		array, isArray := shape.value.(*ast.ArrayType)
		if !isArray {
			return sourceExpr{}, "indexed pointer does not point to an array"
		}
		if array.Len == nil {
			return sourceExpr{}, "indexed pointer does not point to an array"
		}
	}
	switch kind := shape.value.(type) {
	case *ast.ArrayType:
		return sourceExpr{value: kind.Elt, file: shape.file, pkg: shape.pkg}, ""
	case *ast.MapType:
		return sourceExpr{value: kind.Value, file: shape.file, pkg: shape.pkg}, ""
	}
	return sourceExpr{}, "indexed operand is not an array, slice or map"
}

func (e *externalEffects) declaredFieldType(datatype sourceExpr, name string, depth int) (sourceExpr, string) {
	layout, reason := e.literalLayout(datatype, depth, false)
	if reason != "" {
		return sourceExpr{}, "root field ownership is unresolved: " + reason
	}
	if layout.kind != externalLiteralStruct {
		return sourceExpr{}, "root field ownership is not a retained struct"
	}
	field, found := layout.types[name]
	if !found {
		return sourceExpr{}, "root field ownership has no direct declared member " + name
	}
	return field, ""
}

func (e *externalEffects) projectField(source sourceExpr, selector *ast.SelectorExpr, receiver externalValue, depth int) externalValue {
	datatype, reason := e.fieldReceiverType(sourceExpr{value: selector.X, file: source.file, pkg: source.pkg}, depth+1)
	if reason == "" {
		_, reason = e.declaredFieldType(datatype, selector.Sel.Name, depth+1)
	}
	if reason != "" {
		e.problem(source, reason)
		return externalValue{unknown: reason}
	}
	if receiver.unresolvedFields != "" {
		e.problem(source, receiver.unresolvedFields)
		return externalValue{unknown: receiver.unresolvedFields}
	}
	field, known := receiver.fields[selector.Sel.Name]
	if !known {
		// A missing field is a fact of this round only: the receiver may be
		// the result of a callee the walk scheduled this round, whose returns
		// the fixpoint has not summarized yet. The problem refuses the round,
		// and beginRound clears it, so a field the last round still cannot
		// find refuses the inventory. An unknown value here would instead
		// reach the ambient pool through an external call taking it as an
		// argument, and the pool's unknown is permanent: no later round could
		// clear it once the callee's returns carried the field.
		e.problem(source, "declared root field value ownership is unresolved")
		return externalValue{reads: receiver.reads}
	}
	var result externalValue
	result.join(field)
	result.join(externalValue{reads: receiver.reads})
	if field.references {
		result.join(externalValue{locations: receiver.locations})
	}
	return result
}

// externalTupleOperand reports whether object is declared by a multi-value
// assignment or declaration whose single right-hand side yields every
// operand, as a call with several results does.
func externalTupleOperand(object *ast.Object) bool {
	switch declaration := object.Decl.(type) {
	case *ast.AssignStmt:
		return len(declaration.Lhs) > 1 && len(declaration.Rhs) == 1
	case *ast.ValueSpec:
		return len(declaration.Names) > 1 && len(declaration.Values) == 1
	}
	return false
}
