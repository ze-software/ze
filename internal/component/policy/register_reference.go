// Design: docs/guide/route-policy.md -- completing and checking filter names in a chain
// Related: reference_source.go -- filterReference, the source registered here
// Related: prefix_list_reference.go -- prefixListReference, the source registered here

package policy

import "github.com/ze-software/ze/internal/component/config/yang"

func init() {
	// A chain's filters leaf-list declares `ze:reference "policy-filter"`:
	// the editor completes it, and commit judges it, through this source.
	yang.RegisterReferenceSource(ReferenceSourceName, filterReference)
	// A match instance's prefix-list leaf declares
	// `ze:reference "policy-prefix-list"`, judged the same way.
	yang.RegisterReferenceSource(PrefixListReferenceSource, prefixListReference)
}
