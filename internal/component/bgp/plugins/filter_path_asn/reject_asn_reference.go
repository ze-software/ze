// Design: docs/guide/route-policy.md -- a match instance's reject-asn is completed and checked at commit
// Related: register.go -- registers rejectASNReference as the policy-reject-asn source
// Related: condition.go -- the reject-asn condition the leaf names

package filter_path_asn

import (
	"slices"

	"github.com/ze-software/ze/internal/component/config/yang"
	"github.com/ze-software/ze/internal/component/policy"
)

// rejectASNReferenceSource names the ze:reference source the reject-asn
// leaf of a match instance declares (`ze:reference "policy-reject-asn"`):
// the editor completes the reject-asn instances the candidate's policy
// root defines, and commit refuses a name it does not define, naming the
// ones it does.
const rejectASNReferenceSource = "policy-reject-asn"

// rejectASNReference answers the names of the reject-asn instances
// the candidate's policy root defines. root holds one entry per top-level
// section, as Tree.ToPluginMap lowers it, so a list is a map keyed by entry
// name.
func rejectASNReference(root map[string]any, _ []string) yang.ReferenceAnswer {
	policyCfg, _ := root[policy.RootName].(map[string]any) // no policy root: no reject-asn is defined
	lists, _ := policyCfg[filterTypeName].(map[string]any) // no reject-asn list: none is defined
	names := make([]string, 0, len(lists))
	for name := range lists {
		names = append(names, name)
	}
	slices.Sort(names)
	return yang.ReferenceAnswer{Names: names}
}
