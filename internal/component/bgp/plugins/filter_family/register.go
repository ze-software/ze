package filter_family

import (
	ffyang "github.com/ze-software/ze/internal/component/bgp/plugins/filter_family/yang"
	"github.com/ze-software/ze/internal/component/plugin/registry"
	"github.com/ze-software/ze/internal/component/policy"
)

// filterTypeName is the ze:filter list this plugin owns under the policy root.
const filterTypeName = "family-filter"

func init() {
	// The route fields the filter type reads and sets, against which a chain
	// at an attachment point is checked (spec "Fields each filter type reads
	// and sets").
	policy.MustRegisterFilterType(filterTypeName,
		[]policy.Field{policy.FieldFamily},
		nil)
	// The in-process evaluator the Loc-RIB import gate runs (rib_evaluator.go).
	policy.MustRegisterRouteEvaluator(filterTypeName, compileRouteEvaluator)

	_ = registry.Register(registry.Registration{
		Name:         "bgp-filter-family",
		Description:  "Named address-family policy filter: remove a family's NLRI or tear down the session",
		ConfigRoots:  []string{policy.RootName},
		ConfigReads:  []string{configRootBGP},
		Dependencies: []string{configRootBGP, policy.PluginName},
		YANG:         ffyang.ZeFilterFamilyYANG,
		FilterTypes:  []string{filterTypeName},
		RunEngine:    runFilterFamily,
		CLIHandler:   func(_ []string) int { return 0 },
	})
}
