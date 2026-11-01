package filter_community_match

import (
	"log/slog"
	"os"

	cmyang "github.com/ze-software/ze/internal/component/bgp/plugins/filter_community_match/yang"
	"github.com/ze-software/ze/internal/component/bgp/policychain"
	"github.com/ze-software/ze/internal/component/config/yang"
	"github.com/ze-software/ze/internal/component/plugin/cli"
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

	// The community condition of a match instance (condition.go), keyed by
	// the field it names and judged at a BGP chain over the attributes'
	// octets. The Loc-RIB route carries no community, so it has no check
	// there and a match naming it is refused at a Loc-RIB attachment.
	communityCondition := policy.MustRegisterMatchCondition(policy.FieldCommunity.String(),
		[]policy.Field{policy.FieldCommunity, policy.FieldLargeCommunity, policy.FieldExtendedCommunity}, false)
	policychain.MatchConditions.MustAdd(communityCondition, compileBGPCondition)
	// Its decision line, the one the filter writes (condition.go).
	policychain.MustAddMatchReporter(communityCondition,
		policychain.MatchReporter{Enabled: bgpReportEnabled, Compile: compileBGPReport})
	// The match leaf naming a list completes and is checked at commit
	// through this source (community_match_reference.go).
	yang.RegisterReferenceSource(communityMatchReferenceSource, communityMatchReference)

	reg := registry.Registration{
		Name:         "bgp-filter-community-match",
		Description:  "Named community match filter (ordered entries, first match wins, accept/reject)",
		ConfigRoots:  []string{configRootBGP},
		ConfigReads:  []string{policy.RootName},
		Dependencies: []string{configRootBGP, policy.PluginName},
		YANG:         cmyang.ZeFilterCommunityMatchYANG,
		RunEngine:    runFilterCommunityMatch,
	}
	// The forked form (`ze plugin <name>`) runs the engine over TLS
	// connect-back, as every external plugin does.
	reg.CLIHandler = func(args []string) int {
		cfg := cli.BaseConfig(&reg)
		cfg.GetYANG = func() string { return cmyang.ZeFilterCommunityMatchYANG }
		return cli.RunPlugin(cfg, args)
	}
	if err := registry.Register(reg); err != nil {
		slog.Error(reg.Name+": registration failed", "error", err)
		os.Exit(1)
	}
}
