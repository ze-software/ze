// Design: docs/guide/redistribution.md -- import rules and their filter chain
// Related: config.go -- compiles each import chain at chainAttachment
// Related: register.go -- registers chainAttachment, which this needs first

package redistribute

import "github.com/ze-software/ze/internal/component/policy"

// importFiltersPattern is the data path of an import rule's filters leaf-list,
// each list key a wildcard: redistribute destination * import * filters.
var importFiltersPattern = []string{importPath[0], importPath[1], "*", importPath[2], "*", "filters"}

func init() {
	// The import filters leaf declares `ze:reference "policy-filter"`; this
	// tells that source the chain compiles at chainAttachment, so completion
	// offers and `ze config validate` checks the members config.go accepts.
	policy.MustRegisterRouteChainPath(importFiltersPattern, chainAttachment)
}
