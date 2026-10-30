package filter_prefix

import (
	fpyang "github.com/ze-software/ze/internal/component/bgp/plugins/filter_prefix/yang"
	"github.com/ze-software/ze/internal/component/plugin/registry"
	"github.com/ze-software/ze/internal/component/policy"
)

// filterTypeName is the ze:filter list this plugin owns under the policy root.
const filterTypeName = "prefix-list"

func init() {
	// The route fields the filter type reads and sets, against which a chain
	// at an attachment point is checked (spec "Fields each filter type reads
	// and sets").
	policy.MustRegisterFilterType(filterTypeName,
		[]policy.Field{policy.FieldPrefix, policy.FieldFamily},
		nil)

	_ = registry.Register(registry.Registration{
		Name:         "bgp-filter-prefix",
		Description:  "Named prefix-list filter (CIDR + ge/le + accept/reject)",
		ConfigRoots:  []string{policy.RootName},
		Dependencies: []string{configRootBGP, policy.PluginName},
		YANG:         fpyang.ZeFilterPrefixYANG,
		FilterTypes:  []string{filterTypeName},
		RunEngine:    RunFilterPrefix,
		CLIHandler:   func(_ []string) int { return 0 },
	})
}
