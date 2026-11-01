package filter_path_asn

import (
	"log/slog"
	"os"

	fpayang "github.com/ze-software/ze/internal/component/bgp/plugins/filter_path_asn/yang"
	"github.com/ze-software/ze/internal/component/bgp/policychain"
	"github.com/ze-software/ze/internal/component/config/yang"
	"github.com/ze-software/ze/internal/component/plugin/cli"
	"github.com/ze-software/ze/internal/component/plugin/registry"
	"github.com/ze-software/ze/internal/component/policy"
	"github.com/ze-software/ze/internal/core/metrics"
)

// filterTypeName is the ze:filter list this plugin owns under the policy root.
const filterTypeName = "reject-asn"

func init() {
	// The route fields the filter type reads and sets, against which a chain
	// at an attachment point is checked (spec "Fields each filter type reads
	// and sets").
	policy.MustRegisterFilterType(filterTypeName,
		[]policy.Field{policy.FieldASPath},
		nil)

	// The reject-asn condition of a match instance (condition.go), keyed like
	// the list it names and judged at a BGP chain. The Loc-RIB route carries
	// no AS path, so it has no check there.
	rejectASNCondition := policy.MustRegisterMatchCondition(filterTypeName,
		[]policy.Field{policy.FieldASPath}, false)
	policychain.MatchConditions.MustAdd(rejectASNCondition, compileBGPCondition)
	// Its decision line, the one the filter writes (condition.go).
	policychain.MustAddMatchReporter(rejectASNCondition,
		policychain.MatchReporter{Enabled: bgpReportEnabled, Compile: compileBGPReport})
	// The match leaf naming a list completes and is checked at commit
	// through this source (reject_asn_reference.go).
	yang.RegisterReferenceSource(rejectASNReferenceSource, rejectASNReference)

	reg := registry.Registration{
		Name:         "bgp-filter-path-asn",
		Description:  "Named reject-asn list: rejects a route whose AS_PATH carries a listed ASN at a listed position",
		ConfigRoots:  []string{configRootBGP},
		ConfigReads:  []string{policy.RootName},
		Dependencies: []string{configRootBGP, policy.PluginName},
		YANG:         fpayang.ZeFilterPathAsnYANG,
		RunEngine:    runFilterPathASN,
		Commands:     commandDecls(),
		ConfigureMetrics: func(reg metrics.Registry) {
			setMetricsRegistry(reg)
		},
	}
	// The forked form (`ze plugin <name>`) runs the engine over TLS
	// connect-back, as every external plugin does.
	reg.CLIHandler = func(args []string) int {
		cfg := cli.BaseConfig(&reg)
		cfg.GetYANG = func() string { return fpayang.ZeFilterPathAsnYANG }
		return cli.RunPlugin(cfg, args)
	}
	if err := registry.Register(reg); err != nil {
		slog.Error(reg.Name+": registration failed", "error", err)
		os.Exit(1)
	}
}
