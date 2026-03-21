package rpki_decorator

import (
	"log/slog"

	decschema "codeberg.org/thomas-mangin/ze/internal/component/bgp/plugins/rpki_decorator/schema"
	"codeberg.org/thomas-mangin/ze/internal/component/plugin/cli"
	"codeberg.org/thomas-mangin/ze/internal/component/plugin/registry"
	"codeberg.org/thomas-mangin/ze/internal/core/slogutil"
)

func init() {
	reg := registry.Registration{
		Name:         "bgp-rpki-decorator",
		Description:  "Correlates UPDATE + RPKI events into merged update-rpki events",
		Features:     "yang",
		YANG:         decschema.ZeRPKIDecoratorYANG,
		Dependencies: []string{"bgp-rpki"},
		EventTypes:   []string{eventTypeUpdateRPKI},
		RunEngine:    RunDecorator,
		ConfigureEngineLogger: func(loggerName string) {
			setLogger(slogutil.Logger(loggerName))
		},
	}
	reg.CLIHandler = func(args []string) int {
		cfg := cli.BaseConfig(&reg)
		cfg.GetYANG = func() string { return decschema.ZeRPKIDecoratorYANG }
		cfg.ConfigLogger = func(level string) {
			setLogger(slogutil.PluginLogger(reg.Name, level))
		}
		return cli.RunPlugin(cfg, args)
	}
	if err := registry.Register(reg); err != nil {
		slog.Error("bgp-rpki-decorator: registration failed", "error", err)
	}
}
