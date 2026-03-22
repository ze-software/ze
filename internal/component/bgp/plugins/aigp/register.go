package aigp

import (
	"log/slog"

	"codeberg.org/thomas-mangin/ze/internal/component/bgp/attribute"
	"codeberg.org/thomas-mangin/ze/internal/component/plugin/cli"
	"codeberg.org/thomas-mangin/ze/internal/component/plugin/registry"
	"codeberg.org/thomas-mangin/ze/internal/core/slogutil"
)

func init() {
	// RFC 7311: Register AIGP attribute (type 26).
	attribute.RegisterName(26, "AIGP")

	reg := registry.Registration{
		Name:        "bgp-aigp",
		Description: "Accumulated IGP Metric (RFC 7311)",
		RFCs:        []string{"7311"},
		RunEngine:   RunAIGPPlugin,
		ConfigureEngineLogger: func(loggerName string) {
			SetAIGPLogger(slogutil.Logger(loggerName))
		},
	}
	reg.CLIHandler = func(args []string) int {
		cfg := cli.BaseConfig(&reg)
		cfg.ConfigLogger = func(level string) {
			SetAIGPLogger(slogutil.PluginLogger(reg.Name, level))
		}
		return cli.RunPlugin(cfg, args)
	}
	if err := registry.Register(reg); err != nil {
		slog.Error("aigp: registration failed", "error", err)
	}
}
