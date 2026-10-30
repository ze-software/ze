// Design: docs/architecture/core-design.md -- community match filter plugin
// Related: filter_community_match.go -- listsByName, the state an install replaces

package filter_community_match

import (
	"github.com/ze-software/ze/internal/component/policy"
	sdk "github.com/ze-software/ze/pkg/plugin/sdk"
)

// newCommunityMatchConfigTx is the filter's side of the config transaction: the community-list definitions
// are its config, and an install stores them whole, so a filter-update reads
// the old set or the new one and never a mix. Before any install none is
// defined.
func newCommunityMatchConfigTx() *sdk.ConfigTx[map[string]*communityList] {
	parse := policy.SectionParser("filter-community-match", errFilterCommunityMatchInvalidBgpConfig, parseCommunityLists)
	return sdk.NewConfigTx(map[string]*communityList(nil), parse, installCommunityLists)
}

// installCommunityLists makes the definitions every later filter-update reads. It
// cannot fail.
func installCommunityLists(defs map[string]*communityList) error {
	listsByName.Store(&defs)
	logger().Debug("configured", "community-lists", len(defs))
	return nil
}
