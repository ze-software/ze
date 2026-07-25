package rpki_decorator

import (
	"log/slog"

	decyang "github.com/ze-software/ze/internal/component/bgp/plugins/rpki_decorator/yang"
	"github.com/ze-software/ze/internal/component/plugin/cli"
	"github.com/ze-software/ze/internal/component/plugin/registry"
	"github.com/ze-software/ze/internal/core/slogutil"
)

func init() {
	reg := registry.Registration{
		Name:         "bgp-rpki-decorator",
		Description:  "Correlates UPDATE + RPKI events into merged update-rpki events",
		Features:     "yang",
		YANG:         decyang.ZeRPKIDecoratorYANG,
		Dependencies: []string{"bgp", "bgp-rpki"},
		EventTypes:   []string{eventTypeUpdateRPKI},
		RunEngine:    RunDecorator,
		ConfigureEngineLogger: func(loggerName string) {
			setLogger(slogutil.Logger(loggerName))
		},
	}
	reg.CLIHandler = func(args []string) int {
		cfg := cli.BaseConfig(&reg)
		cfg.GetYANG = func() string { return decyang.ZeRPKIDecoratorYANG }
		cfg.ConfigLogger = func(level string) {
			setLogger(slogutil.PluginLogger(reg.Name, level))
		}
		return cli.RunPlugin(cfg, args)
	}
	if err := registry.Register(reg); err != nil {
		slog.Error("bgp-rpki-decorator: registration failed", "error", err)
	}
}
