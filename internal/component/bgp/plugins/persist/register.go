package persist

import (
	"log/slog"
	"os"

	"github.com/ze-software/ze/internal/component/plugin/cli"
	"github.com/ze-software/ze/internal/component/plugin/registry"
	"github.com/ze-software/ze/internal/core/metrics"
	"github.com/ze-software/ze/internal/core/slogutil"
)

func init() {
	reg := registry.Registration{
		Name:        "bgp-persist",
		Description: "Route Persistence",
		RunEngine:   RunPersistServer,
		ConfigureEngineLogger: func(loggerName string) {
			SetPersistLogger(slogutil.Logger(loggerName))
		},
		ConfigureMetrics: func(reg metrics.Registry) {
			SetMetricsRegistry(reg)
		},
	}
	reg.CLIHandler = func(args []string) int {
		cfg := cli.BaseConfig(&reg)
		cfg.ConfigLogger = func(level string) {
			SetPersistLogger(slogutil.PluginLogger(reg.Name, level))
		}
		return cli.RunPlugin(cfg, args)
	}
	if err := registry.Register(reg); err != nil {
		slog.Error("persist: registration failed", "error", err)
		os.Exit(1)
	}
}
