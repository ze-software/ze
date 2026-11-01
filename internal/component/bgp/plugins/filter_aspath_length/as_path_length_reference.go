// Design: docs/guide/route-policy.md -- a match instance's as-path-length is completed and checked at commit
// Related: register.go -- registers asPathLengthReference as the policy-as-path-length source
// Related: condition.go -- the as-path-length condition the leaf names

package filter_aspath_length

import (
	"slices"

	"github.com/ze-software/ze/internal/component/config/yang"
	"github.com/ze-software/ze/internal/component/policy"
)

// asPathLengthReferenceSource names the ze:reference source the as-path-length
// leaf of a match instance declares (`ze:reference "policy-as-path-length"`):
// the editor completes the as-path-length instances the candidate's policy root
// defines, and commit refuses a name it does not define, naming the ones it
// does.
const asPathLengthReferenceSource = "policy-as-path-length"

// asPathLengthReference answers the names of the as-path-length instances the
// candidate's policy root defines. root holds one entry per top-level
// section, as Tree.ToPluginMap lowers it, so a list is a map keyed by entry
// name.
func asPathLengthReference(root map[string]any, _ []string) yang.ReferenceAnswer {
	policyCfg, _ := root[policy.RootName].(map[string]any) // no policy root: no as-path-length is defined
	lists, _ := policyCfg[filterTypeName].(map[string]any) // no as-path-length list: none is defined
	names := make([]string, 0, len(lists))
	for name := range lists {
		names = append(names, name)
	}
	slices.Sort(names)
	return yang.ReferenceAnswer{Names: names}
}
