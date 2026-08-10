package watchdog

import (
	"bytes"
	"os"

	"github.com/ze-software/ze/internal/component/plugin/cli"
	"github.com/ze-software/ze/internal/component/plugin/registry"
	"github.com/ze-software/ze/internal/core/metrics"
	"github.com/ze-software/ze/internal/core/slogutil"
)

func init() {
	pluginSetup()
}

func pluginSetup() {
	reg := registry.Registration{
		Name:         "bgp-watchdog",
		Description:  "Watchdog route management plugin",
		ConfigRoots:  []string{"bgp"},
		Dependencies: []string{"bgp"},
		RunEngine:    runWatchdogPlugin,
		InProcessDecoder: func(input, output *bytes.Buffer) int {
			return 0
		},
		ConfigureEngineLogger: func(loggerName string) {
			setLogger(slogutil.Logger(loggerName))
		},
		ConfigureMetrics: func(reg metrics.Registry) {
			SetMetricsRegistry(reg)
		},
	}
	reg.CLIHandler = func(args []string) int {
		cfg := cli.BaseConfig(&reg)
		cfg.ConfigLogger = func(level string) {
			setLogger(slogutil.PluginLogger(reg.Name, level))
		}
		cfg.RunEngine = runWatchdogPlugin
		return cli.RunPlugin(cfg, args)
	}
	if err := registry.Register(reg); err != nil {
		slogutil.Logger("bgp.watchdog").Error("registration failed", "error", err)
		os.Exit(1)
	}
}
