// Design: docs/architecture/plugin/feature-gates.md -- reflection backedges preserve exact substitutions and method visibility.
package repoinventory

import "go/ast"

func externalReflectionFrameEqual(left, right externalReflectionFrame, depth int) bool {
	if left.definition != right.definition {
		return false
	}
	if left.methods != right.methods {
		return false
	}
	if left.valueRoot != right.valueRoot {
		return false
	}
	if len(left.arguments) != len(right.arguments) {
		return false
	}
	for index, argument := range left.arguments {
		if !externalReflectionArgumentEqual(argument, right.arguments[index], depth+1) {
			return false
		}
	}
	return true
}

// externalReflectionArgumentEqual proves equality only for the same retained
// expression and every parameter substitution it mentions. Fresh equivalent
// scope chains are safe; unproved equality continues expansion to its bound.
func externalReflectionArgumentEqual(left, right externalTypeArgument, depth int) bool {
	if depth >= 128 {
		return false
	}
	left = externalResolvedTypeArgument(left, depth)
	right = externalResolvedTypeArgument(right, depth)
	if left.source != right.source {
		return false
	}
	if left.source.value == nil {
		return false
	}
	if left.scope == right.scope {
		return true
	}
	equal := true
	structuralDepth := depth
	ast.Inspect(left.source.value, func(node ast.Node) bool {
		if node == nil {
			structuralDepth--
			return false
		}
		if !equal {
			return false
		}
		if structuralDepth >= 128 {
			equal = false
			return false
		}
		structuralDepth++
		name, identifier := node.(*ast.Ident)
		if !identifier {
			return true
		}
		first, firstFound := left.scope.argument(name)
		second, secondFound := right.scope.argument(name)
		if firstFound != secondFound {
			equal = false
			return false
		}
		if firstFound {
			equal = externalReflectionArgumentEqual(first, second, depth+1)
		}
		return equal
	})
	return equal
}
