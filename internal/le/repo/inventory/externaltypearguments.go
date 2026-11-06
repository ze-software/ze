// Design: docs/architecture/plugin/feature-gates.md -- opaque generic arguments retain descriptor authority without callable ownership.
package repoinventory

import (
	"go/ast"
	"go/types"
)

// indexArgument distinguishes a proved type argument from an ordinary index.
// Its entrance checks MUST preserve value's refusal order. Refused inputs use
// value's existing reason, and an unproved type kind keeps ordinary value flow.
func (e *externalEffects) indexArgument(source sourceExpr, possible bool, depth int) externalValue {
	if e.amb.unknown != "" {
		return e.value(source, possible, depth)
	}
	if e.linkedObject(e.object(source)) {
		return e.value(source, possible, depth)
	}
	if depth == 128 {
		return e.value(source, possible, depth)
	}
	isType, reason := e.typeArgument(source, depth)
	if reason != "" {
		return externalValue{unknown: reason}
	}
	if !isType {
		return e.value(source, possible, depth)
	}
	walk := externalReflectionWalk{effects: e}
	// Match typeMethods' descriptor edge after the argument expression. A type
	// supplies its descriptor even when a value of that type is a nil interface.
	return externalValue{reflection: walk.value(source, e.typeIdentities.lexicalScope(source), true, false, depth+1)}
}

// typeArgument proves only retained or built-in type syntax. It follows at most
// 128 expression edges and never interprets an uninspected selector's spelling.
// A false result means the argument MUST retain its ordinary value evaluation.
func (e *externalEffects) typeArgument(source sourceExpr, depth int) (bool, string) {
	for depth < 128 {
		switch argument := source.value.(type) {
		case *ast.ParenExpr:
			source.value = argument.X
		case *ast.StarExpr:
			source.value = argument.X
		case *ast.IndexExpr:
			source.value = argument.X
		case *ast.IndexListExpr:
			source.value = argument.X
		case *ast.ArrayType, *ast.MapType, *ast.ChanType, *ast.FuncType, *ast.InterfaceType, *ast.StructType:
			return true, ""
		case *ast.Ident:
			binding := argument.Obj
			if binding == nil {
				if _, formal := e.typeIdentities.lexicalScope(source).argument(argument); formal {
					return true, ""
				}
				var reason string
				binding, reason = e.typeIdentities.packageBinding(source.pkg, argument.Name)
				if reason != "" {
					return false, reason
				}
			}
			if binding != nil {
				return binding.Kind == ast.Typ, ""
			}
			_, builtin := types.Universe.Lookup(argument.Name).(*types.TypeName)
			return builtin, ""
		case *ast.SelectorExpr:
			if _, opaque := e.source.externalImport(source, argument); opaque {
				// Ordinary flow retains this origin. Nested retained arguments
				// still pass through indexArgument in that value traversal.
				return false, ""
			}
			directory, imported := e.resolver.callableImport(source, argument)
			if !imported {
				return false, ""
			}
			pkg, retained := e.source.snapshot.packages[directory]
			if !retained {
				return false, ""
			}
			binding, reason := e.typeIdentities.packageBinding(pkg, argument.Sel.Name)
			if reason != "" {
				return false, reason
			}
			if binding == nil {
				return false, ""
			}
			return binding.Kind == ast.Typ, ""
		default:
			return false, ""
		}
		depth++
	}
	// Ordinary evaluation traverses the same base edges and retains the original
	// expression-depth and linkname refusal order when this proof is exhausted.
	return false, ""
}
