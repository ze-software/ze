// Design: docs/guide/route-policy.md -- a match instance's as-path is completed and checked at commit
// Related: register.go -- registers asPathListReference as the policy-as-path-list source
// Related: condition.go -- the as-path condition the leaf names

package filter_aspath

import (
	"slices"

	"github.com/ze-software/ze/internal/component/config/yang"
	"github.com/ze-software/ze/internal/component/policy"
)

// asPathListReferenceSource names the ze:reference source the as-path
// leaf of a match instance declares (`ze:reference "policy-as-path-list"`):
// the editor completes the as-path-list instances the candidate's policy root
// defines, and commit refuses a name it does not define, naming the ones it
// does.
const asPathListReferenceSource = "policy-as-path-list"

// asPathListReference answers the names of the as-path-list instances the
// candidate's policy root defines. root holds one entry per top-level
// section, as Tree.ToPluginMap lowers it, so a list is a map keyed by entry
// name.
func asPathListReference(root map[string]any, _ []string) yang.ReferenceAnswer {
	policyCfg, _ := root[policy.RootName].(map[string]any) // no policy root: no as-path-list is defined
	lists, _ := policyCfg[filterTypeName].(map[string]any) // no as-path-list list: none is defined
	names := make([]string, 0, len(lists))
	for name := range lists {
		names = append(names, name)
	}
	slices.Sort(names)
	return yang.ReferenceAnswer{Names: names}
}
