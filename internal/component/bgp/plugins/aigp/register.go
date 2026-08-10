package aigp

import (
	"log/slog"
	"strconv"

	"github.com/ze-software/ze/internal/component/plugin/cli"
	"github.com/ze-software/ze/internal/component/plugin/registry"
	"github.com/ze-software/ze/internal/core/bgp/attribute"
	"github.com/ze-software/ze/internal/core/slogutil"
)

func appendAIGPJSON(buf []byte, attr attribute.Attribute) []byte {
	a, ok := attr.(*attribute.AIGP)
	if !ok {
		return nil
	}
	metric, found := a.Metric()
	if !found {
		return nil
	}
	return strconv.AppendUint(buf, metric, 10)
}

func init() {
	attribute.RegisterJSONFormatter(attribute.AttrAIGP, "aigp", appendAIGPJSON)
	reg := registry.Registration{
		Name:        "bgp-aigp",
		Description: "Accumulated IGP Metric (RFC 7311)",
		RFCs:        []string{"7311"},
		RunEngine:   runAIGPPlugin,
		ConfigureEngineLogger: func(loggerName string) {
			setAIGPLogger(slogutil.Logger(loggerName))
		},
	}
	reg.CLIHandler = func(args []string) int {
		cfg := cli.BaseConfig(&reg)
		cfg.ConfigLogger = func(level string) {
			setAIGPLogger(slogutil.PluginLogger(reg.Name, level))
		}
		return cli.RunPlugin(cfg, args)
	}
	if err := registry.Register(reg); err != nil {
		slog.Error("aigp: registration failed", "error", err)
	}
}
