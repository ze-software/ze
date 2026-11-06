// Design: docs/architecture/plugin/feature-gates.md -- nil interface values reveal no concrete descriptor, including through aliases and substitutions.
package repoinventory

import (
	"go/ast"
	"go/types"
)

// interfaceRoot classifies only the substituted value's outer representation.
// Generic arguments are not exposed here: a nil generic interface still has no
// runtime descriptor. True descriptor child edges use the full descriptor walk.
// Alias/source traversal and unproved cycles remain bounded at 128 steps.
func (w *externalReflectionWalk) interfaceRoot(source sourceExpr, scope *externalTypeScope, depth int) (bool, string) {
	identity := externalTypeWalk{effects: w.effects}
	var path []externalReflectionFrame
	for depth < 128 {
		depth++
		var arguments []externalTypeArgument
		switch datatype := source.value.(type) {
		case *ast.InterfaceType:
			return true, ""
		case *ast.StarExpr, *ast.ArrayType, *ast.MapType, *ast.ChanType, *ast.StructType, *ast.FuncType:
			return false, ""
		case *ast.ParenExpr:
			source.value = datatype.X
			continue
		case *ast.Ident:
			if argument, found := scope.argument(datatype); found {
				if argument.source.value == nil {
					return false, "reflected value parameter has no retained argument"
				}
				source = argument.source
				scope = argument.scope
				continue
			}
			binding := datatype.Obj
			reason := ""
			if binding == nil {
				binding, reason = w.effects.typeIdentities.packageBinding(source.pkg, datatype.Name)
			}
			if reason != "" {
				return false, reason
			}
			if binding == nil {
				if builtin, found := types.Universe.Lookup(datatype.Name).(*types.TypeName); found {
					_, interfaceType := builtin.Type().Underlying().(*types.Interface)
					return interfaceType, ""
				}
				return false, "reflected value type ownership is unresolved: " + datatype.Name
			}
		case *ast.SelectorExpr:
			if _, external := w.effects.source.externalImport(source, datatype); external {
				// The full walker retains this opaque origin and its witness;
				// no retained generic argument or method is granted by this probe.
				return false, ""
			}
		case *ast.IndexExpr:
			arguments = []externalTypeArgument{{source: sourceExpr{value: datatype.Index, file: source.file, pkg: source.pkg}, scope: scope}}
			source.value = datatype.X
		case *ast.IndexListExpr:
			arguments = make([]externalTypeArgument, 0, len(datatype.Indices))
			for _, argument := range datatype.Indices {
				arguments = append(arguments, externalTypeArgument{source: sourceExpr{value: argument, file: source.file, pkg: source.pkg}, scope: scope})
			}
			source.value = datatype.X
		default:
			return false, "unsupported reflected value root representation"
		}
		definition, owner, reason := identity.definition(source)
		if reason != "" {
			return false, reason
		}
		if len(arguments) != definition.TypeParams.NumFields() {
			return false, "reflected value generic argument count is unresolved"
		}
		for index, argument := range arguments {
			arguments[index] = externalResolvedTypeArgument(argument, depth)
			if arguments[index].source.value == nil {
				return false, "reflected value generic argument ownership is unresolved"
			}
		}
		frame := externalReflectionFrame{definition: definition, arguments: arguments, valueRoot: true}
		for _, active := range path {
			if externalReflectionFrameEqual(active, frame, depth) {
				return false, "cyclic reflected value root representation"
			}
		}
		path = append(path, frame)
		scope = w.effects.typeIdentities.lexicalScope(owner)
		if len(arguments) != 0 {
			scope = externalBindTypeParameters(scope, definition.TypeParams, arguments)
		}
		source = owner
	}
	return false, "reflected value root representation exceeds 128 steps"
}
