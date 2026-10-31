package filter_aspath

import (
	fayang "github.com/ze-software/ze/internal/component/bgp/plugins/filter_aspath/yang"
	"github.com/ze-software/ze/internal/component/plugin/registry"
	"github.com/ze-software/ze/internal/component/policy"
)

// filterTypeName is the ze:filter list this plugin owns under the policy root.
const filterTypeName = "as-path-list"

func init() {
	// The route fields the filter type reads and sets, against which a chain
	// at an attachment point is checked (spec "Fields each filter type reads
	// and sets").
	policy.MustRegisterFilterType(filterTypeName,
		[]policy.Field{policy.FieldASPath},
		nil)

	_ = registry.Register(registry.Registration{
		Name:         "bgp-filter-aspath",
		Description:  "Named AS-path regex filter (ordered entries, first match wins, accept/reject)",
		ConfigRoots:  []string{configRootBGP},
		ConfigReads:  []string{policy.RootName},
		Dependencies: []string{configRootBGP, policy.PluginName},
		YANG:         fayang.ZeFilterAsPathYANG,
		FilterTypes:  []string{filterTypeName},
		RunEngine:    runFilterAsPath,
		CLIHandler:   func(_ []string) int { return 0 },
	})
}
