// Design: docs/architecture/plugin/feature-gates.md -- executed range operands retain source kinds.
package repoinventory

import (
	"go/ast"
	"go/token"
	"go/types"
)

// externalRangeKind describes static Go representation, not the capabilities
// contained in a value. Unresolved is never permission to omit execution.
type externalRangeKind uint8

const (
	externalRangeUnresolved externalRangeKind = iota
	externalRangeArray
	externalRangeArrayPointer
	externalRangeSlice
	externalRangeMap
	externalRangeInteger
	externalRangeString
	externalRangeChannel
	externalRangeIterator
)

// externalRangeWalk is private to one operand proof. Each declaration/type path
// is bounded by 128 edges; an active source identity cannot certify itself.
type externalRangeWalk struct {
	effects *externalEffects
	active  map[ast.Expr]bool
}

func (e *externalEffects) rangeExecution(source sourceExpr) {
	walk := externalRangeWalk{effects: e, active: make(map[ast.Expr]bool)}
	kind, reason := walk.expression(source, 0)
	switch kind {
	case externalRangeArray, externalRangeArrayPointer, externalRangeSlice, externalRangeMap, externalRangeInteger, externalRangeString:
		// These kinds have no implicit user-defined iterator invocation.
	case externalRangeIterator:
		e.problem(source, "function-range execution is unsupported; use a retained collection before publishing inventory")
	case externalRangeChannel:
		e.problem(source, "channel range capability transfer is unsupported")
	case externalRangeUnresolved:
		e.problem(source, "range execution ownership is unknown: "+reason)
	}
}

func (walk *externalRangeWalk) enter(source sourceExpr, depth int) string {
	if depth >= 128 {
		return "range source kind exceeds 128 declaration steps"
	}
	if source.value == nil {
		return "range source kind has no retained expression"
	}
	if walk.active[source.value] {
		return "range source kind contains a declaration cycle"
	}
	walk.active[source.value] = true
	return ""
}

func (walk *externalRangeWalk) expression(source sourceExpr, depth int) (externalRangeKind, string) {
	if reason := walk.enter(source, depth); reason != "" {
		return externalRangeUnresolved, reason
	}
	defer delete(walk.active, source.value)
	child := func(value ast.Expr) (externalRangeKind, string) {
		return walk.expression(sourceExpr{value: value, file: source.file, pkg: source.pkg}, depth+1)
	}
	switch value := source.value.(type) {
	case *ast.ParenExpr:
		return child(value.X)
	case *ast.BasicLit:
		// Only integer and string literals establish non-invoking ranges.
		//exhaustive:ignore
		switch value.Kind {
		case token.INT, token.CHAR:
			return externalRangeInteger, ""
		case token.STRING:
			return externalRangeString, ""
		}
		return externalRangeUnresolved, "range literal is neither an integer nor a string"
	case *ast.CompositeLit:
		return walk.datatype(sourceExpr{value: value.Type, file: source.file, pkg: source.pkg}, depth+1)
	case *ast.FuncLit:
		return externalRangeIterator, ""
	case *ast.Ident:
		return walk.binding(source, depth+1)
	case *ast.SelectorExpr:
		if _, declared := walk.effects.resolver.function(source); declared {
			return externalRangeIterator, ""
		}
		if _, imported := walk.effects.resolver.callableImport(source, value); imported {
			return walk.binding(source, depth+1)
		}
		field, reason := walk.fieldType(source, value, depth+1)
		if reason != "" {
			return externalRangeUnresolved, reason
		}
		return walk.datatype(field, depth+1)
	case *ast.CallExpr:
		return walk.call(source, value, depth+1)
	case *ast.UnaryExpr:
		kind, reason := child(value.X)
		if reason != "" {
			return externalRangeUnresolved, reason
		}
		// Only integer operators and an array address preserve these kinds.
		//exhaustive:ignore
		switch value.Op {
		case token.ADD, token.SUB, token.XOR:
			if kind == externalRangeInteger {
				return kind, ""
			}
		case token.AND:
			if kind == externalRangeArray {
				return externalRangeArrayPointer, ""
			}
		}
		return externalRangeUnresolved, "range unary operand has no proved integer or array kind"
	case *ast.StarExpr:
		kind, reason := child(value.X)
		if reason != "" {
			return externalRangeUnresolved, reason
		}
		if kind == externalRangeArrayPointer {
			return externalRangeArray, ""
		}
		return externalRangeUnresolved, "range dereference has no proved array pointee"
	case *ast.SliceExpr:
		kind, reason := child(value.X)
		if reason != "" {
			return externalRangeUnresolved, reason
		}
		switch kind {
		case externalRangeArray, externalRangeArrayPointer, externalRangeSlice:
			return externalRangeSlice, ""
		case externalRangeString:
			return externalRangeString, ""
		case externalRangeUnresolved, externalRangeMap, externalRangeInteger, externalRangeChannel, externalRangeIterator:
			return externalRangeUnresolved, "range slice has no proved sliceable kind"
		}
	case *ast.BinaryExpr:
		return walk.binary(source, value, depth+1)
	}
	return externalRangeUnresolved, "range operand shape is unsupported"
}

