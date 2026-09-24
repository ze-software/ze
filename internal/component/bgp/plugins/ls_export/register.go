// Design: docs/architecture/wire/nlri-bgpls.md -- native exporter registration

package ls_export

import (
	"fmt"
	"log/slog"

	exportyang "github.com/ze-software/ze/internal/component/bgp/plugins/ls_export/yang"
	"github.com/ze-software/ze/internal/component/plugin/cli"
	"github.com/ze-software/ze/internal/component/plugin/registry"
	"github.com/ze-software/ze/internal/core/slogutil"
)

const exporterName = "bgp-ls-export"

var exportLogger = slog.Default()

func init() {
	// UPDATE RPCs are statically registered by the BGP feature, not a plugin dependency.
	reg := registry.Registration{Name: exporterName, Description: "Export native routing databases through BGP-LS",
		RFCs: []string{"9552", "9085", "9086", "9514"}, Features: "yang", YANG: exportyang.ZeBGPLsExportConfYANG,
		ConfigRoots: []string{exporterName}, Dependencies: []string{"bgp-nlri-ls"},
		RunEngine: runTopologyExporter, InProcessConfigVerifier: verifyExporterConfig,
		ConfigureEngineLogger: func(name string) { exportLogger = slogutil.Logger(name) }}
	reg.CLIHandler = func(args []string) int {
		cfg := cli.BaseConfig(&reg)
		cfg.GetYANG = func() string { return exportyang.ZeBGPLsExportConfYANG }
		cfg.ConfigLogger = func(level string) { exportLogger = slogutil.PluginLogger(reg.Name, level) }
		return cli.RunPlugin(cfg, args)
	}
	if err := registry.Register(reg); err != nil {
		panic(fmt.Sprintf("BUG: register native BGP-LS exporter: %v", err))
	}
}
