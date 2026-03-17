package rpki

import (
	"log/slog"
	"os"

	rpkischema "codeberg.org/thomas-mangin/ze/internal/component/bgp/plugins/rpki/schema"
	"codeberg.org/thomas-mangin/ze/internal/component/plugin/cli"
	"codeberg.org/thomas-mangin/ze/internal/component/plugin/registry"
	"codeberg.org/thomas-mangin/ze/internal/core/slogutil"
)

func init() {
	reg := registry.Registration{
		Name:         "bgp-rpki",
		Description:  "RPKI origin validation via RTR protocol",
		RFCs:         []string{"6811", "8210"},
		Features:     "yang",
		YANG:         rpkischema.ZeRPKIYANG,
		ConfigRoots:  []string{"bgp"},
		Dependencies: []string{"bgp-adj-rib-in"},
		RunEngine:    RunRPKIPlugin,
		ConfigureEngineLogger: func(loggerName string) {
			setLogger(slogutil.Logger(loggerName))
		},
	}
	reg.CLIHandler = func(args []string) int {
		cfg := cli.BaseConfig(&reg)
		cfg.GetYANG = func() string { return rpkischema.ZeRPKIYANG }
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
