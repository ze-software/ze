package filter_aspath_length

import (
	falyang "github.com/ze-software/ze/internal/component/bgp/plugins/filter_aspath_length/yang"
	"github.com/ze-software/ze/internal/component/bgp/policychain"
	"github.com/ze-software/ze/internal/component/config/yang"
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

	// The as-path-length condition of a match instance (condition.go),
	// keyed like the instance it names and judged at a BGP chain only: a
	// Loc-RIB route carries no AS path, so a match naming it there is
	// refused.
	asPathLengthCondition := policy.MustRegisterMatchCondition(filterTypeName,
		[]policy.Field{policy.FieldASPath}, false)
	policychain.MatchConditions.MustAdd(asPathLengthCondition, compileBGPCondition)
	// The match leaf naming an instance completes and is checked at commit
	// through this source (as_path_length_reference.go).
	yang.RegisterReferenceSource(asPathLengthReferenceSource, asPathLengthReference)

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
