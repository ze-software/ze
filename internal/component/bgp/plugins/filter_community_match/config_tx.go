// Design: docs/architecture/core-design.md -- community match filter plugin

package filter_community_match

import (
	"github.com/ze-software/ze/internal/component/policy"
	sdk "github.com/ze-software/ze/pkg/plugin/sdk"
)

// newCommunityMatchConfigTx is the filter's side of the config transaction: the community-list definitions
// are its config. Nothing reads an installed set: a BGP chain judges a
// definition through a match instance compiled from the policy tree
// (condition.go). What the transaction keeps is the verify: a candidate the
// parser refuses is refused before any plugin applies it.
func newCommunityMatchConfigTx() *sdk.ConfigTx[map[string]*communityList] {
	parse := policy.SectionParser("filter-community-match", errFilterCommunityMatchInvalidBgpConfig, parseCommunityLists)
	return sdk.NewConfigTx(map[string]*communityList(nil), parse, installCommunityLists)
}

// installCommunityLists records what an install accepted. It holds no state, so it
// cannot fail.
func installCommunityLists(defs map[string]*communityList) error {
	logger().Debug("configured", "community-lists", len(defs))
	return nil
}