func (walk *externalRangeWalk) binding(source sourceExpr, depth int) (externalRangeKind, string) {
	object := walk.effects.object(source)
	if object == nil {
		return externalRangeUnresolved, "range declaration identity is unresolved"
	}
	if object.Kind == ast.Fun {
		return externalRangeIterator, ""
	}
	if object.Kind != ast.Var {
		if object.Kind != ast.Con {
			return externalRangeUnresolved, "range declaration is not a retained value"
		}
	}
	if datatype, known := walk.effects.types[object]; known {
		return walk.datatype(datatype, depth+1)
	}
	if err := walk.effects.resolver.immutable(source); err != nil {
		return externalRangeUnresolved, "range inferred declaration is not immutable: " + err.Error()
	}
	resolved, err := walk.effects.resolver.target(source)
	if err != nil {
		return externalRangeUnresolved, "range declaration initializer is unresolved: " + err.Error()
	}
	return walk.expression(resolved, depth+1)
}

func (walk *externalRangeWalk) call(source sourceExpr, call *ast.CallExpr, depth int) (externalRangeKind, string) {
	callee := sourceExpr{value: call.Fun, file: source.file, pkg: source.pkg}
	if walk.effects.builtin(callee) {
		name := call.Fun.(*ast.Ident).Name
		switch name {
		case "len", "cap":
			return externalRangeInteger, ""
		case "make", "new":
			if len(call.Args) == 0 {
				return externalRangeUnresolved, "range allocation has no retained result type"
			}
			kind, reason := walk.datatype(sourceExpr{value: call.Args[0], file: source.file, pkg: source.pkg}, depth+1)
			if reason != "" {
				return externalRangeUnresolved, reason
			}
			if name == "make" {
				return kind, ""
			}
			if kind == externalRangeArray {
				return externalRangeArrayPointer, ""
			}
			return externalRangeUnresolved, "range new result has no proved array pointee"
		}
		return externalRangeUnresolved, "range builtin result kind is unsupported"
	}
	if walk.effects.conversion(callee) {
		return walk.datatype(callee, depth+1)
	}
	signature, owner, reason := walk.signature(callee, depth+1)
	if reason != "" {
		return externalRangeUnresolved, reason
	}
	if signature.TypeParams.NumFields() != 0 {
		return externalRangeUnresolved, "generic range result ownership is unsupported"
	}
	if signature.Results.NumFields() != 1 {
		return externalRangeUnresolved, "range result does not have one retained static type"
	}
	owner.value = signature.Results.List[0].Type
	return walk.datatype(owner, depth+1)
}

func (walk *externalRangeWalk) signature(source sourceExpr, depth int) (*ast.FuncType, sourceExpr, string) {
	if depth >= 128 {
		return nil, sourceExpr{}, "range callee exceeds 128 declaration steps"
	}
	if parenthesized, wrapped := source.value.(*ast.ParenExpr); wrapped {
		source.value = parenthesized.X
		return walk.signature(source, depth+1)
	}
	if literal, local := source.value.(*ast.FuncLit); local {
		return literal.Type, source, ""
	}
	if function, declared := walk.effects.resolver.function(source); declared {
		return function.decl.Type, sourceExpr{file: function.file, pkg: function.pkg}, ""
	}
	if err := walk.effects.resolver.callableImmutable(source); err != nil {
		return nil, sourceExpr{}, "range callee ownership is unresolved: " + err.Error()
	}
	resolved, err := walk.effects.resolver.target(source)
	if err != nil {
		return nil, sourceExpr{}, "range callee declaration is unresolved: " + err.Error()
	}
	return walk.signature(resolved, depth+1)
}

func (walk *externalRangeWalk) binary(source sourceExpr, expression *ast.BinaryExpr, depth int) (externalRangeKind, string) {
	left, reason := walk.expression(sourceExpr{value: expression.X, file: source.file, pkg: source.pkg}, depth+1)
	if reason != "" {
		return externalRangeUnresolved, reason
	}
	right, reason := walk.expression(sourceExpr{value: expression.Y, file: source.file, pkg: source.pkg}, depth+1)
	if reason != "" {
		return externalRangeUnresolved, reason
	}
	if expression.Op == token.ADD {
		if left == externalRangeString {
			if right == externalRangeString {
				return externalRangeString, ""
			}
		}
	}
	if left != externalRangeInteger {
		return externalRangeUnresolved, "range binary operands do not prove an integer result"
	}
	if right != externalRangeInteger {
		return externalRangeUnresolved, "range binary operands do not prove an integer result"
	}
	// Only integer arithmetic and bit operations preserve an integer result.
	//exhaustive:ignore
	switch expression.Op {
	case token.ADD, token.SUB, token.MUL, token.QUO, token.REM, token.AND, token.OR, token.XOR, token.AND_NOT, token.SHL, token.SHR:
		return externalRangeInteger, ""
	}
	return externalRangeUnresolved, "range binary operator does not produce an integer"
}

