// Design: docs/guide/route-policy.md -- a match instance's prefix-list is completed and checked at commit
// Related: register_reference.go -- registers prefixListReference as the policy-prefix-list source
// Related: match.go -- parseMatchPrefixList, which compiles the prefix-list the leaf names

package policy

import (
	"slices"

	"github.com/ze-software/ze/internal/component/config/yang"
)

// PrefixListReferenceSource names the ze:reference source the prefix-list
// leaf of a match instance declares (`ze:reference "policy-prefix-list"`):
// the editor completes the prefix-list instances the candidate's policy root
// defines, and commit refuses a name it does not define, naming the ones it
// does.
//
// The prefix-list list is an augment of the policy root by the prefix filter
// plugin, which leaves the schema when ze_bgp compiles it out, so no leafref
// path can reach it from every build. The source lives in this package, which
// every build with a policy root links, so a build without the prefix filter
// still answers: it defines no prefix-list, so every name is refused.
const PrefixListReferenceSource = "policy-prefix-list"

// prefixListReference answers the names of the prefix-list instances the
// candidate's policy root defines. root holds one entry per top-level
// section, as Tree.ToPluginMap lowers it, so a list is a map keyed by entry
// name.
func prefixListReference(root map[string]any, _ []string) yang.ReferenceAnswer {
	policyCfg, _ := root[RootName].(map[string]any)              // no policy root: no prefix-list is defined
	lists, _ := policyCfg[prefixListFilterType].(map[string]any) // no prefix-list list: none is defined
	names := make([]string, 0, len(lists))
	for name := range lists {
		names = append(names, name)
	}
	slices.Sort(names)
	return yang.ReferenceAnswer{Names: names}
}
