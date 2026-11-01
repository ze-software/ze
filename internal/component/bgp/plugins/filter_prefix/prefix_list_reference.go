// Design: docs/guide/route-policy.md -- a match instance's prefix-list is completed and checked at commit
// Related: register.go -- registers prefixListReference as the policy-prefix-list source
// Related: condition.go -- the prefix-list condition the leaf names

package filter_prefix

import (
	"slices"

	"github.com/ze-software/ze/internal/component/config/yang"
	"github.com/ze-software/ze/internal/component/policy"
)

// prefixListReferenceSource names the ze:reference source the prefix-list
// leaf of a match instance declares (`ze:reference "policy-prefix-list"`):
// the editor completes the prefix-list instances the candidate's policy root
// defines, and commit refuses a name it does not define, naming the ones it
// does.
const prefixListReferenceSource = "policy-prefix-list"

// prefixListReference answers the names of the prefix-list instances the
// candidate's policy root defines. root holds one entry per top-level
// section, as Tree.ToPluginMap lowers it, so a list is a map keyed by entry
// name.
func prefixListReference(root map[string]any, _ []string) yang.ReferenceAnswer {
	policyCfg, _ := root[policy.RootName].(map[string]any) // no policy root: no prefix-list is defined
	lists, _ := policyCfg[filterTypeName].(map[string]any) // no prefix-list list: none is defined
	names := make([]string, 0, len(lists))
	for name := range lists {
		names = append(names, name)
	}
	slices.Sort(names)
	return yang.ReferenceAnswer{Names: names}
}
