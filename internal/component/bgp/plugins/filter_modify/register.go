package filter_modify

import (
	"log/slog"
	"os"

	fmyang "github.com/ze-software/ze/internal/component/bgp/plugins/filter_modify/yang"
	"github.com/ze-software/ze/internal/component/plugin/cli"
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
		[]policy.Field{policy.FieldCommunity, policy.FieldLargeCommunity, policy.FieldExtendedCommunity,
			policy.FieldProtocol, policy.FieldTag, policy.FieldLevel, policy.FieldRouteType},
		[]policy.Field{policy.FieldLocalPreference, policy.FieldMED, policy.FieldOrigin, policy.FieldNextHop, policy.FieldASPath,
			policy.FieldCommunity, policy.FieldLargeCommunity, policy.FieldExtendedCommunity, policy.FieldAIGP,
			policy.FieldDistance, policy.FieldTag, policy.FieldMetric, policy.FieldMetricType})
	// Each instance is checked against only what it names, so a BGP-only
	// modifier and a Loc-RIB one share the type (rib_evaluator.go).
	policy.MustRegisterInstanceFields(filterTypeName, modifyInstanceFields)
	policy.MustRegisterRouteEvaluator(filterTypeName, compileRouteEvaluator)

	reg := registry.Registration{
		Name:         "bgp-filter-modify",
		Description:  "Named route attribute modifier (set local-preference, med, origin, next-hop)",
		ConfigRoots:  []string{configRootBGP},
		ConfigReads:  []string{policy.RootName},
		Dependencies: []string{configRootBGP, policy.PluginName},
		YANG:         fmyang.ZeFilterModifyYANG,
		FilterTypes:  []string{filterTypeName},
		RunEngine:    runFilterModify,
	}
	// The forked form (`ze plugin <name>`) runs the engine over TLS
	// connect-back, as every external plugin does.
	reg.CLIHandler = func(args []string) int {
		cfg := cli.BaseConfig(&reg)
		cfg.GetYANG = func() string { return fmyang.ZeFilterModifyYANG }
		return cli.RunPlugin(cfg, args)
	}
	if err := registry.Register(reg); err != nil {
		slog.Error(reg.Name+": registration failed", "error", err)
		os.Exit(1)
	}
}
