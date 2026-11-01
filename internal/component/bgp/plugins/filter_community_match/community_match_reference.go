// Design: docs/guide/route-policy.md -- a match instance's community is completed and checked at commit
// Related: register.go -- registers communityMatchReference as the policy-community-match source
// Related: condition.go -- the community condition the leaf names

package filter_community_match

import (
	"slices"

	"github.com/ze-software/ze/internal/component/config/yang"
	"github.com/ze-software/ze/internal/component/policy"
)

// communityMatchReferenceSource names the ze:reference source the community
// leaf of a match instance declares (`ze:reference "policy-community-match"`):
// the editor completes the community-match instances the candidate's policy
// root defines, and commit refuses a name it does not define, naming the
// ones it does.
const communityMatchReferenceSource = "policy-community-match"

// communityMatchReference answers the names of the community-match instances
// the candidate's policy root defines. root holds one entry per top-level
// section, as Tree.ToPluginMap lowers it, so a list is a map keyed by entry
// name.
func communityMatchReference(root map[string]any, _ []string) yang.ReferenceAnswer {
	policyCfg, _ := root[policy.RootName].(map[string]any) // no policy root: no community-match is defined
	lists, _ := policyCfg[filterTypeName].(map[string]any) // no community-match list: none is defined
	names := make([]string, 0, len(lists))
	for name := range lists {
		names = append(names, name)
	}
	slices.Sort(names)
	return yang.ReferenceAnswer{Names: names}
}
