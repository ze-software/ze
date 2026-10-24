// Design: docs/architecture/wire/nlri-bgpls.md -- native exporter registration

package ls_export

import (
	"fmt"
	"log/slog"

	exportyang "github.com/ze-software/ze/internal/component/bgp/plugins/ls_export/yang"
	"github.com/ze-software/ze/internal/component/config"
	"github.com/ze-software/ze/internal/component/plugin/cli"
	"github.com/ze-software/ze/internal/component/plugin/registry"
	"github.com/ze-software/ze/internal/core/slogutil"
)

const exporterName = "bgp-ls-export"

// The IGP plugins that publish a BGP-LS source under their own name
// (linkstateevents.RegisterSource in internal/plugins/isis and
// internal/plugins/ospf). Each is gated by its own build tag, so a `domain`
// entry may name a source the build left out. bgp-epe needs no need: it is a
// BGP plugin, built whenever this exporter is.
const (
	isisSource = "isis"
	ospfSource = "ospf"
)

// sourceNeed is the config need of a `domain` entry naming source: the
// exporter can only map a namespace some compiled-in plugin publishes, so the
// config is refused when that plugin is absent, and in a build that has it the
// plugin is pulled and ordered before the exporter. A pulled IGP with no root
// of its own stays idle (IS-IS with no NET, OSPF with no `ospf` block).
func sourceNeed(source string) registry.ConfigNeed {
	return registry.ConfigNeed{
		Path:      []string{exporterName, "domain", registry.AnyEntry, "source"},
		Value:     source,
		Providers: []string{source},
	}
}

// sourceNeeds are the exporter's config needs, one per IGP source a build may
// leave out. verifyDomainSources reads the same slice, so a source named here
// is refused by its need, naming the build tag, rather than as unknown.
var sourceNeeds = []registry.ConfigNeed{sourceNeed(isisSource), sourceNeed(ospfSource)}

var exportLogger = slog.Default()

func init() {
	// UPDATE RPCs are statically registered by the BGP feature, not a plugin dependency.
	reg := registry.Registration{Name: exporterName, Description: "Export native routing databases through BGP-LS",
		RFCs: []string{"9552", "9085", "9086", "9514"}, Features: "yang", YANG: exportyang.ZeBGPLsExportConfYANG,
		ConfigRoots: []string{exporterName}, Dependencies: []string{"bgp-nlri-ls"},
		OptionalDependencies: []string{isisSource, ospfSource},
		ConfigNeeds:          sourceNeeds,
		RunEngine:            runTopologyExporter, InProcessConfigVerifier: verifyExporterConfig,
		ConfigureEngineLogger: func(name string) { exportLogger = slogutil.Logger(name) }}
	reg.CLIHandler = func(args []string) int {
		cfg := cli.BaseConfig(&reg)
		cfg.GetYANG = func() string { return exportyang.ZeBGPLsExportConfYANG }
		cfg.ConfigLogger = func(level string) { exportLogger = slogutil.PluginLogger(reg.Name, level) }
		return cli.RunPlugin(cfg, args)
	}
	// The domain source check runs as an original-tree validator, which every
	// door reaches: LoadConfig at boot and reload, `ze config validate`, and
	// the editor commit. The exporter's own verifier does not run at boot.
	if err := config.RegisterTreeValidator(exporterName+" domain source", verifyDomainSources); err != nil {
		panic("BUG: register BGP-LS domain source validator: " + err.Error())
	}
	if err := registry.Register(reg); err != nil {
		panic(fmt.Sprintf("BUG: register native BGP-LS exporter: %v", err))
	}
}
