package adj_rib_in

import (
	"log/slog"
	"os"

	"github.com/ze-software/ze/internal/component/plugin/cli"
	"github.com/ze-software/ze/internal/component/plugin/registry"
	"github.com/ze-software/ze/internal/core/slogutil"
)

func init() {
	reg := registry.Registration{
		Name:        "bgp-adj-rib-in",
		Description: "Adj-RIB-In storage (raw hex replay)",
		RFCs:        []string{"4271"},
		Features:    "yang",
		YANG:        getYANG(),
		RunEngine:   runAdjRIBInPlugin,
		ConfigureEngineLogger: func(loggerName string) {
			setLogger(slogutil.Logger(loggerName))
		},
	}
	reg.CLIHandler = func(args []string) int {
		cfg := cli.BaseConfig(&reg)
		cfg.GetYANG = getYANG
		cfg.ConfigLogger = func(level string) {
			setLogger(slogutil.PluginLogger(reg.Name, level))
		}
		return cli.RunPlugin(cfg, args)
	}
	if err := registry.Register(reg); err != nil {
		slog.Error("adj-rib-in: registration failed", "error", err)
		os.Exit(1)
	}
}
