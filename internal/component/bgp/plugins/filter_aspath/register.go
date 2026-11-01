package filter_aspath

import (
	fayang "github.com/ze-software/ze/internal/component/bgp/plugins/filter_aspath/yang"
	"github.com/ze-software/ze/internal/component/bgp/policychain"
	"github.com/ze-software/ze/internal/component/config/yang"
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

	// The as-path condition of a match instance (condition.go), keyed by
	// the field it reads and judged at a BGP chain only: a Loc-RIB route
	// carries no AS path, so a match naming it there is refused.
	asPathCondition := policy.MustRegisterMatchCondition(policy.FieldASPath.String(),
		[]policy.Field{policy.FieldASPath}, false)
	policychain.MatchConditions.MustAdd(asPathCondition, compileBGPCondition)
	// The match leaf naming a list completes and is checked at commit
	// through this source (as_path_list_reference.go).
	yang.RegisterReferenceSource(asPathListReferenceSource, asPathListReference)

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
