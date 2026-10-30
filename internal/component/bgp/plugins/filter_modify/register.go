package filter_modify

import (
	fmyang "github.com/ze-software/ze/internal/component/bgp/plugins/filter_modify/yang"
	"github.com/ze-software/ze/internal/component/plugin/registry"
	"github.com/ze-software/ze/internal/component/policy"
)

// filterTypeName is the ze:filter list this plugin owns under the policy root.
const filterTypeName = "modify"

func init() {
	// The route fields the filter type reads and sets, against which a chain
	// at an attachment point is checked (spec "Fields each filter type reads
	// and sets").
	policy.MustRegisterFilterType(filterTypeName,
		[]policy.Field{policy.FieldCommunity, policy.FieldLargeCommunity, policy.FieldExtendedCommunity},
		[]policy.Field{policy.FieldLocalPreference, policy.FieldMED, policy.FieldOrigin, policy.FieldNextHop, policy.FieldASPath, policy.FieldCommunity, policy.FieldLargeCommunity, policy.FieldExtendedCommunity, policy.FieldAIGP})

	_ = registry.Register(registry.Registration{
		Name:         "bgp-filter-modify",
		Description:  "Named route attribute modifier (set local-preference, med, origin, next-hop)",
		ConfigRoots:  []string{policy.RootName},
		ConfigReads:  []string{configRootBGP},
		Dependencies: []string{configRootBGP, policy.PluginName},
		YANG:         fmyang.ZeFilterModifyYANG,
		FilterTypes:  []string{filterTypeName},
		RunEngine:    runFilterModify,
		CLIHandler:   func(_ []string) int { return 0 },
	})
}
