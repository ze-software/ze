package bmp

import (
	"log/slog"
	"os"

	bmpschema "codeberg.org/thomas-mangin/ze/internal/component/bgp/plugins/bmp/schema"
	"codeberg.org/thomas-mangin/ze/internal/component/plugin/cli"
	"codeberg.org/thomas-mangin/ze/internal/component/plugin/registry"
	"codeberg.org/thomas-mangin/ze/internal/core/slogutil"
)

func init() {
	reg := registry.Registration{
		Name:        "bgp-bmp",
		Description: "BMP receiver and sender (RFC 7854)",
		RFCs:        []string{"7854"},
		Features:    "yang",
		YANG:        bmpschema.ZeBMPConfYANG,
		ConfigRoots: []string{"bgp", "environment"},
		RunEngine:   RunBMPPlugin,
		ConfigureEngineLogger: func(loggerName string) {
			setLogger(slogutil.Logger(loggerName))
		},
	}
	reg.CLIHandler = func(args []string) int {
		cfg := cli.BaseConfig(&reg)
		cfg.ConfigLogger = func(level string) {
			setLogger(slogutil.PluginLogger(reg.Name, level))
		}
		return cli.RunPlugin(cfg, args)
	}
	if err := registry.Register(reg); err != nil {
		slog.Error("bgp-bmp: registration failed", "error", err)
		os.Exit(1)
	}
}
