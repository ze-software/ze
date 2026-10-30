package filter_community_match

import (
	cmyang "github.com/ze-software/ze/internal/component/bgp/plugins/filter_community_match/yang"
	"github.com/ze-software/ze/internal/component/plugin/registry"
	"github.com/ze-software/ze/internal/component/policy"
)

// filterTypeName is the ze:filter list this plugin owns under the policy root.
const filterTypeName = "community-match"

func init() {
	// The route fields the filter type reads and sets, against which a chain
	// at an attachment point is checked (spec "Fields each filter type reads
	// and sets").
	policy.MustRegisterFilterType(filterTypeName,
		[]policy.Field{policy.FieldCommunity, policy.FieldLargeCommunity, policy.FieldExtendedCommunity},
		nil)

	_ = registry.Register(registry.Registration{
		Name:         "bgp-filter-community-match",
		Description:  "Named community match filter (ordered entries, first match wins, accept/reject)",
		ConfigRoots:  []string{policy.RootName},
		Dependencies: []string{configRootBGP, policy.PluginName},
		YANG:         cmyang.ZeFilterCommunityMatchYANG,
		FilterTypes:  []string{filterTypeName},
		RunEngine:    runFilterCommunityMatch,
		CLIHandler:   func(_ []string) int { return 0 },
	})
}
