package healthcheck

import (
	"os"

	"github.com/ze-software/ze/internal/component/bgp/plugins/healthcheck/yang"
	"github.com/ze-software/ze/internal/component/plugin/cli"
	"github.com/ze-software/ze/internal/component/plugin/registry"
	"github.com/ze-software/ze/internal/core/slogutil"
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
		YANG:         yang.ZeHealthcheckConfYANG,
		RunEngine:    runHealthcheckPlugin,
		ConfigureEngineLogger: func(loggerName string) {
			SetLogger(slogutil.Logger(loggerName))
		},
	}
	reg.CLIHandler = func(args []string) int {
		cfg := cli.BaseConfig(&reg)
		cfg.ConfigLogger = func(level string) {
			SetLogger(slogutil.PluginLogger(reg.Name, level))
		}
		cfg.RunEngine = runHealthcheckPlugin
		return cli.RunPlugin(cfg, args)
	}
	if err := registry.Register(reg); err != nil {
		slogutil.Logger("bgp.healthcheck").Error("registration failed", "error", err)
		os.Exit(1)
	}
}
