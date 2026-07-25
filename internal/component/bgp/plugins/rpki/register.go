package rpki

import (
	"log/slog"
	"os"

	rpkiyang "github.com/ze-software/ze/internal/component/bgp/plugins/rpki/yang"
	"github.com/ze-software/ze/internal/component/plugin/cli"
	"github.com/ze-software/ze/internal/component/plugin/registry"
	"github.com/ze-software/ze/internal/core/metrics"
	"github.com/ze-software/ze/internal/core/slogutil"
)

func init() {
	reg := registry.Registration{
		Name:         "bgp-rpki",
		Description:  "RPKI origin validation via RTR protocol",
		RFCs:         []string{"6811", "8210"},
		Features:     "yang",
		YANG:         rpkiyang.ZeRPKIYANG,
		ConfigRoots:  []string{"bgp"},
		Dependencies: []string{"bgp", "bgp-adj-rib-in"},
		RunEngine:    RunRPKIPlugin,
		ConfigureEngineLogger: func(loggerName string) {
			setLogger(slogutil.Logger(loggerName))
		},
		ConfigureMetrics: func(reg metrics.Registry) {
			SetMetricsRegistry(reg)
		},
	}
	reg.CLIHandler = func(args []string) int {
		cfg := cli.BaseConfig(&reg)
		cfg.GetYANG = func() string { return rpkiyang.ZeRPKIYANG }
		cfg.ConfigLogger = func(level string) {
			setLogger(slogutil.PluginLogger(reg.Name, level))
		}
		return cli.RunPlugin(cfg, args)
	}
	if err := registry.Register(reg); err != nil {
		slog.Error("bgp-rpki: registration failed", "error", err)
		os.Exit(1)
	}
}
