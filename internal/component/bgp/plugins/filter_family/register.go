package filter_family

import (
	"log/slog"
	"os"

	ffyang "github.com/ze-software/ze/internal/component/bgp/plugins/filter_family/yang"
	"github.com/ze-software/ze/internal/component/plugin/cli"
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

	reg := registry.Registration{
		Name:         "bgp-filter-family",
		Description:  "Named address-family policy filter: remove a family's NLRI or tear down the session",
		ConfigRoots:  []string{configRootBGP},
		ConfigReads:  []string{policy.RootName},
		Dependencies: []string{configRootBGP, policy.PluginName},
		YANG:         ffyang.ZeFilterFamilyYANG,
		FilterTypes:  []string{filterTypeName},
		RunEngine:    runFilterFamily,
	}
	// The forked form (`ze plugin <name>`) runs the engine over TLS
	// connect-back, as every external plugin does.
	reg.CLIHandler = func(args []string) int {
		cfg := cli.BaseConfig(&reg)
		cfg.GetYANG = func() string { return ffyang.ZeFilterFamilyYANG }
		return cli.RunPlugin(cfg, args)
	}
	if err := registry.Register(reg); err != nil {
		slog.Error(reg.Name+": registration failed", "error", err)
		os.Exit(1)
	}
}
