// Design: docs/architecture/plugin/feature-gates.md -- completed receiver source queries.
// Related: externaleffecttypeidentity.go -- receiverSource preserves the raw resolver.
package repoinventory

import "go/ast"

// externalReceiverQuery retains the complete source identity and incoming depth.
// Only selected immutable receiver sources at depths 0 through 127 are stored.
type externalReceiverQuery struct {
	source sourceExpr
	depth  int
}

// externalReceiverResult is published only after receiverSource returns. Lookup
// presence marks completion, including a failed query with a nil definition.
type externalReceiverResult struct {
	definition *ast.TypeSpec
	generic    bool
	reason     string
}

// receiver reuses source facts within one effects reader. The table holds at
// most 128 outcomes per selected receiver and stores no method capabilities.
func (w *externalTypeWalk) receiver(source sourceExpr, depth int) (*ast.TypeSpec, bool, string) {
	if depth < 0 {
		return w.receiverSource(source, depth)
	}
	if depth >= 128 {
		return w.receiverSource(source, depth)
	}
	if w.effects == nil {
		return w.receiverSource(source, depth)
	}
	if w.effects.typeIdentities == nil {
		return w.receiverSource(source, depth)
	}
	query := externalReceiverQuery{source: source, depth: depth}
	if result, complete := w.effects.receiverQueries[query]; complete {
		return result.definition, result.generic, result.reason
	}
	if !w.effects.typeIdentities.receiverSources[source] {
		return w.receiverSource(source, depth)
	}
	definition, generic, reason := w.receiverSource(source, depth)
	w.effects.receiverQueries[query] = externalReceiverResult{definition: definition, generic: generic, reason: reason}
	return definition, generic, reason
}
