// Design: docs/guide/bfd.md -- a client-named profile is checked at commit
// Related: register.go -- registers profileReference as the bfd-profile source
// Related: profile_check.go -- the mode and authentication check a client still runs

package api

import (
	"slices"

	"github.com/ze-software/ze/internal/component/config/yang"
)

// ProfileReferenceSource names the ze:reference source a client's profile
// leaf declares (`ze:reference "bfd-profile"`): the editor completes the
// profiles the candidate's bfd section defines, and commit refuses a name it
// does not define, naming the ones it does.
//
// A client's leaf lives in another module than the bfd profile list, and the
// bfd module leaves the schema when ze_bfd compiles it out, so no leafref
// path can reach the list from every build. The source lives in this package,
// which every client build links, so a build without BFD still answers: it
// has no bfd section, so every name is refused as naming no profile.
const ProfileReferenceSource = "bfd-profile"

// profileList is the bfd module's list of named profiles (ze-bfd-conf.yang,
// `list profile`), keyed by name.
const profileList = "profile"

// profileReference answers the names of the profiles the candidate's bfd
// section defines. root holds one entry per top-level section, as
// Tree.ToPluginMap lowers it, so a list is a map keyed by entry name.
func profileReference(root map[string]any, _ []string) yang.ReferenceAnswer {
	section, _ := root[ConfigRoot].(map[string]any) // no bfd section: no profile is defined
	profiles, _ := section[profileList].(map[string]any)
	names := make([]string, 0, len(profiles))
	for name := range profiles {
		names = append(names, name)
	}
	slices.Sort(names)
	return yang.ReferenceAnswer{Names: names}
}
