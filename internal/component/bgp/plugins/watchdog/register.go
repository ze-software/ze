package watchdog

import (
	"bytes"
	"os"

	"codeberg.org/thomas-mangin/ze/internal/component/plugin/cli"
	"codeberg.org/thomas-mangin/ze/internal/component/plugin/registry"
	"codeberg.org/thomas-mangin/ze/internal/core/slogutil"
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
		RunEngine:    RunWatchdogPlugin,
		InProcessDecoder: func(input, output *bytes.Buffer) int {
			return 0
		},
		ConfigureEngineLogger: func(loggerName string) {
			setLogger(slogutil.Logger(loggerName))
		},
	}
	reg.CLIHandler = func(args []string) int {
		cfg := cli.BaseConfig(&reg)
		cfg.ConfigLogger = func(level string) {
			setLogger(slogutil.PluginLogger(reg.Name, level))
		}
		cfg.RunEngine = RunWatchdogPlugin
		return cli.RunPlugin(cfg, args)
	}
	if err := registry.Register(reg); err != nil {
		slogutil.Logger("bgp.watchdog").Error("registration failed", "error", err)
		os.Exit(1)
	}
}
