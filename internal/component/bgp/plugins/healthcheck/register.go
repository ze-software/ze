package healthcheck

import (
	"os"

	"codeberg.org/thomas-mangin/ze/internal/component/bgp/plugins/healthcheck/schema"
	"codeberg.org/thomas-mangin/ze/internal/component/plugin/cli"
	"codeberg.org/thomas-mangin/ze/internal/component/plugin/registry"
	"codeberg.org/thomas-mangin/ze/internal/core/slogutil"
)

func init() {
	pluginSetup()
}

func pluginSetup() {
	reg := registry.Registration{
		Name:         "bgp-healthcheck",
		Description:  "Service healthcheck plugin with watchdog route control",
		ConfigRoots:  []string{"bgp"},
		Dependencies: []string{"bgp", "bgp-watchdog"},
		Features:     "yang",
		YANG:         schema.ZeHealthcheckConfYANG,
		RunEngine:    RunHealthcheckPlugin,
		ConfigureEngineLogger: func(loggerName string) {
			SetLogger(slogutil.Logger(loggerName))
		},
	}
	reg.CLIHandler = func(args []string) int {
		cfg := cli.BaseConfig(&reg)
		cfg.ConfigLogger = func(level string) {
			SetLogger(slogutil.PluginLogger(reg.Name, level))
		}
		cfg.RunEngine = RunHealthcheckPlugin
		return cli.RunPlugin(cfg, args)
	}
	if err := registry.Register(reg); err != nil {
		slogutil.Logger("bgp.healthcheck").Error("registration failed", "error", err)
		os.Exit(1)
	}
}