func (walk *externalRangeWalk) datatype(source sourceExpr, depth int) (externalRangeKind, string) {
	if reason := walk.enter(source, depth); reason != "" {
		return externalRangeUnresolved, reason
	}
	defer delete(walk.active, source.value)
	child := func(value ast.Expr) (externalRangeKind, string) {
		return walk.datatype(sourceExpr{value: value, file: source.file, pkg: source.pkg}, depth+1)
	}
	switch datatype := source.value.(type) {
	case *ast.ParenExpr:
		return child(datatype.X)
	case *ast.ArrayType:
		if datatype.Len == nil {
			return externalRangeSlice, ""
		}
		return externalRangeArray, ""
	case *ast.MapType:
		return externalRangeMap, ""
	case *ast.ChanType:
		return externalRangeChannel, ""
	case *ast.FuncType:
		return externalRangeIterator, ""
	case *ast.StarExpr:
		kind, reason := child(datatype.X)
		if reason != "" {
			return externalRangeUnresolved, reason
		}
		if kind == externalRangeArray {
			return externalRangeArrayPointer, ""
		}
		return externalRangeUnresolved, "range pointer has no proved array pointee"
	case *ast.Ident:
		if datatype.Obj == nil {
			if callablePackageBinding(source.pkg, datatype.Name) == nil {
				if name, builtin := types.Universe.Lookup(datatype.Name).(*types.TypeName); builtin {
					if primitive, basic := name.Type().Underlying().(*types.Basic); basic {
						if primitive.Info()&types.IsInteger != 0 {
							return externalRangeInteger, ""
						}
						if primitive.Info()&types.IsString != 0 {
							return externalRangeString, ""
						}
					}
					return externalRangeUnresolved, "range basic type is neither integer nor string"
				}
			}
		}
	case *ast.SelectorExpr:
		// The exact retained declaration is resolved below.
	case *ast.IndexExpr, *ast.IndexListExpr:
		return externalRangeUnresolved, "generic range kind substitution is unsupported"
	default:
		return externalRangeUnresolved, "range type representation is unsupported"
	}
	shapeWalk := externalTypeWalk{effects: walk.effects}
	definition, owner, reason := shapeWalk.definition(source)
	if reason != "" {
		return externalRangeUnresolved, reason
	}
	if definition.TypeParams.NumFields() != 0 {
		return externalRangeUnresolved, "generic range declaration ownership is unsupported"
	}
	return walk.datatype(owner, depth+1)
}

func (walk *externalRangeWalk) fieldType(source sourceExpr, selector *ast.SelectorExpr, depth int) (sourceExpr, string) {
	receiver := sourceExpr{value: selector.X, file: source.file, pkg: source.pkg}
	var datatype sourceExpr
	if object := walk.effects.object(receiver); object != nil {
		datatype = walk.effects.types[object]
	}
	if datatype.value == nil {
		var known bool
		datatype, known = walk.effects.resolver.valueType(receiver, 0)
		if !known {
			return sourceExpr{}, "range field receiver type is unresolved"
		}
	}
	seen := make(map[*ast.TypeSpec]bool)
	indirect := false
	for depth < 128 {
		switch shape := datatype.value.(type) {
		case *ast.ParenExpr:
			datatype.value = shape.X
		case *ast.StarExpr:
			if indirect {
				return sourceExpr{}, "range field receiver has multiple pointer layers"
			}
			indirect = true
			datatype.value = shape.X
		case *ast.StructType:
			var result sourceExpr
			for _, field := range shape.Fields.List {
				for _, name := range field.Names {
					if name.Name != selector.Sel.Name {
						continue
					}
					if result.value != nil {
						return sourceExpr{}, "range field declaration is ambiguous"
					}
					result = sourceExpr{value: field.Type, file: datatype.file, pkg: datatype.pkg}
				}
			}
			if result.value == nil {
				return sourceExpr{}, "range field type needs unsupported promotion or selection"
			}
			return result, ""
		case *ast.Ident, *ast.SelectorExpr:
			shapeWalk := externalTypeWalk{effects: walk.effects}
			definition, owner, reason := shapeWalk.definition(datatype)
			if reason != "" {
				return sourceExpr{}, reason
			}
			if definition.TypeParams.NumFields() != 0 {
				return sourceExpr{}, "generic range field ownership is unsupported"
			}
			if seen[definition] {
				return sourceExpr{}, "range field type contains a declaration cycle"
			}
			seen[definition] = true
			datatype = owner
		default:
			return sourceExpr{}, "range field receiver is not a retained struct"
		}
		depth++
	}
	return sourceExpr{}, "range field type exceeds 128 declaration steps"
}
