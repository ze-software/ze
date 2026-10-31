package filter_aspath_length

import (
	falyang "github.com/ze-software/ze/internal/component/bgp/plugins/filter_aspath_length/yang"
	"github.com/ze-software/ze/internal/component/plugin/registry"
	"github.com/ze-software/ze/internal/component/policy"
)

// filterTypeName is the ze:filter list this plugin owns under the policy root.
const filterTypeName = "as-path-length"

func init() {
	// The route fields the filter type reads and sets, against which a chain
	// at an attachment point is checked (spec "Fields each filter type reads
	// and sets").
	policy.MustRegisterFilterType(filterTypeName,
		[]policy.Field{policy.FieldASPath},
		nil)

	_ = registry.Register(registry.Registration{
		Name:         "bgp-filter-aspath-length",
		Description:  "Named AS-path length filter (accept/reject based on hop count)",
		ConfigRoots:  []string{configRootBGP},
		ConfigReads:  []string{policy.RootName},
		Dependencies: []string{configRootBGP, policy.PluginName},
		YANG:         falyang.ZeFilterAsPathLengthYANG,
		FilterTypes:  []string{filterTypeName},
		RunEngine:    RunFilterAsPathLength,
		CLIHandler:   func(_ []string) int { return 0 },
	})
}
