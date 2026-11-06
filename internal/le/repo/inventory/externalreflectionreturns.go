// Design: docs/architecture/plugin/feature-gates.md -- retained call results preserve their declared concrete type without exposing the callee.
package repoinventory

import "go/ast"

// returnedReflection supplies only declared Results type authority after an
// actual retained call's ownership checks. It neither invokes nor exposes the
// callee, and no implementation or existing storage is granted by the summary.
func (e *externalEffects) returnedReflection(signature *ast.FuncType, source sourceExpr, depth int) externalReflection {
	if e.amb.unknown != "" {
		return externalReflection{unknown: e.amb.unknown}
	}
	var result externalReflection
	if signature.Results == nil {
		return result
	}
	for _, field := range signature.Results.List {
		source.value = field.Type
		result.join(e.reflectedValue(source, e.typeIdentities.lexicalScope(source), depth))
	}
	return result
}
