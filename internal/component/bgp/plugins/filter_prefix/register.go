package filter_prefix

import (
	"log/slog"
	"os"

	fpyang "github.com/ze-software/ze/internal/component/bgp/plugins/filter_prefix/yang"
	"github.com/ze-software/ze/internal/component/bgp/policychain"
	"github.com/ze-software/ze/internal/component/config/yang"
	"github.com/ze-software/ze/internal/component/plugin/cli"
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
	// The in-process evaluator the Loc-RIB import gate runs (rib_evaluator.go).
	policy.MustRegisterRouteEvaluator(filterTypeName, compileRouteEvaluator)

	// The prefix-list condition of a match instance (condition.go): the one
	// prefix check a match runs, keyed like the list it names.
	prefixListCondition := policy.MustRegisterMatchCondition(filterTypeName,
		[]policy.Field{policy.FieldPrefix, policy.FieldFamily}, false)
	policy.RouteConditions.MustAdd(prefixListCondition, compileRouteCondition)
	// The same condition at a BGP chain, judged prefix by prefix.
	policychain.MatchConditions.MustAdd(prefixListCondition, compileBGPCondition)
	// Its decision line, the one the list's filter writes.
	policychain.MustAddMatchReporter(prefixListCondition,
		policychain.MatchReporter{Enabled: bgpReportEnabled, Compile: compileBGPReport})
	// The match leaf naming a list completes and is checked at commit
	// through this source (prefix_list_reference.go).
	yang.RegisterReferenceSource(prefixListReferenceSource, prefixListReference)

	reg := registry.Registration{
		Name:         "bgp-filter-prefix",
		Description:  "Named prefix-list filter (CIDR + ge/le + accept/reject)",
		ConfigRoots:  []string{configRootBGP},
		ConfigReads:  []string{policy.RootName},
		Dependencies: []string{configRootBGP, policy.PluginName},
		YANG:         fpyang.ZeFilterPrefixYANG,
		RunEngine:    RunFilterPrefix,
	}
	// The forked form (`ze plugin <name>`) runs the engine over TLS
	// connect-back, as every external plugin does.
	reg.CLIHandler = func(args []string) int {
		cfg := cli.BaseConfig(&reg)
		cfg.GetYANG = func() string { return fpyang.ZeFilterPrefixYANG }
		return cli.RunPlugin(cfg, args)
	}
	if err := registry.Register(reg); err != nil {
		slog.Error(reg.Name+": registration failed", "error", err)
		os.Exit(1)
	}
}
